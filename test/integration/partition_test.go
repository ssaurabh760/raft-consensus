package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

func TestSingleNodeFailureAndRecovery(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit an entry.
	leader.Submit("before-failure")
	time.Sleep(500 * time.Millisecond)

	// Disconnect a follower.
	followers := cluster.GetFollowers()
	if len(followers) == 0 {
		t.Fatal("no followers")
	}
	failedID := followers[0].GetID()
	cluster.DisconnectNode(failedID)
	t.Logf("Disconnected node %d", failedID)

	// Submit more entries — should still work (4/5 nodes).
	leader.Submit("during-failure")
	time.Sleep(500 * time.Millisecond)

	if leader.GetCommitIndex() < 2 {
		t.Errorf("leader should commit with 4/5 nodes, commitIndex=%d", leader.GetCommitIndex())
	}

	// Reconnect the failed node.
	cluster.ReconnectNode(failedID)
	t.Logf("Reconnected node %d", failedID)
	time.Sleep(1 * time.Second)

	// Failed node should catch up.
	node := cluster.GetNode(failedID)
	if node.GetLog().Len() < 2 {
		t.Errorf("recovered node should have >= 2 entries, got %d", node.GetLog().Len())
	}
}

func TestLeaderFailureTriggersReElection(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	oldLeaderID := leader.GetID()
	oldTerm := leader.GetCurrentTerm()
	t.Logf("Initial leader: Node %d, term %d", oldLeaderID, oldTerm)

	// Kill the leader.
	cluster.DisconnectNode(oldLeaderID)

	// Wait for re-election.
	time.Sleep(1 * time.Second)

	// A new leader should emerge among the remaining nodes.
	newLeaderFound := false
	for _, node := range cluster.Nodes {
		if node.GetID() != oldLeaderID && node.GetRole() == raft.Leader {
			newLeaderFound = true
			t.Logf("New leader: Node %d, term %d", node.GetID(), node.GetCurrentTerm())
			if node.GetCurrentTerm() <= oldTerm {
				t.Errorf("new leader's term (%d) should be > old term (%d)",
					node.GetCurrentTerm(), oldTerm)
			}
			break
		}
	}
	if !newLeaderFound {
		t.Error("expected a new leader to be elected after leader failure")
	}
}

func TestMinorityPartitionCannotCommit(t *testing.T) {
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

	// Partition the leader into a minority (disconnect 3 out of 4 followers).
	followers := cluster.GetFollowers()
	disconnected := 0
	for _, f := range followers {
		if disconnected >= 3 {
			break
		}
		cluster.DisconnectNode(f.GetID())
		disconnected++
	}
	t.Logf("Partitioned leader %d into minority (only 1 follower reachable)", leaderID)

	// Try to submit — leader has the entry but cannot commit.
	leader.Submit("should-not-commit")
	time.Sleep(500 * time.Millisecond)

	// Leader should either have stepped down or not committed the entry.
	if leader.GetRole() == raft.Leader {
		// If still leader (briefly), commitIndex should not advance.
		if leader.GetCommitIndex() >= 1 {
			t.Errorf("leader in minority should not be able to commit, commitIndex=%d",
				leader.GetCommitIndex())
		}
	}
}

func TestMajorityPartitionElectsNewLeader(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	oldLeaderID := leader.GetID()

	// Disconnect the leader — the majority (4 nodes) should elect a new leader.
	cluster.DisconnectNode(oldLeaderID)
	t.Logf("Disconnected leader (node %d)", oldLeaderID)

	time.Sleep(1 * time.Second)

	// Verify a new leader exists among remaining nodes.
	newLeaderCount := 0
	for _, node := range cluster.Nodes {
		if node.GetID() != oldLeaderID && node.GetRole() == raft.Leader {
			newLeaderCount++
		}
	}
	if newLeaderCount != 1 {
		t.Errorf("expected exactly 1 new leader, got %d", newLeaderCount)
	}
}

func TestSplitBrainPrevention(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	_ = leader

	// Let the cluster stabilize.
	time.Sleep(500 * time.Millisecond)

	// At no point should there be two leaders in the same term.
	// Check across several intervals.
	for i := 0; i < 10; i++ {
		leadersByTerm := make(map[int][]int) // term -> list of leader node IDs
		for _, node := range cluster.Nodes {
			if node.GetRole() == raft.Leader {
				term := node.GetCurrentTerm()
				leadersByTerm[term] = append(leadersByTerm[term], node.GetID())
			}
		}
		for term, leaders := range leadersByTerm {
			if len(leaders) > 1 {
				t.Errorf("split brain! Multiple leaders in term %d: %v", term, leaders)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestPartitionHealingOldLeaderBecomesFollower(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	oldLeaderID := leader.GetID()
	t.Logf("Initial leader: Node %d", oldLeaderID)

	// Partition the leader.
	cluster.DisconnectNode(oldLeaderID)

	// Wait for new leader.
	time.Sleep(1 * time.Second)

	var newLeader *raft.RaftNode
	for _, node := range cluster.Nodes {
		if node.GetID() != oldLeaderID && node.GetRole() == raft.Leader {
			newLeader = node
			break
		}
	}
	if newLeader == nil {
		t.Fatal("expected new leader after partition")
	}
	t.Logf("New leader: Node %d in term %d", newLeader.GetID(), newLeader.GetCurrentTerm())

	// Submit entries on new leader.
	newLeader.Submit("after-partition")
	time.Sleep(500 * time.Millisecond)

	// Heal the partition — old leader should step down and become follower.
	cluster.ReconnectNode(oldLeaderID)
	time.Sleep(1 * time.Second)

	oldLeaderNode := cluster.GetNode(oldLeaderID)
	if oldLeaderNode.GetRole() == raft.Leader {
		t.Errorf("old leader (node %d) should have stepped down to follower", oldLeaderID)
	}
}

func TestLogReconciliationAfterPartition(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit some entries.
	leader.Submit("entry-1")
	leader.Submit("entry-2")
	time.Sleep(500 * time.Millisecond)

	// Disconnect a follower.
	followers := cluster.GetFollowers()
	if len(followers) == 0 {
		t.Fatal("no followers")
	}
	partitionedID := followers[0].GetID()
	cluster.DisconnectNode(partitionedID)

	// Submit more entries while node is partitioned.
	leader.Submit("entry-3")
	leader.Submit("entry-4")
	leader.Submit("entry-5")
	time.Sleep(500 * time.Millisecond)

	// Verify partitioned node is behind.
	partitioned := cluster.GetNode(partitionedID)
	if partitioned.GetLog().Len() >= 5 {
		t.Errorf("partitioned node should have < 5 entries, got %d", partitioned.GetLog().Len())
	}

	// Reconnect — log should be reconciled.
	cluster.ReconnectNode(partitionedID)
	time.Sleep(1 * time.Second)

	if partitioned.GetLog().Len() < 5 {
		t.Errorf("reconciled node should have >= 5 entries, got %d", partitioned.GetLog().Len())
	}
}
