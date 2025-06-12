package rest

import (
	"time"

	"github.com/google/uuid"
)

// Common request types with comprehensive validation

// PaginationRequest provides standard pagination parameters
type PaginationRequest struct {
	Page     int    `json:"page" query:"page" validate:"min=1" default:"1"`
	PageSize int    `json:"page_size" query:"page_size" validate:"min=1,max=100" default:"20"`
	Sort     string `json:"sort" query:"sort" validate:"omitempty,oneof=created_at -created_at updated_at -updated_at"`
	Order    string `json:"order" query:"order" validate:"omitempty,oneof=asc desc" default:"asc"`
}

// DateRangeRequest provides date filtering
type DateRangeRequest struct {
	StartDate *time.Time `json:"start_date" query:"start_date" validate:"omitempty"`
	EndDate   *time.Time `json:"end_date" query:"end_date" validate:"omitempty,gtfield=StartDate"`
}

// Call-related requests

// CreateCallRequestV2 for creating a new call (enhanced version)
type CreateCallRequestV2 struct {
	FromNumber string            `json:"from_number" validate:"required,phone,e164"`
	ToNumber   string            `json:"to_number" validate:"required,phone,e164,nefield=FromNumber"`
	Direction  string            `json:"direction" validate:"omitempty,oneof=inbound outbound" default:"outbound"`
	Metadata   map[string]string `json:"metadata" validate:"omitempty,dive,keys,alphanum,endkeys,max=255"`
}

// UpdateCallRequest for updating call details
type UpdateCallRequest struct {
	Status   *string           `json:"status,omitempty" validate:"omitempty,oneof=pending queued ringing in_progress completed failed canceled no_answer busy"`
	Duration *int              `json:"duration,omitempty" validate:"omitempty,min=0"`
	Cost     *float64          `json:"cost,omitempty" validate:"omitempty,min=0"`
	Metadata map[string]string `json:"metadata,omitempty" validate:"omitempty,dive,keys,alphanum,endkeys,max=255"`
}

// RouteCallRequest for routing a call
type RouteCallRequest struct {
	Algorithm   string                 `json:"algorithm" validate:"omitempty,oneof=round_robin skill_based cost_based quality_based" default:"round_robin"`
	Preferences map[string]interface{} `json:"preferences" validate:"omitempty"`
	MaxRetries  int                    `json:"max_retries" validate:"omitempty,min=0,max=5" default:"3"`
}

// CompleteCallRequestV2 for marking a call complete (enhanced version)
type CompleteCallRequestV2 struct {
	Duration      int                    `json:"duration" validate:"required,min=0"`
	QualityScore  *float64               `json:"quality_score,omitempty" validate:"omitempty,min=0,max=5"`
	DispositionCode string               `json:"disposition_code,omitempty" validate:"omitempty,alphanum,max=50"`
	Notes         string                 `json:"notes,omitempty" validate:"omitempty,max=1000"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Bid-related requests

// CreateBidProfileRequest for creating a bid profile
type CreateBidProfileRequest struct {
	Name        string                 `json:"name" validate:"required,min=3,max=100"`
	Description string                 `json:"description,omitempty" validate:"omitempty,max=500"`
	Active      bool                   `json:"active" default:"true"`
	Criteria    BidCriteriaRequest     `json:"criteria" validate:"required,dive"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// BidCriteriaRequest defines bid criteria
type BidCriteriaRequest struct {
	MinPrice        float64    `json:"min_price" validate:"required,min=0"`
	MaxPrice        float64    `json:"max_price" validate:"required,gtfield=MinPrice"`
	TargetStates    []string   `json:"target_states,omitempty" validate:"omitempty,dive,len=2,alpha"`
	TargetAreaCodes []string   `json:"target_area_codes,omitempty" validate:"omitempty,dive,len=3,numeric"`
	TimeRestrictions TimeRange `json:"time_restrictions,omitempty" validate:"omitempty,dive"`
	QualityThreshold float64   `json:"quality_threshold" validate:"min=0,max=100" default:"80"`
}

// TimeRange for time-based restrictions
type TimeRange struct {
	StartTime string   `json:"start_time" validate:"required,datetime=15:04"`
	EndTime   string   `json:"end_time" validate:"required,datetime=15:04,gtfield=StartTime"`
	Timezone  string   `json:"timezone" validate:"required,timezone"`
	DaysOfWeek []string `json:"days_of_week,omitempty" validate:"omitempty,dive,oneof=monday tuesday wednesday thursday friday saturday sunday"`
}

// CreateBidRequestV2 for placing a bid (enhanced version)
type CreateBidRequestV2 struct {
	AuctionID    uuid.UUID              `json:"auction_id" validate:"required,uuid"`
	Amount       float64                `json:"amount" validate:"required,gt=0"`
	AutoBid      bool                   `json:"auto_bid" default:"false"`
	MaxAutoBid   *float64               `json:"max_auto_bid,omitempty" validate:"omitempty,gtfield=Amount"`
	ValidForSecs int                    `json:"valid_for_secs" validate:"min=10,max=300" default:"60"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Auction requests

// CreateAuctionRequestV2 for creating an auction (enhanced version)
type CreateAuctionRequestV2 struct {
	CallID       uuid.UUID              `json:"call_id" validate:"required,uuid"`
	ReservePrice float64                `json:"reserve_price" validate:"min=0"`
	Duration     int                    `json:"duration" validate:"min=5,max=300" default:"30"`
	AutoExtend   bool                   `json:"auto_extend" default:"true"`
	ExtendSecs   int                    `json:"extend_secs" validate:"min=5,max=60" default:"10"`
	Criteria     map[string]interface{} `json:"criteria,omitempty"`
}

// Authentication requests

// LoginRequest for user authentication
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	MFACode  string `json:"mfa_code,omitempty" validate:"omitempty,len=6,numeric"`
}

// RegisterRequest for new user registration
type RegisterRequest struct {
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8,containsany=!@#$%^&*,containsany=0123456789,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	AccountType     string `json:"account_type" validate:"required,oneof=buyer seller"`
	CompanyName     string `json:"company_name" validate:"required,min=2,max=100"`
	PhoneNumber     string `json:"phone_number" validate:"required,phone,e164"`
	AcceptTerms     bool   `json:"accept_terms" validate:"required,eq=true"`
}

// RefreshTokenRequest for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,jwt"`
}

// Account management requests

// UpdateAccountRequest for updating account details
type UpdateAccountRequest struct {
	CompanyName *string                `json:"company_name,omitempty" validate:"omitempty,min=2,max=100"`
	ContactName *string                `json:"contact_name,omitempty" validate:"omitempty,min=2,max=100"`
	PhoneNumber *string                `json:"phone_number,omitempty" validate:"omitempty,phone,e164"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// UpdatePasswordRequest for changing password
type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,containsany=!@#$%^&*,containsany=0123456789,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ,nefield=CurrentPassword"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=NewPassword"`
}

// Compliance requests

// AddDNCRequestV2 for adding to Do Not Call list (enhanced version)
type AddDNCRequestV2 struct {
	PhoneNumber string    `json:"phone_number" validate:"required,phone,e164"`
	Reason      string    `json:"reason" validate:"required,oneof=consumer_request internal_policy legal_requirement"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" validate:"omitempty,gtfield=Now"`
	Notes       string     `json:"notes,omitempty" validate:"omitempty,max=500"`
}

// SetTCPAHoursRequestV2 for configuring TCPA calling hours (enhanced version)
type SetTCPAHoursRequestV2 struct {
	StartTime   string   `json:"start_time" validate:"required,datetime=15:04"`
	EndTime     string   `json:"end_time" validate:"required,datetime=15:04,gtfield=StartTime"`
	Timezone    string   `json:"timezone" validate:"required,timezone"`
	DaysOfWeek  []string `json:"days_of_week" validate:"required,min=1,dive,oneof=monday tuesday wednesday thursday friday saturday sunday"`
	HolidayMode string   `json:"holiday_mode" validate:"omitempty,oneof=block allow custom" default:"block"`
}

// Financial requests

// CreatePaymentRequest for processing payments
type CreatePaymentRequest struct {
	Amount      float64 `json:"amount" validate:"required,gt=0"`
	Currency    string  `json:"currency" validate:"required,iso4217" default:"USD"`
	Method      string  `json:"method" validate:"required,oneof=credit_card ach wire transfer balance"`
	Description string  `json:"description,omitempty" validate:"omitempty,max=255"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WithdrawRequest for withdrawing funds
type WithdrawRequest struct {
	Amount          float64 `json:"amount" validate:"required,gt=0"`
	Currency        string  `json:"currency" validate:"required,iso4217" default:"USD"`
	BankAccountID   string  `json:"bank_account_id" validate:"required,uuid"`
	VerificationCode string `json:"verification_code" validate:"required,len=6,numeric"`
}

// Analytics requests

// GetMetricsRequest for retrieving metrics
type GetMetricsRequest struct {
	PaginationRequest
	DateRangeRequest
	MetricTypes []string `json:"metric_types" query:"metric_types" validate:"required,min=1,dive,oneof=calls bids revenue quality conversion"`
	GroupBy     string   `json:"group_by" query:"group_by" validate:"omitempty,oneof=hour day week month"`
	Filters     map[string][]string `json:"filters" query:"filters" validate:"omitempty"`
}

// GenerateReportRequest for creating reports
type GenerateReportRequest struct {
	ReportType  string     `json:"report_type" validate:"required,oneof=performance financial compliance quality"`
	StartDate   time.Time  `json:"start_date" validate:"required"`
	EndDate     time.Time  `json:"end_date" validate:"required,gtfield=StartDate"`
	Format      string     `json:"format" validate:"required,oneof=pdf csv excel json" default:"pdf"`
	Email       bool       `json:"email" default:"false"`
	Scheduled   bool       `json:"scheduled" default:"false"`
	ScheduleCron string    `json:"schedule_cron,omitempty" validate:"omitempty,cron"`
}

// Search requests

// SearchRequest provides full-text search capabilities
type SearchRequest struct {
	PaginationRequest
	Query       string              `json:"query" query:"q" validate:"required,min=2,max=255"`
	SearchIn    []string            `json:"search_in" query:"search_in" validate:"omitempty,dive,oneof=calls bids accounts"`
	Filters     map[string][]string `json:"filters" query:"filters"`
	Highlight   bool                `json:"highlight" query:"highlight" default:"true"`
	FuzzySearch bool                `json:"fuzzy_search" query:"fuzzy" default:"false"`
}

// Webhook requests

// CreateWebhookRequest for registering webhooks
type CreateWebhookRequest struct {
	URL         string   `json:"url" validate:"required,url,https"`
	Events      []string `json:"events" validate:"required,min=1,dive,oneof=call.created call.completed bid.placed bid.won auction.started auction.ended"`
	Secret      string   `json:"secret" validate:"required,min=32"`
	Active      bool     `json:"active" default:"true"`
	Description string   `json:"description,omitempty" validate:"omitempty,max=255"`
}

// Batch operation requests

// BatchOperationRequest for bulk operations
type BatchOperationRequest struct {
	Operation string                   `json:"operation" validate:"required,oneof=create update delete"`
	Resource  string                   `json:"resource" validate:"required,oneof=calls bids accounts"`
	Items     []map[string]interface{} `json:"items" validate:"required,min=1,max=100"`
	Options   BatchOptions             `json:"options"`
}

// BatchOptions for batch operations
type BatchOptions struct {
	StopOnError      bool `json:"stop_on_error" default:"false"`
	ValidateOnly     bool `json:"validate_only" default:"false"`
	ReturnResults    bool `json:"return_results" default:"true"`
	ParallelExecution bool `json:"parallel_execution" default:"false"`
}

// These types might already exist elsewhere - check handlers.go for definitions