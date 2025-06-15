package performance

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// LatencyMonitor provides real-time latency tracking and SLA monitoring
type LatencyMonitor struct {
	logger *zap.Logger
	
	// Configuration
	config *MonitorConfig
	
	// Latency tracking
	latencyBuffer *LatencyBuffer
	percentiles   *PercentileTracker
	
	// SLA monitoring
	slaViolations *SLAViolationTracker
	alertManager  *AlertManager
	
	// Performance degradation detection
	degradationDetector *DegradationDetector
	
	// Metrics
	metrics *MonitorMetrics
	
	// State management
	running int32
	stopped chan struct{}
	wg      sync.WaitGroup
	
	// Real-time statistics
	currentStats *LatencyStats
	statsLock    sync.RWMutex
}

// MonitorConfig contains monitoring configuration
type MonitorConfig struct {
	// Buffer settings
	BufferSize         int           `yaml:"buffer_size" default:"10000"`
	FlushInterval      time.Duration `yaml:"flush_interval" default:"1s"`
	RetentionPeriod    time.Duration `yaml:"retention_period" default:"5m"`
	
	// SLA thresholds
	SLALatencyP50      time.Duration `yaml:"sla_latency_p50" default:"5ms"`
	SLALatencyP95      time.Duration `yaml:"sla_latency_p95" default:"10ms"`
	SLALatencyP99      time.Duration `yaml:"sla_latency_p99" default:"20ms"`
	SLACacheHitP99     time.Duration `yaml:"sla_cache_hit_p99" default:"1ms"`
	SLAThroughputMin   int           `yaml:"sla_throughput_min" default:"10000"`
	
	// Alert thresholds
	AlertLatencyMultiplier float64       `yaml:"alert_latency_multiplier" default:"2.0"`
	AlertThroughputMin     int           `yaml:"alert_throughput_min" default:"5000"`
	AlertWindow            time.Duration `yaml:"alert_window" default:"30s"`
	AlertCooldown          time.Duration `yaml:"alert_cooldown" default:"5m"`
	
	// Degradation detection
	DegradationWindow     time.Duration `yaml:"degradation_window" default:"2m"`
	DegradationThreshold  float64       `yaml:"degradation_threshold" default:"1.5"`
	DegradationMinSamples int           `yaml:"degradation_min_samples" default:"100"`
	
	// Percentile calculation
	PercentileAccuracy    float64 `yaml:"percentile_accuracy" default:"0.01"`
	PercentileCompression float64 `yaml:"percentile_compression" default:"100.0"`
}

// LatencyStats provides real-time latency statistics
type LatencyStats struct {
	Count       int64         `json:"count"`
	P50         time.Duration `json:"p50"`
	P95         time.Duration `json:"p95"`
	P99         time.Duration `json:"p99"`
	P999        time.Duration `json:"p999"`
	Mean        time.Duration `json:"mean"`
	Min         time.Duration `json:"min"`
	Max         time.Duration `json:"max"`
	Throughput  float64       `json:"throughput"` // queries per second
	LastUpdated time.Time     `json:"last_updated"`
	
	// Cache-specific stats
	CacheHitP50  time.Duration `json:"cache_hit_p50"`
	CacheHitP95  time.Duration `json:"cache_hit_p95"`
	CacheHitP99  time.Duration `json:"cache_hit_p99"`
	CacheMissP50 time.Duration `json:"cache_miss_p50"`
	CacheMissP95 time.Duration `json:"cache_miss_p95"`
	CacheMissP99 time.Duration `json:"cache_miss_p99"`
	
	// SLA compliance
	SLACompliance *SLACompliance `json:"sla_compliance"`
}

// SLACompliance tracks SLA adherence
type SLACompliance struct {
	P50Compliance  float64 `json:"p50_compliance"`  // percentage
	P95Compliance  float64 `json:"p95_compliance"`
	P99Compliance  float64 `json:"p99_compliance"`
	ThroughputCompliance float64 `json:"throughput_compliance"`
	OverallScore   float64 `json:"overall_score"`
}

// MonitorMetrics contains Prometheus metrics for monitoring
type MonitorMetrics struct {
	// Latency histograms
	QueryLatency      prometheus.Histogram
	CacheHitLatency   prometheus.Histogram
	CacheMissLatency  prometheus.Histogram
	
	// Percentile gauges
	LatencyP50        prometheus.Gauge
	LatencyP95        prometheus.Gauge
	LatencyP99        prometheus.Gauge
	LatencyP999       prometheus.Gauge
	
	// SLA metrics
	SLAViolationsTotal prometheus.Counter
	SLAComplianceP50   prometheus.Gauge
	SLAComplianceP95   prometheus.Gauge
	SLAComplianceP99   prometheus.Gauge
	
	// Throughput metrics
	ThroughputCurrent  prometheus.Gauge
	ThroughputTotal    prometheus.Counter
	
	// Degradation metrics
	DegradationDetected prometheus.Counter
	DegradationSeverity prometheus.Gauge
	
	// Alert metrics
	AlertsTriggered    prometheus.Counter
	AlertsSuppressed   prometheus.Counter
	AlertCooldownActive prometheus.Gauge
}

// NewLatencyMonitor creates a new latency monitor
func NewLatencyMonitor(config *MonitorConfig, logger *zap.Logger) *LatencyMonitor {
	if config == nil {
		config = &MonitorConfig{} // Use defaults
	}
	
	monitor := &LatencyMonitor{
		logger:  logger,
		config:  config,
		stopped: make(chan struct{}),
		metrics: createMonitorMetrics(),
		currentStats: &LatencyStats{
			LastUpdated: time.Now(),
		},
	}
	
	monitor.initializeComponents()
	
	return monitor
}

// Start begins latency monitoring
func (m *LatencyMonitor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return fmt.Errorf("monitor already running")
	}
	
	m.logger.Info("Starting latency monitor",
		zap.Duration("sla_p50", m.config.SLALatencyP50),
		zap.Duration("sla_p95", m.config.SLALatencyP95),
		zap.Duration("sla_p99", m.config.SLALatencyP99),
		zap.Int("sla_throughput", m.config.SLAThroughputMin),
	)
	
	// Start background processing
	m.wg.Add(1)
	go m.runStatsProcessor(ctx)
	
	m.wg.Add(1)
	go m.runSLAMonitor(ctx)
	
	m.wg.Add(1)
	go m.runDegradationDetector(ctx)
	
	m.wg.Add(1)
	go m.runMetricsExporter(ctx)
	
	m.logger.Info("Latency monitor started successfully")
	return nil
}

// Stop gracefully shuts down the monitor
func (m *LatencyMonitor) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return nil
	}
	
	m.logger.Info("Stopping latency monitor")
	
	close(m.stopped)
	
	// Wait for background routines
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		m.logger.Info("Latency monitor stopped gracefully")
	case <-ctx.Done():
		m.logger.Warn("Latency monitor stop timed out")
		return ctx.Err()
	}
	
	return nil
}

// RecordLatency records a latency measurement
func (m *LatencyMonitor) RecordLatency(operation OperationType, duration time.Duration, cacheHit bool) {
	if atomic.LoadInt32(&m.running) == 0 {
		return
	}
	
	// Record in buffer
	measurement := &LatencyMeasurement{
		Operation: operation,
		Duration:  duration,
		CacheHit:  cacheHit,
		Timestamp: time.Now(),
	}
	
	m.latencyBuffer.Add(measurement)
	
	// Update Prometheus metrics
	switch operation {
	case OperationDNCQuery:
		m.metrics.QueryLatency.Observe(duration.Seconds())
		if cacheHit {
			m.metrics.CacheHitLatency.Observe(duration.Seconds())
		} else {
			m.metrics.CacheMissLatency.Observe(duration.Seconds())
		}
	}
	
	m.metrics.ThroughputTotal.Inc()
	
	// Check for immediate SLA violations
	m.checkImmediateSLAViolation(operation, duration, cacheHit)
}

// GetCurrentStats returns current latency statistics
func (m *LatencyMonitor) GetCurrentStats() *LatencyStats {
	m.statsLock.RLock()
	defer m.statsLock.RUnlock()
	
	// Create a copy to avoid race conditions
	stats := *m.currentStats
	return &stats
}

// GetSLACompliance returns current SLA compliance status
func (m *LatencyMonitor) GetSLACompliance() *SLACompliance {
	stats := m.GetCurrentStats()
	if stats.SLACompliance == nil {
		return &SLACompliance{}
	}
	return stats.SLACompliance
}

// GetPerformanceTrend returns performance trend over specified window
func (m *LatencyMonitor) GetPerformanceTrend(window time.Duration) *PerformanceTrend {
	return m.degradationDetector.GetTrend(window)
}

// SetSLAThresholds updates SLA thresholds dynamically
func (m *LatencyMonitor) SetSLAThresholds(p50, p95, p99 time.Duration, throughput int) {
	m.config.SLALatencyP50 = p50
	m.config.SLALatencyP95 = p95
	m.config.SLALatencyP99 = p99
	m.config.SLAThroughputMin = throughput
	
	m.logger.Info("Updated SLA thresholds",
		zap.Duration("p50", p50),
		zap.Duration("p95", p95),
		zap.Duration("p99", p99),
		zap.Int("throughput", throughput),
	)
}

// initializeComponents sets up all monitoring components
func (m *LatencyMonitor) initializeComponents() {
	// Latency buffer
	m.latencyBuffer = NewLatencyBuffer(&LatencyBufferConfig{
		Size:            m.config.BufferSize,
		FlushInterval:   m.config.FlushInterval,
		RetentionPeriod: m.config.RetentionPeriod,
	}, m.logger)
	
	// Percentile tracker
	m.percentiles = NewPercentileTracker(&PercentileConfig{
		Accuracy:    m.config.PercentileAccuracy,
		Compression: m.config.PercentileCompression,
	})
	
	// SLA violation tracker
	m.slaViolations = NewSLAViolationTracker(&SLAViolationConfig{
		RetentionPeriod: m.config.RetentionPeriod,
		AlertWindow:     m.config.AlertWindow,
	}, m.logger)
	
	// Alert manager
	m.alertManager = NewAlertManager(&AlertManagerConfig{
		Cooldown: m.config.AlertCooldown,
		Window:   m.config.AlertWindow,
	}, m.logger)
	
	// Degradation detector
	m.degradationDetector = NewDegradationDetector(&DegradationConfig{
		Window:     m.config.DegradationWindow,
		Threshold:  m.config.DegradationThreshold,
		MinSamples: m.config.DegradationMinSamples,
	}, m.logger)
}

// runStatsProcessor continuously processes latency measurements
func (m *LatencyMonitor) runStatsProcessor(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopped:
			return
		case <-ticker.C:
			m.processLatencyBuffer()
		}
	}
}

// runSLAMonitor continuously monitors SLA compliance
func (m *LatencyMonitor) runSLAMonitor(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopped:
			return
		case <-ticker.C:
			m.checkSLACompliance()
		}
	}
}

// runDegradationDetector continuously monitors for performance degradation
func (m *LatencyMonitor) runDegradationDetector(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopped:
			return
		case <-ticker.C:
			m.checkPerformanceDegradation()
		}
	}
}

// runMetricsExporter continuously exports metrics to Prometheus
func (m *LatencyMonitor) runMetricsExporter(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopped:
			return
		case <-ticker.C:
			m.exportMetrics()
		}
	}
}

// processLatencyBuffer processes accumulated latency measurements
func (m *LatencyMonitor) processLatencyBuffer() {
	measurements := m.latencyBuffer.Flush()
	if len(measurements) == 0 {
		return
	}
	
	// Update percentile tracker
	for _, measurement := range measurements {
		m.percentiles.Add(measurement.Duration)
		m.degradationDetector.Add(measurement)
	}
	
	// Calculate current statistics
	stats := m.calculateStats(measurements)
	
	// Update current stats
	m.statsLock.Lock()
	m.currentStats = stats
	m.statsLock.Unlock()
	
	m.logger.Debug("Processed latency measurements",
		zap.Int("count", len(measurements)),
		zap.Duration("p50", stats.P50),
		zap.Duration("p95", stats.P95),
		zap.Duration("p99", stats.P99),
		zap.Float64("throughput", stats.Throughput),
	)
}

// calculateStats calculates statistics from measurements
func (m *LatencyMonitor) calculateStats(measurements []*LatencyMeasurement) *LatencyStats {
	if len(measurements) == 0 {
		return m.currentStats
	}
	
	// Extract durations for percentile calculation
	durations := make([]float64, len(measurements))
	var total time.Duration
	min := measurements[0].Duration
	max := measurements[0].Duration
	
	cacheHitDurations := make([]time.Duration, 0)
	cacheMissDurations := make([]time.Duration, 0)
	
	for i, measurement := range measurements {
		duration := measurement.Duration
		durations[i] = float64(duration.Nanoseconds())
		total += duration
		
		if duration < min {
			min = duration
		}
		if duration > max {
			max = duration
		}
		
		if measurement.CacheHit {
			cacheHitDurations = append(cacheHitDurations, duration)
		} else {
			cacheMissDurations = append(cacheMissDurations, duration)
		}
	}
	
	// Calculate percentiles
	p50 := m.percentiles.Quantile(0.50)
	p95 := m.percentiles.Quantile(0.95)
	p99 := m.percentiles.Quantile(0.99)
	p999 := m.percentiles.Quantile(0.999)
	
	// Calculate throughput (measurements per second)
	elapsed := time.Since(measurements[0].Timestamp)
	if elapsed > 0 {
		elapsed = time.Since(measurements[len(measurements)-1].Timestamp)
	}
	if elapsed == 0 {
		elapsed = time.Second
	}
	throughput := float64(len(measurements)) / elapsed.Seconds()
	
	// Calculate cache-specific stats
	cacheHitStats := m.calculateCacheStats(cacheHitDurations)
	cacheMissStats := m.calculateCacheStats(cacheMissDurations)
	
	// Calculate SLA compliance
	slaCompliance := m.calculateSLACompliance(p50, p95, p99, throughput)
	
	return &LatencyStats{
		Count:       int64(len(measurements)),
		P50:         p50,
		P95:         p95,
		P99:         p99,
		P999:        p999,
		Mean:        total / time.Duration(len(measurements)),
		Min:         min,
		Max:         max,
		Throughput:  throughput,
		LastUpdated: time.Now(),
		
		CacheHitP50:  cacheHitStats.P50,
		CacheHitP95:  cacheHitStats.P95,
		CacheHitP99:  cacheHitStats.P99,
		CacheMissP50: cacheMissStats.P50,
		CacheMissP95: cacheMissStats.P95,
		CacheMissP99: cacheMissStats.P99,
		
		SLACompliance: slaCompliance,
	}
}

// calculateCacheStats calculates percentiles for cache hit/miss durations
func (m *LatencyMonitor) calculateCacheStats(durations []time.Duration) *CacheStats {
	if len(durations) == 0 {
		return &CacheStats{}
	}
	
	// Convert to float64 and sort
	values := make([]float64, len(durations))
	for i, d := range durations {
		values[i] = float64(d.Nanoseconds())
	}
	sort.Float64s(values)
	
	return &CacheStats{
		P50: time.Duration(percentile(values, 0.50)),
		P95: time.Duration(percentile(values, 0.95)),
		P99: time.Duration(percentile(values, 0.99)),
	}
}

// calculateSLACompliance calculates SLA compliance percentages
func (m *LatencyMonitor) calculateSLACompliance(p50, p95, p99 time.Duration, throughput float64) *SLACompliance {
	p50Compliance := 100.0
	if p50 > m.config.SLALatencyP50 {
		p50Compliance = float64(m.config.SLALatencyP50) / float64(p50) * 100.0
	}
	
	p95Compliance := 100.0
	if p95 > m.config.SLALatencyP95 {
		p95Compliance = float64(m.config.SLALatencyP95) / float64(p95) * 100.0
	}
	
	p99Compliance := 100.0
	if p99 > m.config.SLALatencyP99 {
		p99Compliance = float64(m.config.SLALatencyP99) / float64(p99) * 100.0
	}
	
	throughputCompliance := 100.0
	if throughput < float64(m.config.SLAThroughputMin) {
		throughputCompliance = throughput / float64(m.config.SLAThroughputMin) * 100.0
	}
	
	// Overall score is weighted average
	overallScore := (p50Compliance*0.2 + p95Compliance*0.3 + p99Compliance*0.4 + throughputCompliance*0.1)
	
	return &SLACompliance{
		P50Compliance:        p50Compliance,
		P95Compliance:        p95Compliance,
		P99Compliance:        p99Compliance,
		ThroughputCompliance: throughputCompliance,
		OverallScore:         overallScore,
	}
}

// checkImmediateSLAViolation checks for immediate SLA violations
func (m *LatencyMonitor) checkImmediateSLAViolation(operation OperationType, duration time.Duration, cacheHit bool) {
	var violated bool
	var violationType string
	
	// Check cache hit SLA
	if cacheHit && duration > m.config.SLACacheHitP99 {
		violated = true
		violationType = "cache_hit_p99"
	}
	
	// Check general latency SLA (P99 threshold)
	if !cacheHit && duration > m.config.SLALatencyP99 {
		violated = true
		violationType = "query_p99"
	}
	
	if violated {
		m.slaViolations.Record(&SLAViolation{
			Type:      violationType,
			Operation: operation,
			Duration:  duration,
			Threshold: m.config.SLALatencyP99,
			CacheHit:  cacheHit,
			Timestamp: time.Now(),
		})
		
		m.metrics.SLAViolationsTotal.Inc()
		
		// Trigger alert if needed
		m.alertManager.TriggerAlert(&Alert{
			Type:        AlertTypeSLAViolation,
			Severity:    AlertSeverityWarning,
			Message:     fmt.Sprintf("SLA violation: %s exceeded %v (actual: %v)", violationType, m.config.SLALatencyP99, duration),
			Timestamp:   time.Now(),
			Metadata:    map[string]interface{}{"operation": operation, "cache_hit": cacheHit},
		})
	}
}

// checkSLACompliance performs periodic SLA compliance checks
func (m *LatencyMonitor) checkSLACompliance() {
	stats := m.GetCurrentStats()
	if stats.SLACompliance == nil {
		return
	}
	
	// Check if overall compliance is below threshold
	if stats.SLACompliance.OverallScore < 95.0 {
		m.alertManager.TriggerAlert(&Alert{
			Type:      AlertTypeSLACompliance,
			Severity:  AlertSeverityWarning,
			Message:   fmt.Sprintf("SLA compliance degraded to %.2f%%", stats.SLACompliance.OverallScore),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"p50_compliance":        stats.SLACompliance.P50Compliance,
				"p95_compliance":        stats.SLACompliance.P95Compliance,
				"p99_compliance":        stats.SLACompliance.P99Compliance,
				"throughput_compliance": stats.SLACompliance.ThroughputCompliance,
			},
		})
	}
}

// checkPerformanceDegradation checks for performance degradation
func (m *LatencyMonitor) checkPerformanceDegradation() {
	degradation := m.degradationDetector.CheckDegradation()
	if degradation != nil {
		m.metrics.DegradationDetected.Inc()
		m.metrics.DegradationSeverity.Set(degradation.Severity)
		
		severity := AlertSeverityWarning
		if degradation.Severity > 2.0 {
			severity = AlertSeverityCritical
		}
		
		m.alertManager.TriggerAlert(&Alert{
			Type:      AlertTypePerformanceDegradation,
			Severity:  severity,
			Message:   fmt.Sprintf("Performance degradation detected: %.2fx slower than baseline", degradation.Severity),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"severity":           degradation.Severity,
				"baseline_latency":   degradation.BaselineLatency,
				"current_latency":    degradation.CurrentLatency,
				"degradation_start":  degradation.StartTime,
			},
		})
	}
}

// exportMetrics exports current statistics to Prometheus
func (m *LatencyMonitor) exportMetrics() {
	stats := m.GetCurrentStats()
	
	// Update percentile gauges
	m.metrics.LatencyP50.Set(stats.P50.Seconds())
	m.metrics.LatencyP95.Set(stats.P95.Seconds())
	m.metrics.LatencyP99.Set(stats.P99.Seconds())
	m.metrics.LatencyP999.Set(stats.P999.Seconds())
	
	// Update throughput
	m.metrics.ThroughputCurrent.Set(stats.Throughput)
	
	// Update SLA compliance
	if stats.SLACompliance != nil {
		m.metrics.SLAComplianceP50.Set(stats.SLACompliance.P50Compliance)
		m.metrics.SLAComplianceP95.Set(stats.SLACompliance.P95Compliance)
		m.metrics.SLAComplianceP99.Set(stats.SLACompliance.P99Compliance)
	}
}

// percentile calculates percentile from sorted values
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	
	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if lower == upper {
		return sorted[lower]
	}
	
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// createMonitorMetrics initializes Prometheus metrics
func createMonitorMetrics() *MonitorMetrics {
	return &MonitorMetrics{
		QueryLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_monitor_query_latency_seconds",
			Help:    "DNC query latency distribution",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 20), // 0.1ms to ~100s
		}),
		CacheHitLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_monitor_cache_hit_latency_seconds",
			Help:    "DNC cache hit latency distribution",
			Buckets: prometheus.ExponentialBuckets(0.00001, 2, 15), // 0.01ms to ~0.3s
		}),
		CacheMissLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_monitor_cache_miss_latency_seconds",
			Help:    "DNC cache miss latency distribution",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~30s
		}),
		LatencyP50: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_latency_p50_seconds",
			Help: "DNC query latency 50th percentile",
		}),
		LatencyP95: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_latency_p95_seconds",
			Help: "DNC query latency 95th percentile",
		}),
		LatencyP99: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_latency_p99_seconds",
			Help: "DNC query latency 99th percentile",
		}),
		LatencyP999: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_latency_p999_seconds",
			Help: "DNC query latency 99.9th percentile",
		}),
		SLAViolationsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_monitor_sla_violations_total",
			Help: "Total number of SLA violations",
		}),
		SLAComplianceP50: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_sla_compliance_p50_percent",
			Help: "SLA compliance for P50 latency",
		}),
		SLAComplianceP95: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_sla_compliance_p95_percent",
			Help: "SLA compliance for P95 latency",
		}),
		SLAComplianceP99: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_sla_compliance_p99_percent",
			Help: "SLA compliance for P99 latency",
		}),
		ThroughputCurrent: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_throughput_current",
			Help: "Current DNC query throughput (queries/second)",
		}),
		ThroughputTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_monitor_throughput_total",
			Help: "Total number of DNC queries processed",
		}),
		DegradationDetected: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_monitor_degradation_detected_total",
			Help: "Total number of performance degradations detected",
		}),
		DegradationSeverity: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_degradation_severity",
			Help: "Current performance degradation severity multiplier",
		}),
		AlertsTriggered: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_monitor_alerts_triggered_total",
			Help: "Total number of alerts triggered",
		}),
		AlertsSuppressed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_monitor_alerts_suppressed_total",
			Help: "Total number of alerts suppressed due to cooldown",
		}),
		AlertCooldownActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_monitor_alert_cooldown_active",
			Help: "Whether alert cooldown is currently active",
		}),
	}
}