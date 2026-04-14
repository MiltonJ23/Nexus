# HTTP API Reference

The HTTP gateway (`nexus-api`) is a Gin server that proxies all operations to the `nexusd` daemon via gRPC. It also forwards auth validation to the `nexus-auth` service.

Default listen address is configured in the gateway binary.

---

## Authentication

All `/api/*` routes require a valid JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

The middleware validates the token by calling `AuthService.ValidateToken` over gRPC on every request. On success the `username` claim is injected into the Gin context for downstream handlers.

### Register

**Step 1 — initiate registration**

```
POST /auth/register/init
Content-Type: application/json

{
  "Username": "alice",
  "Password": "hunter2",
  "Email": "alice@example.com"
}
```

Response `200 OK`:
```json
{ "message": "OTP sent to email" }
```

The server stores a `PendingRegistration` row (hashed password, 5-minute OTP) and sends the OTP by email.

**Step 2 — verify OTP**

```
POST /auth/register/verify
Content-Type: application/json

{
  "Email": "alice@example.com",
  "Code": "123456"
}
```

Response `200 OK`:
```json
{ "token": "<jwt>", "username": "alice" }
```

JWT is valid for 24 hours.

---

### Login

**Step 1 — initiate login**

```
POST /auth/login/init
Content-Type: application/json

{
  "Username": "alice",
  "Password": "hunter2"
}
```

Response `200 OK`:
```json
{ "message": "OTP sent to alice@example.com" }
```

**Step 2 — verify OTP**

```
POST /auth/login/verify
Content-Type: application/json

{
  "Email": "alice@example.com",
  "Code": "654321"
}
```

Response `200 OK`:
```json
{ "token": "<jwt>", "username": "alice" }
```

---

## Nodes

### Create node

```
POST /api/nodes
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "name": "worker1",
  "memory_mb": 256,
  "cpu_shares": 512,
  "storage_size": "1G"
}
```

Response `201 Created`:
```json
{ "id": "worker1", "ip": "10.0.42.2", "status": "Running" }
```

---

## Files (raw)

### Upload file

```
POST /api/files/upload
Authorization: Bearer <jwt>
Content-Type: multipart/form-data

file=@/path/to/file.tar.gz
```

The gateway saves the file to `/tmp/nexus_uploads/<filename>`, then sends the path to the daemon via `UploadFile` RPC.

Response `200 OK`:
```json
{
  "id": "<uuid>",
  "name": "file.tar.gz",
  "size": 104857600,
  "chunks_count": 10,
  "status": "Uploaded"
}
```

### List files

```
GET /api/files
Authorization: Bearer <jwt>
```

Response `200 OK`: array of file objects.

### Download file

```
GET /api/files/:id/download
Authorization: Bearer <jwt>
```

The daemon reconstructs the file to `/tmp/nexus_downloads/<id>` and the gateway serves it directly via `c.File()`.

---

## Metrics

```
GET /api/nodes/:id/metrics
Authorization: Bearer <jwt>
```

Response `200 OK`:
```json
{
  "node_id": "worker1",
  "memory_usage": 52428800,
  "memory_limit": 268435456,
  "cpu_percent": 0.42
}
```

Metrics are collected from cgroups every 5 seconds and kept in memory.

---

## Lambda

```
POST /api/lambda/run
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "Code": "print('hello from nexus')",
  "Runtime": "python3"
}
```

Response `200 OK`:
```json
{
  "stdout": "hello from nexus\n",
  "stderr": "",
  "exit_code": 0,
  "execution_time_ms": 312
}
```

Code runs in an ephemeral container (Alpine + python3, 128 MB RAM, isolated network namespace). The container is destroyed after execution.

---

## Virtual File System

All VFS routes extract `username` from the validated JWT.

### Create directory

```
POST /api/fs/mkdir
Authorization: Bearer <jwt>
Content-Type: application/json

{ "path": "/documents/projects" }
```

Response `200 OK`:
```json
{ "message": "Directory created", "path": "/documents/projects" }
```

### List directory

```
GET /api/fs/ls?path=/documents
Authorization: Bearer <jwt>
```

Response `200 OK`: array of `{ name, type, size }` objects where `type` is `"file"` or `"folder"`.

### Upload to VFS path

```
POST /api/fs/upload
Authorization: Bearer <jwt>
Content-Type: multipart/form-data

path=/documents/report.pdf
file=@/local/report.pdf
```

Performs quota check. Rolls back physical chunks if quota exceeded.

### Delete

```
DELETE /api/fs/delete?path=/documents/old.txt
Authorization: Bearer <jwt>
```

Recursive for directories. Deletes physical chunks.

### Move / Rename

```
POST /api/fs/move
Authorization: Bearer <jwt>
Content-Type: application/json

{ "old_path": "/docs/draft.txt", "new_path": "/archive/final.txt" }
```

---

## Error format

All errors return JSON:

```json
{ "error": "human readable message" }
```
