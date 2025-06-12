package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func TestDockerConnectivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try a simple container without network to test Docker connectivity
	t.Log("Testing Docker connectivity...")
	
	req := testcontainers.ContainerRequest{
		Image: "alpine:3.18",
		Cmd:   []string{"echo", "hello"},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	
	if err != nil {
		t.Logf("Failed to create container: %v", err)
		t.Logf("Make sure Docker is running and accessible")
		t.Logf("If using Docker Desktop, check if it's running")
		t.Logf("If using Colima: docker context use colima")
		t.Logf("If using Rancher Desktop, set TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock")
		t.FailNow()
	}
	
	defer container.Terminate(ctx)

	t.Log("Successfully created test container - Docker connectivity is working")
}

func TestSimpleContainerWithDefaults(t *testing.T) {
	ctx := context.Background()

	// Test with the simplest possible container
	req := testcontainers.ContainerRequest{
		Image: "hello-world:latest",
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	
	if err != nil {
		t.Logf("Error creating container: %v", err)
		t.Log("This might be a Docker connectivity issue")
		t.FailNow()
	}
	
	defer container.Terminate(ctx)

	// Check container state
	state, err := container.State(ctx)
	require.NoError(t, err)
	t.Logf("Container state: %+v", state)
}