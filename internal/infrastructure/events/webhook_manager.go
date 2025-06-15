package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"go.uber.org/zap"
)

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(logger *zap.Logger, timeout time.Duration) *WebhookManager {
	return &WebhookManager{
		endpoints: make(map[string]WebhookEndpoint),
		client:    NewHTTPWebhookClient(timeout),
		logger:    logger,
	}
}

// AddEndpoint adds a new webhook endpoint
func (wm *WebhookManager) AddEndpoint(endpoint WebhookEndpoint) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	wm.endpoints[endpoint.URL] = endpoint
	
	wm.logger.Info("Added webhook endpoint",
		zap.String("url", endpoint.URL),
		zap.Strings("event_filters", eventTypesToStrings(endpoint.EventFilters)),
		zap.Bool("enabled", endpoint.Enabled),
	)
	
	return nil
}

// RemoveEndpoint removes a webhook endpoint
func (wm *WebhookManager) RemoveEndpoint(url string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	if _, exists := wm.endpoints[url]; !exists {
		return errors.NewNotFoundError("webhook endpoint not found")
	}
	
	delete(wm.endpoints, url)
	
	wm.logger.Info("Removed webhook endpoint", zap.String("url", url))
	
	return nil
}

// NotifyWebhooks sends the event to all matching webhook endpoints
func (wm *WebhookManager) NotifyWebhooks(ctx context.Context, event DNCDomainEvent) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	var wg sync.WaitGroup
	var errors []error
	var errorsMu sync.Mutex
	
	for _, endpoint := range wm.endpoints {
		if !endpoint.Enabled {
			continue
		}
		
		// Check if event matches filter
		if !wm.eventMatchesFilter(event, endpoint.EventFilters) {
			continue
		}
		
		wg.Add(1)
		go func(ep WebhookEndpoint) {
			defer wg.Done()
			
			if err := wm.client.Send(ctx, ep, event); err != nil {
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("webhook %s failed: %w", ep.URL, err))
				errorsMu.Unlock()
				
				wm.logger.Error("Webhook delivery failed",
					zap.String("url", ep.URL),
					zap.String("event_id", event.GetEventID().String()),
					zap.Error(err),
				)
			} else {
				wm.logger.Debug("Webhook delivered successfully",
					zap.String("url", ep.URL),
					zap.String("event_id", event.GetEventID().String()),
				)
			}
		}(endpoint)
	}
	
	wg.Wait()
	
	if len(errors) > 0 {
		// Return first error, but log all
		return errors[0]
	}
	
	return nil
}

// GetEndpoints returns all configured endpoints
func (wm *WebhookManager) GetEndpoints() []WebhookEndpoint {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	endpoints := make([]WebhookEndpoint, 0, len(wm.endpoints))
	for _, endpoint := range wm.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	
	return endpoints
}

// Close shuts down the webhook manager
func (wm *WebhookManager) Close() error {
	wm.logger.Info("Shutting down webhook manager")
	return nil
}

// Private methods

func (wm *WebhookManager) eventMatchesFilter(event DNCDomainEvent, filters []audit.EventType) bool {
	if len(filters) == 0 {
		return true // No filter means match all
	}
	
	eventType := event.GetEventType()
	for _, filter := range filters {
		if filter == eventType {
			return true
		}
	}
	
	return false
}

// HTTPWebhookClient implements WebhookClient using HTTP
type HTTPWebhookClient struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPWebhookClient creates a new HTTP webhook client
func NewHTTPWebhookClient(timeout time.Duration) *HTTPWebhookClient {
	return &HTTPWebhookClient{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Send sends the event to a webhook endpoint via HTTP POST
func (c *HTTPWebhookClient) Send(ctx context.Context, endpoint WebhookEndpoint, event DNCDomainEvent) error {
	// Create webhook payload
	payload := WebhookPayload{
		EventID:       event.GetEventID(),
		EventType:     event.GetEventType(),
		EventVersion:  event.GetEventVersion(),
		Timestamp:     event.GetTimestamp(),
		AggregateID:   event.GetAggregateID(),
		AggregateType: event.GetAggregateType(),
		Data:          event,
	}
	
	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.NewInternalError("failed to marshal webhook payload").WithCause(err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return errors.NewInternalError("failed to create webhook request").WithCause(err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DNC-Event-Publisher/1.0")
	req.Header.Set("X-Event-Type", string(event.GetEventType()))
	req.Header.Set("X-Event-ID", event.GetEventID().String())
	req.Header.Set("X-Event-Version", event.GetEventVersion())
	
	// Add signature for authentication
	if endpoint.Secret != "" {
		signature := c.generateSignature(payloadBytes, endpoint.Secret)
		req.Header.Set("X-Signature-SHA256", signature)
	}
	
	// Apply timeout from endpoint if specified
	if endpoint.TimeoutDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, endpoint.TimeoutDuration)
		defer cancel()
		req = req.WithContext(ctx)
	}
	
	// Send request with retry logic
	return c.sendWithRetry(req, endpoint.RetryPolicy)
}

func (c *HTTPWebhookClient) sendWithRetry(req *http.Request, retryPolicy RetryPolicy) error {
	var lastErr error
	
	for attempt := 1; attempt <= retryPolicy.MaxAttempts; attempt++ {
		// Clone request for retry
		reqClone := req.Clone(req.Context())
		
		resp, err := c.client.Do(reqClone)
		if err != nil {
			lastErr = err
			if attempt < retryPolicy.MaxAttempts {
				delay := c.calculateDelay(attempt, retryPolicy)
				time.Sleep(delay)
				continue
			}
			break
		}
		
		resp.Body.Close()
		
		// Check status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Success
		}
		
		// Check if error is retryable
		if !c.isRetryable(resp.StatusCode, retryPolicy.RetryableErrors) {
			return errors.NewInternalError(fmt.Sprintf("webhook returned non-retryable status: %d", resp.StatusCode))
		}
		
		lastErr = errors.NewInternalError(fmt.Sprintf("webhook returned status: %d", resp.StatusCode))
		
		if attempt < retryPolicy.MaxAttempts {
			delay := c.calculateDelay(attempt, retryPolicy)
			time.Sleep(delay)
		}
	}
	
	return errors.NewInternalError("webhook delivery failed after retries").WithCause(lastErr)
}

func (c *HTTPWebhookClient) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func (c *HTTPWebhookClient) calculateDelay(attempt int, policy RetryPolicy) time.Duration {
	delay := policy.InitialDelay
	
	// Apply exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * policy.BackoffFactor)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
			break
		}
	}
	
	return delay
}

func (c *HTTPWebhookClient) isRetryable(statusCode int, retryableErrors []string) bool {
	// Retry on server errors and specific client errors
	if statusCode >= 500 {
		return true
	}
	
	if statusCode == 408 || statusCode == 429 {
		return true
	}
	
	// Additional retryable conditions based on configuration
	for _, errType := range retryableErrors {
		switch errType {
		case "timeout":
			if statusCode == 408 {
				return true
			}
		case "rate_limit":
			if statusCode == 429 {
				return true
			}
		case "server_error":
			if statusCode >= 500 {
				return true
			}
		}
	}
	
	return false
}

// WebhookPayload represents the payload sent to webhook endpoints
type WebhookPayload struct {
	EventID       interface{}       `json:"event_id"`
	EventType     audit.EventType   `json:"event_type"`
	EventVersion  string            `json:"event_version"`
	Timestamp     time.Time         `json:"timestamp"`
	AggregateID   string            `json:"aggregate_id"`
	AggregateType string            `json:"aggregate_type"`
	Data          interface{}       `json:"data"`
}

// Helper functions

func eventTypesToStrings(eventTypes []audit.EventType) []string {
	result := make([]string, len(eventTypes))
	for i, et := range eventTypes {
		result[i] = string(et)
	}
	return result
}