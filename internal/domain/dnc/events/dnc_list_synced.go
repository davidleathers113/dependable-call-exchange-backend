package events

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// SyncStatus represents the status of a sync operation
type SyncStatus string

const (
	SyncStatusSuccess    SyncStatus = "success"
	SyncStatusPartial    SyncStatus = "partial"
	SyncStatusFailed     SyncStatus = "failed"
	SyncStatusTimeout    SyncStatus = "timeout"
	SyncStatusCancelled  SyncStatus = "cancelled"
	SyncStatusSkipped    SyncStatus = "skipped"
)

// SyncTrigger represents what triggered the sync operation
type SyncTrigger string

const (
	SyncTriggerScheduled   SyncTrigger = "scheduled"
	SyncTriggerManual      SyncTrigger = "manual"
	SyncTriggerRetry       SyncTrigger = "retry"
	SyncTriggerWebhook     SyncTrigger = "webhook"
	SyncTriggerForced      SyncTrigger = "forced"
	SyncTriggerInitial     SyncTrigger = "initial"
	SyncTriggerIncremental SyncTrigger = "incremental"
)

// DNCListSyncedEvent represents when a DNC list synchronization occurs
type DNCListSyncedEvent struct {
	// Base event information
	EventID       uuid.UUID                    `json:"event_id"`
	EventType     audit.EventType              `json:"event_type"`
	EventVersion  string                       `json:"event_version"`
	Timestamp     time.Time                    `json:"timestamp"`
	
	// Sync operation fields
	Provider        string                     `json:"provider"`
	ProviderID      uuid.UUID                  `json:"provider_id"`
	SyncID          uuid.UUID                  `json:"sync_id"`
	SyncDuration    time.Duration              `json:"sync_duration"`
	SyncStatus      SyncStatus                 `json:"sync_status"`
	SyncTrigger     SyncTrigger                `json:"sync_trigger"`
	
	// Data metrics
	RecordsAdded    int64                      `json:"records_added"`
	RecordsRemoved  int64                      `json:"records_removed"`
	RecordsUpdated  int64                      `json:"records_updated"`
	RecordsSkipped  int64                      `json:"records_skipped"`
	RecordsTotal    int64                      `json:"records_total"`
	RecordsProcessed int64                     `json:"records_processed"`
	
	// Performance metrics
	ThroughputPerSecond  float64               `json:"throughput_per_second"`
	MemoryUsedMB        float64                `json:"memory_used_mb"`
	NetworkBytesIn      int64                  `json:"network_bytes_in"`
	NetworkBytesOut     int64                  `json:"network_bytes_out"`
	DatabaseConnections int                    `json:"database_connections"`
	
	// Sync details
	StartedAt       time.Time                  `json:"started_at"`
	CompletedAt     *time.Time                 `json:"completed_at,omitempty"`
	LastSyncAt      *time.Time                 `json:"last_sync_at,omitempty"`
	NextSyncAt      *time.Time                 `json:"next_sync_at,omitempty"`
	
	// Error handling
	ErrorCount      int                        `json:"error_count"`
	ErrorCode       *string                    `json:"error_code,omitempty"`
	ErrorMessage    *string                    `json:"error_message,omitempty"`
	WarningCount    int                        `json:"warning_count"`
	WarningMessages []string                   `json:"warning_messages,omitempty"`
	
	// Sync configuration
	SyncType        string                     `json:"sync_type"`        // full, incremental, delta
	BatchSize       int                        `json:"batch_size"`
	MaxRetries      int                        `json:"max_retries"`
	RetryCount      int                        `json:"retry_count"`
	TimeoutSeconds  int                        `json:"timeout_seconds"`
	
	// Data quality metrics
	DuplicatesFound     int64                  `json:"duplicates_found"`
	InvalidRecords      int64                  `json:"invalid_records"`
	DataQualityScore    float64                `json:"data_quality_score"`
	ConsistencyChecks   map[string]bool        `json:"consistency_checks,omitempty"`
	
	// Initiation context
	InitiatedBy     uuid.UUID                  `json:"initiated_by"`
	InitiatedFrom   string                     `json:"initiated_from"`   // system, api, ui, scheduler
	
	// Audit trail metadata
	RequestID       string                     `json:"request_id"`
	SessionID       string                     `json:"session_id,omitempty"`
	UserAgent       string                     `json:"user_agent,omitempty"`
	IPAddress       string                     `json:"ip_address,omitempty"`
	
	// System metadata
	ServerID        string                     `json:"server_id,omitempty"`
	Environment     string                     `json:"environment,omitempty"`
	WorkerID        string                     `json:"worker_id,omitempty"`
	QueueName       string                     `json:"queue_name,omitempty"`
	
	// Compliance metadata
	ComplianceFlags map[string]bool            `json:"compliance_flags,omitempty"`
	DataClasses     []string                   `json:"data_classes,omitempty"`
	LegalBasis      string                     `json:"legal_basis,omitempty"`
	
	// Additional metadata
	ProviderVersion string                     `json:"provider_version,omitempty"`
	SchemaVersion   string                     `json:"schema_version,omitempty"`
	ConfigChecksum  string                     `json:"config_checksum,omitempty"`
	DataChecksum    string                     `json:"data_checksum,omitempty"`
}

// NewDNCListSyncedEvent creates a new DNC list synced event
func NewDNCListSyncedEvent(
	provider string,
	providerID uuid.UUID,
	syncDuration time.Duration,
	status SyncStatus,
	trigger SyncTrigger,
	initiatedBy uuid.UUID,
) (*DNCListSyncedEvent, error) {
	// Validate provider
	if provider == "" {
		return nil, errors.NewValidationError("INVALID_PROVIDER", 
			"provider name cannot be empty")
	}
	
	// Validate provider ID
	if providerID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_PROVIDER_ID", 
			"provider ID cannot be empty")
	}
	
	// Validate initiated by user ID
	if initiatedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", 
			"initiated by user ID cannot be empty")
	}
	
	// Validate sync status
	if err := validateSyncStatus(status); err != nil {
		return nil, err
	}
	
	// Validate sync trigger
	if err := validateSyncTrigger(trigger); err != nil {
		return nil, err
	}
	
	now := time.Now().UTC()
	
	event := &DNCListSyncedEvent{
		EventID:             uuid.New(),
		EventType:           audit.EventDNCListSynced,
		EventVersion:        "1.0",
		Timestamp:           now,
		Provider:            provider,
		ProviderID:          providerID,
		SyncID:              uuid.New(),
		SyncDuration:        syncDuration,
		SyncStatus:          status,
		SyncTrigger:         trigger,
		StartedAt:           now.Add(-syncDuration),
		InitiatedBy:         initiatedBy,
		InitiatedFrom:       "system",
		SyncType:            "incremental",
		BatchSize:           1000,
		MaxRetries:          3,
		TimeoutSeconds:      300,
		DataQualityScore:    1.0,
		ComplianceFlags:     make(map[string]bool),
		DataClasses:         []string{"phone_number", "dnc_list"},
		LegalBasis:          "legitimate_interest",
		WarningMessages:     make([]string, 0),
		ConsistencyChecks:   make(map[string]bool),
	}
	
	// Set completed time for successful syncs
	if status == SyncStatusSuccess || status == SyncStatusPartial {
		event.CompletedAt = &now
	}
	
	// Set compliance flags
	event.setComplianceFlags()
	
	// Calculate derived metrics
	event.updateDerivedMetrics()
	
	return event, nil
}

// GetEventType returns the event type
func (e *DNCListSyncedEvent) GetEventType() audit.EventType {
	return audit.EventDNCListSynced
}

// GetEventVersion returns the event version
func (e *DNCListSyncedEvent) GetEventVersion() string {
	return e.EventVersion
}

// GetEventID returns the event ID
func (e *DNCListSyncedEvent) GetEventID() uuid.UUID {
	return e.EventID
}

// GetTimestamp returns the event timestamp
func (e *DNCListSyncedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetAggregateID returns the provider ID as the aggregate ID
func (e *DNCListSyncedEvent) GetAggregateID() string {
	return e.ProviderID.String()
}

// GetAggregateType returns the aggregate type
func (e *DNCListSyncedEvent) GetAggregateType() string {
	return "dnc_provider"
}

// SetRecordMetrics sets detailed record processing metrics
func (e *DNCListSyncedEvent) SetRecordMetrics(added, removed, updated, skipped, total, processed int64) {
	e.RecordsAdded = added
	e.RecordsRemoved = removed
	e.RecordsUpdated = updated
	e.RecordsSkipped = skipped
	e.RecordsTotal = total
	e.RecordsProcessed = processed
	e.updateDerivedMetrics()
}

// SetPerformanceMetrics sets detailed performance metrics
func (e *DNCListSyncedEvent) SetPerformanceMetrics(throughput, memoryMB float64, bytesIn, bytesOut int64, dbConnections int) {
	e.ThroughputPerSecond = throughput
	e.MemoryUsedMB = memoryMB
	e.NetworkBytesIn = bytesIn
	e.NetworkBytesOut = bytesOut
	e.DatabaseConnections = dbConnections
}

// SetDataQualityMetrics sets data quality metrics
func (e *DNCListSyncedEvent) SetDataQualityMetrics(duplicates, invalid int64, qualityScore float64) {
	e.DuplicatesFound = duplicates
	e.InvalidRecords = invalid
	e.DataQualityScore = qualityScore
}

// SetSyncConfiguration sets sync configuration details
func (e *DNCListSyncedEvent) SetSyncConfiguration(syncType string, batchSize, maxRetries, timeoutSeconds int) {
	e.SyncType = syncType
	e.BatchSize = batchSize
	e.MaxRetries = maxRetries
	e.TimeoutSeconds = timeoutSeconds
}

// SetSyncTiming sets sync timing information
func (e *DNCListSyncedEvent) SetSyncTiming(startedAt time.Time, completedAt, lastSyncAt, nextSyncAt *time.Time) {
	e.StartedAt = startedAt
	e.CompletedAt = completedAt
	e.LastSyncAt = lastSyncAt
	e.NextSyncAt = nextSyncAt
	
	// Recalculate duration if we have both times
	if completedAt != nil {
		e.SyncDuration = completedAt.Sub(startedAt)
	}
}

// SetError sets error information for failed syncs
func (e *DNCListSyncedEvent) SetError(code, message string, errorCount int) {
	e.ErrorCode = &code
	e.ErrorMessage = &message
	e.ErrorCount = errorCount
	if e.SyncStatus == SyncStatusSuccess {
		e.SyncStatus = SyncStatusFailed
	}
}

// AddWarning adds a warning message
func (e *DNCListSyncedEvent) AddWarning(message string) {
	e.WarningMessages = append(e.WarningMessages, message)
	e.WarningCount = len(e.WarningMessages)
}

// SetRetryInfo sets retry-related information
func (e *DNCListSyncedEvent) SetRetryInfo(retryCount int) {
	e.RetryCount = retryCount
	if retryCount > 0 {
		e.SyncTrigger = SyncTriggerRetry
	}
}

// SetConsistencyCheck sets a consistency check result
func (e *DNCListSyncedEvent) SetConsistencyCheck(check string, passed bool) {
	if e.ConsistencyChecks == nil {
		e.ConsistencyChecks = make(map[string]bool)
	}
	e.ConsistencyChecks[check] = passed
}

// SetRequestContext sets the request context information
func (e *DNCListSyncedEvent) SetRequestContext(requestID, sessionID, userAgent, ipAddress string) {
	e.RequestID = requestID
	if sessionID != "" {
		e.SessionID = sessionID
	}
	if userAgent != "" {
		e.UserAgent = userAgent
	}
	if ipAddress != "" {
		e.IPAddress = ipAddress
	}
}

// SetSystemContext sets system-specific context
func (e *DNCListSyncedEvent) SetSystemContext(serverID, environment, workerID, queueName string) {
	e.ServerID = serverID
	e.Environment = environment
	e.WorkerID = workerID
	e.QueueName = queueName
}

// SetVersionInfo sets version and checksum information
func (e *DNCListSyncedEvent) SetVersionInfo(providerVersion, schemaVersion, configChecksum, dataChecksum string) {
	e.ProviderVersion = providerVersion
	e.SchemaVersion = schemaVersion
	e.ConfigChecksum = configChecksum
	e.DataChecksum = dataChecksum
}

// IsSuccessful returns true if the sync completed successfully
func (e *DNCListSyncedEvent) IsSuccessful() bool {
	return e.SyncStatus == SyncStatusSuccess
}

// IsPartial returns true if the sync completed partially
func (e *DNCListSyncedEvent) IsPartial() bool {
	return e.SyncStatus == SyncStatusPartial
}

// IsFailed returns true if the sync failed
func (e *DNCListSyncedEvent) IsFailed() bool {
	return e.SyncStatus == SyncStatusFailed
}

// IsTimeout returns true if the sync timed out
func (e *DNCListSyncedEvent) IsTimeout() bool {
	return e.SyncStatus == SyncStatusTimeout
}

// IsCancelled returns true if the sync was cancelled
func (e *DNCListSyncedEvent) IsCancelled() bool {
	return e.SyncStatus == SyncStatusCancelled
}

// IsRetry returns true if this sync was a retry
func (e *DNCListSyncedEvent) IsRetry() bool {
	return e.SyncTrigger == SyncTriggerRetry || e.RetryCount > 0
}

// IsScheduled returns true if this sync was scheduled
func (e *DNCListSyncedEvent) IsScheduled() bool {
	return e.SyncTrigger == SyncTriggerScheduled
}

// IsManual returns true if this sync was manually triggered
func (e *DNCListSyncedEvent) IsManual() bool {
	return e.SyncTrigger == SyncTriggerManual
}

// IsFullSync returns true if this was a full sync
func (e *DNCListSyncedEvent) IsFullSync() bool {
	return e.SyncType == "full"
}

// IsIncrementalSync returns true if this was an incremental sync
func (e *DNCListSyncedEvent) IsIncrementalSync() bool {
	return e.SyncType == "incremental"
}

// HasErrors returns true if there were errors during sync
func (e *DNCListSyncedEvent) HasErrors() bool {
	return e.ErrorCount > 0 || e.ErrorCode != nil
}

// HasWarnings returns true if there are warning messages
func (e *DNCListSyncedEvent) HasWarnings() bool {
	return e.WarningCount > 0 || len(e.WarningMessages) > 0
}

// GetProcessingRate returns the percentage of records processed
func (e *DNCListSyncedEvent) GetProcessingRate() float64 {
	if e.RecordsTotal == 0 {
		return 0.0
	}
	return float64(e.RecordsProcessed) / float64(e.RecordsTotal) * 100.0
}

// GetErrorRate returns the error rate as a percentage
func (e *DNCListSyncedEvent) GetErrorRate() float64 {
	if e.RecordsProcessed == 0 {
		return 0.0
	}
	return float64(e.ErrorCount) / float64(e.RecordsProcessed) * 100.0
}

// GetNetDataChange returns the net change in data (added - removed)
func (e *DNCListSyncedEvent) GetNetDataChange() int64 {
	return e.RecordsAdded - e.RecordsRemoved
}

// GetTotalDataChanges returns the total number of data changes
func (e *DNCListSyncedEvent) GetTotalDataChanges() int64 {
	return e.RecordsAdded + e.RecordsRemoved + e.RecordsUpdated
}

// GetEfficiencyScore returns a performance efficiency score (0.0-1.0)
func (e *DNCListSyncedEvent) GetEfficiencyScore() float64 {
	baseScore := 1.0
	
	// Reduce score for errors
	if e.ErrorCount > 0 {
		baseScore -= float64(e.ErrorCount) / float64(e.RecordsProcessed) * 0.5
	}
	
	// Reduce score for warnings
	if e.WarningCount > 0 {
		baseScore -= float64(e.WarningCount) / float64(e.RecordsProcessed) * 0.1
	}
	
	// Factor in data quality score
	baseScore = (baseScore + e.DataQualityScore) / 2.0
	
	// Ensure score is between 0.0 and 1.0
	if baseScore < 0.0 {
		baseScore = 0.0
	}
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	
	return baseScore
}

// GetPerformanceMetrics returns performance-related metadata
func (e *DNCListSyncedEvent) GetPerformanceMetrics() map[string]interface{} {
	return map[string]interface{}{
		"sync_duration_ms":      e.SyncDuration.Milliseconds(),
		"throughput_per_second": e.ThroughputPerSecond,
		"memory_used_mb":        e.MemoryUsedMB,
		"network_bytes_in":      e.NetworkBytesIn,
		"network_bytes_out":     e.NetworkBytesOut,
		"database_connections":  e.DatabaseConnections,
		"records_processed":     e.RecordsProcessed,
		"records_total":         e.RecordsTotal,
		"processing_rate_pct":   e.GetProcessingRate(),
		"error_rate_pct":        e.GetErrorRate(),
		"efficiency_score":      e.GetEfficiencyScore(),
		"data_quality_score":    e.DataQualityScore,
		"batch_size":           e.BatchSize,
	}
}

// GetComplianceMetadata returns compliance-specific metadata
func (e *DNCListSyncedEvent) GetComplianceMetadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"provider":             e.Provider,
		"provider_id":          e.ProviderID.String(),
		"sync_status":          string(e.SyncStatus),
		"sync_trigger":         string(e.SyncTrigger),
		"sync_type":            e.SyncType,
		"initiated_by":         e.InitiatedBy.String(),
		"initiated_from":       e.InitiatedFrom,
		"legal_basis":          e.LegalBasis,
		"data_classes":         e.DataClasses,
		"records_added":        e.RecordsAdded,
		"records_removed":      e.RecordsRemoved,
		"records_updated":      e.RecordsUpdated,
		"records_total":        e.RecordsTotal,
		"net_data_change":      e.GetNetDataChange(),
		"total_data_changes":   e.GetTotalDataChanges(),
		"has_errors":           e.HasErrors(),
		"has_warnings":         e.HasWarnings(),
		"is_retry":             e.IsRetry(),
		"data_quality_score":   e.DataQualityScore,
		"duplicates_found":     e.DuplicatesFound,
		"invalid_records":      e.InvalidRecords,
	}
	
	if e.ErrorCode != nil {
		metadata["error_code"] = *e.ErrorCode
	}
	
	if e.ErrorMessage != nil {
		metadata["error_message"] = *e.ErrorMessage
	}
	
	if len(e.WarningMessages) > 0 {
		metadata["warnings"] = e.WarningMessages
	}
	
	if e.LastSyncAt != nil {
		metadata["last_sync_at"] = *e.LastSyncAt
	}
	
	if e.NextSyncAt != nil {
		metadata["next_sync_at"] = *e.NextSyncAt
	}
	
	if e.ProviderVersion != "" {
		metadata["provider_version"] = e.ProviderVersion
	}
	
	if e.SchemaVersion != "" {
		metadata["schema_version"] = e.SchemaVersion
	}
	
	if len(e.ConsistencyChecks) > 0 {
		metadata["consistency_checks"] = e.ConsistencyChecks
	}
	
	return metadata
}

// ToAuditEvent converts the domain event to an audit event
func (e *DNCListSyncedEvent) ToAuditEvent() (*audit.Event, error) {
	auditEvent, err := audit.NewEvent(
		e.EventType,
		e.InitiatedBy.String(),
		e.ProviderID.String(),
		"sync_dnc_list",
	)
	if err != nil {
		return nil, err
	}
	
	// Set result based on sync outcome
	result := "success"
	if e.IsFailed() {
		result = "error"
	} else if e.IsTimeout() {
		result = "timeout"
	} else if e.IsCancelled() {
		result = "cancelled"
	} else if e.IsPartial() {
		result = "partial"
	}
	
	// Set additional audit event fields
	auditEvent.TargetType = "dnc_provider"
	auditEvent.Result = result
	auditEvent.RequestID = e.RequestID
	auditEvent.SessionID = e.SessionID
	auditEvent.ActorAgent = e.UserAgent
	auditEvent.ActorIP = e.IPAddress
	auditEvent.LegalBasis = e.LegalBasis
	auditEvent.DataClasses = e.DataClasses
	auditEvent.ComplianceFlags = e.ComplianceFlags
	
	// Add DNC-specific metadata
	auditEvent.Metadata = map[string]interface{}{
		"provider":             e.Provider,
		"sync_id":              e.SyncID.String(),
		"sync_status":          string(e.SyncStatus),
		"sync_trigger":         string(e.SyncTrigger),
		"sync_type":            e.SyncType,
		"sync_duration_ms":     e.SyncDuration.Milliseconds(),
		"initiated_from":       e.InitiatedFrom,
		"records_added":        e.RecordsAdded,
		"records_removed":      e.RecordsRemoved,
		"records_updated":      e.RecordsUpdated,
		"records_skipped":      e.RecordsSkipped,
		"records_total":        e.RecordsTotal,
		"records_processed":    e.RecordsProcessed,
		"net_data_change":      e.GetNetDataChange(),
		"total_data_changes":   e.GetTotalDataChanges(),
		"throughput_per_second": e.ThroughputPerSecond,
		"memory_used_mb":       e.MemoryUsedMB,
		"network_bytes_in":     e.NetworkBytesIn,
		"network_bytes_out":    e.NetworkBytesOut,
		"database_connections": e.DatabaseConnections,
		"batch_size":           e.BatchSize,
		"retry_count":          e.RetryCount,
		"error_count":          e.ErrorCount,
		"warning_count":        e.WarningCount,
		"data_quality_score":   e.DataQualityScore,
		"duplicates_found":     e.DuplicatesFound,
		"invalid_records":      e.InvalidRecords,
		"processing_rate_pct":  e.GetProcessingRate(),
		"error_rate_pct":       e.GetErrorRate(),
		"efficiency_score":     e.GetEfficiencyScore(),
	}
	
	if e.ErrorCode != nil {
		auditEvent.Metadata["error_code"] = *e.ErrorCode
	}
	
	if e.ErrorMessage != nil {
		auditEvent.Metadata["error_message"] = *e.ErrorMessage
	}
	
	if len(e.WarningMessages) > 0 {
		auditEvent.Metadata["warnings"] = e.WarningMessages
	}
	
	if e.CompletedAt != nil {
		auditEvent.Metadata["completed_at"] = *e.CompletedAt
	}
	
	if e.LastSyncAt != nil {
		auditEvent.Metadata["last_sync_at"] = *e.LastSyncAt
	}
	
	if e.NextSyncAt != nil {
		auditEvent.Metadata["next_sync_at"] = *e.NextSyncAt
	}
	
	if e.ProviderVersion != "" {
		auditEvent.Metadata["provider_version"] = e.ProviderVersion
	}
	
	if e.DataChecksum != "" {
		auditEvent.Metadata["data_checksum"] = e.DataChecksum
	}
	
	if len(e.ConsistencyChecks) > 0 {
		auditEvent.Metadata["consistency_checks"] = e.ConsistencyChecks
	}
	
	return auditEvent, nil
}

// setComplianceFlags sets compliance flags based on sync context
func (e *DNCListSyncedEvent) setComplianceFlags() {
	e.ComplianceFlags["dnc_sync"] = true
	e.ComplianceFlags["contains_pii"] = true
	e.ComplianceFlags["data_processing"] = true
	
	// Set flags based on sync trigger
	if e.IsScheduled() {
		e.ComplianceFlags["scheduled_operation"] = true
	}
	
	if e.IsManual() {
		e.ComplianceFlags["manual_operation"] = true
	}
	
	// Set flags based on sync type
	if e.IsFullSync() {
		e.ComplianceFlags["full_sync"] = true
		e.ComplianceFlags["bulk_processing"] = true
	}
	
	if e.IsIncrementalSync() {
		e.ComplianceFlags["incremental_sync"] = true
	}
	
	// Set flags based on data changes
	if e.RecordsAdded > 0 {
		e.ComplianceFlags["data_added"] = true
	}
	
	if e.RecordsRemoved > 0 {
		e.ComplianceFlags["data_removed"] = true
		e.ComplianceFlags["data_deletion"] = true
	}
	
	if e.RecordsUpdated > 0 {
		e.ComplianceFlags["data_updated"] = true
		e.ComplianceFlags["data_modification"] = true
	}
	
	// Set flags based on results
	if e.HasErrors() {
		e.ComplianceFlags["sync_errors"] = true
	}
	
	if e.HasWarnings() {
		e.ComplianceFlags["sync_warnings"] = true
	}
	
	if e.IsRetry() {
		e.ComplianceFlags["retry_operation"] = true
	}
	
	// Set data quality flags
	if e.DataQualityScore < 0.8 {
		e.ComplianceFlags["low_data_quality"] = true
	}
	
	if e.DuplicatesFound > 0 {
		e.ComplianceFlags["duplicates_detected"] = true
	}
	
	if e.InvalidRecords > 0 {
		e.ComplianceFlags["invalid_data"] = true
	}
}

// updateDerivedMetrics updates calculated metrics
func (e *DNCListSyncedEvent) updateDerivedMetrics() {
	// Update throughput if we have duration and processed records
	if e.SyncDuration > 0 && e.RecordsProcessed > 0 {
		seconds := e.SyncDuration.Seconds()
		e.ThroughputPerSecond = float64(e.RecordsProcessed) / seconds
	}
	
	// Update warning count to match warning messages
	e.WarningCount = len(e.WarningMessages)
	
	// Ensure processed count doesn't exceed total
	if e.RecordsProcessed > e.RecordsTotal && e.RecordsTotal > 0 {
		e.RecordsProcessed = e.RecordsTotal
	}
}

// validateSyncStatus validates the sync status
func validateSyncStatus(status SyncStatus) error {
	switch status {
	case SyncStatusSuccess, SyncStatusPartial, SyncStatusFailed,
		SyncStatusTimeout, SyncStatusCancelled, SyncStatusSkipped:
		return nil
	default:
		return errors.NewValidationError("INVALID_SYNC_STATUS", 
			"invalid sync status: "+string(status))
	}
}

// validateSyncTrigger validates the sync trigger
func validateSyncTrigger(trigger SyncTrigger) error {
	switch trigger {
	case SyncTriggerScheduled, SyncTriggerManual, SyncTriggerRetry,
		SyncTriggerWebhook, SyncTriggerForced, SyncTriggerInitial,
		SyncTriggerIncremental:
		return nil
	default:
		return errors.NewValidationError("INVALID_SYNC_TRIGGER", 
			"invalid sync trigger: "+string(trigger))
	}
}

// Validate performs validation of the event
func (e *DNCListSyncedEvent) Validate() error {
	if e.EventID == uuid.Nil {
		return errors.NewValidationError("MISSING_EVENT_ID", "event ID is required")
	}
	
	if e.EventType != audit.EventDNCListSynced {
		return errors.NewValidationError("INVALID_EVENT_TYPE", "invalid event type")
	}
	
	if e.EventVersion == "" {
		return errors.NewValidationError("MISSING_EVENT_VERSION", "event version is required")
	}
	
	if e.Provider == "" {
		return errors.NewValidationError("MISSING_PROVIDER", "provider name is required")
	}
	
	if e.ProviderID == uuid.Nil {
		return errors.NewValidationError("MISSING_PROVIDER_ID", "provider ID is required")
	}
	
	if e.InitiatedBy == uuid.Nil {
		return errors.NewValidationError("MISSING_INITIATED_BY", "initiated by user ID is required")
	}
	
	if e.RequestID == "" {
		return errors.NewValidationError("MISSING_REQUEST_ID", "request ID is required")
	}
	
	// Validate sync status
	if err := validateSyncStatus(e.SyncStatus); err != nil {
		return err
	}
	
	// Validate sync trigger
	if err := validateSyncTrigger(e.SyncTrigger); err != nil {
		return err
	}
	
	// Validate record counts are non-negative
	if e.RecordsAdded < 0 || e.RecordsRemoved < 0 || e.RecordsUpdated < 0 || 
		e.RecordsSkipped < 0 || e.RecordsTotal < 0 || e.RecordsProcessed < 0 {
		return errors.NewValidationError("INVALID_RECORD_COUNTS", 
			"record counts cannot be negative")
	}
	
	// Validate data quality score
	if e.DataQualityScore < 0.0 || e.DataQualityScore > 1.0 {
		return errors.NewValidationError("INVALID_DATA_QUALITY_SCORE", 
			"data quality score must be between 0.0 and 1.0")
	}
	
	// Validate timing consistency
	if e.CompletedAt != nil && e.CompletedAt.Before(e.StartedAt) {
		return errors.NewValidationError("INVALID_TIMING", 
			"completed time cannot be before started time")
	}
	
	return nil
}