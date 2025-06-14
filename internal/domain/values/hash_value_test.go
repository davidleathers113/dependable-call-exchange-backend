package values

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHashValue(t *testing.T) {
	validHash := generateValidHash(t)
	
	tests := []struct {
		name    string
		hash    string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid hash",
			hash:    validHash,
			wantErr: false,
		},
		{
			name:    "valid hash uppercase",
			hash:    strings.ToUpper(validHash),
			wantErr: false,
		},
		{
			name:    "empty hash",
			hash:    "",
			wantErr: true,
			errCode: "EMPTY_HASH",
		},
		{
			name:    "invalid characters",
			hash:    "g" + validHash[1:], // 'g' is not hex
			wantErr: true,
			errCode: "INVALID_HASH_FORMAT",
		},
		{
			name:    "too short",
			hash:    validHash[:32], // 32 chars instead of 64
			wantErr: true,
			errCode: "INVALID_HASH_FORMAT",
		},
		{
			name:    "too long",
			hash:    validHash + "00", // 66 chars instead of 64
			wantErr: true,
			errCode: "INVALID_HASH_FORMAT",
		},
		{
			name:    "hash with whitespace",
			hash:    " " + validHash + " ",
			wantErr: false, // Should be trimmed and normalized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := NewHashValue(tt.hash)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
				assert.True(t, hash.IsEmpty())
			} else {
				assert.NoError(t, err)
				assert.False(t, hash.IsEmpty())
				assert.Equal(t, strings.ToLower(strings.TrimSpace(tt.hash)), hash.String())
			}
		})
	}
}

func TestNewHashValueFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		wantErr bool
		errCode string
	}{
		{
			name:    "valid 32 bytes",
			bytes:   make([]byte, 32),
			wantErr: false,
		},
		{
			name:    "empty bytes",
			bytes:   []byte{},
			wantErr: true,
			errCode: "EMPTY_HASH_BYTES",
		},
		{
			name:    "wrong length",
			bytes:   make([]byte, 16),
			wantErr: true,
			errCode: "INVALID_HASH_LENGTH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := NewHashValueFromBytes(tt.bytes)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				expected := hex.EncodeToString(tt.bytes)
				assert.Equal(t, expected, hash.String())
			}
		})
	}
}

func TestComputeHashValue(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		errCode string
	}{
		{
			name:    "valid data",
			data:    []byte("test data"),
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errCode: "EMPTY_DATA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := ComputeHashValue(tt.data)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.False(t, hash.IsEmpty())
				
				// Verify the hash was computed correctly
				expected := sha256.Sum256(tt.data)
				expectedHex := hex.EncodeToString(expected[:])
				assert.Equal(t, expectedHex, hash.String())
			}
		})
	}
}

func TestComputeHashValueFromString(t *testing.T) {
	data := "test string"
	hash, err := ComputeHashValueFromString(data)
	require.NoError(t, err)

	// Verify against manual computation
	expected := sha256.Sum256([]byte(data))
	expectedHex := hex.EncodeToString(expected[:])
	assert.Equal(t, expectedHex, hash.String())
}

func TestHashValue_Equal(t *testing.T) {
	data := []byte("test data")
	
	hash1, _ := ComputeHashValue(data)
	hash2, _ := ComputeHashValue(data)
	hash3, _ := ComputeHashValue([]byte("different data"))

	assert.True(t, hash1.Equal(hash2))
	assert.False(t, hash1.Equal(hash3))
	assert.True(t, hash1.Equal(hash1))
}

func TestHashValue_Compare(t *testing.T) {
	hash1 := MustNewHashValue("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	hash2 := MustNewHashValue("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	hash3 := MustNewHashValue("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	assert.Equal(t, -1, hash1.Compare(hash2))
	assert.Equal(t, 1, hash2.Compare(hash1))
	assert.Equal(t, 0, hash1.Compare(hash3))
}

func TestHashValue_IsZero(t *testing.T) {
	zeroHash := MustNewHashValue(strings.Repeat("0", 64))
	nonZeroHash := generateValidHashValue(t)

	assert.True(t, zeroHash.IsZero())
	assert.False(t, nonZeroHash.IsZero())
}

func TestHashValue_Bytes(t *testing.T) {
	originalBytes := []byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
	}

	hash, err := NewHashValueFromBytes(originalBytes)
	require.NoError(t, err)

	bytes, err := hash.Bytes()
	require.NoError(t, err)
	assert.Equal(t, originalBytes, bytes)
}

func TestHashValue_Verify(t *testing.T) {
	data := []byte("test data")
	hash, err := ComputeHashValue(data)
	require.NoError(t, err)

	tests := []struct {
		name      string
		hash      HashValue
		data      []byte
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid verification",
			hash:      hash,
			data:      data,
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "wrong data",
			hash:      hash,
			data:      []byte("wrong data"),
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "empty hash",
			hash:      HashValue{},
			data:      data,
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := tt.hash.Verify(tt.data)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantValid, valid)
			}
		})
	}
}

func TestHashValue_VerifyString(t *testing.T) {
	data := "test string"
	hash, err := ComputeHashValueFromString(data)
	require.NoError(t, err)

	valid, err := hash.VerifyString(data)
	require.NoError(t, err)
	assert.True(t, valid)

	valid, err = hash.VerifyString("wrong string")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestHashValue_Truncate(t *testing.T) {
	hash := generateValidHashValue(t)

	truncated := hash.Truncate()
	assert.Len(t, truncated, 8)
	assert.Equal(t, hash.String()[:8], truncated)

	truncatedLong := hash.TruncateLong()
	assert.Len(t, truncatedLong, 16)
	assert.Equal(t, hash.String()[:16], truncatedLong)
}

func TestHashValue_Format(t *testing.T) {
	hash := generateValidHashValue(t)
	emptyHash := HashValue{}

	formatted := hash.Format()
	assert.True(t, strings.HasPrefix(formatted, "hash:"), "Formatted hash should start with 'hash:'")
	assert.Equal(t, "hash:"+hash.Truncate(), formatted)

	formattedLong := hash.FormatLong()
	assert.True(t, strings.HasPrefix(formattedLong, "hash:"), "Long formatted hash should start with 'hash:'")
	assert.Equal(t, "hash:"+hash.TruncateLong(), formattedLong)

	emptyFormatted := emptyHash.Format()
	assert.Equal(t, "<empty>", emptyFormatted)
}

func TestHashValue_StartsWith(t *testing.T) {
	hash := MustNewHashValue("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	assert.True(t, hash.StartsWith("abcd"))
	assert.True(t, hash.StartsWith("ABCD")) // Case insensitive
	assert.False(t, hash.StartsWith("xyz"))
	assert.False(t, hash.StartsWith(hash.String()+"extra")) // Longer than hash
}

func TestHashValue_EndsWith(t *testing.T) {
	hash := MustNewHashValue("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	assert.True(t, hash.EndsWith("7890"))
	assert.True(t, hash.EndsWith("7890")) // Case insensitive
	assert.False(t, hash.EndsWith("xyz"))
	assert.False(t, hash.EndsWith("extra"+hash.String())) // Longer than hash
}

func TestHashValue_JSON(t *testing.T) {
	hash := generateValidHashValue(t)

	// Test marshaling
	data, err := json.Marshal(hash)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled HashValue
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, hash.Equal(unmarshaled))
}

func TestHashValue_Database(t *testing.T) {
	hash := generateValidHashValue(t)

	// Test Value
	value, err := hash.Value()
	require.NoError(t, err)
	assert.Equal(t, hash.String(), value)

	// Test Scan
	var scanned HashValue
	err = scanned.Scan(value)
	require.NoError(t, err)
	assert.True(t, hash.Equal(scanned))

	// Test Scan with nil
	var nilHash HashValue
	err = nilHash.Scan(nil)
	require.NoError(t, err)
	assert.True(t, nilHash.IsEmpty())

	// Test Scan with bytes
	var bytesHash HashValue
	err = bytesHash.Scan([]byte(hash.String()))
	require.NoError(t, err)
	assert.True(t, hash.Equal(bytesHash))
}

func TestHashChain(t *testing.T) {
	hash1 := generateValidHashValue(t)
	hash2 := generateValidHashValue(t)
	hash3 := generateValidHashValue(t)

	// Test NewHashChain
	chain, err := NewHashChain([]HashValue{hash1, hash2, hash3})
	require.NoError(t, err)
	assert.Equal(t, 3, chain.Length())

	// Test empty chain
	_, err = NewHashChain([]HashValue{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EMPTY_HASH_CHAIN")

	// Test chain with empty hash
	_, err = NewHashChain([]HashValue{hash1, HashValue{}, hash3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_HASH_CHAIN")

	// Test Add
	err = chain.Add(generateValidHashValue(t))
	assert.NoError(t, err)
	assert.Equal(t, 4, chain.Length())

	// Test Get
	retrieved, err := chain.Get(0)
	require.NoError(t, err)
	assert.True(t, hash1.Equal(retrieved))

	// Test invalid index
	_, err = chain.Get(10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_INDEX")

	// Test ComputeChainHash
	chainHash, err := chain.ComputeChainHash()
	require.NoError(t, err)
	assert.False(t, chainHash.IsEmpty())
}

func TestHashChain_Verify(t *testing.T) {
	data1 := []byte("first data")
	data2 := []byte("second data")
	data3 := []byte("third data")

	hash1, _ := ComputeHashValue(data1)
	hash2, _ := ComputeHashValue(data2)
	hash3, _ := ComputeHashValue(data3)

	chain, _ := NewHashChain([]HashValue{hash1, hash2, hash3})

	// Test valid verification
	err := chain.Verify([][]byte{data1, data2, data3})
	assert.NoError(t, err)

	// Test mismatched length
	err = chain.Verify([][]byte{data1, data2})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HASH_DATA_MISMATCH")

	// Test invalid hash
	err = chain.Verify([][]byte{data1, []byte("wrong data"), data3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_HASH_CHAIN")
}

func TestZeroHash(t *testing.T) {
	zero := ZeroHash()
	assert.True(t, zero.IsZero())
	assert.Equal(t, strings.Repeat("0", 64), zero.String())
}

func TestValidateHashFormat(t *testing.T) {
	validHash := generateValidHash(t)

	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "valid hash",
			hash:    validHash,
			wantErr: false,
		},
		{
			name:    "empty hash",
			hash:    "",
			wantErr: true,
		},
		{
			name:    "hash with whitespace",
			hash:    validHash + " ",
			wantErr: true,
		},
		{
			name:    "invalid format",
			hash:    "not-a-hash",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHashFormat(tt.hash)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Property-based tests
func TestHashValue_Properties(t *testing.T) {
	// Property: Hash of the same data should always be equal
	t.Run("deterministic_hashing", func(t *testing.T) {
		data := []byte("test data")

		hash1, err := ComputeHashValue(data)
		require.NoError(t, err)

		hash2, err := ComputeHashValue(data)
		require.NoError(t, err)

		assert.True(t, hash1.Equal(hash2))
	})

	// Property: Valid hash should always verify against original data
	t.Run("hash_verify_roundtrip", func(t *testing.T) {
		data := []byte("test data for verification")

		hash, err := ComputeHashValue(data)
		require.NoError(t, err)

		valid, err := hash.Verify(data)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	// Property: Different data should produce different hashes
	t.Run("different_data_different_hashes", func(t *testing.T) {
		hash1, err := ComputeHashValue([]byte("data1"))
		require.NoError(t, err)

		hash2, err := ComputeHashValue([]byte("data2"))
		require.NoError(t, err)

		assert.False(t, hash1.Equal(hash2))
	})

	// Property: JSON marshaling/unmarshaling should preserve equality
	t.Run("json_roundtrip_preserves_equality", func(t *testing.T) {
		original := generateValidHashValue(t)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored HashValue
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Bytes roundtrip should preserve equality
	t.Run("bytes_roundtrip_preserves_equality", func(t *testing.T) {
		original := generateValidHashValue(t)

		bytes, err := original.Bytes()
		require.NoError(t, err)

		restored, err := NewHashValueFromBytes(bytes)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})
}

// Helper functions
func generateValidHash(t *testing.T) string {
	t.Helper()
	data := []byte("test data for hash generation")
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func generateValidHashValue(t *testing.T) HashValue {
	t.Helper()
	hash, err := NewHashValue(generateValidHash(t))
	require.NoError(t, err)
	return hash
}

// Benchmark tests
func BenchmarkNewHashValue(b *testing.B) {
	hash := generateValidHash(&testing.T{})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewHashValue(hash)
	}
}

func BenchmarkComputeHashValue(b *testing.B) {
	data := []byte("test data to hash")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeHashValue(data)
	}
}

func BenchmarkHashValue_Verify(b *testing.B) {
	data := []byte("test data")
	hash, _ := ComputeHashValue(data)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hash.Verify(data)
	}
}