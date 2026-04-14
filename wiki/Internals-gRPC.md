# Internals: gRPC Daemon

Packages: `internal/grpc/server.go`, `cmd/daemon.go`, `cmd/client_utils.go`, `proto/nexus.proto`

---

## Transport

Unix domain socket at `/var/run/nexus.sock`.

The daemon (server) creates the socket with `net.Listen("unix", "/var/run/nexus.sock")`. Permissions are set to `0777` after binding so that non-root CLI processes can connect. In production this should be restricted to a group (e.g. `nexus`) rather than world-writable.

The CLI (client) connects via `grpc.NewClient("unix:///var/run/nexus.sock", ...)` with no TLS (`insecure.NewCredentials()`).

---

## Proto: NexusController

```protobuf
service NexusController {
    // Nodes
    rpc CreateNode       (CreateNodeRequest)    returns (NodeResponse);

    // Raw file store
    rpc UploadFile       (UploadFileRequest)    returns (FileResponse);
    rpc DownloadFile     (DownloadFileRequest)  returns (FileResponse);
    rpc ListFiles        (Empty)                returns (FileListResponse);

    // Metrics
    rpc GetNodeMetrics   (NodeMetricsRequest)   returns (NodeMetricsResponse);

    // Lambda
    rpc RunLambda        (LambdaRequest)        returns (LambdaResponse);

    // Virtual File System
    rpc FSMakeDir        (FSRequest)            returns (Empty);
    rpc FSList           (FSRequest)            returns (FSListResponse);
    rpc FSDelete         (FSRequest)            returns (GenericResponse);
    rpc FSUpload         (FSUploadRequest)      returns (GenericResponse);
    rpc FSMove           (FSMoveRequest)        returns (GenericResponse);
}
```

Generated files: `proto/nexus.pb.go`, `proto/nexus_grpc.pb.go`.

---

## NexusServer

```go
type NexusServer struct {
    pb.UnimplementedNexusControllerServer
    nodeService   *service.NodeService
    fileService   *service.FileService
    lambdaService *service.LambdaService
}
```

`NewNexusServer()` initialises all services and wires them. `NodeService` initialisation includes:
- Libcontainer runtime setup (creates `/run/nexus`, cgroup parent)
- Bridge setup (`nexus0`)
- State load from `nexus.json`

If any of these fail, the daemon panics at startup.

---

## Daemon startup sequence

```
cmd/daemon.go: daemonCmd.Run()
  1. Assert euid == 0
  2. Remove stale socket if exists
  3. net.Listen("unix", /var/run/nexus.sock)
  4. grpc.NewServer()
  5. grpcserver.NewNexusServer()         ← initialises services
  6. pb.RegisterNexusControllerServer()
  7. service.NewMetricsService().Start(5s)
  8. goroutine: wait for SIGINT/SIGTERM → GracefulStop + remove socket
  9. os.Chmod(socket, 0777)
 10. grpcServer.Serve(lis)               ← blocks
```

---

## Key RPC implementations

### CreateNode

```go
nodeService.CreateNode(name, memMB, cpuShares, storageSize)
    → (NodeState{id, ip, status}, error)
```

Maps `NodeResponse{id, ip, status}` back to the caller.

### UploadFile

The daemon resolves the path to absolute (`filepath.Abs`) before passing it to `FileService`. This is critical: the daemon runs as root with a potentially different working directory than the client. The gateway pre-stages files at `/tmp/nexus_uploads/<filename>` before calling this RPC.

### DownloadFile

The daemon writes the reassembled file to `req.DestPath`. The gateway sets `DestPath` to `/tmp/nexus_downloads/<fileID>` and then serves it via `c.File()`.

### GetNodeMetrics

Reads `state.GlobalState.GetNode(nodeID).LatestMetrics`. Returns the last value written by the metrics goroutine (may be up to 5 s stale).

### RunLambda

Calls `lambdaService.Execute(code, runtime)`. Synchronous — blocks until the container exits. Timeout handling is not yet implemented; a malicious/infinite loop will block this RPC indefinitely.

---

## Client utility

`cmd/client_utils.go: getClient()` creates a gRPC connection and returns `(NexusControllerClient, *grpc.ClientConn, context.Context)`. The context has a 10-second connection timeout. The connection is kept open only for the duration of one CLI invocation (`defer conn.Close()`).
