package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestNetworkCreation(t *testing.T) {
	ctx := context.Background()

	// Test network creation
	nm, err := NewNetworkManager(ctx, "test-network")
	require.NoError(t, err, "Failed to create network manager")
	defer nm.Cleanup()

	// Verify network
	err = nm.VerifyNetwork()
	require.NoError(t, err, "Network verification failed")

	// Test creating a simple container on the network
	req := testcontainers.ContainerRequest{
		Image:    "alpine:3.18",
		Networks: []string{nm.Name()},
		NetworkAliases: map[string][]string{
			nm.Name(): {"test-container"},
		},
		Cmd: []string{"sh", "-c", "sleep 300"},
		WaitingFor: wait.ForLog("").
			WithStartupTimeout(10 * time.Second).
			WithPollInterval(100 * time.Millisecond),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to create test container")
	defer container.Terminate(ctx)

	// Verify container is running
	state, err := container.State(ctx)
	require.NoError(t, err, "Failed to get container state")
	assert.True(t, state.Running, "Container should be running")
}

func TestMinimalPostgresWithNetwork(t *testing.T) {
	ctx := context.Background()

	// Create network
	nm, err := NewNetworkManager(ctx, "postgres-test")
	require.NoError(t, err, "Failed to create network manager")
	defer nm.Cleanup()

	// Start PostgreSQL with network
	req := testcontainers.ContainerRequest{
		Image:    "postgres:15-alpine",
		Networks: []string{nm.Name()},
		NetworkAliases: map[string][]string{
			nm.Name(): {"postgres-test"},
		},
		Env: map[string]string{
			"POSTGRES_DB":       "test",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer container.Terminate(ctx)

	// Get connection details
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	t.Logf("PostgreSQL started on %s:%s", host, port.Port())
}