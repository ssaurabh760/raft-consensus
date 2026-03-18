// Package transport defines the network transport interface for Raft inter-node communication.
package transport

import (
	"context"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

// Transport defines the interface for sending RPCs between Raft nodes.
// Implementations can use gRPC, TCP, or mock transports for testing.
type Transport interface {
	// SendRequestVote sends a RequestVote RPC to the target node.
	SendRequestVote(ctx context.Context, target int, req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error)

	// SendAppendEntries sends an AppendEntries RPC to the target node.
	SendAppendEntries(ctx context.Context, target int, req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error)

	// Start begins listening for incoming RPCs.
	Start() error

	// Stop gracefully shuts down the transport.
	Stop() error
}

// RPCHandler processes incoming RPCs. The Raft node implements this interface.
type RPCHandler interface {
	// HandleRequestVote processes an incoming RequestVote RPC.
	HandleRequestVote(req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error)

	// HandleAppendEntries processes an incoming AppendEntries RPC.
	HandleAppendEntries(req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error)
}
