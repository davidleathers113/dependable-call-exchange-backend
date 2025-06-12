package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ContractValidationMiddleware validates requests and responses against OpenAPI contract
type ContractValidationMiddleware struct {
	validator *ContractValidator
	config    ContractValidationConfig
	logger    *slog.Logger
}

// ContractValidationConfig configures contract validation behavior
type ContractValidationConfig struct {
	// ValidateRequests enables request validation
	ValidateRequests bool
	
	// ValidateResponses enables response validation
	ValidateResponses bool
	
	// FailOnValidationError determines if validation errors should return HTTP errors
	FailOnValidationError bool
	
	// LogValidationErrors enables logging of validation errors
	LogValidationErrors bool
	
	// SkipValidationForPaths lists paths to skip validation for
	SkipValidationForPaths []string
	
	// OnlyValidateContentTypes restricts validation to specific content types
	OnlyValidateContentTypes []string
}

// DefaultContractValidationConfig returns sensible defaults
func DefaultContractValidationConfig() ContractValidationConfig {
	return ContractValidationConfig{
		ValidateRequests:         true,
		ValidateResponses:        false, // Response validation can be expensive
		FailOnValidationError:    false, // Log but don't fail in production
		LogValidationErrors:      true,
		SkipValidationForPaths:   []string{"/health", "/metrics", "/ready"},
		OnlyValidateContentTypes: []string{"application/json"},
	}
}

// NewContractValidationMiddleware creates a new contract validation middleware
func NewContractValidationMiddleware(specPath string, config ContractValidationConfig, logger *slog.Logger) (*ContractValidationMiddleware, error) {
	validator, err := NewContractValidator(specPath)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &ContractValidationMiddleware{
		validator: validator,
		config:    config,
		logger:    logger,
	}, nil
}

// Middleware returns the HTTP middleware function
func (cvm *ContractValidationMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should skip validation for this path
			if cvm.shouldSkipValidation(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Add request validation
			if cvm.config.ValidateRequests {
				if err := cvm.validateRequest(r); err != nil {
					cvm.handleValidationError(w, r, "request", err)
					if cvm.config.FailOnValidationError {
						return
					}
				}
			}

			// Wrap response writer for response validation
			if cvm.config.ValidateResponses {
				wrappedWriter := &contractResponseWriter{
					ResponseWriter: w,
					request:        r,
					validator:      cvm,
				}
				next.ServeHTTP(wrappedWriter, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// shouldSkipValidation checks if validation should be skipped for this request
func (cvm *ContractValidationMiddleware) shouldSkipValidation(r *http.Request) bool {
	path := r.URL.Path

	// Skip if path is in skip list
	for _, skipPath := range cvm.config.SkipValidationForPaths {
		if path == skipPath || strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	// Skip if content type should not be validated
	if len(cvm.config.OnlyValidateContentTypes) > 0 {
		contentType := r.Header.Get("Content-Type")
		shouldValidate := false
		for _, validType := range cvm.config.OnlyValidateContentTypes {
			if strings.HasPrefix(contentType, validType) {
				shouldValidate = true
				break
			}
		}
		if !shouldValidate && r.ContentLength > 0 {
			return true
		}
	}

	return false
}

// validateRequest validates the incoming request
func (cvm *ContractValidationMiddleware) validateRequest(r *http.Request) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		cvm.logger.Debug("contract validation completed",
			"duration", duration,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}()

	return cvm.validator.ValidateRequest(r)
}

// handleValidationError handles validation errors
func (cvm *ContractValidationMiddleware) handleValidationError(w http.ResponseWriter, r *http.Request, validationType string, err error) {
	if cvm.config.LogValidationErrors {
		validationErrors := ParseValidationError(err)
		
		cvm.logger.Error("contract validation failed",
			"type", validationType,
			"method", r.Method,
			"path", r.URL.Path,
			"errors", validationErrors,
			"user_agent", r.Header.Get("User-Agent"),
			"remote_addr", r.RemoteAddr,
		)
	}

	if cvm.config.FailOnValidationError {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		
		response := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "CONTRACT_VALIDATION_ERROR",
				"message": "Request does not conform to API contract",
				"details": ParseValidationError(err),
			},
		}
		
		json.NewEncoder(w).Encode(response)
	}
}

// contractResponseWriter wraps http.ResponseWriter to capture response for validation
type contractResponseWriter struct {
	http.ResponseWriter
	request       *http.Request
	validator     *ContractValidationMiddleware
	statusCode    int
	responseBody  bytes.Buffer
	headerWritten bool
}

// WriteHeader captures the status code
func (crw *contractResponseWriter) WriteHeader(statusCode int) {
	crw.statusCode = statusCode
	crw.headerWritten = true
	crw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body and validates it
func (crw *contractResponseWriter) Write(data []byte) (int, error) {
	// Capture response body
	crw.responseBody.Write(data)
	
	// If this is the end of the response, validate it
	if !crw.headerWritten {
		crw.statusCode = http.StatusOK
	}
	
	// Write to actual response
	n, err := crw.ResponseWriter.Write(data)
	
	// Validate response if we have a complete response
	if err == nil {
		crw.validateResponse()
	}
	
	return n, err
}

// validateResponse validates the captured response
func (crw *contractResponseWriter) validateResponse() {
	// Create a mock response for validation
	resp := &http.Response{
		StatusCode: crw.statusCode,
		Header:     crw.Header(),
		Body:       io.NopCloser(bytes.NewReader(crw.responseBody.Bytes())),
		Request:    crw.request,
	}

	if err := crw.validator.validator.ValidateResponse(crw.request, resp); err != nil {
		crw.validator.handleValidationError(crw.ResponseWriter, crw.request, "response", err)
	}
}

// ContractValidationMetrics tracks validation metrics
type ContractValidationMetrics struct {
	RequestValidationCount    int64
	RequestValidationErrors   int64
	ResponseValidationCount   int64
	ResponseValidationErrors  int64
	ValidationDurationTotal   time.Duration
	SkippedValidationCount    int64
}

// MetricsCollector collects contract validation metrics
type MetricsCollector struct {
	metrics ContractValidationMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// GetMetrics returns current metrics
func (mc *MetricsCollector) GetMetrics() ContractValidationMetrics {
	return mc.metrics
}

// WithMetrics adds metrics collection to contract validation middleware
func (cvm *ContractValidationMiddleware) WithMetrics(collector *MetricsCollector) *ContractValidationMiddleware {
	// In a full implementation, this would wire up metrics collection
	return cvm
}

// ContractValidationReporter generates validation reports
type ContractValidationReporter struct {
	violations []ContractViolation
	logger     *slog.Logger
}

// ContractViolation represents a contract violation
type ContractViolation struct {
	Timestamp    time.Time                `json:"timestamp"`
	Method       string                   `json:"method"`
	Path         string                   `json:"path"`
	Type         string                   `json:"type"` // "request" or "response"
	StatusCode   int                      `json:"status_code,omitempty"`
	Errors       []ContractValidationError `json:"errors"`
	RequestID    string                   `json:"request_id,omitempty"`
	UserAgent    string                   `json:"user_agent,omitempty"`
	RemoteAddr   string                   `json:"remote_addr,omitempty"`
	Headers      map[string][]string      `json:"headers,omitempty"`
	Body         interface{}              `json:"body,omitempty"`
}

// NewContractValidationReporter creates a new validation reporter
func NewContractValidationReporter(logger *slog.Logger) *ContractValidationReporter {
	return &ContractValidationReporter{
		violations: make([]ContractViolation, 0),
		logger:     logger,
	}
}

// ReportViolation records a contract violation
func (cvr *ContractValidationReporter) ReportViolation(violation ContractViolation) {
	cvr.violations = append(cvr.violations, violation)
	
	cvr.logger.Warn("contract violation detected",
		"method", violation.Method,
		"path", violation.Path,
		"type", violation.Type,
		"errors", len(violation.Errors),
		"request_id", violation.RequestID,
	)
}

// GetViolations returns all recorded violations
func (cvr *ContractValidationReporter) GetViolations() []ContractViolation {
	return cvr.violations
}

// ClearViolations clears all recorded violations
func (cvr *ContractValidationReporter) ClearViolations() {
	cvr.violations = make([]ContractViolation, 0)
}

// GenerateReport generates a validation report
func (cvr *ContractValidationReporter) GenerateReport() map[string]interface{} {
	totalViolations := len(cvr.violations)
	requestViolations := 0
	responseViolations := 0
	
	violationsByPath := make(map[string]int)
	violationsByMethod := make(map[string]int)
	
	for _, violation := range cvr.violations {
		if violation.Type == "request" {
			requestViolations++
		} else {
			responseViolations++
		}
		
		violationsByPath[violation.Path]++
		violationsByMethod[violation.Method]++
	}
	
	return map[string]interface{}{
		"summary": map[string]interface{}{
			"total_violations":    totalViolations,
			"request_violations":  requestViolations,
			"response_violations": responseViolations,
		},
		"by_path":   violationsByPath,
		"by_method": violationsByMethod,
		"violations": cvr.violations,
	}
}

// TestingContractValidator provides utilities for testing contract validation
type TestingContractValidator struct {
	*ContractValidator
	strict bool
}

// NewTestingContractValidator creates a contract validator for testing
func NewTestingContractValidator(specPath string, strict bool) (*TestingContractValidator, error) {
	validator, err := NewContractValidator(specPath)
	if err != nil {
		return nil, err
	}
	
	return &TestingContractValidator{
		ContractValidator: validator,
		strict:           strict,
	}, nil
}

// ValidateTestRequest validates a test request with detailed error reporting
func (tcv *TestingContractValidator) ValidateTestRequest(r *http.Request) []ContractValidationError {
	err := tcv.ValidateRequest(r)
	if err == nil {
		return nil
	}
	
	return ParseValidationError(err)
}

// ValidateTestResponse validates a test response with detailed error reporting
func (tcv *TestingContractValidator) ValidateTestResponse(req *http.Request, resp *http.Response) []ContractValidationError {
	err := tcv.ValidateResponse(req, resp)
	if err == nil {
		return nil
	}
	
	return ParseValidationError(err)
}

// AssertValidRequest asserts that a request is valid (for use in tests)
func (tcv *TestingContractValidator) AssertValidRequest(r *http.Request) error {
	errors := tcv.ValidateTestRequest(r)
	if len(errors) > 0 {
		return &ValidationAssertionError{
			Type:   "request",
			Errors: errors,
		}
	}
	return nil
}

// AssertValidResponse asserts that a response is valid (for use in tests)
func (tcv *TestingContractValidator) AssertValidResponse(req *http.Request, resp *http.Response) error {
	errors := tcv.ValidateTestResponse(req, resp)
	if len(errors) > 0 {
		return &ValidationAssertionError{
			Type:   "response",
			Errors: errors,
		}
	}
	return nil
}

// ValidationAssertionError represents a validation assertion failure
type ValidationAssertionError struct {
	Type   string            `json:"type"`
	Errors []ContractValidationError `json:"errors"`
}

// Error implements the error interface
func (vae *ValidationAssertionError) Error() string {
	return fmt.Sprintf("%s validation failed with %d errors", vae.Type, len(vae.Errors))
}

// ContractTestRunner runs contract tests against an API
type ContractTestRunner struct {
	validator *TestingContractValidator
	baseURL   string
	client    *http.Client
	reporter  *ContractValidationReporter
}

// NewContractTestRunner creates a new contract test runner
func NewContractTestRunner(specPath, baseURL string, strict bool) (*ContractTestRunner, error) {
	validator, err := NewTestingContractValidator(specPath, strict)
	if err != nil {
		return nil, err
	}
	
	return &ContractTestRunner{
		validator: validator,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		client:    &http.Client{Timeout: 30 * time.Second},
		reporter:  NewContractValidationReporter(slog.Default()),
	}, nil
}

// RunContractTests runs a suite of contract tests
func (ctr *ContractTestRunner) RunContractTests(testCases []ContractTestCase) (*ContractTestResults, error) {
	results := &ContractTestResults{
		TotalTests:  len(testCases),
		StartTime:   time.Now(),
		TestResults: make([]ContractTestResult, 0, len(testCases)),
	}
	
	for _, testCase := range testCases {
		result := ctr.runSingleTest(testCase)
		results.TestResults = append(results.TestResults, result)
		
		if !result.Passed {
			results.FailedTests++
		} else {
			results.PassedTests++
		}
	}
	
	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)
	
	return results, nil
}

// runSingleTest runs a single contract test
func (ctr *ContractTestRunner) runSingleTest(testCase ContractTestCase) ContractTestResult {
	start := time.Now()
	result := ContractTestResult{
		Name:      testCase.Name,
		StartTime: start,
	}
	
	// Create and execute request
	req, err := ctr.createTestRequest(testCase)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		return result
	}
	
	// Validate request
	if requestErrors := ctr.validator.ValidateTestRequest(req); len(requestErrors) > 0 {
		result.RequestValidationErrors = requestErrors
	}
	
	// Execute request
	resp, err := ctr.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		return result
	}
	defer resp.Body.Close()
	
	result.StatusCode = resp.StatusCode
	result.Headers = resp.Header
	
	// Check status code
	if resp.StatusCode != testCase.ExpectedStatus {
		result.Error = fmt.Sprintf("expected status %d, got %d", testCase.ExpectedStatus, resp.StatusCode)
	}
	
	// Validate response
	if responseErrors := ctr.validator.ValidateTestResponse(req, resp); len(responseErrors) > 0 {
		result.ResponseValidationErrors = responseErrors
	}
	
	// Determine if test passed
	result.Passed = result.Error == "" && 
		len(result.RequestValidationErrors) == 0 && 
		len(result.ResponseValidationErrors) == 0
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)
	
	return result
}

// createTestRequest creates an HTTP request from a test case
func (ctr *ContractTestRunner) createTestRequest(testCase ContractTestCase) (*http.Request, error) {
	var body io.Reader
	if testCase.RequestBody != nil {
		jsonBody, err := json.Marshal(testCase.RequestBody)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(string(jsonBody))
	}
	
	req, err := http.NewRequest(testCase.Method, ctr.baseURL+testCase.URL, body)
	if err != nil {
		return nil, err
	}
	
	// Add headers
	for key, value := range testCase.RequestHeaders {
		req.Header.Set(key, value)
	}
	
	// Set content type if body is present
	if testCase.RequestBody != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return req, nil
}

// ContractTestResults represents the results of running contract tests
type ContractTestResults struct {
	TotalTests    int                   `json:"total_tests"`
	PassedTests   int                   `json:"passed_tests"`
	FailedTests   int                   `json:"failed_tests"`
	StartTime     time.Time             `json:"start_time"`
	EndTime       time.Time             `json:"end_time"`
	Duration      time.Duration         `json:"duration"`
	TestResults   []ContractTestResult  `json:"test_results"`
}

// ContractTestResult represents the result of a single contract test
type ContractTestResult struct {
	Name                     string            `json:"name"`
	Passed                   bool              `json:"passed"`
	Error                    string            `json:"error,omitempty"`
	StatusCode               int               `json:"status_code"`
	Headers                  http.Header       `json:"headers,omitempty"`
	RequestValidationErrors  []ContractValidationError `json:"request_validation_errors,omitempty"`
	ResponseValidationErrors []ContractValidationError `json:"response_validation_errors,omitempty"`
	StartTime                time.Time         `json:"start_time"`
	EndTime                  time.Time         `json:"end_time"`
	Duration                 time.Duration     `json:"duration"`
}

// Summary returns a summary of the test results
func (ctr *ContractTestResults) Summary() map[string]interface{} {
	return map[string]interface{}{
		"total":        ctr.TotalTests,
		"passed":       ctr.PassedTests,
		"failed":       ctr.FailedTests,
		"success_rate": float64(ctr.PassedTests) / float64(ctr.TotalTests) * 100,
		"duration":     ctr.Duration.String(),
	}
}