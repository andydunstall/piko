// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// NewMetrics creates metrics middleware.
//
// Metrics are split into 'pico' and 'proxy' to distinguish between internal
// management requests and proxied requests.
func NewMetrics(registry *prometheus.Registry) gin.HandlerFunc {
	var picoRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pico_http_requests_total",
			Help: "Pico HTTP requests.",
		},
		[]string{"status"},
	)
	var picoRequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pico_http_request_latency_seconds",
			Help:    "Pico HTTP request latency.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"status"},
	)

	var proxyRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_http_requests_total",
			Help: "Proxy HTTP requests.",
		},
		[]string{"status"},
	)
	var proxyRequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxy_http_request_latency_seconds",
			Help:    "Proxy HTTP request latency.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"status"},
	)

	registry.MustRegister(picoRequests)
	registry.MustRegister(picoRequestLatency)
	registry.MustRegister(proxyRequests)
	registry.MustRegister(proxyRequestLatency)

	return func(c *gin.Context) {
		start := time.Now()

		// Process request.
		c.Next()

		if strings.HasPrefix(c.Request.URL.Path, "/pico") {
			picoRequests.With(prometheus.Labels{
				"status": strconv.Itoa(c.Writer.Status()),
			}).Inc()
			picoRequestLatency.With(prometheus.Labels{
				"status": strconv.Itoa(c.Writer.Status()),
			}).Observe(float64(time.Since(start).Milliseconds()) / 1000)
		} else {
			proxyRequests.With(prometheus.Labels{
				"status": strconv.Itoa(c.Writer.Status()),
			}).Inc()
			proxyRequestLatency.With(prometheus.Labels{
				"status": strconv.Itoa(c.Writer.Status()),
			}).Observe(float64(time.Since(start).Milliseconds()) / 1000)
		}
	}
}
