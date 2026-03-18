package transport

import (
	"context"
	"testing"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

func TestGRPCTransportRequestVote(t *testing.T) {
	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	// Start node 2's transport first so node 1 can connect to it.
	t2 := NewGRPCTransport(2, "127.0.0.1:0", nil, handler2)
	if err := t2.Start(); err != nil {
		t.Fatalf("failed to start transport 2: %v", err)
	}
	defer t2.Stop()

	// Create node 1's transport with node 2's actual address.
	peerAddrs := map[int]string{2: t2.Addr()}
	t1 := NewGRPCTransport(1, "127.0.0.1:0", peerAddrs, handler1)
	if err := t1.Start(); err != nil {
		t.Fatalf("failed to start transport 1: %v", err)
	}
	defer t1.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &rpc.RequestVoteRequest{
		Term:         1,
		CandidateID:  1,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := t1.SendRequestVote(ctx, 2, req)
	if err != nil {
		t.Fatalf("SendRequestVote failed: %v", err)
	}
	if !resp.VoteGranted {
		t.Error("expected vote granted")
	}
	if handler2.lastVoteReq.CandidateID != 1 {
		t.Errorf("expected CandidateID 1, got %d", handler2.lastVoteReq.CandidateID)
	}
}

func TestGRPCTransportAppendEntries(t *testing.T) {
	handler1 := &stubHandler{nodeID: 1}
	handler2 := &stubHandler{nodeID: 2}

	t2 := NewGRPCTransport(2, "127.0.0.1:0", nil, handler2)
	if err := t2.Start(); err != nil {
		t.Fatalf("failed to start transport 2: %v", err)
	}
	defer t2.Stop()

	peerAddrs := map[int]string{2: t2.Addr()}
	t1 := NewGRPCTransport(1, "127.0.0.1:0", peerAddrs, handler1)
	if err := t1.Start(); err != nil {
		t.Fatalf("failed to start transport 1: %v", err)
	}
	defer t1.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &rpc.AppendEntriesRequest{
		Term:         1,
		LeaderID:     1,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      []rpc.LogEntry{{Term: 1, Index: 1, Command: []byte("cmd1")}},
		LeaderCommit: 0,
	}

	resp, err := t1.SendAppendEntries(ctx, 2, req)
	if err != nil {
		t.Fatalf("SendAppendEntries failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if len(handler2.lastAppendReq.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(handler2.lastAppendReq.Entries))
	}
}

func TestGRPCTransportUnknownPeer(t *testing.T) {
	handler := &stubHandler{nodeID: 1}
	transport := NewGRPCTransport(1, "127.0.0.1:0", map[int]string{}, handler)
	if err := transport.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer transport.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := transport.SendRequestVote(ctx, 99, &rpc.RequestVoteRequest{Term: 1})
	if err == nil {
		t.Error("expected error for unknown peer")
	}
}

func TestGRPCTransportStartStop(t *testing.T) {
	handler := &stubHandler{nodeID: 1}
	transport := NewGRPCTransport(1, "127.0.0.1:0", nil, handler)

	if err := transport.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if err := transport.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}
