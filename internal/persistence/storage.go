// Package persistence defines the interface and implementations for
// persisting Raft state (currentTerm, votedFor, log) to stable storage.
package persistence

// PersistentState holds the Raft state that must survive crashes.
// Per Raft paper Figure 2: currentTerm, votedFor, and log[] must be
// persisted before responding to RPCs.
type PersistentState struct {
	CurrentTerm int        `json:"currentTerm"`
	VotedFor    int        `json:"votedFor"`
	Log         []LogEntry `json:"log"`
}

// LogEntry is a persistence-layer log entry.
type LogEntry struct {
	Term    int    `json:"term"`
	Index   int    `json:"index"`
	Command []byte `json:"command,omitempty"`
}

// Storage defines the interface for persisting Raft state.
type Storage interface {
	// Save persists the entire Raft state atomically.
	Save(state *PersistentState) error

	// Load retrieves the persisted Raft state.
	// Returns a zero-value PersistentState if no state has been saved.
	Load() (*PersistentState, error)

	// SaveTermAndVote persists just the currentTerm and votedFor fields.
	// This is called frequently and should be fast.
	SaveTermAndVote(term, votedFor int) error

	// AppendLogEntries persists new log entries.
	AppendLogEntries(entries []LogEntry) error

	// TruncateLog removes log entries from the given index onward.
	TruncateLog(fromIndex int) error

	// Close releases any resources held by the storage.
	Close() error
}
