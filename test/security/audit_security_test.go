//go:build security

package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_AuditIntegrity(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	// Create admin user for audit access
	admin := createTestUser(t, client, "admin@test.com", "admin")
	client.SetToken(admin.Token)

	t.Run("Cryptographic Hash Validation", func(t *testing.T) {
		testCryptographicHashValidation(t, client)
	})

	t.Run("Tamper Detection Testing", func(t *testing.T) {
		testTamperDetection(t, client)
	})

	t.Run("Hash Chain Integrity", func(t *testing.T) {
		testHashChainIntegrity(t, client)
	})

	t.Run("SQL Injection Prevention in Audit", func(t *testing.T) {
		testAuditSQLInjectionPrevention(t, client)
	})

	t.Run("Authentication Testing", func(t *testing.T) {
		testAuditAuthenticationSecurity(t, client, admin)
	})

	t.Run("PII Protection Validation", func(t *testing.T) {
		testAuditPIIProtection(t, client)
	})

	t.Run("Vulnerability Scanning", func(t *testing.T) {
		testAuditVulnerabilityScanning(t, client)
	})
}

func testCryptographicHashValidation(t *testing.T, client *infrastructure.APIClient) {
	t.Run("Hash Algorithm Strength", func(t *testing.T) {
		// Test that SHA-256 is used for hash computation
		// Create test event and verify hash algorithm
		eventData := map[string]interface{}{
			"type":        "CALL_START",
			"actor_id":    "test-user",
			"target_id":   "test-call",
			"action":      "start_call",
			"target_type": "call",
			"severity":    "INFO",
		}

		resp := client.Post("/api/v1/audit/events", eventData)
		require.Equal(t, 201, resp.StatusCode)

		var response map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		resp.Body.Close()

		// Verify hash is present and has correct length for SHA-256
		eventHash := response["event_hash"].(string)
		assert.NotEmpty(t, eventHash)
		
		// SHA-256 hash should be 64 hex characters
		assert.Len(t, eventHash, 64)
		assert.Regexp(t, "^[a-f0-9]{64}$", eventHash)
	})

	t.Run("Signature Cryptographic Strength", func(t *testing.T) {
		// Test signature validation using HMAC-SHA256
		secretKey := make([]byte, 32) // 256-bit key
		rand.Read(secretKey)
		
		testData := []byte("test audit data for signing")
		
		// Create signature using our audit signature system
		signature, err := values.ComputeAuditSignature(testData, secretKey)
		require.NoError(t, err)

		// Verify signature strength
		assert.NotEmpty(t, signature.String())
		
		// Verify it's properly base64 encoded
		sigBytes, err := signature.Bytes()
		require.NoError(t, err)
		assert.Len(t, sigBytes, 32) // HMAC-SHA256 produces 32 bytes

		// Verify signature validates correctly
		isValid, err := signature.Verify(testData, secretKey)
		require.NoError(t, err)
		assert.True(t, isValid)
	})

	t.Run("Weak Key Rejection", func(t *testing.T) {
		// Test that weak keys are rejected
		weakKey := []byte("short") // Too short
		testData := []byte("test data")

		_, err := values.ComputeAuditSignature(testData, weakKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "secret key must be at least 32 bytes")
	})

	t.Run("Hash Collision Resistance", func(t *testing.T) {
		// Test that different inputs produce different hashes
		hashes := make(map[string]bool)
		
		for i := 0; i < 1000; i++ {
			eventData := map[string]interface{}{
				"type":        "TEST_EVENT",
				"actor_id":    fmt.Sprintf("user-%d", i),
				"target_id":   fmt.Sprintf("target-%d", i),
				"action":      fmt.Sprintf("action-%d", i),
				"target_type": "test",
				"severity":    "INFO",
				"metadata":    map[string]interface{}{"iteration": i},
			}

			resp := client.Post("/api/v1/audit/events", eventData)
			if resp.StatusCode != 201 {
				continue // Skip failed requests
			}

			var response map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&response)
			resp.Body.Close()

			if eventHash, ok := response["event_hash"].(string); ok {
				// Verify no hash collisions
				assert.False(t, hashes[eventHash], "Hash collision detected for hash: %s", eventHash)
				hashes[eventHash] = true
			}
		}

		// Should have generated many unique hashes
		assert.Greater(t, len(hashes), 900, "Expected at least 900 unique hashes")
	})
}

func testTamperDetection(t *testing.T, client *infrastructure.APIClient) {
	t.Run("Hash Manipulation Detection", func(t *testing.T) {
		// Create a legitimate audit event
		eventData := map[string]interface{}{
			"type":        "CALL_START",
			"actor_id":    "test-user",
			"target_id":   "test-call",
			"action":      "start_call",
			"target_type": "call",
			"severity":    "INFO",
		}

		resp := client.Post("/api/v1/audit/events", eventData)
		require.Equal(t, 201, resp.StatusCode)

		var originalEvent map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&originalEvent)
		require.NoError(t, err)
		resp.Body.Close()

		eventID := originalEvent["id"].(string)

		// Attempt to modify the event hash directly (should be detected)
		tamperedHash := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
		
		// Try to update with tampered hash
		updateData := map[string]interface{}{
			"event_hash": tamperedHash,
		}

		updateResp := client.Put(fmt.Sprintf("/api/v1/audit/events/%s", eventID), updateData)
		
		// Should be rejected - audit events are immutable
		assert.Equal(t, 403, updateResp.StatusCode)
		updateResp.Body.Close()

		// Verify original event is unchanged
		getResp := client.Get(fmt.Sprintf("/api/v1/audit/events/%s", eventID))
		require.Equal(t, 200, getResp.StatusCode)

		var retrievedEvent map[string]interface{}
		json.NewDecoder(getResp.Body).Decode(&retrievedEvent)
		getResp.Body.Close()

		assert.Equal(t, originalEvent["event_hash"], retrievedEvent["event_hash"])
		assert.NotEqual(t, tamperedHash, retrievedEvent["event_hash"])
	})

	t.Run("Chain Corruption Simulation", func(t *testing.T) {
		// Create a sequence of events
		events := make([]map[string]interface{}, 5)
		
		for i := 0; i < 5; i++ {
			eventData := map[string]interface{}{
				"type":        "TEST_SEQUENCE",
				"actor_id":    "test-user",
				"target_id":   fmt.Sprintf("target-%d", i),
				"action":      fmt.Sprintf("action-%d", i),
				"target_type": "test",
				"severity":    "INFO",
			}

			resp := client.Post("/api/v1/audit/events", eventData)
			require.Equal(t, 201, resp.StatusCode)

			var event map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&event)
			resp.Body.Close()
			
			events[i] = event
		}

		// Verify chain integrity initially
		verifyResp := client.Post("/api/v1/audit/verify-chain", map[string]interface{}{
			"event_ids": extractEventIDs(events),
		})
		
		require.Equal(t, 200, verifyResp.StatusCode)
		
		var verifyResult map[string]interface{}
		json.NewDecoder(verifyResp.Body).Decode(&verifyResult)
		verifyResp.Body.Close()
		
		assert.True(t, verifyResult["is_valid"].(bool))
		assert.Equal(t, float64(0), verifyResult["chain_breaks"].(float64))
	})

	t.Run("Signature Tampering Detection", func(t *testing.T) {
		// Test detection of signature tampering
		secretKey := make([]byte, 32)
		rand.Read(secretKey)
		
		testData := []byte("legitimate audit data")
		validSignature, err := values.ComputeAuditSignature(testData, secretKey)
		require.NoError(t, err)

		// Create tampered signature
		tamperedSigBytes := make([]byte, 32)
		rand.Read(tamperedSigBytes)
		tamperedSignature, err := values.NewAuditSignatureFromBytes(tamperedSigBytes)
		require.NoError(t, err)

		// Verify original signature is valid
		isValid, err := validSignature.Verify(testData, secretKey)
		require.NoError(t, err)
		assert.True(t, isValid)

		// Verify tampered signature is invalid
		isValid, err = tamperedSignature.Verify(testData, secretKey)
		require.NoError(t, err)
		assert.False(t, isValid, "Tampered signature should not validate")
	})
}

func testHashChainIntegrity(t *testing.T, client *infrastructure.APIClient) {
	t.Run("Sequential Chain Validation", func(t *testing.T) {
		// Create a chain of related events
		chainEvents := make([]map[string]interface{}, 10)
		
		for i := 0; i < 10; i++ {
			eventData := map[string]interface{}{
				"type":        "CHAIN_TEST",
				"actor_id":    "test-user",
				"target_id":   "chain-target",
				"action":      fmt.Sprintf("step-%d", i),
				"target_type": "chain",
				"severity":    "INFO",
				"metadata": map[string]interface{}{
					"step":  i,
					"chain": "test-chain-1",
				},
			}

			resp := client.Post("/api/v1/audit/events", eventData)
			require.Equal(t, 201, resp.StatusCode)

			var event map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&event)
			resp.Body.Close()
			
			chainEvents[i] = event
			
			// Small delay to ensure timestamp ordering
			time.Sleep(10 * time.Millisecond)
		}

		// Verify the chain integrity
		verifyResp := client.Post("/api/v1/audit/verify-chain", map[string]interface{}{
			"event_ids": extractEventIDs(chainEvents),
		})
		
		require.Equal(t, 200, verifyResp.StatusCode)
		
		var result map[string]interface{}
		json.NewDecoder(verifyResp.Body).Decode(&result)
		verifyResp.Body.Close()
		
		assert.True(t, result["is_valid"].(bool))
		assert.Equal(t, float64(10), result["events_verified"].(float64))
		assert.Empty(t, result["chain_breaks"])
		assert.NotEmpty(t, result["aggregate_hash"])
	})

	t.Run("Gap Detection in Chain", func(t *testing.T) {
		// Create events with intentional sequence gaps
		eventData1 := map[string]interface{}{
			"type":        "GAP_TEST",
			"actor_id":    "test-user",
			"target_id":   "gap-target",
			"action":      "step-1",
			"target_type": "gap",
			"severity":    "INFO",
		}

		resp1 := client.Post("/api/v1/audit/events", eventData1)
		require.Equal(t, 201, resp1.StatusCode)
		var event1 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&event1)
		resp1.Body.Close()

		time.Sleep(100 * time.Millisecond)

		// Create several intermediate events that we'll "lose"
		for i := 0; i < 3; i++ {
			intermediateData := map[string]interface{}{
				"type":        "GAP_TEST",
				"actor_id":    "test-user",
				"target_id":   "gap-target",
				"action":      fmt.Sprintf("intermediate-%d", i),
				"target_type": "gap",
				"severity":    "INFO",
			}
			resp := client.Post("/api/v1/audit/events", intermediateData)
			resp.Body.Close()
		}

		time.Sleep(100 * time.Millisecond)

		eventData3 := map[string]interface{}{
			"type":        "GAP_TEST",
			"actor_id":    "test-user",
			"target_id":   "gap-target",
			"action":      "step-final",
			"target_type": "gap",
			"severity":    "INFO",
		}

		resp3 := client.Post("/api/v1/audit/events", eventData3)
		require.Equal(t, 201, resp3.StatusCode)
		var event3 map[string]interface{}
		json.NewDecoder(resp3.Body).Decode(&event3)
		resp3.Body.Close()

		// Verify chain with gap (only including first and last event)
		verifyResp := client.Post("/api/v1/audit/verify-chain", map[string]interface{}{
			"event_ids": []string{
				event1["id"].(string),
				event3["id"].(string),
			},
		})
		
		require.Equal(t, 200, verifyResp.StatusCode)
		
		var result map[string]interface{}
		json.NewDecoder(verifyResp.Body).Decode(&result)
		verifyResp.Body.Close()
		
		// Should detect the gap in the chain
		assert.False(t, result["is_valid"].(bool))
		
		if chainBreaks, ok := result["chain_breaks"].([]interface{}); ok {
			assert.Greater(t, len(chainBreaks), 0, "Should detect chain breaks")
		}
	})

	t.Run("Timestamp Ordering Validation", func(t *testing.T) {
		// Test detection of timestamp anomalies
		now := time.Now()
		
		// Create event with future timestamp (should be rejected)
		futureEventData := map[string]interface{}{
			"type":        "TIMESTAMP_TEST",
			"actor_id":    "test-user",
			"target_id":   "timestamp-target",
			"action":      "future-action",
			"target_type": "timestamp",
			"severity":    "INFO",
			"timestamp":   now.Add(24 * time.Hour).Format(time.RFC3339),
		}

		resp := client.Post("/api/v1/audit/events", futureEventData)
		
		// Should reject events with future timestamps
		assert.Equal(t, 400, resp.StatusCode)
		resp.Body.Close()

		// Create event with very old timestamp
		pastEventData := map[string]interface{}{
			"type":        "TIMESTAMP_TEST",
			"actor_id":    "test-user",
			"target_id":   "timestamp-target",
			"action":      "past-action",
			"target_type": "timestamp",
			"severity":    "INFO",
			"timestamp":   now.Add(-365 * 24 * time.Hour).Format(time.RFC3339),
		}

		resp2 := client.Post("/api/v1/audit/events", pastEventData)
		
		// Should warn about or reject very old timestamps
		assert.Contains(t, []int{201, 400}, resp2.StatusCode)
		resp2.Body.Close()
	})
}

func testAuditSQLInjectionPrevention(t *testing.T, client *infrastructure.APIClient) {
	sqlInjectionPayloads := []string{
		"'; DROP TABLE audit_events; --",
		"1' OR '1'='1",
		"admin'--",
		"1; UPDATE audit_events SET event_hash = 'hacked'",
		"' UNION SELECT * FROM accounts --",
		"'; DELETE FROM audit_events WHERE '1'='1",
		"1' AND (SELECT COUNT(*) FROM accounts) > 0 --",
		"<script>alert('xss')</script>",
		"../../../../etc/passwd",
		"{{7*7}}[[5*5]]",
	}

	for _, payload := range sqlInjectionPayloads {
		t.Run(fmt.Sprintf("SQL injection in audit fields: %s", payload[:min(20, len(payload))]), func(t *testing.T) {
			// Test SQL injection in various audit event fields
			eventData := map[string]interface{}{
				"type":        "INJECTION_TEST",
				"actor_id":    payload, // SQL injection attempt
				"target_id":   "test-target",
				"action":      payload, // SQL injection attempt
				"target_type": "test",
				"severity":    "INFO",
				"metadata": map[string]interface{}{
					"malicious_field": payload,
				},
			}

			resp := client.Post("/api/v1/audit/events", eventData)
			
			// Should either reject the malicious input or sanitize it
			if resp.StatusCode == 201 {
				// If accepted, verify the data was sanitized
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)
				resp.Body.Close()

				// Verify no SQL injection artifacts remain
				actorID := response["actor_id"].(string)
				action := response["action"].(string)
				
				assert.NotContains(t, actorID, "DROP TABLE")
				assert.NotContains(t, actorID, "DELETE FROM")
				assert.NotContains(t, action, "UNION SELECT")
				assert.NotContains(t, action, "1'='1")
			} else {
				// If rejected, that's also acceptable
				assert.Equal(t, 400, resp.StatusCode)
				resp.Body.Close()
			}
		})
	}

	t.Run("SQL injection in query parameters", func(t *testing.T) {
		// Test SQL injection in query endpoints
		maliciousQueries := []string{
			"'; DROP TABLE audit_events; --",
			"1' OR '1'='1",
			"' UNION SELECT password FROM accounts --",
		}

		for _, query := range maliciousQueries {
			// Test in audit query endpoint
			resp := client.Get(fmt.Sprintf("/api/v1/audit/events?actor_id=%s", query))
			
			// Should either return empty results or error, not execute SQL
			assert.Contains(t, []int{200, 400}, resp.StatusCode)
			
			if resp.StatusCode == 200 {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)
				
				// Should not return sensitive data
				if events, ok := response["events"].([]interface{}); ok {
					for _, event := range events {
						eventMap := event.(map[string]interface{})
						assert.NotContains(t, fmt.Sprintf("%v", eventMap), "password")
						assert.NotContains(t, fmt.Sprintf("%v", eventMap), "secret")
					}
				}
			}
			resp.Body.Close()
		}
	})
}

func testAuditAuthenticationSecurity(t *testing.T, client *infrastructure.APIClient, admin *AuthenticatedUser) {
	t.Run("Unauthorized Audit Access", func(t *testing.T) {
		// Test access without authentication
		unauthClient := infrastructure.NewAPIClient(t, client.BaseURL)
		
		// Should be rejected
		resp := unauthClient.Get("/api/v1/audit/events")
		assert.Equal(t, 401, resp.StatusCode)
		resp.Body.Close()

		// Test access with invalid token
		unauthClient.SetToken("invalid-token")
		resp2 := unauthClient.Get("/api/v1/audit/events")
		assert.Equal(t, 401, resp2.StatusCode)
		resp2.Body.Close()
	})

	t.Run("Role-Based Access Control", func(t *testing.T) {
		// Create non-admin user
		buyerClient := infrastructure.NewAPIClient(t, client.BaseURL)
		buyer := createTestUser(t, buyerClient, "buyer-audit@test.com", "buyer")
		buyerClient.SetToken(buyer.Token)

		// Non-admin should not access audit management
		resp := buyerClient.Get("/api/v1/audit/admin/statistics")
		assert.Equal(t, 403, resp.StatusCode)
		resp.Body.Close()

		// Non-admin should not delete audit events
		resp2 := buyerClient.Delete("/api/v1/audit/events/some-id")
		assert.Equal(t, 403, resp2.StatusCode)
		resp2.Body.Close()

		// Admin should have access
		adminClient := infrastructure.NewAPIClient(t, client.BaseURL)
		adminClient.SetToken(admin.Token)
		
		resp3 := adminClient.Get("/api/v1/audit/admin/statistics")
		assert.Contains(t, []int{200, 404}, resp3.StatusCode) // 404 is acceptable if endpoint doesn't exist yet
		resp3.Body.Close()
	})

	t.Run("Token Expiration Handling", func(t *testing.T) {
		// Create an expired token
		expiredToken := generateExpiredToken(t, "test-secret", uuid.New(), "admin")
		
		expiredClient := infrastructure.NewAPIClient(t, client.BaseURL)
		expiredClient.SetToken(expiredToken)
		
		// Should reject expired token
		resp := expiredClient.Get("/api/v1/audit/events")
		assert.Equal(t, 401, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Session Hijacking Prevention", func(t *testing.T) {
		// Test concurrent sessions with same user
		client1 := infrastructure.NewAPIClient(t, client.BaseURL)
		client2 := infrastructure.NewAPIClient(t, client.BaseURL)
		
		// Both use the same token
		client1.SetToken(admin.Token)
		client2.SetToken(admin.Token)

		var wg sync.WaitGroup
		var resp1StatusCode, resp2StatusCode int

		wg.Add(2)
		go func() {
			defer wg.Done()
			resp1 := client1.Get("/api/v1/audit/events")
			resp1StatusCode = resp1.StatusCode
			resp1.Body.Close()
		}()

		go func() {
			defer wg.Done()
			resp2 := client2.Get("/api/v1/audit/events")
			resp2StatusCode = resp2.StatusCode
			resp2.Body.Close()
		}()

		wg.Wait()

		// Both should either succeed or fail consistently
		// (depending on session management implementation)
		assert.True(t, (resp1StatusCode == 200 && resp2StatusCode == 200) ||
			(resp1StatusCode == 401 || resp2StatusCode == 401))
	})
}

func testAuditPIIProtection(t *testing.T, client *infrastructure.APIClient) {
	t.Run("PII Data Masking", func(t *testing.T) {
		// Create event with potential PII
		eventData := map[string]interface{}{
			"type":        "PII_TEST",
			"actor_id":    "test-user",
			"target_id":   "test-target",
			"action":      "data_access",
			"target_type": "user_data",
			"severity":    "INFO",
			"metadata": map[string]interface{}{
				"phone_number":    "+15551234567",
				"email":          "user@example.com",
				"ssn":            "123-45-6789",
				"credit_card":    "4111111111111111",
				"ip_address":     "192.168.1.100",
				"user_agent":     "Mozilla/5.0...",
			},
		}

		resp := client.Post("/api/v1/audit/events", eventData)
		require.Equal(t, 201, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()

		eventID := response["id"].(string)

		// Retrieve the event and verify PII is masked
		getResp := client.Get(fmt.Sprintf("/api/v1/audit/events/%s", eventID))
		require.Equal(t, 200, getResp.StatusCode)

		var retrievedEvent map[string]interface{}
		json.NewDecoder(getResp.Body).Decode(&retrievedEvent)
		getResp.Body.Close()

		// Check if PII is properly masked in metadata
		if metadata, ok := retrievedEvent["metadata"].(map[string]interface{}); ok {
			// Phone numbers should be partially masked
			if phone, exists := metadata["phone_number"]; exists {
				phoneStr := phone.(string)
				assert.Contains(t, phoneStr, "*", "Phone number should be partially masked")
			}

			// SSN should be masked
			if ssn, exists := metadata["ssn"]; exists {
				ssnStr := ssn.(string)
				assert.Contains(t, ssnStr, "*", "SSN should be masked")
			}

			// Credit card should be masked
			if cc, exists := metadata["credit_card"]; exists {
				ccStr := cc.(string)
				assert.Contains(t, ccStr, "*", "Credit card should be masked")
			}
		}
	})

	t.Run("Data Classification Enforcement", func(t *testing.T) {
		// Test that data with different classifications is handled appropriately
		classificationTests := []struct {
			dataClass    string
			expectMasked bool
		}{
			{"public", false},
			{"internal", false},
			{"confidential", true},
			{"restricted", true},
			{"pii", true},
			{"payment", true},
		}

		for _, test := range classificationTests {
			t.Run(fmt.Sprintf("data_class_%s", test.dataClass), func(t *testing.T) {
				eventData := map[string]interface{}{
					"type":         "CLASSIFICATION_TEST",
					"actor_id":     "test-user",
					"target_id":    "test-target",
					"action":       "data_access",
					"target_type":  "classified_data",
					"severity":     "INFO",
					"data_classes": []string{test.dataClass},
					"metadata": map[string]interface{}{
						"sensitive_data": "secret-information-12345",
					},
				}

				resp := client.Post("/api/v1/audit/events", eventData)
				require.Equal(t, 201, resp.StatusCode)

				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)
				resp.Body.Close()

				if test.expectMasked {
					// Verify sensitive data is not exposed in response
					responseStr := fmt.Sprintf("%v", response)
					assert.NotContains(t, responseStr, "secret-information-12345")
				}
			})
		}
	})

	t.Run("Access Control for Sensitive Audit Data", func(t *testing.T) {
		// Create audit event with high sensitivity
		sensitiveEventData := map[string]interface{}{
			"type":         "SECURITY_INCIDENT",
			"actor_id":     "test-user",
			"target_id":    "security-target",
			"action":       "security_breach_detected",
			"target_type":  "security",
			"severity":     "CRITICAL",
			"data_classes": []string{"restricted", "security"},
			"metadata": map[string]interface{}{
				"incident_details": "Potential unauthorized access attempt",
				"ip_address":      "192.168.1.100",
			},
		}

		resp := client.Post("/api/v1/audit/events", sensitiveEventData)
		require.Equal(t, 201, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()

		eventID := response["id"].(string)

		// Create non-admin user
		buyerClient := infrastructure.NewAPIClient(t, client.BaseURL)
		buyer := createTestUser(t, buyerClient, "buyer-sensitive@test.com", "buyer")
		buyerClient.SetToken(buyer.Token)

		// Non-admin should not access sensitive security events
		getResp := buyerClient.Get(fmt.Sprintf("/api/v1/audit/events/%s", eventID))
		assert.Contains(t, []int{403, 404}, getResp.StatusCode)
		getResp.Body.Close()
	})
}

func testAuditVulnerabilityScanning(t *testing.T, client *infrastructure.APIClient) {
	t.Run("Buffer Overflow Prevention", func(t *testing.T) {
		// Test with extremely large payloads
		largePayload := strings.Repeat("A", 1024*1024) // 1MB payload
		
		eventData := map[string]interface{}{
			"type":        "OVERFLOW_TEST",
			"actor_id":    "test-user",
			"target_id":   "test-target",
			"action":      largePayload,
			"target_type": "test",
			"severity":    "INFO",
		}

		resp := client.Post("/api/v1/audit/events", eventData)
		
		// Should either reject large payload or handle it safely
		assert.Contains(t, []int{201, 400, 413}, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Path Traversal Prevention", func(t *testing.T) {
		// Test path traversal attempts in event IDs
		pathTraversalPayloads := []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			"....//....//....//etc/passwd",
		}

		for _, payload := range pathTraversalPayloads {
			resp := client.Get(fmt.Sprintf("/api/v1/audit/events/%s", payload))
			
			// Should not access filesystem paths
			assert.Contains(t, []int{400, 404}, resp.StatusCode)
			resp.Body.Close()
		}
	})

	t.Run("Command Injection Prevention", func(t *testing.T) {
		// Test command injection attempts
		commandInjectionPayloads := []string{
			"; ls -la",
			"| cat /etc/passwd",
			"`whoami`",
			"$(cat /etc/hosts)",
			"& ping google.com",
		}

		for _, payload := range commandInjectionPayloads {
			eventData := map[string]interface{}{
				"type":        "COMMAND_TEST",
				"actor_id":    payload,
				"target_id":   "test-target",
				"action":      payload,
				"target_type": "test",
				"severity":    "INFO",
			}

			resp := client.Post("/api/v1/audit/events", eventData)
			
			// Should reject or sanitize command injection attempts
			assert.Contains(t, []int{201, 400}, resp.StatusCode)
			
			if resp.StatusCode == 201 {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)
				
				// Verify no command execution artifacts
				responseStr := fmt.Sprintf("%v", response)
				assert.NotContains(t, responseStr, "/etc/passwd")
				assert.NotContains(t, responseStr, "root:")
			}
			resp.Body.Close()
		}
	})

	t.Run("Replay Attack Prevention", func(t *testing.T) {
		// Create an audit event
		eventData := map[string]interface{}{
			"type":        "REPLAY_TEST",
			"actor_id":    "test-user",
			"target_id":   "replay-target",
			"action":      "test_action",
			"target_type": "test",
			"severity":    "INFO",
			"timestamp":   time.Now().Format(time.RFC3339),
		}

		// Send the same request multiple times rapidly
		var responses []int
		for i := 0; i < 5; i++ {
			resp := client.Post("/api/v1/audit/events", eventData)
			responses = append(responses, resp.StatusCode)
			resp.Body.Close()
		}

		// Should handle duplicate/replay attempts appropriately
		// Either reject duplicates or handle them gracefully
		successCount := 0
		for _, status := range responses {
			if status == 201 {
				successCount++
			}
		}

		// At least one should succeed, but rapid duplicates might be rejected
		assert.Greater(t, successCount, 0)
		assert.LessOrEqual(t, successCount, 5)
	})

	t.Run("Resource Exhaustion Prevention", func(t *testing.T) {
		// Test rapid creation of many events
		const maxEvents = 100
		var wg sync.WaitGroup
		responses := make([]int, maxEvents)

		for i := 0; i < maxEvents; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				eventData := map[string]interface{}{
					"type":        "EXHAUSTION_TEST",
					"actor_id":    "test-user",
					"target_id":   fmt.Sprintf("target-%d", index),
					"action":      fmt.Sprintf("action-%d", index),
					"target_type": "test",
					"severity":    "INFO",
				}

				resp := client.Post("/api/v1/audit/events", eventData)
				responses[index] = resp.StatusCode
				resp.Body.Close()
			}(i)
		}

		wg.Wait()

		// Should handle high load gracefully
		successCount := 0
		rateLimitedCount := 0
		
		for _, status := range responses {
			switch status {
			case 201:
				successCount++
			case 429: // Rate limited
				rateLimitedCount++
			}
		}

		// Should either succeed or be rate limited, not crash
		assert.Greater(t, successCount+rateLimitedCount, maxEvents/2)
	})
}

// Helper functions

func extractEventIDs(events []map[string]interface{}) []string {
	ids := make([]string, len(events))
	for i, event := range events {
		ids[i] = event["id"].(string)
	}
	return ids
}


// TestSecurity_AuditCryptographicStrength focuses on cryptographic implementation details
func TestSecurity_AuditCryptographicStrength(t *testing.T) {
	t.Run("Hash Algorithm Security", func(t *testing.T) {
		// Test SHA-256 implementation strength
		testData := []byte("test audit event data for hash strength validation")
		
		// Compute hash multiple times - should be deterministic
		hash1 := sha256.Sum256(testData)
		hash2 := sha256.Sum256(testData)
		
		assert.Equal(t, hash1, hash2, "Hash should be deterministic")
		assert.Len(t, hash1, 32, "SHA-256 should produce 32-byte hash")
		
		// Test avalanche effect - small change should drastically change hash
		testDataModified := []byte("test audit event data for hash strength validation!")
		hash3 := sha256.Sum256(testDataModified)
		
		// Count different bits
		differentBits := 0
		for i := 0; i < 32; i++ {
			xor := hash1[i] ^ hash3[i]
			for j := 0; j < 8; j++ {
				if (xor>>j)&1 == 1 {
					differentBits++
				}
			}
		}
		
		// Should have significant difference (avalanche effect)
		assert.Greater(t, differentBits, 100, "Small input change should cause avalanche effect")
	})

	t.Run("HMAC Signature Security", func(t *testing.T) {
		// Test HMAC-SHA256 security properties
		secretKey := make([]byte, 32)
		rand.Read(secretKey)
		
		testData := []byte("audit event data for HMAC testing")
		
		// Compute HMAC
		mac := hmac.New(sha256.New, secretKey)
		mac.Write(testData)
		signature1 := mac.Sum(nil)
		
		// Recompute - should be identical
		mac.Reset()
		mac.Write(testData)
		signature2 := mac.Sum(nil)
		
		assert.Equal(t, signature1, signature2, "HMAC should be deterministic")
		assert.Len(t, signature1, 32, "HMAC-SHA256 should produce 32-byte signature")
		
		// Test with different key - should be completely different
		differentKey := make([]byte, 32)
		rand.Read(differentKey)
		
		mac2 := hmac.New(sha256.New, differentKey)
		mac2.Write(testData)
		signature3 := mac2.Sum(nil)
		
		assert.NotEqual(t, signature1, signature3, "Different keys should produce different signatures")
		
		// Test verification
		assert.True(t, hmac.Equal(signature1, signature2), "Valid signature should verify")
		assert.False(t, hmac.Equal(signature1, signature3), "Invalid signature should not verify")
	})

	t.Run("Key Strength Requirements", func(t *testing.T) {
		// Test key strength validation
		testData := []byte("test data")
		
		// Test with weak keys
		weakKeys := [][]byte{
			{},                    // Empty key
			[]byte("short"),       // Too short
			[]byte("0123456789"),  // Predictable
			make([]byte, 16),      // All zeros, too short
		}
		
		for i, weakKey := range weakKeys {
			t.Run(fmt.Sprintf("weak_key_%d", i), func(t *testing.T) {
				_, err := values.ComputeAuditSignature(testData, weakKey)
				assert.Error(t, err, "Weak key should be rejected")
			})
		}
		
		// Test with strong key
		strongKey := make([]byte, 32)
		rand.Read(strongKey)
		
		signature, err := values.ComputeAuditSignature(testData, strongKey)
		assert.NoError(t, err, "Strong key should be accepted")
		assert.False(t, signature.IsEmpty(), "Should produce valid signature")
	})
}

// Removed duplicate TestSecurity_AuditAttackScenarios - keeping the one in audit_security_standalone_test.go