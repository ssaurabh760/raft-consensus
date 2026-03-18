package replication

import "time"

// HeartbeatConfig holds configuration for the heartbeat mechanism.
type HeartbeatConfig struct {
	// Interval is the time between heartbeats.
	Interval time.Duration
}

// DefaultHeartbeatConfig returns default heartbeat configuration.
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval: 50 * time.Millisecond,
	}
}

// IsHeartbeat returns true if the AppendEntries RPC contains no log entries.
func IsHeartbeat(entriesLen int) bool {
	return entriesLen == 0
}
