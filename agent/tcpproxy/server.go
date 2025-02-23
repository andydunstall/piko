package tcpproxy

import (
	"fmt"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
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

	s.forward(c, upstream)
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
	if s.conf.AccessLog.Disable {
		s.accessLogger.Debug("connection opened")
	} else {
		s.accessLogger.Info("connection opened")
	}
}

func (s *Server) logConnClosed() {
	if s.conf.AccessLog.Disable {
		s.accessLogger.Debug("connection closed")
	} else {
		s.accessLogger.Info("connection closed")
	}
}

func (s *Server) forward(conn net.Conn, upstream net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer conn.Close()
		_, err := io.Copy(conn, upstream)
		if err != nil {
			s.logger.Debug("copy to conn closed", zap.Error(err))
		}
	}()
	go func() {
		defer wg.Done()
		defer upstream.Close()
		_, err := io.Copy(upstream, conn)
		if err != nil {
			s.logger.Debug("copy to upstream closed", zap.Error(err))
		}
	}()
	wg.Wait()
}
