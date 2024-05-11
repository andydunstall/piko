package cluster

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// Nodes contains the number of known nodes in the cluster, labelled by
	// status.
	Nodes *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		Nodes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pico",
				Subsystem: "cluster",
				Name:      "nodes",
				Help:      "Number of nodes in the cluster state",
			},
			[]string{"status"},
		),
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.Nodes,
	)
}
