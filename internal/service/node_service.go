package service

import (
	"Nexus/internal/adapters/network"
	"Nexus/internal/adapters/runtime"
	"Nexus/internal/adapters/storage"
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"Nexus/internal/state"
	"fmt"
)

// NodeService is the kind of mastermind of the orchestration of the nodes
type NodeService struct {
	runtime ports.ContainerRuntime
	network ports.NetworkManager
	storage ports.StorageManager
	state   *state.StateManager
}

// NewNodeService is the method that will allow us to create from the logic metier a new container
func NewNodeService() (*NodeService, error) {
	// Here, we are going to instantiate the runtime adapter
	rt, runtimeInitializationError := runtime.NewLibContainerRuntime()
	if runtimeInitializationError != nil {
		return nil, fmt.Errorf("unable to initialize libcontainer runtime %v", runtimeInitializationError.Error())
	}
	nm := network.NewNetlinkManager() // creating an instance of the network adapter
	// now let's make sure the bridge is ready before the first node
	sm := storage.NewLoopStorageManager()

	st := state.GlobalState
	LoadingErr := st.Load()
	if LoadingErr != nil {
		return nil, fmt.Errorf("unable to load the state of the application %v", LoadingErr.Error())
	}
	settingInitBridge := nm.SetupBridge()
	if settingInitBridge != nil {
		return nil, fmt.Errorf("unable to initialize the bridge  ")
	}
	return &NodeService{runtime: rt, network: nm, storage: sm, state: st}, nil
}

// CreateNode is the application logic to create a new node
func (s *NodeService) CreateNode(name string, mem int64, cpuShare uint64, storageSize string) (*core.NodeState, error) {
	// Let's perform a little validation
	if name == "" {
		return nil, fmt.Errorf("the node must have a name")
	}

	//Let's configure the network for the IP Allocation procedure. We must provide the IP before starting the container
	ip, assignIPError := s.network.AssignIP(name)
	if assignIPError != nil {
		return nil, fmt.Errorf("unable to assign the IP address %v", assignIPError.Error())
	}

	// let's deal with the storage
	// First of all, let's create the storage file
	volPath, volumeCreationError := s.storage.CreateVolume(name, storageSize)
	if volumeCreationError != nil {
		return nil, fmt.Errorf("unable to create volume %v", volumeCreationError.Error())
	}
	// let's try to attach the created volume
	loopDevice, mountingError := s.storage.AttachVolume(volPath)
	if mountingError != nil {
		return nil, fmt.Errorf("an error occured while mounting the loop device : %v", mountingError.Error())
	}

	// let's now create the configuration to be launched

	conf := core.NodeConfig{
		ID:         name,
		Hostname:   name,
		RootfsPath: "/var/lib/nexus/images/alpine-base",
		Memory:     mem,
		VolumePath: loopDevice,
		CPUShares:  cpuShare,
		Command:    []string{"/bin/sh", "-c", "sleep 3600"}, // our process
	}

	// Now let's call the adapter by using the interface
	nodeState, ContainerLaunchError := s.runtime.CreateAndStart(conf)
	if ContainerLaunchError != nil {
		return nil, ContainerLaunchError
	}

	// 5. Network Setup
	nodeState.IP = ip
	NetworkConfigurationError := s.network.SetupContainerNetwork(nodeState.ID, nodeState.PID, ip)
	if NetworkConfigurationError != nil {
		fmt.Printf("Warning: Network setup failed: %v\n", NetworkConfigurationError)
	}

	// 6. PERSISTANCE (Le chaînon manquant !)
	// On enregistre le noeud dans le JSON pour que 'file upload' le trouve plus tard
	s.state.State.Node[nodeState.ID] = *nodeState
	if err := s.state.Save(); err != nil {
		fmt.Printf("Warning: Failed to save state: %v\n", err)
	}

	//TODO: on top of this, I will need a persistence logic to keep the node state in the hard
	return nodeState, nil
}
