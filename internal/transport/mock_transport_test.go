package transport

import (
	"context"
	"testing"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

// stubHandler is a simple RPCHandler for testing.
type stubHandler struct {
	nodeID          int
	voteResponse    *rpc.RequestVoteResponse
	appendResponse  *rpc.AppendEntriesResponse
	lastVoteReq     *rpc.RequestVoteRequest
	lastAppendReq   *rpc.AppendEntriesRequest
}

func (h *stubHandler) HandleRequestVote(req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error) {
	h.lastVoteReq = req
	if h.voteResponse != nil {
		return h.voteResponse, nil
	}
	return &rpc.RequestVoteResponse{Term: req.Term, VoteGranted: true}, nil
}

func (h *stubHandler) HandleAppendEntries(req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error) {
	h.lastAppendReq = req
	if h.appendResponse != nil {
		return h.appendResponse, nil
	}
	return &rpc.AppendEntriesResponse{Term: req.Term, Success: true}, nil
}

func TestMockNetworkRequestVote(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	t1 := network.AddNode(1, handler1)
	network.AddNode(2, handler2)

	req := &rpc.RequestVoteRequest{
		Term:         1,
		CandidateID:  1,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := t1.SendRequestVote(context.Background(), 2, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote to be granted")
	}
	if handler2.lastVoteReq.CandidateID != 1 {
		t.Errorf("expected CandidateID 1, got %d", handler2.lastVoteReq.CandidateID)
	}
}

func TestMockNetworkAppendEntries(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	t1 := network.AddNode(1, handler1)
	network.AddNode(2, handler2)

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     1,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      []rpc.LogEntry{{Term: 1, Index: 1, Command: []byte("set x=1")}},
		LeaderCommit: 0,
	}

	resp, err := t1.SendAppendEntries(context.Background(), 2, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if len(handler2.lastAppendReq.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(handler2.lastAppendReq.Entries))
	}
}

func TestMockNetworkDisconnect(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	t1 := network.AddNode(1, handler1)
	network.AddNode(2, handler2)

	// Disconnect nodes 1 and 2
	network.Disconnect(1, 2)

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}
	_, err := t1.SendRequestVote(context.Background(), 2, req)
	if err == nil {
		t.Error("expected error when sending to disconnected node")
	}

	// Reconnect and verify communication works again
	network.Reconnect(1, 2)
	resp, err := t1.SendRequestVote(context.Background(), 2, req)
	if err != nil {
		t.Fatalf("unexpected error after reconnect: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote granted after reconnect")
	}
}

func TestMockNetworkDisconnectNode(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}
	handler3 := &stubHandler{nodeID: 3}

	t1 := network.AddNode(1, handler1)
	t2 := network.AddNode(2, handler2)
	network.AddNode(3, handler3)

	// Isolate node 3 from all others
	network.DisconnectNode(3)

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}

	// Node 1 -> Node 3 should fail
	_, err := t1.SendRequestVote(context.Background(), 3, req)
	if err == nil {
		t.Error("expected error when sending to isolated node")
	}

	// Node 2 -> Node 3 should fail
	_, err = t2.SendRequestVote(context.Background(), 3, req)
	if err == nil {
		t.Error("expected error when sending to isolated node")
	}

	// Node 1 -> Node 2 should still work
	resp, err := t1.SendRequestVote(context.Background(), 2, req)
	if err != nil {
		t.Fatalf("unexpected error between connected nodes: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote granted between connected nodes")
	}

	// Reconnect node 3 and verify
	network.ReconnectNode(3)
	resp, err = t1.SendRequestVote(context.Background(), 3, req)
	if err != nil {
		t.Fatalf("unexpected error after reconnecting node: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote granted after reconnect")
	}
}

func TestMockNetworkUnknownNode(t *testing.T) {
	network := NewMockNetwork()
	handler1 := &stubHandler{nodeID: 1}
	t1 := network.AddNode(1, handler1)

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}
	_, err := t1.SendRequestVote(context.Background(), 99, req)
	if err == nil {
		t.Error("expected error when sending to unknown node")
	}
}

func TestMockNetworkCancelledContext(t *testing.T) {
	network := NewMockNetwork()
	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}
	t1 := network.AddNode(1, handler1)
	network.AddNode(2, handler2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}
	_, err := t1.SendRequestVote(ctx, 2, req)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestMockTransportStartStop(t *testing.T) {
	network := NewMockNetwork()
	handler := &stubHandler{nodeID: 1}
	transport := network.AddNode(1, handler)

	if err := transport.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	if err := transport.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}
}

func TestMockNetworkCustomResponse(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{
		nodeID: 2,
		voteResponse: &rpc.RequestVoteResponse{Term: 5, VoteGranted: false},
	}

	t1 := network.AddNode(1, handler1)
	network.AddNode(2, handler2)

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}
	resp, err := t1.SendRequestVote(context.Background(), 2, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VoteGranted {
		t.Error("expected vote denied")
	}
	if resp.Term != 5 {
		t.Errorf("expected term 5, got %d", resp.Term)
	}
}

func TestMockNetworkBidirectionalDisconnect(t *testing.T) {
	network := NewMockNetwork()

	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	t1 := network.AddNode(1, handler1)
	t2 := network.AddNode(2, handler2)

	network.Disconnect(1, 2)

	req := &rpc.RequestVoteRequest{Term: 1, CandidateID: 1}

	// Both directions should fail
	_, err := t1.SendRequestVote(context.Background(), 2, req)
	if err == nil {
		t.Error("expected error 1->2")
	}
	_, err = t2.SendRequestVote(context.Background(), 1, req)
	if err == nil {
		t.Error("expected error 2->1")
	}
}
