package rest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	dncService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/dnc"
)

// DNCHandler handles Do Not Call (DNC) API endpoints
type DNCHandler struct {
	*BaseHandler
	dncService dncService.Service
	logger     *slog.Logger
	validator  *validator.Validate
}

// NewDNCHandler creates a new DNC handler
func NewDNCHandler(baseHandler *BaseHandler, service dncService.Service, logger *slog.Logger) *DNCHandler {
	validator := validator.New()
	
	// Register custom validation for phone numbers
	validator.RegisterValidation("e164", validateE164PhoneNumber)
	
	return &DNCHandler{
		BaseHandler: baseHandler,
		dncService:  service,
		logger:      logger,
		validator:   validator,
	}
}

// RegisterDNCRoutes registers all DNC-related routes with proper middleware
func (h *DNCHandler) RegisterDNCRoutes(mux *http.ServeMux) {
	// Create middleware chain for protected endpoints
	authChain := NewMiddlewareChain(
		SecurityHeadersMiddleware(),
		RequestIDMiddleware(),
		RequestLoggingMiddleware(h.logger),
		MetricsMiddleware(),
		ContentNegotiationMiddleware(),
		// AuthMiddleware would be added here for production
	)

	// High-performance rate limiter for DNC checks (critical path)
	dncCheckRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1000, // High throughput for sub-millisecond checks
		Burst:             2000,
		ByUser:            true,
		ByIP:              true,
	})

	// Standard rate limiter for management operations
	dncManagementRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 100,
		Burst:             200,
		ByUser:            true,
	})

	// Administrative rate limiter for reporting
	dncReportingRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		ByUser:            true,
	})

	// Core DNC Check Endpoints (High Performance)
	mux.Handle("POST /api/v1/dnc/check", authChain.Then(
		dncCheckRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/dnc/check", h.handleDNCCheck,
				WithMaxBodySize(1<<10), // 1KB - small payloads for speed
				WithTimeout(5*time.Second), // Fast timeout for sub-10ms target
				WithCache(4*time.Hour), // Cache check results
			),
		),
	))

	mux.Handle("POST /api/v1/dnc/check/bulk", authChain.Then(
		dncCheckRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/dnc/check/bulk", h.handleBulkDNCCheck,
				WithMaxBodySize(100<<10), // 100KB for bulk operations
				WithTimeout(30*time.Second), // Longer timeout for bulk processing
			),
		),
	))

	// Suppression List Management Endpoints
	mux.Handle("POST /api/v1/dnc/suppress", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/dnc/suppress", h.handleAddSuppression,
				WithMaxBodySize(10<<10), // 10KB
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("GET /api/v1/dnc/entries", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/dnc/entries", h.handleListDNCEntries,
				WithCache(15*time.Minute),
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("PUT /api/v1/dnc/entries/{id}", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("PUT", "/api/v1/dnc/entries/{id}", h.handleUpdateDNCEntry,
				WithMaxBodySize(10<<10),
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("DELETE /api/v1/dnc/entries/{id}", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("DELETE", "/api/v1/dnc/entries/{id}", h.handleRemoveSuppression,
				WithTimeout(30*time.Second),
			),
		),
	))

	// Provider Management Endpoints
	mux.Handle("GET /api/v1/dnc/providers", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/dnc/providers", h.handleListProviders,
				WithCache(15*time.Minute),
			),
		),
	))

	mux.Handle("POST /api/v1/dnc/providers/{id}/sync", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/dnc/providers/{id}/sync", h.handleSyncProvider,
				WithTimeout(300*time.Second), // 5 minutes for sync operations
			),
		),
	))

	mux.Handle("GET /api/v1/dnc/providers/{id}/status", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/dnc/providers/{id}/status", h.handleProviderStatus,
				WithCache(5*time.Minute),
			),
		),
	))

	// Compliance and Reporting Endpoints
	mux.Handle("GET /api/v1/dnc/compliance/report", authChain.Then(
		dncReportingRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/dnc/compliance/report", h.handleComplianceReport,
				WithCache(1*time.Hour),
				WithTimeout(60*time.Second),
			),
		),
	))

	// Health Check Endpoint (No auth required)
	mux.Handle("GET /api/v1/dnc/health", 
		h.WrapHandler("GET", "/api/v1/dnc/health", h.handleDNCHealth),
	)

	// Administrative Endpoints (High privilege required)
	mux.Handle("POST /api/v1/dnc/cache/clear", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/dnc/cache/clear", h.handleClearCache,
				WithTimeout(30*time.Second),
				WithMaxBodySize(1<<10),
			),
		),
	))

	mux.Handle("GET /api/v1/dnc/cache/stats", authChain.Then(
		dncManagementRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/dnc/cache/stats", h.handleCacheStats,
				WithCache(1*time.Minute),
			),
		),
	))
}

// handleDNCCheck performs a single phone number DNC compliance check
// @Summary Check if a phone number is on Do Not Call lists
// @Description Performs a comprehensive DNC check with sub-10ms latency using cached results when available
// @Tags DNC
// @Accept json
// @Produce json
// @Param request body DNCCheckRequest true "DNC check request"
// @Success 200 {object} ResponseEnvelope{data=DNCCheckResponse} "DNC check result"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/check [post]
func (h *DNCHandler) handleDNCCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse and validate request
	var req DNCCheckRequest
	if err := h.ParseJSONRequest(r, &req); err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
			"Failed to parse DNC check request", err.Error())
		return
	}

	// Convert phone number to domain value object
	phoneNumber, err := values.NewPhoneNumber(req.PhoneNumber)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PHONE_NUMBER", 
			"Phone number must be in E.164 format", err.Error())
		return
	}

	// Perform DNC check with service
	checkResult, err := h.dncService.CheckDNC(ctx, phoneNumber, time.Now())
	if err != nil {
		h.handleServiceError(w, err, "DNC check failed")
		return
	}

	// Convert to response DTO
	response := h.convertDNCCheckToResponse(checkResult)

	// Add HATEOAS links
	links := map[string]string{
		"self": fmt.Sprintf("/api/v1/dnc/check?phone=%s", req.PhoneNumber),
		"bulk": "/api/v1/dnc/check/bulk",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleBulkDNCCheck performs DNC checks for multiple phone numbers
// @Summary Bulk check multiple phone numbers against DNC lists
// @Description Efficiently processes multiple phone numbers using parallel checking and batch operations
// @Tags DNC
// @Accept json
// @Produce json
// @Param request body BulkDNCCheckRequest true "Bulk DNC check request"
// @Success 200 {object} ResponseEnvelope{data=BulkDNCCheckResponse} "Bulk DNC check results"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 413 {object} ResponseEnvelope{error=ErrorResponse} "Request too large"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/check/bulk [post]
func (h *DNCHandler) handleBulkDNCCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse and validate request
	var req BulkDNCCheckRequest
	if err := h.ParseJSONRequest(r, &req); err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
			"Failed to parse bulk DNC check request", err.Error())
		return
	}

	// Validate bulk request limits
	if len(req.PhoneNumbers) == 0 {
		h.WriteErrorResponse(w, http.StatusBadRequest, "EMPTY_REQUEST", 
			"At least one phone number is required", "")
		return
	}

	if len(req.PhoneNumbers) > 1000 { // Configurable limit
		h.WriteErrorResponse(w, http.StatusBadRequest, "REQUEST_TOO_LARGE", 
			"Maximum 1000 phone numbers per bulk request", "")
		return
	}

	// Convert phone numbers to domain value objects
	phoneNumbers := make([]*values.PhoneNumber, 0, len(req.PhoneNumbers))
	invalidNumbers := make([]string, 0)

	for _, phoneStr := range req.PhoneNumbers {
		phone, err := values.NewPhoneNumber(phoneStr)
		if err != nil {
			invalidNumbers = append(invalidNumbers, phoneStr)
			continue
		}
		phoneNumbers = append(phoneNumbers, phone)
	}

	if len(invalidNumbers) > 0 {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PHONE_NUMBERS", 
			"Some phone numbers are invalid", fmt.Sprintf("Invalid numbers: %v", invalidNumbers))
		return
	}

	// Perform bulk DNC check
	callTime := time.Now()
	if req.CallTime != nil {
		callTime = *req.CallTime
	}

	checkResults, err := h.dncService.CheckDNCBulk(ctx, phoneNumbers, callTime)
	if err != nil {
		h.handleServiceError(w, err, "Bulk DNC check failed")
		return
	}

	// Convert to response DTO
	response := h.convertBulkDNCCheckToResponse(checkResults, req.IncludeDetails)

	// Add HATEOAS links
	links := map[string]string{
		"self":   "/api/v1/dnc/check/bulk",
		"single": "/api/v1/dnc/check",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleAddSuppression adds a phone number to the suppression list
// @Summary Add phone number to suppression list
// @Description Adds a phone number to internal suppression list with proper validation and audit trail
// @Tags DNC
// @Accept json
// @Produce json
// @Param request body CreateDNCEntryRequest true "Suppression request"
// @Success 201 {object} ResponseEnvelope{data=DNCEntryResponse} "Suppression entry created"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 409 {object} ResponseEnvelope{error=ErrorResponse} "Number already suppressed"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/suppress [post]
func (h *DNCHandler) handleAddSuppression(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse and validate request
	var req CreateDNCEntryRequest
	if err := h.ParseJSONRequest(r, &req); err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
			"Failed to parse suppression request", err.Error())
		return
	}

	// Convert to service request using the converter
	serviceReq, err := req.ToAddSuppressionRequest(requestMeta.UserID)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "CONVERSION_ERROR", 
			"Failed to convert request", err.Error())
		return
	}

	// Add to suppression list
	response, err := h.dncService.AddToSuppressionList(ctx, *serviceReq)
	if err != nil {
		h.handleServiceError(w, err, "Failed to add suppression")
		return
	}

	// Convert to response DTO
	responseDTO := h.convertSuppressionToResponse(response)

	// Add HATEOAS links
	links := map[string]string{
		"self":   fmt.Sprintf("/api/v1/dnc/entries/%s", response.Entry.ID().String()),
		"check":  fmt.Sprintf("/api/v1/dnc/check?phone=%s", req.PhoneNumber),
		"delete": fmt.Sprintf("/api/v1/dnc/entries/%s", response.Entry.ID().String()),
	}

	h.WriteSuccessResponse(w, http.StatusCreated, responseDTO, requestMeta, links)
}

// handleListDNCEntries lists DNC entries with filtering and pagination
// @Summary List DNC entries
// @Description Retrieves DNC entries with filtering, sorting, and pagination support
// @Tags DNC
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 50, max: 1000)"
// @Param source query string false "Filter by list source (federal, state, internal, custom)"
// @Param phone query string false "Filter by phone number (partial match)"
// @Param status query string false "Filter by status (active, expired, pending)"
// @Param sort_by query string false "Sort field (created_at, phone_number, expires_at)"
// @Param sort_order query string false "Sort order (asc, desc)"
// @Success 200 {object} ResponseEnvelope{data=PaginatedDNCEntriesResponse} "DNC entries list"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request parameters"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/entries [get]
func (h *DNCHandler) handleListDNCEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse query parameters
	params, err := h.parseListDNCParams(r)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PARAMETERS", 
			"Invalid query parameters", err.Error())
		return
	}

	// Convert to service search criteria
	searchCriteria, err := params.ToSearchCriteria()
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_CRITERIA", 
			"Invalid search criteria", err.Error())
		return
	}

	// Search DNC entries
	searchResponse, err := h.dncService.SearchSuppressions(ctx, *searchCriteria)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search DNC entries")
		return
	}

	// Convert to response DTO
	response := h.convertSearchResultsToResponse(searchResponse, params)

	// Add HATEOAS links for pagination
	links := h.generatePaginationLinks("/api/v1/dnc/entries", params, searchResponse.TotalCount)

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleUpdateDNCEntry updates an existing DNC entry
// @Summary Update DNC entry
// @Description Updates an existing DNC entry with new information and maintains audit trail
// @Tags DNC
// @Accept json
// @Produce json
// @Param id path string true "DNC entry ID"
// @Param request body UpdateDNCEntryRequest true "Update request"
// @Success 200 {object} ResponseEnvelope{data=DNCEntryResponse} "Updated DNC entry"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 404 {object} ResponseEnvelope{error=ErrorResponse} "DNC entry not found"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/entries/{id} [put]
func (h *DNCHandler) handleUpdateDNCEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Extract entry ID from path
	entryIDStr := r.PathValue("id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_ENTRY_ID", 
			"Entry ID must be a valid UUID", err.Error())
		return
	}

	// Parse and validate request
	var req UpdateDNCEntryRequest
	if err := h.ParseJSONRequest(r, &req); err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
			"Failed to parse update request", err.Error())
		return
	}

	// Convert to service request
	serviceReq, err := req.ToUpdateSuppressionRequest(entryID, requestMeta.UserID)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "CONVERSION_ERROR", 
			"Failed to convert request", err.Error())
		return
	}

	// Update suppression entry
	response, err := h.dncService.UpdateSuppressionEntry(ctx, *serviceReq)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update DNC entry")
		return
	}

	// Convert to response DTO
	responseDTO := h.convertSuppressionToResponse(response)

	// Add HATEOAS links
	links := map[string]string{
		"self":   fmt.Sprintf("/api/v1/dnc/entries/%s", entryID.String()),
		"delete": fmt.Sprintf("/api/v1/dnc/entries/%s", entryID.String()),
		"list":   "/api/v1/dnc/entries",
	}

	h.WriteSuccessResponse(w, http.StatusOK, responseDTO, requestMeta, links)
}

// handleRemoveSuppression removes a phone number from suppression list
// @Summary Remove phone number from suppression list
// @Description Removes a phone number from internal suppression list with audit trail
// @Tags DNC
// @Accept json
// @Produce json
// @Param id path string true "DNC entry ID"
// @Param reason query string true "Removal reason"
// @Success 204 "Suppression entry removed"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 404 {object} ResponseEnvelope{error=ErrorResponse} "DNC entry not found"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/entries/{id} [delete]
func (h *DNCHandler) handleRemoveSuppression(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Extract entry ID from path
	entryIDStr := r.PathValue("id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_ENTRY_ID", 
			"Entry ID must be a valid UUID", err.Error())
		return
	}

	// Get removal reason from query parameter
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		h.WriteErrorResponse(w, http.StatusBadRequest, "MISSING_REASON", 
			"Removal reason is required", "")
		return
	}

	// First get the entry to obtain the phone number
	entry, err := h.dncService.GetSuppressionEntry(ctx, entryID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get DNC entry")
		return
	}

	// Remove from suppression list
	err = h.dncService.RemoveFromSuppressionList(ctx, entry.PhoneNumber(), requestMeta.UserID, reason)
	if err != nil {
		h.handleServiceError(w, err, "Failed to remove suppression")
		return
	}

	// Return 204 No Content for successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// handleListProviders lists all DNC providers and their status
// @Summary List DNC providers
// @Description Retrieves all configured DNC providers with their current status and health information
// @Tags DNC
// @Produce json
// @Success 200 {object} ResponseEnvelope{data=[]DNCProviderResponse} "DNC providers list"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/providers [get]
func (h *DNCHandler) handleListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Get all providers with status
	providers, err := h.dncService.ListProviders(ctx)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list providers")
		return
	}

	// Convert to response DTOs
	response := make([]DNCProviderResponse, len(providers))
	for i, provider := range providers {
		response[i] = h.convertProviderToResponse(provider)
	}

	// Add HATEOAS links
	links := map[string]string{
		"self": "/api/v1/dnc/providers",
		"sync": "/api/v1/dnc/providers/sync",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleSyncProvider synchronizes data from a specific DNC provider
// @Summary Synchronize DNC provider data
// @Description Triggers synchronization of DNC data from a specific provider
// @Tags DNC
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Param request body ProviderSyncRequest false "Sync options"
// @Success 202 {object} ResponseEnvelope{data=ProviderSyncResponse} "Sync initiated"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 404 {object} ResponseEnvelope{error=ErrorResponse} "Provider not found"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/providers/{id}/sync [post]
func (h *DNCHandler) handleSyncProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Extract provider ID from path
	providerIDStr := r.PathValue("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PROVIDER_ID", 
			"Provider ID must be a valid UUID", err.Error())
		return
	}

	// Parse optional sync request body
	var req ProviderSyncRequest
	if r.ContentLength > 0 {
		if err := h.ParseJSONRequest(r, &req); err != nil {
			h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
				"Failed to parse sync request", err.Error())
			return
		}
	}
	req.ProviderID = providerID // Ensure consistency

	// Initiate provider sync
	syncResponse, err := h.dncService.SyncWithProvider(ctx, providerID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to sync provider")
		return
	}

	// Convert to response DTO
	response := h.convertProviderSyncToResponse(syncResponse)

	// Add HATEOAS links
	links := map[string]string{
		"self":     fmt.Sprintf("/api/v1/dnc/providers/%s/sync", providerID.String()),
		"status":   fmt.Sprintf("/api/v1/dnc/providers/%s/status", providerID.String()),
		"provider": fmt.Sprintf("/api/v1/dnc/providers/%s", providerID.String()),
	}

	h.WriteSuccessResponse(w, http.StatusAccepted, response, requestMeta, links)
}

// handleProviderStatus gets the current status of a DNC provider
// @Summary Get DNC provider status
// @Description Retrieves current health and synchronization status of a DNC provider
// @Tags DNC
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} ResponseEnvelope{data=ProviderStatusResponse} "Provider status"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 404 {object} ResponseEnvelope{error=ErrorResponse} "Provider not found"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/providers/{id}/status [get]
func (h *DNCHandler) handleProviderStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Extract provider ID from path
	providerIDStr := r.PathValue("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PROVIDER_ID", 
			"Provider ID must be a valid UUID", err.Error())
		return
	}

	// Get provider status
	status, err := h.dncService.GetProviderStatus(ctx, providerID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get provider status")
		return
	}

	// Convert to response DTO
	response := h.convertProviderStatusToResponse(status)

	// Add HATEOAS links
	links := map[string]string{
		"self":     fmt.Sprintf("/api/v1/dnc/providers/%s/status", providerID.String()),
		"provider": fmt.Sprintf("/api/v1/dnc/providers/%s", providerID.String()),
		"sync":     fmt.Sprintf("/api/v1/dnc/providers/%s/sync", providerID.String()),
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleComplianceReport generates DNC compliance reports
// @Summary Generate DNC compliance report
// @Description Generates comprehensive compliance reports with filtering and analysis
// @Tags DNC
// @Produce json
// @Param start_date query string false "Start date (RFC3339 format)"
// @Param end_date query string false "End date (RFC3339 format)"
// @Param format query string false "Report format (summary, detailed)"
// @Param include_violations query bool false "Include compliance violations"
// @Param include_stats query bool false "Include statistics"
// @Success 200 {object} ResponseEnvelope{data=ComplianceReportResponse} "Compliance report"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request parameters"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/compliance/report [get]
func (h *DNCHandler) handleComplianceReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse report parameters
	params, err := h.parseComplianceReportParams(r)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_PARAMETERS", 
			"Invalid report parameters", err.Error())
		return
	}

	// Convert to service criteria
	criteria, err := params.ToReportCriteria()
	if err != nil {
		h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_CRITERIA", 
			"Invalid report criteria", err.Error())
		return
	}

	// Generate compliance report
	report, err := h.dncService.GetComplianceReport(ctx, *criteria)
	if err != nil {
		h.handleServiceError(w, err, "Failed to generate compliance report")
		return
	}

	// Convert to response DTO
	response := h.convertComplianceReportToResponse(report)

	// Add HATEOAS links
	links := map[string]string{
		"self": "/api/v1/dnc/compliance/report",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleDNCHealth checks the health of DNC service and dependencies
// @Summary DNC service health check
// @Description Performs comprehensive health check of DNC service and all dependencies
// @Tags DNC
// @Produce json
// @Success 200 {object} ResponseEnvelope{data=HealthResponse} "Service is healthy"
// @Failure 503 {object} ResponseEnvelope{data=HealthResponse} "Service is unhealthy"
// @Router /api/v1/dnc/health [get]
func (h *DNCHandler) handleDNCHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Perform health check
	health, err := h.dncService.HealthCheck(ctx)
	if err != nil {
		h.WriteErrorResponse(w, http.StatusServiceUnavailable, "HEALTH_CHECK_FAILED", 
			"Health check failed", err.Error())
		return
	}

	// Convert to response DTO
	response := h.convertHealthToResponse(health)

	// Determine status code based on health
	statusCode := http.StatusOK
	if !health.IsHealthy() {
		statusCode = http.StatusServiceUnavailable
	}

	// Add HATEOAS links
	links := map[string]string{
		"self":  "/api/v1/dnc/health",
		"cache": "/api/v1/dnc/cache/stats",
	}

	h.WriteSuccessResponse(w, statusCode, response, requestMeta, links)
}

// handleClearCache clears DNC cache with optional pattern matching
// @Summary Clear DNC cache
// @Description Clears DNC cache entries with optional pattern matching for selective clearing
// @Tags DNC
// @Accept json
// @Produce json
// @Param request body ClearCacheRequest false "Cache clear options"
// @Success 200 {object} ResponseEnvelope{data=ClearCacheResponse} "Cache cleared"
// @Failure 400 {object} ResponseEnvelope{error=ErrorResponse} "Invalid request"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/cache/clear [post]
func (h *DNCHandler) handleClearCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Parse optional request body
	var req ClearCacheRequest
	if r.ContentLength > 0 {
		if err := h.ParseJSONRequest(r, &req); err != nil {
			h.WriteErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", 
				"Failed to parse cache clear request", err.Error())
			return
		}
	}

	// Clear cache with pattern
	pattern := req.Pattern
	if pattern == "" {
		pattern = "*" // Clear all by default
	}

	err := h.dncService.ClearCache(ctx, pattern)
	if err != nil {
		h.handleServiceError(w, err, "Failed to clear cache")
		return
	}

	// Create response
	response := ClearCacheResponse{
		Pattern:   pattern,
		Timestamp: time.Now(),
		Success:   true,
	}

	// Add HATEOAS links
	links := map[string]string{
		"self":  "/api/v1/dnc/cache/clear",
		"stats": "/api/v1/dnc/cache/stats",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// handleCacheStats returns DNC cache statistics
// @Summary Get DNC cache statistics
// @Description Retrieves detailed statistics about DNC cache performance and utilization
// @Tags DNC
// @Produce json
// @Success 200 {object} ResponseEnvelope{data=CacheStatsResponse} "Cache statistics"
// @Failure 429 {object} ResponseEnvelope{error=ErrorResponse} "Rate limit exceeded"
// @Failure 500 {object} ResponseEnvelope{error=ErrorResponse} "Internal server error"
// @Router /api/v1/dnc/cache/stats [get]
func (h *DNCHandler) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestMeta := h.ExtractRequestMeta(r)

	// Get cache statistics
	stats, err := h.dncService.GetCacheStats(ctx)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get cache stats")
		return
	}

	// Convert to response DTO
	response := h.convertCacheStatsToResponse(stats)

	// Add HATEOAS links
	links := map[string]string{
		"self":  "/api/v1/dnc/cache/stats",
		"clear": "/api/v1/dnc/cache/clear",
	}

	h.WriteSuccessResponse(w, http.StatusOK, response, requestMeta, links)
}

// Helper methods for request parsing and validation

func (h *DNCHandler) parseListDNCParams(r *http.Request) (*ListDNCParams, error) {
	query := r.URL.Query()
	
	params := &ListDNCParams{
		Page:      1,
		Limit:     50,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	// Parse pagination
	if pageStr := query.Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			params.Page = page
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 1000 {
			params.Limit = limit
		}
	}

	// Parse filters
	if source := query.Get("source"); source != "" {
		params.Source = &source
	}

	if phone := query.Get("phone"); phone != "" {
		params.Phone = &phone
	}

	if status := query.Get("status"); status != "" {
		params.Status = &status
	}

	// Parse sorting
	if sortBy := query.Get("sort_by"); sortBy != "" {
		validSortFields := map[string]bool{
			"created_at":   true,
			"phone_number": true,
			"expires_at":   true,
			"list_source":  true,
		}
		if validSortFields[sortBy] {
			params.SortBy = sortBy
		}
	}

	if sortOrder := query.Get("sort_order"); sortOrder != "" {
		if sortOrder == "asc" || sortOrder == "desc" {
			params.SortOrder = sortOrder
		}
	}

	return params, nil
}

func (h *DNCHandler) parseComplianceReportParams(r *http.Request) (*ComplianceReportParams, error) {
	query := r.URL.Query()
	
	params := &ComplianceReportParams{
		Format:           "summary",
		IncludeViolations: false,
		IncludeStats:     true,
	}

	// Parse date range
	if startStr := query.Get("start_date"); startStr != "" {
		if start, err := time.Parse(time.RFC3339, startStr); err == nil {
			params.StartDate = &start
		} else {
			return nil, fmt.Errorf("invalid start_date format: %s", startStr)
		}
	}

	if endStr := query.Get("end_date"); endStr != "" {
		if end, err := time.Parse(time.RFC3339, endStr); err == nil {
			params.EndDate = &end
		} else {
			return nil, fmt.Errorf("invalid end_date format: %s", endStr)
		}
	}

	// Parse format
	if format := query.Get("format"); format != "" {
		if format == "summary" || format == "detailed" {
			params.Format = format
		}
	}

	// Parse boolean flags
	if violationsStr := query.Get("include_violations"); violationsStr != "" {
		params.IncludeViolations = violationsStr == "true"
	}

	if statsStr := query.Get("include_stats"); statsStr != "" {
		params.IncludeStats = statsStr == "true"
	}

	return params, nil
}

// Helper method to generate pagination links
func (h *DNCHandler) generatePaginationLinks(baseURL string, params *ListDNCParams, totalCount int) map[string]string {
	links := make(map[string]string)
	
	// Calculate total pages
	totalPages := (totalCount + params.Limit - 1) / params.Limit
	
	// Self link
	links["self"] = fmt.Sprintf("%s?page=%d&limit=%d", baseURL, params.Page, params.Limit)
	
	// First and last
	links["first"] = fmt.Sprintf("%s?page=1&limit=%d", baseURL, params.Limit)
	links["last"] = fmt.Sprintf("%s?page=%d&limit=%d", baseURL, totalPages, params.Limit)
	
	// Previous and next
	if params.Page > 1 {
		links["prev"] = fmt.Sprintf("%s?page=%d&limit=%d", baseURL, params.Page-1, params.Limit)
	}
	if params.Page < totalPages {
		links["next"] = fmt.Sprintf("%s?page=%d&limit=%d", baseURL, params.Page+1, params.Limit)
	}
	
	return links
}

// Helper method to handle service errors consistently
func (h *DNCHandler) handleServiceError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.IsValidationError(err):
		h.WriteErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", message, err.Error())
	case errors.IsNotFoundError(err):
		h.WriteErrorResponse(w, http.StatusNotFound, "NOT_FOUND", message, err.Error())
	case errors.IsConflictError(err):
		h.WriteErrorResponse(w, http.StatusConflict, "CONFLICT", message, err.Error())
	case errors.IsComplianceError(err):
		h.WriteErrorResponse(w, http.StatusUnprocessableEntity, "COMPLIANCE_ERROR", message, err.Error())
	case errors.IsExternalServiceError(err):
		h.WriteErrorResponse(w, http.StatusBadGateway, "EXTERNAL_SERVICE_ERROR", message, err.Error())
	default:
		h.WriteErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", message, err.Error())
	}
}

// Custom validator for E.164 phone numbers
func validateE164PhoneNumber(fl validator.FieldLevel) bool {
	phoneStr := fl.Field().String()
	if phoneStr == "" {
		return false
	}
	
	// Basic E.164 validation
	if !strings.HasPrefix(phoneStr, "+") {
		return false
	}
	
	// Remove the + and check if all remaining characters are digits
	digits := phoneStr[1:]
	if len(digits) < 7 || len(digits) > 15 {
		return false
	}
	
	for _, char := range digits {
		if char < '0' || char > '9' {
			return false
		}
	}
	
	return true
}

// Response conversion methods

func (h *DNCHandler) convertDNCCheckToResponse(result *dncService.DNCCheckResponse) DNCCheckResponse {
	return DNCCheckResponse{
		PhoneHash:        generatePhoneHash(result.PhoneNumber.String()),
		IsBlocked:        result.IsBlocked,
		BlockReason:      h.convertBlockReason(result.BlockReason),
		ListSources:      result.ListSources,
		ComplianceLevel:  result.ComplianceLevel,
		CheckedAt:        result.CheckedAt,
		ExpiresAt:        result.ExpiresAt,
		RiskScore:        result.RiskScore,
		Confidence:       result.Confidence,
		CacheHit:         result.CacheHit,
		ResponseTimeMs:   result.ResponseTimeMs,
		ProviderResults:  h.convertProviderResults(result.ProviderResults),
		Metadata:         filterSensitiveMetadata(result.Metadata),
	}
}

func (h *DNCHandler) convertBulkDNCCheckToResponse(results []*dncService.DNCCheckResponse, includeDetails bool) BulkDNCCheckResponse {
	response := BulkDNCCheckResponse{
		TotalRequested: len(results),
		TotalBlocked:   0,
		Results:        make([]DNCCheckResponse, len(results)),
		ProcessedAt:    time.Now(),
	}

	for i, result := range results {
		response.Results[i] = h.convertDNCCheckToResponse(result)
		if result.IsBlocked {
			response.TotalBlocked++
		}
	}

	if includeDetails {
		response.Summary = &BulkCheckSummary{
			BlockedCount:      response.TotalBlocked,
			AllowedCount:      response.TotalRequested - response.TotalBlocked,
			AverageRiskScore:  h.calculateAverageRiskScore(results),
			TotalResponseTime: h.calculateTotalResponseTime(results),
		}
	}

	return response
}

func (h *DNCHandler) convertSuppressionToResponse(response *dncService.SuppressionResponse) DNCEntryResponse {
	entry := response.Entry
	return DNCEntryResponse{
		ID:              entry.ID().String(),
		PhoneHash:       generatePhoneHash(entry.PhoneNumber().String()),
		ListSource:      entry.ListSource(),
		SuppressReason:  entry.SuppressReason(),
		Status:          string(entry.Status()),
		CreatedAt:       entry.CreatedAt(),
		UpdatedAt:       entry.UpdatedAt(),
		ExpiresAt:       entry.ExpiresAt(),
		SourceReference: entry.SourceReference(),
		AddedBy:         entry.AddedBy().String(),
		Metadata:        filterSensitiveMetadata(entry.Metadata()),
	}
}

func (h *DNCHandler) convertSearchResultsToResponse(searchResponse *dncService.SearchResponse, params *ListDNCParams) PaginatedDNCEntriesResponse {
	entries := make([]DNCEntryResponse, len(searchResponse.Entries))
	for i, entry := range searchResponse.Entries {
		entries[i] = h.convertEntryToResponse(entry)
	}

	return PaginatedDNCEntriesResponse{
		Entries:     entries,
		TotalCount:  searchResponse.TotalCount,
		Page:        params.Page,
		Limit:       params.Limit,
		TotalPages:  (searchResponse.TotalCount + params.Limit - 1) / params.Limit,
		HasNext:     params.Page*params.Limit < searchResponse.TotalCount,
		HasPrevious: params.Page > 1,
	}
}

func (h *DNCHandler) convertEntryToResponse(entry *dnc.DNCEntry) DNCEntryResponse {
	return DNCEntryResponse{
		ID:              entry.ID().String(),
		PhoneHash:       generatePhoneHash(entry.PhoneNumber().String()),
		ListSource:      entry.ListSource(),
		SuppressReason:  entry.SuppressReason(),
		Status:          string(entry.Status()),
		CreatedAt:       entry.CreatedAt(),
		UpdatedAt:       entry.UpdatedAt(),
		ExpiresAt:       entry.ExpiresAt(),
		SourceReference: entry.SourceReference(),
		AddedBy:         entry.AddedBy().String(),
		Metadata:        filterSensitiveMetadata(entry.Metadata()),
	}
}

func (h *DNCHandler) convertProviderToResponse(provider *dnc.DNCProvider) DNCProviderResponse {
	return DNCProviderResponse{
		ID:                provider.ID().String(),
		Name:              provider.Name(),
		Type:              string(provider.Type()),
		Status:            string(provider.Status()),
		BaseURL:           provider.BaseURL(),
		AuthType:          string(provider.AuthType()),
		IsEnabled:         provider.IsEnabled(),
		LastSyncAt:        provider.LastSyncAt(),
		SyncInterval:      int(provider.SyncInterval().Seconds()),
		EntriesCount:      provider.EntriesCount(),
		SuccessRate:       provider.SuccessRate(),
		AverageLatency:    int(provider.AverageLatency().Milliseconds()),
		Configuration:     filterSensitiveConfig(provider.Configuration()),
		HealthStatus:      string(provider.HealthStatus()),
		ErrorCount:        provider.ErrorCount(),
		LastError:         provider.LastError(),
		CreatedAt:         provider.CreatedAt(),
		UpdatedAt:         provider.UpdatedAt(),
	}
}

func (h *DNCHandler) convertProviderSyncToResponse(response *dncService.ProviderSyncResponse) ProviderSyncResponse {
	return ProviderSyncResponse{
		ProviderID:     response.ProviderID.String(),
		Status:         response.Status,
		StartedAt:      response.StartedAt,
		CompletedAt:    response.CompletedAt,
		Duration:       response.Duration,
		RecordsAdded:   response.RecordsAdded,
		RecordsUpdated: response.RecordsUpdated,
		RecordsRemoved: response.RecordsRemoved,
		ErrorCount:     response.ErrorCount,
		LastError:      response.LastError,
		NextSyncAt:     response.NextSyncAt,
		Metadata:       response.Metadata,
	}
}

func (h *DNCHandler) convertProviderStatusToResponse(status *dncService.ProviderStatusResponse) ProviderStatusResponse {
	return ProviderStatusResponse{
		ProviderID:        status.ProviderID.String(),
		Name:              status.Name,
		Status:            status.Status,
		IsHealthy:         status.IsHealthy,
		LastCheckAt:       status.LastCheckAt,
		ResponseTime:      status.ResponseTime,
		SuccessRate:       status.SuccessRate,
		ErrorCount:        status.ErrorCount,
		CircuitState:      status.CircuitState,
		LastSyncAt:        status.LastSyncAt,
		NextSyncAt:        status.NextSyncAt,
		EntriesCount:      status.EntriesCount,
		Configuration:     filterSensitiveConfig(status.Configuration),
		Metrics:           status.Metrics,
		Warnings:          status.Warnings,
	}
}

func (h *DNCHandler) convertComplianceReportToResponse(report *dncService.ComplianceReportResponse) ComplianceReportResponse {
	return ComplianceReportResponse{
		ReportID:          report.ReportID.String(),
		GeneratedAt:       report.GeneratedAt,
		Period:            report.Period,
		Summary:           h.convertComplianceSummary(report.Summary),
		Violations:        h.convertViolations(report.Violations),
		Statistics:        report.Statistics,
		Recommendations:   report.Recommendations,
		Metadata:          report.Metadata,
	}
}

func (h *DNCHandler) convertHealthToResponse(health *dncService.HealthResponse) HealthResponse {
	return HealthResponse{
		Status:      health.Status,
		Timestamp:   health.Timestamp,
		Version:     health.Version,
		Uptime:      health.Uptime,
		Checks:      health.Checks,
		Metrics:     health.Metrics,
		Dependencies: health.Dependencies,
	}
}

func (h *DNCHandler) convertCacheStatsToResponse(stats *dncService.CacheStatsResponse) CacheStatsResponse {
	return CacheStatsResponse{
		HitRate:       stats.HitRate,
		MissRate:      stats.MissRate,
		TotalKeys:     stats.TotalKeys,
		MemoryUsage:   stats.MemoryUsage,
		Evictions:     stats.Evictions,
		LastUpdated:   stats.LastUpdated,
		TTLDistribution: stats.TTLDistribution,
		PerformanceMetrics: stats.PerformanceMetrics,
	}
}

// Helper conversion methods

func (h *DNCHandler) convertBlockReason(reason *dncService.DNCBlockReason) *DNCBlockReasonResponse {
	if reason == nil {
		return nil
	}
	
	return &DNCBlockReasonResponse{
		Code:        reason.Code,
		Description: reason.Description,
		Source:      reason.Source,
		RuleID:      reason.RuleID,
		Severity:    reason.Severity,
		Timestamp:   reason.Timestamp,
		Metadata:    filterSensitiveMetadata(reason.Metadata),
	}
}

func (h *DNCHandler) convertProviderResults(results []dncService.ProviderResult) []ProviderResultResponse {
	responses := make([]ProviderResultResponse, len(results))
	for i, result := range results {
		responses[i] = ProviderResultResponse{
			ProviderID:   result.ProviderID.String(),
			Status:       result.Status,
			IsBlocked:    result.IsBlocked,
			ResponseTime: result.ResponseTime,
			Error:        result.Error,
			CacheHit:     result.CacheHit,
		}
	}
	return responses
}

func (h *DNCHandler) convertComplianceSummary(summary dncService.ComplianceSummary) ComplianceSummaryResponse {
	return ComplianceSummaryResponse{
		TotalChecks:       summary.TotalChecks,
		BlockedCalls:      summary.BlockedCalls,
		AllowedCalls:      summary.AllowedCalls,
		ViolationCount:    summary.ViolationCount,
		ComplianceRate:    summary.ComplianceRate,
		RiskScore:         summary.RiskScore,
		TopViolationTypes: summary.TopViolationTypes,
	}
}

func (h *DNCHandler) convertViolations(violations []dncService.ComplianceViolation) []ComplianceViolationResponse {
	responses := make([]ComplianceViolationResponse, len(violations))
	for i, violation := range violations {
		responses[i] = ComplianceViolationResponse{
			ID:          violation.ID.String(),
			Type:        violation.Type,
			Severity:    violation.Severity,
			Description: violation.Description,
			PhoneHash:   generatePhoneHash(violation.PhoneNumber),
			Timestamp:   violation.Timestamp,
			Resolution:  violation.Resolution,
			Metadata:    filterSensitiveMetadata(violation.Metadata),
		}
	}
	return responses
}

// Utility methods

func (h *DNCHandler) calculateAverageRiskScore(results []*dncService.DNCCheckResponse) float64 {
	if len(results) == 0 {
		return 0.0
	}
	
	var total float64
	for _, result := range results {
		total += result.RiskScore
	}
	return total / float64(len(results))
}

func (h *DNCHandler) calculateTotalResponseTime(results []*dncService.DNCCheckResponse) int {
	var total int
	for _, result := range results {
		total += result.ResponseTimeMs
	}
	return total
}

func generatePhoneHash(phoneNumber string) string {
	hasher := sha256.New()
	hasher.Write([]byte(phoneNumber))
	return fmt.Sprintf("ph_%x", hasher.Sum(nil)[:8])
}

func filterSensitiveMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	
	filtered := make(map[string]string)
	for key, value := range metadata {
		if !isSensitiveMetadataKey(key) {
			filtered[key] = value
		}
	}
	return filtered
}

func filterSensitiveConfig(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return nil
	}
	
	filtered := make(map[string]interface{})
	for key, value := range config {
		if !isSensitiveConfigKey(key) {
			filtered[key] = value
		} else {
			filtered[key] = "[REDACTED]"
		}
	}
	return filtered
}

func isSensitiveMetadataKey(key string) bool {
	sensitiveKeys := []string{
		"password", "secret", "key", "token", "credential",
		"ssn", "social", "dob", "birth", "address", "email",
		"internal_id", "customer_id", "account_number",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

func isSensitiveConfigKey(key string) bool {
	sensitiveKeys := []string{
		"password", "secret", "key", "token", "credential",
		"api_key", "auth", "cert", "private",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}