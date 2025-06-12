package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// APIContainerOptions configures how the API container is started
type APIContainerOptions struct {
	// UsePrebuiltBinary builds the binary locally and mounts it (fast)
	UsePrebuiltBinary bool
	
	// DockerfilePath for building from source (slower but more isolated)
	DockerfilePath string
	
	// Environment variables for the API
	Environment map[string]string
	
	// HealthCheckEndpoint to verify API is ready
	HealthCheckEndpoint string
	
	// StartupTimeout for the API to become healthy
	StartupTimeout time.Duration
}

// DefaultAPIContainerOptions returns sensible defaults for E2E testing
func DefaultAPIContainerOptions() *APIContainerOptions {
	return &APIContainerOptions{
		UsePrebuiltBinary:   true, // Fast by default
		HealthCheckEndpoint: "/health",
		StartupTimeout:      30 * time.Second,
		Environment:         make(map[string]string),
	}
}

// StartAPIContainer starts the API container with proper health checks
func StartAPIContainer(ctx context.Context, t *testing.T, postgresURL, redisURL string, opts *APIContainerOptions) (testcontainers.Container, string, error) {
	if opts == nil {
		opts = DefaultAPIContainerOptions()
	}

	// For Docker Desktop on macOS, containers need to use host.docker.internal
	// to connect to services running on the host
	dockerPostgresURL := postgresURL
	dockerRedisURL := redisURL
	
	// Replace localhost/127.0.0.1 with host.docker.internal for Docker Desktop
	if runtime.GOOS == "darwin" {
		dockerPostgresURL = replaceHostWithDockerInternal(postgresURL)
		dockerRedisURL = replaceHostWithDockerInternal(redisURL)
	}

	// Set required environment variables
	opts.Environment["DCE_DATABASE_URL"] = dockerPostgresURL
	opts.Environment["DCE_REDIS_URL"] = dockerRedisURL
	opts.Environment["DCE_ENVIRONMENT"] = "test"
	opts.Environment["DCE_LOG_LEVEL"] = "debug"
	opts.Environment["DCE_SECURITY_JWT_SECRET"] = "test-secret-for-e2e-testing"
	opts.Environment["PORT"] = "8080"
	// Disable telemetry to avoid OpenTelemetry conflicts during tests
	opts.Environment["DCE_TELEMETRY_ENABLED"] = "false"

	if opts.UsePrebuiltBinary {
		return startAPIWithPrebuiltBinary(ctx, t, opts)
	}
	
	return startAPIWithDockerBuild(ctx, t, opts)
}

// startAPIWithPrebuiltBinary builds the binary locally and mounts it into a lightweight container
func startAPIWithPrebuiltBinary(ctx context.Context, t *testing.T, opts *APIContainerOptions) (testcontainers.Container, string, error) {
	// Get project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	
	// Build the binary locally (much faster than Docker build)
	binaryPath := filepath.Join(projectRoot, "test-api-binary")
	t.Logf("Building API binary locally at %s", binaryPath)
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(projectRoot, "main.go"))
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("failed to build API binary: %w\nOutput: %s", err, output)
	}
	
	// Ensure cleanup
	t.Cleanup(func() {
		os.Remove(binaryPath)
	})
	
	// Create container request
	req := testcontainers.ContainerRequest{
		Image: "alpine:3.18", // Lightweight base image
		Env:   opts.Environment,
		ExposedPorts: []string{"8080/tcp"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      binaryPath,
				ContainerFilePath: "/app/api",
				FileMode:          0755,
			},
		},
		Cmd: []string{"/app/api"},
		WaitingFor: wait.ForAll(
			wait.ForHTTP(opts.HealthCheckEndpoint).
				WithPort("8080/tcp").
				WithStartupTimeout(opts.StartupTimeout).
				WithStatusCodeMatcher(func(status int) bool {
					return status >= 200 && status < 300
				}),
			wait.ForLog("starting HTTP server").
				WithStartupTimeout(opts.StartupTimeout),
		),
	}
	
	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start API container: %w", err)
	}
	
	// Get the API URL
	host, err := container.Host(ctx)
	if err != nil {
		return container, "", fmt.Errorf("failed to get container host: %w", err)
	}
	
	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return container, "", fmt.Errorf("failed to get mapped port: %w", err)
	}
	
	apiURL := fmt.Sprintf("http://%s:%s", host, port.Port())
	t.Logf("API container started at %s", apiURL)
	
	return container, apiURL, nil
}

// startAPIWithDockerBuild builds from Dockerfile (slower but more production-like)
func startAPIWithDockerBuild(ctx context.Context, t *testing.T, opts *APIContainerOptions) (testcontainers.Container, string, error) {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	
	dockerfilePath := opts.DockerfilePath
	if dockerfilePath == "" {
		dockerfilePath = filepath.Join(projectRoot, "Dockerfile.dev")
	}
	
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    projectRoot,
			Dockerfile: dockerfilePath,
		},
		Env:          opts.Environment,
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForHTTP(opts.HealthCheckEndpoint).
				WithPort("8080/tcp").
				WithStartupTimeout(opts.StartupTimeout),
			wait.ForLog("starting HTTP server").
				WithStartupTimeout(opts.StartupTimeout),
		),
	}
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start API container: %w", err)
	}
	
	host, err := container.Host(ctx)
	if err != nil {
		return container, "", fmt.Errorf("failed to get container host: %w", err)
	}
	
	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return container, "", fmt.Errorf("failed to get mapped port: %w", err)
	}
	
	apiURL := fmt.Sprintf("http://%s:%s", host, port.Port())
	return container, apiURL, nil
}

// WaitForAPIReady performs additional readiness checks beyond basic HTTP
func WaitForAPIReady(ctx context.Context, apiURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Try to connect to the health endpoint
			resp, err := httpClient.Get(apiURL + "/health")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					return nil
				}
			}
			
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	return fmt.Errorf("API did not become ready within %s", timeout)
}

// httpClient is a shared HTTP client with reasonable timeouts
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// replaceHostWithDockerInternal replaces localhost addresses with host.docker.internal
// for Docker Desktop compatibility
func replaceHostWithDockerInternal(url string) string {
	// Common patterns to replace
	replacements := map[string]string{
		"localhost":  "host.docker.internal",
		"127.0.0.1":  "host.docker.internal",
		"[::1]":      "host.docker.internal",
		"0.0.0.0":    "host.docker.internal",
	}
	
	result := url
	for old, new := range replacements {
		if strings.Contains(result, old) {
			// For URLs like postgres://user:pass@localhost:5432/db
			result = strings.ReplaceAll(result, "@"+old, "@"+new)
			// For URLs like redis://localhost:6379
			result = strings.ReplaceAll(result, "//"+old, "//"+new)
		}
	}
	
	return result
}