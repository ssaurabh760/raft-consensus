package integration

import (
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/pkg/kvstore"
)

func TestKVStoreWithRaftCluster(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	// Create KV stores and appliers for each node.
	stores := make(map[int]*kvstore.Store)
	appliers := make(map[int]*kvstore.Applier)
	for _, node := range cluster.Nodes {
		store := kvstore.NewStore()
		applier := kvstore.NewApplier(store, node.ApplyCh())
		applier.Start()
		stores[node.GetID()] = store
		appliers[node.GetID()] = applier
	}
	defer func() {
		for _, a := range appliers {
			a.Stop()
		}
	}()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit KV commands via the leader.
	putCmd := kvstore.Command{Op: kvstore.OpPut, Key: "name", Value: "alice"}
	data, _ := kvstore.EncodeCommand(putCmd)
	leader.Submit(string(data))

	putCmd2 := kvstore.Command{Op: kvstore.OpPut, Key: "age", Value: "30"}
	data2, _ := kvstore.EncodeCommand(putCmd2)
	leader.Submit(string(data2))

	// Wait for commit and apply.
	time.Sleep(1 * time.Second)

	// Verify the leader's KV store has the values.
	leaderStore := stores[leader.GetID()]
	val, ok := leaderStore.Get("name")
	if !ok || val != "alice" {
		t.Errorf("leader store: expected name='alice', got '%s' (ok=%v)", val, ok)
	}

	val, ok = leaderStore.Get("age")
	if !ok || val != "30" {
		t.Errorf("leader store: expected age='30', got '%s' (ok=%v)", val, ok)
	}

	// Verify followers also applied the entries.
	for _, node := range cluster.Nodes {
		if node.GetID() == leader.GetID() {
			continue
		}
		store := stores[node.GetID()]
		val, ok := store.Get("name")
		if !ok || val != "alice" {
			t.Errorf("node %d store: expected name='alice', got '%s' (ok=%v)",
				node.GetID(), val, ok)
		}
	}
}

func TestKVStoreDeleteOperation(t *testing.T) {
	cluster := NewTestCluster(3)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	stores := make(map[int]*kvstore.Store)
	appliers := make(map[int]*kvstore.Applier)
	for _, node := range cluster.Nodes {
		store := kvstore.NewStore()
		applier := kvstore.NewApplier(store, node.ApplyCh())
		applier.Start()
		stores[node.GetID()] = store
		appliers[node.GetID()] = applier
	}
	defer func() {
		for _, a := range appliers {
			a.Stop()
		}
	}()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Put then Delete.
	putCmd, _ := kvstore.EncodeCommand(kvstore.Command{Op: kvstore.OpPut, Key: "temp", Value: "val"})
	leader.Submit(string(putCmd))
	time.Sleep(500 * time.Millisecond)

	delCmd, _ := kvstore.EncodeCommand(kvstore.Command{Op: kvstore.OpDelete, Key: "temp"})
	leader.Submit(string(delCmd))
	time.Sleep(500 * time.Millisecond)

	// Key should be gone on all nodes.
	for _, node := range cluster.Nodes {
		store := stores[node.GetID()]
		if _, ok := store.Get("temp"); ok {
			t.Errorf("node %d: key 'temp' should be deleted", node.GetID())
		}
	}
}

func TestKVStoreSurvivesLeaderFailure(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	stores := make(map[int]*kvstore.Store)
	appliers := make(map[int]*kvstore.Applier)
	for _, node := range cluster.Nodes {
		store := kvstore.NewStore()
		applier := kvstore.NewApplier(store, node.ApplyCh())
		applier.Start()
		stores[node.GetID()] = store
		appliers[node.GetID()] = applier
	}
	defer func() {
		for _, a := range appliers {
			a.Stop()
		}
	}()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("leader election failed: %v", err)
	}

	// Submit initial data.
	cmd, _ := kvstore.EncodeCommand(kvstore.Command{Op: kvstore.OpPut, Key: "x", Value: "1"})
	leader.Submit(string(cmd))
	time.Sleep(500 * time.Millisecond)

	// Disconnect the leader.
	oldLeaderID := leader.GetID()
	cluster.DisconnectNode(oldLeaderID)

	// Wait for new leader.
	time.Sleep(1 * time.Second)
	newLeader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("re-election failed: %v", err)
	}

	// Submit more data on new leader.
	cmd2, _ := kvstore.EncodeCommand(kvstore.Command{Op: kvstore.OpPut, Key: "y", Value: "2"})
	newLeader.Submit(string(cmd2))
	time.Sleep(500 * time.Millisecond)

	// Verify the new leader's store has both entries.
	newLeaderStore := stores[newLeader.GetID()]
	if val, ok := newLeaderStore.Get("x"); !ok || val != "1" {
		t.Errorf("new leader should have x=1, got '%s' (ok=%v)", val, ok)
	}
	if val, ok := newLeaderStore.Get("y"); !ok || val != "2" {
		t.Errorf("new leader should have y=2, got '%s' (ok=%v)", val, ok)
	}
}
