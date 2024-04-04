package proxy

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	// ProxyRequestsTotal is the number proxied HTTP request. Labelled by
	// status code.
	//
	// Only contains requests that are successfully proxied.
	ProxyRequestsTotal *prometheus.CounterVec

	// ProxyRequestLatency is a histogram of the proxied HTTP requests.
	ProxyRequestLatency *prometheus.HistogramVec

	// ProxyErrorsTotal is the number of errors sending requests to upstream
	// listeners.
	ProxyErrorsTotal prometheus.Counter
}

func newMetrics() *metrics {
	return &metrics{
		ProxyRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: "proxy",
				Name:      "requests_total",
				Help:      "Proxied requests.",
			},
			[]string{"status"},
		),
		ProxyRequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: "proxy",
				Name:      "request_latency_seconds",
				Help:      "Proxy request latency.",
				Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
			},
			[]string{"status"},
		),
		ProxyErrorsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Subsystem: "proxy",
				Name:      "errors_total",
				Help:      "Proxy errors.",
			},
		),
	}
}

func (m *metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.ProxyRequestsTotal,
		m.ProxyRequestLatency,
		m.ProxyErrorsTotal,
	)
}
