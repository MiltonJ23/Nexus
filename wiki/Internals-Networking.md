# Internals: Networking

Packages: `internal/adapters/network`, `internal/ports/network.go`

---

## Interface

```go
type NetworkManager interface {
    SetupBridge() error
    AssignIP(id string) (core.IPAddress, error)
    SetupContainerNetwork(nodeID string, nodePID int, ip core.IPAddress) error
}
```

Implemented by `NetlinkManager` using the `github.com/vishvananda/netlink` library (a Go wrapper around Linux's netlink socket API).

---

## Network topology

```
Host
├── nexus0 (bridge)  10.0.42.1/24
│   ├── nex-worker1  (veth host end)
│   ├── nex-worker2
│   └── ...
│
└── (veth guest ends are inside container network namespaces)
    worker1 netns: eth0 @ 10.0.42.2/24,  gw 10.0.42.1
    worker2 netns: eth0 @ 10.0.42.3/24,  gw 10.0.42.1
```

All containers are on `10.0.42.0/24`. The bridge (`10.0.42.1`) acts as the default gateway. Containers can reach each other and the host. Outbound internet requires NAT (not configured automatically — add `iptables -t nat -A POSTROUTING -s 10.0.42.0/24 -j MASQUERADE` if needed).

---

## SetupBridge

Called once at daemon startup (NodeService init).

1. `netlink.LinkByName("nexus0")` — if the bridge already exists, return immediately (idempotent).
2. `netlink.LinkAdd(&netlink.Bridge{…})` — create the bridge.
3. `netlink.AddrAdd(bridge, "10.0.42.1/24")` — assign the gateway IP.
4. `netlink.LinkSetUp(bridge)` — bring the bridge up.

---

## IPAM (SimpleIPAM)

Sequential allocation from `10.0.42.2` to `10.0.42.254`.

On each `AssignIP(nodeID)` call:

1. Reads `state.GlobalState.State.Node` to find all IPs already in use.
2. Scans `10.0.42.2` → `10.0.42.254`, skipping used IPs.
3. Returns the first free `IPAddress{IP, Subnet: "10.0.42.0/24", Gateway: "10.0.42.1"}`.

No IP is persisted as "assigned" until the node state is saved — the scan is done against the live state on every call. The mutex (`SimpleIPAM.mu`) serialises concurrent allocation attempts.

Maximum nodes per daemon instance: **253** (addresses .2 through .254).

---

## SetupContainerNetwork

Called after the container process is started (host PID known).

This is a delicate multi-namespace operation. The OS thread must be locked for the duration because Go goroutines can be moved between OS threads, and `netns.Set()` is per-thread.

### Step-by-step

```go
runtime.LockOSThread()
defer runtime.UnlockOSThread()
```

**1. Save host network namespace**
```go
hostNS, _ := netns.Get()
defer hostNS.Close()
```

**2. Create veth pair on the host**
```go
netlink.LinkAdd(&netlink.Veth{
    LinkAttrs: netlink.LinkAttrs{Name: "nex-<nodeID>"},
    PeerName:  "eth0",
})
```
Both ends are created in the host namespace initially.

**3. Attach host end to bridge**
```go
netlink.LinkSetMaster(veth, bridge)
netlink.LinkSetUp(veth)
```

**4. Move guest end into container namespace**
```go
peer, _ := netlink.LinkByName("eth0")
netlink.LinkSetNsPid(peer, nodePID)
```
`LinkSetNsPid` uses `NETLINK_ROUTE / RTM_SETLINK` with `IFLA_NET_NS_PID` to atomically move the interface into the target PID's network namespace.

**5. Enter container network namespace**
```go
targetNS, _ := netns.GetFromPid(nodePID)
netns.Set(targetNS)
```
From this point all netlink calls operate inside the container.

**6. Bring up loopback**
```go
lo, _ := netlink.LinkByName("lo")
netlink.LinkSetUp(lo)
```

**7. Bring up eth0 and assign IP**
```go
eth0, _ := netlink.LinkByName("eth0")
netlink.LinkSetUp(eth0)
netlink.AddrAdd(eth0, "<ip>/24")
```

**8. Add default gateway route**
```go
netlink.RouteAdd(&netlink.Route{
    LinkIndex: eth0.Attrs().Index,
    Gw:        net.ParseIP("10.0.42.1"),
})
```

**9. Return to host namespace**
```go
netns.Set(hostNS)
```

---

## Naming conventions

| Interface | Location | Name pattern |
|-----------|----------|-------------|
| Bridge | host | `nexus0` |
| veth host end | host | `nex-<nodeID>` (truncated if >15 chars) |
| veth guest end | container netns | `eth0` |

> **Note:** Linux interface names are limited to 15 characters (`IFNAMSIZ - 1`). Node IDs longer than 11 characters will cause `nex-<nodeID>` to exceed this limit and `LinkAdd` will fail.
