package compliance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RuleRepository defines the interface for compliance rule persistence
type RuleRepository interface {
	// Save creates or updates a compliance rule
	Save(ctx context.Context, rule *ComplianceRule) error

	// GetByID retrieves a rule by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*ComplianceRule, error)

	// FindByTypeAndGeography finds rules by type and geographic scope
	FindByTypeAndGeography(ctx context.Context, ruleType RuleType, geography GeographicScope) ([]*ComplianceRule, error)

	// FindActiveRules retrieves all currently active rules
	FindActiveRules(ctx context.Context) ([]*ComplianceRule, error)

	// FindByType retrieves rules by type
	FindByType(ctx context.Context, ruleType RuleType) ([]*ComplianceRule, error)

	// FindExpiring finds rules expiring within specified days
	FindExpiring(ctx context.Context, days int) ([]*ComplianceRule, error)

	// Delete removes a rule (soft delete for audit trail)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ViolationRepository defines the interface for compliance violation persistence
type ViolationRepository interface {
	// Save creates or updates a compliance violation
	Save(ctx context.Context, violation *ComplianceViolation) error

	// GetByID retrieves a violation by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*ComplianceViolation, error)

	// FindByCallID retrieves violations for a specific call
	FindByCallID(ctx context.Context, callID uuid.UUID) ([]*ComplianceViolation, error)

	// FindByAccountID retrieves violations for a specific account
	FindByAccountID(ctx context.Context, accountID uuid.UUID) ([]*ComplianceViolation, error)

	// FindByRuleID retrieves violations for a specific rule
	FindByRuleID(ctx context.Context, ruleID uuid.UUID) ([]*ComplianceViolation, error)

	// FindUnresolved retrieves all unresolved violations
	FindUnresolved(ctx context.Context) ([]*ComplianceViolation, error)

	// FindBySeverity retrieves violations by severity level
	FindBySeverity(ctx context.Context, severity Severity) ([]*ComplianceViolation, error)

	// MarkResolved marks a violation as resolved
	MarkResolved(ctx context.Context, id uuid.UUID, resolvedBy uuid.UUID) error
}

// ConsentRecordRepository defines the interface for consent record persistence
type ConsentRecordRepository interface {
	// Save creates or updates a consent record
	Save(ctx context.Context, record *ConsentRecord) error

	// GetByID retrieves a consent record by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*ConsentRecord, error)

	// FindByPhoneNumber retrieves consent records for a phone number
	FindByPhoneNumber(ctx context.Context, phoneNumber string) ([]*ConsentRecord, error)

	// FindActive retrieves active consent records
	FindActive(ctx context.Context) ([]*ConsentRecord, error)

	// FindExpired retrieves expired consent records
	FindExpired(ctx context.Context) ([]*ConsentRecord, error)

	// FindExpiring finds consent records expiring within specified days
	FindExpiring(ctx context.Context, days int) ([]*ConsentRecord, error)

	// RevokeConsent revokes consent for a phone number
	RevokeConsent(ctx context.Context, phoneNumber string) error
}

// ComplianceFilter defines filters for querying compliance data
type ComplianceFilter struct {
	RuleType     *RuleType
	Status       *RuleStatus
	Severity     *Severity
	AccountID    *uuid.UUID
	CallID       *uuid.UUID
	Geography    *GeographicScope
	CreatedAfter *time.Time
	CreatedBefore *time.Time
	Resolved     *bool
	Limit        int
	Offset       int
}

// QueryRepository defines additional query methods for compliance data
type QueryRepository interface {
	// FindRulesByFilter searches rules with advanced filtering
	FindRulesByFilter(ctx context.Context, filter ComplianceFilter) ([]*ComplianceRule, error)

	// FindViolationsByFilter searches violations with advanced filtering
	FindViolationsByFilter(ctx context.Context, filter ComplianceFilter) ([]*ComplianceViolation, error)

	// GetComplianceMetrics retrieves aggregated compliance metrics
	GetComplianceMetrics(ctx context.Context, timeRange DateRange) (*ComplianceMetrics, error)

	// GetViolationTrends retrieves violation trend data
	GetViolationTrends(ctx context.Context, timeRange DateRange, granularity string) ([]ViolationTrend, error)
}

// DateRange represents a time range for queries
type DateRange struct {
	Start time.Time
	End   time.Time
}

// ComplianceMetrics represents aggregated compliance data
type ComplianceMetrics struct {
	TotalRules          int64
	ActiveRules         int64
	TotalViolations     int64
	UnresolvedViolations int64
	ViolationsByType    map[ViolationType]int64
	ViolationsBySeverity map[Severity]int64
	ComplianceRate      float64 // Percentage of compliant calls
}

// ViolationTrend represents violation trend data over time
type ViolationTrend struct {
	Date       time.Time
	Count      int64
	Type       ViolationType
	Severity   Severity
}