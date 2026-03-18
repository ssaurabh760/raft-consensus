package replication

import (
	"testing"
	"time"
)

func TestDefaultHeartbeatConfig(t *testing.T) {
	cfg := DefaultHeartbeatConfig()
	if cfg.Interval != 50*time.Millisecond {
		t.Errorf("expected interval 50ms, got %v", cfg.Interval)
	}
}

func TestIsHeartbeat(t *testing.T) {
	if !IsHeartbeat(0) {
		t.Error("0 entries should be a heartbeat")
	}
	if IsHeartbeat(1) {
		t.Error("1 entry should not be a heartbeat")
	}
	if IsHeartbeat(5) {
		t.Error("5 entries should not be a heartbeat")
	}
}
