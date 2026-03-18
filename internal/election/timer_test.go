package election

import (
	"testing"
	"time"
)

func TestRandomTimeoutRange(t *testing.T) {
	timer := NewTimer(150*time.Millisecond, 300*time.Millisecond)

	// Generate many random timeouts and verify they're all within range.
	for i := 0; i < 1000; i++ {
		d := timer.RandomTimeout()
		if d < 150*time.Millisecond || d >= 300*time.Millisecond {
			t.Errorf("timeout %v out of range [150ms, 300ms)", d)
		}
	}
}

func TestRandomTimeoutDistribution(t *testing.T) {
	timer := NewTimer(150*time.Millisecond, 300*time.Millisecond)

	// Verify that timeouts are not all the same (basic randomness check).
	seen := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		seen[timer.RandomTimeout()] = true
	}
	if len(seen) < 10 {
		t.Errorf("expected diverse timeouts, but only got %d unique values", len(seen))
	}
}

func TestRandomTimeoutEqualMinMax(t *testing.T) {
	timer := NewTimer(100*time.Millisecond, 100*time.Millisecond)
	d := timer.RandomTimeout()
	if d != 100*time.Millisecond {
		t.Errorf("expected 100ms when min==max, got %v", d)
	}
}

func TestTimerFires(t *testing.T) {
	timer := NewTimer(10*time.Millisecond, 20*time.Millisecond)
	timer.Reset()

	select {
	case <-timer.C:
		// Timer fired — expected.
	case <-time.After(100 * time.Millisecond):
		t.Error("timer did not fire within expected time")
	}
}

func TestTimerReset(t *testing.T) {
	timer := NewTimer(50*time.Millisecond, 60*time.Millisecond)
	timer.Reset()

	// Reset before it fires.
	time.Sleep(20 * time.Millisecond)
	timer.Reset()

	// The timer should fire ~50-60ms from the reset, not from the original start.
	select {
	case <-timer.C:
		// Good.
	case <-time.After(200 * time.Millisecond):
		t.Error("timer did not fire after reset")
	}
}

func TestTimerStop(t *testing.T) {
	timer := NewTimer(10*time.Millisecond, 20*time.Millisecond)
	timer.Reset()
	timer.Stop()

	select {
	case <-timer.C:
		t.Error("stopped timer should not fire")
	case <-time.After(50 * time.Millisecond):
		// Good — timer didn't fire.
	}
}

func TestTimerIsStopped(t *testing.T) {
	timer := NewTimer(100*time.Millisecond, 200*time.Millisecond)

	if !timer.IsStopped() {
		t.Error("new timer should be stopped")
	}

	timer.Reset()
	if timer.IsStopped() {
		t.Error("reset timer should not be stopped")
	}

	timer.Stop()
	if !timer.IsStopped() {
		t.Error("stopped timer should report stopped")
	}
}

func TestTimerMultipleResets(t *testing.T) {
	timer := NewTimer(10*time.Millisecond, 15*time.Millisecond)

	// Reset multiple times rapidly.
	for i := 0; i < 10; i++ {
		timer.Reset()
	}

	// Should still eventually fire.
	select {
	case <-timer.C:
		// Good.
	case <-time.After(100 * time.Millisecond):
		t.Error("timer did not fire after multiple resets")
	}
}
