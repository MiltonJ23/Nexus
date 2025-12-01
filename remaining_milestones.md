##  1. The Roadmap (Milestone 4 to Full Platform)
   This is how we align your immediate work with the "Big Picture" requests.

## Milestone 4 (Sunday - TODAY): The Storage Engine

Implement: Upload, Download, Delete, Info in the CLI.

Logic: Chunking + Round Robin distribution.

Tech: mount -o loop to inject data into node disks.

Why: This proves the storage logic works before we wrap it in an API.

## Milestone 5 (Mon/Tue): The Daemon (cloudsimd)

Shift: Stop running the CLI as root.

Build: A long-running Go process (cloudsimd) that runs as root. It holds the State and manages libcontainer.

Interface: It exposes a gRPC Socket (Unix Socket).

Client: Rewrite cmd/ to be a lightweight gRPC Client that sends commands to cloudsimd.

## Milestone 6 (Wed): Identity & Backend

Identity: Implement a gRPC service for User Auth (JWT).

Backend: Create a Gin server that connects to cloudsimd via gRPC. It exposes HTTP endpoints (POST /upload, GET /nodes).

Integration: The Next.js UI will talk to this Gin Backend.

## Milestone 7 (Thu): Compute (SSH Instances)

Image: Build a custom Alpine image with openssh-server installed and keys configured.

Networking: Port Forwarding. We need to map Host:2222 -> Container:22.

Access: User clicks "Connect" in UI -> Gets SSH string.

7. Immediate Action Plan (Milestone 4)
   We need to finish the Storage Logic today.

Refactor NodeConfig: Ensure nodes have a defined volume path in the State.

Implement LoopStorageManager methods: MountOnHost, UnmountOnHost, WriteChunk.

Implement FileService: The chunking logic.

CLI Commands:

cloudsim file upload <path>

cloudsim file download <id> <dest>

cloudsim file delete <id>

cloudsim file ls

Pro Tip: Be very careful with MountOnHost. If the container is also writing to /data while the host has it mounted, you might corrupt the filesystem. For now (Simulated Cloud), assume only one writer at a time. In a real kernel scenario, you cannot mount the same ext4 filesystem twice Read/Write.

Beast Mode Fix: If the container is running, use nsenter to write the file (using cat). If the container is stopped, use mount.