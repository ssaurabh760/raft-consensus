// Package chaos provides a testing framework for injecting random failures
// into a Raft cluster to verify correctness under adversarial conditions.
package chaos

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/saurabhsrivastava/raft-consensus-go/internal/raft"
	"github.com/saurabhsrivastava/raft-consensus-go/internal/transport"
)

// Action represents a type of chaos action.
type Action int

const (
	ActionDisconnect Action = iota
	ActionReconnect
	ActionSubmit
)

func (a Action) String() string {
	switch a {
	case ActionDisconnect:
		return "disconnect"
	case ActionReconnect:
		return "reconnect"
	case ActionSubmit:
		return "submit"
	default:
		return "unknown"
	}
}

// Event records a chaos action that was taken.
type Event struct {
	Time   time.Duration
	Action Action
	NodeID int
	Detail string
}

func (e Event) String() string {
	return fmt.Sprintf("[%v] %s node=%d %s", e.Time, e.Action, e.NodeID, e.Detail)
}

// Config configures the chaos test.
type Config struct {
	// NumNodes is the cluster size.
	NumNodes int

	// Duration is how long to run the chaos test.
	Duration time.Duration

	// ActionInterval is the time between random actions.
	ActionInterval time.Duration

	// MaxDisconnected is the maximum number of simultaneously disconnected nodes.
	// Defaults to (NumNodes-1)/2 to maintain quorum possibility.
	MaxDisconnected int

	// SubmitInterval is how often the leader submits a command.
	SubmitInterval time.Duration

	// Seed for deterministic random behavior. 0 = time-based.
	Seed int64
}

// DefaultConfig returns a default chaos test configuration.
func DefaultConfig(numNodes int) *Config {
	return &Config{
		NumNodes:        numNodes,
		Duration:        5 * time.Second,
		ActionInterval:  200 * time.Millisecond,
		MaxDisconnected: (numNodes - 1) / 2,
		SubmitInterval:  100 * time.Millisecond,
		Seed:            0,
	}
}

// Result contains the outcomes of a chaos test.
type Result struct {
	Events         []Event
	TotalSubmits   int
	SuccessSubmits int
	FinalLeader    int
	FinalTerm      int
	NodesConverged bool
	MaxLogLen      int
	MinLogLen      int
}

// Runner manages a chaos test execution.
type Runner struct {
	config       *Config
	nodes        []*raft.RaftNode
	network      *transport.MockNetwork
	nodeIDs      []int
	disconnected map[int]bool
	events       []Event
	startTime    time.Time
	rng          *rand.Rand
}

// NewRunner creates a new chaos test runner.
func NewRunner(config *Config) *Runner {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Runner{
		config:       config,
		disconnected: make(map[int]bool),
		rng:          rand.New(rand.NewSource(seed)),
	}
}

// Run executes the chaos test and returns results.
func (r *Runner) Run() (*Result, error) {
	// Create cluster.
	r.network = transport.NewMockNetwork()
	r.nodes = make([]*raft.RaftNode, r.config.NumNodes)
	r.nodeIDs = make([]int, r.config.NumNodes)

	for i := 0; i < r.config.NumNodes; i++ {
		r.nodeIDs[i] = i + 1
	}

	for i := 0; i < r.config.NumNodes; i++ {
		peers := make([]int, 0, r.config.NumNodes-1)
		for j := 0; j < r.config.NumNodes; j++ {
			if i != j {
				peers = append(peers, r.nodeIDs[j])
			}
		}

		cfg := &raft.Config{
			NodeID:             r.nodeIDs[i],
			Peers:              peers,
			ElectionTimeoutMin: 150 * time.Millisecond,
			ElectionTimeoutMax: 300 * time.Millisecond,
			HeartbeatInterval:  50 * time.Millisecond,
			RPCTimeout:         100 * time.Millisecond,
		}
		node := raft.NewRaftNode(cfg)
		t := r.network.AddNode(r.nodeIDs[i], node)
		node.SetTransport(t)
		r.nodes[i] = node
	}

	// Start all nodes.
	for _, node := range r.nodes {
		if err := node.Start(); err != nil {
			return nil, fmt.Errorf("failed to start node %d: %w", node.GetID(), err)
		}
	}
	defer func() {
		for _, node := range r.nodes {
			node.Stop()
		}
	}()

	r.startTime = time.Now()

	// Wait for initial leader election.
	time.Sleep(500 * time.Millisecond)

	// Run chaos actions until duration expires.
	totalSubmits := 0
	successSubmits := 0
	actionTicker := time.NewTicker(r.config.ActionInterval)
	submitTicker := time.NewTicker(r.config.SubmitInterval)
	defer actionTicker.Stop()
	defer submitTicker.Stop()

	deadline := time.After(r.config.Duration)

	for {
		select {
		case <-deadline:
			goto done

		case <-actionTicker.C:
			r.performRandomAction()

		case <-submitTicker.C:
			totalSubmits++
			if r.trySubmit() {
				successSubmits++
			}
		}
	}

done:
	// Reconnect all nodes and let cluster converge.
	for id := range r.disconnected {
		r.network.ReconnectNode(id)
	}
	time.Sleep(2 * time.Second)

	// Collect results.
	result := &Result{
		Events:         r.events,
		TotalSubmits:   totalSubmits,
		SuccessSubmits: successSubmits,
		FinalLeader:    -1,
	}

	maxLogLen := 0
	minLogLen := int(^uint(0) >> 1) // max int

	for _, node := range r.nodes {
		if node.GetRole() == raft.Leader {
			result.FinalLeader = node.GetID()
			result.FinalTerm = node.GetCurrentTerm()
		}
		logLen := node.GetLog().Len()
		if logLen > maxLogLen {
			maxLogLen = logLen
		}
		if logLen < minLogLen {
			minLogLen = logLen
		}
	}

	result.MaxLogLen = maxLogLen
	result.MinLogLen = minLogLen
	result.NodesConverged = (maxLogLen - minLogLen) <= 1

	return result, nil
}

// performRandomAction picks a random chaos action.
func (r *Runner) performRandomAction() {
	numDisconnected := len(r.disconnected)
	canDisconnect := numDisconnected < r.config.MaxDisconnected
	canReconnect := numDisconnected > 0

	var action Action
	switch {
	case canDisconnect && canReconnect:
		if r.rng.Float64() < 0.5 {
			action = ActionDisconnect
		} else {
			action = ActionReconnect
		}
	case canDisconnect:
		action = ActionDisconnect
	case canReconnect:
		action = ActionReconnect
	default:
		return
	}

	switch action {
	case ActionDisconnect:
		// Pick a random connected node.
		connected := r.getConnectedNodes()
		if len(connected) == 0 {
			return
		}
		target := connected[r.rng.Intn(len(connected))]
		r.network.DisconnectNode(target)
		r.disconnected[target] = true
		r.recordEvent(ActionDisconnect, target, "")
		log.Printf("[Chaos] disconnected node %d", target)

	case ActionReconnect:
		// Pick a random disconnected node.
		var disconnectedList []int
		for id := range r.disconnected {
			disconnectedList = append(disconnectedList, id)
		}
		if len(disconnectedList) == 0 {
			return
		}
		target := disconnectedList[r.rng.Intn(len(disconnectedList))]
		r.network.ReconnectNode(target)
		delete(r.disconnected, target)
		r.recordEvent(ActionReconnect, target, "")
		log.Printf("[Chaos] reconnected node %d", target)
	}
}

// trySubmit attempts to submit a command on the current leader.
func (r *Runner) trySubmit() bool {
	for _, node := range r.nodes {
		if node.GetRole() == raft.Leader && !r.disconnected[node.GetID()] {
			cmd := fmt.Sprintf("chaos-%d", time.Now().UnixNano())
			_, _, isLeader := node.Submit(cmd)
			if isLeader {
				r.recordEvent(ActionSubmit, node.GetID(), cmd)
				return true
			}
		}
	}
	return false
}

func (r *Runner) getConnectedNodes() []int {
	var connected []int
	for _, id := range r.nodeIDs {
		if !r.disconnected[id] {
			connected = append(connected, id)
		}
	}
	return connected
}

func (r *Runner) recordEvent(action Action, nodeID int, detail string) {
	r.events = append(r.events, Event{
		Time:   time.Since(r.startTime),
		Action: action,
		NodeID: nodeID,
		Detail: detail,
	})
}
