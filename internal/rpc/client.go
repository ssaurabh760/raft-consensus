package rpc

import (
	"context"
	"fmt"
)

// Client sends RPCs to a target Raft node via the transport layer.
type Client struct {
	// sender is a function that sends RPCs. It's injected to decouple
	// the client from the transport implementation.
	voteSender   func(ctx context.Context, target int, req *RequestVoteRequest) (*RequestVoteResponse, error)
	appendSender func(ctx context.Context, target int, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
}

// ClientTransport defines what the RPC client needs from the transport layer.
type ClientTransport interface {
	SendRequestVote(ctx context.Context, target int, req *RequestVoteRequest) (*RequestVoteResponse, error)
	SendAppendEntries(ctx context.Context, target int, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
}

// NewClient creates a new RPC client backed by the given transport.
func NewClient(transport ClientTransport) *Client {
	return &Client{
		voteSender:   transport.SendRequestVote,
		appendSender: transport.SendAppendEntries,
	}
}

// RequestVote sends a RequestVote RPC to the target node.
func (c *Client) RequestVote(ctx context.Context, target int, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	if c.voteSender == nil {
		return nil, fmt.Errorf("rpc client: vote sender not configured")
	}
	return c.voteSender(ctx, target, req)
}

// AppendEntries sends an AppendEntries RPC to the target node.
func (c *Client) AppendEntries(ctx context.Context, target int, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	if c.appendSender == nil {
		return nil, fmt.Errorf("rpc client: append sender not configured")
	}
	return c.appendSender(ctx, target, req)
}
