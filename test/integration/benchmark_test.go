package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkElectionConvergence(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cluster := NewTestCluster(5)
		if err := cluster.Start(); err != nil {
			b.Fatalf("failed to start: %v", err)
		}

		start := time.Now()
		_, err := cluster.WaitForLeader(5 * time.Second)
		elapsed := time.Since(start)

		if err != nil {
			b.Fatalf("no leader: %v", err)
		}
		b.ReportMetric(float64(elapsed.Milliseconds()), "ms/election")
		cluster.Stop()
	}
}

func BenchmarkReplicationThroughput(b *testing.B) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		b.Fatalf("failed to start: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		b.Fatalf("no leader: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		leader.Submit(fmt.Sprintf("bench-%d", i))
	}
	b.StopTimer()

	// Wait for replication.
	time.Sleep(1 * time.Second)
}

func TestStressConcurrentSubmits(t *testing.T) {
	cluster := NewTestCluster(5)
	if err := cluster.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer cluster.Stop()

	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("no leader: %v", err)
	}

	// Submit 100 commands concurrently.
	numSubmits := 100
	var wg sync.WaitGroup
	wg.Add(numSubmits)

	successes := make(chan bool, numSubmits)
	for i := 0; i < numSubmits; i++ {
		go func(idx int) {
			defer wg.Done()
			_, _, ok := leader.Submit(fmt.Sprintf("concurrent-%d", idx))
			successes <- ok
		}(i)
	}

	wg.Wait()
	close(successes)

	successCount := 0
	for ok := range successes {
		if ok {
			successCount++
		}
	}

	t.Logf("Concurrent submits: %d/%d successful", successCount, numSubmits)
	if successCount != numSubmits {
		t.Errorf("expected all %d submits to succeed, got %d", numSubmits, successCount)
	}

	// Wait for replication and check convergence.
	time.Sleep(2 * time.Second)

	leaderLogLen := leader.GetLog().Len()
	if leaderLogLen < numSubmits {
		t.Errorf("leader log should have >= %d entries, got %d", numSubmits, leaderLogLen)
	}

	for _, node := range cluster.Nodes {
		if node.GetID() == leader.GetID() {
			continue
		}
		logLen := node.GetLog().Len()
		if logLen < numSubmits {
			t.Errorf("node %d log should have >= %d entries, got %d",
				node.GetID(), numSubmits, logLen)
		}
	}
}
