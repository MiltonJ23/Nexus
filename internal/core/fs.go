package core

import "time"

type NodeType string

const (
	NodeTypeFile   NodeType = "file"
	NodeTypeFolder NodeType = "folder"
)

// FSNode represents a node in the Virtual File System (File or Folder)
type FSNode struct {
	Name      string    `json:"name"`
	Type      NodeType  `json:"type"`
	FileID    string    `json:"file_id,omitempty"` // Only if Type == File (Links to FileMetadata)
	Size      int64     `json:"size"`              // File size or recursive folder size
	CreatedAt time.Time `json:"created_at"`
	Children  []*FSNode `json:"children,omitempty"` // Only if Type == Folder
	Parent    *FSNode   `json:"-"`                  // internal link for traversal, ignore in JSON
}

// UserFileSystem represents the root of a user's storage
type UserFileSystem struct {
	Username   string  `json:"username"`
	Root       *FSNode `json:"root"`
	QuotaLimit int64   `json:"quota_limit"`
	QuotaUsed  int64   `json:"quota_used"`
}
