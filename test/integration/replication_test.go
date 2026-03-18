package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

func TestReplicateEntriesAcrossFiveNodeCluster(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}
	t.Logf("Leader: Node %d in term %d", leader.GetID(), leader.GetCurrentTerm())

	// Submit a command.
	idx, term, isLeader := leader.Submit("set x=1")
	if !isLeader {
		t.Fatal("expected node to be leader")
	}
	if idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
	t.Logf("Submitted 'set x=1' at index %d, term %d", idx, term)

	// Submit more commands.
	leader.Submit("set y=2")
	leader.Submit("set z=3")

	// Wait for replication and commit.
	time.Sleep(500 * time.Millisecond)

	// Verify leader's log has 3 entries.
	if leader.GetLog().Len() != 3 {
		t.Errorf("leader log length: expected 3, got %d", leader.GetLog().Len())
	}

	// Verify leader's commitIndex advanced.
	if leader.GetCommitIndex() < 3 {
		t.Errorf("leader commitIndex should be >= 3, got %d", leader.GetCommitIndex())
	}

	// Verify followers have replicated the entries.
	for _, node := range cluster.Nodes {
		if node.GetID() == leader.GetID() {
			continue
		}
		logLen := node.GetLog().Len()
		if logLen < 3 {
			t.Errorf("follower %d log length: expected >= 3, got %d", node.GetID(), logLen)
		}
	}
}

func TestSubmitOnlyAcceptedByLeader(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	_, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Try to submit on a follower.
	followers := cluster.GetFollowers()
	if len(followers) == 0 {
		t.Fatal("no followers found")
	}

	_, _, isLeader := followers[0].Submit("should fail")
	if isLeader {
		t.Error("follower should not accept submissions")
	}
}

func TestCommitIndexAdvancesWithMajority(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Disconnect 2 nodes — cluster still has majority (3/5).
	followers := cluster.GetFollowers()
	if len(followers) < 2 {
		t.Fatal("not enough followers")
	}
	cluster.DisconnectNode(followers[0].GetID())
	cluster.DisconnectNode(followers[1].GetID())

	leader.Submit("cmd1")

	// Wait for replication to connected followers.
	time.Sleep(500 * time.Millisecond)

	// Leader should still be able to commit with 3/5 majority.
	if leader.GetCommitIndex() < 1 {
		t.Errorf("leader commitIndex should be >= 1 with majority, got %d", leader.GetCommitIndex())
	}
}

func TestReplicateMultipleEntriesInSequence(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit entries one at a time.
	for i := 1; i <= 10; i++ {
		leader.Submit(i)
	}

	// Wait for replication.
	time.Sleep(1 * time.Second)

	// All nodes should have 10 entries.
	for _, node := range cluster.Nodes {
		logLen := node.GetLog().Len()
		if logLen != 10 {
			t.Errorf("node %d log length: expected 10, got %d", node.GetID(), logLen)
		}
	}

	// CommitIndex should be 10.
	if leader.GetCommitIndex() != 10 {
		t.Errorf("leader commitIndex: expected 10, got %d", leader.GetCommitIndex())
	}
}

func TestFollowerCatchesUpAfterReconnection(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Disconnect one follower.
	followers := cluster.GetFollowers()
	if len(followers) == 0 {
		t.Fatal("no followers")
	}
	disconnected := followers[0]
	cluster.DisconnectNode(disconnected.GetID())
	t.Logf("Disconnected node %d", disconnected.GetID())

	// Submit entries while follower is disconnected.
	for i := 1; i <= 5; i++ {
		leader.Submit(i)
	}

	// Wait for replication to connected followers.
	time.Sleep(500 * time.Millisecond)

	// Disconnected follower should have fewer entries.
	if disconnected.GetLog().Len() >= 5 {
		t.Errorf("disconnected follower should have < 5 entries, got %d", disconnected.GetLog().Len())
	}

	// Reconnect the follower.
	cluster.ReconnectNode(disconnected.GetID())
	t.Logf("Reconnected node %d", disconnected.GetID())

	// Wait for catch-up.
	time.Sleep(1 * time.Second)

	// Follower should now have all entries.
	if disconnected.GetLog().Len() < 5 {
		t.Errorf("reconnected follower should have >= 5 entries, got %d", disconnected.GetLog().Len())
	}
}

func TestAppliedEntriesDeliveredViaApplyCh(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit a command.
	leader.Submit("hello")

	// Read from applyCh.
	select {
	case entry := <-leader.ApplyCh():
		if entry.Command != "hello" {
			t.Errorf("expected command 'hello', got %v", entry.Command)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for applied entry on leader's applyCh")
	}
}

func TestLogMatchingProperty(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit several entries.
	for i := 0; i < 5; i++ {
		leader.Submit(i)
	}

	// Wait for full replication.
	time.Sleep(1 * time.Second)

	// Verify Log Matching Property: if two logs contain an entry with the same
	// index and term, then the logs are identical in all entries up through that index.
	leaderLog := leader.GetLog()
	for _, node := range cluster.Nodes {
		if node.GetID() == leader.GetID() {
			continue
		}
		nodeLog := node.GetLog()
		minLen := leaderLog.Len()
		if nodeLog.Len() < minLen {
			minLen = nodeLog.Len()
		}
		for idx := 1; idx <= minLen; idx++ {
			leaderEntry, _ := leaderLog.GetEntry(idx)
			nodeEntry, _ := nodeLog.GetEntry(idx)
			if leaderEntry.Term != nodeEntry.Term {
				t.Errorf("Log Matching violation at index %d: leader term=%d, node %d term=%d",
					idx, leaderEntry.Term, node.GetID(), nodeEntry.Term)
			}
		}
	}
}

func TestNoCommitWithoutMajority(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Disconnect 3 followers — leader + 1 follower = 2/5, no majority.
	followers := cluster.GetFollowers()
	for i := 0; i < 3 && i < len(followers); i++ {
		cluster.DisconnectNode(followers[i].GetID())
	}

	leader.Submit("should not commit")

	// Wait a bit.
	time.Sleep(500 * time.Millisecond)

	// Leader has the entry in its log but should NOT have committed it.
	if leader.GetLog().Len() < 1 {
		t.Error("leader should have the entry in its log")
	}

	// Without majority, commitIndex should not advance for this entry.
	// (It might be 0 or whatever it was before)
	if leader.GetRole() == raft.Leader && leader.GetCommitIndex() >= 1 {
		t.Errorf("should not commit without majority, commitIndex=%d", leader.GetCommitIndex())
	}
}
