package fraud

import (
	"time"

	"github.com/google/uuid"
)

// FraudSeverity represents the severity level of fraud indicators
type FraudSeverity string

const (
	SeverityLow      FraudSeverity = "low"
	SeverityMedium   FraudSeverity = "medium"
	SeverityHigh     FraudSeverity = "high"
	SeverityCritical FraudSeverity = "critical"
)

// LogicalOperator represents logical operators for rule conditions
type LogicalOperator string

const (
	LogicalAND LogicalOperator = "AND"
	LogicalOR  LogicalOperator = "OR"
)

// ComparisonOperator represents comparison operators for rule conditions
type ComparisonOperator string

const (
	OpEqual         ComparisonOperator = "eq"
	OpNotEqual      ComparisonOperator = "ne"
	OpGreaterThan   ComparisonOperator = "gt"
	OpGreaterEqual  ComparisonOperator = "gte"
	OpLessThan      ComparisonOperator = "lt"
	OpLessEqual     ComparisonOperator = "lte"
	OpContains      ComparisonOperator = "contains"
	OpNotContains   ComparisonOperator = "not_contains"
	OpRegex         ComparisonOperator = "regex"
	OpIn            ComparisonOperator = "in"
	OpNotIn         ComparisonOperator = "not_in"
)

// CallFeatures represents features extracted from a call for fraud detection
type CallFeatures struct {
	Duration          time.Duration `json:"duration"`
	CallerReputation  float64       `json:"caller_reputation"`
	CalleeReputation  float64       `json:"callee_reputation"`
	TimeOfDay         int           `json:"time_of_day"`         // 0-23
	DayOfWeek         int           `json:"day_of_week"`         // 0-6 (Sunday=0)
	CallFrequency     int           `json:"call_frequency"`      // Calls in last hour
	GeographicRisk    float64       `json:"geographic_risk"`     // 0.0-1.0
	PriceDeviation    float64       `json:"price_deviation"`     // Deviation from average
	CallType          string        `json:"call_type"`           // "inbound", "outbound"
	SourceCountry     string        `json:"source_country"`      // ISO country code
	DestCountry       string        `json:"dest_country"`        // ISO country code
	CarrierReputation float64       `json:"carrier_reputation"`  // 0.0-1.0
	IsInternational   bool          `json:"is_international"`
	HasCLI            bool          `json:"has_cli"`             // Caller Line Identification
	CLIValidated      bool          `json:"cli_validated"`
}

// BidFeatures represents features extracted from a bid for fraud detection
type BidFeatures struct {
	BidAmount        float64       `json:"bid_amount"`
	BuyerReputation  float64       `json:"buyer_reputation"`
	BidFrequency     int           `json:"bid_frequency"`        // Bids in last hour
	TimeToSubmit     time.Duration `json:"time_to_submit"`       // Time from auction start
	PriceDeviation   float64       `json:"price_deviation"`      // Deviation from market rate
	HistoricalWins   int           `json:"historical_wins"`      // Wins in last 30 days
	WinRate          float64       `json:"win_rate"`             // Historical win percentage
	AverageMargin    float64       `json:"average_margin"`       // Typical profit margin
	AccountAge       time.Duration `json:"account_age"`          // Age of buyer account
	PaymentHistory   float64       `json:"payment_history"`      // Payment reliability score
	RegionMatch      bool          `json:"region_match"`         // Bid matches buyer region
	SkillsMatch      float64       `json:"skills_match"`         // Match with required skills
	VelocityScore    float64       `json:"velocity_score"`       // Recent bidding velocity
}

// AccountFeatures represents features extracted from an account for fraud detection
type AccountFeatures struct {
	AccountAge       time.Duration `json:"account_age"`
	TransactionCount int           `json:"transaction_count"`
	AverageAmount    float64       `json:"average_amount"`
	FailedPayments   int           `json:"failed_payments"`
	DisputeCount     int           `json:"dispute_count"`
	LoginFrequency   float64       `json:"login_frequency"`      // Logins per day
	DeviceCount      int           `json:"device_count"`         // Unique devices used
	LocationCount    int           `json:"location_count"`       // Unique locations
	OfficeHours      float64       `json:"office_hours"`         // % activity during business hours
	WeekendActivity  float64       `json:"weekend_activity"`     // % activity on weekends
	KYCStatus        string        `json:"kyc_status"`           // Know Your Customer status
	ComplianceScore  float64       `json:"compliance_score"`     // 0.0-1.0
}

// FraudEvidence represents structured evidence for fraud detection
type FraudEvidence struct {
	Type        string        `json:"type"`        // "velocity", "pattern", "geographic", "behavioral"
	Severity    FraudSeverity `json:"severity"`
	Description string        `json:"description"`
	Confidence  float64       `json:"confidence"`  // 0.0-1.0
	Timestamp   time.Time     `json:"timestamp"`
	Source      string        `json:"source"`      // "ml_model", "rule_engine", "manual", "external"
	Details     string        `json:"details"`     // Human-readable details
	EntityID    uuid.UUID     `json:"entity_id"`   // Related entity
	Value       string        `json:"value"`       // The suspicious value
	Threshold   string        `json:"threshold"`   // Expected threshold
}

// RuleConditionValue represents a typed value for rule conditions
type RuleConditionValue struct {
	StringValue  *string    `json:"string_value,omitempty"`
	NumberValue  *float64   `json:"number_value,omitempty"`
	BoolValue    *bool      `json:"bool_value,omitempty"`
	TimeValue    *time.Time `json:"time_value,omitempty"`
	DurationValue *time.Duration `json:"duration_value,omitempty"`
	StringArrayValue []string `json:"string_array_value,omitempty"`
	NumberArrayValue []float64 `json:"number_array_value,omitempty"`
}

// GetValue returns the actual value based on the type set
func (v RuleConditionValue) GetValue() interface{} {
	switch {
	case v.StringValue != nil:
		return *v.StringValue
	case v.NumberValue != nil:
		return *v.NumberValue
	case v.BoolValue != nil:
		return *v.BoolValue
	case v.TimeValue != nil:
		return *v.TimeValue
	case v.DurationValue != nil:
		return *v.DurationValue
	case v.StringArrayValue != nil:
		return v.StringArrayValue
	case v.NumberArrayValue != nil:
		return v.NumberArrayValue
	default:
		return nil
	}
}

// MLFeatures represents a union type for different feature types
type MLFeatures struct {
	Call    *CallFeatures    `json:"call,omitempty"`
	Bid     *BidFeatures     `json:"bid,omitempty"`
	Account *AccountFeatures `json:"account,omitempty"`
}

// GetFeatureMap converts typed features to map for backwards compatibility
func (f MLFeatures) GetFeatureMap() map[string]interface{} {
	features := make(map[string]interface{})
	
	if f.Call != nil {
		features["duration"] = f.Call.Duration.Seconds()
		features["caller_reputation"] = f.Call.CallerReputation
		features["callee_reputation"] = f.Call.CalleeReputation
		features["time_of_day"] = f.Call.TimeOfDay
		features["day_of_week"] = f.Call.DayOfWeek
		features["call_frequency"] = f.Call.CallFrequency
		features["geographic_risk"] = f.Call.GeographicRisk
		features["price_deviation"] = f.Call.PriceDeviation
		features["call_type"] = f.Call.CallType
		features["source_country"] = f.Call.SourceCountry
		features["dest_country"] = f.Call.DestCountry
		features["carrier_reputation"] = f.Call.CarrierReputation
		features["is_international"] = f.Call.IsInternational
		features["has_cli"] = f.Call.HasCLI
		features["cli_validated"] = f.Call.CLIValidated
	}
	
	if f.Bid != nil {
		features["bid_amount"] = f.Bid.BidAmount
		features["buyer_reputation"] = f.Bid.BuyerReputation
		features["bid_frequency"] = f.Bid.BidFrequency
		features["time_to_submit"] = f.Bid.TimeToSubmit.Seconds()
		features["price_deviation"] = f.Bid.PriceDeviation
		features["historical_wins"] = f.Bid.HistoricalWins
		features["win_rate"] = f.Bid.WinRate
		features["average_margin"] = f.Bid.AverageMargin
		features["account_age"] = f.Bid.AccountAge.Hours() / 24 // days
		features["payment_history"] = f.Bid.PaymentHistory
		features["region_match"] = f.Bid.RegionMatch
		features["skills_match"] = f.Bid.SkillsMatch
		features["velocity_score"] = f.Bid.VelocityScore
	}
	
	if f.Account != nil {
		features["account_age"] = f.Account.AccountAge.Hours() / 24 // days
		features["transaction_count"] = f.Account.TransactionCount
		features["average_amount"] = f.Account.AverageAmount
		features["failed_payments"] = f.Account.FailedPayments
		features["dispute_count"] = f.Account.DisputeCount
		features["login_frequency"] = f.Account.LoginFrequency
		features["device_count"] = f.Account.DeviceCount
		features["location_count"] = f.Account.LocationCount
		features["office_hours"] = f.Account.OfficeHours
		features["weekend_activity"] = f.Account.WeekendActivity
		features["kyc_status"] = f.Account.KYCStatus
		features["compliance_score"] = f.Account.ComplianceScore
	}
	
	return features
}

// RiskAttributes represents typed risk profile attributes
type RiskAttributes struct {
	LastFraudCheck   *time.Time `json:"last_fraud_check,omitempty"`
	FraudCategory    *string    `json:"fraud_category,omitempty"`
	MonitoringLevel  *string    `json:"monitoring_level,omitempty"`  // "none", "standard", "enhanced"
	WhitelistStatus  *bool      `json:"whitelist_status,omitempty"`
	ManualReview     *bool      `json:"manual_review,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	LastUpdate       *time.Time `json:"last_update,omitempty"`
	UpdatedBy        *uuid.UUID `json:"updated_by,omitempty"`
	TrustScore       *float64   `json:"trust_score,omitempty"`       // 0.0-1.0
	BusinessVerified *bool      `json:"business_verified,omitempty"`
	ComplianceFlags  []string   `json:"compliance_flags,omitempty"`
}

// GetAttributeMap converts typed attributes to map for backwards compatibility  
func (a RiskAttributes) GetAttributeMap() map[string]interface{} {
	attrs := make(map[string]interface{})
	
	if a.LastFraudCheck != nil {
		attrs["last_fraud_check"] = *a.LastFraudCheck
	}
	if a.FraudCategory != nil {
		attrs["fraud_category"] = *a.FraudCategory
	}
	if a.MonitoringLevel != nil {
		attrs["monitoring_level"] = *a.MonitoringLevel
	}
	if a.WhitelistStatus != nil {
		attrs["whitelist_status"] = *a.WhitelistStatus
	}
	if a.ManualReview != nil {
		attrs["manual_review"] = *a.ManualReview
	}
	if a.Notes != nil {
		attrs["notes"] = *a.Notes
	}
	if a.LastUpdate != nil {
		attrs["last_update"] = *a.LastUpdate
	}
	if a.UpdatedBy != nil {
		attrs["updated_by"] = *a.UpdatedBy
	}
	if a.TrustScore != nil {
		attrs["trust_score"] = *a.TrustScore
	}
	if a.BusinessVerified != nil {
		attrs["business_verified"] = *a.BusinessVerified
	}
	if a.ComplianceFlags != nil {
		attrs["compliance_flags"] = a.ComplianceFlags
	}
	
	return attrs
}

// FraudMetadata represents typed metadata for fraud check results
type FraudMetadata struct {
	ModelVersion    *string    `json:"model_version,omitempty"`
	RulesVersion    *string    `json:"rules_version,omitempty"`
	ProcessingTime  *time.Duration `json:"processing_time,omitempty"`
	DataSources     []string   `json:"data_sources,omitempty"`
	FeatureCount    *int       `json:"feature_count,omitempty"`
	RulesTriggered  []string   `json:"rules_triggered,omitempty"`
	MLConfidence    *float64   `json:"ml_confidence,omitempty"`
	RuleConfidence  *float64   `json:"rule_confidence,omitempty"`
	CheckMethod     *string    `json:"check_method,omitempty"` // "realtime", "batch", "manual"
	RequestID       *string    `json:"request_id,omitempty"`
	SessionID       *string    `json:"session_id,omitempty"`
}

// GetMetadataMap converts typed metadata to map for backwards compatibility
func (m FraudMetadata) GetMetadataMap() map[string]interface{} {
	metadata := make(map[string]interface{})
	
	if m.ModelVersion != nil {
		metadata["model_version"] = *m.ModelVersion
	}
	if m.RulesVersion != nil {
		metadata["rules_version"] = *m.RulesVersion
	}
	if m.ProcessingTime != nil {
		metadata["processing_time"] = *m.ProcessingTime
	}
	if m.DataSources != nil {
		metadata["data_sources"] = m.DataSources
	}
	if m.FeatureCount != nil {
		metadata["feature_count"] = *m.FeatureCount
	}
	if m.RulesTriggered != nil {
		metadata["rules_triggered"] = m.RulesTriggered
	}
	if m.MLConfidence != nil {
		metadata["ml_confidence"] = *m.MLConfidence
	}
	if m.RuleConfidence != nil {
		metadata["rule_confidence"] = *m.RuleConfidence
	}
	if m.CheckMethod != nil {
		metadata["check_method"] = *m.CheckMethod
	}
	if m.RequestID != nil {
		metadata["request_id"] = *m.RequestID
	}
	if m.SessionID != nil {
		metadata["session_id"] = *m.SessionID
	}
	
	return metadata
}