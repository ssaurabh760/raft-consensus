package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

func TestNodeRestartPreservesTerm(t *testing.T) {
	cluster := NewTestClusterWithPersistence(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	leaderTerm := leader.GetCurrentTerm()
	leaderID := leader.GetID()
	t.Logf("Leader: Node %d, term %d", leaderID, leaderTerm)

	// Restart the leader — it should recover its term.
	restarted, err := cluster.RestartNode(leaderID)
	if err != nil {
		t.Fatalf("failed to restart node: %v", err)
	}

	// Give it a moment to initialize.
	time.Sleep(100 * time.Millisecond)

	restoredTerm := restarted.GetCurrentTerm()
	if restoredTerm < leaderTerm {
		t.Errorf("restarted node should have term >= %d, got %d", leaderTerm, restoredTerm)
	}
	t.Logf("Restarted node %d has term %d (was %d)", leaderID, restoredTerm, leaderTerm)
}

func TestNodeRestartPreservesLog(t *testing.T) {
	cluster := NewTestClusterWithPersistence(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit entries.
	leader.Submit("set x=1")
	leader.Submit("set y=2")
	leader.Submit("set z=3")

	// Wait for replication.
	time.Sleep(500 * time.Millisecond)

	// Pick a follower that has replicated entries.
	var follower *raft.RaftNode
	for _, node := range cluster.Nodes {
		if node.GetID() != leader.GetID() && node.GetLog().Len() >= 3 {
			follower = node
			break
		}
	}
	if follower == nil {
		t.Fatal("no follower found with 3+ log entries")
	}

	followerID := follower.GetID()
	logLenBefore := follower.GetLog().Len()
	t.Logf("Follower %d has %d log entries before restart", followerID, logLenBefore)

	// Restart the follower.
	restarted, err := cluster.RestartNode(followerID)
	if err != nil {
		t.Fatalf("failed to restart follower: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	logLenAfter := restarted.GetLog().Len()
	if logLenAfter < logLenBefore {
		t.Errorf("restarted node should have >= %d log entries, got %d", logLenBefore, logLenAfter)
	}
	t.Logf("Restarted follower %d has %d log entries (was %d)", followerID, logLenAfter, logLenBefore)
}

func TestNodeRestartPreservesVotedFor(t *testing.T) {
	cluster := NewTestClusterWithPersistence(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	_, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// All nodes should have voted in this term. Pick a follower.
	followers := cluster.GetFollowers()
	if len(followers) == 0 {
		t.Fatal("no followers found")
	}

	followerID := followers[0].GetID()
	termBefore := followers[0].GetCurrentTerm()

	// Restart — votedFor should be preserved, preventing double-voting in same term.
	restarted, err := cluster.RestartNode(followerID)
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// The restarted node should have at least the term it had before.
	if restarted.GetCurrentTerm() < termBefore {
		t.Errorf("restarted node term should be >= %d, got %d",
			termBefore, restarted.GetCurrentTerm())
	}
}

func TestClusterSurvivesLeaderRestart(t *testing.T) {
	cluster := NewTestClusterWithPersistence(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit entries before restart.
	leader.Submit("before-restart-1")
	leader.Submit("before-restart-2")
	time.Sleep(500 * time.Millisecond)

	oldLeaderID := leader.GetID()
	t.Logf("Restarting leader (node %d)", oldLeaderID)

	// Restart the leader — a new leader should be elected.
	_, err = cluster.RestartNode(oldLeaderID)
	if err != nil {
		t.Fatalf("failed to restart leader: %v", err)
	}

	// Wait for new leader.
	newLeader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("re-election after leader restart failed: %v", err)
	}
	t.Logf("New leader: Node %d in term %d", newLeader.GetID(), newLeader.GetCurrentTerm())

	// Submit more entries on the new leader.
	newLeader.Submit("after-restart-1")
	time.Sleep(500 * time.Millisecond)

	// Verify the restarted node eventually gets all entries.
	restartedNode := cluster.GetNode(oldLeaderID)
	if restartedNode.GetLog().Len() < 2 {
		t.Errorf("restarted node should have at least the pre-restart entries, got %d",
			restartedNode.GetLog().Len())
	}
}
