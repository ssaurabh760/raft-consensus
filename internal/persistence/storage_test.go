package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

// storageTestSuite runs a common set of tests against any Storage implementation.
func storageTestSuite(t *testing.T, newStore func(t *testing.T) Storage) {
	t.Run("LoadEmpty", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if state.CurrentTerm != 0 {
			t.Errorf("expected term 0, got %d", state.CurrentTerm)
		}
		if state.VotedFor != -1 {
			t.Errorf("expected votedFor -1, got %d", state.VotedFor)
		}
		if len(state.Log) != 0 {
			t.Errorf("expected empty log, got %d entries", len(state.Log))
		}
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		state := &PersistentState{
			CurrentTerm: 5,
			VotedFor:    2,
			Log: []LogEntry{
				{Term: 1, Index: 1, Command: []byte("set x=1")},
				{Term: 2, Index: 2, Command: []byte("set y=2")},
				{Term: 5, Index: 3, Command: []byte("set z=3")},
			},
		}
		if err := store.Save(state); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded.CurrentTerm != 5 {
			t.Errorf("term: expected 5, got %d", loaded.CurrentTerm)
		}
		if loaded.VotedFor != 2 {
			t.Errorf("votedFor: expected 2, got %d", loaded.VotedFor)
		}
		if len(loaded.Log) != 3 {
			t.Fatalf("log length: expected 3, got %d", len(loaded.Log))
		}
		if string(loaded.Log[0].Command) != "set x=1" {
			t.Errorf("log[0] command: expected 'set x=1', got '%s'", loaded.Log[0].Command)
		}
		if loaded.Log[2].Term != 5 {
			t.Errorf("log[2] term: expected 5, got %d", loaded.Log[2].Term)
		}
	})

	t.Run("SaveTermAndVote", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		if err := store.SaveTermAndVote(3, 1); err != nil {
			t.Fatalf("SaveTermAndVote failed: %v", err)
		}

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if state.CurrentTerm != 3 {
			t.Errorf("term: expected 3, got %d", state.CurrentTerm)
		}
		if state.VotedFor != 1 {
			t.Errorf("votedFor: expected 1, got %d", state.VotedFor)
		}
	})

	t.Run("SaveTermAndVotePreservesLog", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		// First save some log entries.
		err := store.Save(&PersistentState{
			CurrentTerm: 1,
			VotedFor:    -1,
			Log:         []LogEntry{{Term: 1, Index: 1, Command: []byte("cmd1")}},
		})
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Update term and vote — log should be preserved.
		if err := store.SaveTermAndVote(2, 3); err != nil {
			t.Fatalf("SaveTermAndVote failed: %v", err)
		}

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if state.CurrentTerm != 2 {
			t.Errorf("term: expected 2, got %d", state.CurrentTerm)
		}
		if len(state.Log) != 1 {
			t.Fatalf("log should be preserved, got %d entries", len(state.Log))
		}
	})

	t.Run("AppendLogEntries", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		entries1 := []LogEntry{
			{Term: 1, Index: 1, Command: []byte("a")},
			{Term: 1, Index: 2, Command: []byte("b")},
		}
		if err := store.AppendLogEntries(entries1); err != nil {
			t.Fatalf("AppendLogEntries failed: %v", err)
		}

		entries2 := []LogEntry{
			{Term: 2, Index: 3, Command: []byte("c")},
		}
		if err := store.AppendLogEntries(entries2); err != nil {
			t.Fatalf("AppendLogEntries failed: %v", err)
		}

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(state.Log) != 3 {
			t.Fatalf("log length: expected 3, got %d", len(state.Log))
		}
		if state.Log[2].Index != 3 || state.Log[2].Term != 2 {
			t.Errorf("log[2]: expected index=3 term=2, got index=%d term=%d",
				state.Log[2].Index, state.Log[2].Term)
		}
	})

	t.Run("TruncateLog", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		err := store.Save(&PersistentState{
			CurrentTerm: 3,
			VotedFor:    1,
			Log: []LogEntry{
				{Term: 1, Index: 1, Command: []byte("a")},
				{Term: 1, Index: 2, Command: []byte("b")},
				{Term: 2, Index: 3, Command: []byte("c")},
				{Term: 3, Index: 4, Command: []byte("d")},
				{Term: 3, Index: 5, Command: []byte("e")},
			},
		})
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Truncate from index 3 onward.
		if err := store.TruncateLog(3); err != nil {
			t.Fatalf("TruncateLog failed: %v", err)
		}

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(state.Log) != 2 {
			t.Fatalf("log length after truncate: expected 2, got %d", len(state.Log))
		}
		if state.Log[1].Index != 2 {
			t.Errorf("last entry index: expected 2, got %d", state.Log[1].Index)
		}
	})

	t.Run("TruncateLogAll", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		err := store.Save(&PersistentState{
			Log: []LogEntry{
				{Term: 1, Index: 1, Command: []byte("a")},
				{Term: 1, Index: 2, Command: []byte("b")},
			},
		})
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Truncate from index 1 — removes everything.
		if err := store.TruncateLog(1); err != nil {
			t.Fatalf("TruncateLog failed: %v", err)
		}

		state, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(state.Log) != 0 {
			t.Errorf("log should be empty after full truncate, got %d", len(state.Log))
		}
	})

	t.Run("SaveOverwritesPrevious", func(t *testing.T) {
		store := newStore(t)
		defer store.Close()

		state1 := &PersistentState{CurrentTerm: 1, VotedFor: 0, Log: []LogEntry{{Term: 1, Index: 1}}}
		store.Save(state1)

		state2 := &PersistentState{CurrentTerm: 5, VotedFor: 3, Log: []LogEntry{}}
		store.Save(state2)

		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded.CurrentTerm != 5 || loaded.VotedFor != 3 {
			t.Errorf("expected term=5 votedFor=3, got term=%d votedFor=%d",
				loaded.CurrentTerm, loaded.VotedFor)
		}
		if len(loaded.Log) != 0 {
			t.Errorf("expected empty log, got %d entries", len(loaded.Log))
		}
	})
}

// --- MemoryStore tests ---

func newMemoryStore(t *testing.T) Storage {
	return NewMemoryStore()
}

func TestMemoryStore(t *testing.T) {
	storageTestSuite(t, newMemoryStore)
}

func TestMemoryStoreDeepCopy(t *testing.T) {
	store := NewMemoryStore()

	original := &PersistentState{
		CurrentTerm: 1,
		VotedFor:    0,
		Log: []LogEntry{
			{Term: 1, Index: 1, Command: []byte("hello")},
		},
	}
	store.Save(original)

	// Mutate original — should not affect stored state.
	original.CurrentTerm = 99
	original.Log[0].Command[0] = 'X'

	loaded, _ := store.Load()
	if loaded.CurrentTerm != 1 {
		t.Errorf("stored state should not be affected by mutation, got term=%d", loaded.CurrentTerm)
	}
	if string(loaded.Log[0].Command) != "hello" {
		t.Errorf("stored command should not be affected, got '%s'", loaded.Log[0].Command)
	}

	// Mutate loaded — should not affect stored state.
	loaded.Log[0].Command[0] = 'Y'
	loaded2, _ := store.Load()
	if string(loaded2.Log[0].Command) != "hello" {
		t.Errorf("loading again should return clean copy, got '%s'", loaded2.Log[0].Command)
	}
}

// --- FileStore tests ---

func newFileStore(t *testing.T) Storage {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	return store
}

func TestFileStore(t *testing.T) {
	storageTestSuite(t, newFileStore)
}

func TestFileStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	_, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore should create nested directories: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory should have been created")
	}
}

func TestFileStorePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()

	// First instance saves state.
	store1, _ := NewFileStore(dir)
	store1.Save(&PersistentState{
		CurrentTerm: 7,
		VotedFor:    3,
		Log:         []LogEntry{{Term: 7, Index: 1, Command: []byte("persist")}},
	})
	store1.Close()

	// Second instance loads state.
	store2, _ := NewFileStore(dir)
	defer store2.Close()

	state, err := store2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if state.CurrentTerm != 7 {
		t.Errorf("term should persist across instances, got %d", state.CurrentTerm)
	}
	if state.VotedFor != 3 {
		t.Errorf("votedFor should persist, got %d", state.VotedFor)
	}
	if len(state.Log) != 1 || string(state.Log[0].Command) != "persist" {
		t.Errorf("log should persist, got %v", state.Log)
	}
}

func TestFileStoreAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir)
	defer store.Close()

	// Save state — no temp file should remain.
	store.Save(&PersistentState{CurrentTerm: 1})

	tmpFile := filepath.Join(dir, stateFileName+tempSuffix)
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temp file should not remain after successful save")
	}
}
