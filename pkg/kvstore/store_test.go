package kvstore

import (
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store.Len() != 0 {
		t.Errorf("new store should be empty, got %d", store.Len())
	}
}

func TestPutAndGet(t *testing.T) {
	store := NewStore()

	store.Apply(Command{Op: OpPut, Key: "name", Value: "alice"})

	val, ok := store.Get("name")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "alice" {
		t.Errorf("expected 'alice', got '%s'", val)
	}
}

func TestGetMissing(t *testing.T) {
	store := NewStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestDelete(t *testing.T) {
	store := NewStore()
	store.Apply(Command{Op: OpPut, Key: "key", Value: "val"})
	store.Apply(Command{Op: OpDelete, Key: "key"})

	_, ok := store.Get("key")
	if ok {
		t.Error("key should be deleted")
	}
	if store.Len() != 0 {
		t.Errorf("store should be empty after delete, got %d", store.Len())
	}
}

func TestDeleteMissing(t *testing.T) {
	store := NewStore()
	// Deleting a non-existent key should not error.
	err := store.Apply(Command{Op: OpDelete, Key: "ghost"})
	if err != nil {
		t.Errorf("delete of missing key should not error: %v", err)
	}
}

func TestOverwrite(t *testing.T) {
	store := NewStore()
	store.Apply(Command{Op: OpPut, Key: "x", Value: "1"})
	store.Apply(Command{Op: OpPut, Key: "x", Value: "2"})

	val, ok := store.Get("x")
	if !ok || val != "2" {
		t.Errorf("expected '2', got '%s'", val)
	}
	if store.Len() != 1 {
		t.Errorf("overwrite should not increase count, got %d", store.Len())
	}
}

func TestUnknownOp(t *testing.T) {
	store := NewStore()
	err := store.Apply(Command{Op: "invalid", Key: "x"})
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}

func TestEncodeDecodeCommand(t *testing.T) {
	original := Command{Op: OpPut, Key: "hello", Value: "world"}
	data, err := EncodeCommand(original)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := DecodeCommand(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.Op != original.Op || decoded.Key != original.Key || decoded.Value != original.Value {
		t.Errorf("decoded command doesn't match: %+v vs %+v", decoded, original)
	}
}

func TestApplyBytes(t *testing.T) {
	store := NewStore()

	cmd := Command{Op: OpPut, Key: "test", Value: "123"}
	data, _ := EncodeCommand(cmd)

	err := store.ApplyBytes(data)
	if err != nil {
		t.Fatalf("ApplyBytes failed: %v", err)
	}

	val, ok := store.Get("test")
	if !ok || val != "123" {
		t.Errorf("expected '123', got '%s'", val)
	}
}

func TestApplyBytesInvalidJSON(t *testing.T) {
	store := NewStore()
	// Non-JSON should be treated as no-op.
	err := store.ApplyBytes([]byte("not json"))
	if err != nil {
		t.Errorf("non-JSON should be no-op, got error: %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	store := NewStore()
	store.Apply(Command{Op: OpPut, Key: "a", Value: "1"})
	store.Apply(Command{Op: OpPut, Key: "b", Value: "2"})

	snap := store.Snapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 entries, got %d", len(snap))
	}
	if snap["a"] != "1" || snap["b"] != "2" {
		t.Errorf("snapshot mismatch: %v", snap)
	}

	// Mutating snapshot should not affect store.
	snap["a"] = "modified"
	val, _ := store.Get("a")
	if val != "1" {
		t.Error("mutating snapshot should not affect store")
	}
}

func TestKeys(t *testing.T) {
	store := NewStore()
	store.Apply(Command{Op: OpPut, Key: "x", Value: "1"})
	store.Apply(Command{Op: OpPut, Key: "y", Value: "2"})
	store.Apply(Command{Op: OpPut, Key: "z", Value: "3"})

	keys := store.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestMultipleOperations(t *testing.T) {
	store := NewStore()

	// Sequence of mixed operations.
	ops := []Command{
		{Op: OpPut, Key: "a", Value: "1"},
		{Op: OpPut, Key: "b", Value: "2"},
		{Op: OpPut, Key: "c", Value: "3"},
		{Op: OpDelete, Key: "b"},
		{Op: OpPut, Key: "a", Value: "10"},
		{Op: OpPut, Key: "d", Value: "4"},
	}
	for _, op := range ops {
		store.Apply(op)
	}

	if store.Len() != 3 {
		t.Errorf("expected 3 keys, got %d", store.Len())
	}
	if val, _ := store.Get("a"); val != "10" {
		t.Errorf("a should be '10', got '%s'", val)
	}
	if _, ok := store.Get("b"); ok {
		t.Error("b should be deleted")
	}
}
