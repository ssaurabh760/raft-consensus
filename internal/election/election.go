package election

// VoteTracker tracks votes received during a leader election.
type VoteTracker struct {
	votes  map[int]bool // nodeID -> granted
	quorum int
}

// NewVoteTracker creates a tracker for an election requiring the given quorum.
func NewVoteTracker(quorum int) *VoteTracker {
	return &VoteTracker{
		votes:  make(map[int]bool),
		quorum: quorum,
	}
}

// RecordVote records a vote from the given node.
func (vt *VoteTracker) RecordVote(nodeID int, granted bool) {
	vt.votes[nodeID] = granted
}

// HasQuorum returns true if enough votes have been granted to win the election.
func (vt *VoteTracker) HasQuorum() bool {
	granted := 0
	for _, v := range vt.votes {
		if v {
			granted++
		}
	}
	return granted >= vt.quorum
}

// GrantedCount returns the number of granted votes.
func (vt *VoteTracker) GrantedCount() int {
	count := 0
	for _, v := range vt.votes {
		if v {
			count++
		}
	}
	return count
}

// DeniedCount returns the number of denied votes.
func (vt *VoteTracker) DeniedCount() int {
	count := 0
	for _, v := range vt.votes {
		if !v {
			count++
		}
	}
	return count
}

// IsLost returns true if it's impossible to reach quorum given remaining nodes.
func (vt *VoteTracker) IsLost(totalNodes int) bool {
	remaining := totalNodes - len(vt.votes)
	return vt.GrantedCount()+remaining < vt.quorum
}
