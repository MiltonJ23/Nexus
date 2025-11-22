package ports

type StorageManager interface {
	// CreateVolume is a method that will allow us to create an image file that is then going to be mounted in the specified container
	CreateVolume(NodeID string, size string) (string, error)

	//AttachVolume readies the volume for the mounting procedure
	AttachVolume(VolumePath string)
}
