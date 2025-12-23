package gossip

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/dragonflydb/piko/pkg/log"
)

const (
	streamTimeout = time.Second * 10

	suspicionThreshold = 20
	compactThreshold   = 100
)

type Gossip struct {
	state *clusterState

	config *Config

	streamListener *streamListener
	packetListener *packetListener

	dialer     *net.Dialer
	packetConn net.PacketConn

	metrics *Metrics

	logger log.Logger

	closed     *atomic.Bool
	shutdownCh chan struct{}
}

func New(
	nodeID string,
	config *Config,
	streamLn net.Listener,
	packetLn net.PacketConn,
	watcher Watcher,
	logger log.Logger,
) *Gossip {
	logger = logger.WithSubsystem("gossip")

	logger.Info(
		"starting gossip",
		zap.String("node-id", nodeID),
		zap.String("bind-addr", config.BindAddr),
		zap.String("advertise-addr", config.AdvertiseAddr),
	)

	metrics := newMetrics()

	failureDetector := newAccrualFailureDetector(
		config.Interval*2, 50,
	)
	state := newClusterState(
		nodeID,
		config.AdvertiseAddr,
		failureDetector,
		metrics,
		watcher,
	)

	streamListener := newStreamListener(
		streamLn, state, streamTimeout, metrics, logger,
	)
	go streamListener.Serve()

	packetListener := newPacketListener(
		packetLn, state, failureDetector, config.MaxPacketSize, metrics, logger,
	)
	go packetListener.Serve()

	gossip := &Gossip{
		state:          state,
		config:         config,
		streamListener: streamListener,
		packetListener: packetListener,
		dialer: &net.Dialer{
			Timeout: streamTimeout,
		},
		packetConn: packetLn,
		metrics:    metrics,
		logger:     logger,
		closed:     atomic.NewBool(false),
		shutdownCh: make(chan struct{}),
	}
	gossip.schedule()
	return gossip
}

// UpsertLocal updates the local node state entry with the given key.
func (g *Gossip) UpsertLocal(key, value string) {
	g.state.UpsertLocal(key, value)
}

// DeleteLocal deletes the local state entry with the given key.
func (g *Gossip) DeleteLocal(key string) {
	g.state.DeleteLocal(key)
}

// Node returns the known state for the node with the given ID.
func (g *Gossip) Node(id string) (*NodeState, bool) {
	return g.state.Node(id)
}

// LocalNode returns the state of the local node.
func (g *Gossip) LocalNode() *NodeState {
	return g.state.LocalNode()
}

// Nodes returns the known metadata of each node in the cluster.
func (g *Gossip) Nodes() []NodeMetadata {
	return g.state.Nodes()
}

// Join attempts to join an existing cluster by syncronising with the nodes
// at the given addresses.
//
// The addresses may contain either IP addresses or domain names. When a domain
// name is used, the domain is resolved and each resolved IP address is
// attempted. Will also periodically re-resolve the joined domains and
// attempt to gossip with any unknown nodes. If the port is omitted the
// default bind port is used.
//
// Returns the IDs of joined nodes. Or if addresses were provided by no
// nodes could be joined an error is returned. Note if a domain was provided
// that only resolved to the current node then Join will return nil.
func (g *Gossip) Join(addrs []string) ([]string, error) {
	if len(addrs) == 0 {
		return nil, nil
	}

	var joined []string
	var lastJoinErr error
	for _, unresolvedAddr := range addrs {
		unresolvedAddr = g.ensurePort(unresolvedAddr)
		resolvedAddrs, err := resolveAddr(unresolvedAddr)
		if err != nil {
			return nil, fmt.Errorf("resolve: %s: %w", unresolvedAddr, err)
		}

		// TODO(andydunstall): If unresolvedAddr contains a domain, store
		// and periodically re-resolve.

		if len(resolvedAddrs) == 0 {
			g.logger.Warn(
				"join: domain did not resolve any addresses",
				zap.String("addr", unresolvedAddr),
			)
			continue
		}

		for _, addr := range resolvedAddrs {
			nodeID, err := g.join(addr)
			if err != nil {
				lastJoinErr = err

				g.logger.Warn(
					"failed to join node",
					zap.String("addr", addr),
					zap.Error(err),
				)
			} else {
				joined = append(joined, nodeID)
			}
		}
	}

	// Return an error if we couldn't join any resolved addresses (if there
	// were no resolved addresses return nil).
	if len(joined) == 0 && lastJoinErr != nil {
		return nil, lastJoinErr
	}
	return joined, nil
}

// Leave gracefully leaves the cluster.
//
// This block while it attempts to notify upto 3 nodes in the cluster that the
// node is leaving to ensure the status update is propagated.
//
// After the node has left it's state should not be updated again.
//
// Returns an error if no nodes could be notified.
func (g *Gossip) Leave() error {
	g.state.LeaveLocal()

	knownNodes := g.state.Nodes()
	rand.Shuffle(len(knownNodes), func(i, j int) {
		knownNodes[i], knownNodes[j] = knownNodes[j], knownNodes[i]
	})

	// Attempt to sync with upto 3 known nodes to ensure the 'left' status is
	// propagated.
	notified := 0
	var lastLeaveErr error
	for _, node := range knownNodes {
		if node.ID == g.state.LocalNodeMetadata().ID {
			// Ignore ourselves.
			continue
		}
		if node.Left || node.Unreachable {
			// Ignore left/unreachable nodes.
			continue
		}

		if err := g.leave(node.Addr); err != nil {
			g.logger.Warn(
				"failed to send leave to node",
				zap.String("node-id", node.ID),
				zap.Error(err),
			)
			lastLeaveErr = err
		} else {
			g.logger.Info(
				"notified node of leave",
				zap.String("node-id", node.ID),
			)

			notified++

			if notified > 3 {
				// If we've notified 3 nodes thats enough to be confident the
				// update will be propagated.
				return nil
			}
		}

	}

	if notified > 0 {
		return nil
	}
	return lastLeaveErr
}

func (g *Gossip) Metrics() *Metrics {
	return g.metrics
}

// Close stops gossiping and closes all listeners.
//
// To leave gracefully, first call Leave, otherwise other nodes in the
// cluster will detect this nodes as failed rather than as having left.
func (g *Gossip) Close() error {
	if !g.closed.CompareAndSwap(false, true) {
		// Already closed.
		return nil
	}

	close(g.shutdownCh)

	var errs error
	if err := g.streamListener.Close(); err != nil {
		errs = errors.Join(errs, err)
	}
	if err := g.packetListener.Close(); err != nil {
		errs = errors.Join(errs, err)
	}
	return errs
}

// schedule gossips at the configured rate.
func (g *Gossip) schedule() {
	go g.scheduleFunc(g.config.Interval, func() {
		if err := g.gossipRound(); err != nil {
			g.logger.Warn("gossip round failed", zap.Error(err))
		}
	})
	go g.scheduleFunc(g.config.Interval, func() {
		g.state.UpdateLiveness(float64(suspicionThreshold))
	})
	go g.scheduleFunc(g.config.Interval*10, func() {
		g.state.CompactLocal(compactThreshold)
	})
	go g.scheduleFunc(g.config.Interval*10, func() {
		g.state.RemoveExpired()
	})
}

func (g *Gossip) scheduleFunc(interval time.Duration, f func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Add 10% jitter to avoid nodes synchronising.
			jitterMs := (rand.Int63() % interval.Milliseconds()) / 10
			select {
			case <-time.After(time.Duration(jitterMs) * time.Millisecond):
				f()
			case <-g.shutdownCh:
				return
			}

		case <-g.shutdownCh:
			return
		}
	}
}

// gossipRound initiates a round of gossip.
func (g *Gossip) gossipRound() error {
	// Select a random live node to gossip with.
	nodes := g.state.LiveNodes()
	if len(nodes) > 0 {
		node := nodes[rand.Int()%len(nodes)]
		if err := g.gossip(node); err != nil {
			return fmt.Errorf("gossip: %s: %w", node.ID, err)
		}
	}

	// Select a random unreachable node to gossip with.
	//
	// We continue to gossip with unreachable nodes to avoid the case where two
	// healthy nodes both consider one another unreachable so never attempt to
	// gossip with each other.
	nodes = g.state.UnreachableNodes()
	if len(nodes) > 0 {
		node := nodes[rand.Int()%len(nodes)]
		if err := g.gossip(node); err != nil {
			return fmt.Errorf("gossip: %s: %w", node.ID, err)
		}
	}

	return nil
}

func (g *Gossip) gossip(node NodeMetadata) error {
	var buf bytes.Buffer
	_ = buf.WriteByte(uint8(messageTypeDigest))
	_ = buf.WriteByte(supportedVersion)

	encoder := newEncoder(&buf)

	localMeta := g.state.LocalNodeMetadata()
	if err := encoder.Encode(&digestHeader{
		NodeID:  localMeta.ID,
		Addr:    localMeta.Addr,
		Request: true,
	}); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if buf.Len() > g.config.MaxPacketSize {
		return fmt.Errorf(
			"max packet size too small for header: %d < %d",
			g.config.MaxPacketSize, buf.Len(),
		)
	}

	digest := g.state.Digest()
	// Shuffle since we may not be able to send all digest entries.
	rand.Shuffle(len(digest), func(i, j int) {
		digest[i], digest[j] = digest[j], digest[i]
	})

	// Keep appending digest entries until we exceed the max packet size.
	// bufLen contains the number of bytes to send (which may be less than
	// buf.Len() if we exceed the packet limit).
	bufLen := buf.Len()
	for _, entry := range digest {
		if err := encoder.Encode(&entry); err != nil {
			return fmt.Errorf("encode: %w", err)
		}

		if buf.Len() > g.config.MaxPacketSize {
			break
		}
		bufLen = buf.Len()

		g.metrics.DigestEntriesOutbound.Inc()
	}

	udpAddr, err := net.ResolveUDPAddr("udp", node.Addr)
	if err != nil {
		return fmt.Errorf("resolve udp: %s: %w", node.Addr, err)
	}
	if _, err = g.packetConn.WriteTo(buf.Bytes()[:bufLen], udpAddr); err != nil {
		return fmt.Errorf("write packet: %s: %w", node.Addr, err)
	}

	g.metrics.PacketBytesOutbound.Add(float64(bufLen))

	return nil
}

// join attempts to synchronise with the node at the given address.
func (g *Gossip) join(addr string) (string, error) {
	conn, err := g.dialer.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(streamTimeout))

	g.metrics.ConnectionsOutbound.Inc()

	trackedReader := newTrackedReader(conn)
	defer func() {
		g.metrics.StreamBytesInbound.Add(float64(trackedReader.NumBytesRead()))
	}()

	trackedWriter := newTrackedWriter(conn)
	defer func() {
		g.metrics.StreamBytesOutbound.Add(float64(trackedWriter.NumBytesWritten()))
	}()

	r := bufio.NewReader(trackedReader)
	w := bufio.NewWriter(trackedWriter)

	if err := w.WriteByte(byte(messageTypeJoin)); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	if err := w.WriteByte(supportedVersion); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	encoder := newEncoder(w)

	localMeta := g.state.LocalNodeMetadata()
	if err := encoder.Encode(&joinHeader{
		NodeID: localMeta.ID,
		Addr:   localMeta.Addr,
	}); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}

	delta := g.state.LocalDelta()
	if err := encoder.Encode(g.state.LocalDelta()); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}
	g.metrics.DeltaEntriesOutbound.Add(float64(delta.EntriesTotal()))

	digest := g.state.Digest()
	if err := encoder.Encode(digest); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}
	g.metrics.DigestEntriesOutbound.Add(float64(len(digest)))

	if err := w.Flush(); err != nil {
		return "", fmt.Errorf("flush: %w", err)
	}

	decoder := newDecoder(r)

	var header joinHeader
	if err := decoder.Decode(&header); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	if err := decoder.Decode(&delta); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	g.metrics.DeltaEntriesInbound.Add(float64(delta.EntriesTotal()))

	g.state.ApplyDelta(delta)

	return header.NodeID, nil
}

// leave attempts to send our local state to the node at the given address.
func (g *Gossip) leave(addr string) error {
	conn, err := g.dialer.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(streamTimeout))

	g.metrics.ConnectionsOutbound.Inc()

	trackedReader := newTrackedReader(conn)
	defer func() {
		g.metrics.StreamBytesInbound.Add(float64(trackedReader.NumBytesRead()))
	}()

	trackedWriter := newTrackedWriter(conn)
	defer func() {
		g.metrics.StreamBytesOutbound.Add(float64(trackedWriter.NumBytesWritten()))
	}()

	r := bufio.NewReader(trackedReader)
	w := bufio.NewWriter(trackedWriter)

	if err := w.WriteByte(byte(messageTypeLeave)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.WriteByte(supportedVersion); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	encoder := newEncoder(w)

	localMeta := g.state.LocalNodeMetadata()
	if err := encoder.Encode(&joinHeader{
		NodeID: localMeta.ID,
		Addr:   localMeta.Addr,
	}); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	delta := g.state.LocalDelta()
	if err := encoder.Encode(delta); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	g.metrics.DeltaEntriesOutbound.Add(float64(delta.EntriesTotal()))

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	decoder := newDecoder(r)

	// Wait for a header as an acknowledgement.
	var header leaveHeader
	if err := decoder.Decode(&header); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	return nil
}

// ensurePort adds the configured bind port to addr if addr doesn't already
// have a port.
func (g *Gossip) ensurePort(addr string) string {
	if strings.Contains(addr, ":") {
		return addr
	}

	_, bindPort, err := net.SplitHostPort(g.config.BindAddr)
	if err != nil {
		// We've already bound to bind addr so expect it to be valid.
		panic("invalid bind addr:" + g.config.BindAddr)
	}

	return addr + ":" + bindPort
}

// resolveAddr resolves the given address, which may be a domain pointing
// to multiple IP addresses. If no port is given the bind port is used.
func resolveAddr(addr string) ([]string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid addr: %s: %w", addr, err)
	}

	// If the address already contains an IP address, do nothing.
	if ip := net.ParseIP(host); ip != nil {
		return []string{addr}, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("lookup host: %s: %w", host, err)
	}

	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String()+":"+port)
	}
	return addrs, nil
}
