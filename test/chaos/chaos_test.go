package chaos

import (
	"testing"
	"time"
)

func TestChaosRandomFailures(t *testing.T) {
	config := &Config{
		NumNodes:        5,
		Duration:        5 * time.Second,
		ActionInterval:  200 * time.Millisecond,
		MaxDisconnected: 2, // maintain quorum
		SubmitInterval:  150 * time.Millisecond,
		Seed:            42, // deterministic
	}

	runner := NewRunner(config)
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("chaos test failed: %v", err)
	}

	t.Logf("Chaos test completed:")
	t.Logf("  Events: %d", len(result.Events))
	t.Logf("  Submits: %d total, %d successful", result.TotalSubmits, result.SuccessSubmits)
	t.Logf("  Final leader: Node %d (term %d)", result.FinalLeader, result.FinalTerm)
	t.Logf("  Log lengths: min=%d, max=%d", result.MinLogLen, result.MaxLogLen)
	t.Logf("  Nodes converged: %v", result.NodesConverged)

	// Verify there is a leader at the end.
	if result.FinalLeader == -1 {
		t.Error("cluster should have a leader after convergence")
	}

	// Verify some submits succeeded.
	if result.SuccessSubmits == 0 {
		t.Error("expected at least some successful submits")
	}

	// Verify log lengths are close (convergence).
	logDiff := result.MaxLogLen - result.MinLogLen
	if logDiff > 3 {
		t.Errorf("log lengths diverged too much: min=%d, max=%d (diff=%d)",
			result.MinLogLen, result.MaxLogLen, logDiff)
	}
}

func TestChaosWithAggressivePartitions(t *testing.T) {
	config := &Config{
		NumNodes:        5,
		Duration:        4 * time.Second,
		ActionInterval:  100 * time.Millisecond, // faster chaos
		MaxDisconnected: 2,
		SubmitInterval:  200 * time.Millisecond,
		Seed:            123,
	}

	runner := NewRunner(config)
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("chaos test failed: %v", err)
	}

	t.Logf("Aggressive chaos test:")
	t.Logf("  Events: %d", len(result.Events))
	t.Logf("  Submits: %d total, %d successful", result.TotalSubmits, result.SuccessSubmits)
	t.Logf("  Final leader: Node %d", result.FinalLeader)
	t.Logf("  Converged: %v", result.NodesConverged)

	// With aggressive partitions, the cluster should still recover.
	if result.FinalLeader == -1 {
		t.Error("cluster should elect a leader after chaos ends")
	}
}

func TestChaosThreeNodeCluster(t *testing.T) {
	config := &Config{
		NumNodes:        3,
		Duration:        4 * time.Second,
		ActionInterval:  250 * time.Millisecond,
		MaxDisconnected: 1, // only 1 can fail in 3-node
		SubmitInterval:  200 * time.Millisecond,
		Seed:            7,
	}

	runner := NewRunner(config)
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("chaos test failed: %v", err)
	}

	t.Logf("3-node chaos test:")
	t.Logf("  Events: %d, Submits: %d/%d", len(result.Events), result.SuccessSubmits, result.TotalSubmits)
	t.Logf("  Final leader: Node %d, converged: %v", result.FinalLeader, result.NodesConverged)

	if result.FinalLeader == -1 {
		t.Error("3-node cluster should have a leader after convergence")
	}
}
