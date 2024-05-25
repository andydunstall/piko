package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	RequestsInFlight prometheus.Gauge
	RequestsTotal    *prometheus.CounterVec
	RequestLatency   *prometheus.HistogramVec
	RequestSize      prometheus.Histogram
	ResponseSize     prometheus.Histogram
}

// NewMetrics creates metrics middleware.
func NewMetrics(subsystem string) *Metrics {
	sizeBuckets := prometheus.ExponentialBuckets(256, 4, 8)
	return &Metrics{
		RequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "piko",
				Subsystem: subsystem,
				Name:      "requests_in_flight",
				Help:      "Number of requests currently handled by this server.",
			},
		),
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "piko",
				Subsystem: subsystem,
				Name:      "requests_total",
				Help:      "Total requests.",
			},
			[]string{"status", "method"},
		),
		RequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "piko",
				Subsystem: subsystem,
				Name:      "request_latency_seconds",
				Help:      "Request latency.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status", "method"},
		),
		RequestSize: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "piko",
				Subsystem: subsystem,
				Name:      "request_size_bytes",
				Help:      "Request size",
				Buckets:   sizeBuckets,
			},
		),
		ResponseSize: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "piko",
				Subsystem: subsystem,
				Name:      "response_size_bytes",
				Help:      "Response size",
				Buckets:   sizeBuckets,
			},
		),
	}
}

func (m *Metrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.RequestsInFlight.Inc()
		defer m.RequestsInFlight.Dec()

		start := time.Now()

		// Process request.
		c.Next()

		m.RequestsTotal.With(prometheus.Labels{
			"status": strconv.Itoa(c.Writer.Status()),
			"method": c.Request.Method,
		}).Inc()
		m.RequestLatency.With(prometheus.Labels{
			"status": strconv.Itoa(c.Writer.Status()),
			"method": c.Request.Method,
		}).Observe(float64(time.Since(start).Milliseconds()) / 1000)
		m.RequestSize.Observe(float64(computeApproximateRequestSize(c.Request)))
		m.ResponseSize.Observe(float64(c.Writer.Size()))
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) {
	registry.MustRegister(
		m.RequestsInFlight,
		m.RequestsTotal,
		m.RequestLatency,
		m.RequestSize,
		m.ResponseSize,
	)
}

func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s += len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}
