//go:build security

package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecuritySetup verifies that the security test infrastructure is properly configured
func TestSecuritySetup(t *testing.T) {
	t.Run("Verify test build tag", func(t *testing.T) {
		// This test only runs when the security build tag is present
		assert.True(t, true, "Security build tag is active")
	})
	
	t.Run("Verify helper functions", func(t *testing.T) {
		// Test that helper functions are available
		assert.NotNil(t, contains, "contains helper function should be available")
		assert.NotNil(t, extractErrorMessage, "extractErrorMessage helper function should be available")
		assert.NotNil(t, generateExpiredToken, "generateExpiredToken helper function should be available")
	})
}
