package raft

import "time"

const (
	// DefaultElectionTimeoutMin is the minimum election timeout in milliseconds.
	DefaultElectionTimeoutMin = 150 * time.Millisecond

	// DefaultElectionTimeoutMax is the maximum election timeout in milliseconds.
	DefaultElectionTimeoutMax = 300 * time.Millisecond

	// DefaultHeartbeatInterval is the interval between leader heartbeats.
	DefaultHeartbeatInterval = 50 * time.Millisecond

	// DefaultRPCTimeout is the timeout for RPC calls.
	DefaultRPCTimeout = 100 * time.Millisecond
)

// Config holds the configuration for a Raft node.
type Config struct {
	// NodeID is the unique identifier for this node.
	NodeID int

	// Peers is the list of peer node IDs in the cluster (excluding this node).
	Peers []int

	// ElectionTimeoutMin is the minimum election timeout duration.
	ElectionTimeoutMin time.Duration

	// ElectionTimeoutMax is the maximum election timeout duration.
	ElectionTimeoutMax time.Duration

	// HeartbeatInterval is the interval between leader heartbeats.
	HeartbeatInterval time.Duration

	// RPCTimeout is the timeout for individual RPC calls.
	RPCTimeout time.Duration

	// DataDir is the directory for persistent storage.
	DataDir string

	// ListenAddr is the address this node listens on for RPCs.
	ListenAddr string

	// HTTPAddr is the address for the HTTP API (e.g., KV store).
	HTTPAddr string
}

// DefaultConfig returns a Config with sensible default values.
// NodeID and Peers must still be set by the caller.
func DefaultConfig() *Config {
	return &Config{
		ElectionTimeoutMin: DefaultElectionTimeoutMin,
		ElectionTimeoutMax: DefaultElectionTimeoutMax,
		HeartbeatInterval:  DefaultHeartbeatInterval,
		RPCTimeout:         DefaultRPCTimeout,
		DataDir:            "/tmp/raft",
		ListenAddr:         ":9000",
		HTTPAddr:           ":8000",
	}
}

// ClusterSize returns the total number of nodes in the cluster (peers + self).
func (c *Config) ClusterSize() int {
	return len(c.Peers) + 1
}

// QuorumSize returns the minimum number of nodes needed for a majority.
func (c *Config) QuorumSize() int {
	return c.ClusterSize()/2 + 1
}
