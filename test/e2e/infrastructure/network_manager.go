package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
)

type NetworkManager struct {
	Network   *testcontainers.DockerNetwork
	ctx       context.Context
	name      string
	startTime time.Time
}

func NewNetworkManager(ctx context.Context, testName string) (*NetworkManager, error) {
	// Create unique network name with timestamp to avoid conflicts
	networkName := fmt.Sprintf("dce-e2e-%s-%d", testName, time.Now().Unix())

	// Create network with retry logic
	var net *testcontainers.DockerNetwork
	var err error

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		net, err = network.New(ctx,
			network.WithCheckDuplicate(),
			network.WithDriver("bridge"),
			network.WithLabels(map[string]string{
				"test-suite": "dce-e2e",
				"test-name":  testName,
				"created-at": time.Now().Format(time.RFC3339),
			}),
		)

		if err == nil {
			break
		}

		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create network after %d retries: %w", maxRetries, err)
	}

	return &NetworkManager{
		Network:   net,
		ctx:       ctx,
		name:      networkName,
		startTime: time.Now(),
	}, nil
}

func (nm *NetworkManager) Name() string {
	if nm.Network == nil {
		return ""
	}
	return nm.Network.Name
}

func (nm *NetworkManager) Cleanup() error {
	if nm.Network != nil {
		// Log network lifetime for debugging
		lifetime := time.Since(nm.startTime)
		fmt.Printf("Network %s lived for %v\n", nm.name, lifetime)

		return nm.Network.Remove(nm.ctx)
	}
	return nil
}

// VerifyNetwork checks if the network is properly created and accessible
func (nm *NetworkManager) VerifyNetwork() error {
	if nm.Network == nil {
		return fmt.Errorf("network is nil")
	}

	// Additional verification can be added here
	return nil
}