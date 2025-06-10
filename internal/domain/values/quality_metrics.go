package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// QualityMetrics represents comprehensive quality and performance metrics as a value object
type QualityMetrics struct {
	// Core Quality Scores (0.0 - 10.0 scale)
	QualityScore     float64 `json:"quality_score"`
	FraudScore       float64 `json:"fraud_score"`
	HistoricalRating float64 `json:"historical_rating"`
	
	// Performance Metrics
	ConversionRate   float64 `json:"conversion_rate"`   // 0.0 - 1.0 (percentage as decimal)
	AverageCallTime  int     `json:"average_call_time"` // seconds
	
	// Trust and Reliability
	TrustScore       float64 `json:"trust_score"`       // 0.0 - 10.0
	ReliabilityScore float64 `json:"reliability_score"` // 0.0 - 10.0
}

// NewQualityMetrics creates a new QualityMetrics value object with validation
func NewQualityMetrics(qualityScore, fraudScore, historicalRating, conversionRate float64, averageCallTime int, trustScore, reliabilityScore float64) (QualityMetrics, error) {
	// Validate quality scores (0.0 - 10.0)
	if err := validateScore(qualityScore, "quality_score", 0.0, 10.0); err != nil {
		return QualityMetrics{}, err
	}
	if err := validateScore(fraudScore, "fraud_score", 0.0, 10.0); err != nil {
		return QualityMetrics{}, err
	}
	if err := validateScore(historicalRating, "historical_rating", 0.0, 10.0); err != nil {
		return QualityMetrics{}, err
	}
	if err := validateScore(trustScore, "trust_score", 0.0, 10.0); err != nil {
		return QualityMetrics{}, err
	}
	if err := validateScore(reliabilityScore, "reliability_score", 0.0, 10.0); err != nil {
		return QualityMetrics{}, err
	}
	
	// Validate conversion rate (0.0 - 1.0)
	if err := validateScore(conversionRate, "conversion_rate", 0.0, 1.0); err != nil {
		return QualityMetrics{}, err
	}
	
	// Validate average call time
	if averageCallTime < 0 {
		return QualityMetrics{}, fmt.Errorf("average_call_time cannot be negative")
	}
	if averageCallTime > 86400 { // Max 24 hours
		return QualityMetrics{}, fmt.Errorf("average_call_time too long (max 24 hours)")
	}
	
	return QualityMetrics{
		QualityScore:     qualityScore,
		FraudScore:       fraudScore,
		HistoricalRating: historicalRating,
		ConversionRate:   conversionRate,
		AverageCallTime:  averageCallTime,
		TrustScore:       trustScore,
		ReliabilityScore: reliabilityScore,
	}, nil
}

// NewDefaultQualityMetrics creates quality metrics with safe default values
func NewDefaultQualityMetrics() QualityMetrics {
	return QualityMetrics{
		QualityScore:     5.0,  // Neutral score
		FraudScore:       0.0,  // No fraud detected
		HistoricalRating: 5.0,  // Neutral rating
		ConversionRate:   0.0,  // No conversions yet
		AverageCallTime:  0,    // No calls yet
		TrustScore:       5.0,  // Neutral trust
		ReliabilityScore: 5.0,  // Neutral reliability
	}
}

// MustNewQualityMetrics creates QualityMetrics and panics on error (for constants/tests)
func MustNewQualityMetrics(qualityScore, fraudScore, historicalRating, conversionRate float64, averageCallTime int, trustScore, reliabilityScore float64) QualityMetrics {
	metrics, err := NewQualityMetrics(qualityScore, fraudScore, historicalRating, conversionRate, averageCallTime, trustScore, reliabilityScore)
	if err != nil {
		panic(err)
	}
	return metrics
}

// OverallScore calculates a weighted overall quality score (0.0 - 10.0)
func (q QualityMetrics) OverallScore() float64 {
	// Weighted average of different metrics
	weights := map[string]float64{
		"quality":     0.25,
		"fraud":       -0.15, // Negative weight for fraud (higher fraud = lower overall)
		"historical":  0.20,
		"conversion":  0.15, // Convert 0-1 scale to 0-10 scale
		"trust":       0.20,
		"reliability": 0.15,
	}
	
	score := (q.QualityScore * weights["quality"]) +
		(q.FraudScore * weights["fraud"]) +
		(q.HistoricalRating * weights["historical"]) +
		(q.ConversionRate * 10.0 * weights["conversion"]) + // Convert to 0-10 scale
		(q.TrustScore * weights["trust"]) +
		(q.ReliabilityScore * weights["reliability"])
	
	// Ensure score stays within bounds
	if score < 0.0 {
		return 0.0
	}
	if score > 10.0 {
		return 10.0
	}
	
	return score
}

// IsHighQuality checks if metrics indicate high quality (overall score >= 7.0)
func (q QualityMetrics) IsHighQuality() bool {
	return q.OverallScore() >= 7.0
}

// IsLowQuality checks if metrics indicate low quality (overall score <= 3.0)
func (q QualityMetrics) IsLowQuality() bool {
	return q.OverallScore() <= 3.0
}

// IsSuspicious checks if metrics indicate suspicious activity
func (q QualityMetrics) IsSuspicious() bool {
	return q.FraudScore > 5.0 || q.TrustScore < 3.0
}

// HasSufficientData checks if enough data is available for reliable scoring
func (q QualityMetrics) HasSufficientData() bool {
	// Consider data sufficient if we have historical rating and some call data
	return q.HistoricalRating > 0.0 && q.AverageCallTime > 0
}

// UpdateConversionRate creates a new QualityMetrics with updated conversion rate
func (q QualityMetrics) UpdateConversionRate(newRate float64) (QualityMetrics, error) {
	if err := validateScore(newRate, "conversion_rate", 0.0, 1.0); err != nil {
		return QualityMetrics{}, err
	}
	
	return QualityMetrics{
		QualityScore:     q.QualityScore,
		FraudScore:       q.FraudScore,
		HistoricalRating: q.HistoricalRating,
		ConversionRate:   newRate,
		AverageCallTime:  q.AverageCallTime,
		TrustScore:       q.TrustScore,
		ReliabilityScore: q.ReliabilityScore,
	}, nil
}

// UpdateAverageCallTime creates a new QualityMetrics with updated average call time
func (q QualityMetrics) UpdateAverageCallTime(newTime int) (QualityMetrics, error) {
	if newTime < 0 {
		return QualityMetrics{}, fmt.Errorf("average_call_time cannot be negative")
	}
	if newTime > 86400 {
		return QualityMetrics{}, fmt.Errorf("average_call_time too long (max 24 hours)")
	}
	
	return QualityMetrics{
		QualityScore:     q.QualityScore,
		FraudScore:       q.FraudScore,
		HistoricalRating: q.HistoricalRating,
		ConversionRate:   q.ConversionRate,
		AverageCallTime:  newTime,
		TrustScore:       q.TrustScore,
		ReliabilityScore: q.ReliabilityScore,
	}, nil
}

// Equal checks if two QualityMetrics are equal
func (q QualityMetrics) Equal(other QualityMetrics) bool {
	return q.QualityScore == other.QualityScore &&
		q.FraudScore == other.FraudScore &&
		q.HistoricalRating == other.HistoricalRating &&
		q.ConversionRate == other.ConversionRate &&
		q.AverageCallTime == other.AverageCallTime &&
		q.TrustScore == other.TrustScore &&
		q.ReliabilityScore == other.ReliabilityScore
}

// String returns a string representation of the quality metrics
func (q QualityMetrics) String() string {
	return fmt.Sprintf("QualityMetrics{Overall: %.2f, Quality: %.2f, Fraud: %.2f, Trust: %.2f}", 
		q.OverallScore(), q.QualityScore, q.FraudScore, q.TrustScore)
}

// MarshalJSON implements JSON marshaling
func (q QualityMetrics) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		QualityScore     float64 `json:"quality_score"`
		FraudScore       float64 `json:"fraud_score"`
		HistoricalRating float64 `json:"historical_rating"`
		ConversionRate   float64 `json:"conversion_rate"`
		AverageCallTime  int     `json:"average_call_time"`
		TrustScore       float64 `json:"trust_score"`
		ReliabilityScore float64 `json:"reliability_score"`
		OverallScore     float64 `json:"overall_score"`
	}{
		QualityScore:     q.QualityScore,
		FraudScore:       q.FraudScore,
		HistoricalRating: q.HistoricalRating,
		ConversionRate:   q.ConversionRate,
		AverageCallTime:  q.AverageCallTime,
		TrustScore:       q.TrustScore,
		ReliabilityScore: q.ReliabilityScore,
		OverallScore:     q.OverallScore(),
	})
}

// UnmarshalJSON implements JSON unmarshaling
func (q *QualityMetrics) UnmarshalJSON(data []byte) error {
	var raw struct {
		QualityScore     float64 `json:"quality_score"`
		FraudScore       float64 `json:"fraud_score"`
		HistoricalRating float64 `json:"historical_rating"`
		ConversionRate   float64 `json:"conversion_rate"`
		AverageCallTime  int     `json:"average_call_time"`
		TrustScore       float64 `json:"trust_score"`
		ReliabilityScore float64 `json:"reliability_score"`
	}
	
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	metrics, err := NewQualityMetrics(
		raw.QualityScore,
		raw.FraudScore,
		raw.HistoricalRating,
		raw.ConversionRate,
		raw.AverageCallTime,
		raw.TrustScore,
		raw.ReliabilityScore,
	)
	if err != nil {
		return err
	}
	
	*q = metrics
	return nil
}

// Scan implements sql.Scanner for database scanning
func (q *QualityMetrics) Scan(value interface{}) error {
	if value == nil {
		*q = NewDefaultQualityMetrics()
		return nil
	}
	
	var jsonData []byte
	switch v := value.(type) {
	case []byte:
		jsonData = v
	case string:
		jsonData = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into QualityMetrics", value)
	}
	
	return q.UnmarshalJSON(jsonData)
}

// Value implements driver.Valuer for database storage
func (q QualityMetrics) Value() (driver.Value, error) {
	return q.MarshalJSON()
}

// Helper function to validate score ranges
func validateScore(score float64, fieldName string, min, max float64) error {
	if math.IsNaN(score) {
		return fmt.Errorf("%s cannot be NaN", fieldName)
	}
	if math.IsInf(score, 0) {
		return fmt.Errorf("%s cannot be infinite", fieldName)
	}
	if score < min || score > max {
		return fmt.Errorf("%s must be between %.1f and %.1f", fieldName, min, max)
	}
	return nil
}