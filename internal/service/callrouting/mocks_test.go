package callrouting

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// MetricsCollector mock for tests
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordRoutingDecision(ctx context.Context, decision *RoutingDecision) {
	m.Called(ctx, decision)
}

func (m *MockMetricsCollector) RecordRoutingLatency(ctx context.Context, algorithm string, latency time.Duration) {
	m.Called(ctx, algorithm, latency)
}