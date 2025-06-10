package values

import (
	"encoding/json"
	"testing"
)

func TestNewQualityMetrics(t *testing.T) {
	tests := []struct {
		name               string
		qualityScore       float64
		fraudScore         float64
		historicalRating   float64
		conversionRate     float64
		averageCallTime    int
		trustScore         float64
		reliabilityScore   float64
		expectError        bool
		expectedErrorMsg   string
	}{
		{
			name:               "valid metrics",
			qualityScore:       8.5,
			fraudScore:         1.2,
			historicalRating:   9.0,
			conversionRate:     0.75,
			averageCallTime:    180,
			trustScore:         8.0,
			reliabilityScore:   7.5,
			expectError:        false,
		},
		{
			name:               "default values",
			qualityScore:       5.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     0.0,
			averageCallTime:    0,
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        false,
		},
		{
			name:               "quality score too high",
			qualityScore:       11.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     0.5,
			averageCallTime:    120,
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        true,
			expectedErrorMsg:   "quality_score must be between 0.0 and 10.0",
		},
		{
			name:               "quality score too low",
			qualityScore:       -1.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     0.5,
			averageCallTime:    120,
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        true,
			expectedErrorMsg:   "quality_score must be between 0.0 and 10.0",
		},
		{
			name:               "conversion rate too high",
			qualityScore:       8.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     1.5,
			averageCallTime:    120,
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        true,
			expectedErrorMsg:   "conversion_rate must be between 0.0 and 1.0",
		},
		{
			name:               "negative average call time",
			qualityScore:       8.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     0.5,
			averageCallTime:    -60,
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        true,
			expectedErrorMsg:   "average_call_time cannot be negative",
		},
		{
			name:               "call time too long",
			qualityScore:       8.0,
			fraudScore:         0.0,
			historicalRating:   5.0,
			conversionRate:     0.5,
			averageCallTime:    90000, // > 24 hours
			trustScore:         5.0,
			reliabilityScore:   5.0,
			expectError:        true,
			expectedErrorMsg:   "average_call_time too long (max 24 hours)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewQualityMetrics(
				tt.qualityScore,
				tt.fraudScore,
				tt.historicalRating,
				tt.conversionRate,
				tt.averageCallTime,
				tt.trustScore,
				tt.reliabilityScore,
			)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if err.Error() != tt.expectedErrorMsg {
					t.Errorf("expected error message %q, got %q", tt.expectedErrorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.QualityScore != tt.qualityScore {
				t.Errorf("expected QualityScore %f, got %f", tt.qualityScore, metrics.QualityScore)
			}
			if metrics.FraudScore != tt.fraudScore {
				t.Errorf("expected FraudScore %f, got %f", tt.fraudScore, metrics.FraudScore)
			}
			if metrics.ConversionRate != tt.conversionRate {
				t.Errorf("expected ConversionRate %f, got %f", tt.conversionRate, metrics.ConversionRate)
			}
			if metrics.AverageCallTime != tt.averageCallTime {
				t.Errorf("expected AverageCallTime %d, got %d", tt.averageCallTime, metrics.AverageCallTime)
			}
		})
	}
}

func TestNewDefaultQualityMetrics(t *testing.T) {
	metrics := NewDefaultQualityMetrics()

	expectedDefaults := QualityMetrics{
		QualityScore:     5.0,
		FraudScore:       0.0,
		HistoricalRating: 5.0,
		ConversionRate:   0.0,
		AverageCallTime:  0,
		TrustScore:       5.0,
		ReliabilityScore: 5.0,
	}

	if !metrics.Equal(expectedDefaults) {
		t.Errorf("default metrics don't match expected values: %+v vs %+v", metrics, expectedDefaults)
	}
}

func TestQualityMetrics_OverallScore(t *testing.T) {
	tests := []struct {
		name           string
		metrics        QualityMetrics
		expectedRange  [2]float64 // min, max expected score
	}{
		{
			name: "high quality metrics",
			metrics: QualityMetrics{
				QualityScore:     9.0,
				FraudScore:       0.0,
				HistoricalRating: 9.0,
				ConversionRate:   0.8,
				TrustScore:       9.0,
				ReliabilityScore: 9.0,
			},
			expectedRange: [2]float64{7.0, 10.0},
		},
		{
			name: "low quality metrics",
			metrics: QualityMetrics{
				QualityScore:     2.0,
				FraudScore:       8.0,
				HistoricalRating: 2.0,
				ConversionRate:   0.1,
				TrustScore:       2.0,
				ReliabilityScore: 2.0,
			},
			expectedRange: [2]float64{0.0, 4.0},
		},
		{
			name:           "default metrics",
			metrics:        NewDefaultQualityMetrics(),
			expectedRange:  [2]float64{4.0, 6.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.metrics.OverallScore()

			if score < tt.expectedRange[0] || score > tt.expectedRange[1] {
				t.Errorf("overall score %f not in expected range [%f, %f]", 
					score, tt.expectedRange[0], tt.expectedRange[1])
			}

			// Score should always be between 0 and 10
			if score < 0.0 || score > 10.0 {
				t.Errorf("overall score %f is outside valid range [0.0, 10.0]", score)
			}
		})
	}
}

func TestQualityMetrics_IsHighQuality(t *testing.T) {
	highQualityMetrics := QualityMetrics{
		QualityScore:     9.0,
		FraudScore:       0.0,
		HistoricalRating: 8.0,
		ConversionRate:   0.7,
		TrustScore:       8.0,
		ReliabilityScore: 8.0,
	}

	lowQualityMetrics := QualityMetrics{
		QualityScore:     3.0,
		FraudScore:       7.0,
		HistoricalRating: 2.0,
		ConversionRate:   0.1,
		TrustScore:       2.0,
		ReliabilityScore: 2.0,
	}

	if !highQualityMetrics.IsHighQuality() {
		t.Error("high quality metrics should return true for IsHighQuality()")
	}

	if lowQualityMetrics.IsHighQuality() {
		t.Error("low quality metrics should return false for IsHighQuality()")
	}
}

func TestQualityMetrics_IsSuspicious(t *testing.T) {
	suspiciousMetrics := QualityMetrics{
		QualityScore:     5.0,
		FraudScore:       8.0, // High fraud score
		HistoricalRating: 5.0,
		ConversionRate:   0.5,
		TrustScore:       2.0, // Low trust score
		ReliabilityScore: 5.0,
	}

	cleanMetrics := QualityMetrics{
		QualityScore:     7.0,
		FraudScore:       1.0,
		HistoricalRating: 7.0,
		ConversionRate:   0.6,
		TrustScore:       8.0,
		ReliabilityScore: 7.0,
	}

	if !suspiciousMetrics.IsSuspicious() {
		t.Error("suspicious metrics should return true for IsSuspicious()")
	}

	if cleanMetrics.IsSuspicious() {
		t.Error("clean metrics should return false for IsSuspicious()")
	}
}

func TestQualityMetrics_UpdateMethods(t *testing.T) {
	original := NewDefaultQualityMetrics()

	// Test UpdateConversionRate
	updated, err := original.UpdateConversionRate(0.8)
	if err != nil {
		t.Errorf("unexpected error updating conversion rate: %v", err)
	}
	if updated.ConversionRate != 0.8 {
		t.Errorf("expected conversion rate 0.8, got %f", updated.ConversionRate)
	}
	if updated.QualityScore != original.QualityScore {
		t.Error("other fields should remain unchanged")
	}

	// Test invalid conversion rate
	_, err = original.UpdateConversionRate(1.5)
	if err == nil {
		t.Error("expected error for invalid conversion rate")
	}

	// Test UpdateAverageCallTime
	updated, err = original.UpdateAverageCallTime(300)
	if err != nil {
		t.Errorf("unexpected error updating average call time: %v", err)
	}
	if updated.AverageCallTime != 300 {
		t.Errorf("expected average call time 300, got %d", updated.AverageCallTime)
	}

	// Test invalid call time
	_, err = original.UpdateAverageCallTime(-100)
	if err == nil {
		t.Error("expected error for negative call time")
	}
}

func TestQualityMetrics_JSON(t *testing.T) {
	original := QualityMetrics{
		QualityScore:     8.5,
		FraudScore:       1.2,
		HistoricalRating: 7.8,
		ConversionRate:   0.65,
		AverageCallTime:  240,
		TrustScore:       8.0,
		ReliabilityScore: 7.5,
	}

	// Test marshaling
	data, err := json.Marshal(original)
	if err != nil {
		t.Errorf("error marshaling to JSON: %v", err)
	}

	// Test unmarshaling
	var restored QualityMetrics
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Errorf("error unmarshaling from JSON: %v", err)
	}

	if !original.Equal(restored) {
		t.Errorf("JSON round-trip failed: original %+v != restored %+v", original, restored)
	}

	// Test that overall score is included in JSON
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	if err != nil {
		t.Errorf("error parsing JSON: %v", err)
	}

	if _, exists := jsonMap["overall_score"]; !exists {
		t.Error("overall_score should be included in JSON output")
	}
}

func TestQualityMetrics_Equal(t *testing.T) {
	metrics1 := QualityMetrics{
		QualityScore:     8.0,
		FraudScore:       1.0,
		HistoricalRating: 7.0,
		ConversionRate:   0.6,
		AverageCallTime:  200,
		TrustScore:       8.0,
		ReliabilityScore: 7.0,
	}

	metrics2 := QualityMetrics{
		QualityScore:     8.0,
		FraudScore:       1.0,
		HistoricalRating: 7.0,
		ConversionRate:   0.6,
		AverageCallTime:  200,
		TrustScore:       8.0,
		ReliabilityScore: 7.0,
	}

	metrics3 := QualityMetrics{
		QualityScore:     7.0, // Different
		FraudScore:       1.0,
		HistoricalRating: 7.0,
		ConversionRate:   0.6,
		AverageCallTime:  200,
		TrustScore:       8.0,
		ReliabilityScore: 7.0,
	}

	if !metrics1.Equal(metrics2) {
		t.Error("identical metrics should be equal")
	}

	if metrics1.Equal(metrics3) {
		t.Error("different metrics should not be equal")
	}
}

func TestQualityMetrics_String(t *testing.T) {
	metrics := QualityMetrics{
		QualityScore:     8.0,
		FraudScore:       1.0,
		HistoricalRating: 7.0,
		ConversionRate:   0.6,
		AverageCallTime:  200,
		TrustScore:       8.0,
		ReliabilityScore: 7.0,
	}

	str := metrics.String()
	
	// Should contain the type name and key metrics
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}
	
	// Should contain formatted overall score
	overallScore := metrics.OverallScore()
	if !contains(str, "Overall") {
		t.Error("String() should contain overall score information")
	}
	
	t.Logf("String representation: %s (Overall: %.2f)", str, overallScore)
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr || 
		   (len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}