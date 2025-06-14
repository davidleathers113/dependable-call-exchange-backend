package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentCache provides caching for consent data
type ConsentCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewConsentCache creates a new consent cache
func NewConsentCache(client *redis.Client, ttl time.Duration) *ConsentCache {
	return &ConsentCache{
		client: client,
		ttl:    ttl,
	}
}

// GetConsent retrieves a consent from cache
func (c *ConsentCache) GetConsent(ctx context.Context, id uuid.UUID) (*consent.ConsentAggregate, error) {
	key := c.consentKey(id)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, errors.NewInternalError("failed to get from cache").WithCause(err)
	}

	var cached cachedConsent
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal cached consent").WithCause(err)
	}

	return cached.toAggregate()
}

// SetConsent stores a consent in cache
func (c *ConsentCache) SetConsent(ctx context.Context, consentAgg *consent.ConsentAggregate) error {
	key := c.consentKey(consentAgg.ID)
	
	cached := fromAggregate(consentAgg)
	data, err := json.Marshal(cached)
	if err != nil {
		return errors.NewInternalError("failed to marshal consent").WithCause(err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return errors.NewInternalError("failed to set cache").WithCause(err)
	}

	// Also cache by consumer-business-channel for quick lookups
	if len(consentAgg.Versions) > 0 {
		current := consentAgg.Versions[consentAgg.CurrentVersion-1]
		if current.Status == consent.StatusActive {
			for _, channel := range current.Channels {
				lookupKey := c.activeLookupKey(consentAgg.ConsumerID, consentAgg.BusinessID, channel)
				if err := c.client.Set(ctx, lookupKey, consentAgg.ID.String(), c.ttl).Err(); err != nil {
					// Log but don't fail
					continue
				}
			}
		}
	}

	return nil
}

// InvalidateConsent removes a consent from cache
func (c *ConsentCache) InvalidateConsent(ctx context.Context, id uuid.UUID) error {
	key := c.consentKey(id)
	
	// Get the consent first to clear lookup keys
	consentAgg, err := c.GetConsent(ctx, id)
	if err == nil && consentAgg != nil {
		// Clear active lookup keys
		if len(consentAgg.Versions) > 0 {
			current := consentAgg.Versions[consentAgg.CurrentVersion-1]
			for _, channel := range current.Channels {
				lookupKey := c.activeLookupKey(consentAgg.ConsumerID, consentAgg.BusinessID, channel)
				c.client.Del(ctx, lookupKey)
			}
		}
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return errors.NewInternalError("failed to delete from cache").WithCause(err)
	}

	return nil
}

// GetActiveConsent retrieves active consent for a specific channel from cache
func (c *ConsentCache) GetActiveConsent(ctx context.Context, consumerID, businessID uuid.UUID, channel consent.Channel) (uuid.UUID, bool) {
	key := c.activeLookupKey(consumerID, businessID, channel)
	
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}

	return id, true
}

// GetConsumer retrieves a consumer from cache
func (c *ConsentCache) GetConsumer(ctx context.Context, id uuid.UUID) (*consent.Consumer, error) {
	key := c.consumerKey(id)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, errors.NewInternalError("failed to get consumer from cache").WithCause(err)
	}

	var consumer consent.Consumer
	if err := json.Unmarshal(data, &consumer); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal cached consumer").WithCause(err)
	}

	return &consumer, nil
}

// SetConsumer stores a consumer in cache
func (c *ConsentCache) SetConsumer(ctx context.Context, consumer *consent.Consumer) error {
	key := c.consumerKey(consumer.ID)
	
	data, err := json.Marshal(consumer)
	if err != nil {
		return errors.NewInternalError("failed to marshal consumer").WithCause(err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return errors.NewInternalError("failed to set consumer cache").WithCause(err)
	}

	// Also cache by phone/email for lookups
	if consumer.PhoneNumber != nil {
		phoneKey := c.consumerPhoneKey(consumer.PhoneNumber.String())
		c.client.Set(ctx, phoneKey, consumer.ID.String(), c.ttl)
	}
	
	if consumer.Email != nil && *consumer.Email != "" {
		emailKey := c.consumerEmailKey(*consumer.Email)
		c.client.Set(ctx, emailKey, consumer.ID.String(), c.ttl)
	}

	return nil
}

// InvalidateConsumer removes a consumer from cache
func (c *ConsentCache) InvalidateConsumer(ctx context.Context, id uuid.UUID) error {
	// Get consumer first to clear lookup keys
	consumer, _ := c.GetConsumer(ctx, id)
	
	key := c.consumerKey(id)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return errors.NewInternalError("failed to delete consumer from cache").WithCause(err)
	}

	// Clear lookup keys
	if consumer != nil {
		if consumer.PhoneNumber != nil {
			phoneKey := c.consumerPhoneKey(consumer.PhoneNumber.String())
			c.client.Del(ctx, phoneKey)
		}
		if consumer.Email != nil && *consumer.Email != "" {
			emailKey := c.consumerEmailKey(*consumer.Email)
			c.client.Del(ctx, emailKey)
		}
	}

	return nil
}

// GetConsumerByPhone looks up consumer ID by phone number
func (c *ConsentCache) GetConsumerByPhone(ctx context.Context, phoneNumber string) (uuid.UUID, bool) {
	key := c.consumerPhoneKey(phoneNumber)
	
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}

	return id, true
}

// GetConsumerByEmail looks up consumer ID by email
func (c *ConsentCache) GetConsumerByEmail(ctx context.Context, email string) (uuid.UUID, bool) {
	key := c.consumerEmailKey(email)
	
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, false
	}

	return id, true
}

// Key generation helpers
func (c *ConsentCache) consentKey(id uuid.UUID) string {
	return fmt.Sprintf("consent:%s", id)
}

func (c *ConsentCache) activeLookupKey(consumerID, businessID uuid.UUID, channel consent.Channel) string {
	return fmt.Sprintf("consent:active:%s:%s:%s", consumerID, businessID, channel)
}

func (c *ConsentCache) consumerKey(id uuid.UUID) string {
	return fmt.Sprintf("consumer:%s", id)
}

func (c *ConsentCache) consumerPhoneKey(phone string) string {
	return fmt.Sprintf("consumer:phone:%s", phone)
}

func (c *ConsentCache) consumerEmailKey(email string) string {
	return fmt.Sprintf("consumer:email:%s", email)
}

// cachedConsent is a simplified version for caching
type cachedConsent struct {
	ID             uuid.UUID                `json:"id"`
	ConsumerID     uuid.UUID                `json:"consumer_id"`
	BusinessID     uuid.UUID                `json:"business_id"`
	CurrentVersion int                      `json:"current_version"`
	Versions       []cachedConsentVersion   `json:"versions"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

type cachedConsentVersion struct {
	VersionNumber int                    `json:"version_number"`
	Status        consent.ConsentStatus  `json:"status"`
	Channels      []consent.Channel      `json:"channels"`
	Purpose       consent.Purpose        `json:"purpose"`
	Source        consent.ConsentSource  `json:"source"`
	SourceDetails map[string]string      `json:"source_details"`
	ConsentedAt   *time.Time            `json:"consented_at,omitempty"`
	ExpiresAt     *time.Time            `json:"expires_at,omitempty"`
	RevokedAt     *time.Time            `json:"revoked_at,omitempty"`
	CreatedBy     uuid.UUID             `json:"created_by"`
	CreatedAt     time.Time             `json:"created_at"`
	Proofs        []consent.ConsentProof `json:"proofs"`
}

func fromAggregate(agg *consent.ConsentAggregate) cachedConsent {
	cached := cachedConsent{
		ID:             agg.ID,
		ConsumerID:     agg.ConsumerID,
		BusinessID:     agg.BusinessID,
		CurrentVersion: agg.CurrentVersion,
		CreatedAt:      agg.CreatedAt,
		UpdatedAt:      agg.UpdatedAt,
		Versions:       make([]cachedConsentVersion, len(agg.Versions)),
	}

	for i, v := range agg.Versions {
		cached.Versions[i] = cachedConsentVersion{
			VersionNumber: v.Version,
			Status:        v.Status,
			Channels:      v.Channels,
			Purpose:       v.Purpose,
			Source:        v.Source,
			SourceDetails: v.SourceDetails,
			ConsentedAt:   v.ConsentedAt,
			ExpiresAt:     v.ExpiresAt,
			RevokedAt:     v.RevokedAt,
			CreatedBy:     v.CreatedBy,
			CreatedAt:     v.CreatedAt,
			Proofs:        v.Proofs,
		}
	}

	return cached
}

func (c cachedConsent) toAggregate() (*consent.ConsentAggregate, error) {
	agg := &consent.ConsentAggregate{
		ID:             c.ID,
		ConsumerID:     c.ConsumerID,
		BusinessID:     c.BusinessID,
		CurrentVersion: c.CurrentVersion,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
		Versions:       make([]consent.ConsentVersion, len(c.Versions)),
	}

	for i, v := range c.Versions {
		agg.Versions[i] = consent.ConsentVersion{
			Version:       v.VersionNumber,
			Status:        v.Status,
			Channels:      v.Channels,
			Purpose:       v.Purpose,
			Source:        v.Source,
			SourceDetails: v.SourceDetails,
			ConsentedAt:   v.ConsentedAt,
			ExpiresAt:     v.ExpiresAt,
			RevokedAt:     v.RevokedAt,
			CreatedBy:     v.CreatedBy,
			CreatedAt:     v.CreatedAt,
			Proofs:        v.Proofs,
		}
	}

	return agg, nil
}