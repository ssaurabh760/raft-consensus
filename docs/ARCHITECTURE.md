# Raft Consensus: A Deep Dive

This document is written for someone who has never heard of Raft, distributed consensus, or this project. It starts from first principles, builds up the "why," walks through every major implementation decision, and ends with how to deploy and test the system yourself.

---

## Table of Contents

1. [The Problem This Project Solves](#1-the-problem-this-project-solves)
2. [What Is Consensus and Why Does It Matter](#2-what-is-consensus-and-why-does-it-matter)
3. [The Raft Algorithm — Explained Simply](#3-the-raft-algorithm--explained-simply)
4. [How This Codebase Is Organized](#4-how-this-codebase-is-organized)
5. [Implementation Deep Dive](#5-implementation-deep-dive)
   - [5.1 The Raft Node](#51-the-raft-node)
   - [5.2 Leader Election](#52-leader-election)
   - [5.3 Log Replication](#53-log-replication)
   - [5.4 Persistence and Crash Recovery](#54-persistence-and-crash-recovery)
   - [5.5 Network Transport](#55-network-transport)
   - [5.6 The Key-Value Store](#56-the-key-value-store)
6. [Concurrency Model](#6-concurrency-model)
7. [Safety Guarantees](#7-safety-guarantees)
8. [Tradeoffs and Design Decisions](#8-tradeoffs-and-design-decisions)
9. [What This Implementation Does NOT Do](#9-what-this-implementation-does-not-do)
10. [How to Build, Deploy, and Test](#10-how-to-build-deploy-and-test)
11. [Testing Strategy](#11-testing-strategy)
12. [Configuration Reference](#12-configuration-reference)
13. [Further Reading](#13-further-reading)

---

## 1. The Problem This Project Solves

Imagine you run a database that stores user accounts. If that database lives on a single server and the server crashes, every user loses access. The obvious fix is to have multiple copies of the data on multiple servers. But now you have a new problem: **how do all the copies agree on what the data looks like?**

If a user updates their email address, which server should accept the write? What if two servers accept conflicting writes at the same time? What if the network between servers goes down mid-update — does half the cluster think the email changed and the other half thinks it didn't?

This is the **distributed consensus problem**: getting multiple machines to agree on a single, consistent sequence of operations, even when machines crash, networks fail, and messages get lost or delayed.

This project is a from-scratch Go implementation of **Raft**, an algorithm that solves exactly this problem. Raft was designed in 2014 by Diego Ongaro and John Ousterhout specifically to be understandable (unlike its predecessor Paxos, which is notoriously difficult to reason about).

**In practical terms:** This codebase gives you a cluster of 5 servers that elect a leader, replicate a command log across all servers, and continue operating correctly even when servers crash or network connections break. On top of Raft, a distributed key-value store demonstrates the algorithm in action.

---

## 2. What Is Consensus and Why Does It Matter

**Consensus** means a group of machines agrees on the same sequence of decisions, even when some of those machines fail.

### Why it matters in the real world

Every major distributed system relies on consensus under the hood:

- **etcd** (used by Kubernetes) uses Raft to store cluster configuration
- **CockroachDB** uses Raft to replicate data across nodes
- **Consul** (by HashiCorp) uses Raft for service discovery
- **TiKV** (used by TiDB) uses Raft for distributed storage

Without consensus, distributed systems are vulnerable to **split-brain** (two parts of the system disagree on the truth), **data loss** (a crashed server takes uncommitted data with it), and **inconsistency** (clients see different answers depending on which server they ask).

### The fundamental tension

There is a fundamental tension in distributed systems between:

- **Availability** — the system responds to every request
- **Consistency** — every read reflects the most recent write
- **Partition tolerance** — the system works despite network failures

The CAP theorem proves you can only have two of the three. Raft chooses **consistency and partition tolerance** at the expense of availability during certain failure modes. When the cluster cannot form a majority (e.g., 3 out of 5 servers are down), it stops accepting writes rather than risk inconsistency.

---

## 3. The Raft Algorithm — Explained Simply

Raft breaks consensus into three sub-problems:

### 3.1 Leader Election

At any given time, one server in the cluster is the **leader**. The leader is the only server that accepts client commands and decides the order of operations. Every other server is a **follower** — it just copies what the leader tells it.

How a leader gets elected:

1. Every server starts as a follower
2. Each follower has a randomized **election timeout** (150–300ms in this implementation)
3. If a follower doesn't hear from a leader within that timeout, it assumes the leader is dead
4. It promotes itself to **candidate**, increments its **term** (a logical clock), votes for itself, and asks all other servers for their votes
5. If it gets votes from a **majority** (3 out of 5 in a 5-node cluster), it becomes the new leader
6. The new leader immediately starts sending heartbeats to prevent other elections

The randomized timeout is critical — without it, multiple servers would time out simultaneously and split the vote. With randomization, usually one server times out first and wins before the others even start.

### 3.2 Log Replication

Once a leader is elected, it manages a **log** — an ordered sequence of commands from clients:

1. Client sends a command (e.g., "set key=value") to the leader
2. Leader appends the command to its own log
3. Leader sends the new entry to all followers via **AppendEntries** RPCs
4. Each follower appends the entry to its own log and acknowledges
5. Once a **majority** of servers have the entry, the leader considers it **committed**
6. The leader notifies followers that the entry is committed
7. Every server applies committed entries to its state machine (e.g., the key-value store)

The key insight: an entry is only committed once a majority has it. This means even if the leader crashes right after committing, the entry is safe because a majority still has it, and any future leader must come from that majority.

### 3.3 Safety

Raft enforces several rules to prevent inconsistency:

- **Election restriction**: A candidate can only win an election if its log is at least as up-to-date as a majority of the cluster. This prevents a stale server from becoming leader and overwriting committed entries.
- **Leader append-only**: A leader never deletes or overwrites entries in its own log.
- **Commit rule**: A leader only commits entries from its own term, not entries left over from previous leaders' terms (this prevents a subtle bug where an entry could be committed and then overwritten).

---

## 4. How This Codebase Is Organized

```
raft-consensus-go/
│
├── cmd/raft/main.go                 # Entry point — wires everything together
│
├── internal/                        # Core implementation (not importable externally)
│   ├── raft/
│   │   ├── raft.go                  # The brain: state machine, event loops, RPC handlers
│   │   ├── state.go                 # Role enum: Follower, Candidate, Leader
│   │   ├── log.go                   # Log data structure (1-based indexing)
│   │   ├── config.go                # Timeouts, cluster configuration
│   │   └── errors.go               # Domain-specific error types
│   │
│   ├── election/
│   │   ├── timer.go                 # Randomized election timeout
│   │   └── election.go              # Vote counting and quorum tracking
│   │
│   ├── replication/
│   │   ├── replication.go           # nextIndex/matchIndex management
│   │   └── heartbeat.go             # Heartbeat identification helpers
│   │
│   ├── rpc/
│   │   └── types.go                 # RequestVote and AppendEntries message structs
│   │
│   ├── persistence/
│   │   ├── storage.go               # Storage interface
│   │   ├── file_store.go            # File-based persistent storage (JSON + atomic rename)
│   │   └── memory_store.go          # In-memory storage (for tests)
│   │
│   └── transport/
│       ├── transport.go             # Transport and RPCHandler interfaces
│       ├── grpc_transport.go        # HTTP/JSON-based network transport
│       └── mock_transport.go        # Simulated network (for tests)
│
├── pkg/kvstore/                     # Demo application built on Raft
│   ├── store.go                     # Key-value state machine
│   ├── http.go                      # HTTP API (GET/PUT/DELETE)
│   └── applier.go                   # Bridges Raft commit channel to KV store
│
├── test/
│   ├── integration/                 # Multi-node cluster tests
│   │   ├── helpers.go               # TestCluster utility
│   │   ├── election_test.go         # Leader election scenarios
│   │   ├── replication_test.go      # Log replication scenarios
│   │   ├── partition_test.go        # Network partition scenarios
│   │   ├── persistence_test.go      # Crash recovery scenarios
│   │   ├── heartbeat_test.go        # Heartbeat behavior
│   │   ├── intermittent_test.go     # Flaky network scenarios
│   │   ├── kvstore_test.go          # End-to-end KV operations
│   │   └── benchmark_test.go        # Performance benchmarks + stress test
│   └── chaos/
│       ├── chaos.go                 # Random failure injection framework
│       └── chaos_test.go            # Chaos test scenarios
│
├── scripts/                         # Operational scripts
├── Dockerfile                       # Multi-stage Docker build
├── docker-compose.yml               # 5-node cluster orchestration
└── Makefile                         # Build, test, lint, deploy targets
```

### Why `internal/` vs `pkg/`?

Go's `internal/` directory enforces that these packages can only be imported by code within this module. The Raft internals are not meant to be a public library — they are implementation details. The `pkg/kvstore/` directory is the user-facing application built on top of Raft, and could theoretically be imported by other projects.

---

## 5. Implementation Deep Dive

### 5.1 The Raft Node

The central type in the entire codebase is `RaftNode` in `internal/raft/raft.go`. It contains everything a single Raft server needs:

**Persistent state** (must survive crashes — written to disk before responding to any RPC):
- `currentTerm` — The latest term this server has seen. Starts at 0. Monotonically increases.
- `votedFor` — Which candidate this server voted for in the current term. -1 if it hasn't voted.
- `log` — The ordered sequence of log entries.

**Volatile state** (lost on crash, rebuilt from log):
- `commitIndex` — The highest log entry known to be committed (replicated to a majority).
- `lastApplied` — The highest log entry that has been applied to the state machine.

**Leader-only state** (reinitialized after each election):
- `nextIndex[peer]` — For each follower, the next log entry the leader will send. Initialized to leader's last log index + 1.
- `matchIndex[peer]` — For each follower, the highest log entry known to be replicated on that follower. Initialized to 0.
- `lastContact[peer]` — Timestamp of the last successful RPC response from each peer. Used by the leader to detect that it has lost contact with the majority.

**The event loop:**

The node runs a main loop (`run()`) that dispatches to one of three role-specific sub-loops:

```
run() loop:
  ├── runFollower()   — Wait for heartbeats or election timeout
  ├── runCandidate()  — Request votes, count responses, handle timeout
  └── runLeader()     — Send heartbeats, replicate logs, check liveness
```

Each sub-loop runs until a state transition occurs (e.g., follower becomes candidate), then returns control to `run()`, which dispatches to the new role's handler. This gives clean separation between roles — there is no tangled state machine with a giant switch statement.

**Channels drive everything:**

Instead of polling or callbacks, the node uses Go channels for event signaling:

| Channel | Buffer Size | Purpose |
|---------|-------------|---------|
| `applyCh` | 100 | Delivers committed log entries to the state machine (KV store) |
| `resetTimerCh` | 1 | Tells follower to reset its election timeout (heartbeat received) |
| `stepDownCh` | 1 | Tells leader/candidate to step down (higher term discovered) |
| `triggerAECh` | 1 | Tells leader to send AppendEntries immediately (new client command) |
| `stopCh` | 0 (unbuffered) | Closed to signal all goroutines to shut down |

The signaling channels have buffer size 1 and use `select/default` for non-blocking sends. This means "signal at most once" — if a signal is already pending, a second signal is harmlessly dropped. This prevents goroutine leaks and deadlocks.

### 5.2 Leader Election

**Election timer** (`internal/election/timer.go`):

Each node has a timer that fires after a random duration between `ElectionTimeoutMin` (150ms) and `ElectionTimeoutMax` (300ms). The randomization is the key to avoiding split votes — it makes it very likely that one node times out before the others.

The timer is reset whenever:
- The node receives a valid heartbeat from the current leader
- The node grants a vote to a candidate

If the timer fires without being reset, the node assumes the leader is dead and starts an election.

**What happens during an election:**

```
1. Increment currentTerm
2. Vote for self (votedFor = myID)
3. Persist currentTerm and votedFor to disk
4. Reset election timer
5. Send RequestVote RPC to every peer (in parallel, one goroutine each)
6. Collect responses:
   - If response has higher term → step down to follower immediately
   - If vote granted → increment vote count
   - If votes >= majority → become leader
7. If election timer fires again → start a new election with term+1
```

**The RequestVote handler** (what a node does when it receives a vote request):

```
1. If requester's term > my term → update my term, reset votedFor
2. If requester's term < my term → reject (stale candidate)
3. If I already voted for someone else this term → reject
4. If requester's log is less up-to-date than mine → reject
5. Otherwise → grant the vote, reset my election timer
```

**"Up-to-date" log comparison:**

Raft compares logs by looking at the last entry. The log with the higher last-entry term wins. If terms are equal, the longer log wins. This ensures that a candidate with missing committed entries can never become leader.

### 5.3 Log Replication

Once a leader is elected, it handles all client commands:

**Client command flow:**

```
Client → HTTP PUT /kv/mykey → KV Store HTTP handler
  → raft.Submit("set mykey myvalue")
  → Leader appends entry to its log
  → Leader signals triggerAECh (send AppendEntries immediately)
  → Leader sends AppendEntries RPC to all followers in parallel
  → Followers append entry and respond
  → Leader counts acknowledgments
  → Once majority acknowledges → leader advances commitIndex
  → Committed entry sent to applyCh → KV store applies it
```

**AppendEntries RPC — what the leader sends:**

Every heartbeat (every 50ms) or immediately after a new client command, the leader sends each follower:

```
{
  Term:         leader's current term
  LeaderID:     leader's node ID
  PrevLogIndex: index of the entry just before the new ones
  PrevLogTerm:  term of that entry
  Entries:      new log entries to append (empty for heartbeat)
  LeaderCommit: leader's commit index
}
```

**The consistency check:**

When a follower receives AppendEntries, it checks: "Do I have an entry at `PrevLogIndex` with term `PrevLogTerm`?" If yes, the logs are consistent up to that point, and the follower appends the new entries. If no, the follower rejects the request.

On rejection, the leader decrements `nextIndex` for that follower and retries with an earlier `PrevLogIndex`. This process is called **log backtracking** — the leader walks backwards through the follower's log until it finds a point of agreement, then sends all entries from there forward.

**Commit index advancement (leader only):**

After receiving successful AppendEntries responses, the leader checks:

```
For each index N from lastIndex down to commitIndex+1:
  1. The entry at index N must be from the current term
  2. Count how many servers have this entry (self + peers with matchIndex >= N)
  3. If count >= majority → set commitIndex = N, stop searching
```

The rule "entry must be from the current term" is critical. Without it, a new leader could commit entries from a previous term that might later be overwritten by a legitimate leader from that term. This is one of the subtlest correctness properties in Raft (see Section 5.4.2 of the paper).

### 5.4 Persistence and Crash Recovery

**What gets persisted:**

Three things must survive crashes: `currentTerm`, `votedFor`, and the `log`. Without these, a restarted node could vote twice in the same term or lose committed entries.

**The Storage interface** (`internal/persistence/storage.go`):

```go
type Storage interface {
    Save(state *PersistentState) error
    Load() (*PersistentState, error)
    SaveTermAndVote(term, votedFor int) error
    AppendLogEntries(entries []LogEntry) error
    TruncateLog(fromIndex int) error
    Close() error
}
```

**Two implementations:**

| Implementation | Use Case | Durability |
|---------------|----------|------------|
| `FileStore` | Production | Writes JSON to disk with atomic rename |
| `MemoryStore` | Tests | In-memory map with deep-copy semantics |

**FileStore's atomic write strategy:**

```
1. Serialize state to JSON
2. Write to a temporary file (raft_state.json.tmp)
3. Rename temp file to final name (raft_state.json)
```

The rename operation is atomic on POSIX filesystems — either the entire new file replaces the old one, or nothing happens. This prevents corrupted state files from half-written data if the process crashes mid-write.

**When persistence is called:**

`persistTermAndVote()` is called synchronously at every point where `currentTerm` or `votedFor` changes:
- When a candidate starts an election (term incremented, voted for self)
- When a follower grants a vote
- When any node discovers a higher term from an RPC

This is a blocking operation — the node does not respond to the RPC until the state is on disk. This is necessary for correctness but adds latency (the tradeoff is discussed in Section 8).

**Recovery on restart:**

When a node starts with a configured storage backend:
```
1. Load persisted state (currentTerm, votedFor, log entries)
2. Restore in-memory state
3. Start as follower with the recovered term
4. Committed entries will be re-applied as the node catches up
```

### 5.5 Network Transport

**The Transport interface** (`internal/transport/transport.go`):

```go
type Transport interface {
    SendRequestVote(ctx context.Context, target int, req *RequestVoteRequest) (*RequestVoteResponse, error)
    SendAppendEntries(ctx context.Context, target int, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
    Start() error
    Stop() error
}
```

This interface abstracts the network layer, allowing the same Raft logic to run over real HTTP or over a simulated in-memory network.

**GRPCTransport** (`internal/transport/grpc_transport.go`):

Despite the name (a historical artifact), this is an **HTTP/JSON** transport. Each RPC is a simple HTTP POST:

| Endpoint | RPC |
|----------|-----|
| `POST /raft/request-vote` | RequestVote |
| `POST /raft/append-entries` | AppendEntries |

The HTTP client is configured with connection pooling (100 max idle connections, 10 per host) for efficiency. Each RPC has a 100ms timeout enforced via `context.WithTimeout`.

**MockTransport** (`internal/transport/mock_transport.go`):

For tests, a `MockNetwork` routes RPCs as direct function calls without any actual networking. It supports:

- `Disconnect(from, to)` — Simulates a one-way network partition
- `DisconnectNode(id)` — Completely isolates a node
- `Reconnect(from, to)` / `ReconnectNode(id)` — Heals partitions

This makes tests deterministic — no flakiness from real network timeouts or port conflicts.

### 5.6 The Key-Value Store

The KV store (`pkg/kvstore/`) is the **application** built on top of Raft. It demonstrates that the consensus layer works by providing a distributed, fault-tolerant key-value database.

**Data flow:**

```
HTTP Request → KV HTTP Handler → Raft Submit → Log Replication → Commit → applyCh → Applier → KV Store
```

**State machine** (`pkg/kvstore/store.go`):

A thread-safe `map[string]string` with three operations:
- `Put(key, value)` — Set a key
- `Get(key)` — Read a key (local, no consensus needed)
- `Delete(key)` — Remove a key

Commands are serialized as JSON and submitted to Raft as opaque byte slices. When Raft commits an entry, the `Applier` goroutine deserializes the command and applies it to the store.

**HTTP API** (`pkg/kvstore/http.go`):

| Method | Path | Description |
|--------|------|-------------|
| `PUT` | `/kv/{key}` | Set a value. Body: `{"value": "..."}`. Returns 202 (accepted, not yet committed). |
| `GET` | `/kv/{key}` | Read a value. Returns 200 or 404. Reads local state (no consensus round). |
| `DELETE` | `/kv/{key}` | Delete a key. Returns 202. |
| `GET` | `/status` | Node status: role, term, leader ID, log length, commit index. |
| `GET` | `/cluster` | Cluster membership info. |

Write requests on a **follower** return `503 Service Unavailable` with a `leader_id` field so the client can redirect to the leader.

---

## 6. Concurrency Model

Raft is inherently concurrent — a node must simultaneously handle incoming RPCs, send outgoing RPCs, manage timers, and apply committed entries. Here is how this implementation manages concurrency:

### Single mutex for all shared state

A single `sync.Mutex` (`mu`) protects all of `RaftNode`'s fields. Every RPC handler locks `mu` at the start and holds it for the entire handler. This is simple and correct, but means RPCs are serialized.

**Why a single mutex?** Raft's correctness depends on atomic read-modify-write sequences. For example, `HandleRequestVote` must atomically check `currentTerm`, `votedFor`, and log state, then update `votedFor` and respond. With fine-grained locks, it would be extremely easy to introduce races.

### Goroutine-per-peer for RPC sending

When the leader sends AppendEntries or a candidate sends RequestVote, it spawns one goroutine per peer. These goroutines run concurrently, each sending an RPC and processing the response independently. The lock is **not held** during the network call — it is acquired, state is read, lock is released, RPC is sent, lock is reacquired, response is processed.

### Background goroutines

| Goroutine | Lifetime | Purpose |
|-----------|----------|---------|
| `run()` | Entire node lifetime | Main event loop (dispatches to role handlers) |
| `applyCommittedEntries()` | Entire node lifetime | Reads commitIndex, sends entries to applyCh |
| Per-peer vote senders | One election | Send RequestVote RPCs in parallel |
| Per-peer AE senders | One heartbeat round | Send AppendEntries RPCs in parallel |

### Shutdown

Graceful shutdown uses the `stopCh` channel (unbuffered, closed to signal). Every `select` statement includes a `case <-stopCh:` branch. When `Stop()` is called:

1. Close `stopCh` — all goroutines see this and exit their loops
2. Stop the election timer
3. Persist final state to storage
4. Mark node as stopped

---

## 7. Safety Guarantees

This implementation guarantees all five properties from the Raft paper:

| Property | Guarantee | How It's Enforced |
|----------|-----------|-------------------|
| **Election Safety** | At most one leader per term | Each node votes at most once per term (`votedFor` persisted). Majority required to win. Two majorities cannot exist in the same term. |
| **Leader Append-Only** | Leader never overwrites/deletes its own entries | Leader only calls `log.Append()`, never `log.Truncate()`. Only followers truncate on conflict. |
| **Log Matching** | If two logs have an entry with the same index and term, they are identical up to that index | `PrevLogIndex`/`PrevLogTerm` consistency check in AppendEntries. Follower rejects if mismatch, leader backs up. |
| **Leader Completeness** | If an entry is committed in term T, all leaders in terms > T will have it | Election restriction: candidate must have log at least as up-to-date as majority. Committed = replicated to majority. Any future leader must come from a majority that has the entry. |
| **State Machine Safety** | If a server applies entry at index I, no other server applies a different entry at I | Combines all above. Only committed entries are applied. Committed entries are never overwritten. |

---

## 8. Tradeoffs and Design Decisions

### 8.1 HTTP/JSON instead of gRPC

**Decision:** Use HTTP/JSON for inter-node communication instead of gRPC with Protocol Buffers.

**Tradeoff:**
- (+) Zero external dependencies (no protoc compiler, no generated code)
- (+) Easy to debug with curl
- (+) Simpler build process
- (-) Higher serialization overhead (JSON vs protobuf)
- (-) No streaming RPCs (would help with large log transfers)
- (-) No built-in connection multiplexing

**Why it's acceptable:** For a demonstration/learning project, simplicity trumps raw performance. The serialization overhead is negligible compared to network latency and disk I/O. A production system would switch to gRPC.

### 8.2 Single mutex vs fine-grained locking

**Decision:** One `sync.Mutex` protects all of `RaftNode`'s state.

**Tradeoff:**
- (+) Impossible to have lock ordering bugs or deadlocks (only one lock)
- (+) RPC handlers are trivially atomic
- (+) Easy to reason about correctness
- (-) RPCs are serialized — only one RPC handler runs at a time
- (-) Leader heartbeats block if another RPC is being processed

**Why it's acceptable:** A 5-node cluster with 50ms heartbeats generates ~80 RPCs/second per node. With RPC handler durations under 1ms (dominated by local computation, not holding the lock during network calls), serialization is not a bottleneck. A production system handling thousands of concurrent clients would need a reader-writer lock or lock-free structures for the hot path.

### 8.3 Linear log backtracking (one index at a time)

**Decision:** When a follower rejects AppendEntries (log inconsistency), the leader decrements `nextIndex` by 1 and retries.

**Tradeoff:**
- (+) Dead simple to implement
- (+) Correct in all cases
- (-) If a follower is 1000 entries behind, it takes 1000 round trips to catch up

**Optimized alternative (not implemented):** The follower could include the conflicting term and the first index of that term in its rejection. The leader could then jump directly to the right `nextIndex`. This reduces O(N) round trips to O(number of conflicting terms).

**Why it's acceptable:** In practice, followers rarely fall far behind. Log divergence typically happens after a leader failure, and the divergence is at most a few entries. For very long divergences (e.g., a node was offline for hours), the optimization matters — but that scenario also suggests a snapshot mechanism would be more appropriate.

### 8.4 No log compaction / snapshots

**Decision:** This implementation does not support snapshots or log compaction.

**Tradeoff:**
- (+) Significantly simpler implementation
- (+) Full log history is always available for debugging
- (-) Log grows without bound — memory and disk usage increase over time
- (-) A node that restarts after a long outage must replay the entire log
- (-) Cannot compact old committed entries

**Why it's acceptable:** For a learning project and short-lived clusters, unbounded log growth is fine. A production system must implement snapshots (Section 7 of the Raft paper): periodically capture the full state machine, discard log entries up to that point, and send the snapshot to lagging followers.

### 8.5 JSON file persistence vs WAL

**Decision:** Use a single JSON file (`raft_state.json`) for all persistent state, rewritten on every update.

**Tradeoff:**
- (+) Simple implementation, easy to inspect state by reading the file
- (+) Atomic rename ensures no partial writes
- (-) Rewrites entire file on every `SaveTermAndVote` call (which happens on every election and vote)
- (-) With large logs, serializing the full state becomes expensive
- (-) Not suitable for high write throughput

**Optimized alternative:** A write-ahead log (WAL) would append only new entries to disk, with periodic full snapshots. This is what production implementations use (etcd, for example, uses a WAL + snap directory).

### 8.6 Synchronous persistence in the RPC path

**Decision:** Persistence calls (`persistTermAndVote()`) happen synchronously before responding to RPCs.

**Tradeoff:**
- (+) Correct — the Raft paper requires persistent state to be updated before responding
- (-) Adds disk I/O latency to every vote and term change
- (-) On slow disks, this can delay election convergence

**Why it's necessary:** If a node responds "I voted for you" but crashes before writing that vote to disk, it could restart and vote for a different candidate in the same term, violating Election Safety. The persistence must complete before the response is sent.

### 8.7 Reads are local (not linearizable)

**Decision:** GET requests to the KV store read directly from local state without going through Raft.

**Tradeoff:**
- (+) Fast reads — no consensus round trip needed
- (-) Stale reads are possible: a follower might return data that doesn't reflect the latest committed write

**Linearizable alternative:** The leader could confirm it is still the leader (by getting heartbeat acknowledgments from a majority) before serving a read. This is called a "read index" protocol. It adds one round trip but guarantees freshness.

### 8.8 Leader liveness detection

**Decision:** The leader tracks `lastContact` timestamps for each peer. A background ticker (every 75ms) checks whether a majority of peers have responded within `ElectionTimeoutMin` (150ms). If not, the leader steps down.

**Tradeoff:**
- (+) Prevents a partitioned leader from accepting writes that can't be committed
- (+) Faster recovery than waiting for followers to start elections
- (-) Aggressive: a temporary network blip can cause unnecessary leader step-down
- (-) Adds complexity to the leader event loop

### 8.9 Buffered `applyCh` (size 100)

**Decision:** The channel that delivers committed entries to the state machine has a buffer of 100.

**Tradeoff:**
- (+) Decouples Raft commit speed from state machine application speed
- (+) Prevents Raft from blocking on a slow state machine
- (-) If the state machine is extremely slow, the buffer fills and Raft blocks anyway
- (-) On crash, up to 100 committed-but-not-yet-applied entries are in the buffer (they'll be re-applied on restart since they're in the log)

---

## 9. What This Implementation Does NOT Do

These features are described in the Raft paper or its extensions but are **not** implemented here:

| Feature | Why Omitted | Impact |
|---------|-------------|--------|
| **Log compaction / Snapshots** | Significant complexity (Section 7 of paper) | Log grows forever; slow restarts |
| **Membership changes** | Joint consensus algorithm is complex (Section 6 of paper) | Cluster size is fixed at startup |
| **Pre-vote protocol** | Optimization to prevent disruptions from partitioned nodes | A partitioned node can cause unnecessary term bumps when it rejoins |
| **Batched AppendEntries** | Optimization for throughput | Each heartbeat carries all pending entries anyway |
| **Pipeline replication** | Optimization for latency | Single in-flight AppendEntries per peer |
| **Lease-based reads** | Optimization for read performance | Reads are local but potentially stale |
| **Client session tracking** | Prevents duplicate command application | Clients must handle idempotency |
| **InstallSnapshot RPC** | Required for snapshot-based catch-up | Lagging followers must replay entire log |

---

## 10. How to Build, Deploy, and Test

### Prerequisites

- **Go 1.22+** — [Download](https://go.dev/dl/)
- **Docker & Docker Compose** — For cluster deployment ([Install Docker](https://docs.docker.com/get-docker/))
- **Make** — For build commands (usually pre-installed on macOS/Linux)
- **curl** — For interacting with the HTTP API

### Build from source

```bash
# Clone the repository
git clone https://github.com/ssaurabh760/raft-consensus.git
cd raft-consensus

# Build the binary
make build
# Output: bin/raft-node
```

### Run a local 5-node cluster with Docker

This is the easiest way to see the system in action:

```bash
# Build Docker images and start 5 nodes
docker compose up -d --build

# Check that all 5 nodes are running
docker compose ps

# View logs from a specific node
docker compose logs -f node1
```

The cluster maps these ports to your host machine:

| Node | HTTP API Port | RPC Port |
|------|--------------|----------|
| node1 | localhost:8001 | 9001 |
| node2 | localhost:8002 | 9002 |
| node3 | localhost:8003 | 9003 |
| node4 | localhost:8004 | 9004 |
| node5 | localhost:8005 | 9005 |

### Interact with the cluster

```bash
# Check which node is the leader
for port in 8001 8002 8003 8004 8005; do
  echo "Node on port $port:"
  curl -s localhost:$port/status | python3 -m json.tool
  echo
done

# Write a value (send to leader — check /status to find it)
curl -X PUT localhost:8001/kv/hello -d '{"value": "world"}'

# Read the value (can read from any node)
curl localhost:8001/kv/hello
curl localhost:8003/kv/hello   # Same data, different node

# Delete a value
curl -X DELETE localhost:8001/kv/hello

# Check cluster membership
curl localhost:8001/cluster
```

### Demonstrate fault tolerance

```bash
# Find the current leader (look for "role": "Leader")
curl -s localhost:8001/status

# Kill the leader container (e.g., if node1 is leader)
docker stop raft-node-1

# Wait a few seconds for re-election, then check for a new leader
sleep 3
for port in 8002 8003 8004 8005; do
  echo "Port $port: $(curl -s localhost:$port/status | python3 -c 'import sys,json; print(json.load(sys.stdin).get("role","unavailable"))' 2>/dev/null)"
done

# Data is still accessible through the new leader
curl localhost:8002/kv/hello

# Restart the stopped node — it will rejoin as a follower
docker start raft-node-1
```

### Run the interactive demo script

```bash
make demo
# This script automatically:
# 1. Checks cluster status
# 2. Writes KV pairs
# 3. Reads from different nodes (shows replication)
# 4. Demonstrates fault tolerance by stopping/restarting a node
```

### Run without Docker (manual 3-node cluster)

```bash
make build

# Terminal 1
./bin/raft-node -id=1 -listen=:9001 -http=:8001 -peers=2=localhost:9002,3=localhost:9003 -data=/tmp/raft-1

# Terminal 2
./bin/raft-node -id=2 -listen=:9002 -http=:8002 -peers=1=localhost:9001,3=localhost:9003 -data=/tmp/raft-2

# Terminal 3
./bin/raft-node -id=3 -listen=:9003 -http=:8003 -peers=1=localhost:9001,2=localhost:9002 -data=/tmp/raft-3
```

### Stop everything

```bash
# Docker
docker compose down

# Remove persistent data volumes too
docker compose down -v
```

---

## 11. Testing Strategy

### Test levels

| Level | Location | What It Tests | Count |
|-------|----------|---------------|-------|
| **Unit** | `internal/*/..._test.go` | Individual functions in isolation | ~70 |
| **Integration** | `test/integration/` | Multi-node cluster behavior | ~40 |
| **Chaos** | `test/chaos/` | Random failures over time | 3 |
| **Benchmark** | `*_benchmark_test.go` | Performance measurement | 6 |
| **Stress** | `test/integration/benchmark_test.go` | 100 concurrent command submissions | 1 |

### Run tests

```bash
# All tests
make test

# All tests with Go's race detector (catches data races)
make test-race

# Generate an HTML coverage report
make coverage
# Then open coverage.html in a browser

# Run a specific test
go test -race -run TestLeaderElection ./test/integration/

# Run benchmarks
go test -bench=. ./internal/raft/
go test -bench=. ./test/integration/

# Run only chaos tests
go test -race -v -timeout 120s ./test/chaos/

# Run with verbose output to see individual test names
go test -race -v ./internal/raft/
```

### What the tests verify

**Election tests:**
- A 5-node cluster elects exactly one leader
- Re-election happens after leader failure
- Split votes resolve (eventually one candidate wins)
- Nodes step down when they discover a higher term
- A node with a stale log cannot win an election

**Replication tests:**
- Entries submitted to the leader are replicated to all followers
- Commit index advances after majority acknowledgment
- A disconnected follower catches up when reconnected
- Concurrent submissions are all replicated correctly

**Partition tests:**
- A leader in a minority partition cannot commit entries
- The majority partition elects a new leader
- After partition heals, the old leader steps down
- Logs are reconciled after partition healing
- No split brain (two leaders in the same term) ever occurs

**Persistence tests:**
- A restarted node recovers its term and votedFor
- A restarted node recovers its log entries
- A cluster survives a leader restart (new election, state preserved)

**Chaos tests:**
- Randomly disconnect and reconnect nodes for 10+ seconds
- Submit commands throughout
- Verify: at most one leader at any time, logs converge after chaos stops

### Linting

```bash
# Run the full lint suite (uses .golangci.yml config)
make lint
```

The `.golangci.yml` enables: errcheck, gosimple, govet, ineffassign, staticcheck, unused, bodyclose, gocritic, gofmt, goimports, misspell, prealloc, unconvert, unparam.

---

## 12. Configuration Reference

### Timeout values

| Parameter | Default | Purpose | Considerations |
|-----------|---------|---------|----------------|
| `ElectionTimeoutMin` | 150ms | Minimum time before follower starts election | Lower = faster failover but more spurious elections |
| `ElectionTimeoutMax` | 300ms | Maximum election timeout | Range should be ~2x min for good randomization |
| `HeartbeatInterval` | 50ms | Leader heartbeat frequency | Must be much less than election timeout (rule of thumb: 10x smaller) |
| `RPCTimeout` | 100ms | Per-RPC deadline | Should be less than election timeout |

**Timing relationship (critical for correctness):**

```
HeartbeatInterval << ElectionTimeoutMin << ElectionTimeoutMax
      50ms        <<      150ms         <<      300ms
```

If `HeartbeatInterval` is too close to `ElectionTimeoutMin`, followers will frequently time out and start unnecessary elections. If `RPCTimeout` is greater than `ElectionTimeoutMin`, a single slow RPC can trigger an election.

### Command-line flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-id` | Yes | — | Unique integer ID for this node |
| `-listen` | Yes | — | Address for Raft RPC server (e.g., `:9001`) |
| `-http` | Yes | — | Address for HTTP KV API (e.g., `:8001`) |
| `-peers` | Yes | — | Comma-separated `id=addr` pairs (e.g., `2=host:9002,3=host:9003`) |
| `-data` | No | (empty) | Directory for persistent storage. If empty, no persistence. |

### Docker Compose environment

Each node in `docker-compose.yml` is configured with:
- Named volume for persistence (`node{i}-data:/data`)
- Bridge network (`raft-net`) for inter-node communication
- `restart: unless-stopped` for automatic restart on crashes
- Service name DNS resolution (e.g., `node2:9002` resolves within the Docker network)

---

## 13. Further Reading

- **The Raft paper:** ["In Search of an Understandable Consensus Algorithm"](https://raft.github.io/raft.pdf) by Diego Ongaro and John Ousterhout — the definitive reference. Read at minimum Sections 1–6 and Figure 2.
- **Raft visualization:** [https://raft.github.io/](https://raft.github.io/) — Interactive animation of election, replication, and partitions.
- **The Raft PhD dissertation:** [https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf](https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf) — Much more detail on cluster membership changes, log compaction, and client interaction.
- **Designing Data-Intensive Applications** by Martin Kleppmann — Chapter 9 covers consensus algorithms in the context of real-world systems.
- **etcd's Raft implementation:** [https://github.com/etcd-io/raft](https://github.com/etcd-io/raft) — A production-grade Go implementation used by Kubernetes.
