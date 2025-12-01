package ports

import "io"

type StorageManager interface {
	// CreateVolume is a method that will allow us to create an image file that is then going to be mounted in the specified container
	CreateVolume(NodeID string, size string) (string, error)

	//AttachVolume readies the volume for the mounting procedure
	AttachVolume(VolumePath string) (string, error)

	// MountVolumeOnHost is supposed to mount . the volume on the host before performing the I/O operations of the
	MountVolumeOnHost(NodeID string) error

	UnMountVolumeOnHost(NodeID string) error

	WriteChunk(MountPoint string, Filename string, data io.Reader) error
	ReadChunk(MountPoint string, Filename string) (io.ReadCloser, error)
	DeleteChunk(MountPoint string, filename string) error
	ListFiles(MountPoint string) ([]string, error)
}
