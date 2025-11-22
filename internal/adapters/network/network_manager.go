package network

import (
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"fmt"
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	BridgeName = "nexus0"
	VethPrefix = "nex-" // this is the prefix for the all the about to be created veth pair
)

type NetlinkManager struct {
	ipam *SimpleIPAM
}

func NewNetlinkManager() ports.NetworkManager {
	return &NetlinkManager{ipam: NewSimpleIPAM()}
}

func (m *NetlinkManager) SetupBridge() error {
	// First of all , let's print an initialization message
	fmt.Printf("Configuration of the bridge : %v", BridgeName)

	// Let's first of all find if there is an already existing bridge
	link, _ := netlink.LinkByName(BridgeName)
	if link != nil {
		fmt.Printf(" -> Bridge %v is already existing. Let's use the current one...", BridgeName)
		return nil
	}
	// Here means the bridge doesn't exist already
	la := netlink.NewLinkAttrs()
	la.Name = BridgeName
	bridge := &netlink.Bridge{LinkAttrs: la}
	bridgeCreationError := netlink.LinkAdd(bridge)
	if bridgeCreationError != nil {
		return fmt.Errorf("unable to create the bridge %v : %v", BridgeName, bridgeCreationError.Error())
	}
	fmt.Printf(" -> bridge %v was created successfully", BridgeName)

	// Now let's assign the Default Gateway IP Address to the bridge
	ipNet, ipParsingError := netlink.ParseAddr(fmt.Sprintf("%s/24", DefaultGateway))
	if ipParsingError != nil {
		return fmt.Errorf("unable to parse the default gateway ip %v  address to the bridge %v", DefaultGateway, BridgeName)
	}
	ipAssignationError := netlink.AddrAdd(bridge, ipNet)
	if ipAssignationError != nil {
		return fmt.Errorf("unable to assign the default gateways ip %v address to the bridge %v", DefaultGateway, BridgeName)
	}
	// if it passes this , we can proudly print that the IP Gateway was successfully assigned
	fmt.Printf(" -> IP Gateway %v successfully assigned to bridge %v", DefaultGateway, BridgeName)

	// Now let's start the bridge
	bridgeLaunchError := netlink.LinkSetUp(bridge)
	if bridgeLaunchError != nil {
		return fmt.Errorf("unable to launch the bridge %v : %v", BridgeName, bridgeLaunchError.Error())
	}
	fmt.Printf(" -> bridge %v was successfully launched, with the default Gateway IP %v", BridgeName, DefaultGateway)

	return nil
}

// AssignIP simply implement the AssignIP clause in the ports.NetworkManger
func (m *NetlinkManager) AssignIP(id string) (core.IPAddress, error) {
	return m.ipam.AssignIP(id)
}

// SetupContainerNetwork will build, link and push the veth into the node namespace
func (m *NetlinkManager) SetupContainerNetwork(NodeID string, NodePID int, ip core.IPAddress) error {
	// To manage the network of the node , we have to temporarily pin the node's network namespace on the actual go thread

	// first we have to lock the thread OS , it is a must
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// let's save the actual namespace of the host, we will put it back later
	hostNS, readActualHostNameSpaceError := netns.Get()
	if readActualHostNameSpaceError != nil {
		return fmt.Errorf("failed to get the host namespace %v", readActualHostNameSpaceError.Error())
	}
	// it means we successfully retrieved it
	defer hostNS.Close()

	// now let's create and link the Veth pair on the node
	// let's start by assigning a new to the 2 pins of the Veth
	vethHost := fmt.Sprintf("%s%s", VethPrefix, NodeID)
	vethGuest := "eth0" // the name of the interface inside of the node

	// let's build the vethpair
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethHost},
		PeerName:  vethGuest,
	}
	buildVethError := netlink.LinkAdd(veth)
	if buildVethError != nil {
		return fmt.Errorf("unable to create veth pair %s : %v ", vethHost, buildVethError.Error())
	}

	// now let's attach the host side to our linux bridge
	bridge, readBridgeError := netlink.LinkByName(BridgeName)
	if readBridgeError != nil {
		return fmt.Errorf("unable to get the bridge %v : %v", BridgeName, readBridgeError.Error())
	}
	// reaching here simply means , we found the bridge, now let's pin the vethHost on the bridge
	pinVethHostOnBridgeError := netlink.LinkSetMaster(veth, bridge)
	if pinVethHostOnBridgeError != nil {
		return fmt.Errorf("failed to pin the vethHost pin %v on the bridge %v: %v", vethHost, BridgeName, pinVethHostOnBridgeError.Error())
	}
	// reaching here, means we were able to pin the vethHost Pin on the linux bridge
	fmt.Printf(" -> veth %v  was created successfully and linked to the bridge ", vethHost)

	// Now let's go configure the Container Network Namespace

	// let's start by toggle into the container NameSpace
	targetNS, readingContainerNetNS := netns.GetFromPid(NodePID)
	if readingContainerNetNS != nil {
		return fmt.Errorf("unable to get the namespace PID %d : %v", NodePID, readingContainerNetNS.Error())
	}
	defer targetNS.Close()

	//let's load the container NS from its PID
	loadingTargetNetNS := netns.Set(targetNS)
	if loadingTargetNetNS != nil {
		return fmt.Errorf("failed to load the target Namespace to NS PID %v:%v", NodePID, loadingTargetNetNS.Error())
	}

	// Starting from here , all of this is happening inside the container

	// now let's launch the loopback device
	lo, _ := netlink.LinkByName("lo")
	launchLoopBackDeviceError := netlink.LinkSetUp(lo)
	if launchLoopBackDeviceError != nil {
		return fmt.Errorf("failed to launch the loop back device %v : %v", lo, launchLoopBackDeviceError.Error())
	}

	// now let's launch the eth0 network interface
	eth0, findingEth0Error := netlink.LinkByName(vethGuest)
	if findingEth0Error != nil {
		return fmt.Errorf("unable to find the interface eth0 inside of the container:%v", findingEth0Error.Error())
	}
	// now let's move the eth0 inside  the container
	movingEth0InContainer := netlink.LinkSetNsPid(eth0, NodePID)
	if movingEth0InContainer != nil {
		return fmt.Errorf("unable to move the eth0 inside of the container:%v", movingEth0InContainer.Error())
	}
	// here it means, we were able to successfully find the eth0 veth inside  the container
	launchingEth0Error := netlink.LinkSetUp(eth0)
	if launchingEth0Error != nil {
		return fmt.Errorf("unable to launch the veth eth0 : %v", launchingEth0Error.Error())
	}
	// reaching here means we were able to successfully launch the eth0 interface

	// now let's assign the ip address to the container
	addr, parsingAddressError := netlink.ParseAddr(fmt.Sprintf("%s/24", ip.IP))
	if parsingAddressError != nil {
		return fmt.Errorf("unable to parse the address %v : %v", ip.IP, parsingAddressError.Error())
	}
	assigIPAddressToContainerError := netlink.AddrAdd(eth0, addr)
	if assigIPAddressToContainerError != nil {
		return fmt.Errorf("failed to assign the IP %v to the container %v eth0 interface", ip.IP, NodeID)
	}
	//reaching here means we were able to successfully assign the IP to the eth0 interface
	fmt.Printf(" -> IP %v assigned successfully to container", ip.IP)

	// now let's configure the default gateway , the thing is without this the node doesn't know how to forward traffic that doesn't come from 10.0.42.0/24
	gatewayIP := net.ParseIP(ip.Gateway)
	route := &netlink.Route{} // with this we can create a new route
	route.ILinkIndex = eth0.Attrs().Index
	route.Gw = gatewayIP
	AddingGatewayError := netlink.RouteAdd(route)
	if AddingGatewayError != nil {
		return fmt.Errorf(" failed to add route gateway %v : %v", ip.Gateway, AddingGatewayError.Error())
	}
	// if it went without mistake, then we can conclude that the route gateway was added successfully
	fmt.Printf(" -> Gateway route %v successfully configured ", ip.Gateway)

	// after performing all of our tasks , let's get back to our host namespace
	HostNSRollbackError := netns.Set(hostNS)
	if HostNSRollbackError != nil {
		return fmt.Errorf("unable to get back on track, failed to return to the host net Namespace: %s", HostNSRollbackError.Error())
	}
	return nil
}
