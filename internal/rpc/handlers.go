package rpc

// RPCMetrics tracks RPC call statistics for observability.
type RPCMetrics struct {
	RequestVoteSent     int64
	RequestVoteReceived int64
	AppendEntriesSent     int64
	AppendEntriesReceived int64
}
