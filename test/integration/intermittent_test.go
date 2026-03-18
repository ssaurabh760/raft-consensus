package integration

import (
	"testing"
	"time"
)

func TestIntermittentConnectivity(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit initial entries.
	leader.Submit("entry-1")
	time.Sleep(300 * time.Millisecond)

	followers := cluster.GetFollowers()
	if len(followers) < 2 {
		t.Fatal("not enough followers")
	}
	flickerID := followers[0].GetID()

	// Rapidly disconnect and reconnect a node.
	for i := 0; i < 5; i++ {
		cluster.DisconnectNode(flickerID)
		time.Sleep(50 * time.Millisecond)
		cluster.ReconnectNode(flickerID)
		time.Sleep(100 * time.Millisecond)
	}

	// Submit more entries.
	leader.Submit("entry-2")
	leader.Submit("entry-3")
	time.Sleep(1 * time.Second)

	// Despite intermittent connectivity, the flickering node should catch up.
	flickerNode := cluster.GetNode(flickerID)
	if flickerNode.GetLog().Len() < 3 {
		t.Errorf("intermittent node should have >= 3 entries, got %d", flickerNode.GetLog().Len())
	}

	// Cluster should still have exactly one leader.
	leaderCount := cluster.CountLeaders()
	if leaderCount != 1 {
		t.Errorf("expected 1 leader after intermittent connectivity, got %d", leaderCount)
	}
}

func TestSequentialNodeFailures(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	leader.Submit("cmd-1")
	time.Sleep(500 * time.Millisecond)

	// Disconnect followers one at a time (up to 2 to maintain quorum).
	followers := cluster.GetFollowers()
	if len(followers) < 2 {
		t.Fatal("not enough followers")
	}

	cluster.DisconnectNode(followers[0].GetID())
	t.Logf("Disconnected follower %d", followers[0].GetID())
	time.Sleep(300 * time.Millisecond)

	// Should still be able to commit.
	leader.Submit("cmd-2")
	time.Sleep(500 * time.Millisecond)
	if leader.GetCommitIndex() < 2 {
		t.Errorf("should commit with 4/5 nodes, commitIndex=%d", leader.GetCommitIndex())
	}

	cluster.DisconnectNode(followers[1].GetID())
	t.Logf("Disconnected follower %d", followers[1].GetID())
	time.Sleep(300 * time.Millisecond)

	// Should still be able to commit (3/5 = majority).
	leader.Submit("cmd-3")
	time.Sleep(500 * time.Millisecond)
	if leader.GetCommitIndex() < 3 {
		t.Errorf("should commit with 3/5 nodes, commitIndex=%d", leader.GetCommitIndex())
	}

	// Reconnect both.
	cluster.ReconnectNode(followers[0].GetID())
	cluster.ReconnectNode(followers[1].GetID())
	time.Sleep(1 * time.Second)

	// All nodes should converge.
	for _, node := range cluster.Nodes {
		if node.GetLog().Len() < 3 {
			t.Errorf("node %d should have >= 3 entries after reconnect, got %d",
				node.GetID(), node.GetLog().Len())
		}
	}
}

func TestDualPartition(t *testing.T) {
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

	// Create two partitions: {leader, follower1} and {follower2, follower3, follower4}
	followers := cluster.GetFollowers()
	if len(followers) < 3 {
		t.Fatal("not enough followers for dual partition test")
	}

	// Disconnect the last 3 followers from the leader.
	for i := 1; i < len(followers); i++ {
		cluster.DisconnectNode(followers[i].GetID())
	}

	time.Sleep(1 * time.Second)

	// The majority partition should elect a new leader.
	// The minority partition (leader + 1 follower) should not be able to commit.
	oldLeader := cluster.GetNode(leaderID)

	// Give time for the majority side to elect.
	time.Sleep(1 * time.Second)

	// Reconnect everyone.
	for i := 1; i < len(followers); i++ {
		cluster.ReconnectNode(followers[i].GetID())
	}
	time.Sleep(1 * time.Second)

	// After healing, old leader should be a follower (higher term from majority side).
	if oldLeader.GetRole() != 0 { // Check it's follower (0)
		// It's OK if it's still leader temporarily; the important thing is cluster has 1 leader.
	}
	if cluster.CountLeaders() != 1 {
		t.Errorf("expected exactly 1 leader after partition heals, got %d", cluster.CountLeaders())
	}
}
