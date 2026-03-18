package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
)

// GRPCTransport implements the Transport interface using HTTP/JSON.
// This provides a gRPC-like transport without requiring protoc code generation.
// It will be migrated to real gRPC once protobuf generation is set up.
type GRPCTransport struct {
	mu       sync.RWMutex
	nodeID   int
	addr     string
	handler  RPCHandler
	server   *http.Server
	listener net.Listener

	// peerAddrs maps node IDs to their network addresses.
	peerAddrs map[int]string
	// client is reused for connection pooling.
	client *http.Client
}

// NewGRPCTransport creates a new HTTP-based transport.
func NewGRPCTransport(nodeID int, addr string, peerAddrs map[int]string, handler RPCHandler) *GRPCTransport {
	return &GRPCTransport{
		nodeID:    nodeID,
		addr:      addr,
		handler:   handler,
		peerAddrs: peerAddrs,
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
			},
		},
	}
}

// Start begins listening for incoming RPCs over HTTP.
func (t *GRPCTransport) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/raft/request-vote", t.handleRequestVote)
	mux.HandleFunc("/raft/append-entries", t.handleAppendEntries)

	t.server = &http.Server{Handler: mux}

	var err error
	t.listener, err = net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("transport: failed to listen on %s: %w", t.addr, err)
	}

	go func() {
		if err := t.server.Serve(t.listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("transport: server error on node %d: %v\n", t.nodeID, err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the transport.
func (t *GRPCTransport) Stop() error {
	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

// Addr returns the actual address the transport is listening on.
func (t *GRPCTransport) Addr() string {
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return t.addr
}

// SendRequestVote sends a RequestVote RPC to the target node.
func (t *GRPCTransport) SendRequestVote(ctx context.Context, target int, req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error) {
	t.mu.RLock()
	addr, ok := t.peerAddrs[target]
	t.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("transport: unknown peer %d", target)
	}

	var resp rpc.RequestVoteResponse
	if err := t.doPost(ctx, addr, "/raft/request-vote", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendAppendEntries sends an AppendEntries RPC to the target node.
func (t *GRPCTransport) SendAppendEntries(ctx context.Context, target int, req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error) {
	t.mu.RLock()
	addr, ok := t.peerAddrs[target]
	t.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("transport: unknown peer %d", target)
	}

	var resp rpc.AppendEntriesResponse
	if err := t.doPost(ctx, addr, "/raft/append-entries", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// doPost sends an HTTP POST request with JSON encoding.
func (t *GRPCTransport) doPost(ctx context.Context, addr, path string, reqBody, respBody interface{}) error {
	url := fmt.Sprintf("http://%s%s", addr, path)

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(json.NewEncoder(pw).Encode(reqBody))
	}()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return fmt.Errorf("transport: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("transport: request to %s failed: %w", url, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("transport: RPC to %s returned status %d: %s", url, httpResp.StatusCode, string(body))
	}

	if err := json.NewDecoder(httpResp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("transport: failed to decode response from %s: %w", url, err)
	}
	return nil
}

// handleRequestVote handles incoming RequestVote RPCs.
func (t *GRPCTransport) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	var req rpc.RequestVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := t.handler.HandleRequestVote(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("handler error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAppendEntries handles incoming AppendEntries RPCs.
func (t *GRPCTransport) handleAppendEntries(w http.ResponseWriter, r *http.Request) {
	var req rpc.AppendEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := t.handler.HandleAppendEntries(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("handler error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
