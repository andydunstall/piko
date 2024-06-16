package tcpproxy

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap"
)

type Server struct {
	conf config.ListenerConfig

	ln net.Listener

	dialer *net.Dialer

	conns   map[net.Conn]struct{}
	connsMu sync.Mutex

	logger       log.Logger
	accessLogger log.Logger
}

func NewServer(
	conf config.ListenerConfig,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("proxy.tcp")
	logger = logger.With(zap.String("endpoint-id", conf.EndpointID))

	s := &Server{
		conf: conf,
		dialer: &net.Dialer{
			Timeout: conf.Timeout,
		},
		conns:        make(map[net.Conn]struct{}),
		logger:       logger,
		accessLogger: logger.WithSubsystem("proxy.tcp.access"),
	}

	return s
}

func (s *Server) Serve(ln net.Listener) error {
	s.ln = ln

	s.logger.Info("starting tcp proxy")

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		s.addConn(conn)
		go s.serveConn(conn)
	}
}

func (s *Server) Close() error {
	if s.ln != nil {
		s.ln.Close()
	}

	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for conn := range s.conns {
		conn.Close()
	}

	return nil
}

func (s *Server) serveConn(c net.Conn) {
	defer s.removeConn(c)
	defer c.Close()

	s.logConnOpened()
	defer s.logConnClosed()

	host, ok := s.conf.Host()
	if !ok {
		// We've already verified the address on boot so don't need to handle
		// the error.
		panic("invalid addr: " + s.conf.Addr)
	}
	upstream, err := s.dialer.Dial("tcp", host)
	if err != nil {
		s.logger.Warn("failed to dial upstream", zap.Error(err))
		return
	}
	defer upstream.Close()

	forward(c, upstream)
}

func (s *Server) addConn(c net.Conn) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()

	s.conns[c] = struct{}{}
}

func (s *Server) removeConn(c net.Conn) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()

	delete(s.conns, c)
}

func (s *Server) logConnOpened() {
	if s.conf.AccessLog {
		s.accessLogger.Info("connection opened")
	} else {
		s.accessLogger.Debug("connection opened")
	}
}

func (s *Server) logConnClosed() {
	if s.conf.AccessLog {
		s.accessLogger.Info("connection closed")
	} else {
		s.accessLogger.Debug("connection closed")
	}
}

func forward(conn1 net.Conn, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer conn1.Close()
		// nolint
		io.Copy(conn1, conn2)
	}()
	go func() {
		defer wg.Done()
		defer conn2.Close()
		// nolint
		io.Copy(conn2, conn1)
	}()
	wg.Wait()
}
