//go:build e2e

package infrastructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleComposeApproach(t *testing.T) {
	// That's it! Just one line to set up the entire test environment
	env := NewSimpleTestEnvironment(t)
	
	// Test database connectivity
	var count int
	err := env.DB.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	
	// Test Redis connectivity
	ctx := context.Background()
	err = env.RedisClient.Set(ctx, "test-key", "test-value", 0).Err()
	require.NoError(t, err)
	
	val, err := env.RedisClient.Get(ctx, "test-key").Result()
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)
	
	// Skip API test since we're not starting the API container
	// Individual tests can start their own API if needed
	t.Log("Database and Redis infrastructure ready for testing")
}

func TestModularApproach(t *testing.T) {
	// Alternative: use individual testcontainers modules
	env := NewModularTestEnvironment(t)
	
	// Same tests work with either approach
	var count int
	err := env.DB.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestParallelExecution(t *testing.T) {
	// Tests can run in parallel - each gets its own containers
	t.Run("Test1", func(t *testing.T) {
		t.Parallel()
		env := NewSimpleTestEnvironment(t)
		
		// Do test 1 things...
		_ = env
	})
	
	t.Run("Test2", func(t *testing.T) {
		t.Parallel()
		env := NewSimpleTestEnvironment(t)
		
		// Do test 2 things...
		_ = env
	})
}