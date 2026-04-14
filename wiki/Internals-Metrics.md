# Internals: Metrics

Packages: `internal/adapters/metrics/cgroup_scraper.go`, `internal/service/metrics_service.go`

---

## Collection pipeline

```
MetricsService.Start(5s)
    └── ticker fires every 5 s
        └── collectAll()
            └── for each Running node:
                Scraper.Collect(nodeID)
                    ├── read /sys/fs/cgroup/memory/nexus/<id>/memory.usage_in_bytes
                    ├── read /sys/fs/cgroup/memory/nexus/<id>/memory.limit_in_bytes
                    └── read /sys/fs/cgroup/cpu/nexus/<id>/cpuacct.usage
                compute CPU % delta
                state.AddNode(node with updated LatestMetrics)
    // No state.Save() — metrics are RAM-only
```

---

## Scraper

`Scraper` holds per-node state for CPU delta computation:

```go
type Scraper struct {
    prevCpu  map[string]int64
    prevTime map[string]time.Time
}
```

### Memory

Read from cgroup v1-style paths (under `/sys/fs/cgroup/memory/`):

```
memory.usage_in_bytes   — current memory used in bytes
memory.limit_in_bytes   — configured hard limit
```

Both are single-integer text files. `readInt64(path)` reads, trims whitespace, and parses as base-10 int64.

### CPU

Read from:

```
/sys/fs/cgroup/cpu/nexus/<id>/cpuacct.usage
```

This is a monotonically increasing counter of CPU time consumed by the cgroup in **nanoseconds**.

CPU percentage is computed as a rate between two consecutive samples:

```
cpuPercent = (Δcpu_nanos / Δwall_nanos) * 100
```

This gives a value relative to one logical CPU core. A value of 100.0 means one full core is saturated.

The first sample always produces `cpuPercent = 0.0` (no previous value).

---

## NodeMetrics type

```go
type NodeMetrics struct {
    NodeID        string
    Timestamp     time.Time
    MemoryUsage   int64    // bytes
    MemoryLimit   int64    // bytes
    CPUUsageTotal int64    // total nanoseconds since container start
    CPUPercent    float64  // instantaneous % relative to 1 core
}
```

Stored in `NodeState.LatestMetrics`. Overwritten on each collection cycle.

---

## Design decisions

**Metrics are not persisted to disk.** Writing every 5 seconds would cause excessive I/O. On daemon restart, metrics start fresh.

**No time-series.** Only the latest sample is kept. A future improvement would push metrics to a TSDB (e.g. Prometheus).

**Failure handling.** If a cgroup file is missing (node stopped between check and collection), `Collect` returns an error and the node is silently skipped.

---

## Cgroup path note

The scraper uses the **cgroup v1** path layout (`/sys/fs/cgroup/memory/...`, `/sys/fs/cgroup/cpu/...`), while the runtime adapter creates cgroups under the **cgroup v2** unified hierarchy (`/sys/fs/cgroup/nexus/`). On kernels running in hybrid mode (v1 + v2), both paths may coexist. On pure cgroup v2 kernels the v1 paths do not exist and the scraper will fail silently for every node. This is a known inconsistency to be resolved.
