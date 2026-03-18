package election

import (
	"testing"
)

func TestVoteTrackerQuorum(t *testing.T) {
	// 5-node cluster, quorum = 3
	vt := NewVoteTracker(3)

	// Self-vote
	vt.RecordVote(1, true)
	if vt.HasQuorum() {
		t.Error("should not have quorum with 1 vote")
	}

	// Second vote
	vt.RecordVote(2, true)
	if vt.HasQuorum() {
		t.Error("should not have quorum with 2 votes")
	}

	// Third vote — quorum reached
	vt.RecordVote(3, true)
	if !vt.HasQuorum() {
		t.Error("should have quorum with 3 votes")
	}
}

func TestVoteTrackerDeniedVotes(t *testing.T) {
	vt := NewVoteTracker(3)

	vt.RecordVote(1, true)  // self
	vt.RecordVote(2, false) // denied
	vt.RecordVote(3, true)  // granted

	if vt.GrantedCount() != 2 {
		t.Errorf("expected 2 granted, got %d", vt.GrantedCount())
	}
	if vt.DeniedCount() != 1 {
		t.Errorf("expected 1 denied, got %d", vt.DeniedCount())
	}
	if vt.HasQuorum() {
		t.Error("should not have quorum with only 2 granted votes")
	}
}

func TestVoteTrackerSplitVote(t *testing.T) {
	// 5-node cluster: candidate gets 2 votes (self + 1), 2 others vote for another candidate
	vt := NewVoteTracker(3)

	vt.RecordVote(1, true)  // self
	vt.RecordVote(2, true)  // granted
	vt.RecordVote(3, false) // voted for someone else
	vt.RecordVote(4, false) // voted for someone else

	if vt.HasQuorum() {
		t.Error("should not have quorum in split vote")
	}
	if vt.GrantedCount() != 2 {
		t.Errorf("expected 2 granted, got %d", vt.GrantedCount())
	}
}

func TestVoteTrackerIsLost(t *testing.T) {
	vt := NewVoteTracker(3)

	vt.RecordVote(1, true)  // self
	vt.RecordVote(2, false) // denied

	// 5 total nodes, 2 responded, 3 remaining. Granted=1, need 3. 1+3=4 >= 3.
	if vt.IsLost(5) {
		t.Error("election is not lost yet — still enough remaining nodes")
	}

	vt.RecordVote(3, false) // another denial

	// Granted=1, remaining=2. 1+2=3 >= 3 — still possible.
	if vt.IsLost(5) {
		t.Error("election should still be possible")
	}

	vt.RecordVote(4, false) // yet another denial

	// Granted=1, remaining=1. 1+1=2 < 3 — impossible.
	if !vt.IsLost(5) {
		t.Error("election should be lost — cannot reach quorum")
	}
}

func TestVoteTrackerIsLostThreeNodeCluster(t *testing.T) {
	vt := NewVoteTracker(2) // 3-node cluster, quorum=2

	vt.RecordVote(1, true)  // self
	vt.RecordVote(2, false) // denied

	// 1 remaining node. Granted=1, 1+1=2 >= 2.
	if vt.IsLost(3) {
		t.Error("election not lost — one remaining node could grant")
	}

	vt.RecordVote(3, false)
	if !vt.IsLost(3) {
		t.Error("election should be lost — no remaining nodes")
	}
}

func TestSplitVoteScenarioIntegration(t *testing.T) {
	// Simulate a split vote in a 5-node cluster where no candidate wins.
	// Candidate A (node 1): gets votes from self + node 2 = 2 votes
	// Candidate B (node 3): gets votes from self + node 4 = 2 votes
	// Node 5: didn't respond yet

	vtA := NewVoteTracker(3)
	vtA.RecordVote(1, true)
	vtA.RecordVote(2, true)
	vtA.RecordVote(3, false) // voted for B
	vtA.RecordVote(4, false) // voted for B

	vtB := NewVoteTracker(3)
	vtB.RecordVote(3, true)
	vtB.RecordVote(4, true)
	vtB.RecordVote(1, false) // voted for A
	vtB.RecordVote(2, false) // voted for A

	// Neither should have quorum.
	if vtA.HasQuorum() {
		t.Error("candidate A should not have quorum")
	}
	if vtB.HasQuorum() {
		t.Error("candidate B should not have quorum")
	}

	// Node 5 votes for A — both trackers update.
	vtA.RecordVote(5, true)
	vtB.RecordVote(5, false) // node 5 voted for A, so denied to B

	if !vtA.HasQuorum() {
		t.Error("candidate A should now have quorum (3 votes)")
	}

	// B's election is now lost — all 5 nodes responded, only 2 granted.
	if !vtB.IsLost(5) {
		t.Error("candidate B's election should be lost")
	}
}
