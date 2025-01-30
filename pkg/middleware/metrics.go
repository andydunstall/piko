package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type gaugeOptions struct {
	RequestsInFlight prometheus.GaugeOpts
	RequestsTotal    prometheus.CounterOpts
	RequestLatency   prometheus.HistogramOpts
	RequestSize      prometheus.HistogramOpts
	ResponseSize     prometheus.HistogramOpts
}

func newOptions(subsystem string) gaugeOptions {
	sizeBuckets := prometheus.ExponentialBuckets(256, 4, 8)
	return gaugeOptions{
		RequestsInFlight: prometheus.GaugeOpts{
			Namespace: "piko",
			Subsystem: subsystem,
			Name:      "requests_in_flight",
			Help:      "Number of requests currently handled by this server.",
		},
		RequestsTotal: prometheus.CounterOpts{
			Namespace: "piko",
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "Total requests.",
		},
		RequestLatency: prometheus.HistogramOpts{
			Namespace: "piko",
			Subsystem: subsystem,
			Name:      "request_latency_seconds",
			Help:      "Request latency.",
			Buckets:   prometheus.DefBuckets,
		},
		RequestSize: prometheus.HistogramOpts{
			Namespace: "piko",
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "Request size",
			Buckets:   sizeBuckets,
		},
		ResponseSize: prometheus.HistogramOpts{
			Namespace: "piko",
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "Response size",
			Buckets:   sizeBuckets,
		},
	}
}

type LabeledMetrics struct {
	RequestsInFlight *prometheus.GaugeVec
	RequestsTotal    *prometheus.CounterVec
	RequestLatency   *prometheus.HistogramVec
	RequestSize      *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec
}

type Metrics struct {
	RequestsInFlight prometheus.Gauge
	RequestsTotal    *prometheus.CounterVec
	RequestLatency   *prometheus.HistogramVec
	RequestSize      prometheus.Histogram
	ResponseSize     prometheus.Histogram
}

func NewLabeledMetrics(subsystem string) *LabeledMetrics {
	opts := newOptions(subsystem)
	return &LabeledMetrics{
		RequestsInFlight: prometheus.NewGaugeVec(
			opts.RequestsInFlight,
			[]string{"endpoint"},
		),
		RequestsTotal: prometheus.NewCounterVec(
			opts.RequestsTotal,
			[]string{"endpoint", "status", "method"},
		),
		RequestLatency: prometheus.NewHistogramVec(
			opts.RequestLatency,
			[]string{"endpoint", "status", "method"},
		),
		RequestSize:  prometheus.NewHistogramVec(opts.RequestSize, []string{"endpoint"}),
		ResponseSize: prometheus.NewHistogramVec(opts.ResponseSize, []string{"endpoint"}),
	}
}

func (lm *LabeledMetrics) Register(registry prometheus.Registerer) {
	registry.MustRegister(
		lm.RequestsInFlight,
		lm.RequestsTotal,
		lm.RequestLatency,
		lm.RequestSize,
		lm.ResponseSize,
	)
}

func NewMetrics(subsystem string) *Metrics {
	opts := newOptions(subsystem)
	return &Metrics{
		RequestsInFlight: prometheus.NewGauge(opts.RequestsInFlight),
		RequestsTotal: prometheus.NewCounterVec(opts.RequestsTotal,
			[]string{"status", "method"},
		),
		RequestLatency: prometheus.NewHistogramVec(
			opts.RequestLatency,
			[]string{"status", "method"},
		),
		RequestSize:  prometheus.NewHistogram(opts.RequestSize),
		ResponseSize: prometheus.NewHistogram(opts.ResponseSize),
	}
}

func (m *Metrics) Register(registry prometheus.Registerer) {
	registry.MustRegister(
		m.RequestsInFlight,
		m.RequestsTotal,
		m.RequestLatency,
		m.RequestSize,
		m.ResponseSize,
	)
}

type observer struct {
	RequestsInFlight prometheus.Gauge
	RequestsTotal    *prometheus.CounterVec
	RequestLatency   prometheus.ObserverVec
	RequestSize      prometheus.Observer
	ResponseSize     prometheus.Observer
}

func (lm *LabeledMetrics) Handler(endpointID string) gin.HandlerFunc {
	obs := observer{
		RequestsInFlight: lm.RequestsInFlight.WithLabelValues(endpointID),
		RequestsTotal:    lm.RequestsTotal.MustCurryWith(prometheus.Labels{"endpoint": endpointID}),
		RequestLatency:   lm.RequestLatency.MustCurryWith(prometheus.Labels{"endpoint": endpointID}),
		RequestSize:      lm.RequestSize.WithLabelValues(endpointID),
		ResponseSize:     lm.ResponseSize.WithLabelValues(endpointID),
	}
	return obs.Handler()
}

func (m *Metrics) Handler() gin.HandlerFunc {
	obs := observer{
		RequestsInFlight: m.RequestsInFlight,
		RequestsTotal:    m.RequestsTotal,
		RequestLatency:   m.RequestLatency,
		RequestSize:      m.RequestSize,
		ResponseSize:     m.ResponseSize,
	}
	return obs.Handler()
}

func (o observer) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		o.RequestsInFlight.Inc()
		defer o.RequestsInFlight.Dec()

		start := time.Now()

		// Process request.
		c.Next()

		o.RequestsTotal.With(prometheus.Labels{
			"status": strconv.Itoa(c.Writer.Status()),
			"method": c.Request.Method,
		}).Inc()
		o.RequestLatency.With(prometheus.Labels{
			"status": strconv.Itoa(c.Writer.Status()),
			"method": c.Request.Method,
		}).Observe(float64(time.Since(start).Milliseconds()) / 1000)

		o.RequestSize.Observe(float64(computeApproximateRequestSize(c.Request)))
		o.ResponseSize.Observe(float64(c.Writer.Size()))
	}
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
