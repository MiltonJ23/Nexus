package core

import "time"

// This is where a node is defined

type NodeConfig struct {
	ID             string
	Hostname       string
	Memory         int64 // This is the limitation that is going to be implemented using cgroups in MB
	CPUShares      uint64
	RootfsPath     string   // the root filesystem path, the little package that is going to give our node process an environment of execution , a little root filesystem
	Command        []string // the command to be executed
	NetworkEnabled bool     // this will allow us to activate/deactivate the network
	StorageSize    string
	VolumePath     string
}

// now let's define, the state of a node that is going to be persisted in hard memory

type NodeState struct {
	NodeConfig
	PID           int         // the process identifier inside the running node
	Status        string      // to see if it's "RUNNING" or "STOPPED" or even "ERROR"
	IP            IPAddress   // the ip address of course
	LatestMetrics NodeMetrics `json:"latest_metrics"`
}

type IPAddress struct {
	IP      string // This is the ip address of the node , let's say for example 192.168.3.8
	Subnet  string // let's say for example 192.168.3.0/24
	Gateway string // This is the bridge ip address
}

type NodeMetrics struct {
	NodeID        string    `json:"node_id"`
	Timestamp     time.Time `json:"timestamp"`
	MemoryUsage   int64     `json:"memory_usage_bytes"`
	MemoryLimit   int64     `json:"memory_limit_bytes"`
	CPUUsageTotal int64     `json:"cpu_usage_nanos"` // This right here is the total cpu consumed since the node launch
	CPUPercent    float64   `json:"cpu_percent"`
}
