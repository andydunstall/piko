package gossip

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/log"
)

// streamListener listens for incoming stream connections and reads messages
// from those connections.
type streamListener struct {
	ln net.Listener

	state *clusterState

	streamTimeout time.Duration

	metrics *Metrics

	logger log.Logger
}

func newStreamListener(
	ln net.Listener,
	state *clusterState,
	streamTimeout time.Duration,
	metrics *Metrics,
	logger log.Logger,
) *streamListener {
	return &streamListener{
		ln:            ln,
		state:         state,
		streamTimeout: streamTimeout,
		metrics:       metrics,
		logger:        logger,
	}
}

// Serve will accept connections until listener is closed.
func (l *streamListener) Serve() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			l.logger.Warn("failed to accept connection", zap.Error(err))
			continue
		}

		l.logger.Debug(
			"accepted conn",
			zap.String("addr", conn.RemoteAddr().String()),
		)

		l.metrics.ConnectionsInbound.Inc()

		go func() {
			if err := l.handleConn(conn); err != nil {
				l.logger.Warn(
					"failed to handle connection",
					zap.String("addr", conn.RemoteAddr().String()),
					zap.Error(err),
				)
			}
		}()
	}
}

func (l *streamListener) Close() error {
	return l.ln.Close()
}

func (l *streamListener) handleConn(conn net.Conn) error {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(l.streamTimeout))

	trackedReader := newTrackedReader(conn)
	defer func() {
		l.metrics.StreamBytesInbound.Add(float64(trackedReader.NumBytesRead()))
	}()

	trackedWriter := newTrackedWriter(conn)
	defer func() {
		l.metrics.StreamBytesOutbound.Add(float64(trackedWriter.NumBytesWritten()))
	}()

	r := bufio.NewReader(trackedReader)
	w := bufio.NewWriter(trackedWriter)

	firstByte, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	messageType := messageType(firstByte)

	version, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if version != supportedVersion {
		return fmt.Errorf("unsupported version: %d", version)
	}

	switch messageType {
	case messageTypeJoin:
		return l.join(r, w)
	case messageTypeLeave:
		return l.leave(r, w)
	default:
		return fmt.Errorf("unsupported message type: %d", version)
	}
}

func (l *streamListener) join(r io.Reader, w *bufio.Writer) error {
	decoder := newDecoder(r)
	var header joinHeader
	if err := decoder.Decode(&header); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	var delta delta
	if err := decoder.Decode(&delta); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	var digest digest
	if err := decoder.Decode(&digest); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// Apply unknown state from the delta.
	l.state.ApplyDelta(delta)

	// Discover any unknown nodes from the digest.
	l.state.ApplyDigest(digest)

	localMeta := l.state.LocalNodeMetadata()
	encoder := newEncoder(w)
	if err := encoder.Encode(&joinHeader{
		NodeID: localMeta.ID,
		Addr:   localMeta.Addr,
	}); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	// Send our own delta response.
	delta = l.state.Delta(digest, true)
	if err := encoder.Encode(delta); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	return nil
}

func (l *streamListener) leave(r io.Reader, w *bufio.Writer) error {
	decoder := newDecoder(r)
	var header leaveHeader
	if err := decoder.Decode(&header); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	var delta delta
	if err := decoder.Decode(&delta); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// Apply unknown state from the delta.
	l.state.ApplyDelta(delta)

	// Send our own header as an acknowledgement.
	localMeta := l.state.LocalNodeMetadata()
	encoder := newEncoder(w)
	if err := encoder.Encode(&leaveHeader{
		NodeID: localMeta.ID,
		Addr:   localMeta.Addr,
	}); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	return nil
}

// packetListener listens for and handles incoming packets.
type packetListener struct {
	ln net.PacketConn

	state *clusterState

	failureDetector failureDetector

	readBuf []byte

	maxPacketSize int

	metrics *Metrics

	logger log.Logger
}

func newPacketListener(
	ln net.PacketConn,
	state *clusterState,
	failureDetector failureDetector,
	maxPacketSize int,
	metrics *Metrics,
	logger log.Logger,
) *packetListener {
	return &packetListener{
		ln:              ln,
		state:           state,
		failureDetector: failureDetector,
		readBuf:         make([]byte, maxPacketSize),
		maxPacketSize:   maxPacketSize,
		metrics:         metrics,
		logger:          logger,
	}
}

func (l *packetListener) Serve() {
	for {
		n, addr, err := l.ln.ReadFrom(l.readBuf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			l.logger.Warn("failed to read packet", zap.Error(err))
			continue
		}

		l.metrics.PacketBytesInbound.Add(float64(n))

		buf := l.readBuf[:n]
		if err = l.handlePacket(buf); err != nil {
			l.logger.Warn(
				"failed to handle packet",
				zap.String("addr", addr.String()),
				zap.Error(err),
			)
		}
	}
}

func (l *packetListener) Close() error {
	return l.ln.Close()
}

func (l *packetListener) handlePacket(b []byte) error {
	if len(b) < 2 {
		return fmt.Errorf("packet too small: %d", len(b))
	}

	messageType := messageType(b[0])
	version := b[1]
	if version != supportedVersion {
		return fmt.Errorf("unsupported version: %d", version)
	}

	switch messageType {
	case messageTypeDigest:
		return l.digest(b)
	case messageTypeDelta:
		return l.delta(b)
	default:
		return fmt.Errorf("unsupported message type: %d", version)
	}
}

func (l *packetListener) digest(b []byte) error {
	header, digest, err := decodeDigest(b)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// Discover any unknown nodes from the digest.
	l.state.ApplyDigest(digest)

	delta := l.state.Delta(digest, false)
	if err := l.sendDelta(delta, header.Addr); err != nil {
		return fmt.Errorf("send delta: %w", err)
	}

	// If the digest was a request, send our own digest response.
	if header.Request {
		if err := l.sendDigest(
			l.state.Digest(),
			header.Addr,
			false,
		); err != nil {
			return fmt.Errorf("send digest: %w", err)
		}
	}

	return nil
}

func (l *packetListener) delta(b []byte) error {
	header, delta, err := decodeDelta(b)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	l.failureDetector.Report(header.NodeID)

	l.state.ApplyDelta(delta)

	return nil
}

// sendDelta writes entries from the given delta upto the packet size limit.
func (l *packetListener) sendDelta(delta delta, addr string) error {
	localMeta := l.state.LocalNodeMetadata()

	header := deltaHeader{
		NodeID: localMeta.ID,
		Addr:   localMeta.Addr,
	}
	b, err := encodeDelta(header, delta, l.maxPacketSize)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve udp: %s: %w", addr, err)
	}
	if _, err = l.ln.WriteTo(b, udpAddr); err != nil {
		return fmt.Errorf("write packet: %s: %w", addr, err)
	}

	l.metrics.PacketBytesOutbound.Add(float64(len(b)))

	return nil
}

func (l *packetListener) sendDigest(
	digest digest,
	addr string,
	request bool,
) error {
	// Shuffle since we may not be able to send all digest entries.
	rand.Shuffle(len(digest), func(i, j int) {
		digest[i], digest[j] = digest[j], digest[i]
	})

	localMeta := l.state.LocalNodeMetadata()
	header := digestHeader{
		NodeID:  localMeta.ID,
		Addr:    localMeta.Addr,
		Request: request,
	}
	b, err := encodeDigest(header, digest, l.maxPacketSize)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve udp: %s: %w", addr, err)
	}
	if _, err = l.ln.WriteTo(b, udpAddr); err != nil {
		return fmt.Errorf("write packet: %s: %w", addr, err)
	}

	l.metrics.PacketBytesOutbound.Add(float64(len(b)))

	return nil
}
