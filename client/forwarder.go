package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Forwarder manages forwarding incoming connections to another address.
type Forwarder struct {
	// ctx is a context to close the forwarder when canceled.
	ctx context.Context

	// ln is the listener to accept connections on.
	ln *listener

	// addr is the address to forward connections to.
	addr string

	group *errgroup.Group

	logger Logger
}

func newForwarder(ctx context.Context, ln *listener, addr string, logger Logger) *Forwarder {
	group, ctx := errgroup.WithContext(ctx)
	f := &Forwarder{
		ctx:    ctx,
		ln:     ln,
		addr:   addr,
		group:  group,
		logger: logger,
	}

	f.group.Go(func() error {
		return f.accept()
	})

	return f
}

// Wait blocks until the forwarder exits, either due to an error or being
// closed.
func (f *Forwarder) Wait() error {
	err := f.group.Wait()
	if f.ctx.Err() != nil || errors.Is(err, ErrClosed) {
		// Ignore context canceled and shutdown errors as they indicate
		// a graceful shutdown.
		return nil
	}
	return err
}

// Close closes the listener and all active connections.
func (f *Forwarder) Close() error {
	// Close the listener to stop accepting connections. This will also close
	// all active connections since the stream to the Piko server is closed.
	return f.ln.Close()
}

func (f *Forwarder) accept() error {
	defer f.ln.Close()

	for {
		conn, err := f.ln.AcceptWithContext(f.ctx)
		if err != nil {
			return err
		}

		f.group.Go(func() error {
			f.forward(conn)
			return nil
		})
	}
}

func (f *Forwarder) forward(downstream net.Conn) {
	defer downstream.Close()

	dialer := &net.Dialer{}
	upstream, err := dialer.DialContext(f.ctx, "tcp", f.addr)
	if err != nil {
		if f.ctx.Err() != nil {
			// If the context was canceled don't log an error.
			return
		}
		f.logger.Warn(
			"failed to dial upstream",
			zap.String("addr", f.addr),
			zap.Error(err),
		)
		return
	}

	g := &sync.WaitGroup{}
	g.Add(2)

	go func() {
		defer g.Done()
		defer downstream.Close()
		_, err := io.Copy(downstream, upstream)
		if err != nil && !errors.Is(err, io.EOF) {
			f.logger.Error("failure to copy from upstream to downstream", zap.String("addr", f.addr), zap.Error(err))
		}
	}()

	go func() {
		defer g.Done()
		defer upstream.Close()
		_, err := io.Copy(upstream, downstream)
		if err != nil && !errors.Is(err, io.EOF) {
			f.logger.Error("failure to copy from downstream to upstream", zap.String("addr", f.addr), zap.Error(err))
		}
	}()

	g.Wait()
}
