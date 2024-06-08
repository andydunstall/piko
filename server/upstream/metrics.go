package upstream

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// ConnectedUpstreams is the number of upstreams connected to this node.
	ConnectedUpstreams prometheus.Gauge

	// RegisteredEndpoints is the number of endpoints registered to this node.
	RegisteredEndpoints prometheus.Gauge

	// UpstreamRequestsTotal is the number of requests sent to an
	// upstream connected to the local node.
	UpstreamRequestsTotal prometheus.Counter

	// RemoteRequestsTotal is the number of requests sent to another node.
	// Labelled by target node ID.
	RemoteRequestsTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		ConnectedUpstreams: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: "upstreams",
				Name:      "connected_upstreams",
				Help:      "Number of upstreams connected to this node",
			},
		),
		RegisteredEndpoints: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: "upstreams",
				Name:      "registered_endpoints",
				Help:      "Number of endpoints registered to this node",
			},
		),
		UpstreamRequestsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "upstreams",
				Name:      "upstream_requests_total",
				Help:      "Number of requests sent to an upstream connected to the local node",
			},
		),
		RemoteRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "upstreams",
				Name:      "remote_requests_total",
				Help:      "Number of requests sent to a remote node",
			},
			[]string{"node_id"},
		),
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.ConnectedUpstreams,
		m.RegisteredEndpoints,
		m.UpstreamRequestsTotal,
		m.RemoteRequestsTotal,
	)
}
