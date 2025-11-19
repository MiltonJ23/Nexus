package runtime

import (
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Those are the constant pointing to the state of the nodes and the Cgroups

const (
	StatePath = "/run/nexus" // folder where to find the state of the container
	Cgroup    = "nexus"      // this is the name of the parent cgroup, we need to precise this to isolate our containers from one another
)

// LibContainerRuntime will be the root path of the state
type LibContainerRuntime struct {
	RootStatePath string
}

// NewLibContainerRuntime initialize the factory and create the Cgroup
func NewLibContainerRuntime() (ports.ContainerRuntime, error) {
	// Let's make sure the folder for node state exist , setting the permission to 0755 ensure only root can read ans write
	err := os.MkdirAll(StatePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("an error occured when tried to create a state folder %s : %w", StatePath, err)
	}
	// Let's create the parent Cgroup that will allow us to monitor the global state of all of our nodes
	ParentCgroupError := createCgroupParent()
	if ParentCgroupError != nil {
		return nil, fmt.Errorf("an error occured when creating the parent cgroup %s : %w", StatePath, ParentCgroupError)
	}
	return &LibContainerRuntime{RootStatePath: StatePath}, nil
}

func createCgroupParent() error {
	// Let's create the parent Cgroup from which we are going to latter create the containers
	cgroupConfig := &cgroups.Cgroup{
		Path: Cgroup,
	}

	config := &configs.Config{
		Cgroups: cgroupConfig,
		Rootfs:  "/var/lib/nexus/images/alpine-base",
	}

	// We will have to manually remove the state folder in the container directory
	// Let's define a tempory id
	tempID := "init-parent-cgroup" // the temporary ID of the container
	tempDir := filepath.Join(StatePath, tempID)

	// Let's create the container
	container, CreateContainerError := libcontainer.Create(StatePath, tempID, config)
	if CreateContainerError != nil {
		return fmt.Errorf("an error occured when tried to create a new container %s : %w", StatePath, CreateContainerError)
	}

	// We will use defer to make sure the state folder get destroyed but ensure we don't call container.Destroy()
	defer func() {
		removeError := os.RemoveAll(tempDir)
		if removeError != nil {
			fmt.Errorf("an error occured when tried to remove the state folder from the container folder")
		}
	}() // ensure this is a function call

	//Let's force the creation of cgroup on the disk using the RUN()
	process := &libcontainer.Process{
		Args: []string{"/bin/true"},
		Env:  []string{"PATH=/bin:/usr/bin"},
	}

	runningError := container.Run(process)
	if runningError != nil {
		defer container.Destroy()
		fmt.Errorf("an error occured when tried to run the container %s : %w", tempID, runningError)
	}
	return nil
}

/*func (r *LibContainerRuntime) CreateAndStart(conf core.NodeConfig) (*core.NodeState, error) {

	// Let's define the isolation contract (Given the OCI standard)
	config := &configs.Config{
		Rootfs: r.RootStatePath,
		// Let's define the isolation with a new pid , filesystem , net configuration
		Namespaces: configs.Namespaces{
			{Type: configs.NEWPID},
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWNET},
		}}
	// Now let's configure the Cgroups
	Cgroups := &cgroups.Cgroup{
		Path: path.Join(StatePath, Cgroup),
		Resources: &cgroups.Resources{
			Memory:    conf.Memory * 1024 * 1024,
			CpuShares: conf.CPUShares,
		},
	},
	Capabilities := &configs.Capabilities{
		Bounding : []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
			"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
			"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",},
	},
}
*/

func (r *LibContainerRuntime) CreateAndStart(conf core.NodeConfig) (*core.NodeState, error) {
	// Here we are going to define the isolation contract
	config := &configs.Config{
		Rootfs: r.RootStatePath, // We use the state path as the root
		Namespaces: configs.Namespaces{
			{Type: configs.NEWPID},
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWNET},
		},

		Cgroups: &cgroups.Cgroup{
			// This is the location where the container will be created in the parent folder
			Path: path.Join(Cgroup, conf.ID),
			Resources: &cgroups.Resources{
				Memory:    conf.Memory * 1024 * 1024,
				CpuShares: conf.CPUShares,
			},
		},

		Capabilities: &configs.Capabilities{
			Bounding: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER",
				"CAP_MKNOD", "CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID",
				"CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL",
			},
		},

		Hostname: conf.Hostname,

		// Mounting points of the container , those are unchanged
		Mounts: []*configs.Mount{
			{Source: "proc", Destination: "/proc", Device: "proc", Flags: syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV},
			{Source: "sysfs", Destination: "/sys", Device: "sysfs", Flags: syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV},
			{Source: "tmpfs", Destination: "/dev", Device: "tmpfs", Flags: syscall.MS_NOSUID | syscall.MS_STRICTATIME, Data: "mode=755"},
			{Source: "devpts", Destination: "/dev/pts", Device: "devpts", Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC},
		},
	}

	//The libcontainer.Create method use our r.RootStatePath to store the state of the container

	container, err := libcontainer.Create(r.RootStatePath, conf.ID, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create the container libcontaire for  %s: %w", conf.ID, err)
	}

	// Here let's define the process that will be trapped inside of our container
	process := &libcontainer.Process{
		Args:   conf.Command,
		Env:    []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin"},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Now we launch the thing
	if err := container.Run(process); err != nil {
		// let's clean in case of a critical failure
		if destroyErr := container.Destroy(); destroyErr != nil {
			return nil, fmt.Errorf("failed to run  %s: %w; also failed the cleanup: %v", conf.ID, err, destroyErr)
		}
		return nil, fmt.Errorf("failed to run for  %s: %w", conf.ID, err)
	}

	// NOw let's fetch the state of the container using container.Create | more precisely , let's fetch the PID
	state, err := container.State()
	if err != nil {
		container.Destroy() // Let's clean up in case of a failure | however i didn't think handling this error would be necessary | We will see
		return nil, fmt.Errorf("unable to read the state of the containre %s: %w", conf.ID, err)
	}

	nodeState := &core.NodeState{
		NodeConfig: conf,
		PID:        state.InitProcessPid, // This is the PID of the running container
		Status:     "Running",
	}

	fmt.Printf(" Container %s launched. PID of the running host : %d\n", conf.ID, nodeState.PID)
	return nodeState, nil
}

func (r *LibContainerRuntime) Stop(id string) error {
	//TODO : The real implementation to stop a running container
	return nil
}
func (r *LibContainerRuntime) GetState(id string) (*core.NodeState, error) {
	//TODO: The implementation to get the state of a running container
	return nil, nil
}
