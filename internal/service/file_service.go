package service

import (
	"Nexus/internal/adapters/storage"
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"Nexus/internal/state"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const ChunkSize = 10 * 1024 * 1024 // 10MB Chunks

type FileService struct {
	storage ports.StorageManager
	state   *state.StateManager
}

func NewFileService() *FileService {
	// Initialize Dependencies
	sm := storage.NewLoopStorageManager()
	st := state.GlobalState
	st.Load() // Ensure state is loaded

	return &FileService{
		storage: sm,
		state:   st,
	}
}

// UploadFile splits a local file and distributes chunks across active nodes
func (s *FileService) UploadFile(localPath string) (*core.FileMetadata, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	// 1. Get Active Nodes
	nodeIDs := s.state.GetActiveNodes()
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("no active nodes found. Create a node first")
	}

	// 2. Prepare Metadata
	fileID := uuid.New().String()
	metadata := &core.FileMetadata{
		ID:        fileID,
		Name:      filepath.Base(localPath),
		Size:      fileSize,
		CreatedAt: time.Now(),
		Chunks:    make([]core.Chunk, 0),
	}

	// 3. Chunking Loop
	buffer := make([]byte, ChunkSize)
	chunkIndex := 0

	fmt.Printf("Starting upload of %s (%d bytes)...\n", metadata.Name, fileSize)

	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if bytesRead == 0 {
			break
		}

		// Round Robin Node Selection
		targetNodeID := nodeIDs[chunkIndex%len(nodeIDs)]
		chunkFilename := fmt.Sprintf("%s_part_%d", fileID, chunkIndex)

		// Calculate MD5
		hasher := md5.New()
		hasher.Write(buffer[:bytesRead])
		checksum := hex.EncodeToString(hasher.Sum(nil))

		// 4. Mount & Write
		fmt.Printf("  -> Writing Chunk %d to Node %s...\n", chunkIndex, targetNodeID)

		mountPoint, err := s.storage.MountOnHost(targetNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to mount node %s: %w", targetNodeID, err)
		}

		// Create reader from the buffer data we already have
		chunkReader := bytes.NewReader(buffer[:bytesRead])

		err = s.storage.WriteChunk(mountPoint, chunkFilename, chunkReader)
		s.storage.UnmountOnHost(targetNodeID) // Always unmount!

		if err != nil {
			return nil, fmt.Errorf("failed to write chunk to %s: %w", targetNodeID, err)
		}

		// 5. Update Metadata
		metadata.Chunks = append(metadata.Chunks, core.Chunk{
			Index:      chunkIndex,
			NodeID:     targetNodeID,
			PathOnDisk: chunkFilename,
			Size:       int64(bytesRead),
			Checksum:   checksum,
		})

		chunkIndex++
	}

	// 6. Save State
	s.state.AddFile(metadata)
	if err := s.state.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return metadata, nil
}

// DownloadFile reassembles chunks from nodes to a local file
func (s *FileService) DownloadFile(fileID string, destPath string) error {
	// 1. Retrieve Metadata
	meta, exists := s.state.GetFile(fileID)
	if !exists {
		return fmt.Errorf("file ID %s not found", fileID)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	fmt.Printf("Downloading %s (%s)...\n", meta.Name, meta.ID)

	// 2. Iterate Chunks
	for _, chunk := range meta.Chunks {
		fmt.Printf("  -> Retrieving Chunk %d from Node %s...\n", chunk.Index, chunk.NodeID)

		mountPoint, err := s.storage.MountOnHost(chunk.NodeID)
		if err != nil {
			return fmt.Errorf("mount error: %w", err)
		}

		reader, err := s.storage.ReadChunk(mountPoint, chunk.PathOnDisk)
		if err != nil {
			s.storage.UnmountOnHost(chunk.NodeID)
			return fmt.Errorf("read error: %w", err)
		}

		// Copy chunk to output file
		_, err = io.Copy(outFile, reader)
		reader.Close()
		s.storage.UnmountOnHost(chunk.NodeID)

		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}

// ListFiles returns all files
func (s *FileService) ListFiles() []*core.FileMetadata {
	return s.state.GetAllFiles()
}

// DeleteFile removes chunks from disks and metadata
func (s *FileService) DeleteFile(fileID string) error {
	meta, exists := s.state.GetFile(fileID)
	if !exists {
		return fmt.Errorf("file not found")
	}

	for _, chunk := range meta.Chunks {
		mountPoint, err := s.storage.MountOnHost(chunk.NodeID)
		if err == nil {
			s.storage.DeleteChunk(mountPoint, chunk.PathOnDisk)
			s.storage.UnmountOnHost(chunk.NodeID)
		} else {
			fmt.Printf("Warning: Could not mount node %s to delete chunk\n", chunk.NodeID)
		}
	}

	s.state.DeleteFile(fileID)
	return s.state.Save()
}
