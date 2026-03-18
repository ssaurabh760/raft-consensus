package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

func TestFiveNodeLeaderElection(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	// Wait for a leader to be elected.
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	t.Logf("Leader elected: Node %d in term %d", leader.GetID(), leader.GetCurrentTerm())

	// Verify exactly one leader.
	leaderCount := cluster.CountLeaders()
	if leaderCount != 1 {
		t.Errorf("expected exactly 1 leader, got %d", leaderCount)
	}

	// Verify leader term > 0.
	if leader.GetCurrentTerm() == 0 {
		t.Error("leader term should be > 0")
	}

	// Verify all other nodes are followers.
	followers := cluster.GetFollowers()
	if len(followers) != 4 {
		t.Errorf("expected 4 followers, got %d", len(followers))
	}

	// Give heartbeats time to propagate.
	time.Sleep(200 * time.Millisecond)

	// Verify all followers know the leader.
	for _, f := range cluster.GetFollowers() {
		if f.GetLeaderID() != leader.GetID() {
			t.Errorf("follower %d has leaderID %d, expected %d",
				f.GetID(), f.GetLeaderID(), leader.GetID())
		}
	}
}

func TestThreeNodeLeaderElection(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	t.Logf("Leader elected: Node %d in term %d", leader.GetID(), leader.GetCurrentTerm())

	if cluster.CountLeaders() != 1 {
		t.Errorf("expected exactly 1 leader, got %d", cluster.CountLeaders())
	}
}

func TestReElectionAfterLeaderFailure(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	// Wait for initial leader.
	leader1, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("initial leader election failed: %v", err)
	}
	leader1ID := leader1.GetID()
	leader1Term := leader1.GetCurrentTerm()
	t.Logf("Initial leader: Node %d in term %d", leader1ID, leader1Term)

	// Kill the leader by stopping it and disconnecting it.
	leader1.Stop()
	cluster.DisconnectNode(leader1ID)

	// Wait for a new leader to be elected from the remaining 4 nodes.
	time.Sleep(500 * time.Millisecond) // Allow election timeout to expire

	var leader2 *raft.RaftNode
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for leader2 == nil {
		select {
		case <-deadline:
			t.Fatal("no new leader elected after leader failure")
		case <-tick.C:
			for _, node := range cluster.Nodes {
				if node.GetID() != leader1ID && node.GetRole() == raft.Leader {
					leader2 = node
					break
				}
			}
		}
	}

	t.Logf("New leader: Node %d in term %d", leader2.GetID(), leader2.GetCurrentTerm())

	// New leader should be different.
	if leader2.GetID() == leader1ID {
		t.Error("new leader should be a different node")
	}

	// New leader should have a higher term.
	if leader2.GetCurrentTerm() <= leader1Term {
		t.Errorf("new leader term %d should be > old leader term %d",
			leader2.GetCurrentTerm(), leader1Term)
	}
}

func TestElectionSafetyOneLeaderPerTerm(t *testing.T) {
	// Run multiple elections and verify at most one leader per term.
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	// Wait for leader to stabilize.
	_, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Check: at most one leader per term.
	leadersByTerm := make(map[int]int) // term -> leader ID
	for _, node := range cluster.Nodes {
		if node.GetRole() == raft.Leader {
			term := node.GetCurrentTerm()
			if existingLeader, ok := leadersByTerm[term]; ok {
				if existingLeader != node.GetID() {
					t.Errorf("Election safety violation: two leaders in term %d: nodes %d and %d",
						term, existingLeader, node.GetID())
				}
			}
			leadersByTerm[term] = node.GetID()
		}
	}
}

func TestNoElectionWithStableLeader(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	initialTerm := leader.GetCurrentTerm()

	// Wait a while — term should not increase with a stable leader.
	time.Sleep(1 * time.Second)

	// Leader should still be the same.
	if leader.GetRole() != raft.Leader {
		t.Error("leader should still be leader after stable period")
	}

	// Term should not have increased significantly (at most by 1 due to timing).
	if leader.GetCurrentTerm() > initialTerm+1 {
		t.Errorf("term increased from %d to %d during stable period",
			initialTerm, leader.GetCurrentTerm())
	}
}
