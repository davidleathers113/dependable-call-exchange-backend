package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// QueryRequest represents a data query request
type QueryRequest struct {
	Entity string                 // Table/entity name
	Filter string                 // SQL-like filter expression
	Sort   string                 // Sort expression (e.g., "created_at DESC")
	Limit  int                    // Maximum number of records
	Offset int                    // Number of records to skip
	Fields []string               // Specific fields to select
	Params map[string]interface{} // Query parameters
}

// QueryResult represents a single query result record
type QueryResult map[string]interface{}

// QueryService provides data querying capabilities for audit exports
type QueryService struct {
	// In a real implementation, this would contain database connections
	// and repository interfaces for different entities
	repositories map[string]Repository
}

// Repository interface for data access
type Repository interface {
	Query(ctx context.Context, req QueryRequest) ([]QueryResult, error)
	Count(ctx context.Context, req QueryRequest) (int64, error)
}

// NewQueryService creates a new query service
func NewQueryService() *QueryService {
	return &QueryService{
		repositories: make(map[string]Repository),
	}
}

// RegisterRepository registers a repository for an entity
func (s *QueryService) RegisterRepository(entity string, repo Repository) {
	s.repositories[entity] = repo
}

// Query executes a query and returns results
func (s *QueryService) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Validate request
	if err := s.validateQueryRequest(req); err != nil {
		return nil, err
	}

	// Get repository for entity
	repo, exists := s.repositories[req.Entity]
	if !exists {
		return nil, errors.NewNotFoundError(fmt.Sprintf("no repository registered for entity %s", req.Entity))
	}

	// Execute query
	return repo.Query(ctx, req)
}

// Count returns the total number of records for a query
func (s *QueryService) Count(ctx context.Context, req QueryRequest) (int64, error) {
	// Create count request (no limit/offset for counting)
	countReq := req
	countReq.Limit = 0
	countReq.Offset = 0

	// Validate request
	if err := s.validateQueryRequest(countReq); err != nil {
		return 0, err
	}

	// Get repository for entity
	repo, exists := s.repositories[req.Entity]
	if !exists {
		return 0, errors.NewNotFoundError(fmt.Sprintf("no repository registered for entity %s", req.Entity))
	}

	// Execute count
	return repo.Count(ctx, countReq)
}

// validateQueryRequest validates a query request
func (s *QueryService) validateQueryRequest(req QueryRequest) error {
	if req.Entity == "" {
		return errors.NewValidationError("INVALID_ENTITY", "entity name is required")
	}

	// Validate entity name to prevent SQL injection
	if !isValidEntityName(req.Entity) {
		return errors.NewValidationError("INVALID_ENTITY", "invalid entity name")
	}

	// Validate filter expression
	if req.Filter != "" {
		if err := validateFilterExpression(req.Filter); err != nil {
			return errors.NewValidationError("INVALID_FILTER", "invalid filter expression").WithCause(err)
		}
	}

	// Validate sort expression
	if req.Sort != "" {
		if err := validateSortExpression(req.Sort); err != nil {
			return errors.NewValidationError("INVALID_SORT", "invalid sort expression").WithCause(err)
		}
	}

	// Validate limits
	if req.Limit < 0 {
		return errors.NewValidationError("INVALID_LIMIT", "limit cannot be negative")
	}

	if req.Offset < 0 {
		return errors.NewValidationError("INVALID_OFFSET", "offset cannot be negative")
	}

	// Maximum limit check to prevent large exports
	if req.Limit > 10000 {
		return errors.NewValidationError("LIMIT_TOO_LARGE", "limit cannot exceed 10000 records per query")
	}

	return nil
}

// isValidEntityName checks if an entity name is valid (alphanumeric + underscore)
func isValidEntityName(name string) bool {
	if name == "" {
		return false
	}

	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_') {
			return false
		}
	}

	return true
}

// validateFilterExpression validates a filter expression for safety
func validateFilterExpression(filter string) error {
	// Basic validation - would need more sophisticated parsing in production

	// Check for dangerous SQL keywords (only as complete words)
	dangerousKeywords := []string{
		"DROP", "DELETE", "INSERT", "UPDATE", "ALTER",
		"EXEC", "EXECUTE", "UNION", "SCRIPT", "DECLARE",
	}

	upperFilter := strings.ToUpper(filter)
	words := strings.Fields(upperFilter)
	for _, word := range words {
		for _, keyword := range dangerousKeywords {
			if word == keyword {
				return fmt.Errorf("dangerous keyword '%s' not allowed in filter", keyword)
			}
		}
	}

	// Check for balanced parentheses
	if !areParenthesesBalanced(filter) {
		return fmt.Errorf("unbalanced parentheses in filter expression")
	}

	return nil
}

// validateSortExpression validates a sort expression
func validateSortExpression(sort string) error {
	// Split by comma for multiple sort fields
	parts := strings.Split(sort, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check format: "field_name ASC|DESC"
		sortParts := strings.Fields(part)
		if len(sortParts) < 1 || len(sortParts) > 2 {
			return fmt.Errorf("invalid sort expression format: %s", part)
		}

		// Validate field name
		if !isValidFieldName(sortParts[0]) {
			return fmt.Errorf("invalid field name in sort: %s", sortParts[0])
		}

		// Validate direction if present
		if len(sortParts) == 2 {
			direction := strings.ToUpper(sortParts[1])
			if direction != "ASC" && direction != "DESC" {
				return fmt.Errorf("invalid sort direction: %s", sortParts[1])
			}
		}
	}

	return nil
}

// isValidFieldName checks if a field name is valid
func isValidFieldName(name string) bool {
	if name == "" {
		return false
	}

	// Allow dots for nested field access
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '.') {
			return false
		}
	}

	return true
}

// areParenthesesBalanced checks if parentheses are balanced in an expression
func areParenthesesBalanced(expr string) bool {
	count := 0
	for _, char := range expr {
		if char == '(' {
			count++
		} else if char == ')' {
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// Mock implementations for demonstration

// MockUserRepository provides mock user data
type MockUserRepository struct{}

func (r *MockUserRepository) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Mock user data
	users := []QueryResult{
		{
			"id":         uuid.New().String(),
			"email":      "john.doe@example.com",
			"phone":      "+1234567890",
			"created_at": time.Now().Add(-30 * 24 * time.Hour),
			"stats": map[string]interface{}{
				"calls_made": 42,
			},
		},
		{
			"id":         uuid.New().String(),
			"email":      "jane.smith@example.com",
			"phone":      "+0987654321",
			"created_at": time.Now().Add(-15 * 24 * time.Hour),
			"stats": map[string]interface{}{
				"calls_made": 18,
			},
		},
	}

	// Apply filters (simplified)
	filtered := r.applyFilters(users, req.Filter, req.Params)

	// Apply sorting (simplified)
	sorted := r.applySorting(filtered, req.Sort)

	// Apply pagination
	return r.applyPagination(sorted, req.Limit, req.Offset), nil
}

func (r *MockUserRepository) Count(ctx context.Context, req QueryRequest) (int64, error) {
	results, err := r.Query(ctx, req)
	if err != nil {
		return 0, err
	}
	return int64(len(results)), nil
}

func (r *MockUserRepository) applyFilters(data []QueryResult, filter string, params map[string]interface{}) []QueryResult {
	if filter == "" {
		return data
	}

	// Simplified filter application - in production would use proper SQL parsing
	var filtered []QueryResult
	for _, record := range data {
		if r.matchesFilter(record, filter, params) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (r *MockUserRepository) matchesFilter(record QueryResult, filter string, params map[string]interface{}) bool {
	// Very simplified filter matching - would need proper SQL parser in production

	// Handle parameter substitution
	processedFilter := filter
	for key, value := range params {
		placeholder := ":" + key
		if strings.Contains(processedFilter, placeholder) {
			processedFilter = strings.ReplaceAll(processedFilter, placeholder, fmt.Sprintf("'%v'", value))
		}
	}

	// Simple equality check for "field = 'value'"
	if strings.Contains(processedFilter, "=") {
		parts := strings.Split(processedFilter, "=")
		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

			if recordValue, exists := record[field]; exists {
				return fmt.Sprintf("%v", recordValue) == value
			}
		}
	}

	return true // Default to include if we can't parse filter
}

func (r *MockUserRepository) applySorting(data []QueryResult, sort string) []QueryResult {
	// Simplified sorting - would need proper implementation in production
	return data
}

func (r *MockUserRepository) applyPagination(data []QueryResult, limit, offset int) []QueryResult {
	start := offset
	if start > len(data) {
		return []QueryResult{}
	}

	end := len(data)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return data[start:end]
}

// MockCallRepository provides mock call data
type MockCallRepository struct{}

func (r *MockCallRepository) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Mock call data
	calls := []QueryResult{
		{
			"id":          uuid.New().String(),
			"user_id":     uuid.New().String(),
			"from_number": "+1234567890",
			"to_number":   "+0987654321",
			"status":      "completed",
			"duration":    "00:05:30",
			"created_at":  time.Now().Add(-2 * time.Hour),
		},
		{
			"id":          uuid.New().String(),
			"user_id":     uuid.New().String(),
			"from_number": "+1234567890",
			"to_number":   "+5555555555",
			"status":      "completed",
			"duration":    "00:02:15",
			"created_at":  time.Now().Add(-1 * time.Hour),
		},
	}

	// Apply filters
	filtered := r.applyFilters(calls, req.Filter, req.Params)

	// Apply pagination
	return r.applyPagination(filtered, req.Limit, req.Offset), nil
}

func (r *MockCallRepository) Count(ctx context.Context, req QueryRequest) (int64, error) {
	results, err := r.Query(ctx, req)
	if err != nil {
		return 0, err
	}
	return int64(len(results)), nil
}

func (r *MockCallRepository) applyFilters(data []QueryResult, filter string, params map[string]interface{}) []QueryResult {
	if filter == "" {
		return data
	}

	var filtered []QueryResult
	for _, record := range data {
		if r.matchesFilter(record, filter, params) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (r *MockCallRepository) matchesFilter(record QueryResult, filter string, params map[string]interface{}) bool {
	// Simple implementation for demonstration
	return true
}

func (r *MockCallRepository) applyPagination(data []QueryResult, limit, offset int) []QueryResult {
	start := offset
	if start > len(data) {
		return []QueryResult{}
	}

	end := len(data)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return data[start:end]
}

// MockConsentRepository provides mock consent data
type MockConsentRepository struct{}

func (r *MockConsentRepository) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Mock consent data
	consents := []QueryResult{
		{
			"id":           uuid.New().String(),
			"user_id":      uuid.New().String(),
			"phone_number": "+1234567890",
			"type":         "marketing",
			"status":       "granted",
			"created_at":   time.Now().Add(-10 * 24 * time.Hour),
			"source":       "web_form",
			"metadata": map[string]interface{}{
				"ip_address": "192.168.1.100",
				"user_agent": "Mozilla/5.0 (compatible)",
			},
		},
		{
			"id":           uuid.New().String(),
			"user_id":      uuid.New().String(),
			"phone_number": "+0987654321",
			"type":         "transactional",
			"status":       "granted",
			"created_at":   time.Now().Add(-5 * 24 * time.Hour),
			"source":       "api",
			"metadata": map[string]interface{}{
				"ip_address": "10.0.0.50",
				"user_agent": "API Client v1.0",
			},
		},
	}

	return r.applyPagination(consents, req.Limit, req.Offset), nil
}

func (r *MockConsentRepository) Count(ctx context.Context, req QueryRequest) (int64, error) {
	results, err := r.Query(ctx, req)
	if err != nil {
		return 0, err
	}
	return int64(len(results)), nil
}

func (r *MockConsentRepository) applyPagination(data []QueryResult, limit, offset int) []QueryResult {
	start := offset
	if start > len(data) {
		return []QueryResult{}
	}

	end := len(data)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return data[start:end]
}

// MockTransactionRepository provides mock transaction data
type MockTransactionRepository struct{}

func (r *MockTransactionRepository) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Mock transaction data
	transactions := []QueryResult{
		{
			"id":         uuid.New().String(),
			"type":       "charge",
			"amount":     "25.50",
			"currency":   "USD",
			"buyer_id":   uuid.New().String(),
			"seller_id":  uuid.New().String(),
			"call_id":    uuid.New().String(),
			"status":     "completed",
			"created_at": time.Now().Add(-1 * time.Hour),
			"audit": map[string]interface{}{
				"user":      "system",
				"action":    "created",
				"timestamp": time.Now().Add(-1 * time.Hour),
			},
		},
		{
			"id":         uuid.New().String(),
			"type":       "payout",
			"amount":     "20.00",
			"currency":   "USD",
			"buyer_id":   uuid.New().String(),
			"seller_id":  uuid.New().String(),
			"call_id":    uuid.New().String(),
			"status":     "pending",
			"created_at": time.Now().Add(-30 * time.Minute),
			"audit": map[string]interface{}{
				"user":      "admin",
				"action":    "approved",
				"timestamp": time.Now().Add(-25 * time.Minute),
			},
		},
	}

	return r.applyPagination(transactions, req.Limit, req.Offset), nil
}

func (r *MockTransactionRepository) Count(ctx context.Context, req QueryRequest) (int64, error) {
	results, err := r.Query(ctx, req)
	if err != nil {
		return 0, err
	}
	return int64(len(results)), nil
}

func (r *MockTransactionRepository) applyPagination(data []QueryResult, limit, offset int) []QueryResult {
	start := offset
	if start > len(data) {
		return []QueryResult{}
	}

	end := len(data)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return data[start:end]
}

// MockSecurityEventRepository provides mock security event data
type MockSecurityEventRepository struct{}

func (r *MockSecurityEventRepository) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	// Mock security event data
	events := []QueryResult{
		{
			"id":         uuid.New().String(),
			"type":       "authentication_failure",
			"severity":   "medium",
			"user_id":    uuid.New().String(),
			"ip_address": "192.168.1.100",
			"action":     "login_attempt",
			"resource":   "/api/v1/auth/login",
			"result":     "failure",
			"created_at": time.Now().Add(-2 * time.Hour),
			"details": map[string]interface{}{
				"reason":     "invalid_credentials",
				"user_agent": "Mozilla/5.0",
				"attempts":   3,
			},
		},
		{
			"id":         uuid.New().String(),
			"type":       "rate_limit_exceeded",
			"severity":   "high",
			"user_id":    nil,
			"ip_address": "10.0.0.100",
			"action":     "api_call",
			"resource":   "/api/v1/calls",
			"result":     "blocked",
			"created_at": time.Now().Add(-1 * time.Hour),
			"details": map[string]interface{}{
				"limit":      100,
				"window":     "1h",
				"violations": 5,
			},
		},
	}

	return r.applyPagination(events, req.Limit, req.Offset), nil
}

func (r *MockSecurityEventRepository) Count(ctx context.Context, req QueryRequest) (int64, error) {
	results, err := r.Query(ctx, req)
	if err != nil {
		return 0, err
	}
	return int64(len(results)), nil
}

func (r *MockSecurityEventRepository) applyPagination(data []QueryResult, limit, offset int) []QueryResult {
	start := offset
	if start > len(data) {
		return []QueryResult{}
	}

	end := len(data)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return data[start:end]
}

// SetupMockQueryService creates a query service with mock repositories for testing
func SetupMockQueryService() *QueryService {
	service := NewQueryService()

	// Register mock repositories
	service.RegisterRepository("users", &MockUserRepository{})
	service.RegisterRepository("calls", &MockCallRepository{})
	service.RegisterRepository("consents", &MockConsentRepository{})
	service.RegisterRepository("tcpa_consents", &MockConsentRepository{})
	service.RegisterRepository("transactions", &MockTransactionRepository{})
	service.RegisterRepository("audit_events", &MockTransactionRepository{})
	service.RegisterRepository("security_events", &MockSecurityEventRepository{})

	return service
}
