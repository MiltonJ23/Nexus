# Nexus

Simulated cloud platform built on Linux primitives. Runs containers via libcontainer, distributes files across nodes using loop-device-backed ext4 volumes, and exposes a privileged gRPC Unix socket as the control plane.

**Status:** Active development. Linux x86\_64 only. Daemon requires root.

---

## What it does

| Subsystem | Implementation |
|-----------|---------------|
| **Nodes** | Linux namespaces + cgroups v2 via libcontainer (runc) |
| **Networking** | Linux bridge `nexus0`, veth pairs, sequential IPAM (10.0.42.0/24) |
| **Storage** | Per-node ext4 image files attached via loop devices (`/var/lib/nexus/volumes/`) |
| **Distributed FS** | 10 MB chunk round-robin across active nodes, MD5 integrity |
| **Virtual FS** | Per-user VFS tree (quota-enforced) layered over the chunk store |
| **Lambda** | Ephemeral containers for sandboxed Python execution |
| **Auth** | OTP email → JWT; SQLite backend via GORM |
| **REST API** | Gin HTTP gateway proxying all operations to the daemon via gRPC |
| **Metrics** | cgroup CPU/memory scraping every 5 s, kept in-memory |

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  CLI (Cobra)          REST clients / Frontend         │
└────────────┬──────────────────┬──────────────────────┘
             │ gRPC (Unix sock) │ HTTP
             ▼                  ▼
    ┌─────────────────┐  ┌─────────────────────┐
    │  nexusd daemon  │  │   HTTP Gateway (Gin) │
    │  /var/run/nexus │  │   + AuthMiddleware   │
    │  .sock          │◄─┤                     │
    └────────┬────────┘  └──────────┬──────────┘
             │                      │ gRPC
             │              ┌───────▼────────┐
             │              │  Auth Service  │
             │              │  (OTP + JWT)   │
             │              └────────────────┘
             ▼
    ┌─────────────────────────────────────┐
    │         Service Layer               │
    │  NodeService / FileService /        │
    │  FSService / LambdaService /        │
    │  MetricsService                     │
    └──┬───────────┬──────────────┬───────┘
       ▼           ▼              ▼
  libcontainer  loop+ext4     netlink
  (namespaces   (StorageMgr)  (bridge/veth
   cgroups)                    IPAM)
```

Internal layout follows hexagonal architecture:

```
internal/
  core/        — domain types (NodeConfig, NodeState, FileMetadata, FSNode)
  ports/       — interfaces (ContainerRuntime, StorageManager, NetworkManager)
  adapters/    — concrete implementations (libcontainer, loop, netlink, cgroup)
  service/     — use-cases (node, file, fs, lambda, metrics)
  state/       — JSON persistence (/var/lib/nexus/nexus.json)
  grpc/        — gRPC server (NexusServer)
  gateway/     — HTTP router (Gin)
  identity/    — auth gRPC server (JWT + OTP + SQLite)
```

---

## Prerequisites

- Linux x86\_64, kernel ≥ 5.x, cgroups v2 unified hierarchy
- `losetup`, `mkfs.ext4`, `mount`, `umount` in PATH
- Go ≥ 1.22 (module requires 1.25.3)
- Alpine rootfs unpacked at `/var/lib/nexus/images/alpine-base`
- Daemon must run as root

---

## Build

```sh
go build -o nexus .
```

---

## Quick start

```sh
# 1. Start the daemon (root required)
sudo ./nexus daemon

# 2. Create a node (256 MB RAM, default CPU weight, 1 G volume)
./nexus node create mynode --mem 256 --cpu 512 --storage 1G

# 3. Distribute a file across active nodes
./nexus file upload /path/to/bigfile.tar.gz

# 4. List stored files
./nexus file ls

# 5. Download by file ID (first 8 chars shown in ls)
./nexus file download <full-uuid> /tmp/restored.tar.gz

# 6. Virtual File System
./nexus fs mkdir /documents
./nexus fs ls /
```

---

## REST API (HTTP Gateway)

All `/api/*` routes require `Authorization: Bearer <jwt>`.

```
POST   /auth/register/init       register → OTP sent
POST   /auth/register/verify     OTP → JWT token
POST   /auth/login/init          login → OTP sent
POST   /auth/login/verify        OTP → JWT token

POST   /api/nodes                create node
POST   /api/files/upload         multipart file upload
GET    /api/files                list all files
GET    /api/files/:id/download   download file by ID
GET    /api/nodes/:id/metrics    live cgroup metrics
POST   /api/lambda/run           run Python code in sandbox

POST   /api/fs/mkdir             mkdir in VFS
GET    /api/fs/ls?path=…         ls in VFS
POST   /api/fs/upload            upload to VFS path
DELETE /api/fs/delete?path=…     rm (recursive) in VFS
POST   /api/fs/move              mv / rename in VFS
```

---

## State

| Path | Content |
|------|---------|
| `/var/lib/nexus/nexus.json` | Nodes, files, VFS trees (JSON) |
| `/var/lib/nexus/auth.db` | Users, pending registrations, login OTPs (SQLite) |
| `/var/lib/nexus/volumes/<id>.img` | Per-node ext4 disk images |
| `/run/nexus/<id>/` | libcontainer container state |
| `/var/run/nexus.sock` | gRPC Unix socket (daemon) |

---

## Wiki

Deep-dive documentation lives in [`wiki/`](wiki/):

- [Home](wiki/Home.md) — navigation
- [Architecture](wiki/Architecture.md) — full system design
- [Getting Started](wiki/Getting-Started.md) — setup from scratch
- [CLI Reference](wiki/CLI-Reference.md) — all commands and flags
- [HTTP API Reference](wiki/HTTP-API-Reference.md) — REST endpoints
- [Internals: Runtime](wiki/Internals-Runtime.md) — libcontainer, namespaces, cgroups
- [Internals: Storage](wiki/Internals-Storage.md) — loop devices, chunking
- [Internals: Networking](wiki/Internals-Networking.md) — bridge, veth, IPAM
- [Internals: VFS](wiki/Internals-VFS.md) — virtual file system
- [Internals: Identity](wiki/Internals-Identity.md) — auth service
- [Internals: Metrics](wiki/Internals-Metrics.md) — cgroup scraper
- [Internals: gRPC](wiki/Internals-gRPC.md) — daemon protocol
- [Roadmap](wiki/Roadmap.md) — milestones

---

## License

See [LICENSE](LICENSE).
