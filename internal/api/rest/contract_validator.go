package rest

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// ContractValidator validates HTTP requests and responses against OpenAPI specification
type ContractValidator struct {
	loader *openapi3.Loader
	doc    *openapi3.T
	router routers.Router
}

// NewContractValidator creates a new contract validator from OpenAPI spec file
func NewContractValidator(specPath string) (*ContractValidator, error) {
	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Validate the document
	err = doc.Validate(loader.Context)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	// Create router for path matching
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	return &ContractValidator{
		loader: loader,
		doc:    doc,
		router: router,
	}, nil
}

// ValidateRequest validates an HTTP request against the OpenAPI specification
func (cv *ContractValidator) ValidateRequest(req *http.Request) error {
	// Find the matching route
	route, pathParams, err := cv.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("no matching route found: %w", err)
	}

	// Create validation input
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
	}

	// Validate the request
	err = openapi3filter.ValidateRequest(cv.loader.Context, requestValidationInput)
	if err != nil {
		return fmt.Errorf("request validation failed: %w", err)
	}

	return nil
}

// ValidateResponse validates an HTTP response against the OpenAPI specification
func (cv *ContractValidator) ValidateResponse(req *http.Request, resp *http.Response) error {
	// Find the matching route
	route, pathParams, err := cv.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("no matching route found: %w", err)
	}

	// Create validation input
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		},
		Status: resp.StatusCode,
		Header: resp.Header,
	}

	// Add response body if present
	if resp.Body != nil {
		responseValidationInput.SetBodyBytes([]byte{}) // Placeholder - would need actual body
	}

	// Validate the response
	err = openapi3filter.ValidateResponse(cv.loader.Context, responseValidationInput)
	if err != nil {
		return fmt.Errorf("response validation failed: %w", err)
	}

	return nil
}

// GetOperationSpec returns the OpenAPI operation spec for a given HTTP request
func (cv *ContractValidator) GetOperationSpec(req *http.Request) (*openapi3.Operation, error) {
	route, _, err := cv.router.FindRoute(req)
	if err != nil {
		return nil, fmt.Errorf("no matching route found: %w", err)
	}

	return route.Operation, nil
}

// ValidateSchema validates a Go struct against an OpenAPI schema
func (cv *ContractValidator) ValidateSchema(schemaName string, data interface{}) error {
	schema := cv.doc.Components.Schemas[schemaName]
	if schema == nil {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	err := schema.Value.VisitJSON(data)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// ContractTestCase represents a single contract test case
type ContractTestCase struct {
	Name           string
	Method         string
	URL            string
	RequestBody    interface{}
	RequestHeaders map[string]string
	ExpectedStatus int
	ExpectedSchema string // Optional: schema name to validate response against
}

// ContractTestSuite manages a collection of contract tests
type ContractTestSuite struct {
	validator *ContractValidator
	baseURL   string
	client    *http.Client
}

// NewContractTestSuite creates a new contract test suite
func NewContractTestSuite(specPath, baseURL string) (*ContractTestSuite, error) {
	validator, err := NewContractValidator(specPath)
	if err != nil {
		return nil, err
	}

	return &ContractTestSuite{
		validator: validator,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		client:    &http.Client{},
	}, nil
}

// RunTest executes a single contract test case
func (cts *ContractTestSuite) RunTest(testCase ContractTestCase) error {
	// Create request
	var reqBody *strings.Reader
	if testCase.RequestBody != nil {
		// Would marshal to JSON here
		reqBody = strings.NewReader("")
	}

	req, err := http.NewRequest(testCase.Method, cts.baseURL+testCase.URL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range testCase.RequestHeaders {
		req.Header.Set(key, value)
	}

	// Validate request against OpenAPI spec
	err = cts.validator.ValidateRequest(req)
	if err != nil {
		return fmt.Errorf("request validation failed: %w", err)
	}

	// Execute request
	resp, err := cts.client.Do(req)
	if err != nil {
		return fmt.Errorf("request execution failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != testCase.ExpectedStatus {
		return fmt.Errorf("expected status %d, got %d", testCase.ExpectedStatus, resp.StatusCode)
	}

	// Validate response against OpenAPI spec
	err = cts.validator.ValidateResponse(req, resp)
	if err != nil {
		return fmt.Errorf("response validation failed: %w", err)
	}

	return nil
}

// ContractValidationError represents a contract validation error with details
type ContractValidationError struct {
	Type    string      `json:"type"`
	Message string      `json:"message"`
	Path    string      `json:"path,omitempty"`
	Value   interface{} `json:"value,omitempty"`
	Schema  string      `json:"schema,omitempty"`
}

// Error implements the error interface
func (ve ContractValidationError) Error() string {
	return fmt.Sprintf("validation error (%s): %s", ve.Type, ve.Message)
}

// ParseValidationError extracts structured information from openapi3filter errors
func ParseValidationError(err error) []ContractValidationError {
	var errors []ContractValidationError
	
	// Parse different types of validation errors
	if requestError, ok := err.(*openapi3filter.RequestError); ok {
		errors = append(errors, ContractValidationError{
			Type:    "request",
			Message: requestError.Error(),
		})
	}

	if securityError, ok := err.(*openapi3filter.SecurityRequirementsError); ok {
		errors = append(errors, ContractValidationError{
			Type:    "security",
			Message: securityError.Error(),
		})
	}

	// Default case
	if len(errors) == 0 {
		errors = append(errors, ContractValidationError{
			Type:    "unknown",
			Message: err.Error(),
		})
	}

	return errors
}