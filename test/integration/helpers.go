// Package integration provides test utilities for Raft integration tests.
package integration

import (
	"fmt"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/transport"
)

// TestCluster manages a cluster of Raft nodes for integration testing.
type TestCluster struct {
	Nodes   []*raft.RaftNode
	Network *transport.MockNetwork
	nodeIDs []int
}

// NewTestCluster creates a new test cluster with the given number of nodes.
func NewTestCluster(numNodes int) *TestCluster {
	network := transport.NewMockNetwork()
	nodes := make([]*raft.RaftNode, numNodes)
	nodeIDs := make([]int, numNodes)

	for i := 0; i < numNodes; i++ {
		nodeIDs[i] = i + 1
	}

	for i := 0; i < numNodes; i++ {
		peers := make([]int, 0, numNodes-1)
		for j := 0; j < numNodes; j++ {
			if i != j {
				peers = append(peers, nodeIDs[j])
			}
		}

		cfg := &raft.Config{
			NodeID:             nodeIDs[i],
			Peers:              peers,
			ElectionTimeoutMin: 150 * time.Millisecond,
			ElectionTimeoutMax: 300 * time.Millisecond,
			HeartbeatInterval:  50 * time.Millisecond,
			RPCTimeout:         100 * time.Millisecond,
		}

		node := raft.NewRaftNode(cfg)
		nodes[i] = node
	}

	// Register each node with the mock network and set transport.
	for i, node := range nodes {
		t := network.AddNode(nodeIDs[i], node)
		node.SetTransport(t)
	}

	return &TestCluster{
		Nodes:   nodes,
		Network: network,
		nodeIDs: nodeIDs,
	}
}

// Start starts all nodes in the cluster.
func (c *TestCluster) Start() error {
	for _, node := range c.Nodes {
		if err := node.Start(); err != nil {
			return fmt.Errorf("failed to start node %d: %w", node.GetID(), err)
		}
	}
	return nil
}

// Stop stops all nodes in the cluster.
func (c *TestCluster) Stop() {
	for _, node := range c.Nodes {
		node.Stop()
	}
}

// WaitForLeader waits for a leader to be elected and returns it.
// Returns an error if no leader is found within the timeout.
func (c *TestCluster) WaitForLeader(timeout time.Duration) (*raft.RaftNode, error) {
	deadline := time.After(timeout)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("no leader elected within %v", timeout)
		case <-tick.C:
			for _, node := range c.Nodes {
				if node.GetRole() == raft.Leader {
					return node, nil
				}
			}
		}
	}
}

// GetLeader returns the current leader, or nil if none.
func (c *TestCluster) GetLeader() *raft.RaftNode {
	for _, node := range c.Nodes {
		if node.GetRole() == raft.Leader {
			return node
		}
	}
	return nil
}

// GetFollowers returns all nodes that are currently followers.
func (c *TestCluster) GetFollowers() []*raft.RaftNode {
	var followers []*raft.RaftNode
	for _, node := range c.Nodes {
		if node.GetRole() == raft.Follower {
			followers = append(followers, node)
		}
	}
	return followers
}

// CountLeaders returns the number of nodes that believe they are the leader.
func (c *TestCluster) CountLeaders() int {
	count := 0
	for _, node := range c.Nodes {
		if node.GetRole() == raft.Leader {
			count++
		}
	}
	return count
}

// GetNode returns the node with the given ID, or nil if not found.
func (c *TestCluster) GetNode(id int) *raft.RaftNode {
	for _, node := range c.Nodes {
		if node.GetID() == id {
			return node
		}
	}
	return nil
}

// DisconnectNode isolates a node from the rest of the cluster.
func (c *TestCluster) DisconnectNode(id int) {
	c.Network.DisconnectNode(id)
}

// ReconnectNode restores a node's connectivity.
func (c *TestCluster) ReconnectNode(id int) {
	c.Network.ReconnectNode(id)
}
