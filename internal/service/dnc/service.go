package dnc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/services"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Ensure service implements the interface
var _ Service = (*service)(nil)

// service implements the DNC orchestration service
// It coordinates between domain services, repositories, and infrastructure
// while maintaining sub-10ms latency for DNC checks
type service struct {
	// Core dependencies (max 5 per service pattern)
	logger            *zap.Logger
	config            *Config
	
	// Repositories
	entryRepo         DNCEntryRepository
	providerRepo      DNCProviderRepository
	checkResultRepo   DNCCheckResultRepository
	
	// Infrastructure
	cache             DNCCache
	eventPublisher    EventPublisher
	circuitBreaker    CircuitBreaker
	
	// Domain services
	complianceService ComplianceService
	riskService       RiskAssessmentService
	conflictResolver  ConflictResolver
	
	// External services
	timeZoneService   TimeZoneService
	callHistoryService CallHistoryService
	auditService      AuditService
	
	// External API clients
	federalDNCClient  FederalDNCClient
	stateDNCClient    StateDNCClient
	
	// Performance monitoring
	metrics           *serviceMetrics
	
	// Synchronization
	syncMutex         sync.RWMutex
	providerSyncStatus map[uuid.UUID]*ProviderSyncStatus
}

// ProviderSyncStatus tracks sync status for each provider
type ProviderSyncStatus struct {
	InProgress  bool
	LastSync    time.Time
	NextSync    time.Time
	ErrorCount  int
	LastError   string
}

// serviceMetrics tracks service performance metrics
type serviceMetrics struct {
	totalChecks       int64
	cacheHits         int64
	cacheMisses       int64
	averageLatency    time.Duration
	errorCount        int64
	slowQueries       int64
	providerErrors    map[uuid.UUID]int64
	mutex             sync.RWMutex
}

// NewService creates a new DNC orchestration service
func NewService(
	logger *zap.Logger,
	config *Config,
	entryRepo DNCEntryRepository,
	providerRepo DNCProviderRepository,
	checkResultRepo DNCCheckResultRepository,
	cache DNCCache,
	eventPublisher EventPublisher,
) (Service, error) {
	if logger == nil {
		return nil, errors.NewValidationError("INVALID_LOGGER", "logger cannot be nil")
	}
	if config == nil {
		return nil, errors.NewValidationError("INVALID_CONFIG", "config cannot be nil")
	}
	if entryRepo == nil {
		return nil, errors.NewValidationError("INVALID_ENTRY_REPO", "entry repository cannot be nil")
	}
	if providerRepo == nil {
		return nil, errors.NewValidationError("INVALID_PROVIDER_REPO", "provider repository cannot be nil")
	}
	if checkResultRepo == nil {
		return nil, errors.NewValidationError("INVALID_CHECK_RESULT_REPO", "check result repository cannot be nil")
	}

	svc := &service{
		logger:            logger,
		config:            config,
		entryRepo:         entryRepo,
		providerRepo:      providerRepo,
		checkResultRepo:   checkResultRepo,
		cache:             cache,
		eventPublisher:    eventPublisher,
		providerSyncStatus: make(map[uuid.UUID]*ProviderSyncStatus),
		metrics: &serviceMetrics{
			providerErrors: make(map[uuid.UUID]int64),
		},
	}

	return svc, nil
}

// SetDomainServices sets the domain services (dependency injection)
func (s *service) SetDomainServices(
	complianceService ComplianceService,
	riskService RiskAssessmentService,
	conflictResolver ConflictResolver,
) {
	s.complianceService = complianceService
	s.riskService = riskService
	s.conflictResolver = conflictResolver
}

// SetExternalServices sets external services (dependency injection)
func (s *service) SetExternalServices(
	timeZoneService TimeZoneService,
	callHistoryService CallHistoryService,
	auditService AuditService,
	circuitBreaker CircuitBreaker,
) {
	s.timeZoneService = timeZoneService
	s.callHistoryService = callHistoryService
	s.auditService = auditService
	s.circuitBreaker = circuitBreaker
}

// SetAPIClients sets external API clients (dependency injection)
func (s *service) SetAPIClients(
	federalDNCClient FederalDNCClient,
	stateDNCClient StateDNCClient,
) {
	s.federalDNCClient = federalDNCClient
	s.stateDNCClient = stateDNCClient
}

// CheckDNC performs a comprehensive DNC check with sub-10ms latency
func (s *service) CheckDNC(ctx context.Context, phoneNumber *values.PhoneNumber, callTime time.Time) (*DNCCheckResponse, error) {
	startTime := time.Now()
	defer func() {
		s.recordMetrics(time.Since(startTime), 1, 0)
	}()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.CheckTimeoutMs)*time.Millisecond)
	defer cancel()

	// Check cache first for sub-ms response
	if s.cache != nil {
		if cachedResult, err := s.cache.GetCheckResult(timeoutCtx, phoneNumber); err == nil && cachedResult != nil {
			if !cachedResult.IsExpired() {
				s.recordCacheHit()
				return s.convertCheckResultToResponse(cachedResult, true), nil
			}
		}
	}

	s.recordCacheMiss()

	// Perform fresh DNC check
	checkResult, err := s.performDNCCheck(timeoutCtx, phoneNumber, callTime)
	if err != nil {
		s.recordError()
		return nil, errors.NewInternalError("DNC check failed").WithCause(err)
	}

	// Cache the result asynchronously
	if s.cache != nil {
		go func() {
			cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cacheCancel()
			if err := s.cache.SetCheckResult(cacheCtx, checkResult); err != nil {
				s.logger.Warn("Failed to cache DNC check result", 
					zap.Error(err),
					zap.String("phone_number", phoneNumber.String()))
			}
		}()
	}

	// Publish audit event asynchronously
	if s.eventPublisher != nil && s.auditService != nil {
		go s.publishAuditEvent(ctx, phoneNumber, checkResult)
	}

	response := s.convertCheckResultToResponse(checkResult, false)
	return response, nil
}

// CheckDNCBulk performs DNC checks for multiple phone numbers efficiently
func (s *service) CheckDNCBulk(ctx context.Context, phoneNumbers []*values.PhoneNumber, callTime time.Time) ([]*DNCCheckResponse, error) {
	if len(phoneNumbers) == 0 {
		return []*DNCCheckResponse{}, nil
	}

	startTime := time.Now()
	defer func() {
		s.recordMetrics(time.Since(startTime), len(phoneNumbers), 0)
	}()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.BulkCheckTimeoutMs)*time.Millisecond)
	defer cancel()

	// Separate cached and uncached numbers
	responses := make([]*DNCCheckResponse, len(phoneNumbers))
	uncachedIndexes := make([]int, 0, len(phoneNumbers))
	uncachedNumbers := make([]*values.PhoneNumber, 0, len(phoneNumbers))

	// Check cache for each number
	if s.cache != nil {
		for i, phoneNumber := range phoneNumbers {
			if cachedResult, err := s.cache.GetCheckResult(timeoutCtx, phoneNumber); err == nil && cachedResult != nil {
				if !cachedResult.IsExpired() {
					responses[i] = s.convertCheckResultToResponse(cachedResult, true)
					s.recordCacheHit()
					continue
				}
			}
			
			uncachedIndexes = append(uncachedIndexes, i)
			uncachedNumbers = append(uncachedNumbers, phoneNumber)
			s.recordCacheMiss()
		}
	} else {
		// No cache, all numbers are uncached
		for i, phoneNumber := range phoneNumbers {
			uncachedIndexes = append(uncachedIndexes, i)
			uncachedNumbers = append(uncachedNumbers, phoneNumber)
		}
	}

	// Process uncached numbers in parallel
	if len(uncachedNumbers) > 0 {
		uncachedResults, err := s.performBulkDNCCheck(timeoutCtx, uncachedNumbers, callTime)
		if err != nil {
			s.recordError()
			return nil, errors.NewInternalError("Bulk DNC check failed").WithCause(err)
		}

		// Map results back to original positions
		for i, result := range uncachedResults {
			originalIndex := uncachedIndexes[i]
			responses[originalIndex] = s.convertCheckResultToResponse(result, false)
		}

		// Cache results asynchronously
		if s.cache != nil {
			go func() {
				cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cacheCancel()
				for _, result := range uncachedResults {
					if err := s.cache.SetCheckResult(cacheCtx, result); err != nil {
						s.logger.Warn("Failed to cache bulk DNC check result", zap.Error(err))
					}
				}
			}()
		}
	}

	return responses, nil
}

// AddToSuppressionList adds a phone number to internal suppression
func (s *service) AddToSuppressionList(ctx context.Context, req AddSuppressionRequest) (*SuppressionResponse, error) {
	// Validate request
	if req.PhoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "phone number is required")
	}
	if req.AddedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", "added_by user ID is required")
	}

	// Create DNC entry
	entry, err := dnc.NewDNCEntry(
		req.PhoneNumber,
		req.ListSource,
		req.SuppressReason,
		req.ExpiresAt,
		req.AddedBy,
		req.Notes,
		req.Metadata,
	)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_ENTRY", "failed to create DNC entry").WithCause(err)
	}

	// Save to repository
	if err := s.entryRepo.Save(ctx, entry); err != nil {
		return nil, errors.NewInternalError("Failed to save suppression entry").WithCause(err)
	}

	// Invalidate cache
	if s.cache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			// Invalidate specific phone number and source patterns
			s.cache.InvalidateSource(cacheCtx, req.ListSource)
		}()
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &dnc.NumberSuppressedEvent{
			PhoneNumber:    req.PhoneNumber,
			ListSource:     req.ListSource,
			SuppressReason: req.SuppressReason,
			AddedBy:        req.AddedBy,
			AddedAt:        entry.AddedAt,
			ExpiresAt:      req.ExpiresAt,
		}
		
		go func() {
			eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.eventPublisher.PublishNumberSuppressed(eventCtx, event); err != nil {
				s.logger.Error("Failed to publish number suppressed event", zap.Error(err))
			}
		}()
	}

	// Audit log
	if s.auditService != nil {
		go func() {
			auditCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			auditReq := SuppressionAuditRequest{
				Action:      "ADD",
				EntryID:     entry.ID,
				PhoneNumber: req.PhoneNumber,
				After:       s.convertEntryToResponse(entry),
				UserID:      req.AddedBy,
				Reason:      fmt.Sprintf("Added to suppression: %s", req.SuppressReason.String()),
				Timestamp:   time.Now(),
			}
			if err := s.auditService.LogSuppressionChange(auditCtx, auditReq); err != nil {
				s.logger.Error("Failed to log suppression audit", zap.Error(err))
			}
		}()
	}

	return s.convertEntryToResponse(entry), nil
}

// RemoveFromSuppressionList removes a phone number from internal suppression
func (s *service) RemoveFromSuppressionList(ctx context.Context, phoneNumber *values.PhoneNumber, removedBy uuid.UUID, reason string) error {
	if phoneNumber == nil {
		return errors.NewValidationError("INVALID_PHONE_NUMBER", "phone number is required")
	}
	if removedBy == uuid.Nil {
		return errors.NewValidationError("INVALID_USER_ID", "removed_by user ID is required")
	}

	// Find existing entries
	entries, err := s.entryRepo.FindByPhone(ctx, phoneNumber)
	if err != nil {
		return errors.NewInternalError("Failed to find suppression entries").WithCause(err)
	}

	if len(entries) == 0 {
		return errors.NewNotFoundError("ENTRY_NOT_FOUND", "no suppression entries found for phone number")
	}

	// Remove active internal entries
	var removedEntries []*dnc.DNCEntry
	for _, entry := range entries {
		if entry.IsActive() && entry.ListSource == values.ListSourceInternal {
			if err := s.entryRepo.Delete(ctx, entry.ID, removedBy); err != nil {
				s.logger.Error("Failed to delete suppression entry", 
					zap.Error(err), 
					zap.String("entry_id", entry.ID.String()))
				continue
			}
			removedEntries = append(removedEntries, entry)
		}
	}

	if len(removedEntries) == 0 {
		return errors.NewNotFoundError("NO_ACTIVE_ENTRIES", "no active internal suppression entries found")
	}

	// Invalidate cache
	if s.cache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			s.cache.InvalidateSource(cacheCtx, values.ListSourceInternal)
		}()
	}

	// Publish events
	if s.eventPublisher != nil {
		for _, entry := range removedEntries {
			event := &dnc.NumberReleasedEvent{
				PhoneNumber:    phoneNumber,
				ListSource:     entry.ListSource,
				SuppressReason: entry.SuppressReason,
				RemovedBy:      removedBy,
				RemovedAt:      time.Now(),
				OriginallyAddedAt: entry.AddedAt,
			}
			
			go func(e *dnc.NumberReleasedEvent) {
				eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := s.eventPublisher.PublishNumberReleased(eventCtx, e); err != nil {
					s.logger.Error("Failed to publish number released event", zap.Error(err))
				}
			}(event)
		}
	}

	return nil
}

// UpdateSuppressionEntry updates an existing suppression entry
func (s *service) UpdateSuppressionEntry(ctx context.Context, req UpdateSuppressionRequest) (*SuppressionResponse, error) {
	if req.ID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_ID", "entry ID is required")
	}
	if req.UpdatedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", "updated_by user ID is required")
	}

	// Get existing entry
	entry, err := s.entryRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get suppression entry").WithCause(err)
	}

	// Store original for audit
	originalEntry := *entry

	// Update fields
	updated := false
	if req.SuppressReason != "" && req.SuppressReason != entry.SuppressReason {
		entry.SuppressReason = req.SuppressReason
		updated = true
	}
	if req.ExpiresAt != nil && (entry.ExpiresAt == nil || !req.ExpiresAt.Equal(*entry.ExpiresAt)) {
		entry.ExpiresAt = req.ExpiresAt
		updated = true
	}
	if req.Notes != entry.Notes {
		entry.Notes = req.Notes
		updated = true
	}
	if req.Metadata != nil && len(req.Metadata) > 0 {
		if entry.Metadata == nil {
			entry.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			entry.Metadata[k] = v
		}
		updated = true
	}

	if !updated {
		return s.convertEntryToResponse(entry), nil
	}

	// Update timestamps
	now := time.Now()
	entry.UpdatedAt = &now
	entry.UpdatedBy = &req.UpdatedBy

	// Save changes
	if err := s.entryRepo.Save(ctx, entry); err != nil {
		return nil, errors.NewInternalError("Failed to update suppression entry").WithCause(err)
	}

	// Invalidate cache
	if s.cache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			s.cache.InvalidateSource(cacheCtx, entry.ListSource)
		}()
	}

	// Audit log
	if s.auditService != nil {
		go func() {
			auditCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			auditReq := SuppressionAuditRequest{
				Action:      "UPDATE",
				EntryID:     entry.ID,
				PhoneNumber: entry.PhoneNumber,
				Before:      s.convertEntryToResponse(&originalEntry),
				After:       s.convertEntryToResponse(entry),
				UserID:      req.UpdatedBy,
				Reason:      "Suppression entry updated",
				Timestamp:   time.Now(),
			}
			if err := s.auditService.LogSuppressionChange(auditCtx, auditReq); err != nil {
				s.logger.Error("Failed to log suppression update audit", zap.Error(err))
			}
		}()
	}

	return s.convertEntryToResponse(entry), nil
}

// SyncWithProviders synchronizes DNC data from all configured providers
func (s *service) SyncWithProviders(ctx context.Context) (*SyncResponse, error) {
	startTime := time.Now()
	
	// Get all active providers
	providers, err := s.providerRepo.FindByStatus(ctx, dnc.ProviderStatusActive)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get active providers").WithCause(err)
	}

	if len(providers) == 0 {
		return &SyncResponse{
			StartedAt:      startTime,
			CompletedAt:    time.Now(),
			Duration:       time.Since(startTime),
			TotalProviders: 0,
			SuccessCount:   0,
			FailureCount:   0,
		}, nil
	}

	// Sync providers in parallel
	type providerResult struct {
		response ProviderSyncResponse
		err      error
	}

	resultChan := make(chan providerResult, len(providers))
	
	for _, provider := range providers {
		go func(p *dnc.DNCProvider) {
			result, err := s.syncSingleProvider(ctx, p)
			resultChan <- providerResult{response: result, err: err}
		}(provider)
	}

	// Collect results
	var providerResults []ProviderSyncResponse
	var successCount, failureCount int
	var totalRecords, newRecords, updatedRecords int
	var errors []string

	for i := 0; i < len(providers); i++ {
		result := <-resultChan
		providerResults = append(providerResults, result.response)
		
		if result.err != nil {
			failureCount++
			errors = append(errors, result.err.Error())
		} else {
			successCount++
			totalRecords += result.response.RecordsProcessed
			newRecords += result.response.RecordsAdded
			updatedRecords += result.response.RecordsUpdated
		}
	}

	response := &SyncResponse{
		StartedAt:       startTime,
		CompletedAt:     time.Now(),
		Duration:        time.Since(startTime),
		TotalProviders:  len(providers),
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		ProviderResults: providerResults,
		TotalRecords:    totalRecords,
		NewRecords:      newRecords,
		UpdatedRecords:  updatedRecords,
		Errors:          errors,
	}

	return response, nil
}

// SyncWithProvider synchronizes data from a specific provider
func (s *service) SyncWithProvider(ctx context.Context, providerID uuid.UUID) (*ProviderSyncResponse, error) {
	provider, err := s.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get provider").WithCause(err)
	}

	return s.syncSingleProvider(ctx, provider)
}

// Helper methods

// performDNCCheck performs the actual DNC check logic
func (s *service) performDNCCheck(ctx context.Context, phoneNumber *values.PhoneNumber, callTime time.Time) (*dnc.DNCCheckResult, error) {
	// Use domain compliance service if available
	if s.complianceService != nil {
		complianceResult, err := s.complianceService.CheckCompliance(ctx, phoneNumber, callTime)
		if err != nil {
			return nil, err
		}
		
		// Convert compliance result to check result
		return s.convertComplianceToCheckResult(complianceResult, phoneNumber), nil
	}

	// Fallback to direct repository check
	entries, err := s.entryRepo.FindByPhone(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}

	// Build check result from entries
	return s.buildCheckResultFromEntries(entries, phoneNumber, callTime), nil
}

// performBulkDNCCheck performs bulk DNC checking
func (s *service) performBulkDNCCheck(ctx context.Context, phoneNumbers []*values.PhoneNumber, callTime time.Time) ([]*dnc.DNCCheckResult, error) {
	results := make([]*dnc.DNCCheckResult, len(phoneNumbers))
	
	// Use goroutines for parallel processing but limit concurrency
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var firstError error

	for i, phoneNumber := range phoneNumbers {
		wg.Add(1)
		go func(index int, pn *values.PhoneNumber) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			result, err := s.performDNCCheck(ctx, pn, callTime)
			
			mutex.Lock()
			if err != nil && firstError == nil {
				firstError = err
			}
			if result != nil {
				results[index] = result
			}
			mutex.Unlock()
		}(i, phoneNumber)
	}

	wg.Wait()

	if firstError != nil {
		return nil, firstError
	}

	return results, nil
}

// syncSingleProvider synchronizes a single provider
func (s *service) syncSingleProvider(ctx context.Context, provider *dnc.DNCProvider) (ProviderSyncResponse, error) {
	startTime := time.Now()
	response := ProviderSyncResponse{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		StartedAt:    startTime,
	}

	// Check if sync is needed
	if !provider.NeedsSync() {
		response.Success = true
		response.CompletedAt = time.Now()
		response.Duration = time.Since(startTime)
		response.NextSyncAt = provider.NextSyncAt
		return response, nil
	}

	// Record sync start
	provider.RecordSyncStart()
	if err := s.providerRepo.Save(ctx, provider); err != nil {
		s.logger.Error("Failed to record sync start", zap.Error(err))
	}

	// Perform actual sync based on provider type
	var syncErr error
	var recordsProcessed, recordsAdded, recordsUpdated int

	switch provider.Type {
	case dnc.ProviderTypeFederal:
		recordsProcessed, recordsAdded, recordsUpdated, syncErr = s.syncFederalProvider(ctx, provider)
	case dnc.ProviderTypeState:
		recordsProcessed, recordsAdded, recordsUpdated, syncErr = s.syncStateProvider(ctx, provider)
	case dnc.ProviderTypeInternal:
		recordsProcessed, recordsAdded, recordsUpdated, syncErr = s.syncInternalProvider(ctx, provider)
	case dnc.ProviderTypeCustom:
		recordsProcessed, recordsAdded, recordsUpdated, syncErr = s.syncCustomProvider(ctx, provider)
	default:
		syncErr = fmt.Errorf("unsupported provider type: %s", provider.Type)
	}

	// Update response
	response.CompletedAt = time.Now()
	response.Duration = time.Since(startTime)
	response.RecordsProcessed = recordsProcessed
	response.RecordsAdded = recordsAdded
	response.RecordsUpdated = recordsUpdated

	if syncErr != nil {
		response.Success = false
		response.Error = syncErr.Error()
		response.ErrorCount = 1
		
		// Record sync error
		provider.RecordSyncError(syncErr)
		s.recordProviderError(provider.ID)
	} else {
		response.Success = true
		
		// Record sync success
		provider.RecordSyncSuccess()
	}

	// Update provider
	response.NextSyncAt = provider.NextSyncAt
	if err := s.providerRepo.Save(ctx, provider); err != nil {
		s.logger.Error("Failed to update provider after sync", zap.Error(err))
	}

	// Invalidate cache for this provider
	if s.cache != nil && response.Success {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.cache.InvalidateProvider(cacheCtx, provider.ID); err != nil {
				s.logger.Warn("Failed to invalidate provider cache", zap.Error(err))
			}
		}()
	}

	// Publish sync event
	if s.eventPublisher != nil {
		event := &dnc.DNCListSyncedEvent{
			ProviderID:       provider.ID,
			ProviderName:     provider.Name,
			RecordsProcessed: recordsProcessed,
			RecordsAdded:     recordsAdded,
			RecordsUpdated:   recordsUpdated,
			Success:          response.Success,
			Duration:         response.Duration,
			SyncedAt:         response.CompletedAt,
		}
		
		go func() {
			eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.eventPublisher.PublishDNCListSynced(eventCtx, event); err != nil {
				s.logger.Error("Failed to publish DNC list synced event", zap.Error(err))
			}
		}()
	}

	return response, syncErr
}

// Conversion helper methods

// convertCheckResultToResponse converts domain check result to service response
func (s *service) convertCheckResultToResponse(result *dnc.DNCCheckResult, cached bool) *DNCCheckResponse {
	response := &DNCCheckResponse{
		PhoneNumber:     result.PhoneNumber,
		IsBlocked:       result.IsBlocked,
		ComplianceLevel: string(result.ComplianceLevel),
		RiskScore:       result.GetRiskScore(),
		CheckedAt:       result.CheckedAt,
		CachedResult:    cached,
		TTL:             result.TTL,
		CanCall:         result.CanCall(),
		Sources:         make([]CheckSource, 0),
		Metadata:        make(map[string]interface{}),
	}

	// Convert block reasons
	if result.IsBlocked {
		blockReasons := result.GetBlockingReasons()
		response.BlockReasons = make([]BlockReason, len(blockReasons))
		for i, reason := range blockReasons {
			response.BlockReasons[i] = BlockReason{
				Source:         reason.Source,
				Reason:         reason.Reason,
				Severity:       string(reason.Severity),
				AddedAt:        reason.AddedAt,
				ExpiresAt:      reason.ExpiresAt,
				Description:    reason.Description,
				ComplianceCode: reason.GetComplianceCode(),
			}
		}
		
		if len(blockReasons) > 0 {
			response.HighestSeverity = string(result.GetHighestSeverity())
		}
	}

	// Add metadata
	response.Metadata["check_duration"] = result.Duration
	response.Metadata["source_count"] = result.SourceCount
	if result.IsBlocked {
		response.Metadata["highest_authority_source"] = result.GetHighestAuthoritySource()
		if result.HasPermanentBlock() {
			response.Metadata["permanent_block"] = true
		}
		if expiration := result.GetEarliestExpiration(); expiration != nil {
			response.Metadata["earliest_expiration"] = expiration
		}
	}

	return response
}

// convertEntryToResponse converts domain entry to service response
func (s *service) convertEntryToResponse(entry *dnc.DNCEntry) *SuppressionResponse {
	return &SuppressionResponse{
		ID:             entry.ID,
		PhoneNumber:    entry.PhoneNumber,
		ListSource:     entry.ListSource,
		SuppressReason: entry.SuppressReason,
		AddedAt:        entry.AddedAt,
		ExpiresAt:      entry.ExpiresAt,
		AddedBy:        entry.AddedBy,
		UpdatedBy:      entry.UpdatedBy,
		UpdatedAt:      entry.UpdatedAt,
		Active:         entry.IsActive(),
		Notes:          entry.Notes,
		Metadata:       entry.Metadata,
	}
}

// convertComplianceToCheckResult converts compliance result to check result
func (s *service) convertComplianceToCheckResult(complianceResult *types.ComplianceResult, phoneNumber *values.PhoneNumber) *dnc.DNCCheckResult {
	// This is a placeholder - actual implementation would depend on the compliance result structure
	checkResult, _ := dnc.NewDNCCheckResult(
		phoneNumber,
		time.Now(),
		s.config.CacheDefaultTTL,
		dnc.ComplianceLevelStandard,
	)
	
	// Add blocking reasons based on compliance result
	// Implementation would map compliance violations to block reasons
	
	return checkResult
}

// buildCheckResultFromEntries builds a check result from DNC entries
func (s *service) buildCheckResultFromEntries(entries []*dnc.DNCEntry, phoneNumber *values.PhoneNumber, callTime time.Time) *dnc.DNCCheckResult {
	checkResult, _ := dnc.NewDNCCheckResult(
		phoneNumber,
		callTime,
		s.config.CacheDefaultTTL,
		dnc.ComplianceLevelStandard,
	)

	// Process active entries
	for _, entry := range entries {
		if entry.IsActive() {
			// Add as blocking reason
			blockReason := dnc.BlockReason{
				Source:      entry.ListSource,
				Reason:      entry.SuppressReason,
				AddedAt:     entry.AddedAt,
				ExpiresAt:   entry.ExpiresAt,
				Description: fmt.Sprintf("Listed in %s", entry.ListSource.String()),
			}
			
			// Set severity based on source authority
			blockReason.Severity = entry.GetPriority().String()
			
			checkResult.AddBlockReason(blockReason)
		}
	}

	return checkResult
}

// Provider sync methods (placeholder implementations)

func (s *service) syncFederalProvider(ctx context.Context, provider *dnc.DNCProvider) (processed, added, updated int, err error) {
	// Implementation would use federalDNCClient to sync federal DNC data
	// This is a placeholder
	return 0, 0, 0, fmt.Errorf("federal provider sync not implemented")
}

func (s *service) syncStateProvider(ctx context.Context, provider *dnc.DNCProvider) (processed, added, updated int, err error) {
	// Implementation would use stateDNCClient to sync state DNC data
	// This is a placeholder
	return 0, 0, 0, fmt.Errorf("state provider sync not implemented")
}

func (s *service) syncInternalProvider(ctx context.Context, provider *dnc.DNCProvider) (processed, added, updated int, err error) {
	// Implementation would handle internal list updates
	// This is a placeholder
	return 0, 0, 0, nil // Internal sync typically doesn't need external calls
}

func (s *service) syncCustomProvider(ctx context.Context, provider *dnc.DNCProvider) (processed, added, updated int, err error) {
	// Implementation would handle custom provider APIs
	// This is a placeholder
	return 0, 0, 0, fmt.Errorf("custom provider sync not implemented")
}

// Metrics and monitoring methods

func (s *service) recordMetrics(duration time.Duration, requestCount int, errorCount int) {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	
	s.metrics.totalChecks += int64(requestCount)
	s.metrics.errorCount += int64(errorCount)
	
	// Update average latency with exponential moving average
	if s.metrics.averageLatency == 0 {
		s.metrics.averageLatency = duration
	} else {
		s.metrics.averageLatency = time.Duration(
			0.9*float64(s.metrics.averageLatency) + 0.1*float64(duration),
		)
	}
	
	// Check for slow queries
	if duration > time.Duration(s.config.SlowQueryThresholdMs)*time.Millisecond {
		s.metrics.slowQueries++
	}
}

func (s *service) recordCacheHit() {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.cacheHits++
}

func (s *service) recordCacheMiss() {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.cacheMisses++
}

func (s *service) recordError() {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.errorCount++
}

func (s *service) recordProviderError(providerID uuid.UUID) {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.providerErrors[providerID]++
}

// publishAuditEvent publishes DNC check audit event
func (s *service) publishAuditEvent(ctx context.Context, phoneNumber *values.PhoneNumber, result *dnc.DNCCheckResult) {
	if s.auditService == nil {
		return
	}

	auditCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	response := s.convertCheckResultToResponse(result, false)
	auditReq := DNCCheckAuditRequest{
		PhoneNumber: phoneNumber,
		Result:      *response,
		UserID:      uuid.Nil, // Would be set from context in real implementation
		RequestID:   uuid.New().String(),
		CheckedAt:   result.CheckedAt,
	}

	if err := s.auditService.LogDNCCheck(auditCtx, auditReq); err != nil {
		s.logger.Error("Failed to log DNC check audit", zap.Error(err))
	}
}