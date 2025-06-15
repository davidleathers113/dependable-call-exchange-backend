package performance

import (
	"container/ring"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LatencyBuffer implements a circular buffer for latency measurements
type LatencyBufferImpl struct {
	logger *zap.Logger
	config *LatencyBufferConfig
	
	buffer *ring.Ring
	mutex  sync.RWMutex
	
	measurements []*LatencyMeasurement
	measureMutex sync.Mutex
}

// NewLatencyBuffer creates a new latency buffer
func NewLatencyBuffer(config *LatencyBufferConfig, logger *zap.Logger) *LatencyBufferImpl {
	return &LatencyBufferImpl{
		logger:       logger,
		config:       config,
		buffer:       ring.New(config.Size),
		measurements: make([]*LatencyMeasurement, 0, config.Size),
	}
}

// Add adds a measurement to the buffer
func (lb *LatencyBufferImpl) Add(measurement *LatencyMeasurement) {
	lb.measureMutex.Lock()
	defer lb.measureMutex.Unlock()
	
	lb.measurements = append(lb.measurements, measurement)
	
	// If buffer is full, remove oldest measurements
	if len(lb.measurements) > lb.config.Size {
		lb.measurements = lb.measurements[1:]
	}
}

// Flush returns and clears all measurements
func (lb *LatencyBufferImpl) Flush() []*LatencyMeasurement {
	lb.measureMutex.Lock()
	defer lb.measureMutex.Unlock()
	
	measurements := make([]*LatencyMeasurement, len(lb.measurements))
	copy(measurements, lb.measurements)
	lb.measurements = lb.measurements[:0] // Clear but keep capacity
	
	return measurements
}

// Size returns the current buffer size
func (lb *LatencyBufferImpl) Size() int {
	lb.measureMutex.Lock()
	defer lb.measureMutex.Unlock()
	return len(lb.measurements)
}

// IsFull returns whether the buffer is full
func (lb *LatencyBufferImpl) IsFull() bool {
	return lb.Size() >= lb.config.Size
}

// PercentileTrackerImpl implements percentile tracking using a simple sorted slice
type PercentileTrackerImpl struct {
	durations []time.Duration
	mutex     sync.RWMutex
	config    *PercentileConfig
}

// NewPercentileTracker creates a new percentile tracker
func NewPercentileTracker(config *PercentileConfig) *PercentileTrackerImpl {
	return &PercentileTrackerImpl{
		durations: make([]time.Duration, 0, 10000),
		config:    config,
	}
}

// Add adds a duration to the tracker
func (pt *PercentileTrackerImpl) Add(duration time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	
	// Insert in sorted order (simple implementation)
	pt.durations = append(pt.durations, duration)
	
	// Keep only recent measurements
	if len(pt.durations) > 10000 {
		pt.durations = pt.durations[1000:] // Remove oldest 1000
	}
}

// Quantile returns the specified quantile
func (pt *PercentileTrackerImpl) Quantile(q float64) time.Duration {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()
	
	if len(pt.durations) == 0 {
		return 0
	}
	
	// Sort durations (this is inefficient but simple)
	durations := make([]time.Duration, len(pt.durations))
	copy(durations, pt.durations)
	
	// Simple insertion sort for small arrays
	for i := 1; i < len(durations); i++ {
		key := durations[i]
		j := i - 1
		for j >= 0 && durations[j] > key {
			durations[j+1] = durations[j]
			j--
		}
		durations[j+1] = key
	}
	
	index := int(float64(len(durations)-1) * q)
	return durations[index]
}

// Count returns the number of measurements
func (pt *PercentileTrackerImpl) Count() int64 {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()
	return int64(len(pt.durations))
}

// Reset clears all measurements
func (pt *PercentileTrackerImpl) Reset() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.durations = pt.durations[:0]
}

// SLAViolationTrackerImpl tracks SLA violations
type SLAViolationTrackerImpl struct {
	logger     *zap.Logger
	config     *SLAViolationConfig
	violations []*SLAViolation
	mutex      sync.RWMutex
}

// NewSLAViolationTracker creates a new SLA violation tracker
func NewSLAViolationTracker(config *SLAViolationConfig, logger *zap.Logger) *SLAViolationTrackerImpl {
	return &SLAViolationTrackerImpl{
		logger:     logger,
		config:     config,
		violations: make([]*SLAViolation, 0),
	}
}

// Record records a new SLA violation
func (svt *SLAViolationTrackerImpl) Record(violation *SLAViolation) {
	svt.mutex.Lock()
	defer svt.mutex.Unlock()
	
	svt.violations = append(svt.violations, violation)
	
	// Clean up old violations
	now := time.Now()
	cutoff := now.Add(-svt.config.RetentionPeriod)
	
	var filtered []*SLAViolation
	for _, v := range svt.violations {
		if v.Timestamp.After(cutoff) {
			filtered = append(filtered, v)
		}
	}
	svt.violations = filtered
	
	svt.logger.Warn("SLA violation recorded",
		zap.String("type", violation.Type),
		zap.String("operation", violation.Operation.String()),
		zap.Duration("duration", violation.Duration),
		zap.Duration("threshold", violation.Threshold),
		zap.Bool("cache_hit", violation.CacheHit),
	)
}

// GetViolations returns violations within the specified window
func (svt *SLAViolationTrackerImpl) GetViolations(window time.Duration) []*SLAViolation {
	svt.mutex.RLock()
	defer svt.mutex.RUnlock()
	
	cutoff := time.Now().Add(-window)
	var result []*SLAViolation
	
	for _, violation := range svt.violations {
		if violation.Timestamp.After(cutoff) {
			result = append(result, violation)
		}
	}
	
	return result
}

// GetViolationRate returns the violation rate within the window
func (svt *SLAViolationTrackerImpl) GetViolationRate(window time.Duration) float64 {
	violations := svt.GetViolations(window)
	
	if len(violations) == 0 {
		return 0.0
	}
	
	// Calculate rate per hour
	return float64(len(violations)) / window.Hours()
}

// Clear clears all violations
func (svt *SLAViolationTrackerImpl) Clear() {
	svt.mutex.Lock()
	defer svt.mutex.Unlock()
	svt.violations = svt.violations[:0]
}

// AlertManagerImpl manages performance alerts
type AlertManagerImpl struct {
	logger   *zap.Logger
	config   *AlertManagerConfig
	
	alerts    []*Alert
	cooldowns map[AlertType]time.Time
	mutex     sync.RWMutex
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config *AlertManagerConfig, logger *zap.Logger) *AlertManagerImpl {
	return &AlertManagerImpl{
		logger:    logger,
		config:    config,
		alerts:    make([]*Alert, 0),
		cooldowns: make(map[AlertType]time.Time),
	}
}

// TriggerAlert triggers a new alert
func (am *AlertManagerImpl) TriggerAlert(alert *Alert) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	// Check cooldown
	if cooldownEnd, exists := am.cooldowns[alert.Type]; exists {
		if time.Now().Before(cooldownEnd) {
			am.logger.Debug("Alert suppressed due to cooldown",
				zap.String("type", alert.Type.String()),
				zap.Duration("remaining", time.Until(cooldownEnd)),
			)
			return nil
		}
	}
	
	// Generate alert ID
	alert.ID = generateAlertID(alert)
	
	// Add to alerts
	am.alerts = append(am.alerts, alert)
	
	// Set cooldown
	am.cooldowns[alert.Type] = time.Now().Add(am.config.Cooldown)
	
	am.logger.Warn("Performance alert triggered",
		zap.String("id", alert.ID),
		zap.String("type", alert.Type.String()),
		zap.String("severity", alert.Severity.String()),
		zap.String("message", alert.Message),
	)
	
	return nil
}

// ResolveAlert resolves an existing alert
func (am *AlertManagerImpl) ResolveAlert(alertID string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	for _, alert := range am.alerts {
		if alert.ID == alertID && !alert.Resolved {
			alert.Resolved = true
			now := time.Now()
			alert.ResolvedAt = &now
			
			am.logger.Info("Alert resolved",
				zap.String("id", alertID),
				zap.String("type", alert.Type.String()),
			)
			return nil
		}
	}
	
	return fmt.Errorf("alert not found: %s", alertID)
}

// GetActiveAlerts returns all active alerts
func (am *AlertManagerImpl) GetActiveAlerts() []*Alert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	var active []*Alert
	for _, alert := range am.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	
	return active
}

// IsInCooldown checks if an alert type is in cooldown
func (am *AlertManagerImpl) IsInCooldown(alertType AlertType) bool {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	if cooldownEnd, exists := am.cooldowns[alertType]; exists {
		return time.Now().Before(cooldownEnd)
	}
	
	return false
}

// SetCooldown sets a cooldown for an alert type
func (am *AlertManagerImpl) SetCooldown(alertType AlertType, duration time.Duration) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	am.cooldowns[alertType] = time.Now().Add(duration)
}

// DegradationDetectorImpl detects performance degradation
type DegradationDetectorImpl struct {
	logger     *zap.Logger
	config     *DegradationConfig
	
	measurements []*LatencyMeasurement
	baseline     *PerformanceBaseline
	mutex        sync.RWMutex
}

// NewDegradationDetector creates a new degradation detector
func NewDegradationDetector(config *DegradationConfig, logger *zap.Logger) *DegradationDetectorImpl {
	return &DegradationDetectorImpl{
		logger:       logger,
		config:       config,
		measurements: make([]*LatencyMeasurement, 0),
	}
}

// Add adds a measurement for degradation analysis
func (dd *DegradationDetectorImpl) Add(measurement *LatencyMeasurement) {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()
	
	dd.measurements = append(dd.measurements, measurement)
	
	// Keep only recent measurements
	cutoff := time.Now().Add(-dd.config.Window)
	var filtered []*LatencyMeasurement
	for _, m := range dd.measurements {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	dd.measurements = filtered
}

// CheckDegradation checks for performance degradation
func (dd *DegradationDetectorImpl) CheckDegradation() *PerformanceDegradation {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()
	
	if len(dd.measurements) < dd.config.MinSamples {
		return nil
	}
	
	if dd.baseline == nil {
		// Create initial baseline
		dd.createBaseline()
		return nil
	}
	
	// Calculate current performance
	current := dd.calculateCurrentPerformance()
	
	// Check for degradation
	degradationRatio := float64(current.P99Latency) / float64(dd.baseline.P99Latency)
	
	if degradationRatio > dd.config.Threshold {
		return &PerformanceDegradation{
			DetectedAt:      time.Now(),
			Severity:        degradationRatio,
			BaselineLatency: dd.baseline.P99Latency,
			CurrentLatency:  current.P99Latency,
			Confidence:      dd.calculateConfidence(current),
		}
	}
	
	return nil
}

// GetTrend returns performance trend analysis
func (dd *DegradationDetectorImpl) GetTrend(window time.Duration) *PerformanceTrend {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()
	
	cutoff := time.Now().Add(-window)
	var windowMeasurements []*LatencyMeasurement
	
	for _, m := range dd.measurements {
		if m.Timestamp.After(cutoff) {
			windowMeasurements = append(windowMeasurements, m)
		}
	}
	
	if len(windowMeasurements) < 2 {
		return nil
	}
	
	// Calculate trend (simplified linear regression)
	trend := dd.calculateTrend(windowMeasurements)
	
	return &PerformanceTrend{
		Window:       window,
		StartTime:    cutoff,
		EndTime:      time.Now(),
		TrendSlope:   trend,
		Measurements: len(windowMeasurements),
		Baseline:     dd.baseline,
		Current:      dd.calculateCurrentPerformance(),
	}
}

// SetBaseline sets a new performance baseline
func (dd *DegradationDetectorImpl) SetBaseline(baseline *PerformanceBaseline) {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()
	dd.baseline = baseline
}

// Reset clears all measurements and baseline
func (dd *DegradationDetectorImpl) Reset() {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()
	dd.measurements = dd.measurements[:0]
	dd.baseline = nil
}

// createBaseline creates a performance baseline from current measurements
func (dd *DegradationDetectorImpl) createBaseline() {
	if len(dd.measurements) < dd.config.MinSamples {
		return
	}
	
	// Calculate percentiles from measurements
	durations := make([]time.Duration, len(dd.measurements))
	for i, m := range dd.measurements {
		durations[i] = m.Duration
	}
	
	// Sort durations
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
	
	p50Index := int(0.50 * float64(len(durations)))
	p95Index := int(0.95 * float64(len(durations)))
	p99Index := int(0.99 * float64(len(durations)))
	
	dd.baseline = &PerformanceBaseline{
		StartTime:    dd.measurements[0].Timestamp,
		EndTime:      dd.measurements[len(dd.measurements)-1].Timestamp,
		P50Latency:   durations[p50Index],
		P95Latency:   durations[p95Index],
		P99Latency:   durations[p99Index],
		Measurements: len(dd.measurements),
	}
	
	dd.logger.Info("Performance baseline created",
		zap.Duration("p50", dd.baseline.P50Latency),
		zap.Duration("p95", dd.baseline.P95Latency),
		zap.Duration("p99", dd.baseline.P99Latency),
		zap.Int("measurements", dd.baseline.Measurements),
	)
}

// calculateCurrentPerformance calculates current performance metrics
func (dd *DegradationDetectorImpl) calculateCurrentPerformance() *PerformanceSnapshot {
	// Use recent measurements for current performance
	recentCutoff := time.Now().Add(-time.Minute)
	var recentMeasurements []*LatencyMeasurement
	
	for _, m := range dd.measurements {
		if m.Timestamp.After(recentCutoff) {
			recentMeasurements = append(recentMeasurements, m)
		}
	}
	
	if len(recentMeasurements) == 0 {
		return &PerformanceSnapshot{Timestamp: time.Now()}
	}
	
	// Calculate percentiles
	durations := make([]time.Duration, len(recentMeasurements))
	for i, m := range recentMeasurements {
		durations[i] = m.Duration
	}
	
	// Sort durations
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
	
	p50Index := int(0.50 * float64(len(durations)))
	p95Index := int(0.95 * float64(len(durations)))
	p99Index := int(0.99 * float64(len(durations)))
	
	return &PerformanceSnapshot{
		Timestamp:  time.Now(),
		P50Latency: durations[p50Index],
		P95Latency: durations[p95Index],
		P99Latency: durations[p99Index],
		Throughput: float64(len(recentMeasurements)) / 60.0, // per second
	}
}

// calculateConfidence calculates confidence in degradation detection
func (dd *DegradationDetectorImpl) calculateConfidence(current *PerformanceSnapshot) float64 {
	// Simple confidence calculation based on sample size
	if len(dd.measurements) < dd.config.MinSamples {
		return 0.0
	}
	
	if len(dd.measurements) > dd.config.MinSamples*10 {
		return 0.95
	}
	
	return float64(len(dd.measurements)) / float64(dd.config.MinSamples*10) * 0.95
}

// calculateTrend calculates trend slope from measurements
func (dd *DegradationDetectorImpl) calculateTrend(measurements []*LatencyMeasurement) float64 {
	if len(measurements) < 2 {
		return 0.0
	}
	
	// Simple linear regression
	n := float64(len(measurements))
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, m := range measurements {
		x := float64(i)
		y := float64(m.Duration.Nanoseconds())
		
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	
	return slope
}

// generateAlertID generates a unique alert ID
func generateAlertID(alert *Alert) string {
	return fmt.Sprintf("%s_%d", alert.Type.String(), alert.Timestamp.Unix())
}