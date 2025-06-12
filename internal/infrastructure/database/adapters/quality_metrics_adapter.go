package adapters

import (
	"database/sql/driver"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// QualityMetricsAdapter handles database conversion for QualityMetrics value objects
type QualityMetricsAdapter struct{}

// NewQualityMetricsAdapter creates a new quality metrics adapter
func NewQualityMetricsAdapter() *QualityMetricsAdapter {
	return &QualityMetricsAdapter{}
}

// Scan implements sql.Scanner for QualityMetrics value objects
// This method is called when reading from the database
func (a *QualityMetricsAdapter) Scan(dest *values.QualityMetrics, value interface{}) error {
	if value == nil {
		*dest = values.NewDefaultQualityMetrics()
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

	return dest.UnmarshalJSON(jsonData)
}

// Value implements driver.Valuer for QualityMetrics value objects
// This method is called when writing to the database
func (a *QualityMetricsAdapter) Value(src values.QualityMetrics) (driver.Value, error) {
	return src.MarshalJSON()
}

// ScanNullable handles nullable QualityMetrics fields
func (a *QualityMetricsAdapter) ScanNullable(dest **values.QualityMetrics, value interface{}) error {
	if value == nil {
		*dest = nil
		return nil
	}

	metrics := &values.QualityMetrics{}
	err := a.Scan(metrics, value)
	if err != nil {
		return err
	}

	*dest = metrics
	return nil
}

// ValueNullable handles nullable QualityMetrics fields
func (a *QualityMetricsAdapter) ValueNullable(src *values.QualityMetrics) (driver.Value, error) {
	if src == nil {
		return nil, nil
	}
	return a.Value(*src)
}

// ScanIndividualFields scans QualityMetrics from individual database columns
// Useful when quality metrics are stored as separate columns instead of JSON
func (a *QualityMetricsAdapter) ScanIndividualFields(dest *values.QualityMetrics,
	qualityScore, fraudScore, historicalRating, conversionRate float64,
	averageCallTime int, trustScore, reliabilityScore float64) error {

	metrics, err := values.NewQualityMetrics(
		qualityScore, fraudScore, historicalRating,
		conversionRate, averageCallTime,
		trustScore, reliabilityScore,
	)
	if err != nil {
		return fmt.Errorf("failed to create quality metrics from individual fields: %w", err)
	}

	*dest = metrics
	return nil
}

// ValueIndividualFields returns individual field values for QualityMetrics
// Useful when quality metrics are stored as separate columns instead of JSON
func (a *QualityMetricsAdapter) ValueIndividualFields(src values.QualityMetrics) (
	qualityScore, fraudScore, historicalRating, conversionRate float64,
	averageCallTime int, trustScore, reliabilityScore float64) {

	return src.QualityScore, src.FraudScore, src.HistoricalRating,
		src.ConversionRate, src.AverageCallTime,
		src.TrustScore, src.ReliabilityScore
}
