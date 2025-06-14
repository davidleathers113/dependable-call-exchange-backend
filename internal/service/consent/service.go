package consent

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Ensure service implements the interface
var _ Service = (*service)(nil)

type service struct {
	logger               *zap.Logger
	consentRepo          ConsentRepository
	consumerRepo         ConsumerRepository
	queryRepo            ConsentQueryRepository
	complianceChecker    ComplianceChecker
	eventPublisher       EventPublisher
}

// ComplianceChecker interface for integration with compliance service
type ComplianceChecker interface {
	CheckConsentRequirements(ctx context.Context, phoneNumber string, consentType consent.Type) (*compliance.ComplianceRule, error)
	ValidateConsentGrant(ctx context.Context, req GrantConsentRequest) error
}

// EventPublisher interface for publishing consent events
type EventPublisher interface {
	PublishConsentGranted(ctx context.Context, event consent.ConsentCreatedEvent) error
	PublishConsentRevoked(ctx context.Context, event consent.ConsentRevokedEvent) error
	PublishConsentUpdated(ctx context.Context, event consent.ConsentUpdatedEvent) error
}

// NewService creates a new consent service
func NewService(
	logger *zap.Logger,
	consentRepo ConsentRepository,
	consumerRepo ConsumerRepository,
	queryRepo ConsentQueryRepository,
	complianceChecker ComplianceChecker,
	eventPublisher EventPublisher,
) Service {
	return &service{
		logger:            logger,
		consentRepo:       consentRepo,
		consumerRepo:      consumerRepo,
		queryRepo:         queryRepo,
		complianceChecker: complianceChecker,
		eventPublisher:    eventPublisher,
	}
}

// GrantConsent grants consent for a consumer
func (s *service) GrantConsent(ctx context.Context, req GrantConsentRequest) (*ConsentResponse, error) {
	logger := s.logger.With(
		zap.String("consent_type", req.ConsentType.String()),
		zap.String("channel", req.Channel.String()),
	)

	// Validate the consent grant request against compliance rules
	if err := s.complianceChecker.ValidateConsentGrant(ctx, req); err != nil {
		return nil, errors.NewValidationError("INVALID_CONSENT_GRANT", "consent grant failed compliance validation").WithCause(err)
	}

	// Get or create consumer
	consumer, err := s.getOrCreateConsumer(ctx, req)
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("consumer_id", consumer.ID.String()))

	// Create consent proof with preferences
	formData := make(map[string]string)
	if req.Preferences != nil {
		for k, v := range req.Preferences {
			formData[k] = v
		}
	}
	
	proof := consent.ConsentProof{
		ID:              uuid.New(),
		VersionID:       uuid.New(), // Will be set properly when version is created
		Type:            consent.ProofTypeDigital,
		StorageLocation: "",
		Hash:            "",
		Metadata: consent.ProofMetadata{
			IPAddress: req.IPAddress,
			UserAgent: req.UserAgent,
			FormData:  formData,
		},
		CreatedAt: time.Now(),
	}

	// Check if consent already exists
	existing, err := s.consentRepo.GetByConsumerAndType(ctx, consumer.ID, req.ConsentType)
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.NewInternalError("failed to check existing consent").WithCause(err)
	}

	var consentAggregate *consent.ConsentAggregate
	if existing != nil {
		// Update existing consent
		if err := existing.Grant(proof, req.Preferences, req.ExpiresAt); err != nil {
			return nil, err
		}
		consentAggregate = existing
	} else {
		// Create new consent aggregate
		channels := []consent.Channel{req.Channel}
		purpose := consent.PurposeMarketing // Default purpose, should come from request
		if req.Preferences != nil {
			if p, ok := req.Preferences["purpose"]; ok {
				// Parse purpose from preferences if provided
				switch p {
				case "marketing":
					purpose = consent.PurposeMarketing
				case "service_calls":
					purpose = consent.PurposeServiceCalls
				case "debt_collection":
					purpose = consent.PurposeDebtCollection
				case "emergency":
					purpose = consent.PurposeEmergency
				}
			}
		}
		
		consentAggregate, err = consent.NewConsentAggregate(
			consumer.ID,
			uuid.New(), // Business ID - should come from request context
			req.ConsentType,
			channels,
			purpose,
			consent.SourceAPI,
		)
		if err != nil {
			return nil, err
		}
		
		// Activate the consent with the proof
		if err := consentAggregate.Grant(proof, req.Preferences, req.ExpiresAt); err != nil {
			return nil, err
		}
	}

	// Save consent
	if err := s.consentRepo.Save(ctx, consentAggregate); err != nil {
		return nil, errors.NewInternalError("failed to save consent").WithCause(err)
	}

	// Publish events
	for _, event := range consentAggregate.Events() {
		switch e := event.(type) {
		case consent.ConsentCreatedEvent:
			if err := s.eventPublisher.PublishConsentGranted(ctx, e); err != nil {
				logger.Error("failed to publish consent created event", zap.Error(err))
			}
		case consent.ConsentActivatedEvent:
			// Convert to ConsentCreatedEvent for backwards compatibility
			createdEvent := consent.ConsentCreatedEvent{
				ConsentID:  e.ConsentID,
				ConsumerID: e.ConsumerID,
				BusinessID: e.BusinessID,
				Channels:   e.Channels,
				Purpose:    consent.PurposeMarketing, // Default, should be derived from aggregate
				Source:     consent.SourceAPI,
				CreatedAt:  e.ActivatedAt,
			}
			if err := s.eventPublisher.PublishConsentGranted(ctx, createdEvent); err != nil {
				logger.Error("failed to publish consent activated event", zap.Error(err))
			}
		}
	}

	logger.Info("consent granted successfully")
	return s.toConsentResponse(consentAggregate), nil
}

// RevokeConsent revokes consent for a consumer
func (s *service) RevokeConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) error {
	logger := s.logger.With(
		zap.String("consumer_id", consumerID.String()),
		zap.String("consent_type", consentType.String()),
	)

	// Get existing consent
	consentAggregate, err := s.consentRepo.GetByConsumerAndType(ctx, consumerID, consentType)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewNotFoundError("consent not found")
		}
		return errors.NewInternalError("failed to get consent").WithCause(err)
	}

	// Revoke consent
	reason := "Consumer requested revocation"
	if err := consentAggregate.Revoke(reason); err != nil {
		return err
	}

	// Save consent
	if err := s.consentRepo.Save(ctx, consentAggregate); err != nil {
		return errors.NewInternalError("failed to save consent").WithCause(err)
	}

	// Publish event
	for _, event := range consentAggregate.Events() {
		if revokedEvent, ok := event.(consent.ConsentRevokedEvent); ok {
			if err := s.eventPublisher.PublishConsentRevoked(ctx, revokedEvent); err != nil {
				logger.Error("failed to publish consent revoked event", zap.Error(err))
			}
		}
	}

	logger.Info("consent revoked successfully")
	return nil
}

// UpdateConsent updates consent preferences
func (s *service) UpdateConsent(ctx context.Context, req UpdateConsentRequest) (*ConsentResponse, error) {
	logger := s.logger.With(
		zap.String("consumer_id", req.ConsumerID.String()),
		zap.String("consent_type", req.ConsentType.String()),
	)

	// Get existing consent
	consentAggregate, err := s.consentRepo.GetByConsumerAndType(ctx, req.ConsumerID, req.ConsentType)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("consent not found")
		}
		return nil, errors.NewInternalError("failed to get consent").WithCause(err)
	}

	// Update preferences
	if err := consentAggregate.UpdatePreferences(req.Preferences); err != nil {
		return nil, err
	}

	// Update expiration if provided
	if req.ExpiresAt != nil {
		if err := consentAggregate.UpdateExpiration(*req.ExpiresAt); err != nil {
			return nil, err
		}
	}

	// Save consent
	if err := s.consentRepo.Save(ctx, consentAggregate); err != nil {
		return nil, errors.NewInternalError("failed to save consent").WithCause(err)
	}

	// Publish event
	for _, event := range consentAggregate.Events() {
		if updatedEvent, ok := event.(consent.ConsentUpdatedEvent); ok {
			if err := s.eventPublisher.PublishConsentUpdated(ctx, updatedEvent); err != nil {
				logger.Error("failed to publish consent updated event", zap.Error(err))
			}
		}
	}

	logger.Info("consent updated successfully")
	return s.toConsentResponse(consentAggregate), nil
}

// GetConsent retrieves consent for a consumer
func (s *service) GetConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) (*ConsentResponse, error) {
	consentAggregate, err := s.consentRepo.GetByConsumerAndType(ctx, consumerID, consentType)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("consent not found")
		}
		return nil, errors.NewInternalError("failed to get consent").WithCause(err)
	}

	return s.toConsentResponse(consentAggregate), nil
}

// GetActiveConsents retrieves all active consents for a consumer
func (s *service) GetActiveConsents(ctx context.Context, consumerID uuid.UUID) ([]*ConsentResponse, error) {
	consents, err := s.queryRepo.FindActiveByConsumer(ctx, consumerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get active consents").WithCause(err)
	}

	responses := make([]*ConsentResponse, len(consents))
	for i, c := range consents {
		responses[i] = s.toConsentResponse(c)
	}

	return responses, nil
}

// CheckConsent checks if a phone number has valid consent
func (s *service) CheckConsent(ctx context.Context, phoneNumber string, consentType consent.Type) (*ConsentStatus, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
	}

	// Find consumer by phone
	consumer, err := s.consumerRepo.GetByPhoneNumber(ctx, phone.String())
	if err != nil {
		if errors.IsNotFound(err) {
			return &ConsentStatus{
				HasConsent: false,
				Status:     consent.StatusRevoked,
			}, nil
		}
		return nil, errors.NewInternalError("failed to get consumer").WithCause(err)
	}

	// Get consent
	consentAggregate, err := s.consentRepo.GetByConsumerAndType(ctx, consumer.ID, consentType)
	if err != nil {
		if errors.IsNotFound(err) {
			return &ConsentStatus{
				HasConsent: false,
				Status:     consent.StatusRevoked,
			}, nil
		}
		return nil, errors.NewInternalError("failed to get consent").WithCause(err)
	}

	// Check if consent is active
	isActive := consentAggregate.IsActive()
	currentStatus := consentAggregate.GetCurrentStatus()

	return &ConsentStatus{
		HasConsent:  isActive,
		ConsentID:   &consentAggregate.ID,
		Status:      currentStatus,
		GrantedAt:   nil, // Will need to get from current version
		ExpiresAt:   nil, // Will need to get from current version
		Preferences: nil, // Will need to get from current version
	}, nil
}

// CreateConsumer creates a new consumer
func (s *service) CreateConsumer(ctx context.Context, req CreateConsumerRequest) (*ConsumerResponse, error) {
	// Validate phone number
	_, err := values.NewPhoneNumber(req.PhoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
	}

	// Validate email if provided
	var emailPtr *string
	if req.Email != "" {
		emailPtr = &req.Email
	}

	// Create consumer
	consumer, err := consent.NewConsumer(req.PhoneNumber, emailPtr, req.FirstName, req.LastName)
	if err != nil {
		return nil, err
	}

	// Save consumer
	if err := s.consumerRepo.Save(ctx, consumer); err != nil {
		if errors.IsConflict(err) {
			return nil, errors.NewConflictError("consumer already exists")
		}
		return nil, errors.NewInternalError("failed to save consumer").WithCause(err)
	}

	return s.toConsumerResponse(consumer), nil
}

// GetConsumerByPhone retrieves a consumer by phone number
func (s *service) GetConsumerByPhone(ctx context.Context, phoneNumber string) (*ConsumerResponse, error) {
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
	}

	consumer, err := s.consumerRepo.GetByPhoneNumber(ctx, phone.String())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("consumer not found")
		}
		return nil, errors.NewInternalError("failed to get consumer").WithCause(err)
	}

	return s.toConsumerResponse(consumer), nil
}

// GetConsumerByEmail retrieves a consumer by email
func (s *service) GetConsumerByEmail(ctx context.Context, email string) (*ConsumerResponse, error) {
	emailValue, err := values.NewEmail(email)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_EMAIL", "invalid email format").WithCause(err)
	}

	consumer, err := s.consumerRepo.GetByEmail(ctx, emailValue.String())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("consumer not found")
		}
		return nil, errors.NewInternalError("failed to get consumer").WithCause(err)
	}

	return s.toConsumerResponse(consumer), nil
}

// ImportConsents imports consents from a file
func (s *service) ImportConsents(ctx context.Context, req ImportConsentsRequest) (*ImportResult, error) {
	startTime := time.Now()
	result := &ImportResult{
		Errors: []ImportError{},
	}

	switch strings.ToLower(req.Format) {
	case "csv":
		return s.importCSV(ctx, req, result, startTime)
	case "json":
		return s.importJSON(ctx, req, result, startTime)
	default:
		return nil, errors.NewValidationError("INVALID_FORMAT", "unsupported import format")
	}
}

// ExportConsents exports consents to a file
func (s *service) ExportConsents(ctx context.Context, req ExportConsentsRequest) (*ExportResult, error) {
	// Query consents based on filters
	var statusPtr *consent.ConsentStatus
	if len(req.Filters.Status) > 0 {
		statusPtr = &req.Filters.Status[0] // Use first status if provided
	}
	
	consents, err := s.queryRepo.FindByFilters(ctx, consent.QueryFilters{
		ConsentType:  nil, // Will need to be set if req.Filters.ConsentTypes provided
		Status:       statusPtr,
		CreatedAfter: req.Filters.StartDate,
		ExpiringDays: nil, // Use EndDate if needed
	})
	if err != nil {
		return nil, errors.NewInternalError("failed to query consents").WithCause(err)
	}

	switch strings.ToLower(req.Format) {
	case "csv":
		return s.exportCSV(ctx, consents)
	case "json":
		return s.exportJSON(ctx, consents)
	default:
		return nil, errors.NewValidationError("INVALID_FORMAT", "unsupported export format")
	}
}

// GetConsentMetrics retrieves consent metrics
func (s *service) GetConsentMetrics(ctx context.Context, req MetricsRequest) (*ConsentMetrics, error) {
	// Query metrics from the repository
	dateRange := consent.DateRange{
		Start: req.StartDate,
		End:   req.EndDate,
	}
	
	var groupBy []string
	if req.GroupBy != "" {
		groupBy = []string{req.GroupBy}
	}
	
	metrics, err := s.queryRepo.GetMetrics(ctx, consent.MetricsQuery{
		DateRange:    dateRange,
		ConsentTypes: req.ConsentTypes,
		Granularity:  "day", // Default granularity
		GroupBy:      groupBy,
	})
	if err != nil {
		return nil, errors.NewInternalError("failed to get consent metrics").WithCause(err)
	}

	// Transform to response format
	return &ConsentMetrics{
		Period:       req.GroupBy,
		TotalGrants:  make(map[consent.Type]int),  // This should be converted from metrics
		TotalRevokes: make(map[consent.Type]int),  // This should be converted from metrics
		ActiveCount:  make(map[consent.Type]int),  // This should be converted from metrics
		Trends:       s.toConsentTrends(metrics.Trends),
	}, nil
}

// Helper methods

func (s *service) getOrCreateConsumer(ctx context.Context, req GrantConsentRequest) (*consent.Consumer, error) {
	// If consumer ID is provided, get existing consumer
	if req.ConsumerID != uuid.Nil {
		consumer, err := s.consumerRepo.GetByID(ctx, req.ConsumerID)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, errors.NewNotFoundError("consumer not found")
			}
			return nil, errors.NewInternalError("failed to get consumer").WithCause(err)
		}
		return consumer, nil
	}

	// Otherwise, find or create by phone/email
	if req.PhoneNumber == "" && req.Email == "" {
		return nil, errors.NewValidationError("MISSING_IDENTIFIER", "either consumer_id, phone_number, or email is required")
	}

	// Try to find by phone first
	if req.PhoneNumber != "" {
		phone, err := values.NewPhoneNumber(req.PhoneNumber)
		if err != nil {
			return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
		}

		consumer, err := s.consumerRepo.GetByPhoneNumber(ctx, phone.String())
		if err == nil {
			return consumer, nil
		}
		if !errors.IsNotFound(err) {
			return nil, errors.NewInternalError("failed to get consumer by phone").WithCause(err)
		}
	}

	// Try to find by email
	if req.Email != "" {
		email, err := values.NewEmail(req.Email)
		if err != nil {
			return nil, errors.NewValidationError("INVALID_EMAIL", "invalid email format").WithCause(err)
		}

		consumer, err := s.consumerRepo.GetByEmail(ctx, email.String())
		if err == nil {
			return consumer, nil
		}
		if !errors.IsNotFound(err) {
			return nil, errors.NewInternalError("failed to get consumer by email").WithCause(err)
		}
	}

	// Create new consumer
	var emailPtr *string
	if req.Email != "" {
		emailPtr = &req.Email
	}
	consumer, err := consent.NewConsumer(req.PhoneNumber, emailPtr, "", "")
	if err != nil {
		return nil, err
	}

	if err := s.consumerRepo.Save(ctx, consumer); err != nil {
		return nil, errors.NewInternalError("failed to create consumer").WithCause(err)
	}

	return consumer, nil
}

func (s *service) toConsentResponse(c *consent.ConsentAggregate) *ConsentResponse {
	return MapConsentToResponse(c)
}

func (s *service) toConsumerResponse(c *consent.Consumer) *ConsumerResponse {
	return MapConsumerToResponse(c)
}

func (s *service) toConsentTrends(trends []consent.MetricTrend) []ConsentTrend {
	result := make([]ConsentTrend, len(trends))
	for i, trend := range trends {
		result[i] = ConsentTrend{
			Date:    trend.Date,
			Grants:  trend.Grants,
			Revokes: trend.Revokes,
		}
	}
	return result
}

// Import/export helpers

func (s *service) importCSV(ctx context.Context, req ImportConsentsRequest, result *ImportResult, startTime time.Time) (*ImportResult, error) {
	reader := csv.NewReader(strings.NewReader(string(req.Data)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CSV", "failed to parse CSV").WithCause(err)
	}

	if len(records) < 2 {
		return nil, errors.NewValidationError("EMPTY_FILE", "CSV file must have header and at least one data row")
	}

	// Process header
	header := records[0]
	phoneIdx := -1
	emailIdx := -1
	typeIdx := -1
	channelIdx := -1

	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "phone", "phone_number":
			phoneIdx = i
		case "email":
			emailIdx = i
		case "type", "consent_type":
			typeIdx = i
		case "channel":
			channelIdx = i
		}
	}

	if phoneIdx == -1 || typeIdx == -1 || channelIdx == -1 {
		return nil, errors.NewValidationError("INVALID_HEADER", "CSV must have phone_number, consent_type, and channel columns")
	}

	// Process data rows
	for rowNum, record := range records[1:] {
		result.TotalRecords++

		if req.ValidateOnly {
			// Just validate, don't import
			if err := s.validateImportRow(record, phoneIdx, emailIdx, typeIdx, channelIdx); err != nil {
				result.FailureCount++
				result.Errors = append(result.Errors, ImportError{
					Row:     rowNum + 2,
					Message: err.Error(),
				})
			} else {
				result.SuccessCount++
			}
			continue
		}

		// Import the consent
		if err := s.importConsentRow(ctx, record, phoneIdx, emailIdx, typeIdx, channelIdx, req.Source); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, ImportError{
				Row:     rowNum + 2,
				Message: err.Error(),
			})
		} else {
			result.SuccessCount++
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func (s *service) importJSON(ctx context.Context, req ImportConsentsRequest, result *ImportResult, startTime time.Time) (*ImportResult, error) {
	var records []map[string]interface{}
	if err := json.Unmarshal(req.Data, &records); err != nil {
		return nil, errors.NewValidationError("INVALID_JSON", "failed to parse JSON").WithCause(err)
	}

	for i, record := range records {
		result.TotalRecords++

		phoneNumber, _ := record["phone_number"].(string)
		email, _ := record["email"].(string)
		consentTypeStr, _ := record["consent_type"].(string)
		channelStr, _ := record["channel"].(string)

		if req.ValidateOnly {
			// Just validate
			if phoneNumber == "" || consentTypeStr == "" || channelStr == "" {
				result.FailureCount++
				result.Errors = append(result.Errors, ImportError{
					Row:     i + 1,
					Message: "missing required fields",
				})
			} else {
				result.SuccessCount++
			}
			continue
		}

		// Import the consent
		consentType, err := consent.ParseType(consentTypeStr)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 1,
				Field:   "consent_type",
				Value:   consentTypeStr,
				Message: err.Error(),
			})
			continue
		}

		channel, err := consent.ParseChannel(channelStr)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 1,
				Field:   "channel",
				Value:   channelStr,
				Message: err.Error(),
			})
			continue
		}

		grantReq := GrantConsentRequest{
			PhoneNumber: phoneNumber,
			Email:       email,
			ConsentType: consentType,
			Channel:     channel,
		}

		if _, err := s.GrantConsent(ctx, grantReq); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 1,
				Message: err.Error(),
			})
		} else {
			result.SuccessCount++
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func (s *service) validateImportRow(record []string, phoneIdx, emailIdx, typeIdx, channelIdx int) error {
	if phoneIdx >= len(record) || record[phoneIdx] == "" {
		return fmt.Errorf("missing phone number")
	}

	if typeIdx >= len(record) || record[typeIdx] == "" {
		return fmt.Errorf("missing consent type")
	}

	if channelIdx >= len(record) || record[channelIdx] == "" {
		return fmt.Errorf("missing channel")
	}

	// Validate phone format
	if _, err := values.NewPhoneNumber(record[phoneIdx]); err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}

	// Validate email if provided
	if emailIdx != -1 && emailIdx < len(record) && record[emailIdx] != "" {
		if _, err := values.NewEmail(record[emailIdx]); err != nil {
			return fmt.Errorf("invalid email: %w", err)
		}
	}

	// Validate consent type
	if _, err := consent.ParseType(record[typeIdx]); err != nil {
		return fmt.Errorf("invalid consent type: %w", err)
	}

	// Validate channel
	if _, err := consent.ParseChannel(record[channelIdx]); err != nil {
		return fmt.Errorf("invalid channel: %w", err)
	}

	return nil
}

func (s *service) importConsentRow(ctx context.Context, record []string, phoneIdx, emailIdx, typeIdx, channelIdx int, source string) error {
	phone := record[phoneIdx]
	email := ""
	if emailIdx != -1 && emailIdx < len(record) {
		email = record[emailIdx]
	}

	consentType, _ := consent.ParseType(record[typeIdx])
	channel, _ := consent.ParseChannel(record[channelIdx])

	req := GrantConsentRequest{
		PhoneNumber: phone,
		Email:       email,
		ConsentType: consentType,
		Channel:     channel,
		Preferences: map[string]string{
			"import_source": source,
		},
	}

	_, err := s.GrantConsent(ctx, req)
	return err
}

func (s *service) exportCSV(ctx context.Context, consents []*consent.ConsentAggregate) (*ExportResult, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"phone_number", "email", "consent_type", "status", "channel", "granted_at", "expires_at", "version"}
	if err := writer.Write(header); err != nil {
		return nil, errors.NewInternalError("failed to write CSV header").WithCause(err)
	}

	// Write data
	for _, c := range consents {
		// Get consumer info
		consumer, err := s.consumerRepo.GetByID(ctx, c.ConsumerID)
		if err != nil {
			continue // Skip if consumer not found
		}

		// Get current version
		current := c.GetCurrentStatus()
		currentVersion := &c.Versions[c.CurrentVersion-1]

		var phoneNumber string
		if consumer.PhoneNumber != nil {
			phoneNumber = consumer.PhoneNumber.String()
		}

		var email string
		if consumer.Email != nil {
			email = *consumer.Email
		}

		var grantedAt string
		if currentVersion.ConsentedAt != nil {
			grantedAt = currentVersion.ConsentedAt.Format(time.RFC3339)
		}

		record := []string{
			phoneNumber,
			email,
			c.Type.String(),
			current.String(),
			currentVersion.Channels[0].String(), // Use first channel
			grantedAt,
			"",
			fmt.Sprintf("%d", currentVersion.Version),
		}

		if currentVersion.ExpiresAt != nil {
			record[6] = currentVersion.ExpiresAt.Format(time.RFC3339)
		}

		if err := writer.Write(record); err != nil {
			return nil, errors.NewInternalError("failed to write CSV record").WithCause(err)
		}
	}

	writer.Flush()
	data := []byte(buf.String())

	return &ExportResult{
		Format:      "csv",
		RecordCount: len(consents),
		FileSize:    int64(len(data)),
		Data:        data,
		GeneratedAt: time.Now(),
	}, nil
}

func (s *service) exportJSON(ctx context.Context, consents []*consent.ConsentAggregate) (*ExportResult, error) {
	type exportRecord struct {
		PhoneNumber string            `json:"phone_number"`
		Email       string            `json:"email,omitempty"`
		ConsentType string            `json:"consent_type"`
		Status      string            `json:"status"`
		Channel     string            `json:"channel"`
		Preferences map[string]string `json:"preferences,omitempty"`
		GrantedAt   time.Time         `json:"granted_at"`
		ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
		Version     int               `json:"version"`
	}

	records := make([]exportRecord, 0, len(consents))
	for _, c := range consents {
		// Get consumer info
		consumer, err := s.consumerRepo.GetByID(ctx, c.ConsumerID)
		if err != nil {
			continue // Skip if consumer not found
		}

		// Get current version
		current := c.GetCurrentStatus()
		currentVersion := &c.Versions[c.CurrentVersion-1]

		var phoneNumber string
		if consumer.PhoneNumber != nil {
			phoneNumber = consumer.PhoneNumber.String()
		}

		var email string
		if consumer.Email != nil {
			email = *consumer.Email
		}

		var grantedAt time.Time
		if currentVersion.ConsentedAt != nil {
			grantedAt = *currentVersion.ConsentedAt
		}

		records = append(records, exportRecord{
			PhoneNumber: phoneNumber,
			Email:       email,
			ConsentType: c.Type.String(),
			Status:      current.String(),
			Channel:     currentVersion.Channels[0].String(), // Use first channel
			Preferences: currentVersion.SourceDetails,
			GrantedAt:   grantedAt,
			ExpiresAt:   currentVersion.ExpiresAt,
			Version:     currentVersion.Version,
		})
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nil, errors.NewInternalError("failed to marshal JSON").WithCause(err)
	}

	return &ExportResult{
		Format:      "json",
		RecordCount: len(records),
		FileSize:    int64(len(data)),
		Data:        data,
		GeneratedAt: time.Now(),
	}, nil
}