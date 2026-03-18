package raft

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/election"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/rpc"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/transport"
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

	// Election state.
	electionTimer *election.Timer
	votesReceived map[int]bool // tracks which peers granted votes in current election

	// Transport for sending RPCs.
	transport transport.Transport

	// Channels.
	applyCh       chan LogEntry // channel for delivering committed entries to the state machine
	resetTimerCh  chan struct{} // signals to reset the election timer
	stepDownCh    chan struct{} // signals leader/candidate to step down to follower
	stopCh        chan struct{} // signals all goroutines to stop
	stopped       bool

	// Configuration.
	config *Config
}

// NewRaftNode creates a new Raft node with the given configuration.
// The node starts in the Follower state with term 0.
func NewRaftNode(config *Config) *RaftNode {
	node := &RaftNode{
		currentTerm:  0,
		votedFor:     -1,
		log:          NewMemoryLog(),
		commitIndex:  0,
		lastApplied:  0,
		nextIndex:    make(map[int]int),
		matchIndex:   make(map[int]int),
		id:           config.NodeID,
		role:         Follower,
		leaderID:     -1,
		peers:        config.Peers,
		applyCh:      make(chan LogEntry, 100),
		resetTimerCh: make(chan struct{}, 1),
		stepDownCh:   make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		config:       config,
	}
	node.electionTimer = election.NewTimer(config.ElectionTimeoutMin, config.ElectionTimeoutMax)
	return node
}

// SetTransport sets the transport layer for the node. Must be called before Start().
func (n *RaftNode) SetTransport(t transport.Transport) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.transport = t
}

// Start begins the Raft node's event loop.
func (n *RaftNode) Start() error {
	n.mu.Lock()
	if n.transport == nil {
		n.mu.Unlock()
		return fmt.Errorf("raft: transport not set")
	}
	n.mu.Unlock()

	n.electionTimer.Reset()
	go n.run()
	log.Printf("[Node %d] started as %s in term %d", n.id, n.role, n.currentTerm)
	return nil
}

// Stop gracefully shuts down the Raft node.
func (n *RaftNode) Stop() {
	n.mu.Lock()
	if n.stopped {
		n.mu.Unlock()
		return
	}
	n.stopped = true
	n.mu.Unlock()

	close(n.stopCh)
	n.electionTimer.Stop()
	log.Printf("[Node %d] stopped", n.id)
}

// run is the main event loop for the Raft node.
func (n *RaftNode) run() {
	for {
		select {
		case <-n.stopCh:
			return
		default:
		}

		n.mu.Lock()
		role := n.role
		n.mu.Unlock()

		switch role {
		case Follower:
			n.runFollower()
		case Candidate:
			n.runCandidate()
		case Leader:
			n.runLeader()
		}
	}
}

// runFollower runs the follower event loop.
// Followers wait for heartbeats/RPCs and start an election if the timer expires.
func (n *RaftNode) runFollower() {
	log.Printf("[Node %d] running as Follower in term %d", n.id, n.GetCurrentTerm())
	n.electionTimer.Reset()

	for {
		select {
		case <-n.stopCh:
			return
		case <-n.resetTimerCh:
			n.electionTimer.Reset()
		case <-n.stepDownCh:
			// Already a follower, just reset timer.
			n.electionTimer.Reset()
		case <-n.electionTimer.C:
			// Election timeout expired — become candidate.
			log.Printf("[Node %d] election timeout expired, becoming candidate", n.id)
			n.mu.Lock()
			n.role = Candidate
			n.mu.Unlock()
			return
		}
	}
}

// runCandidate runs the candidate event loop.
// Candidates increment term, vote for self, request votes, and wait for results.
func (n *RaftNode) runCandidate() {
	n.mu.Lock()
	n.currentTerm++
	n.votedFor = n.id
	n.leaderID = -1
	n.votesReceived = map[int]bool{n.id: true}
	currentTerm := n.currentTerm
	peers := make([]int, len(n.peers))
	copy(peers, n.peers)
	n.mu.Unlock()

	log.Printf("[Node %d] starting election for term %d", n.id, currentTerm)
	n.electionTimer.Reset()

	// Send RequestVote RPCs to all peers in parallel.
	voteResultCh := make(chan *rpc.RequestVoteResponse, len(peers))
	for _, peerID := range peers {
		go func(target int) {
			n.mu.Lock()
			lastLogIndex := n.log.LastIndex()
			lastLogTerm := n.log.LastTerm()
			n.mu.Unlock()

			req := &rpc.RequestVoteRequest{
				Term:         currentTerm,
				CandidateID:  n.id,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}

			ctx, cancel := context.WithTimeout(context.Background(), n.config.RPCTimeout)
			defer cancel()

			resp, err := n.transport.SendRequestVote(ctx, target, req)
			if err != nil {
				log.Printf("[Node %d] failed to send RequestVote to node %d: %v", n.id, target, err)
				return
			}
			voteResultCh <- resp
		}(peerID)
	}

	// Wait for vote results, timer expiry, or step-down signal.
	for {
		select {
		case <-n.stopCh:
			return

		case <-n.stepDownCh:
			log.Printf("[Node %d] stepping down from candidate to follower", n.id)
			return

		case <-n.electionTimer.C:
			// Election timeout — start new election.
			log.Printf("[Node %d] election timeout during candidacy, restarting election", n.id)
			return

		case resp := <-voteResultCh:
			if resp == nil {
				continue
			}

			n.mu.Lock()
			// If response has higher term, step down.
			if resp.Term > n.currentTerm {
				log.Printf("[Node %d] received higher term %d from vote response, stepping down", n.id, resp.Term)
				n.currentTerm = resp.Term
				n.votedFor = -1
				n.role = Follower
				n.mu.Unlock()
				return
			}

			// Count vote if granted and still in the same term.
			if resp.VoteGranted && n.currentTerm == currentTerm {
				// We don't know the exact peer ID, but we track vote count.
				n.votesReceived[len(n.votesReceived)] = true
				voteCount := len(n.votesReceived)
				quorum := n.config.QuorumSize()

				if voteCount >= quorum {
					log.Printf("[Node %d] received majority (%d/%d votes), becoming leader for term %d",
						n.id, voteCount, n.config.ClusterSize(), n.currentTerm)
					n.role = Leader
					n.leaderID = n.id
					// Initialize leader volatile state.
					nextIdx := n.log.LastIndex() + 1
					for _, peer := range n.peers {
						n.nextIndex[peer] = nextIdx
						n.matchIndex[peer] = 0
					}
					n.mu.Unlock()
					return
				}
			}
			n.mu.Unlock()
		}
	}
}

// runLeader runs the leader event loop.
// Leaders send periodic heartbeats and handle log replication.
func (n *RaftNode) runLeader() {
	log.Printf("[Node %d] running as Leader in term %d", n.id, n.GetCurrentTerm())
	n.electionTimer.Stop()

	// Send initial heartbeat immediately.
	n.sendHeartbeats()

	heartbeatTicker := time.NewTicker(n.config.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-n.stopCh:
			return

		case <-n.stepDownCh:
			log.Printf("[Node %d] leader stepping down to follower", n.id)
			return

		case <-heartbeatTicker.C:
			n.sendHeartbeats()
		}
	}
}

// sendHeartbeats sends empty AppendEntries RPCs to all peers.
func (n *RaftNode) sendHeartbeats() {
	n.mu.Lock()
	currentTerm := n.currentTerm
	leaderID := n.id
	commitIndex := n.commitIndex
	peers := make([]int, len(n.peers))
	copy(peers, n.peers)
	n.mu.Unlock()

	for _, peerID := range peers {
		go func(target int) {
			n.mu.Lock()
			prevLogIndex := n.nextIndex[target] - 1
			prevLogTerm := 0
			if prevLogIndex > 0 {
				entry, err := n.log.GetEntry(prevLogIndex)
				if err == nil {
					prevLogTerm = entry.Term
				}
			}
			n.mu.Unlock()

			req := &rpc.AppendEntriesRequest{
				Term:         currentTerm,
				LeaderID:     leaderID,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      nil, // heartbeat
				LeaderCommit: commitIndex,
			}

			ctx, cancel := context.WithTimeout(context.Background(), n.config.RPCTimeout)
			defer cancel()

			resp, err := n.transport.SendAppendEntries(ctx, target, req)
			if err != nil {
				return
			}

			n.mu.Lock()
			defer n.mu.Unlock()

			if resp.Term > n.currentTerm {
				log.Printf("[Node %d] heartbeat response has higher term %d, stepping down", n.id, resp.Term)
				n.currentTerm = resp.Term
				n.votedFor = -1
				n.role = Follower
				n.leaderID = -1
				n.signalStepDown()
			}
		}(peerID)
	}
}

// HandleRequestVote processes an incoming RequestVote RPC (Raft paper Figure 2).
func (n *RaftNode) HandleRequestVote(req *rpc.RequestVoteRequest) (*rpc.RequestVoteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	resp := &rpc.RequestVoteResponse{
		Term:        n.currentTerm,
		VoteGranted: false,
	}

	// Rule: If RPC request term > currentTerm, update currentTerm and convert to follower.
	if req.Term > n.currentTerm {
		log.Printf("[Node %d] RequestVote from %d has higher term %d (current: %d), updating term",
			n.id, req.CandidateID, req.Term, n.currentTerm)
		n.currentTerm = req.Term
		n.votedFor = -1
		n.role = Follower
		n.leaderID = -1
		n.signalStepDown()
	}

	resp.Term = n.currentTerm

	// Rule 1: Reply false if term < currentTerm.
	if req.Term < n.currentTerm {
		return resp, nil
	}

	// Rule 2: If votedFor is null (-1) or candidateId, and candidate's log is at
	// least as up-to-date as receiver's log, grant vote.
	if n.votedFor == -1 || n.votedFor == req.CandidateID {
		if n.isLogUpToDate(req.LastLogIndex, req.LastLogTerm) {
			log.Printf("[Node %d] granting vote to %d for term %d", n.id, req.CandidateID, req.Term)
			n.votedFor = req.CandidateID
			resp.VoteGranted = true
			n.signalResetTimer()
		}
	}

	return resp, nil
}

// HandleAppendEntries processes an incoming AppendEntries RPC (Raft paper Figure 2).
func (n *RaftNode) HandleAppendEntries(req *rpc.AppendEntriesRequest) (*rpc.AppendEntriesResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	resp := &rpc.AppendEntriesResponse{
		Term:    n.currentTerm,
		Success: false,
	}

	// Rule: If RPC request term > currentTerm, update currentTerm and convert to follower.
	if req.Term > n.currentTerm {
		log.Printf("[Node %d] AppendEntries from leader %d has higher term %d (current: %d)",
			n.id, req.LeaderID, req.Term, n.currentTerm)
		n.currentTerm = req.Term
		n.votedFor = -1
		n.role = Follower
		n.signalStepDown()
	}

	resp.Term = n.currentTerm

	// Rule 1: Reply false if term < currentTerm.
	if req.Term < n.currentTerm {
		return resp, nil
	}

	// Valid AppendEntries from current leader — reset election timer and update leader.
	n.leaderID = req.LeaderID
	if n.role != Follower {
		n.role = Follower
		n.signalStepDown()
	}
	n.signalResetTimer()

	// Rule 2: Reply false if log doesn't contain an entry at prevLogIndex with prevLogTerm.
	if req.PrevLogIndex > 0 {
		entry, err := n.log.GetEntry(req.PrevLogIndex)
		if err != nil {
			// Log doesn't have entry at prevLogIndex.
			return resp, nil
		}
		if entry.Term != req.PrevLogTerm {
			// Entry at prevLogIndex has different term — inconsistency.
			// Rule 3: Delete the existing entry and all that follow it.
			n.log.Truncate(req.PrevLogIndex)
			return resp, nil
		}
	}

	// Rule 3 & 4: Append any new entries not already in the log.
	for i, entry := range req.Entries {
		idx := req.PrevLogIndex + 1 + i
		existingEntry, err := n.log.GetEntry(idx)
		if err != nil {
			// No entry at this index — append this and all remaining entries.
			for j := i; j < len(req.Entries); j++ {
				n.log.Append(LogEntry{
					Term:    req.Entries[j].Term,
					Command: req.Entries[j].Command,
				})
			}
			break
		}
		if existingEntry.Term != entry.Term {
			// Conflict — truncate from here and append remaining.
			n.log.Truncate(idx)
			for j := i; j < len(req.Entries); j++ {
				n.log.Append(LogEntry{
					Term:    req.Entries[j].Term,
					Command: req.Entries[j].Command,
				})
			}
			break
		}
	}

	// Rule 5: If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry).
	if req.LeaderCommit > n.commitIndex {
		lastNewIndex := n.log.LastIndex()
		if req.LeaderCommit < lastNewIndex {
			n.commitIndex = req.LeaderCommit
		} else {
			n.commitIndex = lastNewIndex
		}
	}

	resp.Success = true
	return resp, nil
}

// isLogUpToDate checks if the candidate's log is at least as up-to-date as the receiver's.
// Raft determines which log is more up-to-date by comparing the index and term of the last entries.
func (n *RaftNode) isLogUpToDate(candidateLastIndex, candidateLastTerm int) bool {
	lastTerm := n.log.LastTerm()
	lastIndex := n.log.LastIndex()

	// If the logs have last entries with different terms, then the log with the
	// later term is more up-to-date.
	if candidateLastTerm != lastTerm {
		return candidateLastTerm > lastTerm
	}
	// If the logs end with the same term, then whichever log is longer is more up-to-date.
	return candidateLastIndex >= lastIndex
}

// signalResetTimer non-blockingly signals to reset the election timer.
func (n *RaftNode) signalResetTimer() {
	select {
	case n.resetTimerCh <- struct{}{}:
	default:
	}
}

// signalStepDown non-blockingly signals the node to step down to follower.
func (n *RaftNode) signalStepDown() {
	select {
	case n.stepDownCh <- struct{}{}:
	default:
	}
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

// GetLog returns the node's log (for testing).
func (n *RaftNode) GetLog() Log {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.log
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
