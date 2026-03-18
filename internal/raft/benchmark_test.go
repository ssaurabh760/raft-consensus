package raft

import (
	"testing"
)

func BenchmarkLogAppend(b *testing.B) {
	log := NewMemoryLog()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Append(LogEntry{Term: 1, Command: "cmd"})
	}
}

func BenchmarkLogGetEntry(b *testing.B) {
	log := NewMemoryLog()
	for i := 0; i < 10000; i++ {
		log.Append(LogEntry{Term: 1, Command: i})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.GetEntry((i % 10000) + 1)
	}
}

func BenchmarkLogGetEntriesFrom(b *testing.B) {
	log := NewMemoryLog()
	for i := 0; i < 1000; i++ {
		log.Append(LogEntry{Term: 1, Command: i})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.GetEntriesFrom(500)
	}
}

func BenchmarkIsLogUpToDate(b *testing.B) {
	cfg := DefaultConfig()
	cfg.NodeID = 1
	cfg.Peers = []int{2, 3}
	node := NewRaftNode(cfg)
	for i := 0; i < 100; i++ {
		node.log.Append(LogEntry{Term: i/10 + 1, Command: i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.isLogUpToDate(100, 10)
	}
}
