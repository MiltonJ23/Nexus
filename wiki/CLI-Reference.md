# CLI Reference

The `nexus` binary is both the CLI client and the daemon launcher. All client commands connect to the daemon at `/var/run/nexus.sock` via gRPC.

```
nexus <command> [subcommand] [flags]
```

---

## nexus daemon

Start the Nexus control plane. **Requires root.**

```sh
sudo ./nexus daemon
```

- Creates `/var/run/nexus.sock` (chmod 0777)
- Starts gRPC server on the socket
- Initialises cgroup parent `/sys/fs/cgroup/nexus`
- Sets up bridge `nexus0`
- Starts metrics collector (5 s interval)
- Cleans up socket and stops gracefully on SIGINT/SIGTERM

No flags.

---

## nexus node

Manage the lifecycle of compute nodes.

### nexus node create \<name\>

Create and start a new node.

```sh
nexus node create <name> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--mem <MB>` | `128` | Memory limit in megabytes |
| `--cpu <weight>` | `512` | CPU weight (cgroup, range 1–10000) |
| `--storage <size>` | — | Volume size (e.g. `500M`, `1G`). Required. |

**Example:**

```sh
nexus node create worker1 --mem 512 --cpu 1024 --storage 2G
```

**What happens:**
1. IPAM assigns next free IP from `10.0.42.0/24`
2. `truncate -s <size>` creates a sparse image file, then `mkfs.ext4 -F` formats it
3. `losetup -f --show -P` attaches the image to a loop device
4. libcontainer creates and starts the container (Alpine rootfs, `sleep 3600`)
5. Netlink wires a veth pair into the container's network namespace
6. State is persisted to `nexus.json`

**Output:**

```
-> Node Created via gRPC!
   ID: worker1 | IP: 10.0.42.2 | Status: Running
```

---

## nexus file

Manage distributed files.

### nexus file upload \<local-path\>

Split and distribute a file across all running nodes.

```sh
nexus file upload /path/to/file.bin
```

- Chunks: 10 MB each
- Distribution: round-robin across active nodes
- Integrity: MD5 checksum per chunk stored in metadata
- Returns a UUID file ID

### nexus file download \<file-id\> \<dest-path\>

Reassemble chunks and write to `dest-path`.

```sh
nexus file download 3f2a1b0c-... /tmp/restored.bin
```

Chunks are read in index order from their respective nodes and concatenated.

### nexus file ls

List all stored files.

```sh
nexus file ls
```

Output columns: `ID` (first 8 chars), `NAME`, `SIZE` (bytes), `CHUNKS`.

---

## nexus fs

Interact with the per-user Virtual File System.

> **Note:** The CLI currently uses `cli-user` as a hardcoded username. Full auth integration is pending.

### nexus fs mkdir \<path\>

Create a directory in the VFS.

```sh
nexus fs mkdir /documents/projects
```

Parent must exist. No `-p` (recursive create) yet.

### nexus fs ls \[path\]

List contents of a VFS directory.

```sh
nexus fs ls /
nexus fs ls /documents
```

Output: `TYPE`, `NAME`, `SIZE`

---

## Global notes

- The CLI exits non-zero and prints an error if the daemon is not running.
- Socket path: `/var/run/nexus.sock` (constant `SocketAddr = "unix:///var/run/nexus.sock"`).
- Connection timeout: 10 seconds.
- The `nexus init` argument is reserved for libcontainer's internal container-init path. Do not invoke it directly.
