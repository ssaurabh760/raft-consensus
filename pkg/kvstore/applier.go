package kvstore

import (
	"log"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
)

// Applier reads committed log entries from Raft's applyCh and applies them
// to the KV store state machine.
type Applier struct {
	store   *Store
	applyCh <-chan raft.LogEntry
	stopCh  chan struct{}
}

// NewApplier creates a new applier that bridges Raft and the KV store.
func NewApplier(store *Store, applyCh <-chan raft.LogEntry) *Applier {
	return &Applier{
		store:   store,
		applyCh: applyCh,
		stopCh:  make(chan struct{}),
	}
}

// Start begins applying committed entries in a background goroutine.
func (a *Applier) Start() {
	go a.run()
}

// Stop terminates the applier goroutine.
func (a *Applier) Stop() {
	close(a.stopCh)
}

func (a *Applier) run() {
	for {
		select {
		case <-a.stopCh:
			return
		case entry, ok := <-a.applyCh:
			if !ok {
				return
			}
			// Convert the command to bytes.
			var cmdBytes []byte
			if entry.Command != nil {
				switch v := entry.Command.(type) {
				case []byte:
					cmdBytes = v
				case string:
					cmdBytes = []byte(v)
				default:
					continue
				}
			}
			if len(cmdBytes) == 0 {
				continue
			}
			if err := a.store.ApplyBytes(cmdBytes); err != nil {
				log.Printf("Failed to apply entry at index %d: %v", entry.Index, err)
			}
		}
	}
}
