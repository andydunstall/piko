package proxy

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	// RequestsTotal is the number proxied HTTP request. Labelled by
	// status code.
	//
	// Only contains requests that are successfully proxied.
	RequestsTotal *prometheus.CounterVec

	// RequestLatency is a histogram of the proxied HTTP requests.
	RequestLatency *prometheus.HistogramVec

	// ErrorsTotal is the number of errors sending requests to upstream
	// listeners.
	ErrorsTotal prometheus.Counter

	// Listeners is the number of registered upstream listeners.
	Listeners prometheus.Gauge
}

func newMetrics() *metrics {
	return &metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: "proxy",
				Name:      "requests_total",
				Help:      "Proxied requests.",
			},
			[]string{"status"},
		),
		RequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: "proxy",
				Name:      "request_latency_seconds",
				Help:      "Proxy request latency.",
				Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
			},
			[]string{"status"},
		),
		ErrorsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Subsystem: "proxy",
				Name:      "errors_total",
				Help:      "Proxy errors.",
			},
		),
		Listeners: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Subsystem: "proxy",
				Name:      "listeners",
				Help:      "Number of upstream listeners.",
			},
		),
	}
}

func (m *metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.RequestsTotal,
		m.RequestLatency,
		m.ErrorsTotal,
		m.Listeners,
	)
}
