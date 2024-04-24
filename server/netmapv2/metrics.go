package netmap

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// Entries contains the number of entries in the netmap, labelled by
	// status.
	Entries *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		Entries: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Subsystem: "netmap",
				Name:      "entries",
				Help:      "Number of entries in the netmap",
			},
			[]string{"status"},
		),
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.Entries,
	)
}
