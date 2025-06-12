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