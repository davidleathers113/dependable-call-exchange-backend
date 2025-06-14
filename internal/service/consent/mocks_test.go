package consent

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// Simple mock implementations for testing

type mockConsentRepository struct {
	mock.Mock
	consents map[uuid.UUID]*consent.ConsentAggregate
	err      error
}

func (m *mockConsentRepository) Save(ctx context.Context, c *consent.ConsentAggregate) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockConsentRepository) GetByID(ctx context.Context, id uuid.UUID) (*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.consents[id], nil
}

func (m *mockConsentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	delete(m.consents, id)
	return nil
}

func (m *mockConsentRepository) GetByConsumerAndBusiness(ctx context.Context, consumerID, businessID uuid.UUID) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*consent.ConsentAggregate
	for _, c := range m.consents {
		if c.ConsumerID == consumerID && c.BusinessID == businessID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockConsentRepository) FindActiveConsent(ctx context.Context, consumerID, businessID uuid.UUID, channel consent.Channel) (*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, c := range m.consents {
		if c.ConsumerID == consumerID && c.BusinessID == businessID && c.IsActive() {
			if c.HasChannelConsent(channel) {
				return c, nil
			}
		}
	}
	return nil, nil
}

func (m *mockConsentRepository) FindByPhoneNumber(ctx context.Context, phoneNumber string, businessID uuid.UUID) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*consent.ConsentAggregate
	// This would normally require a join with consumers table, but for mock just return empty
	return result, nil
}

func (m *mockConsentRepository) GetByConsumerAndType(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) (*consent.ConsentAggregate, error) {
	args := m.Called(ctx, consumerID, consentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*consent.ConsentAggregate), args.Error(1)
}

func (m *mockConsentRepository) ListExpired(ctx context.Context, before time.Time) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*consent.ConsentAggregate
	for _, c := range m.consents {
		if c.IsExpired() {
			result = append(result, c)
		}
	}
	return result, nil
}

type mockConsumerRepository struct {
	mock.Mock
	consumers map[string]*consent.Consumer
	err       error
}

func (m *mockConsumerRepository) Save(ctx context.Context, c *consent.Consumer) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockConsumerRepository) GetByPhoneNumber(ctx context.Context, phone string) (*consent.Consumer, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*consent.Consumer), args.Error(1)
}

func (m *mockConsumerRepository) FindOrCreate(ctx context.Context, phoneNumber string, email *string, firstName, lastName string) (*consent.Consumer, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	if existing := m.consumers[phoneNumber]; existing != nil {
		return existing, nil
	}
	
	consumer, err := consent.NewConsumer(phoneNumber, email, firstName, lastName)
	if err != nil {
		return nil, err
	}
	
	return consumer, m.Save(ctx, consumer)
}

func (m *mockConsumerRepository) GetByEmail(ctx context.Context, email string) (*consent.Consumer, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	for _, consumer := range m.consumers {
		if consumer.Email != nil && *consumer.Email == email {
			return consumer, nil
		}
	}
	return nil, nil
}

func (m *mockConsumerRepository) GetByID(ctx context.Context, id uuid.UUID) (*consent.Consumer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*consent.Consumer), args.Error(1)
}

type mockQueryRepository struct {
	mock.Mock
	consents map[uuid.UUID]*consent.ConsentAggregate
	err      error
}

func (m *mockQueryRepository) SaveConsent(ctx context.Context, c *consent.ConsentAggregate) error {
	if m.err != nil {
		return m.err
	}
	if m.consents == nil {
		m.consents = make(map[uuid.UUID]*consent.ConsentAggregate)
	}
	m.consents[c.ID] = c
	return nil
}

func (m *mockQueryRepository) Count(ctx context.Context, filter consent.ConsentFilter) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	count := int64(0)
	for _, c := range m.consents {
		if filter.ConsumerID != nil && c.ConsumerID != *filter.ConsumerID {
			continue
		}
		if filter.BusinessID != nil && c.BusinessID != *filter.BusinessID {
			continue
		}
		if filter.Status != nil && c.GetCurrentStatus() != *filter.Status {
			continue
		}
		count++
	}
	return count, nil
}

func (m *mockQueryRepository) Find(ctx context.Context, filter consent.ConsentFilter) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*consent.ConsentAggregate
	count := 0
	for _, c := range m.consents {
		if filter.Limit > 0 && count >= filter.Limit {
			break
		}
		if filter.ConsumerID != nil && c.ConsumerID != *filter.ConsumerID {
			continue
		}
		if filter.BusinessID != nil && c.BusinessID != *filter.BusinessID {
			continue
		}
		if filter.Status != nil && c.GetCurrentStatus() != *filter.Status {
			continue
		}
		result = append(result, c)
		count++
	}
	return result, nil
}

func (m *mockQueryRepository) FindActiveByConsumer(ctx context.Context, consumerID uuid.UUID) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*consent.ConsentAggregate
	for _, c := range m.consents {
		if c.ConsumerID == consumerID && c.IsActive() {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockQueryRepository) FindByFilters(ctx context.Context, filters consent.QueryFilters) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Simple implementation - in practice would apply all filters
	var result []*consent.ConsentAggregate
	for _, c := range m.consents {
		if filters.ConsumerID != nil && c.ConsumerID != *filters.ConsumerID {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

func (m *mockQueryRepository) GetConsentHistory(ctx context.Context, consentID uuid.UUID) ([]consent.ConsentVersion, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return empty history for mock
	return []consent.ConsentVersion{}, nil
}

func (m *mockQueryRepository) GetProofs(ctx context.Context, consentID uuid.UUID) ([]consent.ConsentProof, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return empty proofs for mock
	return []consent.ConsentProof{}, nil
}

func (m *mockQueryRepository) GetMetrics(ctx context.Context, query consent.MetricsQuery) (*consent.ConsentMetrics, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return basic metrics for mock
	return &consent.ConsentMetrics{
		TotalGrants:  0,
		TotalRevokes: 0,
		ActiveCount:  0,
	}, nil
}

func (m *mockQueryRepository) FindExpiring(ctx context.Context, days int) ([]*consent.ConsentAggregate, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return empty list for mock
	return []*consent.ConsentAggregate{}, nil
}

type mockComplianceChecker struct {
	mock.Mock
	err error
}

func (m *mockComplianceChecker) CheckConsentRequirements(ctx context.Context, phoneNumber string, consentType consent.Type) (*compliance.ComplianceRule, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return a basic compliance rule for testing
	return &compliance.ComplianceRule{
		Type:     compliance.RuleTypeTCPA,
		Name:     "TCPA Basic",
		Status:   compliance.RuleStatusActive,
		Priority: 100,
	}, nil
}

func (m *mockComplianceChecker) ValidateConsentGrant(ctx context.Context, req GrantConsentRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

type mockEventPublisher struct {
	mock.Mock
	events []interface{}
	err    error
}

func (m *mockEventPublisher) PublishConsentGranted(ctx context.Context, event consent.ConsentCreatedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockEventPublisher) PublishConsentRevoked(ctx context.Context, event consent.ConsentRevokedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockEventPublisher) PublishConsentUpdated(ctx context.Context, event consent.ConsentUpdatedEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

