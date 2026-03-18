package raft

import (
	"testing"
)

func TestNewMemoryLog(t *testing.T) {
	log := NewMemoryLog()
	if log.Len() != 0 {
		t.Errorf("expected empty log, got length %d", log.Len())
	}
	if log.LastIndex() != 0 {
		t.Errorf("expected LastIndex 0, got %d", log.LastIndex())
	}
	if log.LastTerm() != 0 {
		t.Errorf("expected LastTerm 0, got %d", log.LastTerm())
	}
}

func TestMemoryLogAppend(t *testing.T) {
	log := NewMemoryLog()

	idx1 := log.Append(LogEntry{Term: 1, Command: "set x=1"})
	if idx1 != 1 {
		t.Errorf("expected index 1, got %d", idx1)
	}

	idx2 := log.Append(LogEntry{Term: 1, Command: "set y=2"})
	if idx2 != 2 {
		t.Errorf("expected index 2, got %d", idx2)
	}

	idx3 := log.Append(LogEntry{Term: 2, Command: "set z=3"})
	if idx3 != 3 {
		t.Errorf("expected index 3, got %d", idx3)
	}

	if log.Len() != 3 {
		t.Errorf("expected length 3, got %d", log.Len())
	}
	if log.LastIndex() != 3 {
		t.Errorf("expected LastIndex 3, got %d", log.LastIndex())
	}
	if log.LastTerm() != 2 {
		t.Errorf("expected LastTerm 2, got %d", log.LastTerm())
	}
}

func TestMemoryLogGetEntry(t *testing.T) {
	log := NewMemoryLog()
	log.Append(LogEntry{Term: 1, Command: "cmd1"})
	log.Append(LogEntry{Term: 2, Command: "cmd2"})

	entry, err := log.GetEntry(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Term != 1 || entry.Command != "cmd1" {
		t.Errorf("unexpected entry: %v", entry)
	}

	entry, err = log.GetEntry(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Term != 2 || entry.Command != "cmd2" {
		t.Errorf("unexpected entry: %v", entry)
	}

	// Out of bounds
	_, err = log.GetEntry(0)
	if err == nil {
		t.Error("expected error for index 0")
	}
	_, err = log.GetEntry(3)
	if err == nil {
		t.Error("expected error for index 3")
	}
}

func TestMemoryLogGetEntriesFrom(t *testing.T) {
	log := NewMemoryLog()
	log.Append(LogEntry{Term: 1, Command: "a"})
	log.Append(LogEntry{Term: 1, Command: "b"})
	log.Append(LogEntry{Term: 2, Command: "c"})

	entries := log.GetEntriesFrom(2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Command != "b" || entries[1].Command != "c" {
		t.Errorf("unexpected entries: %v", entries)
	}

	// From beginning
	entries = log.GetEntriesFrom(1)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Out of bounds
	entries = log.GetEntriesFrom(0)
	if entries != nil {
		t.Errorf("expected nil for index 0, got %v", entries)
	}
	entries = log.GetEntriesFrom(4)
	if entries != nil {
		t.Errorf("expected nil for index 4, got %v", entries)
	}
}

func TestMemoryLogGetEntriesFromReturnsCopy(t *testing.T) {
	log := NewMemoryLog()
	log.Append(LogEntry{Term: 1, Command: "a"})
	log.Append(LogEntry{Term: 1, Command: "b"})

	entries := log.GetEntriesFrom(1)
	entries[0].Command = "modified"

	original, _ := log.GetEntry(1)
	if original.Command == "modified" {
		t.Error("GetEntriesFrom should return a copy, not a reference to internal data")
	}
}

func TestMemoryLogTruncate(t *testing.T) {
	log := NewMemoryLog()
	log.Append(LogEntry{Term: 1, Command: "a"})
	log.Append(LogEntry{Term: 1, Command: "b"})
	log.Append(LogEntry{Term: 2, Command: "c"})

	log.Truncate(2)
	if log.Len() != 1 {
		t.Errorf("expected length 1 after truncate, got %d", log.Len())
	}
	if log.LastIndex() != 1 {
		t.Errorf("expected LastIndex 1, got %d", log.LastIndex())
	}

	entry, _ := log.GetEntry(1)
	if entry.Command != "a" {
		t.Errorf("expected command 'a', got '%v'", entry.Command)
	}

	// Truncate from 1 should empty the log
	log.Truncate(1)
	if log.Len() != 0 {
		t.Errorf("expected empty log after truncate from 1, got %d", log.Len())
	}
}

func TestMemoryLogTruncateOutOfBounds(t *testing.T) {
	log := NewMemoryLog()
	log.Append(LogEntry{Term: 1, Command: "a"})

	// Truncate with invalid index should be a no-op
	log.Truncate(0)
	if log.Len() != 1 {
		t.Errorf("truncate(0) should be no-op, got length %d", log.Len())
	}

	log.Truncate(5)
	if log.Len() != 1 {
		t.Errorf("truncate(5) should be no-op, got length %d", log.Len())
	}
}

func TestLogEntryString(t *testing.T) {
	entry := LogEntry{Term: 3, Index: 7, Command: "set x=1"}
	s := entry.String()
	if s != "LogEntry{Term:3, Index:7, Command:set x=1}" {
		t.Errorf("unexpected String(): %s", s)
	}
}

func TestNodeRoleString(t *testing.T) {
	tests := []struct {
		role     NodeRole
		expected string
	}{
		{Follower, "Follower"},
		{Candidate, "Candidate"},
		{Leader, "Leader"},
		{NodeRole(99), "Unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.role.String(); got != tt.expected {
			t.Errorf("NodeRole(%d).String() = %q, want %q", tt.role, got, tt.expected)
		}
	}
}
