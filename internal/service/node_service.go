package service

import (
	"Nexus/internal/adapters/network"
	"Nexus/internal/adapters/runtime"
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"fmt"
)

// NodeService is the kind of mastermind of the orchestration of the nodes
type NodeService struct {
	runtime ports.ContainerRuntime
	network ports.NetworkManager
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
	settingInitBridge := nm.SetupBridge()
	if settingInitBridge != nil {
		return nil, fmt.Errorf("unable to initialize the bridge  ")
	}
	return &NodeService{runtime: rt, network: nm}, nil
}

// CreateNode is the application logic to create a new node
func (s *NodeService) CreateNode(name string, mem int64, cpuShare uint64) (*core.NodeState, error) {
	// Let's perform a little validation
	if name == "" {
		return nil, fmt.Errorf("the node must have a name")
	}

	//Let's configure the network for the IP Allocation procedure. We must provide the IP before starting the container
	ip, assignIPError := s.network.AssignIP(name)
	if assignIPError != nil {
		return nil, fmt.Errorf("unable to assign the IP address %v", assignIPError.Error())
	}

	// let's now create the configuration to be launched
	rootfsPath := "/var/lib/nexus/images/alpine-base" // this the location of the root filesystem
	conf := core.NodeConfig{
		ID:         name,
		Hostname:   name,
		RootfsPath: rootfsPath,
		Memory:     mem,
		CPUShares:  cpuShare,
		Command:    []string{"/bin/sh", "-c", "sleep 3600"}, // our process
	}

	// Now let's call the adapter by using the interface
	state, err := s.runtime.CreateAndStart(conf)
	if err != nil {
		//TODO: we will implement the releasing of the assigned ip address when the container fails
		return nil, fmt.Errorf("unable to create a new node %v", err.Error())
	}
	//now let's perform our network plumbing
	// Now that we have a running PID, we can move our network interface inside of it
	networkConfigOfContainerError := s.network.SetupContainerNetwork(state.ID, state.PID, ip)
	if networkConfigOfContainerError != nil {
		fmt.Printf("WARNING: failed to setup the network for node %v (running without net capabilities): %v", state.ID, networkConfigOfContainerError.Error())
	} else {
		state.IP = ip
	}

	//TODO: on top of this, I will need a persistence logic to keep the node state in the hard
	return state, nil
}
