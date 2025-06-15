package rest

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Common response types with HATEOAS support

// PaginationResponse provides pagination metadata
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
	TotalItems int `json:"total_items"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// Links implements HATEOAS links
func (p PaginationResponse) Links(baseURL string, params map[string]string) map[string]string {
	links := make(map[string]string)
	
	// Self link
	links["self"] = buildURL(baseURL, params)
	
	// Navigation links
	if p.HasNext {
		nextParams := copyMap(params)
		nextParams["page"] = fmt.Sprintf("%d", p.Page+1)
		links["next"] = buildURL(baseURL, nextParams)
	}
	
	if p.HasPrev {
		prevParams := copyMap(params)
		prevParams["page"] = fmt.Sprintf("%d", p.Page-1)
		links["prev"] = buildURL(baseURL, prevParams)
	}
	
	// First and last
	firstParams := copyMap(params)
	firstParams["page"] = "1"
	links["first"] = buildURL(baseURL, firstParams)
	
	lastParams := copyMap(params)
	lastParams["page"] = fmt.Sprintf("%d", p.TotalPages)
	links["last"] = buildURL(baseURL, lastParams)
	
	return links
}

// ListResponse wraps paginated list responses
type ListResponse[T any] struct {
	Items      []T                `json:"items"`
	Pagination PaginationResponse `json:"pagination"`
	Links      map[string]string  `json:"_links,omitempty"`
}

// Call responses

// CallResponse represents a call in API responses
type CallResponse struct {
	ID          uuid.UUID              `json:"id"`
	FromNumber  string                 `json:"from_number"`
	ToNumber    string                 `json:"to_number"`
	Direction   string                 `json:"direction"`
	Status      string                 `json:"status"`
	Duration    *int                   `json:"duration,omitempty"`
	Cost        *MoneyResponse         `json:"cost,omitempty"`
	QualityScore *float64              `json:"quality_score,omitempty"`
	RouteID     *uuid.UUID             `json:"route_id,omitempty"`
	BuyerID     uuid.UUID              `json:"buyer_id"`
	SellerID    *uuid.UUID             `json:"seller_id,omitempty"`
	StartTime   *time.Time             `json:"start_time,omitempty"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Links provides HATEOAS links for CallResponse
func (c CallResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self":   fmt.Sprintf("%s/api/v1/calls/%s", baseURL, c.ID),
		"buyer":  fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, c.BuyerID),
	}
	
	if c.SellerID != nil {
		links["seller"] = fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, *c.SellerID)
	}
	
	if c.RouteID != nil {
		links["route"] = fmt.Sprintf("%s/api/v1/routes/%s", baseURL, *c.RouteID)
	}
	
	// Action links based on status
	switch c.Status {
	case "pending":
		links["route"] = fmt.Sprintf("%s/api/v1/calls/%s/route", baseURL, c.ID)
	case "queued", "ringing":
		links["cancel"] = fmt.Sprintf("%s/api/v1/calls/%s/cancel", baseURL, c.ID)
	case "in_progress":
		links["complete"] = fmt.Sprintf("%s/api/v1/calls/%s/complete", baseURL, c.ID)
	}
	
	return links
}

// CallMetricsResponse provides call metrics
type CallMetricsResponse struct {
	TotalCalls      int                    `json:"total_calls"`
	CompletedCalls  int                    `json:"completed_calls"`
	FailedCalls     int                    `json:"failed_calls"`
	AverageDuration float64                `json:"average_duration"`
	TotalCost       MoneyResponse          `json:"total_cost"`
	ConversionRate  float64                `json:"conversion_rate"`
	QualityScore    float64                `json:"average_quality_score"`
	ByStatus        map[string]int         `json:"by_status"`
	ByHour          map[string]int         `json:"by_hour,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// Bid responses

// BidResponse represents a bid in API responses
type BidResponse struct {
	ID         uuid.UUID              `json:"id"`
	CallID     uuid.UUID              `json:"call_id"`
	BuyerID    uuid.UUID              `json:"buyer_id"`
	AuctionID  uuid.UUID              `json:"auction_id"`
	Amount     MoneyResponse          `json:"amount"`
	Status     string                 `json:"status"`
	PlacedAt   time.Time              `json:"placed_at"`
	ExpiresAt  time.Time              `json:"expires_at"`
	WonAt      *time.Time             `json:"won_at,omitempty"`
	Criteria   map[string]interface{} `json:"criteria,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Links provides HATEOAS links for BidResponse
func (b BidResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self":    fmt.Sprintf("%s/api/v1/bids/%s", baseURL, b.ID),
		"call":    fmt.Sprintf("%s/api/v1/calls/%s", baseURL, b.CallID),
		"buyer":   fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, b.BuyerID),
		"auction": fmt.Sprintf("%s/api/v1/auctions/%s", baseURL, b.AuctionID),
	}
	
	// Action links based on status
	if b.Status == "active" {
		links["cancel"] = fmt.Sprintf("%s/api/v1/bids/%s/cancel", baseURL, b.ID)
		links["update"] = fmt.Sprintf("%s/api/v1/bids/%s", baseURL, b.ID)
	}
	
	return links
}

// BidProfileResponse represents a bid profile
type BidProfileResponse struct {
	ID          uuid.UUID              `json:"id"`
	SellerID    uuid.UUID              `json:"seller_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Active      bool                   `json:"active"`
	Criteria    BidCriteriaResponse    `json:"criteria"`
	Stats       BidProfileStats        `json:"stats"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// BidCriteriaResponse represents bid criteria
type BidCriteriaResponse struct {
	MinPrice         MoneyResponse    `json:"min_price"`
	MaxPrice         MoneyResponse    `json:"max_price"`
	TargetStates     []string         `json:"target_states,omitempty"`
	TargetAreaCodes  []string         `json:"target_area_codes,omitempty"`
	TimeRestrictions *TimeRangeResponse `json:"time_restrictions,omitempty"`
	QualityThreshold float64          `json:"quality_threshold"`
}

// TimeRangeResponse for time restrictions
type TimeRangeResponse struct {
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	Timezone   string   `json:"timezone"`
	DaysOfWeek []string `json:"days_of_week,omitempty"`
}

// BidProfileStats provides profile statistics
type BidProfileStats struct {
	TotalBids    int           `json:"total_bids"`
	WonBids      int           `json:"won_bids"`
	WinRate      float64       `json:"win_rate"`
	AverageBid   MoneyResponse `json:"average_bid"`
	TotalSpent   MoneyResponse `json:"total_spent"`
	LastBidAt    *time.Time    `json:"last_bid_at,omitempty"`
}

// Auction responses

// AuctionResponse represents an auction
type AuctionResponse struct {
	ID           uuid.UUID              `json:"id"`
	CallID       uuid.UUID              `json:"call_id"`
	Status       string                 `json:"status"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	ReservePrice MoneyResponse          `json:"reserve_price"`
	CurrentBid   *MoneyResponse         `json:"current_bid,omitempty"`
	BidCount     int                    `json:"bid_count"`
	WinningBid   *uuid.UUID             `json:"winning_bid,omitempty"`
	Criteria     map[string]interface{} `json:"criteria,omitempty"`
}

// Links provides HATEOAS links for AuctionResponse
func (a AuctionResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/auctions/%s", baseURL, a.ID),
		"call": fmt.Sprintf("%s/api/v1/calls/%s", baseURL, a.CallID),
		"bids": fmt.Sprintf("%s/api/v1/auctions/%s/bids", baseURL, a.ID),
	}
	
	// Action links based on status
	switch a.Status {
	case "active":
		links["bid"] = fmt.Sprintf("%s/api/v1/auctions/%s/bid", baseURL, a.ID)
		links["close"] = fmt.Sprintf("%s/api/v1/auctions/%s/close", baseURL, a.ID)
	case "closed":
		if a.WinningBid != nil {
			links["winner"] = fmt.Sprintf("%s/api/v1/bids/%s", baseURL, *a.WinningBid)
		}
	}
	
	return links
}

// Account responses

// AccountResponse represents an account
type AccountResponse struct {
	ID           uuid.UUID               `json:"id"`
	Type         string                  `json:"type"`
	Email        string                  `json:"email"`
	CompanyName  string                  `json:"company_name"`
	ContactName  string                  `json:"contact_name,omitempty"`
	PhoneNumber  string                  `json:"phone_number"`
	Status       string                  `json:"status"`
	Balance      MoneyResponse           `json:"balance"`
	CreditLimit  *MoneyResponse          `json:"credit_limit,omitempty"`
	QualityScore float64                 `json:"quality_score"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	Settings     map[string]interface{}  `json:"settings,omitempty"`
	Stats        AccountStatsResponse    `json:"stats"`
}

// AccountStatsResponse provides account statistics
type AccountStatsResponse struct {
	TotalCalls      int           `json:"total_calls"`
	CompletedCalls  int           `json:"completed_calls"`
	TotalSpent      MoneyResponse `json:"total_spent"`
	TotalEarned     MoneyResponse `json:"total_earned"`
	AverageCallTime float64       `json:"average_call_time"`
	LastActivityAt  *time.Time    `json:"last_activity_at,omitempty"`
}

// Links provides HATEOAS links for AccountResponse
func (a AccountResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self":         fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, a.ID),
		"transactions": fmt.Sprintf("%s/api/v1/accounts/%s/transactions", baseURL, a.ID),
		"settings":     fmt.Sprintf("%s/api/v1/accounts/%s/settings", baseURL, a.ID),
	}
	
	if a.Type == "buyer" {
		links["calls"] = fmt.Sprintf("%s/api/v1/accounts/%s/calls", baseURL, a.ID)
	} else {
		links["bid_profiles"] = fmt.Sprintf("%s/api/v1/accounts/%s/bid-profiles", baseURL, a.ID)
		links["bids"] = fmt.Sprintf("%s/api/v1/accounts/%s/bids", baseURL, a.ID)
	}
	
	return links
}

// Authentication responses

// AuthResponse for login/register responses
type AuthResponse struct {
	Token        string           `json:"token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int              `json:"expires_in"`
	TokenType    string           `json:"token_type"`
	User         UserResponse     `json:"user"`
	Permissions  []string         `json:"permissions"`
	Settings     map[string]interface{} `json:"settings,omitempty"`
}

// UserResponse represents authenticated user info
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	AccountID   uuid.UUID `json:"account_id"`
	AccountType string    `json:"account_type"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	MFAEnabled  bool      `json:"mfa_enabled"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// Financial responses

// TransactionResponse represents a financial transaction
type TransactionResponse struct {
	ID              uuid.UUID              `json:"id"`
	AccountID       uuid.UUID              `json:"account_id"`
	Type            string                 `json:"type"`
	Amount          MoneyResponse          `json:"amount"`
	Balance         MoneyResponse          `json:"balance"`
	Description     string                 `json:"description"`
	ReferenceType   string                 `json:"reference_type,omitempty"`
	ReferenceID     *uuid.UUID             `json:"reference_id,omitempty"`
	Status          string                 `json:"status"`
	ProcessedAt     time.Time              `json:"processed_at"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// MoneyResponse represents monetary values
type MoneyResponse struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Display  string  `json:"display"`
}

// Compliance responses

// ComplianceCheckResponse for compliance validations
type ComplianceCheckResponse struct {
	Compliant    bool                   `json:"compliant"`
	Violations   []ViolationResponse    `json:"violations,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	CheckedAt    time.Time              `json:"checked_at"`
	NextCheckAt  *time.Time             `json:"next_check_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ViolationResponse represents a compliance violation
type ViolationResponse struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Rule        string    `json:"rule"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// WebSocket message types

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      string                 `json:"type"`
	Event     string                 `json:"event"`
	Data      interface{}            `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// RealtimeUpdate for real-time data updates
type RealtimeUpdate struct {
	Resource   string      `json:"resource"`
	Action     string      `json:"action"`
	ResourceID uuid.UUID   `json:"resource_id"`
	Data       interface{} `json:"data"`
	Version    int         `json:"version"`
}

// Analytics responses

// MetricsResponse for analytics data
type MetricsResponse struct {
	Period     string                 `json:"period"`
	StartDate  time.Time              `json:"start_date"`
	EndDate    time.Time              `json:"end_date"`
	Metrics    map[string]interface{} `json:"metrics"`
	Breakdown  []BreakdownResponse    `json:"breakdown,omitempty"`
	Comparison *ComparisonResponse    `json:"comparison,omitempty"`
}

// BreakdownResponse for metric breakdowns
type BreakdownResponse struct {
	Label   string                 `json:"label"`
	Value   interface{}            `json:"value"`
	Percent float64                `json:"percent,omitempty"`
	Trend   string                 `json:"trend,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ComparisonResponse for period comparisons
type ComparisonResponse struct {
	PreviousPeriod map[string]interface{} `json:"previous_period"`
	Change         map[string]float64     `json:"change"`
	PercentChange  map[string]float64     `json:"percent_change"`
}

// Batch operation responses

// BatchOperationResponse for bulk operations
type BatchOperationResponse struct {
	ID          uuid.UUID              `json:"id"`
	Operation   string                 `json:"operation"`
	Resource    string                 `json:"resource"`
	TotalItems  int                    `json:"total_items"`
	Succeeded   int                    `json:"succeeded"`
	Failed      int                    `json:"failed"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Results     []BatchResultResponse  `json:"results,omitempty"`
	Errors      []BatchErrorResponse   `json:"errors,omitempty"`
}

// BatchResultResponse for individual batch results
type BatchResultResponse struct {
	Index      int         `json:"index"`
	ResourceID *uuid.UUID  `json:"resource_id,omitempty"`
	Status     string      `json:"status"`
	Data       interface{} `json:"data,omitempty"`
}

// BatchErrorResponse for batch errors
type BatchErrorResponse struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Helper functions

func buildURL(base string, params map[string]string) string {
	if len(params) == 0 {
		return base
	}
	
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	
	return fmt.Sprintf("%s?%s", base, strings.Join(parts, "&"))
}

func copyMap(m map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Authentication response types for tests

// LoginResponse for login responses
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// UserProfile for user profile responses
type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	CompanyName string    `json:"company_name"`
	AccountType string    `json:"account_type"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
}

// AccountBalance for account balance responses
type AccountBalance struct {
	AccountID        uuid.UUID     `json:"account_id"`
	AvailableBalance MoneyResponse `json:"available_balance"`
	PendingBalance   MoneyResponse `json:"pending_balance"`
	Currency         string        `json:"currency"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// DNCCheckResponse for DNC check responses
type DNCCheckResponse struct {
	PhoneNumber string `json:"phone_number"`
	IsDNC       bool   `json:"is_dnc"`
	Message     string `json:"message"`
}

// Audit response types

// AuditEventResponse represents an audit event in API responses
type AuditEventResponse struct {
	ID         uuid.UUID              `json:"id"`
	EventType  string                 `json:"event_type"`
	Actor      string                 `json:"actor"`
	Resource   string                 `json:"resource"`
	Action     string                 `json:"action"`
	Outcome    string                 `json:"outcome"`
	Severity   string                 `json:"severity"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Links provides HATEOAS links for AuditEventResponse
func (a AuditEventResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/audit/events/%s", baseURL, a.ID),
	}
	
	// Add related resource links if available
	if a.Resource != "" {
		links["resource"] = fmt.Sprintf("%s/api/v1/%s", baseURL, a.Resource)
	}
	
	// Add actor link if it's a user ID
	if _, err := uuid.Parse(a.Actor); err == nil {
		links["actor"] = fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, a.Actor)
	}
	
	return links
}

// AuditEventListResponse for paginated audit event lists
type AuditEventListResponse struct {
	Events     []AuditEventResponse   `json:"events"`
	Pagination PaginationResponse     `json:"pagination"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AuditSearchResponse for advanced search results
type AuditSearchResponse struct {
	Results    []AuditEventResponse   `json:"results"`
	Pagination PaginationResponse     `json:"pagination"`
	Facets     map[string]interface{} `json:"facets,omitempty"`
	Highlights map[string]interface{} `json:"highlights,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AuditStatsResponse for audit statistics
type AuditStatsResponse struct {
	TimeRange    interface{}            `json:"time_range"`
	TotalEvents  int64                  `json:"total_events"`
	EventsByType map[string]int64       `json:"events_by_type"`
	Timeline     []TimelinePoint        `json:"timeline,omitempty"`
	TopActors    []ActorStats           `json:"top_actors,omitempty"`
	ErrorRate    float64                `json:"error_rate"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TimelinePoint represents a point in the event timeline
type TimelinePoint struct {
	Time        time.Time `json:"time"`
	EventCount  int64     `json:"event_count"`
	SuccessRate float64   `json:"success_rate"`
}

// ActorStats provides statistics for an actor
type ActorStats struct {
	Actor       string  `json:"actor"`
	EventCount  int64   `json:"event_count"`
	SuccessRate float64 `json:"success_rate"`
	LastSeen    time.Time `json:"last_seen"`
}

// AuditExportResponse for export operations
type AuditExportResponse struct {
	ExportID    uuid.UUID              `json:"export_id"`
	ReportType  string                 `json:"report_type"`
	Status      string                 `json:"status"`
	Format      string                 `json:"format"`
	Size        int64                  `json:"size,omitempty"`
	RecordCount int64                  `json:"record_count,omitempty"`
	GeneratedAt time.Time              `json:"generated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	DownloadURL string                 `json:"download_url,omitempty"`
	Checksum    string                 `json:"checksum,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Links provides HATEOAS links for AuditExportResponse
func (a AuditExportResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/audit/export/%s", baseURL, a.ExportID),
	}
	
	if a.DownloadURL != "" {
		links["download"] = a.DownloadURL
	}
	
	if a.Status == "completed" && a.DownloadURL != "" {
		links["verify"] = fmt.Sprintf("%s/api/v1/audit/export/%s/verify", baseURL, a.ExportID)
	}
	
	return links
}

// AuditStreamResponse for streaming operations
type AuditStreamResponse struct {
	StreamID  uuid.UUID              `json:"stream_id"`
	Status    string                 `json:"status"`
	ChunkSize int                    `json:"chunk_size"`
	StartedAt time.Time              `json:"started_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamingExportResponse for streaming export operations
type StreamingExportResponse struct {
	StreamID        uuid.UUID              `json:"stream_id"`
	Status          string                 `json:"status"`
	EstimatedTotal  int64                  `json:"estimated_total,omitempty"`
	ChunkSize       int                    `json:"chunk_size"`
	StartedAt       time.Time              `json:"started_at"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// GDPRComplianceResponse for GDPR compliance reports
type GDPRComplianceResponse struct {
	SubjectID       string                 `json:"subject_id"`
	ReportID        uuid.UUID              `json:"report_id"`
	GeneratedAt     time.Time              `json:"generated_at"`
	DataPoints      []GDPRDataPoint        `json:"data_points"`
	ProcessingBases []string               `json:"processing_bases"`
	RetentionPolicy map[string]interface{} `json:"retention_policy"`
	RightsExercised []GDPRRightExercised   `json:"rights_exercised"`
	ConsentHistory  []GDPRConsentRecord    `json:"consent_history"`
	DataTransfers   []GDPRDataTransfer     `json:"data_transfers"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// GDPRDataPoint represents a data point for GDPR compliance
type GDPRDataPoint struct {
	Category        string    `json:"category"`
	DataType        string    `json:"data_type"`
	ProcessingBasis string    `json:"processing_basis"`
	CollectedAt     time.Time `json:"collected_at"`
	RetentionPeriod string    `json:"retention_period"`
	Source          string    `json:"source"`
}

// GDPRRightExercised represents an exercised GDPR right
type GDPRRightExercised struct {
	Right       string    `json:"right"` // access, rectification, erasure, etc.
	RequestedAt time.Time `json:"requested_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Status      string    `json:"status"`
}

// GDPRConsentRecord represents a consent record
type GDPRConsentRecord struct {
	ConsentType string    `json:"consent_type"`
	Granted     bool      `json:"granted"`
	Timestamp   time.Time `json:"timestamp"`
	Method      string    `json:"method"`
	Version     string    `json:"version,omitempty"`
}

// GDPRDataTransfer represents a data transfer record
type GDPRDataTransfer struct {
	Destination     string    `json:"destination"`
	LegalBasis      string    `json:"legal_basis"`
	TransferredAt   time.Time `json:"transferred_at"`
	DataCategories  []string  `json:"data_categories"`
	Safeguards      []string  `json:"safeguards,omitempty"`
}

// TCPAComplianceResponse for TCPA compliance reports
type TCPAComplianceResponse struct {
	PhoneNumber       string                 `json:"phone_number"`
	ReportID          uuid.UUID              `json:"report_id"`
	GeneratedAt       time.Time              `json:"generated_at"`
	ConsentStatus     TCPAConsentStatus      `json:"consent_status"`
	CallHistory       []TCPACallRecord       `json:"call_history"`
	ViolationHistory  []TCPAViolation        `json:"violation_history"`
	OptOutHistory     []TCPAOptOutRecord     `json:"opt_out_history"`
	CallingTimeChecks []TCPATimeCheck        `json:"calling_time_checks"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// TCPAConsentStatus represents current TCPA consent status
type TCPAConsentStatus struct {
	HasConsent     bool       `json:"has_consent"`
	ConsentType    string     `json:"consent_type"` // express, implied
	ConsentDate    *time.Time `json:"consent_date,omitempty"`
	ConsentMethod  string     `json:"consent_method,omitempty"`
	IsOptedOut     bool       `json:"is_opted_out"`
	OptOutDate     *time.Time `json:"opt_out_date,omitempty"`
	LastVerified   time.Time  `json:"last_verified"`
}

// TCPACallRecord represents a TCPA call record
type TCPACallRecord struct {
	CallID        uuid.UUID `json:"call_id"`
	CalledAt      time.Time `json:"called_at"`
	Duration      int       `json:"duration"` // in seconds
	CallType      string    `json:"call_type"` // marketing, transactional, etc.
	ConsentValid  bool      `json:"consent_valid"`
	TimeCompliant bool      `json:"time_compliant"`
	Outcome       string    `json:"outcome"`
}

// TCPAViolation represents a TCPA violation
type TCPAViolation struct {
	ViolationType string    `json:"violation_type"`
	OccurredAt    time.Time `json:"occurred_at"`
	CallID        *uuid.UUID `json:"call_id,omitempty"`
	Description   string    `json:"description"`
	Severity      string    `json:"severity"`
	Resolved      bool      `json:"resolved"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

// TCPAOptOutRecord represents an opt-out record
type TCPAOptOutRecord struct {
	OptedOutAt time.Time `json:"opted_out_at"`
	Method     string    `json:"method"` // call, text, web, etc.
	Source     string    `json:"source"`
	CallID     *uuid.UUID `json:"call_id,omitempty"`
}

// TCPATimeCheck represents a calling time compliance check
type TCPATimeCheck struct {
	CheckedAt     time.Time `json:"checked_at"`
	TimeZone      string    `json:"time_zone"`
	LocalTime     time.Time `json:"local_time"`
	IsPermitted   bool      `json:"is_permitted"`
	RestrictedBy  string    `json:"restricted_by,omitempty"` // federal, state, etc.
}

// IntegrityCheckResponse for integrity check results
type IntegrityCheckResponse struct {
	CheckID         uuid.UUID              `json:"check_id"`
	CheckType       string                 `json:"check_type"`
	Status          string                 `json:"status"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	EventsChecked   int64                  `json:"events_checked"`
	IssuesFound     int64                  `json:"issues_found"`
	IntegrityScore  float64                `json:"integrity_score"`
	Issues          []IntegrityIssue       `json:"issues,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// IntegrityIssue represents an integrity issue
type IntegrityIssue struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	EventID     *uuid.UUID             `json:"event_id,omitempty"`
	DetectedAt  time.Time              `json:"detected_at"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// DNC Integration response types

// DNCCheckResponse represents a DNC check result with compliance details
type DNCCheckResponse struct {
	PhoneNumber     string                 `json:"phone_number"`
	IsBlocked       bool                   `json:"is_blocked"`
	CanCall         bool                   `json:"can_call"`
	CheckedAt       time.Time              `json:"checked_at"`
	ComplianceLevel string                 `json:"compliance_level"`
	RiskScore       float64                `json:"risk_score"`
	TTLSeconds      int                    `json:"ttl_seconds"`
	BlockingReasons []DNCBlockReasonResponse `json:"blocking_reasons,omitempty"`
	SourcesChecked  []string               `json:"sources_checked"`
	SourcesCount    int                    `json:"sources_count"`
	CheckDuration   string                 `json:"check_duration"`
	Recommendation  string                 `json:"recommendation"`
	ComplianceCodes []string               `json:"compliance_codes,omitempty"`
	HighestSeverity string                 `json:"highest_severity"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// DNCBlockReasonResponse represents a reason why a number is blocked
type DNCBlockReasonResponse struct {
	Source          string     `json:"source"`
	Reason          string     `json:"reason"`
	Description     string     `json:"description"`
	Provider        string     `json:"provider"`
	ProviderID      uuid.UUID  `json:"provider_id"`
	Severity        string     `json:"severity"`
	ComplianceCode  string     `json:"compliance_code"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	IsPermanent     bool       `json:"is_permanent"`
	IsRegulatory    bool       `json:"is_regulatory"`
}

// DNCEntryResponse represents a DNC entry with privacy protection
type DNCEntryResponse struct {
	ID              uuid.UUID              `json:"id"`
	PhoneNumber     string                 `json:"phone_number"` // Masked in responses if privacy settings require
	PhoneHash       string                 `json:"phone_hash"`   // Hash for identification without exposing number
	ListSource      string                 `json:"list_source"`
	SuppressReason  string                 `json:"suppress_reason"`
	AddedAt         time.Time              `json:"added_at"`
	ExpiresAt       *time.Time             `json:"expires_at,omitempty"`
	IsActive        bool                   `json:"is_active"`
	IsExpired       bool                   `json:"is_expired"`
	IsTemporary     bool                   `json:"is_temporary"`
	IsPermanent     bool                   `json:"is_permanent"`
	CanCall         bool                   `json:"can_call"`
	Priority        int                    `json:"priority"`
	ComplianceCode  string                 `json:"compliance_code"`
	RequiresDocs    bool                   `json:"requires_documentation"`
	RetentionDays   int                    `json:"retention_days"`
	TimeUntilExpiry *string                `json:"time_until_expiry,omitempty"`
	SourceReference *string                `json:"source_reference,omitempty"`
	Notes           *string                `json:"notes,omitempty"` // Redacted if contains PII
	UpdatedAt       time.Time              `json:"updated_at"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"` // Filtered for privacy
}

// Links provides HATEOAS links for DNCEntryResponse
func (d DNCEntryResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/dnc/entries/%s", baseURL, d.ID),
	}
	
	// Action links based on status
	if d.IsActive {
		links["check"] = fmt.Sprintf("%s/api/v1/dnc/check?phone_number=%s", baseURL, d.PhoneNumber)
		if d.ListSource == "internal" || d.ListSource == "custom" {
			links["update"] = fmt.Sprintf("%s/api/v1/dnc/entries/%s", baseURL, d.ID)
			links["delete"] = fmt.Sprintf("%s/api/v1/dnc/entries/%s", baseURL, d.ID)
		}
	}
	
	return links
}

// ProviderStatusResponse represents the status and health of a DNC provider
type ProviderStatusResponse struct {
	ID               uuid.UUID              `json:"id"`
	Name             string                 `json:"name"`
	Type             string                 `json:"type"`
	Status           string                 `json:"status"`
	Enabled          bool                   `json:"enabled"`
	IsRegulatory     bool                   `json:"is_regulatory"`
	BaseURL          string                 `json:"base_url"`
	AuthType         string                 `json:"auth_type"`
	Priority         int                    `json:"priority"`
	UpdateFrequency  string                 `json:"update_frequency"`
	LastSyncAt       *time.Time             `json:"last_sync_at,omitempty"`
	NextSyncAt       *time.Time             `json:"next_sync_at,omitempty"`
	LastSyncDuration *string                `json:"last_sync_duration,omitempty"`
	LastSyncRecords  *int                   `json:"last_sync_records,omitempty"`
	SuccessRate      float64                `json:"success_rate"`
	ErrorCount       int                    `json:"error_count"`
	SuccessCount     int                    `json:"success_count"`
	LastError        *string                `json:"last_error,omitempty"`
	NeedsSync        bool                   `json:"needs_sync"`
	HealthStatus     string                 `json:"health_status"`
	ComplianceCode   string                 `json:"compliance_code,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// Links provides HATEOAS links for ProviderStatusResponse
func (p ProviderStatusResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/dnc/providers/%s", baseURL, p.ID),
	}
	
	// Action links based on status and type
	if p.Enabled {
		links["sync"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s/sync", baseURL, p.ID)
		links["disable"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s/disable", baseURL, p.ID)
	} else {
		links["enable"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s/enable", baseURL, p.ID)
	}
	
	if p.Type == "internal" || p.Type == "custom" {
		links["update"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s", baseURL, p.ID)
		links["delete"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s", baseURL, p.ID)
	}
	
	links["health"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s/health", baseURL, p.ID)
	links["stats"] = fmt.Sprintf("%s/api/v1/dnc/providers/%s/stats", baseURL, p.ID)
	
	return links
}

// ComplianceReportResponse represents a DNC compliance report
type ComplianceReportResponse struct {
	ReportID        uuid.UUID                    `json:"report_id"`
	ReportType      string                       `json:"report_type"`
	GeneratedAt     time.Time                    `json:"generated_at"`
	TimeRange       DateRangeResponse            `json:"time_range"`
	Summary         ComplianceReportSummary      `json:"summary"`
	SourceBreakdown []SourceComplianceBreakdown  `json:"source_breakdown,omitempty"`
	ViolationSummary []ViolationSummary          `json:"violation_summary,omitempty"`
	CallCompliance  []CallComplianceRecord       `json:"call_compliance,omitempty"`
	ProviderStatus  []ProviderHealthSummary      `json:"provider_status,omitempty"`
	Recommendations []string                     `json:"recommendations,omitempty"`
	RiskAnalysis    ComplianceRiskAnalysis       `json:"risk_analysis"`
	ExportURL       *string                      `json:"export_url,omitempty"`
	ExpiresAt       *time.Time                   `json:"expires_at,omitempty"`
	Metadata        map[string]interface{}       `json:"metadata,omitempty"`
}

// DateRangeResponse represents a date range
type DateRangeResponse struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Days      int       `json:"days"`
}

// ComplianceReportSummary provides high-level compliance metrics
type ComplianceReportSummary struct {
	TotalChecks           int64   `json:"total_checks"`
	TotalBlocked          int64   `json:"total_blocked"`
	TotalAllowed          int64   `json:"total_allowed"`
	BlockRate             float64 `json:"block_rate"`
	ComplianceRate        float64 `json:"compliance_rate"`
	HighRiskCount         int64   `json:"high_risk_count"`
	RegulatoryBlocks      int64   `json:"regulatory_blocks"`
	ConsumerRequestBlocks int64   `json:"consumer_request_blocks"`
	PolicyBlocks          int64   `json:"policy_blocks"`
	ExpiredEntries        int64   `json:"expired_entries"`
	UniqueNumbers         int64   `json:"unique_numbers"`
	AverageCheckTime      string  `json:"average_check_time"`
}

// SourceComplianceBreakdown provides metrics by DNC source
type SourceComplianceBreakdown struct {
	Source          string  `json:"source"`
	DisplayName     string  `json:"display_name"`
	TotalEntries    int64   `json:"total_entries"`
	ActiveEntries   int64   `json:"active_entries"`
	ExpiredEntries  int64   `json:"expired_entries"`
	ChecksPerformed int64   `json:"checks_performed"`
	BlocksTriggered int64   `json:"blocks_triggered"`
	BlockRate       float64 `json:"block_rate"`
	LastUpdated     *time.Time `json:"last_updated,omitempty"`
	IsRegulatory    bool    `json:"is_regulatory"`
	ComplianceCode  string  `json:"compliance_code,omitempty"`
}

// ViolationSummary provides violation statistics
type ViolationSummary struct {
	ViolationType   string  `json:"violation_type"`
	Count           int64   `json:"count"`
	Severity        string  `json:"severity"`
	TrendDirection  string  `json:"trend_direction"` // up, down, stable
	TrendPercent    float64 `json:"trend_percent"`
	LastOccurrence  *time.Time `json:"last_occurrence,omitempty"`
	FirstOccurrence *time.Time `json:"first_occurrence,omitempty"`
	ComplianceCode  string  `json:"compliance_code,omitempty"`
}

// CallComplianceRecord represents compliance status for calls
type CallComplianceRecord struct {
	CallID          uuid.UUID  `json:"call_id"`
	PhoneNumber     string     `json:"phone_number"` // Masked based on privacy settings
	CalledAt        time.Time  `json:"called_at"`
	IsCompliant     bool       `json:"is_compliant"`
	ViolationType   *string    `json:"violation_type,omitempty"`
	ConsentStatus   string     `json:"consent_status"`
	DNCStatus       string     `json:"dnc_status"`
	TCPACompliant   bool       `json:"tcpa_compliant"`
	TimeCompliant   bool       `json:"time_compliant"`
	ComplianceScore float64    `json:"compliance_score"`
	RiskLevel       string     `json:"risk_level"`
}

// ProviderHealthSummary provides health status for providers
type ProviderHealthSummary struct {
	ProviderID      uuid.UUID  `json:"provider_id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	HealthScore     float64    `json:"health_score"`
	Uptime          float64    `json:"uptime_percent"`
	LastSync        *time.Time `json:"last_sync,omitempty"`
	SyncSuccess     bool       `json:"sync_success"`
	RecordsUpdated  int        `json:"records_updated"`
	ErrorCount      int        `json:"error_count"`
	ResponseTime    string     `json:"avg_response_time"`
	IsHealthy       bool       `json:"is_healthy"`
}

// ComplianceRiskAnalysis provides risk assessment
type ComplianceRiskAnalysis struct {
	OverallRisk     string                `json:"overall_risk"` // low, medium, high, critical
	RiskScore       float64               `json:"risk_score"`   // 0.0 to 1.0
	RiskFactors     []string              `json:"risk_factors"`
	Vulnerabilities []ComplianceVulnerability `json:"vulnerabilities"`
	MitigationSteps []string              `json:"mitigation_steps"`
	NextReviewDate  time.Time             `json:"next_review_date"`
}

// ComplianceVulnerability represents a compliance vulnerability
type ComplianceVulnerability struct {
	Type        string  `json:"type"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`
	Likelihood  string  `json:"likelihood"`
	Score       float64 `json:"score"`
}

// Links provides HATEOAS links for ComplianceReportResponse
func (c ComplianceReportResponse) Links(baseURL string) map[string]string {
	links := map[string]string{
		"self": fmt.Sprintf("%s/api/v1/dnc/reports/%s", baseURL, c.ReportID),
	}
	
	if c.ExportURL != nil {
		links["download"] = *c.ExportURL
	}
	
	links["regenerate"] = fmt.Sprintf("%s/api/v1/dnc/reports/generate", baseURL)
	
	return links
}

// BulkDNCCheckResponse represents the result of checking multiple numbers
type BulkDNCCheckResponse struct {
	RequestID       uuid.UUID             `json:"request_id"`
	TotalNumbers    int                   `json:"total_numbers"`
	ProcessedCount  int                   `json:"processed_count"`
	BlockedCount    int                   `json:"blocked_count"`
	AllowedCount    int                   `json:"allowed_count"`
	ErrorCount      int                   `json:"error_count"`
	ProcessingTime  string                `json:"processing_time"`
	Results         []DNCCheckResponse    `json:"results"`
	Errors          []BulkCheckError      `json:"errors,omitempty"`
	Summary         BulkCheckSummary      `json:"summary"`
	CompletedAt     time.Time             `json:"completed_at"`
}

// BulkCheckError represents an error in bulk checking
type BulkCheckError struct {
	PhoneNumber string `json:"phone_number"`
	Error       string `json:"error"`
	ErrorCode   string `json:"error_code"`
	Severity    string `json:"severity"`
}

// BulkCheckSummary provides summary statistics for bulk checks
type BulkCheckSummary struct {
	BlockRate        float64           `json:"block_rate"`
	ComplianceRate   float64           `json:"compliance_rate"`
	HighRiskCount    int               `json:"high_risk_count"`
	SourceBreakdown  map[string]int    `json:"source_breakdown"`
	SeverityBreakdown map[string]int   `json:"severity_breakdown"`
	AverageCheckTime string            `json:"average_check_time"`
	Recommendations  []string          `json:"recommendations"`
}

// DNCConsentVerificationResponse represents consent verification result
type DNCConsentVerificationResponse struct {
	PhoneNumber       string                      `json:"phone_number"`
	ConsentStatus     string                      `json:"consent_status"` // valid, invalid, expired, revoked
	HasValidConsent   bool                        `json:"has_valid_consent"`
	ConsentType       string                      `json:"consent_type"`
	ConsentDate       *time.Time                  `json:"consent_date,omitempty"`
	ExpiresAt         *time.Time                  `json:"expires_at,omitempty"`
	IsOptedOut        bool                        `json:"is_opted_out"`
	OptOutDate        *time.Time                  `json:"opt_out_date,omitempty"`
	TCPACompliant     bool                        `json:"tcpa_compliant"`
	GDPRCompliant     bool                        `json:"gdpr_compliant"`
	ComplianceChecks  []ConsentComplianceCheck    `json:"compliance_checks"`
	RiskLevel         string                      `json:"risk_level"`
	Recommendations   []string                    `json:"recommendations"`
	VerifiedAt        time.Time                   `json:"verified_at"`
	Metadata          map[string]interface{}      `json:"metadata,omitempty"`
}

// ConsentComplianceCheck represents a specific compliance check
type ConsentComplianceCheck struct {
	CheckType   string `json:"check_type"`
	Passed      bool   `json:"passed"`
	Details     string `json:"details"`
	Requirement string `json:"requirement"`
	Severity    string `json:"severity"`
}

// ProviderSyncResponse represents the result of a provider sync operation
type ProviderSyncResponse struct {
	SyncID          uuid.UUID          `json:"sync_id"`
	ProviderID      uuid.UUID          `json:"provider_id"`
	ProviderName    string             `json:"provider_name"`
	Status          string             `json:"status"` // started, in_progress, completed, failed
	SyncType        string             `json:"sync_type"`
	StartedAt       time.Time          `json:"started_at"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty"`
	Duration        *string            `json:"duration,omitempty"`
	RecordsAdded    int                `json:"records_added"`
	RecordsUpdated  int                `json:"records_updated"`
	RecordsRemoved  int                `json:"records_removed"`
	RecordsTotal    int                `json:"records_total"`
	ErrorCount      int                `json:"error_count"`
	Errors          []ProviderSyncError `json:"errors,omitempty"`
	NextSyncAt      *time.Time         `json:"next_sync_at,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderSyncError represents an error during provider sync
type ProviderSyncError struct {
	ErrorType   string `json:"error_type"`
	Message     string `json:"message"`
	RecordID    *string `json:"record_id,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Severity    string `json:"severity"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// DNC Handler Response Types

// PaginatedDNCEntriesResponse represents paginated DNC entries
type PaginatedDNCEntriesResponse struct {
	Entries     []DNCEntryResponse `json:"entries"`
	TotalCount  int                `json:"total_count"`
	Page        int                `json:"page"`
	Limit       int                `json:"limit"`
	TotalPages  int                `json:"total_pages"`
	HasNext     bool               `json:"has_next"`
	HasPrevious bool               `json:"has_previous"`
}

// BulkCheckSummary provides summary stats for bulk DNC checks
type BulkCheckSummary struct {
	BlockedCount      int     `json:"blocked_count"`
	AllowedCount      int     `json:"allowed_count"`
	AverageRiskScore  float64 `json:"average_risk_score"`
	TotalResponseTime int     `json:"total_response_time_ms"`
}

// ProviderResultResponse represents a single provider's check result
type ProviderResultResponse struct {
	ProviderID   string `json:"provider_id"`
	Status       string `json:"status"`
	IsBlocked    bool   `json:"is_blocked"`
	ResponseTime int    `json:"response_time_ms"`
	Error        string `json:"error,omitempty"`
	CacheHit     bool   `json:"cache_hit"`
}

// ComplianceSummaryResponse provides compliance report summary
type ComplianceSummaryResponse struct {
	TotalChecks       int                `json:"total_checks"`
	BlockedCalls      int                `json:"blocked_calls"`
	AllowedCalls      int                `json:"allowed_calls"`
	ViolationCount    int                `json:"violation_count"`
	ComplianceRate    float64            `json:"compliance_rate"`
	RiskScore         float64            `json:"risk_score"`
	TopViolationTypes map[string]int     `json:"top_violation_types"`
}

// ComplianceViolationResponse represents a compliance violation
type ComplianceViolationResponse struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	PhoneHash   string                 `json:"phone_hash"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolution  *string                `json:"resolution,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// HealthResponse represents service health status
type HealthResponse struct {
	Status       string                       `json:"status"`
	Timestamp    time.Time                    `json:"timestamp"`
	Version      string                       `json:"version"`
	Uptime       time.Duration                `json:"uptime"`
	Checks       map[string]string            `json:"checks"`
	Metrics      map[string]interface{}       `json:"metrics"`
	Dependencies map[string]interface{}       `json:"dependencies"`
}

// CacheStatsResponse represents cache performance statistics
type CacheStatsResponse struct {
	HitRate              float64                 `json:"hit_rate"`
	MissRate             float64                 `json:"miss_rate"`
	TotalKeys            int                     `json:"total_keys"`
	MemoryUsage          int64                   `json:"memory_usage_bytes"`
	Evictions            int                     `json:"evictions"`
	LastUpdated          time.Time               `json:"last_updated"`
	TTLDistribution      map[string]int          `json:"ttl_distribution"`
	PerformanceMetrics   map[string]interface{}  `json:"performance_metrics"`
}

// ClearCacheResponse represents cache clear operation result
type ClearCacheResponse struct {
	Pattern   string    `json:"pattern"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}