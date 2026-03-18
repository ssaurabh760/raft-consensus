// Package replication implements log replication logic for the Raft consensus algorithm.
package replication

// ReplicationState tracks the replication progress for a single follower.
type ReplicationState struct {
	NextIndex  int // index of the next log entry to send to this follower
	MatchIndex int // index of highest log entry known to be replicated
}

// NewReplicationState creates a new replication state initialized for a follower.
// nextIndex is initialized to leader's last log index + 1.
func NewReplicationState(lastLogIndex int) *ReplicationState {
	return &ReplicationState{
		NextIndex:  lastLogIndex + 1,
		MatchIndex: 0,
	}
}

// HandleSuccess updates state after a successful AppendEntries response.
// prevLogIndex is the prevLogIndex sent in the request, entriesLen is the number of entries sent.
func (rs *ReplicationState) HandleSuccess(prevLogIndex, entriesLen int) {
	newMatchIndex := prevLogIndex + entriesLen
	if newMatchIndex > rs.MatchIndex {
		rs.MatchIndex = newMatchIndex
	}
	rs.NextIndex = newMatchIndex + 1
}

// HandleFailure updates state after a failed AppendEntries response.
// Decrements nextIndex for log backtracking.
func (rs *ReplicationState) HandleFailure() {
	if rs.NextIndex > 1 {
		rs.NextIndex--
	}
}
