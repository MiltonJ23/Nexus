# Internals: Virtual File System

Packages: `internal/core/fs.go`, `internal/service/fs_service.go`

---

## Data model

The VFS is a pure in-memory tree rooted at `/`. It is serialised as part of `AppState.FileSystems` in `nexus.json`.

```go
type FSNode struct {
    Name      string
    Type      NodeType    // "file" | "folder"
    FileID    string      // non-empty only for Type==file → links to FileMetadata
    Size      int64       // file size, or recursive sum for folders (not auto-updated)
    CreatedAt time.Time
    Children  []*FSNode   // non-nil only for folders
    Parent    *FSNode     // omitted from JSON (internal traversal only)
}

type UserFileSystem struct {
    Username   string
    Root       *FSNode    // root node, Name="/"
    QuotaLimit int64      // default 500 MB
    QuotaUsed  int64
}
```

One `UserFileSystem` per authenticated user. Created on first access with a 500 MB quota limit.

---

## Path resolution

`resolvePath(fs, path)` splits the path on `/`, then walks the tree from `fs.Root` matching each component against children by name. Returns the target node, its parent, and an error if any component is not found.

```
"/documents/projects/readme.txt"
    → parts: ["documents", "projects", "readme.txt"]
    → walk: root → "documents" (folder) → "projects" (folder) → "readme.txt" (file)
```

Returns `(targetNode, parentNode, nil)` on success.

---

## Operations

### MakeDirectory (mkdir)

1. Resolve the full path — error if it already exists.
2. Resolve the parent path — error if parent does not exist or is not a folder.
3. Append a new `FSNode{Type: folder}` to parent's children.
4. Save state.

No `mkdir -p`. Each intermediate directory must exist.

### UploadFileToPath

1. Quota pre-check is deferred until after physical upload.
2. Call `FileService.UploadFile(localPath)` — chunks the file across nodes.
3. If `quota_used + file_size > quota_limit`: delete physical chunks and return error.
4. Resolve parent folder in the VFS tree.
5. If parent not found: delete physical chunks (orphan cleanup) and return error.
6. Append `FSNode{Type: file, FileID: meta.ID, Size: meta.Size}` to parent.
7. Increment `fs.QuotaUsed`.
8. Save state.

### Delete (recursive)

1. Resolve target node and its parent.
2. Refuse to delete root.
3. Recursive walk: for every `file` node, call `FileService.DeleteFile(fileID)` and decrement quota. For folder nodes, recurse into children first.
4. Remove target from parent's children slice.
5. Save state.

### Move / Rename

1. Resolve source node and source parent.
2. Resolve destination parent folder.
3. Detach source from source parent's children.
4. Update `sourceNode.Name` to the last component of the destination path.
5. Append source node to destination parent's children.
6. Save state.

Physical chunks are not moved — only the VFS pointer changes.

### ListDir (ls)

Resolve the path, assert it is a folder, return `node.Children`.

---

## Quota enforcement

- Default: 500 MB per user (`500 * 1024 * 1024` bytes).
- Tracked in `UserFileSystem.QuotaUsed` (incremented on upload, decremented on delete).
- Checked in `UploadFileToPath` **after** physical upload (rollback on excess).
- No enforcement at mkdir.
- No quota for the raw `FileService` (CLI `file upload`); only the VFS layer enforces quotas.

---

## Persistence

The entire VFS tree is serialised to JSON as part of `AppState`. The `Parent` pointer is tagged `json:"-"` to avoid circular reference during marshalling. Parent pointers are **not** restored on deserialization — they are not needed by any current operation (path resolution walks top-down only).

---

## Limitations

- **No atomic rename.** Move is two operations (detach + attach) with no rollback.
- **No hard links or symlinks.**
- **Folder size not recursively maintained.** `FSNode.Size` for folders is not updated on child changes.
- **No file versioning.**
- **Username is hardcoded to `"cli-user"` in the CLI.** Full auth integration is pending.
