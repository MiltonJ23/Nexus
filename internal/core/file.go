package core

import "time"

// FileMetadata represents a file uploaded on the nexus cloud
type FileMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Chunks    []Chunk   `json:"chunks"`
}

type Chunk struct {
	Index      int    `json:"index"`        // This is to keep the order of them chunks
	NodeID     string `json:"node_id"`      // This to tell us on which the chunk is stored
	PathOnDisk string `json:"path_on_disk"` // This is the path to find the chunk on the disk
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"` // we MD5/SHA256 for checking the integrity of the chunk
}
