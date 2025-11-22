package core

// This is where a node is defined

type NodeConfig struct {
	ID             string
	Hostname       string
	Memory         int64 // This is the limitation that is going to be implemented using cgroups in MB
	CPUShares      uint64
	RootfsPath     string   // the root filesystem path, the little package that is going to give our node process an environment of execution , a little root filesystem
	Command        []string // the command to be executed
	NetworkEnabled bool     // this will allow us to activate/deactivate the network
}

// now let's define, the state of a node that is going to be persisted in hard memory

type NodeState struct {
	NodeConfig
	PID    int       // the process identifier inside the running node
	Status string    // to see if it's "RUNNING" or "STOPPED" or even "ERROR"
	IP     IPAddress // the ip address of course
}

type IPAddress struct {
	IP      string // This is the ip address of the node , let's say for example 192.168.3.8
	Subnet  string // let's say for example 192.168.3.0/24
	Gateway string // This is the bridge ip address
}
