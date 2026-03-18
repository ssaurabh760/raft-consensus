package rpc

// Server wraps the RPC handler and manages incoming RPC connections.
// The actual network listening is handled by the transport layer.
type Server struct {
	handler Handler
}

// Handler processes incoming Raft RPCs.
type Handler interface {
	// HandleRequestVote processes an incoming RequestVote RPC.
	HandleRequestVote(req *RequestVoteRequest) (*RequestVoteResponse, error)

	// HandleAppendEntries processes an incoming AppendEntries RPC.
	HandleAppendEntries(req *AppendEntriesRequest) (*AppendEntriesResponse, error)
}

// NewServer creates a new RPC server with the given handler.
func NewServer(handler Handler) *Server {
	return &Server{handler: handler}
}

// RequestVote dispatches a RequestVote RPC to the handler.
func (s *Server) RequestVote(req *RequestVoteRequest) (*RequestVoteResponse, error) {
	return s.handler.HandleRequestVote(req)
}

// AppendEntries dispatches an AppendEntries RPC to the handler.
func (s *Server) AppendEntries(req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	return s.handler.HandleAppendEntries(req)
}
