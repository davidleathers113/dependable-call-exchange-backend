package dnc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// UpdateProvider updates provider configuration and credentials
func (s *service) UpdateProvider(ctx context.Context, req UpdateProviderRequest) (*ProviderResponse, error) {
	if req.ID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_ID", "provider ID is required")
	}

	// Get existing provider
	provider, err := s.providerRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get provider").WithCause(err)
	}

	// Update fields
	updated := false
	if req.Name != "" && req.Name != provider.Name {
		provider.Name = req.Name
		updated = true
	}
	if req.URL != "" && req.URL != provider.URL {
		provider.URL = req.URL
		updated = true
	}
	if req.AuthType != "" && req.AuthType != string(provider.AuthType) {
		provider.AuthType = dnc.AuthType(req.AuthType)
		updated = true
	}
	if req.UpdateFrequency > 0 && req.UpdateFrequency != provider.UpdateFrequency {
		provider.UpdateFrequency = req.UpdateFrequency
		updated = true
	}
	if req.Priority > 0 && req.Priority != provider.Priority {
		provider.Priority = req.Priority
		updated = true
	}
	if req.Active != nil && *req.Active != (provider.Status == dnc.ProviderStatusActive) {
		if *req.Active {
			provider.Status = dnc.ProviderStatusActive
		} else {
			provider.Status = dnc.ProviderStatusInactive
		}
		updated = true
	}
	if req.Configuration != nil && len(req.Configuration) > 0 {
		if provider.Configuration == nil {
			provider.Configuration = make(map[string]interface{})
		}
		for k, v := range req.Configuration {
			provider.Configuration[k] = v
		}
		updated = true
	}

	if !updated {
		return s.convertProviderToResponse(provider), nil
	}

	// Update timestamp
	provider.UpdatedAt = time.Now()

	// Save changes
	if err := s.providerRepo.Save(ctx, provider); err != nil {
		return nil, errors.NewInternalError("Failed to update provider").WithCause(err)
	}

	// Invalidate cache
	if s.cache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			s.cache.InvalidateProvider(cacheCtx, provider.ID)
		}()
	}

	return s.convertProviderToResponse(provider), nil
}

// GetProviderStatus returns current status and health of providers
func (s *service) GetProviderStatus(ctx context.Context, providerID uuid.UUID) (*ProviderStatusResponse, error) {
	if providerID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_ID", "provider ID is required")
	}

	provider, err := s.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get provider").WithCause(err)
	}

	// Get records managed by this provider
	recordsCount, err := s.entryRepo.CountByListSource(ctx, provider.GetListSource())
	if err != nil {
		s.logger.Warn("Failed to get records count", zap.Error(err))
		recordsCount = 0
	}

	// Get sync status
	s.syncMutex.RLock()
	syncStatus, exists := s.providerSyncStatus[providerID]
	s.syncMutex.RUnlock()

	syncStatusStr := "idle"
	if exists && syncStatus.InProgress {
		syncStatusStr = "syncing"
	}

	lastSyncResult := "success"
	if provider.ErrorCount > 0 {
		lastSyncResult = "error"
	}

	// Calculate performance metrics
	performanceMetrics := s.calculateProviderPerformanceMetrics(provider)

	// Get recent errors (placeholder - would come from error tracking)
	var recentErrors []string
	if provider.LastError != "" {
		recentErrors = append(recentErrors, provider.LastError)
	}

	response := &ProviderStatusResponse{
		Provider:           *s.convertProviderToResponse(provider),
		SyncStatus:         syncStatusStr,
		LastSyncResult:     lastSyncResult,
		RecordsManaged:     recordsCount,
		PerformanceMetrics: performanceMetrics,
		RecentErrors:       recentErrors,
	}

	return response, nil
}

// GetComplianceReport generates comprehensive compliance reports
func (s *service) GetComplianceReport(ctx context.Context, criteria ComplianceReportCriteria) (*ComplianceReportResponse, error) {
	// Set defaults
	if criteria.Limit <= 0 {
		criteria.Limit = 1000
	}
	if criteria.Format == "" {
		criteria.Format = "json"
	}

	// Build filter for repository query
	filter := dnc.DNCEntryFilter{
		PhoneNumbers:    criteria.PhoneNumbers,
		ListSources:     criteria.ListSources,
		SuppressReasons: criteria.SuppressReasons,
		StartDate:       criteria.StartDate,
		EndDate:         criteria.EndDate,
		IncludeExpired:  criteria.IncludeExpired,
		Limit:           criteria.Limit,
		Offset:          criteria.Offset,
	}

	// Get entries from repository
	entries, total, err := s.entryRepo.FindWithFilter(ctx, filter)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get compliance data").WithCause(err)
	}

	// Convert entries to responses
	entryResponses := make([]SuppressionResponse, len(entries))
	for i, entry := range entries {
		entryResponses[i] = *s.convertEntryToResponse(entry)
	}

	// Calculate summary and statistics
	summary := s.calculateComplianceSummary(entries, total)
	statistics := s.calculateComplianceStatistics(entries)

	response := &ComplianceReportResponse{
		GeneratedAt: time.Now(),
		Criteria:    criteria,
		Summary:     summary,
		Statistics:  statistics,
		Format:      criteria.Format,
	}

	// Include entries if details requested
	if criteria.IncludeDetails {
		response.Entries = entryResponses
	}

	return response, nil
}

// ValidateCall performs full TCPA and DNC validation for a call
func (s *service) ValidateCall(ctx context.Context, req CallValidationRequest) (*CallValidationResponse, error) {
	if req.FromNumber == nil {
		return nil, errors.NewValidationError("INVALID_FROM_NUMBER", "from number is required")
	}
	if req.ToNumber == nil {
		return nil, errors.NewValidationError("INVALID_TO_NUMBER", "to number is required")
	}

	startTime := time.Now()

	// Perform DNC check on destination number
	dncResult, err := s.CheckDNC(ctx, req.ToNumber, req.CallTime)
	if err != nil {
		return nil, errors.NewInternalError("DNC check failed").WithCause(err)
	}

	// Initialize response
	response := &CallValidationResponse{
		FromNumber:  req.FromNumber,
		ToNumber:    req.ToNumber,
		CallTime:    req.CallTime,
		DNCResult:   *dncResult,
		ValidatedAt: startTime,
	}

	// Check TCPA calling hours if not bypassed
	callingHoursOK := true
	if !req.BypassTimeChecks && s.timeZoneService != nil {
		var err error
		callingHoursOK, err = s.timeZoneService.IsWithinCallingHours(req.ToNumber, req.CallTime)
		if err != nil {
			s.logger.Warn("Failed to check calling hours", zap.Error(err))
			callingHoursOK = false
		}
	}
	response.CallingHours = callingHoursOK

	// Check if number is wireless (requires consent)
	isWireless := req.ToNumber.IsWireless()
	response.ConsentRequired = isWireless || req.RequiresConsent

	// Placeholder for consent check
	response.ConsentPresent = false // Would integrate with consent service

	// Determine overall TCPA compliance
	response.TCPACompliant = callingHoursOK && (!response.ConsentRequired || response.ConsentPresent)

	// Overall call allowance
	response.Allowed = !dncResult.IsBlocked && response.TCPACompliant

	// Get risk assessment if service available
	if s.riskService != nil {
		callContext := types.CallContext{
			CallType:   req.CallType,
			CallerID:   req.FromNumber,
			TimeZone:   req.ToNumber.GetTimeZone(),
		}
		
		riskAssessment, err := s.riskService.AssessRisk(ctx, req.ToNumber, callContext)
		if err != nil {
			s.logger.Warn("Failed to get risk assessment", zap.Error(err))
		} else {
			response.RiskAssessment = *s.convertRiskAssessmentToResponse(riskAssessment)
		}
	}

	// Collect violations
	var violations []ComplianceViolation
	if dncResult.IsBlocked {
		for _, reason := range dncResult.BlockReasons {
			violation := ComplianceViolation{
				Type:        "DNC_VIOLATION",
				Severity:    reason.Severity,
				Description: fmt.Sprintf("Number is listed in %s", reason.Source.String()),
				Regulation:  reason.ComplianceCode,
			}
			violations = append(violations, violation)
		}
	}

	if !callingHoursOK {
		violations = append(violations, ComplianceViolation{
			Type:        "TCPA_TIME_VIOLATION",
			Severity:    "HIGH",
			Description: "Call attempted outside permitted hours (8 AM - 9 PM local time)",
			Regulation:  "TCPA",
		})
	}

	if response.ConsentRequired && !response.ConsentPresent {
		violations = append(violations, ComplianceViolation{
			Type:        "TCPA_CONSENT_VIOLATION",
			Severity:    "HIGH",
			Description: "Wireless number requires prior express written consent",
			Regulation:  "TCPA",
		})
	}

	response.Violations = violations

	// Generate recommendations
	var recommendations []string
	if dncResult.IsBlocked {
		recommendations = append(recommendations, "Do not call - number is on Do Not Call list")
	}
	if !callingHoursOK {
		recommendations = append(recommendations, "Call between 8 AM and 9 PM in recipient's local time")
	}
	if response.ConsentRequired && !response.ConsentPresent {
		recommendations = append(recommendations, "Obtain prior express written consent before calling wireless numbers")
	}
	if response.Allowed {
		recommendations = append(recommendations, "Call is permitted under current compliance rules")
	}

	response.Recommendations = recommendations

	return response, nil
}

// GetRiskAssessment calculates violation risk and potential penalties
func (s *service) GetRiskAssessment(ctx context.Context, phoneNumber *values.PhoneNumber, callContext CallContext) (*RiskAssessmentResponse, error) {
	if phoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "phone number is required")
	}

	if s.riskService == nil {
		return nil, errors.NewServiceError("RISK_SERVICE_UNAVAILABLE", "risk assessment service not available")
	}

	// Convert service call context to domain type
	domainContext := types.CallContext{
		CallType:   callContext.CallType,
		Campaign:   callContext.Campaign,
		CallerID:   callContext.CallerID,
		CallCenter: callContext.CallCenter,
		TimeZone:   callContext.TimeZone,
		Metadata:   callContext.Metadata,
	}

	// Get risk assessment from domain service
	riskAssessment, err := s.riskService.AssessRisk(ctx, phoneNumber, domainContext)
	if err != nil {
		return nil, errors.NewInternalError("Risk assessment failed").WithCause(err)
	}

	return s.convertRiskAssessmentToResponse(riskAssessment), nil
}

// GetSuppressionEntry retrieves details of a specific suppression entry
func (s *service) GetSuppressionEntry(ctx context.Context, id uuid.UUID) (*SuppressionResponse, error) {
	if id == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_ID", "entry ID is required")
	}

	entry, err := s.entryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get suppression entry").WithCause(err)
	}

	return s.convertEntryToResponse(entry), nil
}

// SearchSuppressions finds suppression entries matching criteria
func (s *service) SearchSuppressions(ctx context.Context, criteria SearchCriteria) (*SearchResponse, error) {
	// Set defaults
	if criteria.Limit <= 0 || criteria.Limit > 1000 {
		criteria.Limit = 100
	}
	if criteria.SortBy == "" {
		criteria.SortBy = "added_at"
	}
	if criteria.SortOrder == "" {
		criteria.SortOrder = "desc"
	}

	// Convert to domain filter
	filter := dnc.DNCEntryFilter{
		PhoneNumberPattern: criteria.PhoneNumberPattern,
		ListSources:        criteria.ListSources,
		SuppressReasons:    criteria.SuppressReasons,
		Active:             criteria.Active,
		StartDate:          criteria.StartDate,
		EndDate:            criteria.EndDate,
		AddedBy:            criteria.AddedBy,
		SortBy:             criteria.SortBy,
		SortOrder:          criteria.SortOrder,
		Limit:              criteria.Limit,
		Offset:             criteria.Offset,
	}

	// Search repository
	entries, total, err := s.entryRepo.FindWithFilter(ctx, filter)
	if err != nil {
		return nil, errors.NewInternalError("Search failed").WithCause(err)
	}

	// Convert to responses
	results := make([]SuppressionResponse, len(entries))
	for i, entry := range entries {
		results[i] = *s.convertEntryToResponse(entry)
	}

	response := &SearchResponse{
		Results:    results,
		Total:      total,
		Offset:     criteria.Offset,
		Limit:      criteria.Limit,
		HasMore:    criteria.Offset+criteria.Limit < total,
		SearchedAt: time.Now(),
	}

	return response, nil
}

// GetCacheStats returns DNC cache performance statistics
func (s *service) GetCacheStats(ctx context.Context) (*CacheStatsResponse, error) {
	if s.cache == nil {
		return nil, errors.NewServiceError("CACHE_UNAVAILABLE", "cache service not available")
	}

	cacheStats, err := s.cache.GetStats(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to get cache stats").WithCause(err)
	}

	// Calculate rates
	total := cacheStats.Hits + cacheStats.Misses
	var hitRate, missRate float64
	if total > 0 {
		hitRate = float64(cacheStats.Hits) / float64(total) * 100
		missRate = float64(cacheStats.Misses) / float64(total) * 100
	}

	response := &CacheStatsResponse{
		HitRate:        hitRate,
		MissRate:       missRate,
		TotalRequests:  total,
		CacheHits:      cacheStats.Hits,
		CacheMisses:    cacheStats.Misses,
		AverageLatency: cacheStats.AverageLatency,
		MemoryUsage:    cacheStats.MemoryUsage,
		KeyCount:       cacheStats.KeyCount,
		CollectedAt:    time.Now(),
	}

	return response, nil
}

// ClearCache invalidates DNC cache entries
func (s *service) ClearCache(ctx context.Context, pattern string) error {
	if s.cache == nil {
		return errors.NewServiceError("CACHE_UNAVAILABLE", "cache service not available")
	}

	if err := s.cache.Clear(ctx, pattern); err != nil {
		return errors.NewInternalError("Failed to clear cache").WithCause(err)
	}

	s.logger.Info("Cache cleared", zap.String("pattern", pattern))
	return nil
}

// HealthCheck validates service health and dependencies
func (s *service) HealthCheck(ctx context.Context) (*HealthResponse, error) {
	startTime := time.Now()
	
	response := &HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0", // Would come from build info
		CheckedAt: startTime,
		Metrics:   s.calculateHealthMetrics(),
	}

	var dependencies []DependencyHealth
	var warnings []string
	var errors []string

	// Check database connectivity
	if dbHealth := s.checkDatabaseHealth(ctx); dbHealth.Status != "healthy" {
		dependencies = append(dependencies, dbHealth)
		if dbHealth.Status == "unhealthy" {
			response.Status = "unhealthy"
			errors = append(errors, fmt.Sprintf("Database: %s", dbHealth.Error))
		} else {
			warnings = append(warnings, fmt.Sprintf("Database: %s", dbHealth.Error))
		}
	} else {
		dependencies = append(dependencies, dbHealth)
	}

	// Check cache connectivity
	if cacheHealth := s.checkCacheHealth(ctx); cacheHealth.Status != "healthy" {
		dependencies = append(dependencies, cacheHealth)
		if cacheHealth.Status == "unhealthy" {
			warnings = append(warnings, fmt.Sprintf("Cache: %s", cacheHealth.Error))
		}
	} else {
		dependencies = append(dependencies, cacheHealth)
	}

	// Check external providers
	providerHealth := s.checkProviderHealth(ctx)
	dependencies = append(dependencies, providerHealth...)

	response.Dependencies = dependencies
	response.Warnings = warnings
	response.Errors = errors

	return response, nil
}

// Helper methods for health checks and conversions

func (s *service) checkDatabaseHealth(ctx context.Context) DependencyHealth {
	startTime := time.Now()
	health := DependencyHealth{
		Name:      "database",
		LastCheck: startTime,
	}

	// Simple connectivity test
	_, err := s.entryRepo.GetByID(ctx, uuid.New()) // This should return not found, but proves connectivity
	responseTime := time.Since(startTime)
	health.ResponseTime = responseTime

	if err != nil && !errors.IsNotFoundError(err) {
		health.Status = "unhealthy"
		health.Error = err.Error()
	} else {
		health.Status = "healthy"
	}

	return health
}

func (s *service) checkCacheHealth(ctx context.Context) DependencyHealth {
	startTime := time.Now()
	health := DependencyHealth{
		Name:      "cache",
		LastCheck: startTime,
	}

	if s.cache == nil {
		health.Status = "unavailable"
		health.Error = "cache service not configured"
		return health
	}

	// Simple cache test
	_, err := s.cache.GetStats(ctx)
	responseTime := time.Since(startTime)
	health.ResponseTime = responseTime

	if err != nil {
		health.Status = "unhealthy"
		health.Error = err.Error()
	} else {
		health.Status = "healthy"
	}

	return health
}

func (s *service) checkProviderHealth(ctx context.Context) []DependencyHealth {
	var dependencies []DependencyHealth

	// Get all providers
	providers, err := s.providerRepo.FindByStatus(ctx, dnc.ProviderStatusActive)
	if err != nil {
		dependency := DependencyHealth{
			Name:      "providers",
			Status:    "unhealthy",
			Error:     "failed to get provider list",
			LastCheck: time.Now(),
		}
		return []DependencyHealth{dependency}
	}

	for _, provider := range providers {
		health := DependencyHealth{
			Name:      fmt.Sprintf("provider_%s", provider.Name),
			LastCheck: time.Now(),
		}

		// Check provider health based on recent sync status
		if provider.ErrorCount > 5 {
			health.Status = "degraded"
			health.Error = fmt.Sprintf("high error count: %d", provider.ErrorCount)
		} else if time.Since(provider.LastSyncAt) > 2*provider.UpdateFrequency {
			health.Status = "degraded"
			health.Error = "sync overdue"
		} else {
			health.Status = "healthy"
		}

		dependencies = append(dependencies, health)
	}

	return dependencies
}

func (s *service) calculateHealthMetrics() HealthMetrics {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	var hitRate float64
	totalCacheRequests := s.metrics.cacheHits + s.metrics.cacheMisses
	if totalCacheRequests > 0 {
		hitRate = float64(s.metrics.cacheHits) / float64(totalCacheRequests) * 100
	}

	var errorRate float64
	if s.metrics.totalChecks > 0 {
		errorRate = float64(s.metrics.errorCount) / float64(s.metrics.totalChecks) * 100
	}

	return HealthMetrics{
		AverageResponseTime: s.metrics.averageLatency,
		ErrorRate:           errorRate,
		CacheHitRate:        hitRate,
		// Other metrics would be populated from actual system monitoring
	}
}

func (s *service) calculateProviderPerformanceMetrics(provider *dnc.DNCProvider) PerformanceMetrics {
	// This would typically come from stored metrics
	successRate := 100.0
	if provider.ErrorCount > 0 {
		// Simple calculation - in practice would be more sophisticated
		successRate = 95.0
	}

	return PerformanceMetrics{
		AverageResponseTime: 250 * time.Millisecond, // Placeholder
		SuccessRate:         successRate,
		ErrorRate:           100.0 - successRate,
		TotalRequests:       100, // Placeholder
		SuccessfulRequests:  int64(successRate),
		FailedRequests:      int64(100.0 - successRate),
		LastResponseTime:    200 * time.Millisecond, // Placeholder
		LastUpdated:         provider.UpdatedAt,
	}
}

func (s *service) calculateComplianceSummary(entries []*dnc.DNCEntry, total int) ComplianceSummary {
	summary := ComplianceSummary{
		TotalEntries:    total,
		SourceBreakdown: make(map[string]int),
		ReasonBreakdown: make(map[string]int),
		RiskDistribution: make(map[string]int),
	}

	activeCount := 0
	expiredCount := 0
	recentActivityCount := 0
	cutoff := time.Now().AddDate(0, 0, -30) // Last 30 days

	for _, entry := range entries {
		// Count active/expired
		if entry.IsActive() {
			activeCount++
		} else {
			expiredCount++
		}

		// Recent activity
		if entry.AddedAt.After(cutoff) {
			recentActivityCount++
		}

		// Source breakdown
		source := entry.ListSource.String()
		summary.SourceBreakdown[source]++

		// Reason breakdown
		reason := entry.SuppressReason.String()
		summary.ReasonBreakdown[reason]++

		// Risk distribution (simplified)
		if entry.ListSource == values.ListSourceFederal || entry.ListSource == values.ListSourceLitigation {
			summary.RiskDistribution["high"]++
		} else if entry.ListSource == values.ListSourceState {
			summary.RiskDistribution["medium"]++
		} else {
			summary.RiskDistribution["low"]++
		}
	}

	summary.ActiveEntries = activeCount
	summary.ExpiredEntries = expiredCount
	summary.RecentActivity = recentActivityCount

	return summary
}

func (s *service) calculateComplianceStatistics(entries []*dnc.DNCEntry) ComplianceStatistics {
	stats := ComplianceStatistics{
		BySource:    make(map[values.ListSource]int),
		ByReason:    make(map[values.SuppressReason]int),
		ByMonth:     make(map[string]int),
		ByRiskLevel: make(map[string]int),
	}

	for _, entry := range entries {
		// By source
		stats.BySource[entry.ListSource]++

		// By reason
		stats.ByReason[entry.SuppressReason]++

		// By month
		monthKey := entry.AddedAt.Format("2006-01")
		stats.ByMonth[monthKey]++

		// By risk level
		switch entry.ListSource {
		case values.ListSourceFederal, values.ListSourceLitigation:
			stats.ByRiskLevel["high"]++
		case values.ListSourceState:
			stats.ByRiskLevel["medium"]++
		default:
			stats.ByRiskLevel["low"]++
		}
	}

	// Calculate compliance score (simplified)
	totalEntries := len(entries)
	if totalEntries > 0 {
		regulatoryEntries := stats.BySource[values.ListSourceFederal] + stats.BySource[values.ListSourceState]
		stats.ComplianceScore = float64(regulatoryEntries) / float64(totalEntries) * 100
	}

	// Determine trend (simplified - would use historical data)
	stats.TrendDirection = "stable"

	return stats
}

func (s *service) convertProviderToResponse(provider *dnc.DNCProvider) *ProviderResponse {
	return &ProviderResponse{
		ID:              provider.ID,
		Name:            provider.Name,
		Type:            string(provider.Type),
		URL:             provider.URL,
		AuthType:        string(provider.AuthType),
		UpdateFrequency: provider.UpdateFrequency,
		Priority:        provider.Priority,
		Active:          provider.Status == dnc.ProviderStatusActive,
		CreatedAt:       provider.CreatedAt,
		UpdatedAt:       provider.UpdatedAt,
		LastSyncAt:      &provider.LastSyncAt,
		NextSyncAt:      provider.NextSyncAt,
		HealthStatus:    string(provider.Status),
		ErrorCount:      provider.ErrorCount,
		SuccessRate:     provider.GetSuccessRate(),
		Configuration:   provider.Configuration,
	}
}

func (s *service) convertRiskAssessmentToResponse(assessment *types.RiskAssessment) *RiskAssessmentResponse {
	response := &RiskAssessmentResponse{
		PhoneNumber: assessment.PhoneNumber,
		RiskScore:   assessment.RiskScore,
		RiskLevel:   string(assessment.RiskLevel),
		AssessedAt:  assessment.AssessedAt,
	}

	// Convert factors
	response.Factors = make([]RiskFactor, len(assessment.Factors))
	for i, factor := range assessment.Factors {
		response.Factors[i] = RiskFactor{
			Factor:      factor.Factor,
			Weight:      factor.Weight,
			Score:       factor.Score,
			Description: factor.Description,
		}
	}

	// Convert recommendations
	response.Recommendations = assessment.Recommendations

	// Convert penalty estimate
	if assessment.PenaltyEstimate != nil {
		response.PenaltyEstimate = assessment.PenaltyEstimate
	}

	// Convert call history summary
	if assessment.CallHistory != nil {
		response.CallHistory = CallHistorySummary{
			TotalCalls:      assessment.CallHistory.TotalCalls,
			RecentCalls:     assessment.CallHistory.RecentCalls,
			Violations:      assessment.CallHistory.Violations,
			LastCallAt:      assessment.CallHistory.LastCallAt,
			LastViolationAt: assessment.CallHistory.LastViolationAt,
			CallFrequency:   assessment.CallHistory.CallFrequency,
		}
	}

	return response
}