// Package election implements leader election logic for the Raft consensus algorithm.
package election

import (
	"math/rand"
	"sync"
	"time"
)

// Timer manages the randomized election timeout for a Raft node.
// When the timer fires, the node should start an election.
type Timer struct {
	mu         sync.Mutex
	minTimeout time.Duration
	maxTimeout time.Duration
	timer      *time.Timer
	C          <-chan time.Time // Fires when the election timeout expires.
	stopped    bool
}

// NewTimer creates a new election timer with the given timeout range.
// The timer is initially stopped and must be started with Reset().
func NewTimer(minTimeout, maxTimeout time.Duration) *Timer {
	t := &Timer{
		minTimeout: minTimeout,
		maxTimeout: maxTimeout,
		stopped:    true,
	}
	// Create a stopped timer.
	inner := time.NewTimer(0)
	if !inner.Stop() {
		<-inner.C
	}
	t.timer = inner
	t.C = inner.C
	return t
}

// RandomTimeout returns a random duration between minTimeout and maxTimeout.
func (t *Timer) RandomTimeout() time.Duration {
	diff := t.maxTimeout - t.minTimeout
	if diff <= 0 {
		return t.minTimeout
	}
	return t.minTimeout + time.Duration(rand.Int63n(int64(diff)))
}

// Reset stops the current timer and restarts it with a new random timeout.
// This should be called when a heartbeat or vote grant is received.
func (t *Timer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.timer.Stop() {
		// Drain the channel if the timer already fired.
		select {
		case <-t.timer.C:
		default:
		}
	}
	t.timer.Reset(t.RandomTimeout())
	t.stopped = false
}

// Stop stops the election timer.
func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
	t.stopped = true
}

// IsStopped returns whether the timer is currently stopped.
func (t *Timer) IsStopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}
