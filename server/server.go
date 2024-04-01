package server

import "context"

type Server struct {
}

func NewServer(addr string) *Server {
	return &Server{}
}

func (s *Server) Serve() error {
	return nil
}

// Shutdown attempts to gracefully shutdown the server by closing open
// WebSockets and waiting for pending requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO(andydunstall): Must handle shutting down hijacked connections.
	return nil
}
