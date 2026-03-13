package service

import (
	"Nexus/internal/core"
	"Nexus/internal/state"
	"fmt"
	"strings"
	"time"
)

type FSService struct {
	fileService *FileService // Reusing the chunk logic from Milestone 4
	state       *state.StateManager
}

// NewFSService creates and returns an FSService initialized with a new FileService and the global state manager.
func NewFSService() *FSService {
	return &FSService{
		fileService: NewFileService(),
		state:       state.GlobalState,
	}
}

// --- Helper: Path Resolver ---
// Returns the node at the given path and its parent
func (s *FSService) resolvePath(fs *core.UserFileSystem, path string) (*core.FSNode, *core.FSNode, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return fs.Root, nil, nil // Root request
	}

	current := fs.Root
	var parent *core.FSNode

	for _, part := range parts {
		found := false
		for _, child := range current.Children {
			if child.Name == part {
				parent = current
				current = child
				found = true
				break
			}
		}
		if !found {
			return nil, current, fmt.Errorf("path not found: %s", part)
		}
	}
	return current, parent, nil
}

// 1. CREATE FOLDER (Mkdir)
func (s *FSService) MakeDirectory(username, path string) error {
	fs := s.state.GetUserFS(username)

	// Check if already exists
	node, _, err := s.resolvePath(fs, path)
	if err == nil && node != nil {
		return fmt.Errorf("directory already exists")
	}

	// We expect the parent to exist (mkdir, not mkdir -p for now)
	// 'parent' from resolvePath error return is actually the last found node
	// Logic fix: We need to find the parent of the NEW folder.

	dirName := path[strings.LastIndex(path, "/")+1:]
	parentPath := path[:strings.LastIndex(path, "/")]
	if parentPath == "" {
		parentPath = "/"
	}

	parentNode, _, err := s.resolvePath(fs, parentPath)
	if err != nil {
		return fmt.Errorf("parent directory does not exist")
	}

	if parentNode.Type != core.NodeTypeFolder {
		return fmt.Errorf("parent is not a folder")
	}

	newNode := &core.FSNode{
		Name:      dirName,
		Type:      core.NodeTypeFolder,
		CreatedAt: time.Now(),
		Children:  []*core.FSNode{},
	}

	parentNode.Children = append(parentNode.Children, newNode)
	return s.state.Save()
}

// 2. UPLOAD FILE (With Quota)
func (s *FSService) UploadFileToPath(username, localPath, virtualPath string) error {
	fs := s.state.GetUserFS(username)

	// 1. Quota Check (Pre-check size)
	// Note: Accurate quota check needs file size, we check inside FileService usually,
	// but let's assume we proceed and rollback if quota exceeded.

	// 2. Physical Upload (Milestone 4 Logic)
	meta, err := s.fileService.UploadFile(localPath)
	if err != nil {
		return err
	}

	// 3. Quota Validation
	if fs.QuotaUsed+meta.Size > fs.QuotaLimit {
		// Rollback: Delete physical chunks
		s.fileService.DeleteFile(meta.ID)
		return fmt.Errorf("quota exceeded")
	}

	// 4. Link to VFS
	// Resolve Parent Folder
	fileName := virtualPath[strings.LastIndex(virtualPath, "/")+1:]
	parentPath := virtualPath[:strings.LastIndex(virtualPath, "/")]
	if parentPath == "" {
		parentPath = "/"
	}

	parentNode, _, err := s.resolvePath(fs, parentPath)
	if err != nil {
		// Cleanup orphan file
		s.fileService.DeleteFile(meta.ID)
		return fmt.Errorf("destination folder does not exist")
	}

	newNode := &core.FSNode{
		Name:      fileName,
		Type:      core.NodeTypeFile,
		FileID:    meta.ID,
		Size:      meta.Size,
		CreatedAt: time.Now(),
	}

	parentNode.Children = append(parentNode.Children, newNode)
	fs.QuotaUsed += meta.Size

	return s.state.Save()
}

// 3. DELETE (Recursive)
func (s *FSService) Delete(username, path string) error {
	fs := s.state.GetUserFS(username)
	target, parent, err := s.resolvePath(fs, path)
	if err != nil {
		return err
	}

	if target == fs.Root {
		return fmt.Errorf("cannot delete root")
	}

	// Recursive Deletion Helper
	var deleteNode func(*core.FSNode)
	deleteNode = func(n *core.FSNode) {
		if n.Type == core.NodeTypeFile {
			// Delete physical chunks
			s.fileService.DeleteFile(n.FileID)
			fs.QuotaUsed -= n.Size
		} else {
			// Folder: Recurse
			for _, child := range n.Children {
				deleteNode(child)
			}
		}
	}

	// Execute Deletion
	deleteNode(target)

	// Remove from Parent's Children list
	newChildren := []*core.FSNode{}
	for _, child := range parent.Children {
		if child != target {
			newChildren = append(newChildren, child)
		}
	}
	parent.Children = newChildren

	return s.state.Save()
}

// 4. MOVE / RENAME
func (s *FSService) Move(username, oldPath, newPath string) error {
	fs := s.state.GetUserFS(username)

	// Find Source
	sourceNode, sourceParent, err := s.resolvePath(fs, oldPath)
	if err != nil {
		return err
	}

	// Find Destination Parent
	destDirName := newPath[:strings.LastIndex(newPath, "/")]
	if destDirName == "" {
		destDirName = "/"
	}
	destName := newPath[strings.LastIndex(newPath, "/")+1:]

	destParent, _, err := s.resolvePath(fs, destDirName)
	if err != nil {
		return fmt.Errorf("destination directory invalid")
	}

	// Detach from Source Parent
	newSourceChildren := []*core.FSNode{}
	for _, child := range sourceParent.Children {
		if child != sourceNode {
			newSourceChildren = append(newSourceChildren, child)
		}
	}
	sourceParent.Children = newSourceChildren

	// Update Name
	sourceNode.Name = destName

	// Attach to Dest Parent
	destParent.Children = append(destParent.Children, sourceNode)

	return s.state.Save()
}

// 5. LIST (Ls)
func (s *FSService) ListDir(username, path string) ([]*core.FSNode, error) {
	fs := s.state.GetUserFS(username)
	node, _, err := s.resolvePath(fs, path)
	if err != nil {
		return nil, err
	}

	if node.Type != core.NodeTypeFolder {
		return nil, fmt.Errorf("not a directory")
	}
	return node.Children, nil
}
