# Roadmap

Milestones from the original design document, updated with current implementation status.

---

## Milestone 4 — Storage Engine ✅

**Goal:** Prove that the storage logic works end-to-end from the CLI.

| Task | Status |
|------|--------|
| `file upload <path>` — chunking + round-robin | ✅ Done |
| `file download <id> <dest>` — reassembly | ✅ Done |
| `file delete <id>` — chunk cleanup | ✅ Done |
| `file ls` — list stored files | ✅ Done |
| `MountOnHost` / `UnmountOnHost` via loop device | ✅ Done |
| `WriteChunk` / `ReadChunk` via direct file I/O | ✅ Done |
| MD5 checksum per chunk | ✅ Done (stored, not verified on read) |

---

## Milestone 5 — Daemon (nexusd) ✅

**Goal:** Move privileged operations into a long-running daemon; CLI becomes a thin gRPC client.

| Task | Status |
|------|--------|
| `nexusd` daemon with gRPC Unix socket | ✅ Done |
| Cobra CLI refactored to gRPC client | ✅ Done |
| `nexus daemon` command | ✅ Done |
| Metrics service (5 s cgroup polling) | ✅ Done |
| Graceful shutdown (SIGINT/SIGTERM) | ✅ Done |

---

## Milestone 6 — Identity & REST Gateway ✅

**Goal:** JWT auth + Gin HTTP gateway so the Next.js UI can talk to the platform.

| Task | Status |
|------|--------|
| OTP email registration | ✅ Done |
| OTP email login | ✅ Done |
| JWT token issuance (24 h) | ✅ Done |
| `ValidateToken` RPC | ✅ Done |
| Gin HTTP gateway | ✅ Done |
| `AuthMiddleware` (JWT validation per request) | ✅ Done |
| Virtual File System (mkdir/ls/upload/delete/move) | ✅ Done |
| Lambda (`POST /api/lambda/run`) | ✅ Done |
| Node metrics endpoint | ✅ Done |

---

## Milestone 7 — Compute (SSH Instances) 🔲

**Goal:** Users can launch a container and get an SSH connection string from the UI.

| Task | Status |
|------|--------|
| Alpine image with `openssh-server` and pre-configured keys | 🔲 Pending |
| Port forwarding: `host:2222 → container:22` | 🔲 Pending |
| `StopNode` / `RemoveNode` RPCs | 🔲 Pending |
| `ListNodes` RPC + HTTP endpoint | 🔲 Pending |
| "Connect" button in UI → SSH string | 🔲 Pending |

---

## Known issues / Technical debt

| Issue | Priority |
|-------|----------|
| JWT secret hardcoded (`apotheose-secret-key-2025`) | Critical |
| Unix socket is `0777` (world-writable) | High |
| Loop devices never released (`losetup -d` missing) | High |
| No concurrent write protection per node volume | High |
| Metrics scraper uses cgroup v1 paths, runtime uses cgroup v2 | Medium |
| `Stop()` and `GetState()` in runtime adapter are stubs | Medium |
| MD5 checksums stored but never validated on download | Medium |
| No Lambda execution timeout | Medium |
| CLI VFS commands use hardcoded `"cli-user"` username | Low |
| No `mkdir -p` support | Low |
| Folder size (`FSNode.Size`) not maintained recursively | Low |
| No IP release on node deletion (`ReleaseIP` exists but unused) | Low |
