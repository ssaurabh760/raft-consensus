# Raft Consensus Algorithm

A production-grade implementation of the [Raft consensus algorithm](https://raft.github.io/raft.pdf) in Go for managing leader election, log replication, and fault tolerance in a distributed systems environment.

## Overview

Raft is a consensus algorithm designed to be understandable. It provides the same fault tolerance and performance as Paxos but is decomposed into relatively independent subproblems:

- **Leader Election** — A new leader is elected when an existing leader fails
- **Log Replication** — The leader accepts log entries from clients and replicates them across the cluster
- **Safety** — If any server has applied a particular log entry to its state machine, no other server may apply a different entry for the same log index

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Client (HTTP)                  │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│                KV Store (State Machine)           │
│              pkg/kvstore/store.go                 │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│                 Raft Core                         │
│            internal/raft/raft.go                  │
│                                                   │
│  ┌─────────────┐ ┌──────────────┐ ┌───────────┐ │
│  │  Election    │ │ Replication  │ │ Heartbeat │ │
│  │  Module      │ │ Module       │ │ Module    │ │
│  └─────────────┘ └──────────────┘ └───────────┘ │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│              Transport Layer (RPC)                │
│          internal/transport/transport.go          │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│            Persistent Storage                     │
│         internal/persistence/storage.go           │
└─────────────────────────────────────────────────┘
```

## Safety Properties

This implementation guarantees all five Raft safety properties:

1. **Election Safety** — At most one leader per term
2. **Leader Append-Only** — A leader never overwrites or deletes log entries
3. **Log Matching** — If two logs contain an entry with the same index and term, the logs are identical through that index
4. **Leader Completeness** — If a log entry is committed in a given term, it will be present in the leaders' logs for all higher terms
5. **State Machine Safety** — If a server has applied a log entry at a given index, no other server will ever apply a different entry at that index

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose (for cluster mode)

### Build

```bash
make build
```

### Run Tests

```bash
# Unit tests
make test

# Tests with race detection
make test-race

# Coverage report
make coverage
```

### Start a 5-Node Cluster

```bash
# Using Docker Compose
docker compose up -d --build

# Or using the convenience scripts
make cluster-start

# Run the interactive demo
make demo

# Stop the cluster
make cluster-stop
```

### Interact with the KV Store

```bash
# Set a value
curl -X PUT localhost:8001/kv/hello -d '{"value": "world"}'

# Get a value
curl localhost:8001/kv/hello

# Delete a value
curl -X DELETE localhost:8001/kv/hello

# Check node status
curl localhost:8001/status

# Check cluster info
curl localhost:8001/cluster
```

## Project Structure

```
├── cmd/raft/              # Entry point for raft-node binary
├── internal/
│   ├── raft/              # Core Raft state machine (leader election, log replication)
│   ├── election/          # Election timer and vote tracking
│   ├── replication/       # Replication state management (nextIndex, matchIndex)
│   ├── rpc/               # RPC types and client abstraction
│   ├── persistence/       # Persistent state storage (file-based and in-memory)
│   └── transport/         # Network transport layer (HTTP/JSON + mock for testing)
├── pkg/kvstore/           # Key-value store state machine with HTTP API
├── test/
│   ├── integration/       # Multi-node cluster integration tests
│   └── chaos/             # Chaos/fault injection tests
├── scripts/               # Cluster management and demo scripts
├── Dockerfile             # Multi-stage Docker build
└── docker-compose.yml     # 5-node cluster orchestration
```

## Testing

The project includes 130+ tests at multiple levels:

- **Unit tests** — Core logic for log, election, replication, persistence, transport, KV store
- **Integration tests** — Multi-node cluster tests for election, step-down, replication, persistence/crash-recovery, heartbeat, network partitions, intermittent connectivity, KV operations
- **Chaos tests** — Random failure injection with configurable duration, seed, and node count
- **Benchmarks** — Log operations, election convergence, replication throughput
- **Stress tests** — 100 concurrent command submissions with full replication verification

### Test Coverage

| Package | Coverage |
|---------|----------|
| internal/election | 97.7% |
| internal/replication | 100% |
| internal/persistence | 87.0% |
| internal/transport | 75.7% |
| internal/raft | 30.3% (integration tests cover the rest) |
| test/integration | 78.9% |
| test/chaos | 81.0% |

```bash
make test-race    # All tests with race detection
make coverage     # Generate HTML coverage report
```

## Reference

- [In Search of an Understandable Consensus Algorithm](https://raft.github.io/raft.pdf) — Diego Ongaro, John Ousterhout
- [Raft Visualization](https://raft.github.io/)

## Author

Saurabh Srivastava
