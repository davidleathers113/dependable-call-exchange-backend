package infrastructure

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerNetworking(t *testing.T) {
	// Enable container networking for this test
	os.Setenv("E2E_USE_CONTAINER_NETWORK", "true")
	defer os.Unsetenv("E2E_USE_CONTAINER_NETWORK")

	// Create test environment
	env := NewTestEnvironment(t)
	require.NotNil(t, env, "Failed to create test environment")

	// Verify network manager was created
	assert.NotNil(t, env.networkManager, "Network manager should be created")

	// Verify internal URLs are set
	assert.NotEmpty(t, env.postgresInternalURL, "PostgreSQL internal URL should be set")
	assert.NotEmpty(t, env.redisInternalURL, "Redis internal URL should be set")

	// Verify external URLs are set for test access
	assert.NotEmpty(t, env.PostgresURL, "PostgreSQL external URL should be set")
	assert.NotEmpty(t, env.RedisURL, "Redis external URL should be set")
	assert.NotEmpty(t, env.APIURL, "API URL should be set")

	// Test database connectivity
	var result int
	err := env.DB.QueryRow("SELECT 1").Scan(&result)
	require.NoError(t, err, "Failed to query database")
	assert.Equal(t, 1, result, "Database query should return 1")

	// Test Redis connectivity
	ctx := env.ctx
	err = env.RedisClient.Set(ctx, "test-key", "test-value", 0).Err()
	require.NoError(t, err, "Failed to set Redis key")

	val, err := env.RedisClient.Get(ctx, "test-key").Result()
	require.NoError(t, err, "Failed to get Redis key")
	assert.Equal(t, "test-value", val, "Redis value should match")

	// Test API health endpoint
	client := NewAPIClient(t, env.APIURL)
	resp := client.Get("/health")
	assert.Equal(t, 200, resp.StatusCode, "Health endpoint should return 200")
	resp.Body.Close()
}

func TestLegacyNetworking(t *testing.T) {
	// Ensure legacy mode is used (default)
	os.Unsetenv("E2E_USE_CONTAINER_NETWORK")

	// Create test environment
	env := NewTestEnvironment(t)
	require.NotNil(t, env, "Failed to create test environment")

	// Verify network manager was NOT created
	assert.Nil(t, env.networkManager, "Network manager should not be created in legacy mode")

	// Verify external URLs are set
	assert.NotEmpty(t, env.PostgresURL, "PostgreSQL URL should be set")
	assert.NotEmpty(t, env.RedisURL, "Redis URL should be set")
	assert.NotEmpty(t, env.APIURL, "API URL should be set")

	// Test basic connectivity
	var result int
	err := env.DB.QueryRow("SELECT 1").Scan(&result)
	require.NoError(t, err, "Failed to query database")
	assert.Equal(t, 1, result, "Database query should return 1")
}