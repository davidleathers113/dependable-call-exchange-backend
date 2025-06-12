package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================
// Phone Number Validation Tests
// ====================================

func TestValidation_PhoneNumbers(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		endpoint       string
		method         string
		phoneField     string
		phoneValue     string
		expectedStatus int
		expectedError  string
	}{
		// Valid E.164 formats
		{
			name:           "valid US number",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+14155551234",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid UK number",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+442071838750",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid short code",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+1234567",
			expectedStatus: http.StatusCreated,
		},
		// Invalid formats
		{
			name:           "missing plus sign",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "14155551234",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "must start with +",
		},
		{
			name:           "local format",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "(415) 555-1234",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid phone number format",
		},
		{
			name:           "contains letters",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+1415FLOWERS",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid phone number",
		},
		{
			name:           "too short",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+1",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Phone number too short",
		},
		{
			name:           "too long",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+1234567890123456", // > 15 digits
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Phone number too long",
		},
		{
			name:           "special characters",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "+1-415-555-1234",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid characters",
		},
		{
			name:           "empty string",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			phoneField:     "from_number",
			phoneValue:     "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "required",
		},
		// DNC endpoint validation
		{
			name:           "valid DNC check",
			endpoint:       "/api/v1/compliance/dnc",
			method:         "POST",
			phoneField:     "phone_number",
			phoneValue:     "+14155551234",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid DNC phone",
			endpoint:       "/api/v1/compliance/dnc",
			method:         "POST",
			phoneField:     "phone_number",
			phoneValue:     "415-555-1234",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid phone number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request map[string]interface{}
			
			if tt.endpoint == "/api/v1/calls" {
				request = map[string]interface{}{
					tt.phoneField: tt.phoneValue,
					"to_number":   "+18005551234", // Default valid number
				}
			} else if tt.endpoint == "/api/v1/compliance/dnc" {
				request = map[string]interface{}{
					tt.phoneField: tt.phoneValue,
					"reason":      "test",
				}
			}
			
			w := makeRequest(handler, tt.method, tt.endpoint, request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Email Validation Tests
// ====================================

func TestValidation_Email(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		email          string
		expectedStatus int
		expectedError  string
	}{
		// Valid emails
		{
			name:           "standard email",
			email:          "user@example.com",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "email with dots",
			email:          "first.last@example.com",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "email with plus",
			email:          "user+tag@example.com",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "subdomain email",
			email:          "user@mail.example.com",
			expectedStatus: http.StatusCreated,
		},
		// Invalid emails
		{
			name:           "missing @",
			email:          "userexample.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "missing domain",
			email:          "user@",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "missing local part",
			email:          "@example.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "double @",
			email:          "user@@example.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "spaces in email",
			email:          "user name@example.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "invalid TLD",
			email:          "user@example",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "special chars in domain",
			email:          "user@exa!mple.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email",
		},
		{
			name:           "empty email",
			email:          "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Email is required",
		},
		{
			name:           "email too long",
			email:          strings.Repeat("a", 255) + "@example.com",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Email too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := map[string]interface{}{
				"email":        tt.email,
				"password":     "SecurePass123!",
				"company_name": "Test Company",
				"type":         "buyer",
			}
			
			w := makeRequest(handler, "POST", "/api/v1/auth/register", request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Money/Currency Validation Tests
// ====================================

func TestValidation_Money(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		amount         interface{} // Can be float, string, or invalid type
		expectedStatus int
		expectedError  string
	}{
		// Valid amounts
		{
			name:           "valid integer amount",
			amount:         10,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid decimal amount",
			amount:         10.50,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid string amount",
			amount:         "25.99",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "maximum precision",
			amount:         10.99,
			expectedStatus: http.StatusCreated,
		},
		// Invalid amounts
		{
			name:           "negative amount",
			amount:         -10.50,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "must be positive",
		},
		{
			name:           "zero amount",
			amount:         0,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "must be greater than zero",
		},
		{
			name:           "too many decimal places",
			amount:         10.999,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid decimal places",
		},
		{
			name:           "very large amount",
			amount:         1000000.00,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "exceeds maximum",
		},
		{
			name:           "NaN string",
			amount:         "not-a-number",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid amount",
		},
		{
			name:           "infinity",
			amount:         "Infinity",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid amount",
		},
		{
			name:           "null amount",
			amount:         nil,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Amount is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := map[string]interface{}{
				"auction_id": uuid.New(),
				"amount":     tt.amount,
			}
			
			w := makeRequest(handler, "POST", "/api/v1/bids", request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// UUID Validation Tests
// ====================================

func TestValidation_UUID(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		callID         string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid UUID v4",
			callID:         uuid.New().String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid UUID with hyphens",
			callID:         "550e8400-e29b-41d4-a716-446655440000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID format",
			callID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid UUID",
		},
		{
			name:           "UUID without hyphens",
			callID:         "550e8400e29b41d4a716446655440000",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid UUID",
		},
		{
			name:           "empty string",
			callID:         "",
			expectedStatus: http.StatusNotFound, // Router returns 404 for empty path param
		},
		{
			name:           "null UUID",
			callID:         "00000000-0000-0000-0000-000000000000",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid call ID",
		},
		{
			name:           "malformed UUID",
			callID:         "zzz-invalid-uuid-format",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid UUID",
		},
		{
			name:           "UUID with extra characters",
			callID:         uuid.New().String() + "-extra",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid UUID",
		},
		{
			name:           "SQL injection attempt",
			callID:         "'; DROP TABLE calls; --",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := makeRequest(handler, "POST", "/api/v1/calls/"+tt.callID+"/route", nil)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Time/Date Validation Tests
// ====================================

func TestValidation_TimeFormats(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		startTime      string
		endTime        string
		timezone       string
		expectedStatus int
		expectedError  string
	}{
		// Valid time formats
		{
			name:           "valid 24-hour format",
			startTime:      "09:00",
			endTime:        "20:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid midnight",
			startTime:      "00:00",
			endTime:        "23:59",
			timezone:       "UTC",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid early morning",
			startTime:      "06:30",
			endTime:        "18:30",
			timezone:       "America/Los_Angeles",
			expectedStatus: http.StatusOK,
		},
		// Invalid formats
		{
			name:           "12-hour format",
			startTime:      "9:00 AM",
			endTime:        "8:00 PM",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid time format",
		},
		{
			name:           "missing leading zero",
			startTime:      "9:00",
			endTime:        "20:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid time format",
		},
		{
			name:           "invalid hour",
			startTime:      "25:00",
			endTime:        "20:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid hour",
		},
		{
			name:           "invalid minute",
			startTime:      "09:60",
			endTime:        "20:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid minute",
		},
		{
			name:           "start after end",
			startTime:      "20:00",
			endTime:        "09:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Start time must be before end time",
		},
		{
			name:           "invalid timezone",
			startTime:      "09:00",
			endTime:        "20:00",
			timezone:       "Invalid/Timezone",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid timezone",
		},
		{
			name:           "empty timezone",
			startTime:      "09:00",
			endTime:        "20:00",
			timezone:       "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Timezone is required",
		},
		{
			name:           "seconds included",
			startTime:      "09:00:00",
			endTime:        "20:00:00",
			timezone:       "America/New_York",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid time format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := SetTCPAHoursRequest{
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
				TimeZone:  tt.timezone,
			}
			
			w := makeRequest(handler, "PUT", "/api/v1/compliance/tcpa/hours", request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Enum/Status Validation Tests
// ====================================

func TestValidation_Enums(t *testing.T) {
	handler, _ := setupHandler(t)
	callID := uuid.New()

	tests := []struct {
		name           string
		endpoint       string
		method         string
		field          string
		value          string
		expectedStatus int
		expectedError  string
	}{
		// Call Direction
		{
			name:           "valid inbound direction",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			field:          "direction",
			value:          "inbound",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid outbound direction",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			field:          "direction",
			value:          "outbound",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid direction",
			endpoint:       "/api/v1/calls",
			method:         "POST",
			field:          "direction",
			value:          "sideways",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid direction",
		},
		// Call Status
		{
			name:           "valid status pending",
			endpoint:       "/api/v1/calls/" + callID.String() + "/status",
			method:         "PATCH",
			field:          "status",
			value:          "pending",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid status ringing",
			endpoint:       "/api/v1/calls/" + callID.String() + "/status",
			method:         "PATCH",
			field:          "status",
			value:          "ringing",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid status",
			endpoint:       "/api/v1/calls/" + callID.String() + "/status",
			method:         "PATCH",
			field:          "status",
			value:          "flying",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid status",
		},
		// Account Type
		{
			name:           "valid buyer type",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			field:          "type",
			value:          "buyer",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid seller type",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			field:          "type",
			value:          "seller",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid account type",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			field:          "type",
			value:          "admin",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid account type",
		},
		{
			name:           "case sensitive type",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			field:          "type",
			value:          "BUYER",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid account type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request map[string]interface{}
			
			switch tt.endpoint {
			case "/api/v1/calls":
				request = map[string]interface{}{
					"from_number": "+14155551234",
					"to_number":   "+18005551234",
					tt.field:      tt.value,
				}
			case "/api/v1/calls/" + callID.String() + "/status":
				request = map[string]interface{}{
					tt.field: tt.value,
				}
			case "/api/v1/auth/register":
				request = map[string]interface{}{
					"email":        "test@example.com",
					"password":     "SecurePass123!",
					"company_name": "Test Company",
					tt.field:       tt.value,
				}
			}
			
			w := makeRequest(handler, tt.method, tt.endpoint, request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Array/List Validation Tests
// ====================================

func TestValidation_Arrays(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		countries      interface{}
		states         interface{}
		callTypes      interface{}
		expectedStatus int
		expectedError  string
	}{
		// Valid arrays
		{
			name:           "valid single country",
			countries:      []string{"US"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid multiple countries",
			countries:      []string{"US", "CA", "UK"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid states",
			countries:      []string{"US"},
			states:         []string{"CA", "NY", "TX"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "valid call types",
			countries:      []string{"US"},
			callTypes:      []string{"sales", "support", "survey"},
			expectedStatus: http.StatusCreated,
		},
		// Invalid arrays
		{
			name:           "empty countries array",
			countries:      []string{},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "At least one country required",
		},
		{
			name:           "invalid country code",
			countries:      []string{"USA"}, // Should be "US"
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid country code",
		},
		{
			name:           "duplicate countries",
			countries:      []string{"US", "US", "CA"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Duplicate country",
		},
		{
			name:           "too many countries",
			countries:      []string{"US", "CA", "UK", "FR", "DE", "IT", "ES", "AU", "JP", "CN", "IN"}, // > 10
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Too many countries",
		},
		{
			name:           "invalid state code",
			countries:      []string{"US"},
			states:         []string{"California"}, // Should be "CA"
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid state code",
		},
		{
			name:           "state without country",
			states:         []string{"CA", "NY"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Country required for states",
		},
		{
			name:           "non-array value",
			countries:      "US", // Should be array
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Must be an array",
		},
		{
			name:           "null array",
			countries:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Countries required",
		},
		{
			name:           "array with null values",
			countries:      []interface{}{"US", nil, "CA"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid value in array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			criteria := map[string]interface{}{
				"max_budget": 100.00,
			}
			
			if tt.countries != nil {
				criteria["geography"] = map[string]interface{}{
					"countries": tt.countries,
				}
			}
			if tt.states != nil {
				if criteria["geography"] == nil {
					criteria["geography"] = map[string]interface{}{}
				}
				criteria["geography"].(map[string]interface{})["states"] = tt.states
			}
			if tt.callTypes != nil {
				criteria["call_type"] = tt.callTypes
			}
			
			request := map[string]interface{}{
				"criteria": criteria,
				"active":   true,
			}
			
			// Add seller context
			req := httptest.NewRequest("POST", "/api/v1/bid-profiles", nil)
			ctx := req.Context()
			ctx = context.WithValue(ctx, contextKeyAccountType, "seller")
			ctx = context.WithValue(ctx, contextKeyUserID, uuid.New().String())
			req = req.WithContext(ctx)
			
			jsonBody, _ := json.Marshal(request)
			req.Body = io.NopCloser(bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// String Length Validation Tests
// ====================================

func TestValidation_StringLengths(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		field          string
		value          string
		expectedStatus int
		expectedError  string
	}{
		// Company Name
		{
			name:           "valid company name",
			field:          "company_name",
			value:          "Acme Corporation",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "minimum length company name",
			field:          "company_name",
			value:          "AB", // 2 chars
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "maximum length company name",
			field:          "company_name",
			value:          strings.Repeat("A", 255),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "too short company name",
			field:          "company_name",
			value:          "A", // 1 char
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Company name too short",
		},
		{
			name:           "too long company name",
			field:          "company_name",
			value:          strings.Repeat("A", 256),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Company name too long",
		},
		{
			name:           "empty company name",
			field:          "company_name",
			value:          "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Company name is required",
		},
		{
			name:           "whitespace only",
			field:          "company_name",
			value:          "   ",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Company name cannot be empty",
		},
		// Password
		{
			name:           "valid password",
			field:          "password",
			value:          "SecurePass123!",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "minimum length password",
			field:          "password",
			value:          "Pass123!",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "too short password",
			field:          "password",
			value:          "Pass1!",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password too short",
		},
		{
			name:           "too long password",
			field:          "password",
			value:          strings.Repeat("A", 73) + "123!",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password too long",
		},
		{
			name:           "no uppercase",
			field:          "password",
			value:          "securepass123!",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must contain uppercase",
		},
		{
			name:           "no lowercase",
			field:          "password",
			value:          "SECUREPASS123!",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must contain lowercase",
		},
		{
			name:           "no numbers",
			field:          "password",
			value:          "SecurePass!",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must contain number",
		},
		{
			name:           "no special chars",
			field:          "password",
			value:          "SecurePass123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must contain special character",
		},
		// DNC Reason
		{
			name:           "valid reason",
			field:          "reason",
			value:          "Consumer requested removal",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "empty reason",
			field:          "reason",
			value:          "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Reason is required",
		},
		{
			name:           "too long reason",
			field:          "reason",
			value:          strings.Repeat("A", 501),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Reason too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request map[string]interface{}
			var endpoint string
			var method string
			
			switch tt.field {
			case "company_name", "password":
				endpoint = "/api/v1/auth/register"
				method = "POST"
				request = map[string]interface{}{
					"email":        "test@example.com",
					"password":     "SecurePass123!",
					"company_name": "Test Company",
					"type":         "buyer",
				}
				request[tt.field] = tt.value
			case "reason":
				endpoint = "/api/v1/compliance/dnc"
				method = "POST"
				request = map[string]interface{}{
					"phone_number": "+14155551234",
					"reason":       tt.value,
				}
			}
			
			w := makeRequest(handler, method, endpoint, request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

// ====================================
// Nested Object Validation Tests
// ====================================

func TestValidation_NestedObjects(t *testing.T) {
	handler, _ := setupHandler(t)

	tests := []struct {
		name           string
		request        interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid nested bid criteria",
			request: map[string]interface{}{
				"criteria": map[string]interface{}{
					"geography": map[string]interface{}{
						"countries": []string{"US"},
						"states":    []string{"CA", "NY"},
						"radius":    50.0,
					},
					"call_type":  []string{"sales"},
					"max_budget": 100.00,
					"keywords":   []string{"insurance", "auto"},
				},
				"active": true,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing required nested field",
			request: map[string]interface{}{
				"criteria": map[string]interface{}{
					"geography": map[string]interface{}{
						// Missing countries
						"states": []string{"CA", "NY"},
					},
					"max_budget": 100.00,
				},
				"active": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Countries required",
		},
		{
			name: "invalid nested field type",
			request: map[string]interface{}{
				"criteria": map[string]interface{}{
					"geography": "US", // Should be object
					"max_budget": 100.00,
				},
				"active": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Geography must be an object",
		},
		{
			name: "deeply nested validation",
			request: map[string]interface{}{
				"criteria": map[string]interface{}{
					"geography": map[string]interface{}{
						"countries": []string{"US"},
						"location": map[string]interface{}{
							"lat":    "invalid", // Should be number
							"lng":    -122.4194,
							"radius": 50,
						},
					},
					"max_budget": 100.00,
				},
				"active": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid latitude",
		},
		{
			name: "null nested object",
			request: map[string]interface{}{
				"criteria": nil,
				"active":   true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Criteria is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add seller context
			req := httptest.NewRequest("POST", "/api/v1/bid-profiles", nil)
			ctx := req.Context()
			ctx = context.WithValue(ctx, contextKeyAccountType, "seller")
			ctx = context.WithValue(ctx, contextKeyUserID, uuid.New().String())
			req = req.WithContext(ctx)
			
			jsonBody, _ := json.Marshal(tt.request)
			req.Body = io.NopCloser(bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedError))
			}
		})
	}
}

