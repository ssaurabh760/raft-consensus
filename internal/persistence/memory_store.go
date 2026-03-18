package persistence

import "sync"

// MemoryStore implements Storage using an in-memory map.
// Useful for testing where persistence across restarts is not needed.
type MemoryStore struct {
	mu    sync.RWMutex
	state *PersistentState
}

// NewMemoryStore creates a new in-memory storage.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		state: &PersistentState{
			CurrentTerm: 0,
			VotedFor:    -1,
			Log:         make([]LogEntry, 0),
		},
	}
}

// Save persists the entire Raft state atomically.
func (m *MemoryStore) Save(state *PersistentState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy the state to avoid aliasing.
	logCopy := make([]LogEntry, len(state.Log))
	for i, entry := range state.Log {
		cmdCopy := make([]byte, len(entry.Command))
		copy(cmdCopy, entry.Command)
		logCopy[i] = LogEntry{
			Term:    entry.Term,
			Index:   entry.Index,
			Command: cmdCopy,
		}
	}

	m.state = &PersistentState{
		CurrentTerm: state.CurrentTerm,
		VotedFor:    state.VotedFor,
		Log:         logCopy,
	}
	return nil
}

// Load retrieves the persisted Raft state.
func (m *MemoryStore) Load() (*PersistentState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy on read to prevent external mutation.
	logCopy := make([]LogEntry, len(m.state.Log))
	for i, entry := range m.state.Log {
		cmdCopy := make([]byte, len(entry.Command))
		copy(cmdCopy, entry.Command)
		logCopy[i] = LogEntry{
			Term:    entry.Term,
			Index:   entry.Index,
			Command: cmdCopy,
		}
	}

	return &PersistentState{
		CurrentTerm: m.state.CurrentTerm,
		VotedFor:    m.state.VotedFor,
		Log:         logCopy,
	}, nil
}

// SaveTermAndVote persists just the currentTerm and votedFor fields.
func (m *MemoryStore) SaveTermAndVote(term, votedFor int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.CurrentTerm = term
	m.state.VotedFor = votedFor
	return nil
}

// AppendLogEntries persists new log entries.
func (m *MemoryStore) AppendLogEntries(entries []LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range entries {
		cmdCopy := make([]byte, len(entry.Command))
		copy(cmdCopy, entry.Command)
		m.state.Log = append(m.state.Log, LogEntry{
			Term:    entry.Term,
			Index:   entry.Index,
			Command: cmdCopy,
		})
	}
	return nil
}

// TruncateLog removes log entries from the given index onward.
func (m *MemoryStore) TruncateLog(fromIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the position in the slice where entries have Index >= fromIndex.
	cutPos := len(m.state.Log)
	for i, entry := range m.state.Log {
		if entry.Index >= fromIndex {
			cutPos = i
			break
		}
	}
	m.state.Log = m.state.Log[:cutPos]
	return nil
}

// Close releases resources. No-op for in-memory store.
func (m *MemoryStore) Close() error {
	return nil
}
