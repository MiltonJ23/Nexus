# Nexus Wiki

Nexus is a simulated cloud platform built entirely on Linux kernel primitives — no Docker, no Kubernetes, no managed cloud. It is an educational and experimental project that re-implements the essential building blocks of a cloud provider from scratch in Go.

---

## Pages

### Setup & Usage

| Page | Description |
|------|-------------|
| [Getting Started](Getting-Started.md) | Prerequisites, build, bootstrap Alpine rootfs, first run |
| [CLI Reference](CLI-Reference.md) | All `nexus` commands, flags, and examples |
| [HTTP API Reference](HTTP-API-Reference.md) | REST endpoints, auth flow, request/response shapes |

### Architecture & Internals

| Page | Description |
|------|-------------|
| [Architecture](Architecture.md) | Full system design: layers, data flows, component interactions |
| [Internals: Runtime](Internals-Runtime.md) | libcontainer, Linux namespaces, cgroups v2 |
| [Internals: Storage](Internals-Storage.md) | Loop devices, ext4, chunking algorithm |
| [Internals: Networking](Internals-Networking.md) | Linux bridge, veth pairs, IPAM |
| [Internals: VFS](Internals-VFS.md) | Per-user virtual file system tree, quota |
| [Internals: Identity](Internals-Identity.md) | OTP email auth, JWT, SQLite via GORM |
| [Internals: Metrics](Internals-Metrics.md) | cgroup-based CPU/memory collection |
| [Internals: gRPC](Internals-gRPC.md) | Unix socket daemon, proto definitions |

### Project

| Page | Description |
|------|-------------|
| [Roadmap](Roadmap.md) | Milestone tracker (M4 → full platform) |
