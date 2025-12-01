package storage

import (
	"Nexus/internal/ports"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var _ ports.StorageManager = &LoopStorageManager{}

type LoopStorageManager struct {
	BasePath string
}

func NewLoopStorageManager() ports.StorageManager {
	path := "/var/lib/nexus/volumes"
	FolderCreationerr := os.MkdirAll(path, 0755)
	if FolderCreationerr != nil {
		return nil
	}
	return &LoopStorageManager{BasePath: path}
}
func (s *LoopStorageManager) CreateVolume(NodeID string, size string) (string, error) {
	// Utilisation de filepath.Join pour éviter les erreurs de slash
	volumePath := filepath.Join(s.BasePath, fmt.Sprintf("%s.img", NodeID))

	fmt.Printf("Creating the new volume for %s (%s)....", NodeID, size)

	cmd := exec.Command("truncate", "-s", size, volumePath)
	cmdOutput, cmdError := cmd.CombinedOutput()
	if cmdError != nil {
		return "", fmt.Errorf("failed to truncate %s : %v", string(cmdOutput), cmdError.Error())
	}

	fmt.Printf("  -> now let's format the volume in ext4...\n")
	cmdFormat := exec.Command("mkfs.ext4", "-F", volumePath)
	output, err := cmdFormat.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to format %s to ext4 format: %v", string(output), err.Error())
	}
	return volumePath, nil
}

func (s *LoopStorageManager) AttachVolume(VolumePath string) (string, error) {
	// Let's mount the volume on the node | we will use losetup -f: in charge of finding the first available loop device | --show flag: to display the name of the loop | - P: it is supposed to scan the partitions , a good practice as they say
	cmd := exec.Command("losetup", "-f", "--show", "-P", VolumePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed losetup %s : %v ", string(output), err.Error())
	}

	loopDevice := strings.TrimSpace(string(output))
	fmt.Printf("  -> volume mounted on the device %s\n", loopDevice)
	return loopDevice, nil
}

func (s *LoopStorageManager) MountOnHost(nodeID string) (string, error) {
	// Construct path to the .img file
	imagePath := filepath.Join(s.BasePath, fmt.Sprintf("%s.img", nodeID))

	// Create a temp mount point: /tmp/cloudsim/mounts/node-1
	mountPoint := filepath.Join("/tmp/nexus/mounts", nodeID)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return "", fmt.Errorf("failed to create mount dir: %w", err)
	}

	// Execute mount.
	// NOTE: We use the image file directly. Linux handles loop allocation automatically.
	cmd := exec.Command("mount", "-o", "loop", imagePath, mountPoint)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("mount failed: %s %w", string(out), err)
	}

	return mountPoint, nil
}

func (s *LoopStorageManager) UnmountOnHost(nodeID string) error {
	mountPoint := filepath.Join("/tmp/nexus/mounts", nodeID)
	// Unmount
	cmd := exec.Command("umount", mountPoint)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount failed: %s %w", string(out), err)
	}
	// Cleanup directory
	return os.Remove(mountPoint)
}

func (s *LoopStorageManager) WriteChunk(mountPoint string, filename string, data io.Reader) error {
	destPath := filepath.Join(mountPoint, filename)
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, data)
	return err
}

func (s *LoopStorageManager) ReadChunk(mountPoint string, filename string) (io.ReadCloser, error) {
	srcPath := filepath.Join(mountPoint, filename)
	return os.Open(srcPath)
}

func (s *LoopStorageManager) DeleteChunk(mountPoint string, filename string) error {
	srcPath := filepath.Join(mountPoint, filename)
	return os.Remove(srcPath)
}
