// +build e2e

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	consentservice "github.com/davidleathers/dependable-call-exchange-backend/internal/service/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/test/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestConsentAPIE2E tests the complete consent management API flow
func TestConsentAPIE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	ctx := context.Background()

	// Start containers
	postgresContainer, db := setupPostgres(t, ctx)
	defer postgresContainer.Terminate(ctx)
	defer db.Close()

	redisContainer, redisURL := setupRedis(t, ctx)
	defer redisContainer.Terminate(ctx)

	// Initialize services
	services := setupServices(t, db, redisURL)

	// Create test server
	handler := rest.NewHandler(services)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create test JWT token
	token := testutil.GenerateTestJWT(t, uuid.New(), "buyer", time.Hour)

	// Run E2E test scenarios
	t.Run("CompleteConsentFlow", func(t *testing.T) {
		testCompleteConsentFlow(t, server.URL, token)
	})

	t.Run("BulkConsentOperations", func(t *testing.T) {
		testBulkConsentOperations(t, server.URL, token)
	})

	t.Run("ConsentSearch", func(t *testing.T) {
		testConsentSearch(t, server.URL, token)
	})

	t.Run("ConsentExport", func(t *testing.T) {
		testConsentExport(t, server.URL, token)
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		testValidationErrors(t, server.URL, token)
	})

	t.Run("RateLimiting", func(t *testing.T) {
		testRateLimiting(t, server.URL, token)
	})
}

func testCompleteConsentFlow(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Create consumer
	createConsumerReq := map[string]interface{}{
		"phone_number": "+14155551234",
		"email":        "test@example.com",
		"metadata": map[string]string{
			"source": "web_signup",
		},
	}
	
	consumerResp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent/consumers", token, createConsumerReq)
	assert.Equal(t, http.StatusCreated, consumerResp.StatusCode)
	
	var consumer struct {
		Data struct {
			ID          string `json:"id"`
			PhoneNumber string `json:"phone_number"`
			Email       string `json:"email"`
		} `json:"data"`
	}
	decodeResponse(t, consumerResp, &consumer)
	consumerID := consumer.Data.ID

	// 2. Grant consent
	grantConsentReq := map[string]interface{}{
		"consumer_id": consumerID,
		"channel":     "sms",
		"purpose":     "marketing",
		"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		"metadata": map[string]string{
			"ip_address": "192.168.1.1",
			"user_agent": "Mozilla/5.0",
		},
	}

	consentResp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent", token, grantConsentReq)
	assert.Equal(t, http.StatusCreated, consentResp.StatusCode)

	var consentGrant struct {
		Data struct {
			ID         string    `json:"id"`
			ConsumerID string    `json:"consumer_id"`
			Channel    string    `json:"channel"`
			Purpose    string    `json:"purpose"`
			Status     string    `json:"status"`
			GrantedAt  time.Time `json:"granted_at"`
			ExpiresAt  time.Time `json:"expires_at"`
		} `json:"data"`
	}
	decodeResponse(t, consentResp, &consentGrant)
	consentID := consentGrant.Data.ID
	assert.Equal(t, "active", consentGrant.Data.Status)

	// 3. Verify consent
	verifyResp := makeRequest(t, client, "GET", 
		fmt.Sprintf("%s/api/v1/consent/verify?phone_number=%s&channel=sms&purpose=marketing", 
			baseURL, "+14155551234"), 
		token, nil)
	assert.Equal(t, http.StatusOK, verifyResp.StatusCode)

	var verifyResult struct {
		Data struct {
			IsValid    bool   `json:"is_valid"`
			ConsentID  string `json:"consent_id"`
			GrantedAt  string `json:"granted_at"`
			ExpiresAt  string `json:"expires_at"`
		} `json:"data"`
	}
	decodeResponse(t, verifyResp, &verifyResult)
	assert.True(t, verifyResult.Data.IsValid)
	assert.Equal(t, consentID, verifyResult.Data.ConsentID)

	// 4. Update preferences
	updatePrefsReq := map[string]interface{}{
		"preferences": map[string]interface{}{
			"frequency": "weekly",
			"categories": []string{"promotions", "updates"},
			"time_preference": "morning",
		},
	}

	updateResp := makeRequest(t, client, "PUT", 
		fmt.Sprintf("%s/api/v1/consent/consumers/%s/consents/%s/preferences", baseURL, consumerID, consentID),
		token, updatePrefsReq)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	// 5. Get consent history
	historyResp := makeRequest(t, client, "GET",
		fmt.Sprintf("%s/api/v1/consent/consumers/%s/history", baseURL, consumerID),
		token, nil)
	assert.Equal(t, http.StatusOK, historyResp.StatusCode)

	var history struct {
		Data []struct {
			ID      string `json:"id"`
			Channel string `json:"channel"`
			Purpose string `json:"purpose"`
			Status  string `json:"status"`
		} `json:"data"`
	}
	decodeResponse(t, historyResp, &history)
	assert.Len(t, history.Data, 1)

	// 6. Renew consent
	renewReq := map[string]interface{}{
		"expires_at": time.Now().Add(730 * 24 * time.Hour).Format(time.RFC3339),
		"metadata": map[string]string{
			"renewal_reason": "user_requested",
		},
	}

	renewResp := makeRequest(t, client, "POST",
		fmt.Sprintf("%s/api/v1/consent/consumers/%s/consents/%s/renew", baseURL, consumerID, consentID),
		token, renewReq)
	assert.Equal(t, http.StatusOK, renewResp.StatusCode)

	// 7. Get audit log
	auditResp := makeRequest(t, client, "GET",
		fmt.Sprintf("%s/api/v1/consent/consumers/%s/audit?start_time=%s",
			baseURL, consumerID, time.Now().Add(-1*time.Hour).Format(time.RFC3339)),
		token, nil)
	assert.Equal(t, http.StatusOK, auditResp.StatusCode)

	var auditLog struct {
		Data []struct {
			EventType string    `json:"event_type"`
			Timestamp time.Time `json:"timestamp"`
			Actor     string    `json:"actor"`
		} `json:"data"`
	}
	decodeResponse(t, auditResp, &auditLog)
	assert.GreaterOrEqual(t, len(auditLog.Data), 3) // Grant, Update, Renew

	// 8. Revoke consent
	revokeReq := map[string]interface{}{
		"reason": "user_requested",
		"metadata": map[string]string{
			"method": "email",
		},
	}

	revokeResp := makeRequest(t, client, "POST",
		fmt.Sprintf("%s/api/v1/consent/consumers/%s/consents/%s/revoke", baseURL, consumerID, consentID),
		token, revokeReq)
	assert.Equal(t, http.StatusOK, revokeResp.StatusCode)

	// 9. Verify consent is revoked
	verifyRevokedResp := makeRequest(t, client, "GET",
		fmt.Sprintf("%s/api/v1/consent/verify?phone_number=%s&channel=sms&purpose=marketing",
			baseURL, "+14155551234"),
		token, nil)
	assert.Equal(t, http.StatusOK, verifyRevokedResp.StatusCode)

	var revokedResult struct {
		Data struct {
			IsValid bool   `json:"is_valid"`
			Reason  string `json:"reason"`
		} `json:"data"`
	}
	decodeResponse(t, verifyRevokedResp, &revokedResult)
	assert.False(t, revokedResult.Data.IsValid)
	assert.Equal(t, "consent_revoked", revokedResult.Data.Reason)
}

func testBulkConsentOperations(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create multiple consumers
	consumerIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		req := map[string]interface{}{
			"phone_number": fmt.Sprintf("+1415555%04d", 2000+i),
			"email":        fmt.Sprintf("bulk%d@example.com", i),
		}
		resp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent/consumers", token, req)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var consumer struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &consumer)
		consumerIDs[i] = consumer.Data.ID
	}

	// Bulk grant consents
	bulkGrantReq := map[string]interface{}{
		"consents": []map[string]interface{}{
			{
				"consumer_id": consumerIDs[0],
				"channel":     "sms",
				"purpose":     "marketing",
				"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
			},
			{
				"consumer_id": consumerIDs[1],
				"channel":     "voice",
				"purpose":     "transactional",
				"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
			},
			{
				"consumer_id": consumerIDs[2],
				"channel":     "email",
				"purpose":     "marketing",
				"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	bulkResp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent/bulk", token, bulkGrantReq)
	assert.Equal(t, http.StatusOK, bulkResp.StatusCode)

	var bulkResult struct {
		Data struct {
			Successful int `json:"successful"`
			Failed     int `json:"failed"`
			Results    []struct {
				ConsumerID string `json:"consumer_id"`
				Success    bool   `json:"success"`
				ConsentID  string `json:"consent_id,omitempty"`
				Error      string `json:"error,omitempty"`
			} `json:"results"`
		} `json:"data"`
	}
	decodeResponse(t, bulkResp, &bulkResult)
	assert.Equal(t, 3, bulkResult.Data.Successful)
	assert.Equal(t, 0, bulkResult.Data.Failed)
}

func testConsentSearch(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create test data
	setupSearchTestData(t, client, baseURL, token)

	// Test search with different filters
	testCases := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "Search by channel",
			query:    "channel=sms",
			expected: 3,
		},
		{
			name:     "Search by purpose",
			query:    "purpose=marketing",
			expected: 4,
		},
		{
			name:     "Search by status",
			query:    "status=active",
			expected: 5,
		},
		{
			name:     "Search with date range",
			query:    fmt.Sprintf("start_date=%s&end_date=%s",
				time.Now().Add(-1*time.Hour).Format(time.RFC3339),
				time.Now().Add(1*time.Hour).Format(time.RFC3339)),
			expected: 5,
		},
		{
			name:     "Search with pagination",
			query:    "limit=2&offset=0",
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, client, "GET",
				fmt.Sprintf("%s/api/v1/consent/search?%s", baseURL, tc.query),
				token, nil)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var searchResult struct {
				Data struct {
					Consents []struct {
						ID      string `json:"id"`
						Channel string `json:"channel"`
						Purpose string `json:"purpose"`
						Status  string `json:"status"`
					} `json:"consents"`
					Total      int `json:"total"`
					Limit      int `json:"limit"`
					Offset     int `json:"offset"`
					HasMore    bool `json:"has_more"`
				} `json:"data"`
			}
			decodeResponse(t, resp, &searchResult)
			assert.Equal(t, tc.expected, len(searchResult.Data.Consents))
		})
	}
}

func testConsentExport(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Test different export formats
	formats := []string{"json", "csv"}

	for _, format := range formats {
		t.Run(fmt.Sprintf("Export_%s", format), func(t *testing.T) {
			exportReq := map[string]interface{}{
				"format": format,
				"filter": map[string]interface{}{
					"status":     "active",
					"start_date": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
					"end_date":   time.Now().Format(time.RFC3339),
				},
			}

			resp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent/export", token, exportReq)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			if format == "json" {
				var exportResult struct {
					Data struct {
						ExportID  string `json:"export_id"`
						Status    string `json:"status"`
						Format    string `json:"format"`
						RecordCount int  `json:"record_count"`
					} `json:"data"`
				}
				decodeResponse(t, resp, &exportResult)
				assert.Equal(t, "completed", exportResult.Data.Status)
				assert.Equal(t, format, exportResult.Data.Format)
				assert.Greater(t, exportResult.Data.RecordCount, 0)
			} else if format == "csv" {
				contentType := resp.Header.Get("Content-Type")
				assert.Contains(t, contentType, "text/csv")
				
				contentDisposition := resp.Header.Get("Content-Disposition")
				assert.Contains(t, contentDisposition, "attachment")
				assert.Contains(t, contentDisposition, ".csv")
			}
		})
	}
}

func testValidationErrors(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		endpoint       string
		method         string
		body           map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "Invalid phone number",
			endpoint: "/api/v1/consent/consumers",
			method:   "POST",
			body: map[string]interface{}{
				"phone_number": "invalid-phone",
				"email":        "test@example.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_PHONE_NUMBER",
		},
		{
			name:     "Missing required fields",
			endpoint: "/api/v1/consent",
			method:   "POST",
			body: map[string]interface{}{
				"channel": "sms",
				// missing consumer_id and purpose
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_REQUEST",
		},
		{
			name:     "Invalid channel",
			endpoint: "/api/v1/consent",
			method:   "POST",
			body: map[string]interface{}{
				"consumer_id": uuid.New().String(),
				"channel":     "invalid_channel",
				"purpose":     "marketing",
				"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_CHANNEL",
		},
		{
			name:     "Past expiry date",
			endpoint: "/api/v1/consent",
			method:   "POST",
			body: map[string]interface{}{
				"consumer_id": uuid.New().String(),
				"channel":     "sms",
				"purpose":     "marketing",
				"expires_at":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_EXPIRY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, client, tc.method, baseURL+tc.endpoint, token, tc.body)
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			var errorResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			decodeResponse(t, resp, &errorResp)
			assert.Equal(t, tc.expectedError, errorResp.Error.Code)
		})
	}
}

func testRateLimiting(t *testing.T, baseURL, token string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Make rapid requests to trigger rate limiting
	endpoint := "/api/v1/consent/verify?phone_number=+14155551234&channel=sms&purpose=marketing"
	
	// Assuming rate limit is 10 requests per second
	rateLimitThreshold := 15
	responses := make([]*http.Response, rateLimitThreshold)

	for i := 0; i < rateLimitThreshold; i++ {
		req, _ := http.NewRequest("GET", baseURL+endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		require.NoError(t, err)
		responses[i] = resp
	}

	// Check that some requests were rate limited
	rateLimited := 0
	for _, resp := range responses {
		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimited++
			
			// Check rate limit headers
			assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
			assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
			assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
			assert.NotEmpty(t, resp.Header.Get("Retry-After"))
		}
		resp.Body.Close()
	}

	assert.Greater(t, rateLimited, 0, "Expected some requests to be rate limited")
}

// Helper functions

func setupPostgres(t *testing.T, ctx context.Context) (testcontainers.Container, *sql.DB) {
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("consent_e2e"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	postgresURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := testutil.InitTestDB(postgresURL)
	require.NoError(t, err)

	err = testutil.RunMigrations(db, "../../migrations")
	require.NoError(t, err)

	return postgresContainer, db
}

func setupRedis(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
		redis.WithSnapshotting(10, 1),
	)
	require.NoError(t, err)

	redisURL, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	return redisContainer, redisURL
}

func setupServices(t *testing.T, db *sql.DB, redisURL string) *rest.Services {
	// Initialize repositories
	consentRepo := database.NewConsentRepository(db)
	queryRepo := database.NewConsentQueryRepository(db)
	eventStore := database.NewConsentEventStore(db)
	consumerRepo := database.NewConsumerRepository(db)

	// Initialize cache
	redisCache, err := cache.NewConsentCache(redisURL)
	require.NoError(t, err)

	// Initialize consent service
	consentService := consentservice.NewService(
		consentRepo,
		queryRepo,
		eventStore,
		consumerRepo,
		redisCache,
		&mockComplianceService{},
		&mockEventPublisher{},
		consentservice.NewTransactionManager(db),
	)

	// Return services struct
	return &rest.Services{
		Consent: consentService,
		// Add other services as needed
	}
}

func setupSearchTestData(t *testing.T, client *http.Client, baseURL, token string) {
	// Create consumers and consents with different attributes
	testData := []struct {
		phone   string
		channel string
		purpose string
	}{
		{"+14155553001", "sms", "marketing"},
		{"+14155553002", "sms", "transactional"},
		{"+14155553003", "sms", "marketing"},
		{"+14155553004", "voice", "marketing"},
		{"+14155553005", "email", "marketing"},
	}

	for _, data := range testData {
		// Create consumer
		consumerReq := map[string]interface{}{
			"phone_number": data.phone,
			"email":        fmt.Sprintf("%s@example.com", data.phone[1:]),
		}
		consumerResp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent/consumers", token, consumerReq)
		require.Equal(t, http.StatusCreated, consumerResp.StatusCode)

		var consumer struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		decodeResponse(t, consumerResp, &consumer)

		// Grant consent
		consentReq := map[string]interface{}{
			"consumer_id": consumer.Data.ID,
			"channel":     data.channel,
			"purpose":     data.purpose,
			"expires_at":  time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		}
		consentResp := makeRequest(t, client, "POST", baseURL+"/api/v1/consent", token, consentReq)
		require.Equal(t, http.StatusCreated, consentResp.StatusCode)
	}
}

func makeRequest(t *testing.T, client *http.Client, method, url, token string, body interface{}) *http.Response {
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		require.NoError(t, err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func decodeResponse(t *testing.T, resp *http.Response, v interface{}) {
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(v)
	require.NoError(t, err)
}

// Mock implementations
type mockComplianceService struct{}

func (m *mockComplianceService) CheckDNC(ctx context.Context, phoneNumber string) (bool, error) {
	return false, nil
}

func (m *mockComplianceService) CheckTCPA(ctx context.Context, phoneNumber string, callTime time.Time) (string, error) {
	return "valid", nil
}

func (m *mockComplianceService) ValidateConsent(ctx context.Context, consentRecord *consent.ConsentRecord) error {
	return nil
}

type mockEventPublisher struct{}

func (m *mockEventPublisher) PublishConsentEvent(ctx context.Context, event consent.Event) error {
	return nil
}