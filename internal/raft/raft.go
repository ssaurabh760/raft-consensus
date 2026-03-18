package raft

import (
	"fmt"
	"sync"
)

// RaftNode represents a single node in a Raft cluster.
type RaftNode struct {
	mu sync.Mutex

	// Persistent state on all servers (updated on stable storage before responding to RPCs).
	currentTerm int
	votedFor    int // candidateId that received vote in current term, -1 if none
	log         Log

	// Volatile state on all servers.
	commitIndex int // index of highest log entry known to be committed
	lastApplied int // index of highest log entry applied to state machine

	// Volatile state on leaders (reinitialized after election).
	nextIndex  map[int]int // for each server, index of the next log entry to send
	matchIndex map[int]int // for each server, index of highest log entry known to be replicated

	// Node metadata.
	id       int
	role     NodeRole
	leaderID int // the current leader's ID, -1 if unknown
	peers    []int

	// Channels.
	applyCh chan LogEntry // channel for delivering committed entries to the state machine

	// Configuration.
	config *Config
}

// NewRaftNode creates a new Raft node with the given configuration.
// The node starts in the Follower state with term 0.
func NewRaftNode(config *Config) *RaftNode {
	node := &RaftNode{
		currentTerm: 0,
		votedFor:    -1,
		log:         NewMemoryLog(),
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[int]int),
		matchIndex:  make(map[int]int),
		id:          config.NodeID,
		role:        Follower,
		leaderID:    -1,
		peers:       config.Peers,
		applyCh:     make(chan LogEntry, 100),
		config:      config,
	}
	return node
}

// GetID returns the node's unique identifier.
func (n *RaftNode) GetID() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.id
}

// GetRole returns the node's current role.
func (n *RaftNode) GetRole() NodeRole {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.role
}

// GetCurrentTerm returns the node's current term.
func (n *RaftNode) GetCurrentTerm() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.currentTerm
}

// GetLeaderID returns the current leader's ID, or -1 if unknown.
func (n *RaftNode) GetLeaderID() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.leaderID
}

// GetCommitIndex returns the index of the highest committed log entry.
func (n *RaftNode) GetCommitIndex() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.commitIndex
}

// GetLastApplied returns the index of the highest applied log entry.
func (n *RaftNode) GetLastApplied() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastApplied
}

// ApplyCh returns the channel on which committed log entries are delivered.
func (n *RaftNode) ApplyCh() <-chan LogEntry {
	return n.applyCh
}

// String returns a human-readable summary of the node's state.
func (n *RaftNode) String() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return fmt.Sprintf("RaftNode{ID:%d, Role:%s, Term:%d, Leader:%d, LogLen:%d, CommitIdx:%d}",
		n.id, n.role, n.currentTerm, n.leaderID, n.log.Len(), n.commitIndex)
}
