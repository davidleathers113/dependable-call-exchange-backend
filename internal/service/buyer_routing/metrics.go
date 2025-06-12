package buyer_routing

import (
	"context"

	"github.com/google/uuid"
)

// metricsNoop is a no-op implementation of BuyerMetrics
type metricsNoop struct{}

// NewNoopMetrics creates a no-op metrics implementation
func NewNoopMetrics() BuyerMetrics {
	return &metricsNoop{}
}

func (m *metricsNoop) RecordBuyerRoutingDecision(ctx context.Context, decision *BuyerRoutingDecision) error {
	// No-op
	return nil
}

func (m *metricsNoop) RecordBuyerPerformance(ctx context.Context, buyerID uuid.UUID, callID uuid.UUID, metrics map[string]interface{}) error {
	// No-op
	return nil
}

func (m *metricsNoop) GetBuyerQualityScore(ctx context.Context, buyerID uuid.UUID) (float64, error) {
	// Return a default score
	return 0.75, nil
}
