package network

import (
	"Nexus/internal/core"
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

type SimpleIPAM struct{}

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
}

func (ipam *SimpleIPAM) ReleaseIP(ip string) {
	ipamLock.Lock()
	defer ipamLock.Unlock()

	delete(allocatedIPs, ip)
}
