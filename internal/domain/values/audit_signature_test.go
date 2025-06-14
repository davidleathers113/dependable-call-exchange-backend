package values

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid signature",
			signature: generateValidSignature(t),
			wantErr:   false,
		},
		{
			name:      "empty signature",
			signature: "",
			wantErr:   true,
			errCode:   "EMPTY_SIGNATURE",
		},
		{
			name:      "invalid base64",
			signature: "not-base64!@#",
			wantErr:   true,
			errCode:   "INVALID_SIGNATURE_ENCODING",
		},
		{
			name:      "wrong length",
			signature: base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:   true,
			errCode:   "INVALID_SIGNATURE_LENGTH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := NewAuditSignature(tt.signature)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
				assert.True(t, sig.IsEmpty())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.signature, sig.String())
				assert.False(t, sig.IsEmpty())
			}
		})
	}
}

func TestNewAuditSignatureFromBytes(t *testing.T) {
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
			errCode: "EMPTY_SIGNATURE_BYTES",
		},
		{
			name:    "wrong length",
			bytes:   make([]byte, 16),
			wantErr: true,
			errCode: "INVALID_SIGNATURE_LENGTH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := NewAuditSignatureFromBytes(tt.bytes)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				expected := base64.StdEncoding.EncodeToString(tt.bytes)
				assert.Equal(t, expected, sig.String())
			}
		})
	}
}

func TestComputeAuditSignature(t *testing.T) {
	data := []byte("test data to sign")
	secretKey := make([]byte, 32)
	rand.Read(secretKey)

	tests := []struct {
		name      string
		data      []byte
		secretKey []byte
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid data and key",
			data:      data,
			secretKey: secretKey,
			wantErr:   false,
		},
		{
			name:      "empty data",
			data:      []byte{},
			secretKey: secretKey,
			wantErr:   true,
			errCode:   "EMPTY_DATA",
		},
		{
			name:      "empty secret key",
			data:      data,
			secretKey: []byte{},
			wantErr:   true,
			errCode:   "EMPTY_SECRET_KEY",
		},
		{
			name:      "weak secret key",
			data:      data,
			secretKey: make([]byte, 16), // Less than 32 bytes
			wantErr:   true,
			errCode:   "WEAK_SECRET_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := ComputeAuditSignature(tt.data, tt.secretKey)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.False(t, sig.IsEmpty())
				
				// Verify the signature was computed correctly
				mac := hmac.New(sha256.New, tt.secretKey)
				mac.Write(tt.data)
				expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
				assert.Equal(t, expected, sig.String())
			}
		})
	}
}

func TestAuditSignature_Verify(t *testing.T) {
	data := []byte("test data")
	secretKey := make([]byte, 32)
	rand.Read(secretKey)

	// Create a valid signature
	sig, err := ComputeAuditSignature(data, secretKey)
	require.NoError(t, err)

	tests := []struct {
		name      string
		signature AuditSignature
		data      []byte
		secretKey []byte
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid signature verification",
			signature: sig,
			data:      data,
			secretKey: secretKey,
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "wrong data",
			signature: sig,
			data:      []byte("wrong data"),
			secretKey: secretKey,
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "wrong secret key",
			signature: sig,
			data:      data,
			secretKey: make([]byte, 32), // Different key
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "empty signature",
			signature: AuditSignature{},
			data:      data,
			secretKey: secretKey,
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := tt.signature.Verify(tt.data, tt.secretKey)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantValid, valid)
			}
		})
	}
}

func TestAuditSignature_Equal(t *testing.T) {
	sig1 := generateValidAuditSignature(t)
	sig2 := generateValidAuditSignature(t)
	sig3 := sig1 // Same signature

	assert.False(t, sig1.Equal(sig2))
	assert.True(t, sig1.Equal(sig3))
	assert.True(t, sig1.Equal(sig1))
}

func TestAuditSignature_Bytes(t *testing.T) {
	originalBytes := make([]byte, 32)
	rand.Read(originalBytes)

	sig, err := NewAuditSignatureFromBytes(originalBytes)
	require.NoError(t, err)

	bytes, err := sig.Bytes()
	require.NoError(t, err)
	assert.Equal(t, originalBytes, bytes)
}

func TestAuditSignature_Truncate(t *testing.T) {
	sig := generateValidAuditSignature(t)
	
	truncated := sig.Truncate()
	assert.Len(t, truncated, 11) // 8 characters + "..."
	assert.True(t, strings.HasSuffix(truncated, "..."), "Truncated signature should end with '...'")
	assert.Equal(t, sig.String()[:8]+"...", truncated)
}

func TestAuditSignature_Format(t *testing.T) {
	sig := generateValidAuditSignature(t)
	emptySig := AuditSignature{}

	formatted := sig.Format()
	assert.True(t, strings.HasPrefix(formatted, "sig:"), "Formatted signature should start with 'sig:'")
	assert.Contains(t, formatted, "...")

	emptyFormatted := emptySig.Format()
	assert.Equal(t, "<empty>", emptyFormatted)
}

func TestAuditSignature_JSON(t *testing.T) {
	sig := generateValidAuditSignature(t)

	// Test marshaling
	data, err := json.Marshal(sig)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled AuditSignature
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, sig.Equal(unmarshaled))
}

func TestAuditSignature_Database(t *testing.T) {
	sig := generateValidAuditSignature(t)

	// Test Value
	value, err := sig.Value()
	require.NoError(t, err)
	assert.Equal(t, sig.String(), value)

	// Test Scan
	var scanned AuditSignature
	err = scanned.Scan(value)
	require.NoError(t, err)
	assert.True(t, sig.Equal(scanned))

	// Test Scan with nil
	var nilSig AuditSignature
	err = nilSig.Scan(nil)
	require.NoError(t, err)
	assert.True(t, nilSig.IsEmpty())

	// Test Scan with bytes
	var bytesSig AuditSignature
	err = bytesSig.Scan([]byte(sig.String()))
	require.NoError(t, err)
	assert.True(t, sig.Equal(bytesSig))
}

func TestVerifySignatureChain(t *testing.T) {
	secretKey := make([]byte, 32)
	rand.Read(secretKey)

	data1 := []byte("first data")
	data2 := []byte("second data")
	data3 := []byte("third data")

	sig1, _ := ComputeAuditSignature(data1, secretKey)
	sig2, _ := ComputeAuditSignature(data2, secretKey)
	sig3, _ := ComputeAuditSignature(data3, secretKey)

	tests := []struct {
		name       string
		signatures []AuditSignature
		data       [][]byte
		wantErr    bool
	}{
		{
			name:       "valid chain",
			signatures: []AuditSignature{sig1, sig2, sig3},
			data:       [][]byte{data1, data2, data3},
			wantErr:    false,
		},
		{
			name:       "mismatched length",
			signatures: []AuditSignature{sig1, sig2},
			data:       [][]byte{data1, data2, data3},
			wantErr:    true,
		},
		{
			name:       "invalid signature in chain",
			signatures: []AuditSignature{sig1, sig2, sig3},
			data:       [][]byte{data1, []byte("wrong data"), data3},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySignatureChain(tt.signatures, tt.data, secretKey)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSignatureStrength(t *testing.T) {
	tests := []struct {
		keyLength int
		expected  SignatureStrength
	}{
		{8, SignatureStrengthWeak},
		{16, SignatureStrengthGood},
		{24, SignatureStrengthGood},
		{32, SignatureStrengthStrong},
		{64, SignatureStrengthStrong},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("key_length_%d", tt.keyLength), func(t *testing.T) {
			strength := GetSignatureStrength(tt.keyLength)
			assert.Equal(t, tt.expected, strength)
		})
	}
}

func TestValidateSignatureFormat(t *testing.T) {
	validSig := generateValidSignature(t)
	
	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{
			name:      "valid signature",
			signature: validSig,
			wantErr:   false,
		},
		{
			name:      "empty signature",
			signature: "",
			wantErr:   true,
		},
		{
			name:      "signature with whitespace",
			signature: validSig + " ",
			wantErr:   true,
		},
		{
			name:      "invalid base64",
			signature: "not-base64!",
			wantErr:   true,
		},
		{
			name:      "wrong length",
			signature: base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSignatureFormat(tt.signature)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Property-based tests
func TestAuditSignature_Properties(t *testing.T) {
	// Property: Signature of the same data with same key should always be equal
	t.Run("deterministic_signing", func(t *testing.T) {
		data := []byte("test data")
		secretKey := make([]byte, 32)
		rand.Read(secretKey)

		sig1, err := ComputeAuditSignature(data, secretKey)
		require.NoError(t, err)

		sig2, err := ComputeAuditSignature(data, secretKey)
		require.NoError(t, err)

		assert.True(t, sig1.Equal(sig2))
	})

	// Property: Valid signature should always verify against original data
	t.Run("sign_verify_roundtrip", func(t *testing.T) {
		data := []byte("test data for verification")
		secretKey := make([]byte, 32)
		rand.Read(secretKey)

		sig, err := ComputeAuditSignature(data, secretKey)
		require.NoError(t, err)

		valid, err := sig.Verify(data, secretKey)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	// Property: Different data should produce different signatures
	t.Run("different_data_different_signatures", func(t *testing.T) {
		secretKey := make([]byte, 32)
		rand.Read(secretKey)

		sig1, err := ComputeAuditSignature([]byte("data1"), secretKey)
		require.NoError(t, err)

		sig2, err := ComputeAuditSignature([]byte("data2"), secretKey)
		require.NoError(t, err)

		assert.False(t, sig1.Equal(sig2))
	})

	// Property: JSON marshaling/unmarshaling should preserve equality
	t.Run("json_roundtrip_preserves_equality", func(t *testing.T) {
		original := generateValidAuditSignature(t)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored AuditSignature
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})
}

// Helper functions
func generateValidSignature(t *testing.T) string {
	t.Helper()
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

func generateValidAuditSignature(t *testing.T) AuditSignature {
	t.Helper()
	sig, err := NewAuditSignature(generateValidSignature(t))
	require.NoError(t, err)
	return sig
}

// Benchmark tests
func BenchmarkNewAuditSignature(b *testing.B) {
	signature := generateValidSignature(&testing.T{})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewAuditSignature(signature)
	}
}

func BenchmarkComputeAuditSignature(b *testing.B) {
	data := []byte("test data to sign")
	secretKey := make([]byte, 32)
	rand.Read(secretKey)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeAuditSignature(data, secretKey)
	}
}

func BenchmarkAuditSignature_Verify(b *testing.B) {
	data := []byte("test data")
	secretKey := make([]byte, 32)
	rand.Read(secretKey)
	
	sig, _ := ComputeAuditSignature(data, secretKey)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sig.Verify(data, secretKey)
	}
}