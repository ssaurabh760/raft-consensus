package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

// MockNetwork simulates a network of Raft nodes for deterministic testing.
// It allows controlled message delivery, network partitions, and message loss.
type MockNetwork struct {
	mu         sync.RWMutex
	transports map[int]*MockTransport
	// disconnected tracks pairs of nodes that cannot communicate.
	// Key format: "from-to"
	disconnected map[string]bool
}

// NewMockNetwork creates a new simulated network.
func NewMockNetwork() *MockNetwork {
	return &MockNetwork{
		transports:   make(map[int]*MockTransport),
		disconnected: make(map[string]bool),
	}
}

// AddNode registers a mock transport for the given node ID.
func (n *MockNetwork) AddNode(nodeID int, handler RPCHandler) *MockTransport {
	n.mu.Lock()
	defer n.mu.Unlock()

	t := &MockTransport{
		nodeID:  nodeID,
		handler: handler,
		network: n,
	}
	n.transports[nodeID] = t
	return t
}

// Disconnect simulates a network partition between two nodes.
func (n *MockNetwork) Disconnect(from, to int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.disconnected[fmt.Sprintf("%d-%d", from, to)] = true
	n.disconnected[fmt.Sprintf("%d-%d", to, from)] = true
}

// Reconnect restores connectivity between two nodes.
func (n *MockNetwork) Reconnect(from, to int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.disconnected, fmt.Sprintf("%d-%d", from, to))
	delete(n.disconnected, fmt.Sprintf("%d-%d", to, from))
}

// DisconnectNode isolates a node from all other nodes.
func (n *MockNetwork) DisconnectNode(nodeID int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for id := range n.transports {
		if id != nodeID {
			n.disconnected[fmt.Sprintf("%d-%d", nodeID, id)] = true
			n.disconnected[fmt.Sprintf("%d-%d", id, nodeID)] = true
		}
	}
}

// ReconnectNode restores a node's connectivity to all other nodes.
func (n *MockNetwork) ReconnectNode(nodeID int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for id := range n.transports {
		if id != nodeID {
			delete(n.disconnected, fmt.Sprintf("%d-%d", nodeID, id))
			delete(n.disconnected, fmt.Sprintf("%d-%d", id, nodeID))
		}
	}
}

// isConnected checks if two nodes can communicate.
func (n *MockNetwork) isConnected(from, to int) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return !n.disconnected[fmt.Sprintf("%d-%d", from, to)]
}

// getTransport returns the mock transport for a node.
func (n *MockNetwork) getTransport(nodeID int) (*MockTransport, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	t, ok := n.transports[nodeID]
	return t, ok
}

// MockTransport implements the Transport interface using the MockNetwork for testing.
type MockTransport struct {
	nodeID  int
	handler RPCHandler
	network *MockNetwork
}

// SendRequestVote sends a RequestVote RPC through the mock network.
func (t *MockTransport) SendRequestVote(ctx context.Context, target int, req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !t.network.isConnected(t.nodeID, target) {
		return nil, fmt.Errorf("transport: node %d cannot reach node %d (network partition)", t.nodeID, target)
	}

	targetTransport, ok := t.network.getTransport(target)
	if !ok {
		return nil, fmt.Errorf("transport: node %d not found in network", target)
	}

	return targetTransport.handler.HandleRequestVote(req)
}

// SendAppendEntries sends an AppendEntries RPC through the mock network.
func (t *MockTransport) SendAppendEntries(ctx context.Context, target int, req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !t.network.isConnected(t.nodeID, target) {
		return nil, fmt.Errorf("transport: node %d cannot reach node %d (network partition)", t.nodeID, target)
	}

	targetTransport, ok := t.network.getTransport(target)
	if !ok {
		return nil, fmt.Errorf("transport: node %d not found in network", target)
	}

	return targetTransport.handler.HandleAppendEntries(req)
}

// Start is a no-op for the mock transport.
func (t *MockTransport) Start() error {
	return nil
}

// Stop is a no-op for the mock transport.
func (t *MockTransport) Stop() error {
	return nil
}
