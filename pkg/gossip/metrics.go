package gossip

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// ConnectionsInbound is the total number of incoming stream
	// connections.
	ConnectionsInbound prometheus.Counter

	// StreamBytesInbound is the total number of read bytes via a stream
	// connection.
	StreamBytesInbound prometheus.Counter

	// PacketBytesInbound is the total number of read bytes via a packet
	// connection.
	PacketBytesInbound prometheus.Counter

	// ConnectionsOutbound is the total number of outgoing stream
	// connections.
	ConnectionsOutbound prometheus.Counter

	// StreamBytesOutbound is the total number of written bytes via a stream
	// connection.
	StreamBytesOutbound prometheus.Counter

	// PacketBytesOutbound is the total number of written bytes via a packet
	// connection.
	PacketBytesOutbound prometheus.Counter

	// Entries is the number of entries labelled by node_id, deleted and
	// internal.
	Entries *prometheus.GaugeVec
}

func newMetrics() *Metrics {
	return &Metrics{
		ConnectionsInbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "connections_inbound_total",
				Help:      "Total number of incoming stream connections",
			},
		),
		StreamBytesInbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "stream_bytes_inbound_total",
				Help:      "Total number of read bytes via a stream connection",
			},
		),
		PacketBytesInbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "packet_bytes_inbound_total",
				Help:      "Total number of read bytes via a packet connection",
			},
		),
		ConnectionsOutbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "connections_outbound_total",
				Help:      "Total number of outbound stream connections",
			},
		),
		StreamBytesOutbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "stream_bytes_outbound_total",
				Help:      "Total number of written bytes via a stream connection",
			},
		),
		PacketBytesOutbound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "packet_bytes_outbound_total",
				Help:      "Total number of written bytes via a packet connection",
			},
		),
		Entries: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: "gossip",
				Name:      "entries",
				Help:      "Number of entries",
			},
			[]string{"node_id", "deleted", "internal"},
		),
	}
}

func (m *Metrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		m.ConnectionsInbound,
		m.StreamBytesInbound,
		m.PacketBytesInbound,
		m.ConnectionsOutbound,
		m.StreamBytesOutbound,
		m.PacketBytesOutbound,
		m.Entries,
	)
}
