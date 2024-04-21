package server

import "github.com/andydunstall/pico/pkg/rpc"

type rpcServer struct {
	rpcHandler *rpc.Handler
}

func newRPCServer() *rpcServer {
	server := &rpcServer{
		rpcHandler: rpc.NewHandler(),
	}
	server.rpcHandler.Register(rpc.TypeHeartbeat, server.Heartbeat)
	return server
}

func (s *rpcServer) Handler() *rpc.Handler {
	return s.rpcHandler
}

func (s *rpcServer) Heartbeat(m []byte) []byte {
	return m
}
