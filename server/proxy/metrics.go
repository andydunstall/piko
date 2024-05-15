package proxy

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// ConnectedUpstreams is the number of upstreams connected to this node.
	ConnectedUpstreams prometheus.Gauge

	// RegisteredEndpoints is the number of endpoints registered to this node.
	RegisteredEndpoints prometheus.Gauge

	// ForwardedLocalTotal is the number of requests forwarded to an upstream
	// connected to the local node.
	ForwardedLocalTotal prometheus.Counter

	// ForwardedRemoteTotal is the number of requests forwarded to a remote
	// node. Labelled by target node ID.
	ForwardedRemoteTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		ConnectedUpstreams: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: "proxy",
				Name:      "connected_upstreams",
				Help:      "Number of upstreams connected to this node",
			},
		),
		RegisteredEndpoints: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: "proxy",
				Name:      "registered_endpoints",
				Help:      "Number of endpoints registered to this node",
			},
		),
		ForwardedLocalTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "proxy",
				Name:      "forwarded_local_total",
				Help:      "Number of requests forwarded to an upstream connected to the local node",
			},
		),
		ForwardedRemoteTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: "proxy",
				Name:      "forwarded_remote_total",
				Help:      "Number of requests forwarded to a remote node",
			},
			[]string{"node_id"},
		),
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.ConnectedUpstreams,
		m.RegisteredEndpoints,
		m.ForwardedLocalTotal,
		m.ForwardedRemoteTotal,
	)
}
