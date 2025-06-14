package values

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// AuditSignature represents a cryptographic signature for audit trail integrity
type AuditSignature struct {
	signature string // Base64-encoded HMAC-SHA256 signature
}

// NewAuditSignature creates a new AuditSignature value object with validation
func NewAuditSignature(signature string) (AuditSignature, error) {
	if signature == "" {
		return AuditSignature{}, errors.NewValidationError("EMPTY_SIGNATURE", 
			"audit signature cannot be empty")
	}

	// Validate base64 encoding
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return AuditSignature{}, errors.NewValidationError("INVALID_SIGNATURE_ENCODING", 
			"audit signature must be valid base64").WithCause(err)
	}

	// HMAC-SHA256 produces 32 bytes (256 bits)
	if len(decoded) != 32 {
		return AuditSignature{}, errors.NewValidationError("INVALID_SIGNATURE_LENGTH", 
			"audit signature must be 32 bytes (HMAC-SHA256)")
	}

	return AuditSignature{signature: signature}, nil
}

// NewAuditSignatureFromBytes creates AuditSignature from raw bytes
func NewAuditSignatureFromBytes(bytes []byte) (AuditSignature, error) {
	if len(bytes) == 0 {
		return AuditSignature{}, errors.NewValidationError("EMPTY_SIGNATURE_BYTES", 
			"signature bytes cannot be empty")
	}

	if len(bytes) != 32 {
		return AuditSignature{}, errors.NewValidationError("INVALID_SIGNATURE_LENGTH", 
			"signature must be 32 bytes (HMAC-SHA256)")
	}

	encoded := base64.StdEncoding.EncodeToString(bytes)
	return AuditSignature{signature: encoded}, nil
}

// ComputeAuditSignature computes HMAC-SHA256 signature for data with secret key
func ComputeAuditSignature(data, secretKey []byte) (AuditSignature, error) {
	if len(data) == 0 {
		return AuditSignature{}, errors.NewValidationError("EMPTY_DATA", 
			"data to sign cannot be empty")
	}

	if len(secretKey) == 0 {
		return AuditSignature{}, errors.NewValidationError("EMPTY_SECRET_KEY", 
			"secret key cannot be empty")
	}

	if len(secretKey) < 32 {
		return AuditSignature{}, errors.NewValidationError("WEAK_SECRET_KEY", 
			"secret key must be at least 32 bytes for security")
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write(data)
	signature := mac.Sum(nil)

	return NewAuditSignatureFromBytes(signature)
}

// MustNewAuditSignature creates AuditSignature and panics on error (for constants/tests)
func MustNewAuditSignature(signature string) AuditSignature {
	sig, err := NewAuditSignature(signature)
	if err != nil {
		panic(err)
	}
	return sig
}

// String returns the base64-encoded signature
func (a AuditSignature) String() string {
	return a.signature
}

// Base64 returns the base64-encoded signature (alias for String)
func (a AuditSignature) Base64() string {
	return a.signature
}

// Bytes returns the raw signature bytes
func (a AuditSignature) Bytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(a.signature)
}

// IsEmpty checks if the signature is empty
func (a AuditSignature) IsEmpty() bool {
	return a.signature == ""
}

// Equal checks if two AuditSignature values are equal
func (a AuditSignature) Equal(other AuditSignature) bool {
	return a.signature == other.signature
}

// Verify verifies the signature against data and secret key
func (a AuditSignature) Verify(data, secretKey []byte) (bool, error) {
	if a.IsEmpty() {
		return false, errors.NewValidationError("EMPTY_SIGNATURE", 
			"cannot verify empty signature")
	}

	expectedSig, err := ComputeAuditSignature(data, secretKey)
	if err != nil {
		return false, fmt.Errorf("failed to compute expected signature: %w", err)
	}

	// Use constant-time comparison to prevent timing attacks
	expectedBytes, err := expectedSig.Bytes()
	if err != nil {
		return false, fmt.Errorf("failed to decode expected signature: %w", err)
	}

	actualBytes, err := a.Bytes()
	if err != nil {
		return false, fmt.Errorf("failed to decode actual signature: %w", err)
	}

	return hmac.Equal(actualBytes, expectedBytes), nil
}

// Truncate returns a truncated signature for display purposes (first 8 characters)
func (a AuditSignature) Truncate() string {
	if len(a.signature) <= 8 {
		return a.signature
	}
	return a.signature[:8] + "..."
}

// Format returns a formatted string for logging/display
func (a AuditSignature) Format() string {
	if a.IsEmpty() {
		return "<empty>"
	}
	return fmt.Sprintf("sig:%s", a.Truncate())
}

// MarshalJSON implements JSON marshaling
func (a AuditSignature) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.signature)
}

// UnmarshalJSON implements JSON unmarshaling
func (a *AuditSignature) UnmarshalJSON(data []byte) error {
	var signature string
	if err := json.Unmarshal(data, &signature); err != nil {
		return err
	}

	sig, err := NewAuditSignature(signature)
	if err != nil {
		return err
	}

	*a = sig
	return nil
}

// Value implements driver.Valuer for database storage
func (a AuditSignature) Value() (driver.Value, error) {
	if a.signature == "" {
		return nil, nil
	}
	return a.signature, nil
}

// Scan implements sql.Scanner for database retrieval
func (a *AuditSignature) Scan(value interface{}) error {
	if value == nil {
		*a = AuditSignature{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into AuditSignature", value)
	}

	if str == "" {
		*a = AuditSignature{}
		return nil
	}

	sig, err := NewAuditSignature(str)
	if err != nil {
		return err
	}

	*a = sig
	return nil
}

// ValidationError represents validation errors for audit signatures
type AuditSignatureValidationError struct {
	Signature string
	Reason    string
}

func (e AuditSignatureValidationError) Error() string {
	return fmt.Sprintf("invalid audit signature '%s': %s", e.Signature, e.Reason)
}

// VerifySignatureChain verifies a chain of signatures for integrity
func VerifySignatureChain(signatures []AuditSignature, data [][]byte, secretKey []byte) error {
	if len(signatures) != len(data) {
		return errors.NewValidationError("SIGNATURE_DATA_MISMATCH", 
			"number of signatures must match number of data items")
	}

	for i, sig := range signatures {
		valid, err := sig.Verify(data[i], secretKey)
		if err != nil {
			return fmt.Errorf("failed to verify signature %d: %w", i, err)
		}
		if !valid {
			return errors.NewValidationError("INVALID_SIGNATURE_CHAIN", 
				fmt.Sprintf("signature %d failed verification", i))
		}
	}

	return nil
}

// SignatureStrength represents the cryptographic strength of a signature
type SignatureStrength string

const (
	SignatureStrengthWeak   SignatureStrength = "weak"
	SignatureStrengthGood   SignatureStrength = "good"
	SignatureStrengthStrong SignatureStrength = "strong"
)

// GetSignatureStrength analyzes the signature strength based on the secret key length
func GetSignatureStrength(secretKeyLength int) SignatureStrength {
	switch {
	case secretKeyLength < 16:
		return SignatureStrengthWeak
	case secretKeyLength < 32:
		return SignatureStrengthGood
	default:
		return SignatureStrengthStrong
	}
}

// ValidateSignatureFormat validates that a string could be a valid signature format
func ValidateSignatureFormat(signature string) error {
	if signature == "" {
		return errors.NewValidationError("EMPTY_SIGNATURE", "signature cannot be empty")
	}

	// Check if it looks like base64
	if strings.ContainsAny(signature, " \t\n\r") {
		return errors.NewValidationError("INVALID_SIGNATURE_FORMAT", 
			"signature cannot contain whitespace")
	}

	// Try to decode as base64
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errors.NewValidationError("INVALID_BASE64", 
			"signature must be valid base64 encoding").WithCause(err)
	}

	// Check expected length for HMAC-SHA256
	if len(decoded) != 32 {
		return errors.NewValidationError("INVALID_SIGNATURE_LENGTH", 
			fmt.Sprintf("expected 32 bytes, got %d bytes", len(decoded)))
	}

	return nil
}