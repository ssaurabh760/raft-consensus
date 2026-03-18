package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

func TestHeartbeatPreventsElection(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	initialTerm := leader.GetCurrentTerm()
	leaderID := leader.GetID()
	t.Logf("Leader: Node %d in term %d", leaderID, initialTerm)

	// Wait significantly longer than election timeout — with heartbeats,
	// no new election should happen.
	time.Sleep(1 * time.Second)

	// Verify leader hasn't changed.
	if leader.GetRole() != raft.Leader {
		t.Error("leader should still be leader after 1 second with heartbeats")
	}
	if leader.GetCurrentTerm() != initialTerm {
		t.Errorf("term should not have changed, expected %d got %d",
			initialTerm, leader.GetCurrentTerm())
	}

	// Verify exactly one leader.
	leaderCount := cluster.CountLeaders()
	if leaderCount != 1 {
		t.Errorf("expected exactly 1 leader, got %d", leaderCount)
	}

	// All followers should still be followers.
	for _, node := range cluster.Nodes {
		if node.GetID() == leaderID {
			continue
		}
		if node.GetRole() != raft.Follower {
			t.Errorf("node %d should be follower, got %s", node.GetID(), node.GetRole())
		}
	}
}

func TestHeartbeatStopsWhenLeaderPartitioned(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	leaderID := leader.GetID()
	t.Logf("Initial leader: Node %d", leaderID)

	// Partition the leader from all followers.
	cluster.DisconnectNode(leaderID)

	// Wait for followers to notice heartbeats stopped and elect a new leader.
	time.Sleep(1 * time.Second)

	// A new leader should have been elected among the remaining nodes.
	newLeaderFound := false
	for _, node := range cluster.Nodes {
		if node.GetID() != leaderID && node.GetRole() == raft.Leader {
			newLeaderFound = true
			t.Logf("New leader: Node %d in term %d", node.GetID(), node.GetCurrentTerm())
			break
		}
	}
	if !newLeaderFound {
		t.Error("a new leader should have been elected after partitioning the old leader")
	}
}

func TestHeartbeatKeepsFollowersAlive(t *testing.T) {
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

	// Wait for several heartbeat cycles (5x the election timeout).
	time.Sleep(1500 * time.Millisecond)

	// The cluster should still be stable — same leader, same term.
	if leader.GetRole() != raft.Leader {
		t.Error("leader should remain leader with heartbeats active")
	}
	if leader.GetCurrentTerm() != initialTerm {
		t.Errorf("term should remain %d, got %d", initialTerm, leader.GetCurrentTerm())
	}

	// Verify all followers are still following.
	followers := cluster.GetFollowers()
	if len(followers) != 2 {
		t.Errorf("expected 2 followers, got %d", len(followers))
	}
}
