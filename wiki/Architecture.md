# Architecture

## Overview

Nexus is structured as three independent processes that communicate exclusively over gRPC:

```
┌──────────────────────────────────────────────────────────────────┐
│  User space (unprivileged)                                        │
│                                                                   │
│   CLI binary              REST client / browser frontend          │
│   (cobra)                                                         │
└──────────┬────────────────────────────┬─────────────────────────┘
           │ gRPC over Unix socket      │ HTTP/1.1
           │ /var/run/nexus.sock        ▼
           │                  ┌──────────────────────────┐
           │                  │  HTTP Gateway (Gin)       │
           │                  │  nexus-api binary         │
           │                  │                           │
           │                  │  AuthMiddleware           │
           │                  │   → validates JWT via     │
           │                  │     gRPC to Auth server   │
           │                  └───────────┬──────────────┘
           │                              │ gRPC
           │                  ┌───────────▼──────────────┐
           │                  │  Auth Service             │
           │                  │  nexus-auth binary        │
           │                  │  OTP + JWT + SQLite       │
           │                  └──────────────────────────┘
           ▼
┌──────────────────────────────────────────────────────────────────┐
│  nexusd daemon  (runs as root)                                    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  gRPC Server  (NexusServer)                               │    │
│  │  internal/grpc/server.go                                  │    │
│  └──────────┬──────────────┬───────────────┬───────────────┘    │
│             │              │               │                      │
│             ▼              ▼               ▼                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │
│  │ NodeService  │ │ FileService  │ │  FSService   │            │
│  │              │ │              │ │              │            │
│  │ LambdaService│ │MetricsService│ │              │            │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘            │
│         │                │                │                      │
│         ▼                ▼                ▼                      │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Ports (interfaces)                                       │    │
│  │  ContainerRuntime  |  StorageManager  |  NetworkManager   │    │
│  └──────┬─────────────────────┬──────────────────┬──────────┘    │
│         ▼                     ▼                  ▼               │
│  libcontainer            loop devices        netlink             │
│  (namespaces,            (ext4 images,       (bridge nexus0,     │
│   cgroups v2)            /var/lib/nexus/     veth pairs,         │
│                          volumes/)           IPAM)               │
│                                                                   │
│  State: /var/lib/nexus/nexus.json  (JSON, RWMutex-protected)     │
└──────────────────────────────────────────────────────────────────┘
```

---

## Internal package structure

Hexagonal (ports-and-adapters) architecture. The domain and use-cases have zero dependency on concrete Linux APIs.

```
internal/
├── core/               Domain types — no external imports
│   ├── domain.go       NodeConfig, NodeState, IPAddress, NodeMetrics
│   ├── file.go         FileMetadata, Chunk
│   └── fs.go           FSNode, UserFileSystem, NodeType
│
├── ports/              Interfaces — contracts the service layer depends on
│   ├── runtime.go      ContainerRuntime
│   ├── storage.go      StorageManager
│   └── network.go      NetworkManager
│
├── adapters/           Concrete implementations of the ports
│   ├── runtime/
│   │   └── libcontainer.go   runc/libcontainer, cgroup creation
│   ├── storage/
│   │   └── loop_manager.go   truncate, mkfs.ext4, losetup, mount
│   ├── network/
│   │   ├── network_manager.go  netlink bridge, veth, namespace entry
│   │   └── ipam.go             sequential IPAM (10.0.42.0/24)
│   └── metrics/
│       └── cgroup_scraper.go   reads /sys/fs/cgroup directly
│
├── service/            Use-cases (pure business logic)
│   ├── node_service.go     orchestrates runtime + network + storage + state
│   ├── file_service.go     chunking, upload, download, delete
│   ├── fs_service.go       VFS mkdir/ls/upload/delete/move + quota
│   ├── lambda_service.go   ephemeral container execution
│   └── metrics_service.go  periodic collection loop
│
├── state/
│   └── storage.go          AppState, StateManager (JSON R/W, RWMutex)
│
├── grpc/
│   └── server.go           NexusServer, wires all services to proto RPCs
│
├── gateway/
│   └── router.go           Gin router, CORS, AuthMiddleware, all handlers
│
└── identity/
    ├── service.go          AuthServer: register/login/validate (gRPC)
    └── emailer.go          OTP email delivery
```

---

## Request lifecycle — file upload (CLI)

```
nexus file upload /data/video.mp4
  │
  ├─ cmd/file.go: uploadCmd.Run()
  │     calls getClient() → gRPC conn to /var/run/nexus.sock
  │     sends UploadFileRequest{local_path: "/data/video.mp4"}
  │
  ├─ internal/grpc/server.go: UploadFile()
  │     resolves absolute path
  │     calls fileService.UploadFile(absPath)
  │
  ├─ internal/service/file_service.go: UploadFile()
  │     opens file, reads size
  │     gets active node IDs from state
  │     loops in 10 MB chunks:
  │       round-robin select target node
  │       storage.MountOnHost(nodeID)   → mount -o loop <img> /tmp/nexus/mounts/<id>
  │       storage.WriteChunk(mount, filename, reader)
  │       storage.UnmountOnHost(nodeID) → umount
  │       appends Chunk{index, nodeID, path, size, md5}
  │     state.AddFile(metadata)
  │     state.Save() → /var/lib/nexus/nexus.json
  │
  └─ gRPC response → CLI prints file ID
```

---

## Data model

```
AppState (nexus.json)
├── nodes: map[id]NodeState
│     NodeConfig (id, hostname, memory, cpuShares, rootfsPath, volumePath)
│     PID, Status, IP{ip, subnet, gateway}
│     LatestMetrics{memUsage, memLimit, cpuPercent, cpuTotal}
│
├── files: map[uuid]FileMetadata
│     id, name, size, createdAt
│     chunks[]: {index, nodeID, pathOnDisk, size, checksum}
│
└── file_systems: map[username]UserFileSystem
      username, quotaLimit, quotaUsed
      root: FSNode tree
        FSNode{name, type(file|folder), fileID?, size, createdAt, children[]}
```

---

## Security boundaries

| Trust boundary | Mechanism |
|---------------|-----------|
| CLI ↔ daemon | Unix socket at `/var/run/nexus.sock` (chmod 0777 — see roadmap) |
| HTTP client ↔ gateway | JWT Bearer token validated via Auth gRPC call per request |
| Container ↔ host | Linux namespaces (PID, NET, MNT, UTS, IPC), cgroups v2 resource limits |
| Lambda code | Minimal capability set, isolated network namespace, ephemeral cgroup |

> **Note:** The socket is currently `0777`. Production hardening requires a dedicated `nexus` group.
