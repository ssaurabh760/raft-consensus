// Package rpc defines the RPC message types and handlers for Raft inter-node communication.
package rpc

// RequestVoteRequest is sent by candidates to gather votes (Raft paper Figure 2).
type RequestVoteRequest struct {
	Term         int // candidate's term
	CandidateID  int // candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

// RequestVoteResponse is the response to a RequestVote RPC.
type RequestVoteResponse struct {
	Term        int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

// AppendEntriesRequest is sent by leaders for log replication and heartbeats (Raft paper Figure 2).
type AppendEntriesRequest struct {
	Term         int        // leader's term
	LeaderID     int        // so follower can redirect clients
	PrevLogIndex int        // index of log entry immediately preceding new ones
	PrevLogTerm  int        // term of prevLogIndex entry
	Entries      []LogEntry // log entries to store (empty for heartbeat)
	LeaderCommit int        // leader's commitIndex
}

// AppendEntriesResponse is the response to an AppendEntries RPC.
type AppendEntriesResponse struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching prevLogIndex and prevLogTerm
}

// LogEntry represents a single log entry in RPC messages.
type LogEntry struct {
	Term    int
	Index   int
	Command []byte
}
