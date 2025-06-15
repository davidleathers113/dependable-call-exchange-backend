package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// NewIntegrityScheduler creates a new integrity check scheduler
func NewIntegrityScheduler(service *IntegrityService) *IntegrityScheduler {
	return &IntegrityScheduler{
		service:   service,
		schedules: make(map[string]*ScheduledCheck),
	}
}

// Start begins the scheduling service
func (s *IntegrityScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set up ticker for checking schedules
	s.ticker = time.NewTicker(1 * time.Minute) // Check every minute

	go s.run()

	s.service.logger.Info("Integrity scheduler started")
	return nil
}

// Stop gracefully shuts down the scheduler
func (s *IntegrityScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}

	s.service.logger.Info("Integrity scheduler stopped")
}

// AddSchedule adds a new scheduled check
func (s *IntegrityScheduler) AddSchedule(check *ScheduledCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedules[check.ID] = check

	s.service.logger.Info("Added scheduled check",
		zap.String("check_id", check.ID),
		zap.String("check_name", check.Name),
		zap.String("check_type", check.Type),
		zap.Time("next_run", check.NextRun))
}

// RemoveSchedule removes a scheduled check
func (s *IntegrityScheduler) RemoveSchedule(checkID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.schedules, checkID)

	s.service.logger.Info("Removed scheduled check", zap.String("check_id", checkID))
}

// GetSchedules returns all scheduled checks
func (s *IntegrityScheduler) GetSchedules() map[string]*ScheduledCheck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	schedules := make(map[string]*ScheduledCheck)
	for id, check := range s.schedules {
		checkCopy := *check
		schedules[id] = &checkCopy
	}

	return schedules
}

// GetStatus returns the current scheduler status
func (s *IntegrityScheduler) GetStatus() *SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SchedulerStatus{
		IsRunning:       s.ticker != nil,
		ScheduledChecks: len(s.schedules),
	}

	// Find next scheduled run
	var nextRun *time.Time
	for _, check := range s.schedules {
		if check.IsEnabled && (nextRun == nil || check.NextRun.Before(*nextRun)) {
			nextRun = &check.NextRun
		}
	}
	status.NextScheduledRun = nextRun

	return status
}

// run is the main scheduler loop
func (s *IntegrityScheduler) run() {
	logger := s.service.logger.With(zap.String("component", "scheduler"))

	for {
		select {
		case <-s.service.backgroundCtx.Done():
			logger.Debug("Scheduler stopping due to context cancellation")
			return
		case <-s.ticker.C:
			s.processScheduledChecks()
		}
	}
}

// processScheduledChecks checks for and executes due scheduled checks
func (s *IntegrityScheduler) processScheduledChecks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	logger := s.service.logger.With(zap.String("component", "scheduler"))

	for id, check := range s.schedules {
		if !check.IsEnabled {
			continue
		}

		if now.After(check.NextRun) || now.Equal(check.NextRun) {
			logger.Info("Executing scheduled check",
				zap.String("check_id", id),
				zap.String("check_name", check.Name),
				zap.String("check_type", check.Type))

			// Execute the check
			go s.executeScheduledCheck(check)

			// Update next run time
			check.LastRun = &now
			check.NextRun = s.calculateNextRun(check, now)

			logger.Debug("Scheduled next run",
				zap.String("check_id", id),
				zap.Time("next_run", check.NextRun))
		}
	}
}

// executeScheduledCheck executes a specific scheduled check
func (s *IntegrityScheduler) executeScheduledCheck(check *ScheduledCheck) {
	ctx, cancel := context.WithTimeout(s.service.backgroundCtx, s.service.config.CheckTimeout)
	defer cancel()

	logger := s.service.logger.With(
		zap.String("check_id", check.ID),
		zap.String("check_type", check.Type))

	logger.Debug("Starting scheduled check execution")

	startTime := time.Now()
	var err error

	switch check.Type {
	case "hash_chain":
		err = s.executeHashChainCheck(ctx, check)
	case "sequence":
		err = s.executeSequenceCheck(ctx, check)
	case "corruption":
		err = s.executeCorruptionCheck(ctx, check)
	case "compliance":
		err = s.executeComplianceCheck(ctx, check)
	case "comprehensive":
		err = s.executeComprehensiveCheck(ctx, check)
	default:
		logger.Error("Unknown check type", zap.String("type", check.Type))
		return
	}

	duration := time.Since(startTime)

	if err != nil {
		logger.Error("Scheduled check failed",
			zap.Error(err),
			zap.Duration("duration", duration))
	} else {
		logger.Info("Scheduled check completed successfully",
			zap.Duration("duration", duration))
	}
}

// executeHashChainCheck executes a hash chain verification check
func (s *IntegrityScheduler) executeHashChainCheck(ctx context.Context, check *ScheduledCheck) error {
	// Get latest sequence number
	latestSeq, err := s.service.eventRepo.GetLatestSequenceNumber(ctx)
	if err != nil {
		return err
	}

	// Check recent events (configurable range)
	checkSize := values.SequenceNumber(s.service.config.IncrementalCheckSize)
	startSeq := latestSeq
	if latestSeq > checkSize {
		startSeq = latestSeq - checkSize
	} else {
		startSeq = 1
	}

	_, err = s.service.VerifyHashChain(ctx, startSeq, latestSeq)
	return err
}

// executeSequenceCheck executes a sequence integrity check
func (s *IntegrityScheduler) executeSequenceCheck(ctx context.Context, check *ScheduledCheck) error {
	// Get latest sequence number
	latestSeq, err := s.service.eventRepo.GetLatestSequenceNumber(ctx)
	if err != nil {
		return err
	}

	// Check sequence integrity for recent events
	checkSize := values.SequenceNumber(s.service.config.IncrementalCheckSize)
	startSeq := latestSeq
	if latestSeq > checkSize {
		startSeq = latestSeq - checkSize
	} else {
		startSeq = 1
	}

	criteria := audit.SequenceIntegrityCriteria{
		StartSequence:   &startSeq,
		EndSequence:     &latestSeq,
		CheckGaps:       true,
		CheckDuplicates: true,
		CheckOrder:      true,
		CheckContinuity: true,
	}

	_, err = s.service.VerifySequenceIntegrity(ctx, criteria)
	return err
}

// executeCorruptionCheck executes a corruption detection scan
func (s *IntegrityScheduler) executeCorruptionCheck(ctx context.Context, check *ScheduledCheck) error {
	// Get time range for corruption check (last hour)
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	criteria := audit.CorruptionDetectionCriteria{
		StartTime:           &startTime,
		EndTime:             &endTime,
		CheckHashes:         true,
		CheckMetadata:       true,
		CheckReferences:     true,
		CheckConsistency:    true,
		DeepScan:            false, // Light scan for scheduled checks
		StatisticalAnalysis: true,
		SampleRate:          0.1, // Sample 10% of events
	}

	_, err = s.service.DetectCorruption(ctx, criteria)
	return err
}

// executeComplianceCheck executes a compliance verification check
func (s *IntegrityScheduler) executeComplianceCheck(ctx context.Context, check *ScheduledCheck) error {
	// This would implement compliance-specific scheduled checks
	// For now, return success
	return nil
}

// executeComprehensiveCheck executes a comprehensive integrity check
func (s *IntegrityScheduler) executeComprehensiveCheck(ctx context.Context, check *ScheduledCheck) error {
	// Get time range for comprehensive check (last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	criteria := audit.IntegrityCriteria{
		StartTime:       &startTime,
		EndTime:         &endTime,
		CheckHashChain:  true,
		CheckSequencing: true,
		CheckCompliance: true,
	}

	_, err = s.service.PerformIntegrityCheck(ctx, criteria)
	return err
}

// calculateNextRun calculates the next run time for a scheduled check
func (s *IntegrityScheduler) calculateNextRun(check *ScheduledCheck, lastRun time.Time) time.Time {
	// For now, just add the interval to the last run time
	// In a production system, you'd parse the cron expression
	return lastRun.Add(check.Interval)
}

// Helper methods for default scheduled checks

// SetupDefaultSchedules sets up default integrity check schedules
func (s *IntegrityScheduler) SetupDefaultSchedules() {
	// Hash chain check every 5 minutes
	s.AddSchedule(&ScheduledCheck{
		ID:        uuid.New().String(),
		Name:      "Default Hash Chain Check",
		Type:      "hash_chain",
		Interval:  5 * time.Minute,
		NextRun:   time.Now().Add(5 * time.Minute),
		IsEnabled: true,
	})

	// Sequence check every 10 minutes
	s.AddSchedule(&ScheduledCheck{
		ID:        uuid.New().String(),
		Name:      "Default Sequence Check",
		Type:      "sequence",
		Interval:  10 * time.Minute,
		NextRun:   time.Now().Add(10 * time.Minute),
		IsEnabled: true,
	})

	// Corruption scan every 30 minutes
	s.AddSchedule(&ScheduledCheck{
		ID:        uuid.New().String(),
		Name:      "Default Corruption Scan",
		Type:      "corruption",
		Interval:  30 * time.Minute,
		NextRun:   time.Now().Add(30 * time.Minute),
		IsEnabled: true,
	})

	// Comprehensive check every hour
	s.AddSchedule(&ScheduledCheck{
		ID:        uuid.New().String(),
		Name:      "Default Comprehensive Check",
		Type:      "comprehensive",
		Interval:  1 * time.Hour,
		NextRun:   time.Now().Add(1 * time.Hour),
		IsEnabled: true,
	})
}

// UpdateSchedule updates an existing scheduled check
func (s *IntegrityScheduler) UpdateSchedule(checkID string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	check, exists := s.schedules[checkID]
	if !exists {
		return fmt.Errorf("SCHEDULE_NOT_FOUND: scheduled check not found")
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		check.Name = name
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		check.IsEnabled = enabled
	}
	if interval, ok := updates["interval"].(time.Duration); ok {
		check.Interval = interval
		// Recalculate next run
		now := time.Now()
		check.NextRun = s.calculateNextRun(check, now)
	}

	s.service.logger.Info("Updated scheduled check",
		zap.String("check_id", checkID),
		zap.Any("updates", updates))

	return nil
}
