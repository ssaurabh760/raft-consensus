package raft

import (
	"testing"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

func newTestConfig(nodeID int, peers []int) *Config {
	cfg := DefaultConfig()
	cfg.NodeID = nodeID
	cfg.Peers = peers
	return cfg
}

func newTestNode(nodeID int, peers []int) *RaftNode {
	return NewRaftNode(newTestConfig(nodeID, peers))
}

// --- RequestVote Tests ---

func TestRequestVoteGranted(t *testing.T) {
	node := newTestNode(1, []int{2, 3})

	req := &rpc.RequestVoteRequest{
		Term:         1,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote to be granted")
	}
	if resp.Term != 1 {
		t.Errorf("expected term 1, got %d", resp.Term)
	}
}

func TestRequestVoteDeniedStaleTerm(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	// Manually set node to term 5.
	node.currentTerm = 5

	req := &rpc.RequestVoteRequest{
		Term:         3, // stale term
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VoteGranted {
		t.Error("should not grant vote for stale term")
	}
	if resp.Term != 5 {
		t.Errorf("expected term 5, got %d", resp.Term)
	}
}

func TestRequestVoteDeniedAlreadyVoted(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.votedFor = 3 // already voted for node 3

	req := &rpc.RequestVoteRequest{
		Term:         1,
		CandidateID:  2, // different candidate
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VoteGranted {
		t.Error("should not grant vote when already voted for another candidate")
	}
}

func TestRequestVoteGrantedToSameCandidate(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.votedFor = 2 // already voted for candidate 2

	req := &rpc.RequestVoteRequest{
		Term:         1,
		CandidateID:  2, // same candidate
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("should grant vote to the same candidate again")
	}
}

func TestRequestVoteDeniedLogNotUpToDate_Term(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	// Node has log entries at term 3.
	node.log.Append(LogEntry{Term: 3, Command: "cmd1"})

	req := &rpc.RequestVoteRequest{
		Term:         4,
		CandidateID:  2,
		LastLogIndex: 1,
		LastLogTerm:  2, // candidate's last log term is older
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VoteGranted {
		t.Error("should not grant vote when candidate's log has older last term")
	}
}

func TestRequestVoteDeniedLogNotUpToDate_Index(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	// Node has 3 entries at term 2.
	node.log.Append(LogEntry{Term: 2, Command: "cmd1"})
	node.log.Append(LogEntry{Term: 2, Command: "cmd2"})
	node.log.Append(LogEntry{Term: 2, Command: "cmd3"})

	req := &rpc.RequestVoteRequest{
		Term:         3,
		CandidateID:  2,
		LastLogIndex: 2, // candidate only has 2 entries
		LastLogTerm:  2, // same term
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VoteGranted {
		t.Error("should not grant vote when candidate's log is shorter with same last term")
	}
}

func TestRequestVoteGrantedHigherTerm(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.votedFor = 3 // voted for someone in term 1

	req := &rpc.RequestVoteRequest{
		Term:         5, // much higher term
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("should grant vote for higher term (votedFor resets)")
	}
	if node.currentTerm != 5 {
		t.Errorf("expected node term to update to 5, got %d", node.currentTerm)
	}
	if node.role != Follower {
		t.Errorf("expected node to be follower, got %s", node.role)
	}
}

func TestRequestVoteUpdatesTermOnHigherTerm(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 2
	node.role = Candidate

	req := &rpc.RequestVoteRequest{
		Term:         10,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := node.HandleRequestVote(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Term != 10 {
		t.Errorf("expected response term 10, got %d", resp.Term)
	}
	if !resp.VoteGranted {
		t.Error("expected vote granted after term update")
	}
	if node.role != Follower {
		t.Errorf("expected step-down to follower, got %s", node.role)
	}
}

// --- AppendEntries Tests ---

func TestAppendEntriesHeartbeat(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil, // heartbeat
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("heartbeat should succeed")
	}
	if node.leaderID != 2 {
		t.Errorf("expected leaderID 2, got %d", node.leaderID)
	}
}

func TestAppendEntriesDeniedStaleTerm(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 5

	req := &rpc.AppendEntriesRequest{
		Term:     3, // stale
		LeaderID: 2,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success {
		t.Error("should reject stale term")
	}
	if resp.Term != 5 {
		t.Errorf("expected term 5, got %d", resp.Term)
	}
}

func TestAppendEntriesLogConsistencyCheck(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 2
	node.log.Append(LogEntry{Term: 1, Command: "cmd1"})

	// PrevLogIndex=2 but log only has 1 entry.
	req := &rpc.AppendEntriesRequest{
		Term:         2,
		LeaderID:     2,
		PrevLogIndex: 2,
		PrevLogTerm:  1,
		Entries:      nil,
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success {
		t.Error("should fail log consistency check")
	}
}

func TestAppendEntriesLogTermMismatch(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 3
	node.log.Append(LogEntry{Term: 1, Command: "cmd1"})
	node.log.Append(LogEntry{Term: 1, Command: "cmd2"})

	// PrevLogTerm doesn't match.
	req := &rpc.AppendEntriesRequest{
		Term:         3,
		LeaderID:     2,
		PrevLogIndex: 2,
		PrevLogTerm:  2, // mismatch — actual is term 1
		Entries:      nil,
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success {
		t.Error("should fail on term mismatch at prevLogIndex")
	}
	// Log should be truncated at the conflict.
	if node.log.Len() != 1 {
		t.Errorf("expected log length 1 after truncation, got %d", node.log.Len())
	}
}

func TestAppendEntriesAppendsNewEntries(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries: []rpc.LogEntry{
			{Term: 1, Index: 1, Command: []byte("cmd1")},
			{Term: 1, Index: 2, Command: []byte("cmd2")},
		},
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("should succeed")
	}
	if node.log.Len() != 2 {
		t.Errorf("expected 2 log entries, got %d", node.log.Len())
	}
}

func TestAppendEntriesUpdatesCommitIndex(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.log.Append(LogEntry{Term: 1, Command: "cmd1"})
	node.log.Append(LogEntry{Term: 1, Command: "cmd2"})

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 2,
		PrevLogTerm:  1,
		Entries:      nil,
		LeaderCommit: 2,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("should succeed")
	}
	if node.commitIndex != 2 {
		t.Errorf("expected commitIndex 2, got %d", node.commitIndex)
	}
}

func TestAppendEntriesCommitIndexCappedByLog(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.log.Append(LogEntry{Term: 1, Command: "cmd1"})

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 1,
		PrevLogTerm:  1,
		Entries:      nil,
		LeaderCommit: 10, // higher than log length
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("should succeed")
	}
	if node.commitIndex != 1 {
		t.Errorf("expected commitIndex 1 (capped by log), got %d", node.commitIndex)
	}
}

func TestAppendEntriesStepsDownCandidate(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 2
	node.role = Candidate

	req := &rpc.AppendEntriesRequest{
		Term:         2,
		LeaderID:     2,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("should succeed")
	}
	if node.role != Follower {
		t.Errorf("candidate should step down to follower, got %s", node.role)
	}
	if node.leaderID != 2 {
		t.Errorf("expected leaderID 2, got %d", node.leaderID)
	}
}

func TestAppendEntriesHigherTermUpdatesNode(t *testing.T) {
	node := newTestNode(1, []int{2, 3})
	node.currentTerm = 1
	node.role = Leader

	req := &rpc.AppendEntriesRequest{
		Term:         5,
		LeaderID:     3,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	}

	resp, err := node.HandleAppendEntries(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("should succeed")
	}
	if node.currentTerm != 5 {
		t.Errorf("expected term 5, got %d", node.currentTerm)
	}
	if node.role != Follower {
		t.Errorf("should step down to follower, got %s", node.role)
	}
}

// --- Log up-to-date check ---

func TestIsLogUpToDate(t *testing.T) {
	node := newTestNode(1, []int{2, 3})

	tests := []struct {
		name           string
		nodeLog        []LogEntry
		candidateIndex int
		candidateTerm  int
		expected       bool
	}{
		{
			name:           "both empty",
			nodeLog:        nil,
			candidateIndex: 0,
			candidateTerm:  0,
			expected:       true,
		},
		{
			name:           "candidate has higher term",
			nodeLog:        []LogEntry{{Term: 1, Command: "a"}},
			candidateIndex: 1,
			candidateTerm:  2,
			expected:       true,
		},
		{
			name:           "candidate has lower term",
			nodeLog:        []LogEntry{{Term: 2, Command: "a"}},
			candidateIndex: 1,
			candidateTerm:  1,
			expected:       false,
		},
		{
			name:           "same term candidate longer",
			nodeLog:        []LogEntry{{Term: 1, Command: "a"}},
			candidateIndex: 2,
			candidateTerm:  1,
			expected:       true,
		},
		{
			name:           "same term candidate shorter",
			nodeLog:        []LogEntry{{Term: 1, Command: "a"}, {Term: 1, Command: "b"}},
			candidateIndex: 1,
			candidateTerm:  1,
			expected:       false,
		},
		{
			name:           "same term same length",
			nodeLog:        []LogEntry{{Term: 1, Command: "a"}},
			candidateIndex: 1,
			candidateTerm:  1,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node.log = NewMemoryLog()
			for _, entry := range tt.nodeLog {
				node.log.Append(entry)
			}

			result := node.isLogUpToDate(tt.candidateIndex, tt.candidateTerm)
			if result != tt.expected {
				t.Errorf("isLogUpToDate(%d, %d) = %v, want %v",
					tt.candidateIndex, tt.candidateTerm, result, tt.expected)
			}
		})
	}
}

// --- Config tests ---

func TestConfigQuorumSize(t *testing.T) {
	tests := []struct {
		peers    []int
		expected int
	}{
		{peers: []int{2, 3, 4, 5}, expected: 3},       // 5-node cluster
		{peers: []int{2, 3}, expected: 2},             // 3-node cluster
		{peers: []int{2, 3, 4, 5, 6, 7}, expected: 4}, // 7-node cluster
		{peers: []int{}, expected: 1},                 // single node
	}

	for _, tt := range tests {
		cfg := newTestConfig(1, tt.peers)
		if got := cfg.QuorumSize(); got != tt.expected {
			t.Errorf("QuorumSize() for %d nodes = %d, want %d",
				cfg.ClusterSize(), got, tt.expected)
		}
	}
}

// --- NewRaftNode tests ---

func TestNewRaftNodeDefaults(t *testing.T) {
	node := newTestNode(1, []int{2, 3, 4, 5})

	if node.GetID() != 1 {
		t.Errorf("expected ID 1, got %d", node.GetID())
	}
	if node.GetRole() != Follower {
		t.Errorf("expected Follower, got %s", node.GetRole())
	}
	if node.GetCurrentTerm() != 0 {
		t.Errorf("expected term 0, got %d", node.GetCurrentTerm())
	}
	if node.GetLeaderID() != -1 {
		t.Errorf("expected leaderID -1, got %d", node.GetLeaderID())
	}
	if node.GetCommitIndex() != 0 {
		t.Errorf("expected commitIndex 0, got %d", node.GetCommitIndex())
	}
	if node.GetLastApplied() != 0 {
		t.Errorf("expected lastApplied 0, got %d", node.GetLastApplied())
	}
}
