# Internals: Storage

Packages: `internal/adapters/storage`, `internal/ports/storage.go`, `internal/service/file_service.go`

---

## Interface

```go
type StorageManager interface {
    CreateVolume(nodeID string, size string) (string, error)
    AttachVolume(volumePath string) (string, error)
    MountOnHost(nodeID string) (string, error)
    UnmountOnHost(nodeID string) error
    WriteChunk(mountPoint string, filename string, data io.Reader) error
    ReadChunk(mountPoint string, filename string) (io.ReadCloser, error)
    DeleteChunk(mountPoint string, filename string) error
}
```

Implemented by `LoopStorageManager`.

---

## Volume lifecycle

### 1. CreateVolume

Creates a sparse ext4 disk image for a node:

```sh
truncate -s <size> /var/lib/nexus/volumes/<nodeID>.img
mkfs.ext4 -F /var/lib/nexus/volumes/<nodeID>.img
```

The `-F` flag forces formatting even though the target is not a block device. The result is a regular file containing a valid ext4 filesystem.

Returns the image path.

### 2. AttachVolume

Attaches the image to the next available loop device:

```sh
losetup -f --show -P /var/lib/nexus/volumes/<nodeID>.img
```

`-f` auto-selects the first free loop device (e.g. `/dev/loop3`).
`--show` prints the device name.
`-P` triggers partition scanning (good practice, no partitions in practice here).

Returns the loop device path (e.g. `/dev/loop3`). This is stored in `NodeConfig.VolumePath` and later passed to libcontainer as a mount source.

---

## Chunk I/O

File upload and download always go through a mount/unmount cycle on the host.

### MountOnHost

```sh
mkdir -p /tmp/nexus/mounts/<nodeID>
mount -o loop /var/lib/nexus/volumes/<nodeID>.img /tmp/nexus/mounts/<nodeID>
```

Returns the mount point path.

> **Important:** Linux allows mounting the same ext4 image simultaneously only if one of the mounts is read-only. Nexus does not enforce single-writer ordering. Concurrent uploads to the same node will corrupt the filesystem. This is a known limitation (see roadmap).

### UnmountOnHost

```sh
umount /tmp/nexus/mounts/<nodeID>
rmdir /tmp/nexus/mounts/<nodeID>
```

Always called in a `defer` after every chunk write or read, ensuring mounts are not left dangling.

### WriteChunk / ReadChunk / DeleteChunk

Simple `os.Create` / `os.Open` / `os.Remove` at `<mountPoint>/<chunkFilename>`. No subdirectories — all chunks for all files sharing a node are flat in the root of the mounted filesystem.

---

## Chunking algorithm (FileService)

Chunk size: **10 MB** (`const ChunkSize = 10 * 1024 * 1024`)

```
for each 10 MB slice of the input file:
    targetNode = activeNodes[ chunkIndex % len(activeNodes) ]   // round-robin
    chunkFilename = "<fileUUID>_part_<index>"
    checksum = MD5(chunk bytes)
    MountOnHost(targetNode)
    WriteChunk(mountPoint, chunkFilename, chunkReader)
    UnmountOnHost(targetNode)
    append Chunk{index, nodeID, chunkFilename, size, checksum} to metadata
```

Metadata (file ID, name, size, chunk list) is saved to the state JSON.

**Download** iterates `metadata.Chunks` in index order, reading each chunk from its node and streaming it to the output file:

```
for each chunk in metadata.Chunks (ordered by Index):
    MountOnHost(chunk.NodeID)
    ReadChunk(mountPoint, chunk.PathOnDisk)  → io.ReadCloser
    io.Copy(outputFile, reader)
    UnmountOnHost(chunk.NodeID)
```

**Delete** mounts each node, removes the chunk file, unmounts, then removes the metadata entry from state.

---

## Directory layout

```
/var/lib/nexus/volumes/
├── worker1.img          ext4 disk image
├── worker2.img
└── ...

/tmp/nexus/mounts/
└── worker1/             temporary mount point (exists only during I/O)
```

---

## Limitations

- **No concurrent access control.** Two simultaneous writes to the same node image corrupt ext4.
- **No checksum verification on read.** MD5 is stored per chunk but not validated during download.
- **Loop devices not released on shutdown.** `losetup -d` is never called. Orphaned loop devices accumulate until system reboot or manual `losetup -D`.
- **No replication.** A failed/deleted node means permanent data loss for any chunks stored on it.
