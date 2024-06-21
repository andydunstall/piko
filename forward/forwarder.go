package forward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	piko "github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap"
)

type Forwarder struct {
	client *piko.Client

	endpointID string

	ln net.Listener

	logger log.Logger
}

func NewForwarder(endpointID string, client *piko.Client, logger log.Logger) *Forwarder {
	return &Forwarder{
		client:     client,
		endpointID: endpointID,
		logger:     logger,
	}
}

func (f *Forwarder) Forward(ln net.Listener) error {
	f.ln = ln
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		f.logger.Debug(
			"accepted connection",
			zap.String("client", conn.RemoteAddr().String()),
			zap.String("endpoint-id", f.endpointID),
			zap.Error(err),
		)

		go f.forwardConn(conn)
	}
}

func (f *Forwarder) Close() error {
	if f.ln != nil {
		return f.ln.Close()
	}
	return nil
}

func (f *Forwarder) forwardConn(conn net.Conn) {
	defer conn.Close()

	upstream, err := f.client.Dial(context.Background(), f.endpointID)
	if err != nil {
		f.logger.Error(
			"failed to dial endpoint",
			zap.String("endpoint-id", f.endpointID),
			zap.Error(err),
		)
		return
	}

	f.logger.Debug(
		"dialed endpoint",
		zap.String("endpoint-id", f.endpointID),
		zap.Error(err),
	)

	g := &sync.WaitGroup{}
	g.Add(2)
	go func() {
		defer g.Done()
		defer conn.Close()
		// nolint
		io.Copy(conn, upstream)
	}()
	go func() {
		defer g.Done()
		defer upstream.Close()
		// nolint
		io.Copy(upstream, conn)
	}()
	g.Wait()
}
