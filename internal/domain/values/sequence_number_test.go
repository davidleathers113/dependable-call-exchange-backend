package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSequenceNumber(t *testing.T) {
	tests := []struct {
		name    string
		value   uint64
		wantErr bool
		errCode string
	}{
		{
			name:    "valid sequence number",
			value:   1,
			wantErr: false,
		},
		{
			name:    "large valid sequence number",
			value:   MaxSequenceNumber,
			wantErr: false,
		},
		{
			name:    "zero sequence number",
			value:   0,
			wantErr: true,
			errCode: "ZERO_SEQUENCE",
		},
		{
			name:    "too large sequence number",
			value:   MaxSequenceNumber + 1,
			wantErr: true,
			errCode: "SEQUENCE_TOO_LARGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := NewSequenceNumber(tt.value)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
				assert.True(t, seq.IsZero())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.value, seq.Value())
				assert.False(t, seq.IsZero())
			}
		})
	}
}

func TestNewSequenceNumberFromString(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid string",
			value:   "123",
			wantErr: false,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
			errCode: "EMPTY_SEQUENCE",
		},
		{
			name:    "invalid format",
			value:   "abc",
			wantErr: true,
			errCode: "INVALID_SEQUENCE_FORMAT",
		},
		{
			name:    "negative number",
			value:   "-1",
			wantErr: true,
			errCode: "INVALID_SEQUENCE_FORMAT",
		},
		{
			name:    "zero",
			value:   "0",
			wantErr: true,
			errCode: "ZERO_SEQUENCE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := NewSequenceNumberFromString(tt.value)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.value, seq.String())
			}
		})
	}
}

func TestFirstSequenceNumber(t *testing.T) {
	first := FirstSequenceNumber()
	assert.Equal(t, MinSequenceNumber, first.Value())
	assert.True(t, first.IsFirst())
}

func TestSequenceNumber_Equal(t *testing.T) {
	seq1 := MustNewSequenceNumber(42)
	seq2 := MustNewSequenceNumber(42)
	seq3 := MustNewSequenceNumber(100)

	assert.True(t, seq1.Equal(seq2))
	assert.False(t, seq1.Equal(seq3))
	assert.True(t, seq1.Equal(seq1))
}

func TestSequenceNumber_Compare(t *testing.T) {
	seq1 := MustNewSequenceNumber(10)
	seq2 := MustNewSequenceNumber(20)
	seq3 := MustNewSequenceNumber(10)

	assert.Equal(t, -1, seq1.Compare(seq2))
	assert.Equal(t, 1, seq2.Compare(seq1))
	assert.Equal(t, 0, seq1.Compare(seq3))
}

func TestSequenceNumber_ComparisonMethods(t *testing.T) {
	seq1 := MustNewSequenceNumber(10)
	seq2 := MustNewSequenceNumber(20)
	seq3 := MustNewSequenceNumber(10)

	// LessThan
	assert.True(t, seq1.LessThan(seq2))
	assert.False(t, seq2.LessThan(seq1))
	assert.False(t, seq1.LessThan(seq3))

	// LessThanOrEqual
	assert.True(t, seq1.LessThanOrEqual(seq2))
	assert.False(t, seq2.LessThanOrEqual(seq1))
	assert.True(t, seq1.LessThanOrEqual(seq3))

	// GreaterThan
	assert.False(t, seq1.GreaterThan(seq2))
	assert.True(t, seq2.GreaterThan(seq1))
	assert.False(t, seq1.GreaterThan(seq3))

	// GreaterThanOrEqual
	assert.False(t, seq1.GreaterThanOrEqual(seq2))
	assert.True(t, seq2.GreaterThanOrEqual(seq1))
	assert.True(t, seq1.GreaterThanOrEqual(seq3))
}

func TestSequenceNumber_Next(t *testing.T) {
	seq := MustNewSequenceNumber(10)
	
	next, err := seq.Next()
	require.NoError(t, err)
	assert.Equal(t, uint64(11), next.Value())

	// Test overflow
	maxSeq := MustNewSequenceNumber(MaxSequenceNumber)
	_, err = maxSeq.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_OVERFLOW")
}

func TestSequenceNumber_Previous(t *testing.T) {
	seq := MustNewSequenceNumber(10)
	
	prev, err := seq.Previous()
	require.NoError(t, err)
	assert.Equal(t, uint64(9), prev.Value())

	// Test underflow
	minSeq := MustNewSequenceNumber(MinSequenceNumber)
	_, err = minSeq.Previous()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_UNDERFLOW")
}

func TestSequenceNumber_Add(t *testing.T) {
	seq := MustNewSequenceNumber(10)
	
	// Test normal addition
	result, err := seq.Add(5)
	require.NoError(t, err)
	assert.Equal(t, uint64(15), result.Value())

	// Test adding zero
	result, err = seq.Add(0)
	require.NoError(t, err)
	assert.Equal(t, seq.Value(), result.Value())

	// Test overflow
	largeSeq := MustNewSequenceNumber(MaxSequenceNumber - 5)
	_, err = largeSeq.Add(10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_OVERFLOW")
}

func TestSequenceNumber_Subtract(t *testing.T) {
	seq := MustNewSequenceNumber(10)
	
	// Test normal subtraction
	result, err := seq.Subtract(5)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), result.Value())

	// Test subtracting zero
	result, err = seq.Subtract(0)
	require.NoError(t, err)
	assert.Equal(t, seq.Value(), result.Value())

	// Test underflow
	_, err = seq.Subtract(15)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_UNDERFLOW")

	// Test underflow to below minimum
	minSeq := MustNewSequenceNumber(MinSequenceNumber + 1)
	_, err = minSeq.Subtract(2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_UNDERFLOW")
}

func TestSequenceNumber_Distance(t *testing.T) {
	seq1 := MustNewSequenceNumber(10)
	seq2 := MustNewSequenceNumber(20)
	seq3 := MustNewSequenceNumber(5)

	assert.Equal(t, uint64(10), seq1.Distance(seq2))
	assert.Equal(t, uint64(10), seq2.Distance(seq1))
	assert.Equal(t, uint64(5), seq1.Distance(seq3))
	assert.Equal(t, uint64(0), seq1.Distance(seq1))
}

func TestSequenceNumber_InRange(t *testing.T) {
	seq := MustNewSequenceNumber(15)
	min := MustNewSequenceNumber(10)
	max := MustNewSequenceNumber(20)

	assert.True(t, seq.InRange(min, max))
	assert.True(t, min.InRange(min, max)) // Boundary
	assert.True(t, max.InRange(min, max)) // Boundary

	outOfRange := MustNewSequenceNumber(25)
	assert.False(t, outOfRange.InRange(min, max))
}

func TestSequenceNumber_Format(t *testing.T) {
	seq := MustNewSequenceNumber(123)
	emptySeq := SequenceNumber{}

	formatted := seq.Format()
	assert.Equal(t, "seq:123", formatted)

	emptyFormatted := emptySeq.Format()
	assert.Equal(t, "<invalid>", emptyFormatted)

	withPrefix := seq.FormatWithPrefix("audit")
	assert.Equal(t, "audit:123", withPrefix)

	emptyWithPrefix := emptySeq.FormatWithPrefix("audit")
	assert.Equal(t, "audit:<invalid>", emptyWithPrefix)
}

func TestSequenceNumber_JSON(t *testing.T) {
	seq := MustNewSequenceNumber(42)

	// Test marshaling
	data, err := json.Marshal(seq)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled SequenceNumber
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, seq.Equal(unmarshaled))
}

func TestSequenceNumber_Database(t *testing.T) {
	seq := MustNewSequenceNumber(123)

	// Test Value
	value, err := seq.DatabaseValue()
	require.NoError(t, err)
	assert.Equal(t, int64(123), value)

	// Test Scan with int64
	var scanned SequenceNumber
	err = scanned.Scan(int64(123))
	require.NoError(t, err)
	assert.True(t, seq.Equal(scanned))

	// Test Scan with uint64
	var scannedUint SequenceNumber
	err = scannedUint.Scan(uint64(123))
	require.NoError(t, err)
	assert.True(t, seq.Equal(scannedUint))

	// Test Scan with string
	var scannedString SequenceNumber
	err = scannedString.Scan("123")
	require.NoError(t, err)
	assert.True(t, seq.Equal(scannedString))

	// Test Scan with nil
	var nilSeq SequenceNumber
	err = nilSeq.Scan(nil)
	require.NoError(t, err)
	assert.True(t, nilSeq.IsZero())

	// Test Scan with negative value
	var negativeSeq SequenceNumber
	err = negativeSeq.Scan(int64(-1))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be negative")
}

func TestSequenceGenerator(t *testing.T) {
	// Test NewSequenceGenerator
	gen, err := NewSequenceGenerator(10)
	require.NoError(t, err)

	// Test Next
	seq1, err := gen.Next()
	require.NoError(t, err)
	assert.Equal(t, uint64(10), seq1.Value())

	seq2, err := gen.Next()
	require.NoError(t, err)
	assert.Equal(t, uint64(11), seq2.Value())

	// Test Current
	current := gen.Current()
	assert.Equal(t, uint64(11), current.Value())

	// Test Reset
	err = gen.Reset(20)
	require.NoError(t, err)

	seq3, err := gen.Next()
	require.NoError(t, err)
	assert.Equal(t, uint64(20), seq3.Value())

	// Test invalid start value
	_, err = NewSequenceGenerator(MaxSequenceNumber + 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_START_SEQUENCE")
}

func TestSequenceGenerator_Exhaustion(t *testing.T) {
	// Create generator near maximum
	gen, err := NewSequenceGenerator(MaxSequenceNumber)
	require.NoError(t, err)

	// Generate the last valid sequence
	seq, err := gen.Next()
	require.NoError(t, err)
	assert.Equal(t, MaxSequenceNumber, seq.Value())

	// Try to generate beyond maximum
	_, err = gen.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SEQUENCE_EXHAUSTED")
}

func TestSequenceRange(t *testing.T) {
	start := MustNewSequenceNumber(10)
	end := MustNewSequenceNumber(20)

	// Test NewSequenceRange
	r, err := NewSequenceRange(start, end)
	require.NoError(t, err)

	// Test Contains
	assert.True(t, r.Contains(MustNewSequenceNumber(15)))
	assert.True(t, r.Contains(start)) // Boundary
	assert.True(t, r.Contains(end))   // Boundary
	assert.False(t, r.Contains(MustNewSequenceNumber(5)))
	assert.False(t, r.Contains(MustNewSequenceNumber(25)))

	// Test Size
	assert.Equal(t, uint64(11), r.Size()) // 10-20 inclusive = 11 numbers

	// Test String
	assert.Equal(t, "[10-20]", r.String())

	// Test invalid range
	_, err = NewSequenceRange(end, start) // start > end
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_RANGE")

	// Test zero values
	_, err = NewSequenceRange(SequenceNumber{}, end)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_RANGE")
}

func TestValidateSequenceNumber(t *testing.T) {
	tests := []struct {
		name    string
		value   uint64
		wantErr bool
	}{
		{
			name:    "valid sequence",
			value:   123,
			wantErr: false,
		},
		{
			name:    "zero sequence",
			value:   0,
			wantErr: true,
		},
		{
			name:    "too large sequence",
			value:   MaxSequenceNumber + 1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSequenceNumber(tt.value)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Property-based tests
func TestSequenceNumber_Properties(t *testing.T) {
	// Property: Next().Previous() should return original (except at boundaries)
	t.Run("next_previous_roundtrip", func(t *testing.T) {
		original := MustNewSequenceNumber(100)

		next, err := original.Next()
		require.NoError(t, err)

		restored, err := next.Previous()
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Add(n).Subtract(n) should return original (except at boundaries)
	t.Run("add_subtract_roundtrip", func(t *testing.T) {
		original := MustNewSequenceNumber(100)
		delta := uint64(42)

		added, err := original.Add(delta)
		require.NoError(t, err)

		restored, err := added.Subtract(delta)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Distance is symmetric
	t.Run("distance_is_symmetric", func(t *testing.T) {
		seq1 := MustNewSequenceNumber(50)
		seq2 := MustNewSequenceNumber(100)

		dist1 := seq1.Distance(seq2)
		dist2 := seq2.Distance(seq1)

		assert.Equal(t, dist1, dist2)
	})

	// Property: JSON marshaling/unmarshaling should preserve equality
	t.Run("json_roundtrip_preserves_equality", func(t *testing.T) {
		original := MustNewSequenceNumber(42)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored SequenceNumber
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Compare is consistent with LessThan/GreaterThan
	t.Run("compare_consistent_with_comparisons", func(t *testing.T) {
		seq1 := MustNewSequenceNumber(50)
		seq2 := MustNewSequenceNumber(100)

		cmp := seq1.Compare(seq2)
		assert.Equal(t, -1, cmp)
		assert.True(t, seq1.LessThan(seq2))
		assert.False(t, seq1.GreaterThan(seq2))

		cmp = seq2.Compare(seq1)
		assert.Equal(t, 1, cmp)
		assert.False(t, seq2.LessThan(seq1))
		assert.True(t, seq2.GreaterThan(seq1))
	})
}

// Concurrency tests for SequenceGenerator
func TestSequenceGenerator_Concurrency(t *testing.T) {
	gen, err := NewSequenceGenerator(1)
	require.NoError(t, err)

	const numGoroutines = 100
	results := make(chan uint64, numGoroutines)

	// Launch multiple goroutines to generate sequences
	for i := 0; i < numGoroutines; i++ {
		go func() {
			seq, err := gen.Next()
			if err != nil {
				results <- 0 // Signal error
			} else {
				results <- seq.Value()
			}
		}()
	}

	// Collect results
	seen := make(map[uint64]bool)
	for i := 0; i < numGoroutines; i++ {
		result := <-results
		require.NotEqual(t, uint64(0), result, "Generator should not return error")
		require.False(t, seen[result], "Sequence %d was generated twice", result)
		seen[result] = true
	}

	// Verify we got exactly numGoroutines unique sequences
	assert.Len(t, seen, numGoroutines)
}

// Benchmark tests
func BenchmarkNewSequenceNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewSequenceNumber(uint64(i + 1))
	}
}

func BenchmarkSequenceNumber_Next(b *testing.B) {
	seq := MustNewSequenceNumber(1000000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq, _ = seq.Next()
	}
}

func BenchmarkSequenceGenerator_Next(b *testing.B) {
	gen, _ := NewSequenceGenerator(1)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.Next()
	}
}