package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/persistence"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/transport"
	"github.com/saurabhsrivastava/raft-consensus-go/pkg/kvstore"
)

func main() {
	nodeID := flag.Int("id", 0, "Node ID (required)")
	listenAddr := flag.String("listen", "", "RPC listen address (e.g., :9001)")
	httpAddr := flag.String("http", "", "HTTP API address (e.g., :8001)")
	peersFlag := flag.String("peers", "", "Comma-separated peer addresses (id=addr, e.g., 2=localhost:9002,3=localhost:9003)")
	dataDir := flag.String("data", "", "Data directory for persistence (empty = no persistence)")
	flag.Parse()

	if *nodeID == 0 {
		fmt.Fprintln(os.Stderr, "Error: -id is required")
		flag.Usage()
		os.Exit(1)
	}
	if *listenAddr == "" {
		fmt.Fprintln(os.Stderr, "Error: -listen is required")
		flag.Usage()
		os.Exit(1)
	}
	if *httpAddr == "" {
		fmt.Fprintln(os.Stderr, "Error: -http is required")
		flag.Usage()
		os.Exit(1)
	}

	// Parse peers.
	peerIDs, peerAddrs := parsePeers(*peersFlag)

	cfg := &raft.Config{
		NodeID:             *nodeID,
		Peers:              peerIDs,
		ElectionTimeoutMin: 150 * time.Millisecond,
		ElectionTimeoutMax: 300 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
		RPCTimeout:         100 * time.Millisecond,
		DataDir:            *dataDir,
		ListenAddr:         *listenAddr,
		HTTPAddr:           *httpAddr,
	}

	node := raft.NewRaftNode(cfg)

	// Set up gRPC transport.
	grpcTransport := transport.NewGRPCTransport(*nodeID, *listenAddr, peerAddrs, node)
	node.SetTransport(grpcTransport)

	// Set up persistence.
	if *dataDir != "" {
		store, err := persistence.NewFileStore(*dataDir)
		if err != nil {
			log.Fatalf("Failed to create file store: %v", err)
		}
		node.SetStorage(store)
	}

	// Start transport.
	if err := grpcTransport.Start(); err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}

	// Start Raft node.
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	// Set up KV store and applier.
	kvStore := kvstore.NewStore()
	applier := kvstore.NewApplier(kvStore, node.ApplyCh())
	applier.Start()

	// Start HTTP API.
	httpServer := kvstore.NewHTTPServer(node, kvStore, *httpAddr)
	srv := httpServer.Start()

	log.Printf("Raft node %d started: RPC=%s HTTP=%s", *nodeID, *listenAddr, *httpAddr)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("Shutting down node %d...", *nodeID)
	applier.Stop()
	node.Stop()
	grpcTransport.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Printf("Node %d shutdown complete", *nodeID)
}

// parsePeers parses "id=addr,id=addr" format into peer IDs and address map.
func parsePeers(s string) ([]int, map[int]string) {
	if s == "" {
		return nil, nil
	}
	peerIDs := []int{}
	peerAddrs := map[int]string{}
	for _, p := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		peerIDs = append(peerIDs, id)
		peerAddrs[id] = parts[1]
	}
	return peerIDs, peerAddrs
}
