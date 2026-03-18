package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	stateFileName = "raft_state.json"
	tempSuffix    = ".tmp"
)

// FileStore implements Storage using a JSON file on disk.
// Writes are atomic: data is written to a temp file, then renamed.
type FileStore struct {
	mu       sync.RWMutex
	dir      string
	filePath string
}

// NewFileStore creates a new file-based storage in the given directory.
// Creates the directory if it doesn't exist.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &FileStore{
		dir:      dir,
		filePath: filepath.Join(dir, stateFileName),
	}, nil
}

// Save persists the entire Raft state atomically.
func (fs *FileStore) Save(state *PersistentState) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.writeState(state)
}

// Load retrieves the persisted Raft state.
// Returns a zero-value PersistentState if no state file exists.
func (fs *FileStore) Load() (*PersistentState, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &PersistentState{
				CurrentTerm: 0,
				VotedFor:    -1,
				Log:         make([]LogEntry, 0),
			}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	if state.Log == nil {
		state.Log = make([]LogEntry, 0)
	}
	return &state, nil
}

// SaveTermAndVote persists just the currentTerm and votedFor fields.
// Loads current state, updates term/vote, and saves back.
func (fs *FileStore) SaveTermAndVote(term, votedFor int) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	state, err := fs.loadStateLocked()
	if err != nil {
		return err
	}
	state.CurrentTerm = term
	state.VotedFor = votedFor
	return fs.writeState(state)
}

// AppendLogEntries persists new log entries.
func (fs *FileStore) AppendLogEntries(entries []LogEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	state, err := fs.loadStateLocked()
	if err != nil {
		return err
	}
	state.Log = append(state.Log, entries...)
	return fs.writeState(state)
}

// TruncateLog removes log entries from the given index onward.
func (fs *FileStore) TruncateLog(fromIndex int) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	state, err := fs.loadStateLocked()
	if err != nil {
		return err
	}

	cutPos := len(state.Log)
	for i, entry := range state.Log {
		if entry.Index >= fromIndex {
			cutPos = i
			break
		}
	}
	state.Log = state.Log[:cutPos]
	return fs.writeState(state)
}

// Close releases resources. Flushes any pending state.
func (fs *FileStore) Close() error {
	return nil
}

// loadStateLocked loads state without acquiring the lock (caller must hold it).
func (fs *FileStore) loadStateLocked() (*PersistentState, error) {
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &PersistentState{
				CurrentTerm: 0,
				VotedFor:    -1,
				Log:         make([]LogEntry, 0),
			}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	if state.Log == nil {
		state.Log = make([]LogEntry, 0)
	}
	return &state, nil
}

// writeState atomically writes state to disk using write-to-temp + rename.
func (fs *FileStore) writeState(state *PersistentState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmpPath := fs.filePath + tempSuffix
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, fs.filePath); err != nil {
		os.Remove(tmpPath) // Clean up on failure.
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}
