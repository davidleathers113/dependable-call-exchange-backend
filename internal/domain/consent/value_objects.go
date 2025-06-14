package consent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentHash represents a hash of consent data for integrity verification
type ConsentHash struct {
	value string
}

// NewConsentHash creates a new consent hash
func NewConsentHash(consentID, consumerID, businessID string, channels []Channel, timestamp time.Time) ConsentHash {
	data := fmt.Sprintf("%s:%s:%s:%v:%d", consentID, consumerID, businessID, channels, timestamp.Unix())
	hash := sha256.Sum256([]byte(data))
	return ConsentHash{
		value: hex.EncodeToString(hash[:]),
	}
}

// String returns the hash value
func (h ConsentHash) String() string {
	return h.value
}

// Equals checks if two hashes are equal
func (h ConsentHash) Equals(other ConsentHash) bool {
	return h.value == other.value
}

// TCPADisclosure represents the TCPA disclosure text
type TCPADisclosure struct {
	text      string
	version   string
	timestamp time.Time
}

// NewTCPADisclosure creates a new TCPA disclosure
func NewTCPADisclosure(text, version string) (*TCPADisclosure, error) {
	if text == "" {
		return nil, errors.NewValidationError("EMPTY_DISCLOSURE", "TCPA disclosure text cannot be empty")
	}

	if len(text) < 50 {
		return nil, errors.NewValidationError("DISCLOSURE_TOO_SHORT", "TCPA disclosure must be at least 50 characters")
	}

	if version == "" {
		return nil, errors.NewValidationError("EMPTY_VERSION", "TCPA disclosure version cannot be empty")
	}

	return &TCPADisclosure{
		text:      text,
		version:   version,
		timestamp: time.Now(),
	}, nil
}

// Text returns the disclosure text
func (d TCPADisclosure) Text() string {
	return d.text
}

// Version returns the disclosure version
func (d TCPADisclosure) Version() string {
	return d.version
}

// Timestamp returns when the disclosure was created
func (d TCPADisclosure) Timestamp() time.Time {
	return d.timestamp
}

// ConsentDuration represents how long consent is valid
type ConsentDuration struct {
	duration time.Duration
}

// NewConsentDuration creates a new consent duration
func NewConsentDuration(duration time.Duration) (*ConsentDuration, error) {
	if duration <= 0 {
		return nil, errors.NewValidationError("INVALID_DURATION", "consent duration must be positive")
	}

	// Maximum consent duration is 2 years for TCPA compliance
	maxDuration := 2 * 365 * 24 * time.Hour
	if duration > maxDuration {
		return nil, errors.NewValidationError("DURATION_TOO_LONG", "consent duration cannot exceed 2 years")
	}

	return &ConsentDuration{
		duration: duration,
	}, nil
}

// Duration returns the duration value
func (d ConsentDuration) Duration() time.Duration {
	return d.duration
}

// ExpiresAt calculates the expiration time from a start time
func (d ConsentDuration) ExpiresAt(from time.Time) time.Time {
	return from.Add(d.duration)
}

// ConsentScope defines the scope of consent
type ConsentScope struct {
	BusinessUnits []string
	Products      []string
	Campaigns     []string
}

// NewConsentScope creates a new consent scope
func NewConsentScope(businessUnits, products, campaigns []string) (*ConsentScope, error) {
	// At least one scope dimension must be specified
	if len(businessUnits) == 0 && len(products) == 0 && len(campaigns) == 0 {
		return nil, errors.NewValidationError("EMPTY_SCOPE", "at least one scope dimension must be specified")
	}

	return &ConsentScope{
		BusinessUnits: businessUnits,
		Products:      products,
		Campaigns:     campaigns,
	}, nil
}

// Contains checks if the scope contains a specific element
func (s ConsentScope) Contains(businessUnit, product, campaign string) bool {
	// Empty scope dimensions mean "all"
	buMatch := len(s.BusinessUnits) == 0 || contains(s.BusinessUnits, businessUnit)
	prodMatch := len(s.Products) == 0 || contains(s.Products, product)
	campMatch := len(s.Campaigns) == 0 || contains(s.Campaigns, campaign)

	return buMatch && prodMatch && campMatch
}

// ProofHash represents a cryptographic hash of proof content
type ProofHash struct {
	algorithm string
	value     string
}

// NewProofHash creates a new proof hash using SHA-256
func NewProofHash(content []byte) ProofHash {
	hash := sha256.Sum256(content)
	return ProofHash{
		algorithm: "SHA256",
		value:     hex.EncodeToString(hash[:]),
	}
}

// Algorithm returns the hashing algorithm used
func (h ProofHash) Algorithm() string {
	return h.algorithm
}

// Value returns the hash value
func (h ProofHash) Value() string {
	return h.value
}

// String returns the formatted hash
func (h ProofHash) String() string {
	return fmt.Sprintf("%s:%s", h.algorithm, h.value)
}

// Verify checks if the provided content matches the hash
func (h ProofHash) Verify(content []byte) bool {
	newHash := NewProofHash(content)
	return h.value == newHash.value
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}