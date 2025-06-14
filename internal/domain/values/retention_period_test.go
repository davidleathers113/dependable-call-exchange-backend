package values

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetentionPeriod(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		wantErr  bool
		errCode  string
	}{
		{
			name:     "valid 7 year retention",
			duration: 7 * RetentionYear,
			wantErr:  false,
		},
		{
			name:     "valid 10 year retention",
			duration: 10 * RetentionYear,
			wantErr:  false,
		},
		{
			name:     "zero duration",
			duration: 0,
			wantErr:  true,
			errCode:  "INVALID_RETENTION_DURATION",
		},
		{
			name:     "negative duration",
			duration: -time.Hour,
			wantErr:  true,
			errCode:  "INVALID_RETENTION_DURATION",
		},
		{
			name:     "too short retention",
			duration: 5 * RetentionYear,
			wantErr:  true,
			errCode:  "RETENTION_TOO_SHORT",
		},
		{
			name:     "too long retention",
			duration: 150 * RetentionYear,
			wantErr:  true,
			errCode:  "RETENTION_TOO_LONG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp, err := NewRetentionPeriod(tt.duration)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
				assert.True(t, rp.IsZero())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.duration, rp.Duration())
				assert.False(t, rp.IsZero())
			}
		})
	}
}

func TestNewRetentionPeriodFromYears(t *testing.T) {
	tests := []struct {
		name    string
		years   int
		wantErr bool
		errCode string
	}{
		{
			name:    "valid 7 years",
			years:   7,
			wantErr: false,
		},
		{
			name:    "valid minimum years",
			years:   MinRetentionYears,
			wantErr: false,
		},
		{
			name:    "valid maximum years",
			years:   MaxRetentionYears,
			wantErr: false,
		},
		{
			name:    "too few years",
			years:   5,
			wantErr: true,
			errCode: "RETENTION_TOO_SHORT",
		},
		{
			name:    "too many years",
			years:   150,
			wantErr: true,
			errCode: "RETENTION_TOO_LONG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp, err := NewRetentionPeriodFromYears(tt.years)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.years, rp.Years())
				assert.Equal(t, time.Duration(tt.years)*RetentionYear, rp.Duration())
			}
		})
	}
}

func TestNewRetentionPeriodFromString(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantErr  bool
		expected int // expected years if no error
	}{
		{
			name:     "empty string",
			value:    "",
			wantErr:  true,
		},
		{
			name:     "minimum",
			value:    "minimum",
			wantErr:  false,
			expected: MinRetentionYears,
		},
		{
			name:     "standard",
			value:    "standard",
			wantErr:  false,
			expected: 7,
		},
		{
			name:     "extended",
			value:    "extended",
			wantErr:  false,
			expected: 10,
		},
		{
			name:     "permanent",
			value:    "permanent",
			wantErr:  false,
			expected: MaxRetentionYears,
		},
		{
			name:     "duration format",
			value:    "61320h", // 7 years in hours
			wantErr:  false,
			expected: 7,
		},
		{
			name:     "years with suffix",
			value:    "10years",
			wantErr:  false,
			expected: 10,
		},
		{
			name:     "years with y suffix",
			value:    "8y",
			wantErr:  false,
			expected: 8,
		},
		{
			name:     "invalid format",
			value:    "invalid",
			wantErr:  true,
		},
		{
			name:     "too short years",
			value:    "5years",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp, err := NewRetentionPeriodFromString(tt.value)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, rp.Years())
			}
		})
	}
}

func TestStandardRetentionPeriods(t *testing.T) {
	standard := StandardRetention()
	assert.Equal(t, 7, standard.Years())
	assert.True(t, standard.IsStandard())
	assert.False(t, standard.IsExtended())
	assert.False(t, standard.IsMinimum())

	extended := ExtendedRetention()
	assert.Equal(t, 10, extended.Years())
	assert.False(t, extended.IsStandard())
	assert.True(t, extended.IsExtended())
	assert.False(t, extended.IsMinimum())

	minimum := MinimumRetention()
	assert.Equal(t, MinRetentionYears, minimum.Years())
	assert.False(t, minimum.IsStandard())
	assert.False(t, minimum.IsExtended())
	assert.True(t, minimum.IsMinimum())
}

func TestRetentionPeriod_Equal(t *testing.T) {
	rp1 := MustNewRetentionPeriodFromYears(7)
	rp2 := MustNewRetentionPeriodFromYears(7)
	rp3 := MustNewRetentionPeriodFromYears(10)

	assert.True(t, rp1.Equal(rp2))
	assert.False(t, rp1.Equal(rp3))
	assert.True(t, rp1.Equal(rp1))
}

func TestRetentionPeriod_Compare(t *testing.T) {
	rp1 := MustNewRetentionPeriodFromYears(7)
	rp2 := MustNewRetentionPeriodFromYears(10)
	rp3 := MustNewRetentionPeriodFromYears(7)

	assert.Equal(t, -1, rp1.Compare(rp2))
	assert.Equal(t, 1, rp2.Compare(rp1))
	assert.Equal(t, 0, rp1.Compare(rp3))
}

func TestRetentionPeriod_ComparisonMethods(t *testing.T) {
	rp1 := MustNewRetentionPeriodFromYears(7)
	rp2 := MustNewRetentionPeriodFromYears(10)

	assert.True(t, rp1.LessThan(rp2))
	assert.False(t, rp2.LessThan(rp1))

	assert.False(t, rp1.GreaterThan(rp2))
	assert.True(t, rp2.GreaterThan(rp1))
}

func TestRetentionPeriod_String(t *testing.T) {
	rp1 := MustNewRetentionPeriodFromYears(1)
	rp7 := MustNewRetentionPeriodFromYears(7)

	assert.Equal(t, "1 year", rp1.String())
	assert.Equal(t, "7 years", rp7.String())
}

func TestRetentionPeriod_CalculateExpirationDate(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)
	createdAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	
	expiration := rp.CalculateExpirationDate(createdAt)
	expected := createdAt.Add(7 * RetentionYear)
	
	assert.Equal(t, expected, expiration)
}

func TestRetentionPeriod_IsExpired(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)
	
	// Data from 10 years ago should be expired
	oldData := time.Now().Add(-10 * RetentionYear)
	assert.True(t, rp.IsExpired(oldData))
	
	// Data from 5 years ago should not be expired
	recentData := time.Now().Add(-5 * RetentionYear)
	assert.False(t, rp.IsExpired(recentData))
	
	// Future data should not be expired
	futureData := time.Now().Add(time.Hour)
	assert.False(t, rp.IsExpired(futureData))
}

func TestRetentionPeriod_TimeUntilExpiration(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)
	
	// Data from 5 years ago should have ~2 years until expiration
	createdAt := time.Now().Add(-5 * RetentionYear)
	remaining := rp.TimeUntilExpiration(createdAt)
	
	// Should be approximately 2 years (allow some tolerance for test execution time)
	expectedRemaining := 2 * RetentionYear
	tolerance := time.Hour
	assert.InDelta(t, expectedRemaining, remaining, float64(tolerance))
	
	// Expired data should return 0
	expiredData := time.Now().Add(-10 * RetentionYear)
	remaining = rp.TimeUntilExpiration(expiredData)
	assert.Equal(t, time.Duration(0), remaining)
}

func TestRetentionPeriod_TimeSinceExpiration(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)
	
	// Data from 10 years ago should be ~3 years past expiration
	createdAt := time.Now().Add(-10 * RetentionYear)
	elapsed := rp.TimeSinceExpiration(createdAt)
	
	// Should be approximately 3 years
	expectedElapsed := 3 * RetentionYear
	tolerance := time.Hour
	assert.InDelta(t, expectedElapsed, elapsed, float64(tolerance))
	
	// Non-expired data should return 0
	recentData := time.Now().Add(-5 * RetentionYear)
	elapsed = rp.TimeSinceExpiration(recentData)
	assert.Equal(t, time.Duration(0), elapsed)
}

func TestRetentionPeriod_IsCompliant(t *testing.T) {
	rp7 := MustNewRetentionPeriodFromYears(7)
	rp5 := MustNewRetentionPeriodFromYears(5) // Below minimum for testing

	jurisdictions := []string{"US", "EU", "UK", "CA", "OTHER"}
	
	for _, jurisdiction := range jurisdictions {
		assert.True(t, rp7.IsCompliant(jurisdiction), "7 years should be compliant for %s", jurisdiction)
		assert.False(t, rp5.IsCompliant(jurisdiction), "5 years should not be compliant for %s", jurisdiction)
	}
}

func TestRetentionPeriod_GetComplianceNote(t *testing.T) {
	tests := []struct {
		years    int
		expected string
	}{
		{5, "Below minimum compliance requirements"},
		{7, "Meets minimum compliance requirements"},
		{8, "Meets standard compliance requirements"},
		{10, "Exceeds standard compliance requirements"},
		{15, "Exceeds standard compliance requirements"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_years", tt.years), func(t *testing.T) {
			// Create with a duration that bypasses validation for testing
			rp := RetentionPeriod{
				duration: time.Duration(tt.years) * RetentionYear,
				years:    tt.years,
			}
			assert.Equal(t, tt.expected, rp.GetComplianceNote())
		})
	}
}

func TestRetentionPeriod_Format(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)
	emptyRp := RetentionPeriod{}

	formatted := rp.Format()
	assert.Equal(t, "retention:7 years", formatted)

	formattedWithCompliance := rp.FormatWithCompliance()
	assert.Contains(t, formattedWithCompliance, "retention:7 years")
	assert.Contains(t, formattedWithCompliance, "Meets minimum compliance requirements")

	emptyFormatted := emptyRp.Format()
	assert.Equal(t, "<invalid>", emptyFormatted)
}

func TestRetentionPeriod_JSON(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)

	// Test marshaling
	data, err := json.Marshal(rp)
	require.NoError(t, err)

	// Verify JSON structure
	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	require.NoError(t, err)
	assert.Equal(t, float64(7), jsonData["years"])
	assert.Contains(t, jsonData["duration"], "h") // Duration string should contain hours

	// Test unmarshaling
	var unmarshaled RetentionPeriod
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, rp.Equal(unmarshaled))
}

func TestRetentionPeriod_JSONWithDuration(t *testing.T) {
	// Test unmarshaling with duration only
	jsonData := `{"duration":"61320h"}`
	
	var rp RetentionPeriod
	err := json.Unmarshal([]byte(jsonData), &rp)
	require.NoError(t, err)
	
	assert.Equal(t, 7, rp.Years())
}

func TestRetentionPeriod_JSONErrors(t *testing.T) {
	// Test missing both years and duration
	jsonData := `{}`
	
	var rp RetentionPeriod
	err := json.Unmarshal([]byte(jsonData), &rp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING_RETENTION_DATA")
}

func TestRetentionPeriod_Database(t *testing.T) {
	rp := MustNewRetentionPeriodFromYears(7)

	// Test Value
	value, err := rp.Value()
	require.NoError(t, err)
	assert.Equal(t, 7, value)

	// Test Scan with int64
	var scanned RetentionPeriod
	err = scanned.Scan(int64(7))
	require.NoError(t, err)
	assert.True(t, rp.Equal(scanned))

	// Test Scan with string
	var scannedString RetentionPeriod
	err = scannedString.Scan("7")
	require.NoError(t, err)
	assert.True(t, rp.Equal(scannedString))

	// Test Scan with nil
	var nilRp RetentionPeriod
	err = nilRp.Scan(nil)
	require.NoError(t, err)
	assert.True(t, nilRp.IsZero())
}

func TestRetentionPolicy(t *testing.T) {
	policy := NewRetentionPolicy()
	
	assert.True(t, policy.AuditEvents.IsStandard())
	assert.True(t, policy.CallRecords.IsStandard())
	assert.True(t, policy.BidData.IsStandard())
	assert.True(t, policy.FinancialData.IsExtended())
	assert.True(t, policy.PersonalData.IsStandard())

	// Test compliance
	assert.True(t, policy.IsCompliantWith("US"))
	assert.True(t, policy.IsCompliantWith("EU"))
	assert.True(t, policy.IsCompliantWith("UK"))
}

func TestValidateRetentionPeriod(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		wantErr  bool
	}{
		{
			name:     "valid duration",
			duration: 7 * RetentionYear,
			wantErr:  false,
		},
		{
			name:     "zero duration",
			duration: 0,
			wantErr:  true,
		},
		{
			name:     "too short duration",
			duration: 5 * RetentionYear,
			wantErr:  true,
		},
		{
			name:     "too long duration",
			duration: 150 * RetentionYear,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRetentionPeriod(tt.duration)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Property-based tests
func TestRetentionPeriod_Properties(t *testing.T) {
	// Property: Years conversion should be consistent
	t.Run("years_conversion_consistent", func(t *testing.T) {
		years := 8
		rp, err := NewRetentionPeriodFromYears(years)
		require.NoError(t, err)
		
		assert.Equal(t, years, rp.Years())
		assert.Equal(t, time.Duration(years)*RetentionYear, rp.Duration())
	})

	// Property: Expiration calculation should be symmetric
	t.Run("expiration_calculation_symmetric", func(t *testing.T) {
		rp := MustNewRetentionPeriodFromYears(7)
		now := time.Now()
		
		expiration := rp.CalculateExpirationDate(now)
		duration := expiration.Sub(now)
		
		// Duration should be approximately 7 years
		expectedDuration := 7 * RetentionYear
		tolerance := time.Hour
		assert.InDelta(t, expectedDuration, duration, float64(tolerance))
	})

	// Property: JSON marshaling/unmarshaling should preserve equality
	t.Run("json_roundtrip_preserves_equality", func(t *testing.T) {
		original := MustNewRetentionPeriodFromYears(10)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored RetentionPeriod
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Database value/scan roundtrip should preserve equality
	t.Run("database_roundtrip_preserves_equality", func(t *testing.T) {
		original := MustNewRetentionPeriodFromYears(8)

		value, err := original.Value()
		require.NoError(t, err)

		var restored RetentionPeriod
		err = restored.Scan(value)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Standard periods should meet compliance
	t.Run("standard_periods_are_compliant", func(t *testing.T) {
		periods := []RetentionPeriod{
			StandardRetention(),
			ExtendedRetention(),
			MinimumRetention(),
		}

		jurisdictions := []string{"US", "EU", "UK", "CA"}

		for _, period := range periods {
			for _, jurisdiction := range jurisdictions {
				assert.True(t, period.IsCompliant(jurisdiction),
					"Period %s should be compliant with %s", period.String(), jurisdiction)
			}
		}
	})
}

// Edge case tests
func TestRetentionPeriod_EdgeCases(t *testing.T) {
	// Test exactly at boundaries
	t.Run("minimum_boundary", func(t *testing.T) {
		rp, err := NewRetentionPeriodFromYears(MinRetentionYears)
		require.NoError(t, err)
		assert.True(t, rp.IsMinimum())
	})

	t.Run("maximum_boundary", func(t *testing.T) {
		rp, err := NewRetentionPeriodFromYears(MaxRetentionYears)
		require.NoError(t, err)
		assert.Equal(t, MaxRetentionYears, rp.Years())
	})

	// Test expiration edge cases
	t.Run("expiration_edge_cases", func(t *testing.T) {
		rp := MustNewRetentionPeriodFromYears(7)
		
		// Data created exactly at expiration time
		exactExpiration := time.Now().Add(-7 * RetentionYear)
		// This should be considered expired (>= expiration time)
		assert.True(t, rp.IsExpired(exactExpiration))
		
		// Data created just before expiration
		justBeforeExpiration := exactExpiration.Add(time.Second)
		assert.False(t, rp.IsExpired(justBeforeExpiration))
	})
}

// Benchmark tests
func BenchmarkNewRetentionPeriodFromYears(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewRetentionPeriodFromYears(7)
	}
}

func BenchmarkRetentionPeriod_IsExpired(b *testing.B) {
	rp := MustNewRetentionPeriodFromYears(7)
	createdAt := time.Now().Add(-5 * RetentionYear)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rp.IsExpired(createdAt)
	}
}

func BenchmarkRetentionPeriod_CalculateExpirationDate(b *testing.B) {
	rp := MustNewRetentionPeriodFromYears(7)
	createdAt := time.Now()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rp.CalculateExpirationDate(createdAt)
	}
}