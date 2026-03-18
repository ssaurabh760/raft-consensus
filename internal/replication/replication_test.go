package replication

import (
	"testing"
)

func TestNewReplicationState(t *testing.T) {
	rs := NewReplicationState(5)
	if rs.NextIndex != 6 {
		t.Errorf("expected NextIndex 6, got %d", rs.NextIndex)
	}
	if rs.MatchIndex != 0 {
		t.Errorf("expected MatchIndex 0, got %d", rs.MatchIndex)
	}
}

func TestReplicationStateHandleSuccess(t *testing.T) {
	rs := NewReplicationState(5) // NextIndex=6, MatchIndex=0

	// Sent 3 entries with prevLogIndex=5
	rs.HandleSuccess(5, 3)
	if rs.MatchIndex != 8 {
		t.Errorf("expected MatchIndex 8, got %d", rs.MatchIndex)
	}
	if rs.NextIndex != 9 {
		t.Errorf("expected NextIndex 9, got %d", rs.NextIndex)
	}
}

func TestReplicationStateHandleSuccessHeartbeat(t *testing.T) {
	rs := NewReplicationState(5)
	rs.MatchIndex = 5

	// Heartbeat: 0 entries sent
	rs.HandleSuccess(5, 0)
	if rs.MatchIndex != 5 {
		t.Errorf("expected MatchIndex 5 (unchanged), got %d", rs.MatchIndex)
	}
	if rs.NextIndex != 6 {
		t.Errorf("expected NextIndex 6, got %d", rs.NextIndex)
	}
}

func TestReplicationStateHandleFailure(t *testing.T) {
	rs := NewReplicationState(5) // NextIndex=6

	rs.HandleFailure()
	if rs.NextIndex != 5 {
		t.Errorf("expected NextIndex 5 after failure, got %d", rs.NextIndex)
	}

	rs.HandleFailure()
	if rs.NextIndex != 4 {
		t.Errorf("expected NextIndex 4, got %d", rs.NextIndex)
	}
}

func TestReplicationStateHandleFailureFloor(t *testing.T) {
	rs := &ReplicationState{NextIndex: 1, MatchIndex: 0}

	rs.HandleFailure()
	if rs.NextIndex != 1 {
		t.Errorf("NextIndex should not go below 1, got %d", rs.NextIndex)
	}
}

func TestReplicationStateSequentialUpdates(t *testing.T) {
	rs := NewReplicationState(0) // Empty log, NextIndex=1

	// First batch: send 2 entries
	rs.HandleSuccess(0, 2)
	if rs.MatchIndex != 2 || rs.NextIndex != 3 {
		t.Errorf("after first batch: MatchIndex=%d, NextIndex=%d", rs.MatchIndex, rs.NextIndex)
	}

	// Second batch: send 3 more entries
	rs.HandleSuccess(2, 3)
	if rs.MatchIndex != 5 || rs.NextIndex != 6 {
		t.Errorf("after second batch: MatchIndex=%d, NextIndex=%d", rs.MatchIndex, rs.NextIndex)
	}
}

func TestReplicationStateBacktrackThenSuccess(t *testing.T) {
	rs := NewReplicationState(10) // NextIndex=11

	// Several failures (log backtracking)
	rs.HandleFailure() // NextIndex=10
	rs.HandleFailure() // NextIndex=9
	rs.HandleFailure() // NextIndex=8

	if rs.NextIndex != 8 {
		t.Errorf("expected NextIndex 8 after 3 failures, got %d", rs.NextIndex)
	}

	// Success after backtracking: sent entries from index 8-10
	rs.HandleSuccess(7, 3)
	if rs.MatchIndex != 10 || rs.NextIndex != 11 {
		t.Errorf("after success: MatchIndex=%d, NextIndex=%d", rs.MatchIndex, rs.NextIndex)
	}
}
