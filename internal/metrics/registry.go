package metrics

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Registry holds all domain-specific metrics for the application
type Registry struct {
	meter metric.Meter

	// Bid Domain Metrics
	BidProcessingDuration metric.Float64Histogram
	BidsPerSecond         metric.Float64ObservableGauge
	BidSuccessCounter     metric.Int64Counter
	BidFailureCounter     metric.Int64Counter
	AuctionQueueDepth     metric.Int64ObservableGauge

	// Call Domain Metrics
	CallRoutingLatency metric.Float64Histogram
	ActiveCalls        metric.Int64ObservableGauge
	CallsPerSecond     metric.Float64ObservableGauge
	CallSuccessCounter metric.Int64Counter
	CallFailureCounter metric.Int64Counter
	CallDuration       metric.Float64Histogram

	// Compliance Domain Metrics
	ComplianceCheckDuration metric.Float64Histogram
	DNCListSize             metric.Int64ObservableGauge
	TCPAViolationCounter    metric.Int64Counter
	ConsentCheckCounter     metric.Int64Counter
	CompliancePassRate      metric.Float64ObservableGauge

	// Financial Domain Metrics
	TransactionAmount     metric.Float64Histogram
	PaymentProcessingTime metric.Float64Histogram
	AccountBalanceGauge   metric.Float64ObservableGauge
	TransactionCounter    metric.Int64Counter
	PaymentFailureCounter metric.Int64Counter

	// System Metrics
	DatabaseConnectionPool metric.Int64ObservableGauge
	CacheHitRate           metric.Float64ObservableGauge
	MessageQueueDepth      metric.Int64ObservableGauge
	APIRequestDuration     metric.Float64Histogram
	APIRequestCounter      metric.Int64Counter

	// State for observable metrics
	mu                sync.RWMutex
	activeCalls       int64
	auctionQueueDepth int64
	dncListSize       int64
	dbPoolSize        int64
	messageQueueDepth int64
	callsProcessed    int64
	bidsProcessed     int64
	lastCallCount     int64
	lastBidCount      int64
	lastCallTime      time.Time
	lastBidTime       time.Time
}

// NewRegistry creates a new metrics registry with all domain metrics
func NewRegistry(meterName string) (*Registry, error) {
	meter := otel.Meter(meterName)
	r := &Registry{
		meter:        meter,
		lastCallTime: time.Now(),
		lastBidTime:  time.Now(),
	}

	if err := r.initBidMetrics(); err != nil {
		return nil, err
	}

	if err := r.initCallMetrics(); err != nil {
		return nil, err
	}

	if err := r.initComplianceMetrics(); err != nil {
		return nil, err
	}

	if err := r.initFinancialMetrics(); err != nil {
		return nil, err
	}

	if err := r.initSystemMetrics(); err != nil {
		return nil, err
	}

	return r, nil
}

// initBidMetrics initializes bid domain metrics
func (r *Registry) initBidMetrics() error {
	var err error

	// Bid processing duration histogram
	r.BidProcessingDuration, err = r.meter.Float64Histogram(
		"dce.bid.processing_duration",
		metric.WithDescription("Duration of bid processing in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1, 5, 10, 50, 100, 500),
	)
	if err != nil {
		return err
	}

	// Bids per second gauge
	r.BidsPerSecond, err = r.meter.Float64ObservableGauge(
		"dce.bid.throughput_per_second",
		metric.WithDescription("Current bid processing throughput per second"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()

			now := time.Now()
			elapsed := now.Sub(r.lastBidTime).Seconds()
			if elapsed > 0 {
				rate := float64(r.bidsProcessed-r.lastBidCount) / elapsed
				o.Observe(rate)
				r.lastBidCount = r.bidsProcessed
				r.lastBidTime = now
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Bid counters
	r.BidSuccessCounter, err = r.meter.Int64Counter(
		"dce.bid.success_total",
		metric.WithDescription("Total number of successful bids"),
	)
	if err != nil {
		return err
	}

	r.BidFailureCounter, err = r.meter.Int64Counter(
		"dce.bid.failure_total",
		metric.WithDescription("Total number of failed bids"),
	)
	if err != nil {
		return err
	}

	// Auction queue depth
	r.AuctionQueueDepth, err = r.meter.Int64ObservableGauge(
		"dce.bid.auction_queue_depth",
		metric.WithDescription("Current depth of the auction processing queue"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(r.auctionQueueDepth)
			return nil
		}),
	)

	return err
}

// initCallMetrics initializes call domain metrics
func (r *Registry) initCallMetrics() error {
	var err error

	// Call routing latency histogram
	r.CallRoutingLatency, err = r.meter.Float64Histogram(
		"dce.call.routing_latency",
		metric.WithDescription("Call routing decision latency in microseconds"),
		metric.WithUnit("us"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 50, 100, 500, 1000, 5000, 10000),
	)
	if err != nil {
		return err
	}

	// Active calls gauge
	r.ActiveCalls, err = r.meter.Int64ObservableGauge(
		"dce.call.active_total",
		metric.WithDescription("Number of currently active calls"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(r.activeCalls)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Calls per second gauge
	r.CallsPerSecond, err = r.meter.Float64ObservableGauge(
		"dce.call.throughput_per_second",
		metric.WithDescription("Current call processing throughput per second"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()

			now := time.Now()
			elapsed := now.Sub(r.lastCallTime).Seconds()
			if elapsed > 0 {
				rate := float64(r.callsProcessed-r.lastCallCount) / elapsed
				o.Observe(rate)
				r.lastCallCount = r.callsProcessed
				r.lastCallTime = now
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Call counters
	r.CallSuccessCounter, err = r.meter.Int64Counter(
		"dce.call.success_total",
		metric.WithDescription("Total number of successful calls"),
	)
	if err != nil {
		return err
	}

	r.CallFailureCounter, err = r.meter.Int64Counter(
		"dce.call.failure_total",
		metric.WithDescription("Total number of failed calls"),
	)
	if err != nil {
		return err
	}

	// Call duration histogram
	r.CallDuration, err = r.meter.Float64Histogram(
		"dce.call.duration",
		metric.WithDescription("Call duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600),
	)

	return err
}

// initComplianceMetrics initializes compliance domain metrics
func (r *Registry) initComplianceMetrics() error {
	var err error

	// Compliance check duration
	r.ComplianceCheckDuration, err = r.meter.Float64Histogram(
		"dce.compliance.check_duration",
		metric.WithDescription("Compliance check duration in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 5, 10, 50, 100),
	)
	if err != nil {
		return err
	}

	// DNC list size
	r.DNCListSize, err = r.meter.Int64ObservableGauge(
		"dce.compliance.dnc_list_size",
		metric.WithDescription("Current size of the Do Not Call list"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(r.dncListSize)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Violation counters
	r.TCPAViolationCounter, err = r.meter.Int64Counter(
		"dce.compliance.tcpa_violation_total",
		metric.WithDescription("Total TCPA violations detected"),
	)
	if err != nil {
		return err
	}

	r.ConsentCheckCounter, err = r.meter.Int64Counter(
		"dce.compliance.consent_check_total",
		metric.WithDescription("Total consent checks performed"),
	)

	return err
}

// initFinancialMetrics initializes financial domain metrics
func (r *Registry) initFinancialMetrics() error {
	var err error

	// Transaction amount histogram
	r.TransactionAmount, err = r.meter.Float64Histogram(
		"dce.financial.transaction_amount",
		metric.WithDescription("Transaction amounts in USD"),
		metric.WithUnit("USD"),
		metric.WithExplicitBucketBoundaries(0.01, 0.1, 1, 10, 100, 1000, 10000),
	)
	if err != nil {
		return err
	}

	// Payment processing time
	r.PaymentProcessingTime, err = r.meter.Float64Histogram(
		"dce.financial.payment_processing_time",
		metric.WithDescription("Payment processing time in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30),
	)
	if err != nil {
		return err
	}

	// Transaction counter
	r.TransactionCounter, err = r.meter.Int64Counter(
		"dce.financial.transaction_total",
		metric.WithDescription("Total number of transactions"),
	)
	if err != nil {
		return err
	}

	// Payment failure counter
	r.PaymentFailureCounter, err = r.meter.Int64Counter(
		"dce.financial.payment_failure_total",
		metric.WithDescription("Total number of payment failures"),
	)

	return err
}

// initSystemMetrics initializes system-level metrics
func (r *Registry) initSystemMetrics() error {
	var err error

	// Database connection pool
	r.DatabaseConnectionPool, err = r.meter.Int64ObservableGauge(
		"dce.system.db_connection_pool_size",
		metric.WithDescription("Current database connection pool size"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(r.dbPoolSize)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Message queue depth
	r.MessageQueueDepth, err = r.meter.Int64ObservableGauge(
		"dce.system.message_queue_depth",
		metric.WithDescription("Current message queue depth"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(r.messageQueueDepth)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// API request duration
	r.APIRequestDuration, err = r.meter.Float64Histogram(
		"dce.api.request_duration",
		metric.WithDescription("API request duration in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 50, 100, 500, 1000, 5000),
	)
	if err != nil {
		return err
	}

	// API request counter
	r.APIRequestCounter, err = r.meter.Int64Counter(
		"dce.api.request_total",
		metric.WithDescription("Total number of API requests"),
	)

	return err
}

// Helper methods for updating observable metric values

// UpdateActiveCalls updates the active calls count
func (r *Registry) UpdateActiveCalls(delta int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeCalls += delta
}

// SetAuctionQueueDepth sets the auction queue depth
func (r *Registry) SetAuctionQueueDepth(depth int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auctionQueueDepth = depth
}

// SetDNCListSize sets the DNC list size
func (r *Registry) SetDNCListSize(size int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dncListSize = size
}

// SetDBPoolSize sets the database connection pool size
func (r *Registry) SetDBPoolSize(size int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dbPoolSize = size
}

// SetMessageQueueDepth sets the message queue depth
func (r *Registry) SetMessageQueueDepth(depth int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageQueueDepth = depth
}

// IncrementCallsProcessed increments the calls processed counter
func (r *Registry) IncrementCallsProcessed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callsProcessed++
}

// IncrementBidsProcessed increments the bids processed counter
func (r *Registry) IncrementBidsProcessed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bidsProcessed++
}

// Helper methods for recording metrics with common attribute patterns

// RecordBidProcessing records bid processing metrics
func (r *Registry) RecordBidProcessing(ctx context.Context, duration float64, auctionType string, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("auction_type", auctionType),
		attribute.Bool("success", success),
	}

	r.BidProcessingDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	if success {
		r.BidSuccessCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		r.BidFailureCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	r.IncrementBidsProcessed()
}

// RecordCallRouting records call routing metrics
func (r *Registry) RecordCallRouting(ctx context.Context, latencyUS float64, algorithm string, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("algorithm", algorithm),
		attribute.Bool("success", success),
	}

	r.CallRoutingLatency.Record(ctx, latencyUS, metric.WithAttributes(attrs...))

	if success {
		r.CallSuccessCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		r.CallFailureCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	r.IncrementCallsProcessed()
}

// RecordComplianceCheck records compliance check metrics
func (r *Registry) RecordComplianceCheck(ctx context.Context, duration float64, checkType string, passed bool) {
	attrs := []attribute.KeyValue{
		attribute.String("check_type", checkType),
		attribute.Bool("passed", passed),
	}

	r.ComplianceCheckDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	if checkType == "consent" {
		r.ConsentCheckCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	if !passed && checkType == "tcpa" {
		r.TCPAViolationCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordTransaction records financial transaction metrics
func (r *Registry) RecordTransaction(ctx context.Context, amount float64, transactionType string, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("transaction_type", transactionType),
		attribute.Bool("success", success),
	}

	r.TransactionAmount.Record(ctx, amount, metric.WithAttributes(attrs...))
	r.TransactionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	if !success {
		r.PaymentFailureCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordAPIRequest records API request metrics
func (r *Registry) RecordAPIRequest(ctx context.Context, duration float64, method, path string, statusCode int) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	r.APIRequestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
	r.APIRequestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}
