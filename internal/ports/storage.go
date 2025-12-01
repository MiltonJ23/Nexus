package ports

import "io"

type StorageManager interface {
	// CreateVolume is a method that will allow us to create an image file that is then going to be mounted in the specified container
	CreateVolume(NodeID string, size string) (string, error)

	//AttachVolume readies the volume for the mounting procedure
	AttachVolume(VolumePath string) (string, error)

	// MountOnHost mounts a node's loop device to a temporary path on the host.
	MountOnHost(nodeID string) (string, error)

	// UnmountOnHost unmounts the temporary path.
	UnmountOnHost(nodeID string) error

	// WriteChunk writes data stream to a specific file on the mounted path.
	WriteChunk(mountPoint string, filename string, data io.Reader) error

	// ReadChunk returns a reader for a specific file on the mounted path.
	ReadChunk(mountPoint string, filename string) (io.ReadCloser, error)

	// DeleteChunk removes a file from the mounted path.
	DeleteChunk(mountPoint string, filename string) error
}
