# Internals: Container Runtime

Package: `internal/adapters/runtime`, `internal/ports/runtime.go`

---

## Interface

```go
type ContainerRuntime interface {
    CreateAndStart(conf core.NodeConfig) (*core.NodeState, error)
    Stop(id string) error
    GetState(id string) (*core.NodeState, error)
    RunEphemeral(conf core.NodeConfig, stdout, stderr io.Writer) (int, error)
}
```

The single implementation is `LibContainerRuntime`, backed by `github.com/opencontainers/runc/libcontainer`.

---

## libcontainer

libcontainer is the container creation library that powers `runc`. Nexus calls it directly without the OCI spec layer, giving full control over the container configuration.

Key call sites:

```go
container, err := libcontainer.Create(statePath, id, config)
container.Run(process)
process.Pid()       // host PID for network namespace entry
process.Wait()      // synchronous wait (ephemeral/lambda)
container.Destroy() // cleanup state directory
```

State is persisted under `/run/nexus/<id>/` in libcontainer's own JSON format.

---

## Namespaces

Each node (and lambda) gets its own:

| Namespace | Constant | Effect |
|-----------|----------|--------|
| PID | `configs.NEWPID` | Isolated process tree, PID 1 inside |
| Mount | `configs.NEWNS` | Isolated filesystem mounts |
| UTS | `configs.NEWUTS` | Isolated hostname |
| IPC | `configs.NEWIPC` | Isolated System V IPC / POSIX MQ |
| Network | `configs.NEWNET` | Isolated network stack |

No user namespace. No cgroup namespace. The container runs as `root` mapped to host `root` (UID 0).

---

## Cgroups v2

The daemon creates a parent cgroup at boot:

```
/sys/fs/cgroup/nexus/
```

Enabled controllers: `+cpu +memory +pids` written to `cgroup.subtree_control`.

For each node, a child cgroup is created:

```
/sys/fs/cgroup/nexus/<node-id>/
```

libcontainer is given the relative path `/nexus/<node-id>` and applies:

| Resource | Source field | cgroup file |
|----------|-------------|-------------|
| Memory limit | `NodeConfig.Memory * 1024 * 1024` | `memory.max` |
| CPU weight | `NodeConfig.CPUShares` | `cpu.weight` |

Lambda cgroups are created with `defer os.RemoveAll(nodeCgroupPath)` — cleaned up on return.

---

## Rootfs and mounts

All containers share the Alpine rootfs at `/var/lib/nexus/images/alpine-base` (read via pivot_root, not bind-mount). Each container gets its own mount namespace so writes inside do not affect the shared image.

Standard mounts applied to every container:

| Source | Destination | Type | Notes |
|--------|------------|------|-------|
| `proc` | `/proc` | proc | `NOEXEC,NOSUID,NODEV` |
| `sysfs` | `/sys` | sysfs | `RDONLY` |
| `tmpfs` | `/dev` | tmpfs | `mode=755` |
| `devpts` | `/dev/pts` | devpts | new instance |
| `tmpfs` | `/dev/shm` | tmpfs | 64 MB |

For persistent nodes, the loop device is added:

```go
&configs.Mount{
    Source:      loopDevice,   // e.g. /dev/loop3
    Destination: "/data",
    Device:      "ext4",
    Flags:       unix.MS_NOATIME,
}
```

For lambda, the script is bind-mounted read-only:

```go
&configs.Mount{
    Source:      hostScriptPath,  // /tmp/nexus/lambdas/<id>.py
    Destination: "/code/main.py",
    Device:      "bind",
    Flags:       unix.MS_BIND | unix.MS_RDONLY | unix.MS_REC,
}
```

---

## Container init re-exec

`main.go` checks `os.Args[1] == "init"` before Cobra runs. This is the libcontainer re-exec hook: when the container is created, the runtime forks the `nexus` binary itself with `nexus init` as the child. libcontainer intercepts this, reads the config pipe, sets up the filesystem (pivot_root), drops into the container, and execs the user command.

```go
func main() {
    if len(os.Args) > 1 && os.Args[1] == "init" {
        libcontainer.Init()
        return
    }
    cmd.Execute()
}
```

`_ "github.com/opencontainers/runc/libcontainer/nsenter"` — the `nsenter` package registers a `cgo` constructor that runs before `main()` and handles the namespace setup in the forked process.

---

## Long-running vs ephemeral

| Method | Persistence | Wait behaviour | Cleanup |
|--------|-------------|---------------|---------|
| `CreateAndStart` | Container state in `/run/nexus/` | Returns immediately after fork | Manual `Stop()` |
| `RunEphemeral` | No (state never written to disk beyond cgroup) | Synchronous `process.Wait()` | `defer container.Destroy()`, `defer os.RemoveAll(cgroupPath)` |
