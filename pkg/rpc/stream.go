package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/andydunstall/pico/pkg/conn"
	"github.com/andydunstall/pico/pkg/log"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var (
	ErrStreamClosed = errors.New("stream closed")
)

type message struct {
	Header  *header
	Payload []byte
}

// Stream represents a bi-directional RPC stream between two peers. Either peer
// can send an RPC request to the other.
//
// The stream uses the underlying bi-directional connection to send RPC
// requests, and multiplexes multiple concurrent request/response RPCs on the
// same connection.
//
// Incoming RPC requests are handled in their own goroutine to avoid blocking
// the stream.
type Stream struct {
	conn conn.Conn

	// nextMessageID is the ID of the next RPC message to send.
	nextMessageID *atomic.Uint64

	writeCh chan *message

	// responseHandlers contains channels for RPC responses.
	responseHandlers   map[uint64]chan<- *message
	responseHandlersMu sync.Mutex

	// shutdownCh is closed when the stream is shutdown.
	shutdownCh chan struct{}
	// shutdownErr is the first error that caused the stream to shutdown.
	shutdownErr error
	// shutdown indicates whether the stream is already shutdown.
	shutdown *atomic.Bool

	logger *log.Logger
}

// NewStream creates an RPC stream on top of the given message-oriented
// connection.
func NewStream(conn conn.Conn, logger *log.Logger) *Stream {
	stream := &Stream{
		conn:          conn,
		nextMessageID: atomic.NewUint64(0),
		writeCh:       make(chan *message, 64),
		responseHandlers: make(map[uint64]chan<- *message),
		shutdownCh:    make(chan struct{}),
		shutdown:      atomic.NewBool(false),
		logger:        logger.WithSubsystem("rpc"),
	}
	go stream.reader()
	go stream.writer()

	return stream
}

// RPC sends the given request message to the peer and returns the response or
// an error.
//
// RPC is thread safe.
func (s *Stream) RPC(ctx context.Context, rpcType Type, req []byte) ([]byte, error) {
	header := &header{
		RPCType: rpcType,
		ID:      s.nextMessageID.Inc(),
	}
	msg := &message{
		Header:  header,
		Payload: req,
	}

	ch := make(chan *message, 1)
	s.registerResponseHandler(header.ID, ch)
	defer s.unregisterResponseHandler(header.ID)

	select {
	case s.writeCh <- msg:
	case <-s.shutdownCh:
		return nil, s.shutdownErr
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case resp := <-ch:
		if resp.Header.Flags.ErrNotSupported() {
			return nil, fmt.Errorf("not supported")
		}
		return resp.Payload, nil
	case <-s.shutdownCh:
		return nil, s.shutdownErr
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Stream) Close() error {
	return s.closeStream(ErrStreamClosed)
}

func (s *Stream) reader() {
	defer s.recoverPanic("reader()")

	for {
		b, err := s.conn.ReadMessage()
		if err != nil {
			_ = s.closeStream(fmt.Errorf("read: %w", err))
			return
		}

		var header header
		if err = header.Decode(b); err != nil {
			_ = s.closeStream(fmt.Errorf("decode header: %w", err))
			return
		}
		payload := b[headerSize:]

		s.logger.Debug(
			"message received",
			zap.String("type", header.RPCType.String()),
			zap.Bool("response", header.Flags.Response()),
			zap.Uint64("message_id", header.ID),
			zap.Int("len", len(payload)),
		)

		if header.Flags.Response() {
			s.handleResponse(&message{
				Header: &header,
				Payload: payload,
			})
		} else {
			s.handleRequest(&message{
				Header: &header,
				Payload: payload,
			})
		}

		select {
		case <-s.shutdownCh:
			return
		default:
		}
	}
}

func (s *Stream) writer() {
	defer s.recoverPanic("writer()")

	for {
		select {
		case req := <-s.writeCh:
			if err := s.write(req); err != nil {
				_ = s.closeStream(fmt.Errorf("write: %w", err))
				return
			}

			s.logger.Debug(
				"message sent",
				zap.String("type", req.Header.RPCType.String()),
				zap.Bool("response", req.Header.Flags.Response()),
				zap.Uint64("message_id", req.Header.ID),
				zap.Int("len", len(req.Payload)),
			)
		case <-s.shutdownCh:
			return
		}
	}
}

func (s *Stream) write(req *message) error {
	w, err := s.conn.NextWriter()
	if err != nil {
		return err
	}
	if _, err = w.Write(req.Header.Encode()); err != nil {
		return err
	}
	if len(req.Payload) > 0 {
		if _, err = w.Write(req.Payload); err != nil {
			return err
		}
	}
	return w.Close()
}

func (s *Stream) closeStream(err error) error {
	// Only shutdown once.
	if !s.shutdown.CompareAndSwap(false, true) {
		return ErrStreamClosed
	}

	s.shutdownErr = ErrStreamClosed
	// Close to cancel pending RPC requests.
	close(s.shutdownCh)

	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("close conn: %w", err)
	}

	s.logger.Debug(
		"stream closed",
		zap.Error(err),
	)

	return nil
}

func (s *Stream) handleRequest(m *message) {
	m.Header.Flags.SetResponse()
	s.writeCh <- m
}

func (s *Stream) handleResponse(m *message) {
	// TODO(andydunstall): For now echo the message with the response flag.

	// If no handler is found, it means RPC has already returned so discard
	// the response.
	ch, ok := s.findResponseHandler(m.Header.ID)
	if ok {
		ch <- m
	}
}

func (s *Stream) recoverPanic(prefix string) {
	if r := recover(); r != nil {
		_ = s.closeStream(fmt.Errorf("panic: %s: %v", prefix, r))
	}
}

func (s *Stream) registerResponseHandler(id uint64, ch chan<- *message) {
	s.responseHandlersMu.Lock()
	defer s.responseHandlersMu.Unlock()

	s.responseHandlers[id] = ch
}

func (s *Stream) unregisterResponseHandler(id uint64) {
	s.responseHandlersMu.Lock()
	defer s.responseHandlersMu.Unlock()

	delete(s.responseHandlers, id)
}

func (s *Stream) findResponseHandler(id uint64) (chan<- *message, bool) {
	s.responseHandlersMu.Lock()
	defer s.responseHandlersMu.Unlock()

	ch, ok := s.responseHandlers[id]
	return ch, ok
}
