// Package raft implements the core Raft consensus algorithm.
package raft

import "fmt"

// NodeRole represents the current role of a Raft node.
type NodeRole int

const (
	// Follower is the default state. Followers respond to RPCs from leaders and candidates.
	Follower NodeRole = iota
	// Candidate is the state when a node is requesting votes to become leader.
	Candidate
	// Leader is the state when a node has been elected and manages log replication.
	Leader
)

// String returns a human-readable representation of the NodeRole.
func (r NodeRole) String() string {
	switch r {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return fmt.Sprintf("Unknown(%d)", int(r))
	}
}
