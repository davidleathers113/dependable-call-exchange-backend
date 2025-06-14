package consent

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentAggregate is the root aggregate for consent management
type ConsentAggregate struct {
	ID              uuid.UUID
	ConsumerID      uuid.UUID
	BusinessID      uuid.UUID
	Type            Type
	CurrentVersion  int
	Versions        []ConsentVersion
	CreatedAt       time.Time
	UpdatedAt       time.Time
	events          []interface{} // Domain events
}

// ConsentVersion tracks each version of consent
type ConsentVersion struct {
	ID              uuid.UUID
	ConsentID       uuid.UUID
	Version         int
	Status          ConsentStatus
	Channels        []Channel
	Purpose         Purpose
	ConsentedAt     *time.Time
	RevokedAt       *time.Time
	ExpiresAt       *time.Time
	Source          ConsentSource
	SourceDetails   map[string]string
	Proofs          []ConsentProof
	CreatedAt       time.Time
	CreatedBy       uuid.UUID
}

// ConsentProof stores evidence of consent
type ConsentProof struct {
	ID              uuid.UUID
	VersionID       uuid.UUID
	Type            ProofType
	StorageLocation string // S3 key or blob reference
	Hash            string // SHA-256 of content
	Metadata        ProofMetadata
	CreatedAt       time.Time
}

// ProofMetadata contains proof-specific data
type ProofMetadata struct {
	IPAddress       string
	UserAgent       string
	RecordingURL    string
	TranscriptURL   string
	FormData        map[string]string
	TCPALanguage    string
	Duration        *time.Duration
}

// ConsentStatus represents the current state of consent
type ConsentStatus string

const (
	StatusActive    ConsentStatus = "active"
	StatusRevoked   ConsentStatus = "revoked"
	StatusExpired   ConsentStatus = "expired"
	StatusPending   ConsentStatus = "pending"
)

// String returns the string representation of the consent status
func (s ConsentStatus) String() string {
	return string(s)
}

// Type represents different types of consent (TCPA, GDPR, etc.)
type Type string

const (
	TypeTCPA      Type = "tcpa"
	TypeGDPR      Type = "gdpr"
	TypeCCPA      Type = "ccpa"
	TypeMarketing Type = "marketing"
	TypeDNC       Type = "dnc"
)

// String returns the string representation of the consent type
func (t Type) String() string {
	return string(t)
}

// Channel represents communication channels
type Channel string

const (
	ChannelVoice    Channel = "voice"
	ChannelSMS      Channel = "sms"
	ChannelEmail    Channel = "email"
	ChannelFax      Channel = "fax"
	ChannelWeb      Channel = "web"
	ChannelAPI      Channel = "api"
)

// String returns the string representation of the channel
func (c Channel) String() string {
	return string(c)
}

// Purpose defines the reason for communication
type Purpose string

const (
	PurposeMarketing        Purpose = "marketing"
	PurposeServiceCalls     Purpose = "service_calls"
	PurposeDebtCollection   Purpose = "debt_collection"
	PurposeEmergency        Purpose = "emergency"
)

// ConsentSource indicates how consent was obtained
type ConsentSource string

const (
	SourceWebForm       ConsentSource = "web_form"
	SourceVoiceRecording ConsentSource = "voice_recording"
	SourceSMS           ConsentSource = "sms_reply"
	SourceEmailReply    ConsentSource = "email_reply"
	SourceAPI           ConsentSource = "api"
	SourceImport        ConsentSource = "import"
)

// ProofType categorizes evidence
type ProofType string

const (
	ProofTypeRecording      ProofType = "recording"
	ProofTypeTranscript     ProofType = "transcript"
	ProofTypeFormSubmission ProofType = "form_submission"
	ProofTypeSMSLog         ProofType = "sms_log"
	ProofTypeEmailLog       ProofType = "email_log"
	ProofTypeSignature      ProofType = "signature"
	ProofTypeDigital        ProofType = "digital"
)

// NewConsentAggregate creates a new consent with validation
func NewConsentAggregate(consumerID, businessID uuid.UUID, consentType Type, channels []Channel, purpose Purpose, source ConsentSource) (*ConsentAggregate, error) {
	if consumerID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_CONSUMER", "consumer ID is required")
	}
	if businessID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_BUSINESS", "business ID is required")
	}
	if len(channels) == 0 {
		return nil, errors.NewValidationError("NO_CHANNELS", "at least one channel is required")
	}

	now := time.Now()
	consentID := uuid.New()

	firstVersion := ConsentVersion{
		ID:            uuid.New(),
		ConsentID:     consentID,
		Version:       1,
		Status:        StatusPending,
		Channels:      channels,
		Purpose:       purpose,
		Source:        source,
		SourceDetails: make(map[string]string),
		CreatedAt:     now,
	}

	aggregate := &ConsentAggregate{
		ID:             consentID,
		ConsumerID:     consumerID,
		BusinessID:     businessID,
		Type:           consentType,
		CurrentVersion: 1,
		Versions:       []ConsentVersion{firstVersion},
		CreatedAt:      now,
		UpdatedAt:      now,
		events:         []interface{}{},
	}

	// Emit domain event
	aggregate.addEvent(ConsentCreatedEvent{
		ConsentID:  consentID,
		ConsumerID: consumerID,
		BusinessID: businessID,
		Channels:   channels,
		Purpose:    purpose,
		Source:     source,
		CreatedAt:  now,
	})

	return aggregate, nil
}

// ActivateConsent records consent with proof
func (c *ConsentAggregate) ActivateConsent(proofs []ConsentProof, expiresAt *time.Time) error {
	current := c.getCurrentVersion()
	if current == nil {
		return errors.NewInternalError("no current version found")
	}

	if current.Status != StatusPending {
		return errors.NewValidationError("INVALID_STATE", "consent must be pending to activate")
	}

	if len(proofs) == 0 {
		return errors.NewValidationError("NO_PROOF", "at least one proof is required for activation")
	}

	now := time.Now()
	current.Status = StatusActive
	current.ConsentedAt = &now
	current.ExpiresAt = expiresAt
	current.Proofs = proofs

	// Extract preferences from proof metadata and store in SourceDetails
	if current.SourceDetails == nil {
		current.SourceDetails = make(map[string]string)
	}
	for _, proof := range proofs {
		if proof.Metadata.FormData != nil {
			for k, v := range proof.Metadata.FormData {
				current.SourceDetails[k] = v
			}
		}
	}

	c.UpdatedAt = now

	// Emit domain event
	c.addEvent(ConsentActivatedEvent{
		ConsentID:   c.ID,
		ConsumerID:  c.ConsumerID,
		BusinessID:  c.BusinessID,
		Channels:    current.Channels,
		ActivatedAt: now,
		ExpiresAt:   expiresAt,
	})

	return nil
}

// RevokeConsent creates a new version with revoked status
func (c *ConsentAggregate) RevokeConsent(reason string, revokedBy uuid.UUID) error {
	current := c.getCurrentVersion()
	if current == nil {
		return errors.NewInternalError("no current version found")
	}

	if current.Status != StatusActive {
		return errors.NewValidationError("NOT_ACTIVE", "only active consent can be revoked")
	}

	now := time.Now()
	newVersion := ConsentVersion{
		ID:            uuid.New(),
		ConsentID:     c.ID,
		Version:       c.CurrentVersion + 1,
		Status:        StatusRevoked,
		Channels:      current.Channels,
		Purpose:       current.Purpose,
		RevokedAt:     &now,
		Source:        current.Source,
		SourceDetails: map[string]string{"revoke_reason": reason},
		CreatedAt:     now,
		CreatedBy:     revokedBy,
	}

	c.Versions = append(c.Versions, newVersion)
	c.CurrentVersion++
	c.UpdatedAt = now

	// Emit domain event
	c.addEvent(ConsentRevokedEvent{
		ConsentID:  c.ID,
		ConsumerID: c.ConsumerID,
		BusinessID: c.BusinessID,
		RevokedAt:  now,
		Reason:     reason,
		RevokedBy:  revokedBy,
	})

	return nil
}

// UpdateChannels updates the consent channels
func (c *ConsentAggregate) UpdateChannels(channels []Channel, updatedBy uuid.UUID) error {
	current := c.getCurrentVersion()
	if current == nil {
		return errors.NewInternalError("no current version found")
	}

	if current.Status != StatusActive {
		return errors.NewValidationError("NOT_ACTIVE", "only active consent can be updated")
	}

	if len(channels) == 0 {
		return errors.NewValidationError("NO_CHANNELS", "at least one channel is required")
	}

	now := time.Now()
	newVersion := ConsentVersion{
		ID:            uuid.New(),
		ConsentID:     c.ID,
		Version:       c.CurrentVersion + 1,
		Status:        current.Status,
		Channels:      channels,
		Purpose:       current.Purpose,
		ConsentedAt:   current.ConsentedAt,
		ExpiresAt:     current.ExpiresAt,
		Source:        current.Source,
		SourceDetails: current.SourceDetails,
		Proofs:        current.Proofs,
		CreatedAt:     now,
		CreatedBy:     updatedBy,
	}

	c.Versions = append(c.Versions, newVersion)
	c.CurrentVersion++
	c.UpdatedAt = now

	// Emit domain event
	c.addEvent(ConsentUpdatedEvent{
		ConsentID:      c.ID,
		ConsumerID:     c.ConsumerID,
		BusinessID:     c.BusinessID,
		UpdatedAt:      now,
		UpdatedBy:      updatedBy,
		OldChannels:    current.Channels,
		NewChannels:    channels,
	})

	return nil
}

// IsActive checks if the consent is currently active
func (c *ConsentAggregate) IsActive() bool {
	current := c.getCurrentVersion()
	if current == nil {
		return false
	}

	if current.Status != StatusActive {
		return false
	}

	// Check expiration
	if current.ExpiresAt != nil && time.Now().After(*current.ExpiresAt) {
		return false
	}

	return true
}

// GetActiveChannels returns the currently active channels
func (c *ConsentAggregate) GetActiveChannels() []Channel {
	if !c.IsActive() {
		return []Channel{}
	}

	current := c.getCurrentVersion()
	if current == nil {
		return []Channel{}
	}

	return current.Channels
}

// HasChannelConsent checks if consent exists for a specific channel
func (c *ConsentAggregate) HasChannelConsent(channel Channel) bool {
	if !c.IsActive() {
		return false
	}

	current := c.getCurrentVersion()
	if current == nil {
		return false
	}

	for _, ch := range current.Channels {
		if ch == channel {
			return true
		}
	}

	return false
}

// GetCurrentStatus returns the current consent status
func (c *ConsentAggregate) GetCurrentStatus() ConsentStatus {
	current := c.getCurrentVersion()
	if current == nil {
		return StatusExpired
	}

	// Check if expired
	if current.Status == StatusActive && current.ExpiresAt != nil && time.Now().After(*current.ExpiresAt) {
		return StatusExpired
	}

	return current.Status
}

// GetEvents returns all domain events
func (c *ConsentAggregate) GetEvents() []interface{} {
	return c.events
}

// ClearEvents clears the domain events (after processing)
func (c *ConsentAggregate) ClearEvents() {
	c.events = []interface{}{}
}

// Grant grants consent with proof (alias for ActivateConsent for service compatibility)
func (c *ConsentAggregate) Grant(proof ConsentProof, preferences map[string]string, expiresAt *time.Time) error {
	// Add preferences to the proof metadata if provided
	if preferences != nil {
		if proof.Metadata.FormData == nil {
			proof.Metadata.FormData = make(map[string]string)
		}
		for k, v := range preferences {
			proof.Metadata.FormData[k] = v
		}
	}
	
	// Store preferences in the current version's SourceDetails as well
	current := c.getCurrentVersion()
	if current != nil && preferences != nil {
		if current.SourceDetails == nil {
			current.SourceDetails = make(map[string]string)
		}
		for k, v := range preferences {
			current.SourceDetails[k] = v
		}
	}
	
	proofs := []ConsentProof{proof}
	return c.ActivateConsent(proofs, expiresAt)
}

// Revoke revokes consent with default reason (alias for RevokeConsent for service compatibility)
func (c *ConsentAggregate) Revoke(reason string) error {
	// Use system UUID as revokedBy - in production this should come from context
	systemUserID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	return c.RevokeConsent(reason, systemUserID)
}

// UpdatePreferences updates consent preferences
func (c *ConsentAggregate) UpdatePreferences(preferences map[string]string) error {
	current := c.getCurrentVersion()
	if current == nil {
		return errors.NewInternalError("no current version found")
	}

	if current.Status != StatusActive {
		return errors.NewValidationError("NOT_ACTIVE", "only active consent can be updated")
	}

	// Create a new version with updated preferences
	now := time.Now()
	systemUserID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	
	// Update source details with new preferences
	newSourceDetails := make(map[string]string)
	for k, v := range current.SourceDetails {
		newSourceDetails[k] = v
	}
	for k, v := range preferences {
		newSourceDetails[k] = v
	}

	newVersion := ConsentVersion{
		ID:            uuid.New(),
		ConsentID:     c.ID,
		Version:       c.CurrentVersion + 1,
		Status:        current.Status,
		Channels:      current.Channels,
		Purpose:       current.Purpose,
		ConsentedAt:   current.ConsentedAt,
		ExpiresAt:     current.ExpiresAt,
		Source:        current.Source,
		SourceDetails: newSourceDetails,
		Proofs:        current.Proofs,
		CreatedAt:     now,
		CreatedBy:     systemUserID,
	}

	c.Versions = append(c.Versions, newVersion)
	c.CurrentVersion++
	c.UpdatedAt = now

	// Emit domain event
	c.addEvent(ConsentUpdatedEvent{
		ConsentID:      c.ID,
		ConsumerID:     c.ConsumerID,
		BusinessID:     c.BusinessID,
		UpdatedAt:      now,
		UpdatedBy:      systemUserID,
		OldChannels:    current.Channels,
		NewChannels:    current.Channels, // Same channels, different preferences
	})

	return nil
}

// UpdateExpiration updates the consent expiration time
func (c *ConsentAggregate) UpdateExpiration(expiresAt time.Time) error {
	current := c.getCurrentVersion()
	if current == nil {
		return errors.NewInternalError("no current version found")
	}

	if current.Status != StatusActive {
		return errors.NewValidationError("NOT_ACTIVE", "only active consent can be updated")
	}

	// Create a new version with updated expiration
	now := time.Now()
	systemUserID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	newVersion := ConsentVersion{
		ID:            uuid.New(),
		ConsentID:     c.ID,
		Version:       c.CurrentVersion + 1,
		Status:        current.Status,
		Channels:      current.Channels,
		Purpose:       current.Purpose,
		ConsentedAt:   current.ConsentedAt,
		ExpiresAt:     &expiresAt,
		Source:        current.Source,
		SourceDetails: current.SourceDetails,
		Proofs:        current.Proofs,
		CreatedAt:     now,
		CreatedBy:     systemUserID,
	}

	c.Versions = append(c.Versions, newVersion)
	c.CurrentVersion++
	c.UpdatedAt = now

	// Emit domain event
	c.addEvent(ConsentUpdatedEvent{
		ConsentID:      c.ID,
		ConsumerID:     c.ConsumerID,
		BusinessID:     c.BusinessID,
		UpdatedAt:      now,
		UpdatedBy:      systemUserID,
		OldChannels:    current.Channels,
		NewChannels:    current.Channels,
	})

	return nil
}

// Events returns all domain events (alias for GetEvents for service compatibility)
func (c *ConsentAggregate) Events() []interface{} {
	return c.GetEvents()
}

// Private helper methods

func (c *ConsentAggregate) getCurrentVersion() *ConsentVersion {
	if c.CurrentVersion <= 0 || c.CurrentVersion > len(c.Versions) {
		return nil
	}
	return &c.Versions[c.CurrentVersion-1]
}

func (c *ConsentAggregate) addEvent(event interface{}) {
	c.events = append(c.events, event)
}

// Validation helper methods

// ValidateChannel validates if a channel is valid
func ValidateChannel(channel Channel) error {
	switch channel {
	case ChannelVoice, ChannelSMS, ChannelEmail, ChannelFax:
		return nil
	default:
		return errors.NewValidationError("INVALID_CHANNEL", fmt.Sprintf("invalid channel: %s", channel))
	}
}

// ValidatePurpose validates if a purpose is valid
func ValidatePurpose(purpose Purpose) error {
	switch purpose {
	case PurposeMarketing, PurposeServiceCalls, PurposeDebtCollection, PurposeEmergency:
		return nil
	default:
		return errors.NewValidationError("INVALID_PURPOSE", fmt.Sprintf("invalid purpose: %s", purpose))
	}
}

// ValidateSource validates if a consent source is valid
func ValidateSource(source ConsentSource) error {
	switch source {
	case SourceWebForm, SourceVoiceRecording, SourceSMS, SourceEmailReply, SourceAPI, SourceImport:
		return nil
	default:
		return errors.NewValidationError("INVALID_SOURCE", fmt.Sprintf("invalid source: %s", source))
	}
}

// IsExpired checks if the consent has expired
func (c *ConsentAggregate) IsExpired() bool {
	current := c.getCurrentVersion()
	if current == nil {
		return true
	}
	
	if current.ExpiresAt != nil && time.Now().After(*current.ExpiresAt) {
		return true
	}
	
	return false
}

// ValidateType validates if a consent type is valid
func ValidateType(consentType Type) error {
	switch consentType {
	case TypeTCPA, TypeGDPR, TypeCCPA, TypeMarketing:
		return nil
	default:
		return errors.NewValidationError("INVALID_TYPE", fmt.Sprintf("invalid consent type: %s", consentType))
	}
}

// ParseType parses a string into a consent Type
func ParseType(s string) (Type, error) {
	switch s {
	case "tcpa":
		return TypeTCPA, nil
	case "gdpr":
		return TypeGDPR, nil
	case "ccpa":
		return TypeCCPA, nil
	case "marketing":
		return TypeMarketing, nil
	default:
		return "", errors.NewValidationError("INVALID_TYPE", fmt.Sprintf("invalid consent type: %s", s))
	}
}

// ParseChannel parses a string into a consent Channel
func ParseChannel(s string) (Channel, error) {
	switch s {
	case "voice":
		return ChannelVoice, nil
	case "sms":
		return ChannelSMS, nil
	case "email":
		return ChannelEmail, nil
	case "fax":
		return ChannelFax, nil
	case "web":
		return ChannelWeb, nil
	case "api":
		return ChannelAPI, nil
	default:
		return "", errors.NewValidationError("INVALID_CHANNEL", fmt.Sprintf("invalid channel: %s", s))
	}
}

// Domain Events

// ConsentCreatedEvent is emitted when a new consent is created
type ConsentCreatedEvent struct {
	ConsentID  uuid.UUID
	ConsumerID uuid.UUID
	BusinessID uuid.UUID
	Channels   []Channel
	Purpose    Purpose
	Source     ConsentSource
	CreatedAt  time.Time
}

// ConsentActivatedEvent is emitted when consent is activated with proof
type ConsentActivatedEvent struct {
	ConsentID   uuid.UUID
	ConsumerID  uuid.UUID
	BusinessID  uuid.UUID
	Channels    []Channel
	ActivatedAt time.Time
	ExpiresAt   *time.Time
}

// ConsentRevokedEvent is emitted when consent is revoked
type ConsentRevokedEvent struct {
	ConsentID  uuid.UUID
	ConsumerID uuid.UUID
	BusinessID uuid.UUID
	RevokedAt  time.Time
	Reason     string
	RevokedBy  uuid.UUID
}

// ConsentUpdatedEvent is emitted when consent channels are updated
type ConsentUpdatedEvent struct {
	ConsentID   uuid.UUID
	ConsumerID  uuid.UUID
	BusinessID  uuid.UUID
	UpdatedAt   time.Time
	UpdatedBy   uuid.UUID
	OldChannels []Channel
	NewChannels []Channel
}