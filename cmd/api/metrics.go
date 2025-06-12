package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metric definitions for the DCE API

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "api",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "handler", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dce",
			Subsystem: "api",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~32s
		},
		[]string{"method", "handler"},
	)

	// Call domain metrics
	callRoutingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "routing_latency_seconds",
			Help:      "Call routing decision latency",
			Buckets:   prometheus.ExponentialBuckets(0.000001, 2, 20), // 1μs to 1s
		},
		[]string{"algorithm", "result"},
	)

	activeCalls = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "active_total",
			Help:      "Number of active calls",
		},
	)

	callsInitiated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "initiated_total",
			Help:      "Total number of calls initiated",
		},
		[]string{"source"},
	)

	callsCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "completed_total",
			Help:      "Total number of calls completed",
		},
		[]string{"status"},
	)

	callsAbandoned = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "abandoned_total",
			Help:      "Total number of calls abandoned",
		},
	)

	callsNoBids = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "call",
			Name:      "no_bids_total",
			Help:      "Total number of calls that received no bids",
		},
	)

	// Bid domain metrics
	bidProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dce",
			Subsystem: "bid",
			Name:      "processing_duration_seconds",
			Help:      "Duration of bid processing",
			Buckets:   prometheus.ExponentialBuckets(0.00001, 2, 15), // 10μs to 160ms
		},
		[]string{"auction_type", "status"},
	)

	bidProcessingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "bid",
			Name:      "processing_total",
			Help:      "Total number of bids processed",
		},
		[]string{"auction_type", "status"},
	)

	bidsPerSecond = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "dce",
			Subsystem: "bid",
			Name:      "throughput_per_second",
			Help:      "Current bid processing throughput",
		},
		[]string{"auction_type"},
	)

	// Compliance domain metrics
	complianceCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dce",
			Subsystem: "compliance",
			Name:      "check_duration_seconds",
			Help:      "Compliance check duration",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10), // 100μs to 100ms
		},
		[]string{"check_type", "result"},
	)

	dncListSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "dce",
			Subsystem: "compliance",
			Name:      "dnc_list_size",
			Help:      "Current DNC list size",
		},
	)

	complianceViolations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dce",
			Subsystem: "compliance",
			Name:      "violations_total",
			Help:      "Total number of compliance violations",
		},
		[]string{"type", "severity"},
	)

	// Database metrics
	dbConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "pgxpool",
			Name:      "connections",
			Help:      "Current number of connections in the pool",
		},
		[]string{"state"},
	)

	dbConnectionPoolMax = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "pgxpool",
			Name:      "max_conns",
			Help:      "Maximum number of connections in the pool",
		},
	)

	dbConnectionAcquireCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "pgxpool",
			Name:      "acquire_count",
			Help:      "Total number of connection acquisitions",
		},
	)

	dbConnectionReleaseCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "pgxpool",
			Name:      "release_count",
			Help:      "Total number of connection releases",
		},
	)
)

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// InstrumentHTTPHandler wraps an HTTP handler with metrics collection
func InstrumentHTTPHandler(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Execute the handler
		handler(wrapped, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := statusCodeClass(wrapped.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, handlerName, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, handlerName).Observe(duration)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// statusCodeClass returns the status code class (2xx, 3xx, 4xx, 5xx)
func statusCodeClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

// RecordCallRoutingLatency records the latency of a call routing decision
func RecordCallRoutingLatency(algorithm string, result string, duration time.Duration) {
	callRoutingLatency.WithLabelValues(algorithm, result).Observe(duration.Seconds())
}

// RecordBidProcessing records bid processing metrics
func RecordBidProcessing(auctionType string, status string, duration time.Duration) {
	bidProcessingDuration.WithLabelValues(auctionType, status).Observe(duration.Seconds())
	bidProcessingTotal.WithLabelValues(auctionType, status).Inc()
}

// RecordComplianceCheck records compliance check metrics
func RecordComplianceCheck(checkType string, result string, duration time.Duration) {
	complianceCheckDuration.WithLabelValues(checkType, result).Observe(duration.Seconds())
}

// UpdateActiveCalls updates the active calls gauge
func UpdateActiveCalls(count float64) {
	activeCalls.Set(count)
}

// UpdateDNCListSize updates the DNC list size gauge
func UpdateDNCListSize(size float64) {
	dncListSize.Set(size)
}

// UpdateBidThroughput updates the bid throughput gauge
func UpdateBidThroughput(auctionType string, throughput float64) {
	bidsPerSecond.WithLabelValues(auctionType).Set(throughput)
}

// RecordCallInitiated records a call initiation
func RecordCallInitiated(source string) {
	callsInitiated.WithLabelValues(source).Inc()
}

// RecordCallCompleted records a call completion
func RecordCallCompleted(status string) {
	callsCompleted.WithLabelValues(status).Inc()
}

// RecordCallAbandoned records an abandoned call
func RecordCallAbandoned() {
	callsAbandoned.Inc()
}

// RecordCallNoBids records a call that received no bids
func RecordCallNoBids() {
	callsNoBids.Inc()
}

// RecordComplianceViolation records a compliance violation
func RecordComplianceViolation(violationType, severity string) {
	complianceViolations.WithLabelValues(violationType, severity).Inc()
}

// UpdateDBConnectionPoolMetrics updates database connection pool metrics
func UpdateDBConnectionPoolMetrics(active, idle, total, max int) {
	dbConnectionPoolSize.WithLabelValues("active").Set(float64(active))
	dbConnectionPoolSize.WithLabelValues("idle").Set(float64(idle))
	dbConnectionPoolSize.WithLabelValues("total").Set(float64(total))
	dbConnectionPoolMax.Set(float64(max))
}
