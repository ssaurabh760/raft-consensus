package raft

import "errors"

var (
	// ErrNotLeader is returned when a non-leader node receives a client request.
	ErrNotLeader = errors.New("raft: node is not the leader")

	// ErrLeaderUnknown is returned when the leader is not known.
	ErrLeaderUnknown = errors.New("raft: leader unknown")

	// ErrTermMismatch is returned when an RPC has a term that doesn't match expectations.
	ErrTermMismatch = errors.New("raft: term mismatch")

	// ErrLogInconsistency is returned when the log consistency check fails.
	ErrLogInconsistency = errors.New("raft: log inconsistency detected")

	// ErrNodeStopped is returned when an operation is attempted on a stopped node.
	ErrNodeStopped = errors.New("raft: node is stopped")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("raft: operation timed out")

	// ErrAlreadyVoted is returned when a node has already voted in the current term.
	ErrAlreadyVoted = errors.New("raft: already voted in current term")

	// ErrLogIndexOutOfBounds is returned when a log index is out of bounds.
	ErrLogIndexOutOfBounds = errors.New("raft: log index out of bounds")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("raft: invalid configuration")
)
