package runtime

import (
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Those are the constant pointing to the state of the nodes and the Cgroups

const (
	StatePath    = "/run/nexus" // folder where to find the state of the container
	Cgroup       = "/nexus"     // this is the name of the parent cgroup, we need to precise this to isolate our containers from one another
	CgroupFsPath = "/sys/fs/cgroup/nexus"
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
		return nil, fmt.Errorf("an error occured when creating the parent cgroup %s : %w", CgroupFsPath, ParentCgroupError)
	}
	return &LibContainerRuntime{RootStatePath: StatePath}, nil
}

func createCgroupParent() error {
	// 1. Création du dossier physique
	if err := os.MkdirAll(CgroupFsPath, 0755); err != nil {
		return err
	}

	// 2. Activation des contrôleurs pour Cgroup V2 (Ubuntu)
	// Le fichier qui contrôle ça est cgroup.subtree_control
	subtreeControl := path.Join(CgroupFsPath, "cgroup.subtree_control")

	// On essaie d'activer les contrôleurs essentiels UN PAR UN.
	// Comme ça, si le CPU est bloqué par le système, la Mémoire fonctionnera quand même.
	// +pids est souvent requis pour pouvoir ajouter des processus.
	controllers := []string{"+cpu", "+memory", "+pids"}

	for _, ctrl := range controllers {
		// On ignore l'erreur volontairement ici (Best Effort), mais on le fait séparément
		// pour maximiser les chances de succès.
		_ = os.WriteFile(subtreeControl, []byte(ctrl), 0644)
	}

	// Vérification visuelle pour le debug (affiché dans la console)
	fmt.Println("✅ Cgroup parent /sys/fs/cgroup/nexus prêt (Controllers activés)")

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
	// 1. Manual Cgroup Creation (Your existing logic)
	nodeCgroupPath := path.Join(CgroupFsPath, conf.ID)
	if err := os.MkdirAll(nodeCgroupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to manually create node cgroup %s: %w", nodeCgroupPath, err)
	}

	// 2. Define Default Mounts (Moved OUTSIDE the config struct)
	// We do this here so we can dynamically append the volume mount later
	mounts := []*configs.Mount{
		{Source: "proc", Destination: "/proc", Device: "proc", Flags: syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV},
		{Source: "sysfs", Destination: "/sys", Device: "sysfs", Flags: syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV},
		{Source: "tmpfs", Destination: "/dev", Device: "tmpfs", Flags: syscall.MS_NOSUID | syscall.MS_STRICTATIME, Data: "mode=755"},
		{Source: "devpts", Destination: "/dev/pts", Device: "devpts", Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC},
		{
			Source:      "shm",
			Destination: "/dev/shm",
			Device:      "tmpfs",
			Flags:       syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
		},
	}

	// 3. NEW LOGIC: Append Volume Mount if it exists
	if conf.VolumePath != "" {
		mounts = append(mounts, &configs.Mount{
			Source:      conf.VolumePath,    // The loop device (e.g., /dev/loop4)
			Destination: "/data",            // Where it appears inside the container
			Device:      "ext4",             // File system type
			Flags:       syscall.MS_NOATIME, // No Access Time (optimization)
			Data:        "",
		})
		fmt.Printf("    -> Volume detected: Mounting %s to /data\n", conf.VolumePath)
	}

	//lets define the isolation contract by setting up the container NameSpaces
	config := &configs.Config{
		Rootfs: conf.RootfsPath,
		Namespaces: configs.Namespaces{
			{Type: configs.NEWPID},
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWNET},
			{Type: configs.NEWCGROUP},
		},

		Cgroups: &cgroups.Cgroup{
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
		Mounts:   mounts,
	}

	// 6. Container Creation
	container, err := libcontainer.Create(r.RootStatePath, conf.ID, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create the container libcontaire for  %s: %w", conf.ID, err)
	}

	// Let's clearly define the process insisde of the ccontainer
	process := &libcontainer.Process{
		Args:   conf.Command,
		Env:    []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin"},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Then we launch the container
	if err := container.Run(process); err != nil {
		if destroyErr := container.Destroy(); destroyErr != nil {
			return nil, fmt.Errorf("failed to run  %s: %w; also failed the cleanup: %v", conf.ID, err, destroyErr)
		}
		return nil, fmt.Errorf("failed to run for  %s: %w", conf.ID, err)
	}

	// Now let's Fetch the container PID
	hostPID, _ := process.Pid()

	nodeState := &core.NodeState{
		NodeConfig: conf,
		PID:        hostPID,
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
