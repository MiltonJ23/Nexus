package network

import (
	"Nexus/internal/core"
	"Nexus/internal/state"
	"fmt"
	"net"
	"sync"
)

const (
	DefaultNetworkCIDR = "10.0.42.0/24"
	DefaultGateway     = "10.0.42.1"
)

var (
	allocatedIPs = make(map[string]bool) // Will hold the list of already allocated IP Address
	ipamLock     sync.Mutex
)

/*type SimpleIPAM struct{}

// NewSimpleIPAM will preserve the IP Address of the bridge and ensure it is not attributed to any nodes
func NewSimpleIPAM() *SimpleIPAM {
	allocatedIPs[DefaultGateway] = true
	return &SimpleIPAM{}
}

func (ipam *SimpleIPAM) AssignIP(nodeID string) (core.IPAddress, error) {
	ipamLock.Lock()
	defer ipamLock.Unlock() // We report the unlocking of the mutex to later
	ip, _, networkParsingError := net.ParseCIDR(DefaultNetworkCIDR)
	if networkParsingError != nil {
		return core.IPAddress{}, fmt.Errorf("encountered an invalid cidr %v", networkParsingError.Error())
	}

	// let's start by finding the next available ip address
	for i := 2; i < 255; i++ {
		// let's increment the final octet
		ip[len(ip)-1] = byte(i)
		currentIP := ip.String() // get the string literal of the IP Address

		_, ok := allocatedIPs[currentIP]
		if !ok {
			// This means we found out the currentIP to be an available IP Address , so we will attribute it
			allocatedIPs[currentIP] = true // let's mark the current found IP address as an occupied IP address
			return core.IPAddress{
				IP:      currentIP,
				Subnet:  DefaultNetworkCIDR,
				Gateway: DefaultGateway,
			}, nil
		}

	}

	return core.IPAddress{}, fmt.Errorf("unable to find an available IP Address in the subnet %v ", DefaultNetworkCIDR)
}*/

type SimpleIPAM struct {
	mu sync.Mutex
}

func NewSimpleIPAM() *SimpleIPAM {
	return &SimpleIPAM{}
}

func (i *SimpleIPAM) AssignIP(nodeID string) (core.IPAddress, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	// 1. Charger l'état pour voir les IPs utilisées
	st := state.GlobalState
	// Pas besoin de st.Load() ici car NodeService l'a déjà fait
	usedIPs := make(map[string]bool)
	usedIPs[DefaultGateway] = true
	for _, node := range st.State.Node {
		if node.IP.IP != "" {
			usedIPs[node.IP.IP] = true
		}
	}
	// 2. Trouver la prochaine libre
	ip, _, _ := net.ParseCIDR(DefaultNetworkCIDR)

	ip = ip.To4()
	if ip == nil {
		return core.IPAddress{}, fmt.Errorf("this is not a valid IPV4 address ")
	}

	// On scanne de .2 à .254
	for x := 2; x < 255; x++ {
		ip[3] = byte(x) // Modification du dernier octet (IPv4 only)
		ipStr := ip.String()

		if !usedIPs[ipStr] {
			// Trouvé !
			return core.IPAddress{
				IP:      ipStr,
				Subnet:  DefaultNetworkCIDR,
				Gateway: DefaultGateway,
			}, nil
		}
	}

	return core.IPAddress{}, fmt.Errorf("subnet exhausté")
}

func (ipam *SimpleIPAM) ReleaseIP(ip string) {
	ipamLock.Lock()
	defer ipamLock.Unlock()

	delete(allocatedIPs, ip)
}
