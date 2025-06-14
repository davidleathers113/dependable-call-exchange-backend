package values

import (
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// HashValue represents a SHA-256 hash value for audit trail integrity
type HashValue struct {
	hash string // Hex-encoded SHA-256 hash (64 characters)
}

var (
	// SHA-256 hex regex: exactly 64 hex characters
	sha256HexRegex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

// NewHashValue creates a new HashValue value object with validation
func NewHashValue(hash string) (HashValue, error) {
	if hash == "" {
		return HashValue{}, errors.NewValidationError("EMPTY_HASH", 
			"hash value cannot be empty")
	}

	// Normalize to lowercase
	normalized := strings.ToLower(strings.TrimSpace(hash))

	// Validate hex format and length
	if !sha256HexRegex.MatchString(normalized) {
		return HashValue{}, errors.NewValidationError("INVALID_HASH_FORMAT", 
			"hash must be a 64-character hexadecimal string (SHA-256)")
	}

	return HashValue{hash: normalized}, nil
}

// NewHashValueFromBytes creates HashValue from raw bytes
func NewHashValueFromBytes(bytes []byte) (HashValue, error) {
	if len(bytes) == 0 {
		return HashValue{}, errors.NewValidationError("EMPTY_HASH_BYTES", 
			"hash bytes cannot be empty")
	}

	if len(bytes) != 32 {
		return HashValue{}, errors.NewValidationError("INVALID_HASH_LENGTH", 
			"hash must be 32 bytes (SHA-256)")
	}

	hex := hex.EncodeToString(bytes)
	return HashValue{hash: hex}, nil
}

// ComputeHashValue computes SHA-256 hash for the given data
func ComputeHashValue(data []byte) (HashValue, error) {
	if len(data) == 0 {
		return HashValue{}, errors.NewValidationError("EMPTY_DATA", 
			"data to hash cannot be empty")
	}

	hash := sha256.Sum256(data)
	return NewHashValueFromBytes(hash[:])
}

// ComputeHashValueFromString computes SHA-256 hash for string data
func ComputeHashValueFromString(data string) (HashValue, error) {
	return ComputeHashValue([]byte(data))
}

// MustNewHashValue creates HashValue and panics on error (for constants/tests)
func MustNewHashValue(hash string) HashValue {
	h, err := NewHashValue(hash)
	if err != nil {
		panic(err)
	}
	return h
}

// MustComputeHashValue computes hash and panics on error (for constants/tests)
func MustComputeHashValue(data []byte) HashValue {
	h, err := ComputeHashValue(data)
	if err != nil {
		panic(err)
	}
	return h
}

// String returns the hex-encoded hash
func (h HashValue) String() string {
	return h.hash
}

// Hex returns the hex-encoded hash (alias for String)
func (h HashValue) Hex() string {
	return h.hash
}

// Bytes returns the raw hash bytes
func (h HashValue) Bytes() ([]byte, error) {
	return hex.DecodeString(h.hash)
}

// IsEmpty checks if the hash is empty
func (h HashValue) IsEmpty() bool {
	return h.hash == ""
}

// Equal checks if two HashValue objects are equal
func (h HashValue) Equal(other HashValue) bool {
	return h.hash == other.hash
}

// IsZero checks if the hash is all zeros (null hash)
func (h HashValue) IsZero() bool {
	return h.hash == strings.Repeat("0", 64)
}

// Compare returns -1, 0, or 1 based on lexicographic comparison
func (h HashValue) Compare(other HashValue) int {
	if h.hash < other.hash {
		return -1
	}
	if h.hash > other.hash {
		return 1
	}
	return 0
}

// Verify verifies that the hash matches the provided data
func (h HashValue) Verify(data []byte) (bool, error) {
	if h.IsEmpty() {
		return false, errors.NewValidationError("EMPTY_HASH", 
			"cannot verify against empty hash")
	}

	expectedHash, err := ComputeHashValue(data)
	if err != nil {
		return false, fmt.Errorf("failed to compute expected hash: %w", err)
	}

	return h.Equal(expectedHash), nil
}

// VerifyString verifies that the hash matches the provided string data
func (h HashValue) VerifyString(data string) (bool, error) {
	return h.Verify([]byte(data))
}

// Truncate returns a truncated hash for display purposes (first 8 characters)
func (h HashValue) Truncate() string {
	if len(h.hash) <= 8 {
		return h.hash
	}
	return h.hash[:8]
}

// TruncateLong returns a longer truncated hash for display (first 16 characters)
func (h HashValue) TruncateLong() string {
	if len(h.hash) <= 16 {
		return h.hash
	}
	return h.hash[:16]
}

// Format returns a formatted string for logging/display
func (h HashValue) Format() string {
	if h.IsEmpty() {
		return "<empty>"
	}
	return fmt.Sprintf("hash:%s", h.Truncate())
}

// FormatLong returns a longer formatted string for detailed logging
func (h HashValue) FormatLong() string {
	if h.IsEmpty() {
		return "<empty>"
	}
	return fmt.Sprintf("hash:%s", h.TruncateLong())
}

// StartsWith checks if the hash starts with the given prefix
func (h HashValue) StartsWith(prefix string) bool {
	if len(prefix) > len(h.hash) {
		return false
	}
	return strings.HasPrefix(h.hash, strings.ToLower(prefix))
}

// EndsWith checks if the hash ends with the given suffix
func (h HashValue) EndsWith(suffix string) bool {
	if len(suffix) > len(h.hash) {
		return false
	}
	return strings.HasSuffix(h.hash, strings.ToLower(suffix))
}

// MarshalJSON implements JSON marshaling
func (h HashValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.hash)
}

// UnmarshalJSON implements JSON unmarshaling
func (h *HashValue) UnmarshalJSON(data []byte) error {
	var hash string
	if err := json.Unmarshal(data, &hash); err != nil {
		return err
	}

	hashValue, err := NewHashValue(hash)
	if err != nil {
		return err
	}

	*h = hashValue
	return nil
}

// Value implements driver.Valuer for database storage
func (h HashValue) Value() (driver.Value, error) {
	if h.hash == "" {
		return nil, nil
	}
	return h.hash, nil
}

// Scan implements sql.Scanner for database retrieval
func (h *HashValue) Scan(value interface{}) error {
	if value == nil {
		*h = HashValue{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into HashValue", value)
	}

	if str == "" {
		*h = HashValue{}
		return nil
	}

	hashValue, err := NewHashValue(str)
	if err != nil {
		return err
	}

	*h = hashValue
	return nil
}

// Chain represents a chain of hash values for integrity verification
type HashChain struct {
	hashes []HashValue
}

// NewHashChain creates a new hash chain from a slice of hash values
func NewHashChain(hashes []HashValue) (*HashChain, error) {
	if len(hashes) == 0 {
		return nil, errors.NewValidationError("EMPTY_HASH_CHAIN", 
			"hash chain cannot be empty")
	}

	// Validate all hashes
	for i, hash := range hashes {
		if hash.IsEmpty() {
			return nil, errors.NewValidationError("INVALID_HASH_CHAIN", 
				fmt.Sprintf("hash at index %d is empty", i))
		}
	}

	return &HashChain{hashes: hashes}, nil
}

// Add appends a hash to the chain
func (hc *HashChain) Add(hash HashValue) error {
	if hash.IsEmpty() {
		return errors.NewValidationError("EMPTY_HASH", 
			"cannot add empty hash to chain")
	}

	hc.hashes = append(hc.hashes, hash)
	return nil
}

// Get returns the hash at the specified index
func (hc *HashChain) Get(index int) (HashValue, error) {
	if index < 0 || index >= len(hc.hashes) {
		return HashValue{}, errors.NewValidationError("INVALID_INDEX", 
			fmt.Sprintf("index %d out of range [0, %d)", index, len(hc.hashes)))
	}
	return hc.hashes[index], nil
}

// Length returns the number of hashes in the chain
func (hc *HashChain) Length() int {
	return len(hc.hashes)
}

// ComputeChainHash computes a hash of the entire chain
func (hc *HashChain) ComputeChainHash() (HashValue, error) {
	if len(hc.hashes) == 0 {
		return HashValue{}, errors.NewValidationError("EMPTY_CHAIN", 
			"cannot compute hash of empty chain")
	}

	// Concatenate all hashes and compute hash of result
	var combined strings.Builder
	for _, hash := range hc.hashes {
		combined.WriteString(hash.String())
	}

	return ComputeHashValueFromString(combined.String())
}

// Verify verifies the integrity of the hash chain against provided data
func (hc *HashChain) Verify(data [][]byte) error {
	if len(data) != len(hc.hashes) {
		return errors.NewValidationError("HASH_DATA_MISMATCH", 
			"number of hashes must match number of data items")
	}

	for i, hash := range hc.hashes {
		valid, err := hash.Verify(data[i])
		if err != nil {
			return fmt.Errorf("failed to verify hash %d: %w", i, err)
		}
		if !valid {
			return errors.NewValidationError("INVALID_HASH_CHAIN", 
				fmt.Sprintf("hash %d failed verification", i))
		}
	}

	return nil
}

// ValidationError represents validation errors for hash values
type HashValidationError struct {
	Hash   string
	Reason string
}

func (e HashValidationError) Error() string {
	return fmt.Sprintf("invalid hash value '%s': %s", e.Hash, e.Reason)
}

// ValidateHashFormat validates that a string could be a valid hash format
func ValidateHashFormat(hash string) error {
	if hash == "" {
		return errors.NewValidationError("EMPTY_HASH", "hash cannot be empty")
	}

	// Check for invalid characters
	if strings.ContainsAny(hash, " \t\n\r") {
		return errors.NewValidationError("INVALID_HASH_FORMAT", 
			"hash cannot contain whitespace")
	}

	// Normalize and validate format
	normalized := strings.ToLower(strings.TrimSpace(hash))
	if !sha256HexRegex.MatchString(normalized) {
		return errors.NewValidationError("INVALID_HASH_FORMAT", 
			"hash must be a 64-character hexadecimal string")
	}

	return nil
}

// Zero returns a zero hash value (all zeros)
func ZeroHash() HashValue {
	return MustNewHashValue(strings.Repeat("0", 64))
}

// RandomHash generates a hash from random data (for testing purposes)
func RandomHash() HashValue {
	// This is a simple implementation for testing
	// In production, you might want to use crypto/rand
	data := fmt.Sprintf("random_%d", sha256.Sum256([]byte("entropy")))
	hash, _ := ComputeHashValueFromString(data)
	return hash
}