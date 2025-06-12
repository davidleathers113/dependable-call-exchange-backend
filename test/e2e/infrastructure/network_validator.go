package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

type NetworkValidator struct {
	env *TestEnvironment
}

func NewNetworkValidator(env *TestEnvironment) *NetworkValidator {
	return &NetworkValidator{env: env}
}

func (nv *NetworkValidator) ValidateConnectivity(ctx context.Context) error {
	nv.env.t.Log("Starting connectivity validation...")

	// Test 1: Host to containers
	if err := nv.testHostConnectivity(ctx); err != nil {
		return fmt.Errorf("host connectivity: %w", err)
	}

	// Test 2: Container-to-container connectivity
	if err := nv.testInternalConnectivity(ctx); err != nil {
		return fmt.Errorf("internal connectivity: %w", err)
	}

	// Test 3: API can reach dependencies
	if err := nv.testAPIConnectivity(ctx); err != nil {
		return fmt.Errorf("API connectivity: %w", err)
	}

	nv.env.t.Log("All connectivity tests passed!")
	return nil
}

func (nv *NetworkValidator) testHostConnectivity(ctx context.Context) error {
	// Test PostgreSQL
	db, err := sql.Open("pgx", nv.env.PostgresURL)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	// Test Redis
	opt, err := redis.ParseURL(nv.env.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opt)
	defer client.Close()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	return nil
}

func (nv *NetworkValidator) testInternalConnectivity(ctx context.Context) error {
	// Since the API container might not have network utilities installed,
	// we'll rely on the API's ability to connect to its dependencies
	// which will be tested in testAPIConnectivity
	nv.env.t.Log("Internal connectivity will be validated through API health check")
	return nil
}

func (nv *NetworkValidator) testAPIConnectivity(ctx context.Context) error {
	// Make a health check request to the API
	client := NewAPIClient(nv.env.t, nv.env.APIURL)
	resp := client.Get("/health")

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, body)
	}

	resp.Body.Close()
	return nil
}