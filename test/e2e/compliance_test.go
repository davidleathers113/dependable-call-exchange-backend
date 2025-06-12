//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompliance_DNCManagement(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("DNC List Operations", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create admin user
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add number to DNC list
		dncResp := client.Post("/api/v1/compliance/dnc", map[string]interface{}{
			"phone_number": "+14155551234",
			"reason":       "consumer request",
		})
		require.Equal(t, 201, dncResp.StatusCode)
		
		var dncEntry map[string]interface{}
		client.DecodeResponse(dncResp, &dncEntry)
		assert.Equal(t, "+14155551234", dncEntry["phone_number"])
		assert.Equal(t, "consumer request", dncEntry["reason"])
		assert.Equal(t, "internal", dncEntry["list_type"])
		
		// Create buyer
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Try to call DNC number
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "DNC")
		
		// Check DNC status
		client.SetToken(admin.Token)
		checkResp := client.Get("/api/v1/compliance/dnc/+14155551234")
		assert.Equal(t, 200, checkResp.StatusCode)
		
		var dncStatus map[string]interface{}
		client.DecodeResponse(checkResp, &dncStatus)
		assert.Equal(t, true, dncStatus["is_dnc"])
		assert.Equal(t, "consumer request", dncStatus["reason"])
	})
	
	t.Run("Bulk DNC Operations", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create admin user
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Bulk add to DNC list
		bulkDNCResp := client.Post("/api/v1/compliance/dnc/bulk", map[string]interface{}{
			"phone_numbers": []string{
				"+14155551234",
				"+14155551235", 
				"+14155551236",
			},
			"reason": "bulk consumer request",
		})
		require.Equal(t, 201, bulkDNCResp.StatusCode)
		
		var bulkResult map[string]interface{}
		client.DecodeResponse(bulkDNCResp, &bulkResult)
		assert.Equal(t, float64(3), bulkResult["added_count"])
		
		// Verify all numbers are in DNC
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		for _, number := range []string{"+14155551234", "+14155551235", "+14155551236"} {
			callResp := client.Post("/api/v1/calls", map[string]interface{}{
				"from_number": "+18005551234",
				"to_number":   number,
			})
			assert.Equal(t, 403, callResp.StatusCode, "Number %s should be blocked", number)
		}
	})
	
	t.Run("DNC List Removal", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add to DNC
		dncResp := client.Post("/api/v1/compliance/dnc", map[string]interface{}{
			"phone_number": "+14155551234",
			"reason":       "consumer request",
		})
		require.Equal(t, 201, dncResp.StatusCode)
		
		// Remove from DNC
		removeResp := client.Delete("/api/v1/compliance/dnc/+14155551234")
		assert.Equal(t, 204, removeResp.StatusCode)
		
		// Verify number can now be called
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 201, callResp.StatusCode)
	})
}

func TestCompliance_TCPATimeRestrictions(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("TCPA Hours Configuration", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Set TCPA hours
		tcpaResp := client.Put("/api/v1/compliance/tcpa/hours", map[string]interface{}{
			"start_time": "09:00",
			"end_time":   "20:00",
			"timezone":   "America/New_York",
		})
		require.Equal(t, 200, tcpaResp.StatusCode)
		
		var tcpaConfig map[string]interface{}
		client.DecodeResponse(tcpaResp, &tcpaConfig)
		assert.Equal(t, "09:00", tcpaConfig["start_time"])
		assert.Equal(t, "20:00", tcpaConfig["end_time"])
		assert.Equal(t, "America/New_York", tcpaConfig["timezone"])
		
		// Get TCPA hours
		getResp := client.Get("/api/v1/compliance/tcpa/hours")
		assert.Equal(t, 200, getResp.StatusCode)
		
		var tcpaHours map[string]interface{}
		client.DecodeResponse(getResp, &tcpaHours)
		assert.Equal(t, "09:00", tcpaHours["start_time"])
		assert.Equal(t, "20:00", tcpaHours["end_time"])
		assert.Equal(t, "America/New_York", tcpaHours["timezone"])
	})
	
	t.Run("TCPA Compliance Check", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Set restrictive TCPA hours (for testing purposes)
		tcpaResp := client.Put("/api/v1/compliance/tcpa/hours", map[string]interface{}{
			"start_time": "10:00",
			"end_time":   "11:00",
			"timezone":   "UTC",
		})
		require.Equal(t, 200, tcpaResp.StatusCode)
		
		// Check TCPA compliance for current time
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Test compliance check endpoint
		checkResp := client.Get("/api/v1/compliance/tcpa/check?phone_number=+14155551234")
		assert.Equal(t, 200, checkResp.StatusCode)
		
		var complianceCheck map[string]interface{}
		client.DecodeResponse(checkResp, &complianceCheck)
		assert.Contains(t, complianceCheck, "allowed")
		assert.Contains(t, complianceCheck, "current_time")
		assert.Contains(t, complianceCheck, "next_allowed_time")
	})
	
	t.Run("State-Specific TCPA Rules", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Set California-specific TCPA rules
		caRulesResp := client.Put("/api/v1/compliance/tcpa/state/CA", map[string]interface{}{
			"start_time": "08:00",
			"end_time":   "21:00",
			"timezone":   "America/Los_Angeles",
			"restrictions": []string{
				"no_robocalls",
				"express_consent_required",
			},
		})
		require.Equal(t, 200, caRulesResp.StatusCode)
		
		// Get state-specific rules
		getCAResp := client.Get("/api/v1/compliance/tcpa/state/CA")
		assert.Equal(t, 200, getCAResp.StatusCode)
		
		var caRules map[string]interface{}
		client.DecodeResponse(getCAResp, &caRules)
		assert.Equal(t, "08:00", caRules["start_time"])
		assert.Equal(t, "America/Los_Angeles", caRules["timezone"])
		assert.Contains(t, caRules["restrictions"], "no_robocalls")
	})
}

func TestCompliance_GeographicRestrictions(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("State-Level Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		// Setup geographic restrictions
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add state restriction
		restrictResp := client.Post("/api/v1/compliance/geographic/restrictions", map[string]interface{}{
			"state":      "CA",
			"restricted": true,
			"reason":     "state regulations",
		})
		require.Equal(t, 201, restrictResp.StatusCode)
		
		var restriction map[string]interface{}
		client.DecodeResponse(restrictResp, &restriction)
		assert.Equal(t, "CA", restriction["state"])
		assert.Equal(t, true, restriction["restricted"])
		assert.Equal(t, "state regulations", restriction["reason"])
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Try to call California number
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234", // 415 is San Francisco
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "geographic restriction")
	})
	
	t.Run("Time Zone Based Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add timezone-based restriction
		timezoneResp := client.Post("/api/v1/compliance/geographic/timezone-restrictions", map[string]interface{}{
			"timezone":      "America/New_York",
			"restricted_hours": map[string]interface{}{
				"start": "22:00",
				"end":   "08:00",
			},
			"reason": "quiet hours enforcement",
		})
		require.Equal(t, 201, timezoneResp.StatusCode)
		
		// Get timezone restrictions
		getResp := client.Get("/api/v1/compliance/geographic/timezone-restrictions/America/New_York")
		assert.Equal(t, 200, getResp.StatusCode)
		
		var tzRestriction map[string]interface{}
		client.DecodeResponse(getResp, &tzRestriction)
		assert.Equal(t, "America/New_York", tzRestriction["timezone"])
		assert.Contains(t, tzRestriction, "restricted_hours")
	})
	
	t.Run("International Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Block international calls
		intlResp := client.Post("/api/v1/compliance/geographic/international", map[string]interface{}{
			"country_code": "1",
			"allowed":      true,
		})
		require.Equal(t, 200, intlResp.StatusCode)
		
		// Block specific country
		blockResp := client.Post("/api/v1/compliance/geographic/international", map[string]interface{}{
			"country_code": "44", // UK
			"allowed":      false,
			"reason":       "international restrictions",
		})
		require.Equal(t, 200, blockResp.StatusCode)
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Try to call UK number
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+442071234567", // UK London number
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "international restrictions")
		
		// US number should work
		usCallResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 201, usCallResp.StatusCode)
	})
}

func TestCompliance_ConsentManagement(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Consent Recording and Verification", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Record consent
		consentResp := client.Post("/api/v1/compliance/consent", map[string]interface{}{
			"phone_number": "+14155551234",
			"consent_type": "express_written",
			"purpose":      "marketing_calls",
			"granted_at":   time.Now().Format(time.RFC3339),
			"source":       "web_form",
			"ip_address":   "192.168.1.100",
		})
		require.Equal(t, 201, consentResp.StatusCode)
		
		var consent map[string]interface{}
		client.DecodeResponse(consentResp, &consent)
		assert.Equal(t, "+14155551234", consent["phone_number"])
		assert.Equal(t, "express_written", consent["consent_type"])
		assert.Equal(t, "marketing_calls", consent["purpose"])
		
		// Verify consent exists
		checkResp := client.Get("/api/v1/compliance/consent/+14155551234")
		assert.Equal(t, 200, checkResp.StatusCode)
		
		var consentCheck map[string]interface{}
		client.DecodeResponse(checkResp, &consentCheck)
		assert.Equal(t, true, consentCheck["has_consent"])
		assert.Equal(t, "express_written", consentCheck["consent_type"])
	})
	
	t.Run("Consent Revocation", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Record consent first
		consentResp := client.Post("/api/v1/compliance/consent", map[string]interface{}{
			"phone_number": "+14155551234",
			"consent_type": "express_written",
			"purpose":      "marketing_calls",
			"granted_at":   time.Now().Format(time.RFC3339),
		})
		require.Equal(t, 201, consentResp.StatusCode)
		
		// Revoke consent
		revokeResp := client.Post("/api/v1/compliance/consent/+14155551234/revoke", map[string]interface{}{
			"revoked_at": time.Now().Format(time.RFC3339),
			"reason":     "customer_request",
			"source":     "phone_call",
		})
		require.Equal(t, 200, revokeResp.StatusCode)
		
		// Verify consent is revoked
		checkResp := client.Get("/api/v1/compliance/consent/+14155551234")
		assert.Equal(t, 200, checkResp.StatusCode)
		
		var consentCheck map[string]interface{}
		client.DecodeResponse(checkResp, &consentCheck)
		assert.Equal(t, false, consentCheck["has_consent"])
		assert.Equal(t, "revoked", consentCheck["status"])
		
		// Try to make call without consent
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
			"purpose":     "marketing_calls",
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "consent")
	})
	
	t.Run("Consent Audit Trail", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		phoneNumber := "+14155551234"
		
		// Record initial consent
		client.Post("/api/v1/compliance/consent", map[string]interface{}{
			"phone_number": phoneNumber,
			"consent_type": "express_written",
			"purpose":      "marketing_calls",
			"granted_at":   time.Now().Add(-24*time.Hour).Format(time.RFC3339),
		})
		
		// Update consent
		client.Put("/api/v1/compliance/consent/"+phoneNumber, map[string]interface{}{
			"consent_type": "express_oral",
			"purpose":      "sales_calls",
			"updated_at":   time.Now().Add(-12*time.Hour).Format(time.RFC3339),
		})
		
		// Revoke consent
		client.Post("/api/v1/compliance/consent/"+phoneNumber+"/revoke", map[string]interface{}{
			"revoked_at": time.Now().Format(time.RFC3339),
			"reason":     "customer_request",
		})
		
		// Get audit trail
		auditResp := client.Get("/api/v1/compliance/consent/" + phoneNumber + "/audit")
		assert.Equal(t, 200, auditResp.StatusCode)
		
		var auditTrail map[string]interface{}
		client.DecodeResponse(auditResp, &auditTrail)
		
		events := auditTrail["events"].([]interface{})
		assert.Len(t, events, 3) // created, updated, revoked
		
		// Verify chronological order
		for i := 1; i < len(events); i++ {
			prev := events[i-1].(map[string]interface{})
			curr := events[i].(map[string]interface{})
			
			prevTime, _ := time.Parse(time.RFC3339, prev["timestamp"].(string))
			currTime, _ := time.Parse(time.RFC3339, curr["timestamp"].(string))
			
			assert.True(t, prevTime.Before(currTime), "Events should be in chronological order")
		}
	})
}

func TestCompliance_ViolationTracking(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Compliance Violation Detection", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add number to DNC list
		client.Post("/api/v1/compliance/dnc", map[string]interface{}{
			"phone_number": "+14155551234",
			"reason":       "consumer request",
		})
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Attempt to call DNC number (should create violation)
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 403, callResp.StatusCode)
		
		// Check for violation record
		client.SetToken(admin.Token)
		violationsResp := client.Get("/api/v1/compliance/violations")
		assert.Equal(t, 200, violationsResp.StatusCode)
		
		var violations map[string]interface{}
		client.DecodeResponse(violationsResp, &violations)
		
		violationList := violations["violations"].([]interface{})
		assert.Len(t, violationList, 1)
		
		violation := violationList[0].(map[string]interface{})
		assert.Equal(t, "DNC_VIOLATION", violation["violation_type"])
		assert.Equal(t, "+14155551234", violation["phone_number"])
		assert.Equal(t, "attempted", violation["status"])
	})
	
	t.Run("Violation Escalation", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		
		// Add multiple numbers to DNC
		dncNumbers := []string{"+14155551234", "+14155551235", "+14155551236"}
		for _, number := range dncNumbers {
			client.Post("/api/v1/compliance/dnc", map[string]interface{}{
				"phone_number": number,
				"reason":       "consumer request",
			})
		}
		
		client.SetToken(buyer.Token)
		
		// Generate multiple violations
		for _, number := range dncNumbers {
			client.Post("/api/v1/calls", map[string]interface{}{
				"from_number": "+18005551234",
				"to_number":   number,
			})
		}
		
		// Check violation count for buyer
		client.SetToken(admin.Token)
		buyerViolationsResp := client.Get("/api/v1/compliance/violations/account/" + buyer.Email)
		assert.Equal(t, 200, buyerViolationsResp.StatusCode)
		
		var buyerViolations map[string]interface{}
		client.DecodeResponse(buyerViolationsResp, &buyerViolations)
		
		assert.Equal(t, float64(3), buyerViolations["violation_count"])
		assert.Equal(t, "warning", buyerViolations["escalation_level"])
		
		// Check if account is flagged
		accountStatusResp := client.Get("/api/v1/accounts/" + buyer.Email + "/compliance-status")
		assert.Equal(t, 200, accountStatusResp.StatusCode)
		
		var accountStatus map[string]interface{}
		client.DecodeResponse(accountStatusResp, &accountStatus)
		assert.Equal(t, "flagged", accountStatus["compliance_status"])
	})
}

// Helper functions specific to compliance tests

func createAuthenticatedUser(t *testing.T, client *infrastructure.APIClient, email, userType string) *AuthenticatedUser {
	// Register user
	registerReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
		"name":     "Test User",
		"type":     userType,
	}
	
	registerResp := client.Post("/api/v1/auth/register", registerReq)
	// Don't require success as user might already exist
	
	// Login
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}
	
	loginResp := client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, loginResp.StatusCode)
	
	var loginResult map[string]interface{}
	client.DecodeResponse(loginResp, &loginResult)
	
	return &AuthenticatedUser{
		Email:        email,
		Token:        loginResult["token"].(string),
		RefreshToken: loginResult["refresh_token"].(string),
	}
}

type AuthenticatedUser struct {
	Email        string
	Token        string
	RefreshToken string
}