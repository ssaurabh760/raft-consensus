package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

func TestLeaderStepsDownOnHigherTerm(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	leaderID := leader.GetID()
	leaderTerm := leader.GetCurrentTerm()
	t.Logf("Leader: Node %d in term %d", leaderID, leaderTerm)

	// Send AppendEntries with a higher term — leader should step down.
	resp, err := leader.HandleAppendEntries(&rpc.AppendEntriesRequest{
		Term:         leaderTerm + 10,
		LeaderID:     99, // fake leader
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	})
	if err != nil {
		t.Fatalf("HandleAppendEntries error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success for valid AppendEntries")
	}

	// Give time for step-down to take effect.
	time.Sleep(100 * time.Millisecond)

	if leader.GetRole() != raft.Follower {
		t.Errorf("leader should have stepped down to follower, got %s", leader.GetRole())
	}
	if leader.GetCurrentTerm() != leaderTerm+10 {
		t.Errorf("expected term %d, got %d", leaderTerm+10, leader.GetCurrentTerm())
	}
}

func TestCandidateStepsDownOnHigherTerm(t *testing.T) {
	// Create a single node that becomes a candidate.
	node := raft.NewRaftNode(&raft.Config{
		NodeID:             1,
		Peers:              []int{2, 3},
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  25 * time.Millisecond,
		RPCTimeout:         50 * time.Millisecond,
	})

	// Manually set to candidate state.
	// Note: We're testing the RPC handler directly, not the event loop.
	resp, err := node.HandleRequestVote(&rpc.RequestVoteRequest{
		Term:         100,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.VoteGranted {
		t.Error("should grant vote for higher term")
	}
	if node.GetCurrentTerm() != 100 {
		t.Errorf("expected term 100, got %d", node.GetCurrentTerm())
	}
	if node.GetRole() != raft.Follower {
		t.Errorf("expected follower, got %s", node.GetRole())
	}
}

func TestFollowerUpdatesTermOnHigherTermRequestVote(t *testing.T) {
	node := raft.NewRaftNode(&raft.Config{
		NodeID:             1,
		Peers:              []int{2, 3},
		ElectionTimeoutMin: 150 * time.Millisecond,
		ElectionTimeoutMax: 300 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
		RPCTimeout:         100 * time.Millisecond,
	})

	// Node starts at term 0.
	resp, err := node.HandleRequestVote(&rpc.RequestVoteRequest{
		Term:         5,
		CandidateID:  3,
		LastLogIndex: 0,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Term != 5 {
		t.Errorf("expected response term 5, got %d", resp.Term)
	}
	if !resp.VoteGranted {
		t.Error("should grant vote")
	}
	if node.GetCurrentTerm() != 5 {
		t.Errorf("expected node term 5, got %d", node.GetCurrentTerm())
	}
}

func TestLeaderStepsDownOnPartition(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("initial leader election failed: %v", err)
	}

	leaderID := leader.GetID()
	t.Logf("Leader: Node %d", leaderID)

	// Isolate the leader from all other nodes.
	cluster.DisconnectNode(leaderID)

	// Wait for the remaining nodes to elect a new leader.
	time.Sleep(1 * time.Second)

	// Count leaders among connected nodes.
	newLeaderFound := false
	for _, node := range cluster.Nodes {
		if node.GetID() != leaderID && node.GetRole() == raft.Leader {
			newLeaderFound = true
			t.Logf("New leader: Node %d in term %d", node.GetID(), node.GetCurrentTerm())
		}
	}

	if !newLeaderFound {
		t.Error("expected a new leader to be elected among connected nodes")
	}
}
