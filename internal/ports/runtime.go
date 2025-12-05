package ports

// This is the interface of our runtime service
import (
	"Nexus/internal/core"
	"io"
)

// The interface ContainerRuntime is the contract to start, stop and manage containers

type ContainerRuntime interface {
	// CreateAndStart  launch a container using a given configuration | it returns the initial state of the node , giving the PID on the host for example
	CreateAndStart(conf core.NodeConfig) (*core.NodeState, error)

	// Stop will stop the container by using its ID
	Stop(id string) error

	// GetState  will fetch the state of an existing container
	GetState(id string) (*core.NodeState, error)

	RunEphemeral(conf core.NodeConfig, stdout, stderr io.Writer) (int, error)
}
