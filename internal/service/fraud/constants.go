package fraud

import "time"

// Risk score thresholds
const (
	// RiskScoreCritical indicates maximum risk (immediate rejection)
	RiskScoreCritical = 1.0

	// RiskScoreHigh indicates high risk requiring review
	RiskScoreHigh = 0.8

	// RiskScoreMLAnomalyThreshold is the threshold for ML anomaly detection
	RiskScoreMLAnomalyThreshold = 0.7

	// RiskScoreMedium indicates medium risk
	RiskScoreMedium = 0.5

	// RiskScoreLow indicates low risk
	RiskScoreLow = 0.3

	// RiskScoreClean indicates minimal/no risk
	RiskScoreClean = 0.0
)

// Cache configuration
const (
	// RiskCacheExpiryDuration is how long risk scores remain cached
	RiskCacheExpiryDuration = 5 * time.Minute

	// RiskCacheCleanupInterval is how often to clean expired cache entries
	RiskCacheCleanupInterval = 10 * time.Minute
)

// Default rule values
const (
	// DefaultMaxBidAmount is the default maximum bid amount
	DefaultMaxBidAmount = 10000.0

	// DefaultMinAccountAge is the minimum account age in hours
	DefaultMinAccountAge = 24 * time.Hour

	// DefaultMaxVelocityCount is the default maximum velocity count
	DefaultMaxVelocityCount = 10

	// DefaultVelocityWindow is the default velocity check window
	DefaultVelocityWindow = time.Hour
)

// Confidence levels
const (
	// ConfidenceHigh indicates high confidence in the result
	ConfidenceHigh = 0.9

	// ConfidenceMedium indicates medium confidence in the result
	ConfidenceMedium = 0.7

	// ConfidenceLow indicates low confidence in the result
	ConfidenceLow = 0.5
)

// Feature extraction constants
const (
	// MinHistoryForPatternAnalysis is minimum number of past actions needed for pattern analysis
	MinHistoryForPatternAnalysis = 5

	// MaxHistoryLookbackDays is how many days back to look for historical patterns
	MaxHistoryLookbackDays = 30
)
