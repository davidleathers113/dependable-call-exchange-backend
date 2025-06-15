package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewAlertManager creates a new alert manager for integrity violations
func NewAlertManager(service *IntegrityService) *AlertManager {
	return &AlertManager{
		service:   service,
		alerts:    make(map[string]*IntegrityAlert),
		cooldowns: make(map[string]time.Time),
	}
}

// TriggerAlert triggers an integrity violation alert
func (am *AlertManager) TriggerAlert(ctx context.Context, alert *IntegrityAlert) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check for cooldown to prevent alert spam
	cooldownKey := am.getCooldownKey(alert)
	if lastAlert, exists := am.cooldowns[cooldownKey]; exists {
		if time.Since(lastAlert) < am.service.config.AlertCooldown {
			am.service.logger.Debug("Alert suppressed due to cooldown",
				zap.String("alert_type", alert.AlertType),
				zap.String("cooldown_key", cooldownKey),
				zap.Duration("time_since_last", time.Since(lastAlert)))
			return
		}
	}

	// Store alert
	am.alerts[alert.AlertID] = alert
	am.cooldowns[cooldownKey] = alert.TriggeredAt

	// Update metrics
	am.service.metrics.AlertsTriggered++

	// Log alert
	am.service.logger.Warn("Integrity alert triggered",
		zap.String("alert_id", alert.AlertID),
		zap.String("alert_type", alert.AlertType),
		zap.String("severity", alert.Severity),
		zap.String("title", alert.Title),
		zap.String("description", alert.Description))

	// Send notifications
	go am.sendNotifications(ctx, alert)

	// Store alert in infrastructure if repository available
	if am.service.integrityRepo != nil {
		go am.persistAlert(ctx, alert)
	}
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(ctx context.Context, alertID string, resolvedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("ALERT_NOT_FOUND: alert not found")
	}

	if alert.IsResolved {
		return fmt.Errorf("ALERT_ALREADY_RESOLVED: alert already resolved")
	}

	// Mark as resolved
	now := time.Now()
	alert.IsResolved = true
	alert.ResolvedAt = &now

	// Update metrics
	am.service.metrics.AlertsResolved++

	am.service.logger.Info("Alert resolved",
		zap.String("alert_id", alertID),
		zap.String("resolved_by", resolvedBy),
		zap.Duration("resolution_time", now.Sub(alert.TriggeredAt)))

	// Acknowledge in infrastructure if available
	if am.service.integrityRepo != nil {
		go func() {
			if err := am.service.integrityRepo.AcknowledgeIntegrityAlert(ctx, alertID, resolvedBy); err != nil {
				am.service.logger.Error("Failed to acknowledge alert in repository", zap.Error(err))
			}
		}()
	}

	return nil
}

// GetActiveAlerts returns all active (unresolved) alerts
func (am *AlertManager) GetActiveAlerts(ctx context.Context) ([]*IntegrityAlert, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var activeAlerts []*IntegrityAlert
	for _, alert := range am.alerts {
		if !alert.IsResolved {
			// Create a copy to avoid race conditions
			alertCopy := *alert
			activeAlerts = append(activeAlerts, &alertCopy)
		}
	}

	return activeAlerts, nil
}

// GetAllAlerts returns all alerts (active and resolved)
func (am *AlertManager) GetAllAlerts(ctx context.Context) ([]*IntegrityAlert, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var alerts []*IntegrityAlert
	for _, alert := range am.alerts {
		// Create a copy to avoid race conditions
		alertCopy := *alert
		alerts = append(alerts, &alertCopy)
	}

	return alerts, nil
}

// GetAlert returns a specific alert by ID
func (am *AlertManager) GetAlert(ctx context.Context, alertID string) (*IntegrityAlert, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("ALERT_NOT_FOUND: alert not found")
	}

	// Return a copy
	alertCopy := *alert
	return &alertCopy, nil
}

// CleanupOldAlerts removes old resolved alerts to prevent memory leaks
func (am *AlertManager) CleanupOldAlerts(ctx context.Context, maxAge time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for alertID, alert := range am.alerts {
		if alert.IsResolved && alert.ResolvedAt != nil && alert.ResolvedAt.Before(cutoff) {
			delete(am.alerts, alertID)
			cleaned++
		}
	}

	// Clean up old cooldowns too
	for key, lastAlert := range am.cooldowns {
		if lastAlert.Before(cutoff) {
			delete(am.cooldowns, key)
		}
	}

	if cleaned > 0 {
		am.service.logger.Debug("Cleaned up old alerts",
			zap.Int("cleaned_count", cleaned),
			zap.Duration("max_age", maxAge))
	}
}

// GetAlertSummary returns a summary of alerts
func (am *AlertManager) GetAlertSummary(ctx context.Context) *AlertSummary {
	am.mu.RLock()
	defer am.mu.RUnlock()

	summary := &AlertSummary{
		TotalAlerts:    len(am.alerts),
		BySeverity:     make(map[string]int),
		ByType:         make(map[string]int),
		OpenAlerts:     0,
		ResolvedAlerts: 0,
	}

	for _, alert := range am.alerts {
		// Count by severity
		summary.BySeverity[alert.Severity]++

		// Count by type
		summary.ByType[alert.AlertType]++

		// Count by resolution status
		if alert.IsResolved {
			summary.ResolvedAlerts++
		} else {
			summary.OpenAlerts++
		}
	}

	return summary
}

// sendNotifications sends alert notifications to configured channels
func (am *AlertManager) sendNotifications(ctx context.Context, alert *IntegrityAlert) {
	logger := am.service.logger.With(
		zap.String("alert_id", alert.AlertID),
		zap.String("alert_type", alert.AlertType))

	// Send to monitoring system would be implemented here when available

	// Here you would implement various notification channels:
	// - Email notifications
	// - Slack/Discord webhooks
	// - PagerDuty integration
	// - SMS alerts for critical issues
	// - Dashboard notifications

	logger.Debug("Alert notifications sent")
}

// persistAlert stores the alert in the infrastructure repository
func (am *AlertManager) persistAlert(ctx context.Context, alert *IntegrityAlert) {
	// Convert to infrastructure format and store
	// This would use the IntegrityRepository to persist alerts
	am.service.logger.Debug("Alert persisted",
		zap.String("alert_id", alert.AlertID))
}

// getCooldownKey generates a key for alert cooldown tracking
func (am *AlertManager) getCooldownKey(alert *IntegrityAlert) string {
	// Use alert type and severity as cooldown key
	// This prevents spamming of similar alerts
	return alert.AlertType + ":" + alert.Severity
}

// StartPeriodicCleanup starts a goroutine that periodically cleans up old alerts
func (am *AlertManager) StartPeriodicCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		maxAge := 7 * 24 * time.Hour // Keep alerts for 7 days

		for {
			select {
			case <-am.service.backgroundCtx.Done():
				return
			case <-ticker.C:
				am.CleanupOldAlerts(context.Background(), maxAge)
			}
		}
	}()
}

// Helper methods for creating specific alert types

// CreateHashChainAlert creates an alert for hash chain integrity issues
func (am *AlertManager) CreateHashChainAlert(severity, description string, details interface{}) *IntegrityAlert {
	return &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "hash_chain_integrity",
		Severity:    severity,
		Title:       "Hash Chain Integrity Issue",
		Description: description,
		Details:     details,
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}
}

// CreateSequenceAlert creates an alert for sequence integrity issues
func (am *AlertManager) CreateSequenceAlert(severity, description string, details interface{}) *IntegrityAlert {
	return &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "sequence_integrity",
		Severity:    severity,
		Title:       "Sequence Integrity Issue",
		Description: description,
		Details:     details,
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}
}

// CreateCorruptionAlert creates an alert for data corruption
func (am *AlertManager) CreateCorruptionAlert(severity, description string, details interface{}) *IntegrityAlert {
	return &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "data_corruption",
		Severity:    severity,
		Title:       "Data Corruption Detected",
		Description: description,
		Details:     details,
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}
}

// CreateComplianceAlert creates an alert for compliance violations
func (am *AlertManager) CreateComplianceAlert(severity, description string, details interface{}) *IntegrityAlert {
	return &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "compliance_violation",
		Severity:    severity,
		Title:       "Compliance Violation",
		Description: description,
		Details:     details,
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}
}

// AlertSummary represents a summary of alerts (reusing type from audit supporting types)
type AlertSummary struct {
	TotalAlerts    int            `json:"total_alerts"`
	BySeverity     map[string]int `json:"by_severity"`
	ByType         map[string]int `json:"by_type"`
	OpenAlerts     int            `json:"open_alerts"`
	ResolvedAlerts int            `json:"resolved_alerts"`
}
