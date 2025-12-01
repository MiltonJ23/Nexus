package state

import (
	"Nexus/internal/core"
	"encoding/json"
	"os"
	"sync"
)

const StateFile = "nexus.json" // This point to the json holding the state of the whole application
var GlobalState *StateManager

type AppState struct { // Basically this AppState is the single point of truth or should i say where the truth will be found
	Node  map[string]core.NodeState     `json:"nodes"`
	Files map[string]*core.FileMetadata `json:"files"`
}

// StateManager is going to handle concurrent access to the state file
type StateManager struct {
	mu    sync.RWMutex
	state AppState
}

func init() {
	GlobalState = &StateManager{
		state: AppState{
			Node:  make(map[string]core.NodeState),
			Files: make(map[string]*core.FileMetadata),
		},
	}
}

// Load reads the JSON file into memory.
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(StateFile)
	if os.IsNotExist(err) {
		return nil // New installation
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &sm.state)
}

// Save writes memory to the JSON file.
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(StateFile, data, 0644)
}

// --- Helper Methods ---

func (sm *StateManager) AddFile(meta *core.FileMetadata) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Files[meta.ID] = meta
}

func (sm *StateManager) GetFile(id string) (*core.FileMetadata, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	f, exists := sm.state.Files[id]
	return f, exists
}

func (sm *StateManager) GetAllFiles() []*core.FileMetadata {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	files := make([]*core.FileMetadata, 0, len(sm.state.Files))
	for _, f := range sm.state.Files {
		files = append(files, f)
	}
	return files
}

func (sm *StateManager) DeleteFile(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.state.Files, id)
}

func (sm *StateManager) GetActiveNodes() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	nodes := make([]string, 0)
	for id, node := range sm.state.Node {
		if node.Status == "Running" {
			nodes = append(nodes, id)
		}
	}
	return nodes
}
