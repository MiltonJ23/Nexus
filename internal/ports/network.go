package ports

import "Nexus/internal/core"

// NetworkManager is the contract to interact with the network layer of the container
type NetworkManager interface {
	// SetupBridge will create the bridge , assign an IP  and launch the bridge
	SetupBridge() error

	// AssignIP will manage the sequential attribution of IP addresses to nodes
	AssignIP(id string) (core.IPAddress, error)

	// SetupContainerNetwork will create the Veth will connect it to the bridge and force it into the node NameSpace
	SetupContainerNetwork(NodeID string, NodePID int, ip core.IPAddress) error
}
