package runtime

import (
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"golang.org/x/sys/unix"
)

const (
	StatePath    = "/run/nexus"
	Cgroup       = "/nexus" // Changed: Remove leading slash
	CgroupFsPath = "/sys/fs/cgroup/nexus"
)

type LibContainerRuntime struct {
	RootStatePath string
}

func NewLibContainerRuntime() (ports.ContainerRuntime, error) {
	err := os.MkdirAll(StatePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create state folder %s: %w", StatePath, err)
	}

	ParentCgroupError := createCgroupParent()
	if ParentCgroupError != nil {
		return nil, fmt.Errorf("failed to create parent cgroup %s: %w", CgroupFsPath, ParentCgroupError)
	}

	return &LibContainerRuntime{
		RootStatePath: StatePath,
	}, nil
}

func createCgroupParent() error {
	CgroupFolderCreationerr := os.MkdirAll(CgroupFsPath, 0755)
	if CgroupFolderCreationerr != nil {
		return CgroupFolderCreationerr
	}

	subtreeControl := path.Join(CgroupFsPath, "cgroup.subtree_control")
	controllers := []string{"+cpu", "+memory", "+pids"}

	for _, ctrl := range controllers {
		if err := os.WriteFile(subtreeControl, []byte(ctrl), 0644); err != nil {
			fmt.Printf("⚠️  Warning: failed to enable controller %s: %v\n", ctrl, err)
		}
	}

	fmt.Println("Cgroup parent /sys/fs/cgroup/nexus ready")
	return nil
}

func (r *LibContainerRuntime) CreateAndStart(conf core.NodeConfig) (*core.NodeState, error) {
	// let's ensure the correct rootfs if being used  | we check first if the conf.RootfsPath is empty
	if conf.RootfsPath == "" {
		return nil, fmt.Errorf("rootfs path cannot be empty")
	}

	foundObject, folderFindingerr := os.Stat(conf.RootfsPath) // We have to ensure the conf.RootfsPath path is legit | Test to see if the returned object is a folder or not
	if folderFindingerr != nil {
		return nil, fmt.Errorf("rootfs %s does not exist: %w", conf.RootfsPath, folderFindingerr)
	} else if !foundObject.IsDir() {
		return nil, fmt.Errorf("rootfs %s is not a directory", conf.RootfsPath)
	}

	if _, err := os.Stat(path.Join(conf.RootfsPath, "bin")); os.IsNotExist(err) {
		return nil, fmt.Errorf("invalid rootfs at %s: missing /bin directory", conf.RootfsPath)
	}

	fmt.Printf("Using rootfs: %s\n", conf.RootfsPath)

	// Now let's create the cgroup manually
	nodeCgroupPath := filepath.Join(CgroupFsPath, conf.ID)
	if err := os.MkdirAll(nodeCgroupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create node cgroup directory %s: %w", nodeCgroupPath, err)
	}
	fmt.Printf("-> Node cgroup created: %s\n", nodeCgroupPath)

	defaultMountFlags := unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV
	//  Build mount list
	mounts := []*configs.Mount{
		{
			Source:      "proc",
			Destination: "/proc",
			Device:      "proc",
			Flags:       defaultMountFlags,
		},
		{
			Source:      "sysfs",
			Destination: "/sys",
			Device:      "sysfs",
			Flags:       defaultMountFlags | unix.MS_RDONLY,
		},
		{
			Source:      "tmpfs",
			Destination: "/dev",
			Device:      "tmpfs",
			Flags:       unix.MS_NOSUID | unix.MS_STRICTATIME,
			Data:        "mode=755",
		},
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
			Flags:       unix.MS_NOSUID | unix.MS_NOEXEC,
			Data:        "newinstance,ptmxmode=0666,mode=0620",
		},
		{
			Source:      "shm",
			Destination: "/dev/shm",
			Device:      "tmpfs",
			Flags:       unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			Data:        "mode=1777,size=65536k",
		},
	}

	if conf.VolumePath != "" {
		mounts = append(mounts, &configs.Mount{
			Source:      conf.VolumePath,
			Destination: "/data",
			Device:      "ext4",
			Flags:       unix.MS_NOATIME,
		})
		fmt.Printf("    -> Volume mount: %s -> /data\n", conf.VolumePath)
	}

	memoryLimit := conf.Memory * 1024 * 1024

	// CRITICAL: Use ABSOLUTE path starting with "/" to avoid user.slice nesting
	relativeCgroupPath := filepath.Join(Cgroup, conf.ID)
	fmt.Printf("-> Cgroup path: %s\n", relativeCgroupPath)

	//  Create container config
	config := &configs.Config{
		Rootfs:          conf.RootfsPath,
		RootPropagation: syscall.MS_PRIVATE | syscall.MS_REC,
		NoPivotRoot:     false,
		Capabilities: &configs.Capabilities{
			Bounding: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
				"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
				"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",
			},
			Effective: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
				"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
				"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",
			},
			Inheritable: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
				"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
				"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",
			},
			Permitted: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
				"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
				"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",
			},
		},
		Namespaces: configs.Namespaces{
			{Type: configs.NEWPID},
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWNET},
		},
		Cgroups: &cgroups.Cgroup{
			// CRITICAL FIX: Use relative path, let libcontainer create the subdirectory
			Path:        relativeCgroupPath,
			ScopePrefix: "",
			Resources: &cgroups.Resources{
				Memory:    memoryLimit,
				CpuShares: conf.CPUShares,
			},
		},
		Devices:  specconv.AllowedDevices,
		Hostname: conf.Hostname,
		Mounts:   mounts,
	}

	// 5. Create container using libcontainer.Create
	// This will automatically create the cgroup subdirectory
	container, containerCreationerr := libcontainer.Create(r.RootStatePath, conf.ID, config)
	if containerCreationerr != nil {
		return nil, fmt.Errorf("failed to create container %s: %w", conf.ID, containerCreationerr)
	}

	fmt.Printf("	-> Container created\n")

	// 6. Start the container
	process := &libcontainer.Process{
		Args:   conf.Command,
		Env:    []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin", "HOME=/root"},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Init:   true,
	}

	if err := container.Run(process); err != nil {
		container.Destroy()
		return nil, fmt.Errorf("failed to run %s: %w", conf.ID, err)
	}

	hostPID, _ := process.Pid()

	nodeState := &core.NodeState{
		NodeConfig: conf,
		PID:        hostPID,
		Status:     "Running",
	}

	fmt.Printf(" Container %s running. PID: %d\n", conf.ID, hostPID)
	return nodeState, nil
}

func (r *LibContainerRuntime) Stop(id string) error {
	return nil
}

func (r *LibContainerRuntime) GetState(id string) (*core.NodeState, error) {
	return nil, nil
}

func (r *LibContainerRuntime) RunEphemeral(conf core.NodeConfig, stdout, stderr io.Writer) (int, error) {
	// 1. Validation (Same as CreateAndStart)
	if conf.RootfsPath == "" {
		return -1, fmt.Errorf("rootfs path cannot be empty")
	}

	// 2. Manual Cgroup Creation (CRITICAL: Matches your working CreateAndStart logic)
	// We must create the physical directory before libcontainer tries to write to it
	nodeCgroupPath := filepath.Join(CgroupFsPath, conf.ID)
	if err := os.MkdirAll(nodeCgroupPath, 0755); err != nil {
		return -1, fmt.Errorf("failed to create node cgroup directory %s: %w", nodeCgroupPath, err)
	}
	// We will delete this folder when the function exits
	defer os.RemoveAll(nodeCgroupPath)

	// 3. Mounts Configuration
	defaultMountFlags := unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV
	mounts := []*configs.Mount{
		{Source: "proc", Destination: "/proc", Device: "proc", Flags: defaultMountFlags},
		{Source: "sysfs", Destination: "/sys", Device: "sysfs", Flags: defaultMountFlags | unix.MS_RDONLY},
		{Source: "tmpfs", Destination: "/dev", Device: "tmpfs", Flags: unix.MS_NOSUID | unix.MS_STRICTATIME, Data: "mode=755"},
		{Source: "devpts", Destination: "/dev/pts", Device: "devpts", Flags: unix.MS_NOSUID | unix.MS_NOEXEC, Data: "newinstance,ptmxmode=0666,mode=0620"},
		{Source: "shm", Destination: "/dev/shm", Device: "tmpfs", Flags: unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV, Data: "mode=1777,size=65536k"},
	}

	// 4. Code Injection (Specific to Ephemeral/Lambda)
	// Instead of mounting a disk to /data, we bind mount the script file
	if conf.VolumePath != "" {
		mounts = append(mounts, &configs.Mount{
			Source:      conf.VolumePath, // e.g., /tmp/script.py on host
			Destination: "/code/main.py", // Where the container expects code
			Device:      "bind",
			// MS_BIND is required for files, MS_RDONLY for security
			Flags: unix.MS_BIND | unix.MS_RDONLY | unix.MS_REC,
		})
	}

	// 5. Config
	relativeCgroupPath := filepath.Join(Cgroup, conf.ID) // "/nexus/lambda-id"

	config := &configs.Config{
		Rootfs:          conf.RootfsPath,
		RootPropagation: syscall.MS_PRIVATE | syscall.MS_REC,
		NoPivotRoot:     false, // Matches your CreateAndStart
		Capabilities: &configs.Capabilities{
			// Minimal capabilities for ephemeral workloads (safer)
			Bounding:    []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
			Effective:   []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
			Inheritable: []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
			Permitted:   []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
		},
		Namespaces: configs.Namespaces{
			{Type: configs.NEWPID},
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			// Network is often optional for Lambda, but we keep it consistent
			{Type: configs.NEWNET},
		},
		Cgroups: &cgroups.Cgroup{
			Path:        relativeCgroupPath,
			ScopePrefix: "",
			Resources: &cgroups.Resources{
				Memory:    conf.Memory * 1024 * 1024,
				CpuShares: conf.CPUShares,
			},
		},
		Devices:  specconv.AllowedDevices,
		Hostname: "lambda-" + conf.ID, // Distinct hostname
		Mounts:   mounts,
	}

	// 6. Create Factory
	container, err := libcontainer.Create(r.RootStatePath, conf.ID, config)
	if err != nil {
		return -1, fmt.Errorf("create error: %w", err)
	}
	// CRITICAL: Cleanup container metadata on exit
	defer container.Destroy()

	// 7. Define Process
	process := &libcontainer.Process{
		Args:   conf.Command, // e.g. ["python3", "/code/main.py"]
		Env:    []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin", "PYTHONUNBUFFERED=1"},
		Stdin:  nil,
		Stdout: stdout, // Pipe output to the provided writer
		Stderr: stderr, // Pipe errors to the provided writer
		Init:   true,
	}

	// 8. Run
	if err := container.Run(process); err != nil {
		return -1, fmt.Errorf("run error: %w", err)
	}

	// 9. Wait for completion (Synchronous)
	state, err := process.Wait()
	if err != nil {
		return -1, fmt.Errorf("wait error: %w", err)
	}

	// 10. Return Exit Code
	// State.ExitCode() returns the int code (0 = success)
	exitCode := 0
	if state != nil {
		// Use reflection or type assertion if necessary,
		// but standard libcontainer state has ExitCode()
		exitCode = state.ExitCode()
	}

	return exitCode, nil
}
