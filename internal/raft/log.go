package raft

import (
	"fmt"
	"sync"
)

// LogEntry represents a single entry in the Raft log.
type LogEntry struct {
	Term    int
	Index   int
	Command interface{}
}

// String returns a human-readable representation of the log entry.
func (e LogEntry) String() string {
	return fmt.Sprintf("LogEntry{Term:%d, Index:%d, Command:%v}", e.Term, e.Index, e.Command)
}

// Log defines the interface for Raft log storage.
type Log interface {
	// Append adds a new entry to the log and returns its index.
	Append(entry LogEntry) int

	// GetEntry returns the log entry at the given index.
	// Returns an error if the index is out of bounds.
	GetEntry(index int) (LogEntry, error)

	// LastIndex returns the index of the last log entry, or 0 if the log is empty.
	LastIndex() int

	// LastTerm returns the term of the last log entry, or 0 if the log is empty.
	LastTerm() int

	// GetEntriesFrom returns all log entries starting from the given index (inclusive).
	GetEntriesFrom(index int) []LogEntry

	// Truncate removes all log entries from the given index onward (inclusive).
	Truncate(fromIndex int)

	// Len returns the number of entries in the log.
	Len() int
}

// MemoryLog is an in-memory implementation of the Log interface.
// The log uses 1-based indexing to match the Raft paper specification.
type MemoryLog struct {
	mu      sync.RWMutex
	entries []LogEntry
}

// NewMemoryLog creates a new empty in-memory log.
func NewMemoryLog() *MemoryLog {
	return &MemoryLog{
		entries: make([]LogEntry, 0),
	}
}

// Append adds a new entry to the log and returns its index.
func (l *MemoryLog) Append(entry LogEntry) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.Index = len(l.entries) + 1
	l.entries = append(l.entries, entry)
	return entry.Index
}

// GetEntry returns the log entry at the given 1-based index.
func (l *MemoryLog) GetEntry(index int) (LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if index < 1 || index > len(l.entries) {
		return LogEntry{}, fmt.Errorf("log index %d out of bounds [1, %d]", index, len(l.entries))
	}
	return l.entries[index-1], nil
}

// LastIndex returns the index of the last log entry, or 0 if empty.
func (l *MemoryLog) LastIndex() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.entries)
}

// LastTerm returns the term of the last log entry, or 0 if empty.
func (l *MemoryLog) LastTerm() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Term
}

// GetEntriesFrom returns all log entries starting from the given 1-based index.
func (l *MemoryLog) GetEntriesFrom(index int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if index < 1 || index > len(l.entries) {
		return nil
	}
	// Return a copy to prevent external mutation.
	result := make([]LogEntry, len(l.entries)-(index-1))
	copy(result, l.entries[index-1:])
	return result
}

// Truncate removes all log entries from the given 1-based index onward.
func (l *MemoryLog) Truncate(fromIndex int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if fromIndex < 1 || fromIndex > len(l.entries) {
		return
	}
	l.entries = l.entries[:fromIndex-1]
}

// Len returns the number of entries in the log.
func (l *MemoryLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.entries)
}
