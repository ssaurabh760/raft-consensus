package kvstore

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

// HTTPServer provides an HTTP API for the KV store backed by Raft.
type HTTPServer struct {
	node  *raft.RaftNode
	store *Store
	addr  string
}

// NewHTTPServer creates a new HTTP server for the KV store.
func NewHTTPServer(node *raft.RaftNode, store *Store, addr string) *HTTPServer {
	return &HTTPServer{
		node:  node,
		store: store,
		addr:  addr,
	}
}

// Start begins serving HTTP requests. Non-blocking, returns the server.
func (s *HTTPServer) Start() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/kv/", s.handleKV)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/cluster", s.handleCluster)

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return server
}

// handleKV handles PUT /kv/{key}, GET /kv/{key}, DELETE /kv/{key}.
func (s *HTTPServer) handleKV(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/kv/")
	if key == "" {
		http.Error(w, `{"error":"key is required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, key)
	case http.MethodPut:
		s.handlePut(w, r, key)
	case http.MethodDelete:
		s.handleDelete(w, key)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleGet reads a value from the local store.
func (s *HTTPServer) handleGet(w http.ResponseWriter, key string) {
	val, ok := s.store.Get(key)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]string{"error": "key not found"})
		return
	}
	writeJSON(w, map[string]string{"key": key, "value": val})
}

// handlePut writes a key-value pair via Raft consensus.
func (s *HTTPServer) handlePut(w http.ResponseWriter, r *http.Request, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	cmd := Command{Op: OpPut, Key: key, Value: req.Value}
	cmdBytes, err := EncodeCommand(cmd)
	if err != nil {
		http.Error(w, `{"error":"failed to encode command"}`, http.StatusInternalServerError)
		return
	}

	index, term, isLeader := s.node.Submit(string(cmdBytes))
	if !isLeader {
		// Forward info about the leader.
		leaderID := s.node.GetLeaderID()
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]interface{}{
			"error":    "not the leader",
			"leaderID": leaderID,
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]interface{}{
		"status": "accepted",
		"index":  index,
		"term":   term,
	})
}

// handleDelete removes a key via Raft consensus.
func (s *HTTPServer) handleDelete(w http.ResponseWriter, key string) {
	cmd := Command{Op: OpDelete, Key: key}
	cmdBytes, err := EncodeCommand(cmd)
	if err != nil {
		http.Error(w, `{"error":"failed to encode command"}`, http.StatusInternalServerError)
		return
	}

	index, term, isLeader := s.node.Submit(string(cmdBytes))
	if !isLeader {
		leaderID := s.node.GetLeaderID()
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]interface{}{
			"error":    "not the leader",
			"leaderID": leaderID,
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]interface{}{
		"status": "accepted",
		"index":  index,
		"term":   term,
	})
}

// handleStatus returns the node's current status.
func (s *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"nodeID":      s.node.GetID(),
		"role":        s.node.GetRole().String(),
		"term":        s.node.GetCurrentTerm(),
		"leaderID":    s.node.GetLeaderID(),
		"logLength":   s.node.GetLog().Len(),
		"commitIndex": s.node.GetCommitIndex(),
		"lastApplied": s.node.GetLastApplied(),
		"storeSize":   s.store.Len(),
	})
}

// handleCluster returns cluster membership info.
func (s *HTTPServer) handleCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"nodeID":   s.node.GetID(),
		"leaderID": s.node.GetLeaderID(),
		"role":     s.node.GetRole().String(),
		"term":     s.node.GetCurrentTerm(),
	})
}

// Addr returns the configured listen address.
func (s *HTTPServer) Addr() string {
	return s.addr
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
