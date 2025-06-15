package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// QueryBuilder constructs complex audit queries with filtering and optimization
type QueryBuilder struct {
	filter      *audit.EventFilter
	conditions  []string
	params      map[string]interface{}
	orderBy     []string
	limit       int
	offset      int
	aggregation *AggregationQuery
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		filter:     &audit.EventFilter{},
		conditions: make([]string, 0),
		params:     make(map[string]interface{}),
		orderBy:    make([]string, 0),
	}
}

// WithEventTypes filters by event types
func (qb *QueryBuilder) WithEventTypes(types []audit.EventType) *QueryBuilder {
	qb.filter.Types = types
	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i, eventType := range types {
			placeholders[i] = fmt.Sprintf("$%d", len(qb.params)+1)
			qb.params[fmt.Sprintf("type_%d", i)] = string(eventType)
		}
		qb.conditions = append(qb.conditions, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}
	return qb
}

// WithActorIDs filters by actor IDs
func (qb *QueryBuilder) WithActorIDs(actorIDs []string) *QueryBuilder {
	qb.filter.ActorIDs = actorIDs
	if len(actorIDs) > 0 {
		placeholders := make([]string, len(actorIDs))
		for i, actorID := range actorIDs {
			placeholders[i] = fmt.Sprintf("$%d", len(qb.params)+1)
			qb.params[fmt.Sprintf("actor_%d", i)] = actorID
		}
		qb.conditions = append(qb.conditions, fmt.Sprintf("actor_id IN (%s)", strings.Join(placeholders, ",")))
	}
	return qb
}

// WithTargetIDs filters by target IDs
func (qb *QueryBuilder) WithTargetIDs(targetIDs []string) *QueryBuilder {
	qb.filter.TargetIDs = targetIDs
	if len(targetIDs) > 0 {
		placeholders := make([]string, len(targetIDs))
		for i, targetID := range targetIDs {
			placeholders[i] = fmt.Sprintf("$%d", len(qb.params)+1)
			qb.params[fmt.Sprintf("target_%d", i)] = targetID
		}
		qb.conditions = append(qb.conditions, fmt.Sprintf("target_id IN (%s)", strings.Join(placeholders, ",")))
	}
	return qb
}

// WithTimeRange filters by time range
func (qb *QueryBuilder) WithTimeRange(start, end time.Time) *QueryBuilder {
	qb.filter.StartTime = &start
	qb.filter.EndTime = &end

	startParam := fmt.Sprintf("$%d", len(qb.params)+1)
	qb.params["start_time"] = start

	endParam := fmt.Sprintf("$%d", len(qb.params)+1)
	qb.params["end_time"] = end

	qb.conditions = append(qb.conditions,
		fmt.Sprintf("timestamp >= %s AND timestamp <= %s", startParam, endParam))

	return qb
}

// WithSeverity filters by severity levels
func (qb *QueryBuilder) WithSeverity(severities []audit.Severity) *QueryBuilder {
	qb.filter.Severities = severities
	if len(severities) > 0 {
		placeholders := make([]string, len(severities))
		for i, severity := range severities {
			placeholders[i] = fmt.Sprintf("$%d", len(qb.params)+1)
			qb.params[fmt.Sprintf("severity_%d", i)] = string(severity)
		}
		qb.conditions = append(qb.conditions, fmt.Sprintf("severity IN (%s)", strings.Join(placeholders, ",")))
	}
	return qb
}

// WithDataClasses filters by data classes
func (qb *QueryBuilder) WithDataClasses(dataClasses []string) *QueryBuilder {
	qb.filter.DataClasses = dataClasses
	if len(dataClasses) > 0 {
		// Use JSON array overlap operator for PostgreSQL
		placeholders := make([]string, len(dataClasses))
		for i, dataClass := range dataClasses {
			placeholders[i] = fmt.Sprintf("$%d", len(qb.params)+1)
			qb.params[fmt.Sprintf("data_class_%d", i)] = dataClass
		}
		qb.conditions = append(qb.conditions,
			fmt.Sprintf("data_classes && ARRAY[%s]", strings.Join(placeholders, ",")))
	}
	return qb
}

// WithComplianceFlags filters by compliance flags
func (qb *QueryBuilder) WithComplianceFlags(flags map[string]interface{}) *QueryBuilder {
	qb.filter.ComplianceFlags = flags
	for key, value := range flags {
		paramKey := fmt.Sprintf("compliance_%s", strings.ReplaceAll(key, ".", "_"))
		paramPlaceholder := fmt.Sprintf("$%d", len(qb.params)+1)
		qb.params[paramKey] = value

		// Use JSON path for nested compliance flags
		qb.conditions = append(qb.conditions,
			fmt.Sprintf("compliance_flags->>'%s' = %s", key, paramPlaceholder))
	}
	return qb
}

// WithSequenceRange filters by sequence number range
func (qb *QueryBuilder) WithSequenceRange(start, end values.SequenceNumber) *QueryBuilder {
	qb.filter.StartSequence = &start
	qb.filter.EndSequence = &end

	startParam := fmt.Sprintf("$%d", len(qb.params)+1)
	qb.params["start_seq"] = int64(start)

	endParam := fmt.Sprintf("$%d", len(qb.params)+1)
	qb.params["end_seq"] = int64(end)

	qb.conditions = append(qb.conditions,
		fmt.Sprintf("sequence_num >= %s AND sequence_num <= %s", startParam, endParam))

	return qb
}

// WithCustomCondition adds a custom SQL condition
func (qb *QueryBuilder) WithCustomCondition(condition string, params map[string]interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, condition)
	for key, value := range params {
		qb.params[key] = value
	}
	return qb
}

// WithMetadataFilter filters by metadata fields
func (qb *QueryBuilder) WithMetadataFilter(key string, value interface{}) *QueryBuilder {
	paramKey := fmt.Sprintf("metadata_%s", strings.ReplaceAll(key, ".", "_"))
	paramPlaceholder := fmt.Sprintf("$%d", len(qb.params)+1)
	qb.params[paramKey] = value

	// Use JSON path for metadata filtering
	qb.conditions = append(qb.conditions,
		fmt.Sprintf("metadata->>'%s' = %s", key, paramPlaceholder))

	return qb
}

// OrderBy adds ordering to the query
func (qb *QueryBuilder) OrderBy(field string, direction string) *QueryBuilder {
	if direction != "ASC" && direction != "DESC" {
		direction = "DESC"
	}
	qb.orderBy = append(qb.orderBy, fmt.Sprintf("%s %s", field, direction))
	return qb
}

// Limit sets the result limit
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = limit
	return qb
}

// Offset sets the result offset
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.offset = offset
	return qb
}

// WithAggregation adds aggregation to the query
func (qb *QueryBuilder) WithAggregation(agg *AggregationQuery) *QueryBuilder {
	qb.aggregation = agg
	return qb
}

// BuildCountQuery builds a count query
func (qb *QueryBuilder) BuildCountQuery() (string, map[string]interface{}, error) {
	baseQuery := "SELECT COUNT(*) FROM audit_events"

	if len(qb.conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(qb.conditions, " AND ")
	}

	return baseQuery, qb.params, nil
}

// BuildSelectQuery builds a select query
func (qb *QueryBuilder) BuildSelectQuery() (string, map[string]interface{}, error) {
	if qb.aggregation != nil {
		return qb.buildAggregationQuery()
	}

	selectClause := "SELECT id, type, severity, actor_id, target_id, action, result, " +
		"timestamp, timestamp_nano, sequence_num, event_hash, previous_hash, " +
		"data_classes, legal_basis, compliance_flags, metadata"

	query := selectClause + " FROM audit_events"

	if len(qb.conditions) > 0 {
		query += " WHERE " + strings.Join(qb.conditions, " AND ")
	}

	if len(qb.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(qb.orderBy, ", ")
	} else {
		// Default ordering by timestamp descending
		query += " ORDER BY timestamp DESC"
	}

	if qb.limit > 0 {
		limitParam := fmt.Sprintf("$%d", len(qb.params)+1)
		qb.params["limit"] = qb.limit
		query += " LIMIT " + limitParam
	}

	if qb.offset > 0 {
		offsetParam := fmt.Sprintf("$%d", len(qb.params)+1)
		qb.params["offset"] = qb.offset
		query += " OFFSET " + offsetParam
	}

	return query, qb.params, nil
}

// buildAggregationQuery builds an aggregation query
func (qb *QueryBuilder) buildAggregationQuery() (string, map[string]interface{}, error) {
	if qb.aggregation == nil {
		return "", nil, errors.NewValidationError("MISSING_AGGREGATION", "aggregation query required")
	}

	var selectFields []string
	var groupByFields []string

	// Add group by fields
	for _, field := range qb.aggregation.GroupBy {
		selectFields = append(selectFields, field)
		groupByFields = append(groupByFields, field)
	}

	// Add aggregation functions
	for _, agg := range qb.aggregation.Aggregations {
		switch agg.Function {
		case "COUNT":
			if agg.Field == "*" {
				selectFields = append(selectFields, fmt.Sprintf("COUNT(*) as %s", agg.Alias))
			} else {
				selectFields = append(selectFields, fmt.Sprintf("COUNT(%s) as %s", agg.Field, agg.Alias))
			}
		case "SUM":
			selectFields = append(selectFields, fmt.Sprintf("SUM(%s) as %s", agg.Field, agg.Alias))
		case "AVG":
			selectFields = append(selectFields, fmt.Sprintf("AVG(%s) as %s", agg.Field, agg.Alias))
		case "MIN":
			selectFields = append(selectFields, fmt.Sprintf("MIN(%s) as %s", agg.Field, agg.Alias))
		case "MAX":
			selectFields = append(selectFields, fmt.Sprintf("MAX(%s) as %s", agg.Field, agg.Alias))
		case "DISTINCT_COUNT":
			selectFields = append(selectFields, fmt.Sprintf("COUNT(DISTINCT %s) as %s", agg.Field, agg.Alias))
		}
	}

	query := "SELECT " + strings.Join(selectFields, ", ") + " FROM audit_events"

	if len(qb.conditions) > 0 {
		query += " WHERE " + strings.Join(qb.conditions, " AND ")
	}

	if len(groupByFields) > 0 {
		query += " GROUP BY " + strings.Join(groupByFields, ", ")
	}

	// Add having clause if specified
	if qb.aggregation.Having != "" {
		query += " HAVING " + qb.aggregation.Having
	}

	if len(qb.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(qb.orderBy, ", ")
	}

	if qb.limit > 0 {
		limitParam := fmt.Sprintf("$%d", len(qb.params)+1)
		qb.params["limit"] = qb.limit
		query += " LIMIT " + limitParam
	}

	return query, qb.params, nil
}

// GetFilter returns the built filter
func (qb *QueryBuilder) GetFilter() *audit.EventFilter {
	return qb.filter
}

// Clone creates a copy of the query builder
func (qb *QueryBuilder) Clone() *QueryBuilder {
	clone := &QueryBuilder{
		filter:     &audit.EventFilter{},
		conditions: make([]string, len(qb.conditions)),
		params:     make(map[string]interface{}),
		orderBy:    make([]string, len(qb.orderBy)),
		limit:      qb.limit,
		offset:     qb.offset,
	}

	// Deep copy filter
	*clone.filter = *qb.filter

	// Copy conditions
	copy(clone.conditions, qb.conditions)

	// Copy params
	for k, v := range qb.params {
		clone.params[k] = v
	}

	// Copy order by
	copy(clone.orderBy, qb.orderBy)

	// Copy aggregation if present
	if qb.aggregation != nil {
		clone.aggregation = &AggregationQuery{}
		*clone.aggregation = *qb.aggregation
	}

	return clone
}

// Reset clears all filters and conditions
func (qb *QueryBuilder) Reset() *QueryBuilder {
	qb.filter = &audit.EventFilter{}
	qb.conditions = make([]string, 0)
	qb.params = make(map[string]interface{})
	qb.orderBy = make([]string, 0)
	qb.limit = 0
	qb.offset = 0
	qb.aggregation = nil
	return qb
}

// AggregationQuery represents an aggregation query
type AggregationQuery struct {
	GroupBy      []string          `json:"group_by"`
	Aggregations []AggregationFunc `json:"aggregations"`
	Having       string            `json:"having,omitempty"`
}

// AggregationFunc represents an aggregation function
type AggregationFunc struct {
	Function string `json:"function"` // COUNT, SUM, AVG, MIN, MAX, DISTINCT_COUNT
	Field    string `json:"field"`
	Alias    string `json:"alias"`
}

// ComplianceQueryBuilder provides compliance-specific query building
type ComplianceQueryBuilder struct {
	*QueryBuilder
}

// NewComplianceQueryBuilder creates a new compliance query builder
func NewComplianceQueryBuilder() *ComplianceQueryBuilder {
	return &ComplianceQueryBuilder{
		QueryBuilder: NewQueryBuilder(),
	}
}

// WithGDPRCriteria adds GDPR-specific filtering
func (cqb *ComplianceQueryBuilder) WithGDPRCriteria(dataSubjectID string, requestType string) *ComplianceQueryBuilder {
	// Filter by data subject
	cqb.WithActorIDs([]string{dataSubjectID}).WithTargetIDs([]string{dataSubjectID})

	// Add compliance flags for GDPR
	cqb.WithComplianceFlags(map[string]interface{}{
		"gdpr_data_subject": dataSubjectID,
		"gdpr_request_type": requestType,
	})

	// Include relevant data classes
	cqb.WithDataClasses([]string{"personal_data", "contact_data", "usage_data"})

	return cqb
}

// WithTCPACriteria adds TCPA-specific filtering
func (cqb *ComplianceQueryBuilder) WithTCPACriteria(phoneNumber string) *ComplianceQueryBuilder {
	// Filter by phone number as target
	cqb.WithTargetIDs([]string{phoneNumber})

	// Filter for consent-related events
	cqb.WithEventTypes([]audit.EventType{
		audit.EventConsentGranted,
		audit.EventConsentRevoked,
		audit.EventCallInitiated,
	})

	// Add TCPA compliance flags
	cqb.WithComplianceFlags(map[string]interface{}{
		"tcpa_phone_number": phoneNumber,
	})

	return cqb
}

// WithSOXCriteria adds SOX-specific filtering for financial data
func (cqb *ComplianceQueryBuilder) WithSOXCriteria(entityID string, transactionTypes []string) *ComplianceQueryBuilder {
	// Filter by entity (buyer/seller)
	cqb.WithActorIDs([]string{entityID}).WithTargetIDs([]string{entityID})

	// Filter for financial events
	cqb.WithEventTypes([]audit.EventType{
		audit.EventTransactionCreated,
		audit.EventTransactionProcessed,
		audit.EventPaymentProcessed,
	})

	// Add SOX compliance flags
	cqb.WithComplianceFlags(map[string]interface{}{
		"sox_entity":            entityID,
		"sox_transaction_types": transactionTypes,
	})

	// Include financial data classes
	cqb.WithDataClasses([]string{"financial_data", "transaction_data", "payment_data"})

	return cqb
}

// WithSecurityIncidentCriteria adds security-specific filtering
func (cqb *ComplianceQueryBuilder) WithSecurityIncidentCriteria(severityLevel string, incidentTypes []string) *ComplianceQueryBuilder {
	// Filter by security events
	cqb.WithEventTypes([]audit.EventType{
		audit.EventSecurityIncident,
		audit.EventUserLogin,
		audit.EventDataAccess,
		audit.EventSystemFailure,
	})

	// Filter by severity
	if severityLevel != "" {
		switch severityLevel {
		case "critical":
			cqb.WithSeverity([]audit.Severity{audit.SeverityCritical})
		case "high":
			cqb.WithSeverity([]audit.Severity{audit.SeverityHigh})
		case "medium":
			cqb.WithSeverity([]audit.Severity{audit.SeverityMedium})
		case "low":
			cqb.WithSeverity([]audit.Severity{audit.SeverityLow})
		}
	}

	// Add security compliance flags
	if len(incidentTypes) > 0 {
		for i, incidentType := range incidentTypes {
			cqb.WithComplianceFlags(map[string]interface{}{
				fmt.Sprintf("security_incident_type_%d", i): incidentType,
			})
		}
	}

	return cqb
}
