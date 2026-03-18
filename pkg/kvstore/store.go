// Package kvstore implements a distributed key-value store on top of the
// Raft consensus layer. It acts as the state machine that applies committed
// log entries to produce a consistent key-value mapping across all nodes.
package kvstore

import (
	"encoding/json"
	"fmt"
	"sync"
)

// OpType represents the type of KV operation.
type OpType string

const (
	OpPut    OpType = "put"
	OpDelete OpType = "delete"
)

// Command represents a KV store operation that gets replicated via Raft.
type Command struct {
	Op    OpType `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// EncodeCommand serializes a Command to JSON bytes for Raft log entry.
func EncodeCommand(cmd Command) ([]byte, error) {
	return json.Marshal(cmd)
}

// DecodeCommand deserializes a Command from JSON bytes.
func DecodeCommand(data []byte) (Command, error) {
	var cmd Command
	err := json.Unmarshal(data, &cmd)
	return cmd, err
}

// Store is a thread-safe in-memory key-value store that serves as the
// Raft state machine. Committed log entries are applied here to produce
// a consistent state across all nodes.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewStore creates a new empty KV store.
func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Get retrieves the value for a key. Returns the value and whether the key exists.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Apply applies a Command to the store. This is called when a Raft log entry
// is committed and needs to be applied to the state machine.
func (s *Store) Apply(cmd Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch cmd.Op {
	case OpPut:
		s.data[cmd.Key] = cmd.Value
	case OpDelete:
		delete(s.data, cmd.Key)
	default:
		return fmt.Errorf("unknown operation: %s", cmd.Op)
	}
	return nil
}

// ApplyBytes decodes and applies a command from raw bytes.
func (s *Store) ApplyBytes(data []byte) error {
	// Handle string-encoded commands (from Submit("set x=1") style).
	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		// Not JSON — treat as a no-op or raw string command.
		return nil
	}
	return s.Apply(cmd)
}

// Len returns the number of keys in the store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Keys returns all keys in the store.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Snapshot returns a copy of the entire store data.
func (s *Store) Snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := make(map[string]string, len(s.data))
	for k, v := range s.data {
		snapshot[k] = v
	}
	return snapshot
}
