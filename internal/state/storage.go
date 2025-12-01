package state

import (
	"Nexus/internal/core"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const StateDir = "/var/lib/nexus"
const StateFile = "nexus.json" // This point to the json holding the state of the whole application
var GlobalState *StateManager

type AppState struct { // Basically this AppState is the single point of truth or should i say where the truth will be found
	Node  map[string]core.NodeState     `json:"nodes"`
	Files map[string]*core.FileMetadata `json:"files"`
}

// StateManager is going to handle concurrent access to the state file
type StateManager struct {
	mu    sync.RWMutex
	State AppState
	path  string
}

func init() {

	fullPath := filepath.Join(StateDir, StateFile)

	GlobalState = &StateManager{
		State: AppState{
			Node:  make(map[string]core.NodeState),
			Files: make(map[string]*core.FileMetadata),
		},
		path: fullPath,
	}
}

// Load reads the JSON file into memory.
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. Check if file exists
	data, err := os.ReadFile(sm.path)
	if os.IsNotExist(err) {
		return nil // New installation, memory is already empty via init()
	}
	if err != nil {
		return fmt.Errorf("failed to read state file at %s: %w", sm.path, err)
	}

	// 2. Hydrate memory
	return json.Unmarshal(data, &sm.State)
}

// Save writes memory to the JSON file.
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 1. Serialize
	data, err := json.MarshalIndent(sm.State, "", "  ")
	if err != nil {
		return err
	}

	// 2. Ensure Directory Exists (Critical for absolute paths)
	// If /var/lib/nexus doesn't exist, Create won't work.
	if err := os.MkdirAll(StateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory %s: %w", StateDir, err)
	}

	// 3. Write Atomic (Technically os.WriteFile isn't fully atomic but good enough for now)
	return os.WriteFile(sm.path, data, 0644)
}

// --- Helper Methods ---

func (sm *StateManager) AddFile(meta *core.FileMetadata) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.State.Files[meta.ID] = meta
}

func (sm *StateManager) GetFile(id string) (*core.FileMetadata, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	f, exists := sm.State.Files[id]
	return f, exists
}

func (sm *StateManager) GetAllFiles() []*core.FileMetadata {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	files := make([]*core.FileMetadata, 0, len(sm.State.Files))
	for _, f := range sm.State.Files {
		files = append(files, f)
	}
	return files
}

func (sm *StateManager) DeleteFile(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.State.Files, id)
}

func (sm *StateManager) GetActiveNodes() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	nodes := make([]string, 0)
	for id, node := range sm.State.Node {
		if node.Status == "Running" {
			nodes = append(nodes, id)
		}
	}
	return nodes
}

func (sm *StateManager) AddNode(node core.NodeState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.State.Node[node.ID] = node
}

func (sm *StateManager) GetNode(id string) (*core.NodeState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	n, exists := sm.State.Node[id]
	return &n, exists
}
