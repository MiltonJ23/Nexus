package storage

import (
	"Nexus/internal/ports"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

/*func (s *LoopStorageManager) CreateVolume(NodeID string, size string) (string, error) {
	// let's build the volume path, which means including the image file absolute
	volumePath := filepath.Join(s.BasePath, fmt.Sprintf("%s.img", NodeID))

	fmt.Printf("Creating the new volume for %s (%s)....", NodeID, size)

	// let's create the empty file with truncate(our fastest alternative for now ) rather than using system calls as we did
	cmd := exec.Command("truncate", "-s", size, volumePath)
	cmdOutput, cmdError := cmd.CombinedOutput()
	if cmdError != nil {
		return "", fmt.Errorf("failed to truncate %s : %v", string(cmdOutput), cmdError.Error())
	}
	// if we reach here, it means we were able to build the volume in the desired path
	// now let's format the volume in ext4
	fmt.Printf("  -> now let's format the volume in ext4...\n")
	cmdFormat := exec.Command("mkfs.ext4", "-F", volumePath)
	output, err := cmdFormat.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to format %s to ext4 format: %v", string(output), err.Error())
	}
	return volumePath, nil
}*/

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
