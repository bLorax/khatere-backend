package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "khatere_http_requests_total",
			Help: "Total HTTP requests, labeled by route, method, and status code.",
		},
		[]string{"route", "method", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "khatere_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by route and method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)

	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "khatere_cache_hits_total",
			Help: "Cache hits, labeled by cache name (membership, gallery).",
		},
		[]string{"cache"},
	)

	CacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "khatere_cache_misses_total",
			Help: "Cache misses, labeled by cache name (membership, gallery).",
		},
		[]string{"cache"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequestsTotal, HTTPRequestDuration, CacheHitsTotal, CacheMissesTotal)
}

// RegisterThumbnailQueueDepth wires a gauge that Prometheus reads by
// calling depthFunc at scrape time — no background polling goroutine
// needed.
func RegisterThumbnailQueueDepth(depthFunc func() int) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "khatere_thumbnail_queue_depth",
			Help: "Current number of jobs waiting in the thumbnail worker pool's queue.",
		},
		func() float64 { return float64(depthFunc()) },
	))
}

// MetricsHandler exposes /metrics for Prometheus to scrape.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// InstrumentRoute wraps a handler so every request to it records a count
// and a latency observation, labeled by the given route name. route
// should be a fixed string like "GET /events/{id}" — never a raw
// r.URL.Path — so path parameters don't explode the label cardinality.
func InstrumentRoute(route, method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next(sw, r)

		HTTPRequestDuration.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
		HTTPRequestsTotal.WithLabelValues(route, method, strconv.Itoa(sw.status)).Inc()
	}
}

// statusWriter captures the status code a handler actually wrote, since
// http.ResponseWriter doesn't expose it directly.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
