package upstreams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"

	piko "github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pikotest/workload/upstreams/config"
	"github.com/andydunstall/piko/pkg/log"
)

type HTTPUpstream struct {
	server *httptest.Server
}

func NewHTTPUpstream(endpointID string, conf *config.Config, logger log.Logger) (*HTTPUpstream, error) {
	url, _ := url.Parse(conf.Connect.URL)
	upstream := piko.Upstream{
		URL:    url,
		Logger: logger.WithSubsystem("client"),
	}

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		conf.Connect.Timeout,
	)
	defer connectCancel()

	ln, err := upstream.Listen(connectCtx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	// HTTP server to echo the incoming request body.
	server := httptest.NewUnstartedServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Note can't use io.Copy as not supported by http.ResponseWriter.
			b, err := io.ReadAll(r.Body)
			if err != nil {
				panic(fmt.Sprintf("read body: %s", err.Error()))
			}
			n, err := w.Write(b)
			if err != nil {
				panic(fmt.Sprintf("write bytes: %d: %s", n, err))
			}
		},
	))
	server.Listener = ln
	server.Start()

	return &HTTPUpstream{
		server: server,
	}, nil
}

func (u *HTTPUpstream) Close() {
	u.server.Close()
}

type echoServer struct {
	Listener piko.Listener

	wg sync.WaitGroup
}

func (s *echoServer) Start() {
	s.goServe()
}

func (s *echoServer) Close() {
	// Note as we're using a Piko listener, closing the listener will also
	// close all active connections.
	s.Listener.Close()
	s.wg.Wait()
}

func (s *echoServer) serve() {
	for {
		conn, err := s.Listener.Accept()
		if errors.Is(err, piko.ErrClosed) {
			return
		}
		if err != nil {
			panic("accept: " + err.Error())
		}

		s.goHandle(conn)
	}
}

func (s *echoServer) handle(conn net.Conn) {
	defer conn.Close()

	// Echo server.
	buf := make([]byte, 512)
	for {
		n, err := conn.Read(buf)
		if err == io.EOF {
			return
		}
		if err != nil {
			panic("read: " + err.Error())
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			panic("write: " + err.Error())
		}
	}
}

func (s *echoServer) goServe() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serve()
	}()
}

func (s *echoServer) goHandle(conn net.Conn) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.handle(conn)
	}()
}

type TCPUpstream struct {
	server *echoServer
}

func NewTCPUpstream(endpointID string, conf *config.Config, logger log.Logger) (*TCPUpstream, error) {
	url, _ := url.Parse(conf.Connect.URL)
	upstream := piko.Upstream{
		URL:    url,
		Logger: logger.WithSubsystem("client"),
	}

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		conf.Connect.Timeout,
	)
	defer connectCancel()

	ln, err := upstream.Listen(connectCtx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	server := &echoServer{
		Listener: ln,
	}
	server.Start()

	return &TCPUpstream{
		server: server,
	}, nil
}

func (u *TCPUpstream) Close() {
	u.server.Close()
}
