# Getting Started

## System requirements

| Requirement | Notes |
|-------------|-------|
| Linux x86_64 | No other OS/arch supported |
| Kernel ≥ 5.x | cgroups v2 unified hierarchy required |
| Go ≥ 1.22 | `go.mod` declares `go 1.25.3` |
| Root access | Daemon requires root; CLI does not |
| `losetup` | Part of `util-linux` |
| `mkfs.ext4` | Part of `e2fsprogs` |
| `mount` / `umount` | Standard; must be in root's `PATH` |

```sh
# Debian/Ubuntu
apt install util-linux e2fsprogs
```

---

## Build

```sh
git clone https://github.com/MiltonJ23/Nexus.git
cd Nexus
go build -o nexus .
```

The binary is statically self-contained except for CGO dependencies pulled in by `mattn/go-sqlite3`. A CGO-enabled toolchain is required.

---

## Bootstrap the Alpine rootfs

Nexus nodes use an Alpine Linux minimal rootfs. The daemon hard-codes the path `/var/lib/nexus/images/alpine-base`. Prepare it once:

```sh
sudo mkdir -p /var/lib/nexus/images/alpine-base

# Download and extract Alpine minirootfs (adjust version as needed)
curl -Lo /tmp/alpine-minirootfs.tar.gz \
  https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.0-x86_64.tar.gz

sudo tar -xzf /tmp/alpine-minirootfs.tar.gz -C /var/lib/nexus/images/alpine-base
```

For Lambda (Python execution) the rootfs must include `python3`:

```sh
# Install python3 into the rootfs chroot
sudo chroot /var/lib/nexus/images/alpine-base /bin/sh -c \
  "apk update && apk add python3"
```

---

## First run

### 1. Start the daemon

The daemon manages all container, storage, and network operations. It **must** run as root.

```sh
sudo ./nexus daemon
```

Expected output:
```
Cgroup parent /sys/fs/cgroup/nexus ready
Configuration of the bridge : nexus0 -> Bridge nexus0 is already existing...
-> Nexus Daemon listening on /var/run/nexus.sock
📊 Metrics Collector started
```

The daemon blocks. Keep it running in a separate terminal or run it as a service.

### 2. Create your first node

```sh
./nexus node create mynode --mem 256 --cpu 512 --storage 1G
```

- `--mem` — memory limit in MB (enforced via cgroup `memory.max`)
- `--cpu` — CPU weight (cgroup `cpu.weight`, range 1–10000)
- `--storage` — volume size passed to `truncate -s`; accepts suffixes `K`, `M`, `G`

### 3. Upload a file

```sh
./nexus file upload /path/to/archive.tar.gz
```

The file is split into 10 MB chunks and distributed round-robin across all `Running` nodes.

### 4. Verify

```sh
./nexus file ls
```

---

## Running as a service (systemd)

```ini
# /etc/systemd/system/nexusd.service
[Unit]
Description=Nexus Cloud Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/nexus daemon
Restart=on-failure
User=root

[Install]
WantedBy=multi-user.target
```

```sh
sudo cp nexus /usr/local/bin/nexus
sudo systemctl daemon-reload
sudo systemctl enable --now nexusd
```

---

## Directory layout

```
/var/lib/nexus/
├── nexus.json              state file
├── auth.db                 SQLite auth database
├── images/
│   └── alpine-base/        shared rootfs (read-only by containers)
└── volumes/
    ├── mynode.img           node disk image (ext4)
    └── ...

/run/nexus/
└── mynode/                 libcontainer state for container "mynode"

/tmp/nexus/
└── mounts/
    └── mynode/             temporary mount point during chunk I/O

/var/run/nexus.sock         gRPC Unix socket
```
