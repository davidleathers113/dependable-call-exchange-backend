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

// Audit request types

// AuditEventQueryRequest for querying audit events
type AuditEventQueryRequest struct {
	PaginationRequest
	DateRangeRequest
	Actor      string   `json:"actor" query:"actor" validate:"omitempty"`
	EventType  string   `json:"event_type" query:"event_type" validate:"omitempty,oneof=authentication authorization data_access configuration system financial compliance"`
	Resource   string   `json:"resource" query:"resource" validate:"omitempty"`
	Action     string   `json:"action" query:"action" validate:"omitempty"`
	Outcome    string   `json:"outcome" query:"outcome" validate:"omitempty,oneof=success failure partial"`
	Severity   string   `json:"severity" query:"severity" validate:"omitempty,oneof=low medium high critical"`
	IPAddress  string   `json:"ip_address" query:"ip_address" validate:"omitempty,ip"`
	SortBy     string   `json:"sort_by" query:"sort_by" validate:"omitempty,oneof=timestamp actor event_type severity outcome"`
	SortOrder  string   `json:"sort_order" query:"sort_order" validate:"omitempty,oneof=asc desc"`
}

// AuditAdvancedSearchRequest for advanced audit search
type AuditAdvancedSearchRequest struct {
	PaginationRequest
	DateRangeRequest
	Query      string            `json:"query" query:"q" validate:"required,min=2,max=500"`
	Fields     []string          `json:"fields" query:"fields" validate:"omitempty,dive,oneof=actor event_type resource action outcome severity timestamp data"`
	Filters    map[string]string `json:"filters" query:"filters" validate:"omitempty"`
	Facets     []string          `json:"facets" query:"facets" validate:"omitempty,dive,oneof=actor event_type resource outcome severity"`
	Highlight  bool              `json:"highlight" query:"highlight" default:"true"`
	FuzzyMatch bool              `json:"fuzzy_match" query:"fuzzy" default:"false"`
}

// AuditStatsRequest for audit statistics
type AuditStatsRequest struct {
	DateRangeRequest
	GroupBy string   `json:"group_by" query:"group_by" validate:"omitempty,oneof=hour day week month"`
	Metrics []string `json:"metrics" query:"metrics" validate:"omitempty,dive,oneof=events success_rate error_rate top_actors timeline"`
}

// AuditExportRequest for generating audit exports
type AuditExportRequest struct {
	DateRangeRequest
	ReportType      string            `json:"report_type" validate:"required,oneof=gdpr_data_subject tcpa_consent_trail sox_financial_audit security_incident custom_query"`
	Format          string            `json:"format" validate:"required,oneof=json csv parquet pdf"`
	Filters         map[string]string `json:"filters" validate:"omitempty"`
	RedactPII       bool              `json:"redact_pii" default:"true"`
	IncludeMetadata bool              `json:"include_metadata" default:"true"`
	ChunkSize       int               `json:"chunk_size" validate:"omitempty,min=100,max=10000" default:"1000"`
	CustomTemplate  string            `json:"custom_template,omitempty" validate:"omitempty,max=1000"`
	Stream          bool              `json:"stream" default:"false"`
}

// AuditStreamRequest for streaming audit data
type AuditStreamRequest struct {
	DateRangeRequest
	Filters     map[string]string `json:"filters" validate:"omitempty"`
	ChunkSize   int               `json:"chunk_size" validate:"min=10,max=1000" default:"100"`
	Format      string            `json:"format" validate:"required,oneof=json csv"`
	Compression bool              `json:"compression" default:"false"`
}

// GDPRDataSubjectRequest for GDPR data subject requests
type GDPRDataSubjectRequest struct {
	DateRangeRequest
	SubjectID      string `json:"subject_id" validate:"required,min=1,max=255"`
	IncludePII     bool   `json:"include_pii" default:"false"`
	ExportFormat   string `json:"format" validate:"omitempty,oneof=json pdf" default:"json"`
	RightType      string `json:"right_type" validate:"omitempty,oneof=access rectification erasure portability"`
	VerifyIdentity bool   `json:"verify_identity" default:"true"`
}

// TCPAComplianceRequest for TCPA compliance reports
type TCPAComplianceRequest struct {
	DateRangeRequest
	PhoneNumber string `json:"phone_number" validate:"required,e164"`
	Detailed    bool   `json:"detailed" default:"false"`
	IncludeCallHistory bool `json:"include_call_history" default:"true"`
	IncludeViolations  bool `json:"include_violations" default:"true"`
	ExportFormat       string `json:"format" validate:"omitempty,oneof=json pdf csv" default:"json"`
}

// IntegrityCheckRequest for integrity verification
type IntegrityCheckRequest struct {
	DateRangeRequest
	CheckType        string   `json:"check_type" validate:"required,oneof=full incremental hash_chain sequence_check"`
	DeepScan         bool     `json:"deep_scan" default:"false"`
	VerifySignatures bool     `json:"verify_signatures" default:"true"`
	CheckIntegrity   bool     `json:"check_integrity" default:"true"`
	Algorithms       []string `json:"algorithms" validate:"omitempty,dive,oneof=sha256 sha512 blake2b"`
	ParallelCheck    bool     `json:"parallel_check" default:"false"`
}

// ComplianceReportRequest for compliance reporting
type ComplianceReportRequest struct {
	DateRangeRequest
	ComplianceType   string   `json:"compliance_type" validate:"required,oneof=gdpr tcpa sox pci hipaa"`
	Jurisdiction     string   `json:"jurisdiction" validate:"omitempty,alpha,len=2"`
	IncludeEvidence  bool     `json:"include_evidence" default:"true"`
	IncludeMetrics   bool     `json:"include_metrics" default:"true"`
	AuditTrailLevel  string   `json:"audit_trail_level" validate:"omitempty,oneof=basic detailed comprehensive" default:"detailed"`
	CertificationReq bool     `json:"certification_required" default:"false"`
}

// AuditRetentionRequest for audit retention policies
type AuditRetentionRequest struct {
	EventTypes      []string `json:"event_types" validate:"required,min=1,dive,oneof=authentication authorization data_access configuration system financial compliance"`
	RetentionPeriod string   `json:"retention_period" validate:"required,duration"`
	ArchiveAfter    string   `json:"archive_after" validate:"omitempty,duration"`
	PurgeAfter      string   `json:"purge_after" validate:"omitempty,duration"`
	Jurisdiction    string   `json:"jurisdiction" validate:"omitempty,alpha,len=2"`
	LegalHold       bool     `json:"legal_hold" default:"false"`
}

// DNC Integration requests

// CreateDNCEntryRequest for adding entries to Do Not Call lists
type CreateDNCEntryRequest struct {
	PhoneNumber     string    `json:"phone_number" validate:"required,e164"`
	ListSource      string    `json:"list_source" validate:"required,oneof=federal state internal custom"`
	SuppressReason  string    `json:"suppress_reason" validate:"required,oneof=user_request legal_requirement fraud_prevention company_policy tcpa_violation gdpr_erasure"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty" validate:"omitempty,gtfield=Now"`
	SourceReference *string   `json:"source_reference,omitempty" validate:"omitempty,max=255"`
	Notes           *string   `json:"notes,omitempty" validate:"omitempty,max=1000"`
	Metadata        map[string]string `json:"metadata,omitempty" validate:"omitempty,dive,keys,alphanum,endkeys,max=255"`
}

// DNCCheckRequest for checking if a number is on DNC lists
type DNCCheckRequest struct {
	PhoneNumber     string   `json:"phone_number" validate:"required,e164"`
	Sources         []string `json:"sources,omitempty" validate:"omitempty,dive,oneof=federal state internal custom"`
	ComplianceLevel string   `json:"compliance_level,omitempty" validate:"omitempty,oneof=strict standard relaxed" default:"standard"`
	IncludeExpired  bool     `json:"include_expired" default:"false"`
	TTL             *int     `json:"ttl_seconds,omitempty" validate:"omitempty,min=60,max=604800"` // 1 minute to 7 days
}

// ProviderSyncRequest for synchronizing DNC provider data
type ProviderSyncRequest struct {
	ProviderID      uuid.UUID `json:"provider_id" validate:"required,uuid"`
	ForceSync       bool      `json:"force_sync" default:"false"`
	SyncType        string    `json:"sync_type" validate:"omitempty,oneof=full incremental delta" default:"incremental"`
	LastSyncToken   *string   `json:"last_sync_token,omitempty" validate:"omitempty,max=500"`
	MaxRecords      *int      `json:"max_records,omitempty" validate:"omitempty,min=1,max=1000000"`
	TimeoutSeconds  *int      `json:"timeout_seconds,omitempty" validate:"omitempty,min=30,max=3600"`
}

// ComplianceReportRequest for generating DNC compliance reports
type ComplianceReportRequest struct {
	DateRangeRequest
	ReportType      string   `json:"report_type" validate:"required,oneof=violation_summary source_breakdown call_compliance provider_status regulatory_audit"`
	PhoneNumbers    []string `json:"phone_numbers,omitempty" validate:"omitempty,dive,e164"`
	Sources         []string `json:"sources,omitempty" validate:"omitempty,dive,oneof=federal state internal custom"`
	IncludeDetails  bool     `json:"include_details" default:"true"`
	GroupBy         string   `json:"group_by,omitempty" validate:"omitempty,oneof=source provider day week month"`
	ExportFormat    string   `json:"export_format" validate:"omitempty,oneof=json csv pdf excel" default:"json"`
	IncludeMetadata bool     `json:"include_metadata" default:"true"`
}

// UpdateDNCEntryRequest for updating existing DNC entries
type UpdateDNCEntryRequest struct {
	ExpiresAt       *time.Time        `json:"expires_at,omitempty" validate:"omitempty,gtfield=Now"`
	Notes           *string           `json:"notes,omitempty" validate:"omitempty,max=1000"`
	Metadata        map[string]string `json:"metadata,omitempty" validate:"omitempty,dive,keys,alphanum,endkeys,max=255"`
	SourceReference *string           `json:"source_reference,omitempty" validate:"omitempty,max=255"`
}

// BulkDNCCheckRequest for checking multiple numbers
type BulkDNCCheckRequest struct {
	PhoneNumbers    []string `json:"phone_numbers" validate:"required,min=1,max=1000,dive,e164"`
	Sources         []string `json:"sources,omitempty" validate:"omitempty,dive,oneof=federal state internal custom"`
	ComplianceLevel string   `json:"compliance_level,omitempty" validate:"omitempty,oneof=strict standard relaxed" default:"standard"`
	IncludeExpired  bool     `json:"include_expired" default:"false"`
	TTL             *int     `json:"ttl_seconds,omitempty" validate:"omitempty,min=60,max=604800"`
	Parallel        bool     `json:"parallel" default:"true"`
}

// CreateDNCProviderRequest for registering new DNC providers
type CreateDNCProviderRequest struct {
	Name            string            `json:"name" validate:"required,min=3,max=100"`
	Type            string            `json:"type" validate:"required,oneof=federal state internal custom"`
	BaseURL         string            `json:"base_url" validate:"required,url"`
	AuthType        string            `json:"auth_type" validate:"required,oneof=none api_key oauth basic"`
	APIKey          *string           `json:"api_key,omitempty" validate:"omitempty,min=10,max=500"`
	UpdateFrequency *int              `json:"update_frequency_hours,omitempty" validate:"omitempty,min=1,max=720"` // 1 hour to 30 days
	Priority        *int              `json:"priority,omitempty" validate:"omitempty,min=1,max=1000"`
	RetryAttempts   *int              `json:"retry_attempts,omitempty" validate:"omitempty,min=1,max=10"`
	TimeoutSeconds  *int              `json:"timeout_seconds,omitempty" validate:"omitempty,min=5,max=300"`
	RateLimitPerMin *int              `json:"rate_limit_per_min,omitempty" validate:"omitempty,min=1,max=10000"`
	Config          map[string]string `json:"config,omitempty" validate:"omitempty,dive,keys,alphanum,endkeys,max=1000"`
	Enabled         bool              `json:"enabled" default:"false"`
}

// UpdateDNCProviderRequest for updating DNC provider settings
type UpdateDNCProviderRequest struct {
	Name            *string           `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	BaseURL         *string           `json:"base_url,omitempty" validate:"omitempty,url"`
	AuthType        *string           `json:"auth_type,omitempty" validate:"omitempty,oneof=none api_key oauth basic"`
	APIKey          *string           `json:"api_key,omitempty" validate:"omitempty,min=10,max=500"`
	UpdateFrequency *int              `json:"update_frequency_hours,omitempty" validate:"omitempty,min=1,max=720"`
	Priority        *int              `json:"priority,omitempty" validate:"omitempty,min=1,max=1000"`
	RetryAttempts   *int              `json:"retry_attempts,omitempty" validate:"omitempty,min=1,max=10"`
	TimeoutSeconds  *int              `json:"timeout_seconds,omitempty" validate:"omitempty,min=5,max=300"`
	RateLimitPerMin *int              `json:"rate_limit_per_min,omitempty" validate:"omitempty,min=1,max=10000"`
	Config          map[string]string `json:"config,omitempty" validate:"omitempty,dive,keys,alphanum,endkeys,max=1000"`
	Enabled         *bool             `json:"enabled,omitempty"`
}

// DNCNumberLookupRequest for looking up DNC entry details
type DNCNumberLookupRequest struct {
	PhoneNumbers    []string `json:"phone_numbers" validate:"required,min=1,max=100,dive,e164"`
	Sources         []string `json:"sources,omitempty" validate:"omitempty,dive,oneof=federal state internal custom"`
	IncludeExpired  bool     `json:"include_expired" default:"false"`
	IncludeMetadata bool     `json:"include_metadata" default:"true"`
	SortBy          string   `json:"sort_by,omitempty" validate:"omitempty,oneof=added_at expires_at source priority"`
	SortOrder       string   `json:"sort_order,omitempty" validate:"omitempty,oneof=asc desc" default:"desc"`
}

// DNCConsentVerificationRequest for verifying consent status
type DNCConsentVerificationRequest struct {
	PhoneNumber     string     `json:"phone_number" validate:"required,e164"`
	ConsentType     string     `json:"consent_type" validate:"required,oneof=express implied written electronic opt_out"`
	Timestamp       *time.Time `json:"timestamp,omitempty" validate:"omitempty"`
	IPAddress       *string    `json:"ip_address,omitempty" validate:"omitempty,ip"`
	UserAgent       *string    `json:"user_agent,omitempty" validate:"omitempty,max=500"`
	ConsentMethod   *string    `json:"consent_method,omitempty" validate:"omitempty,oneof=web phone sms email paper ivr"`
	ConsentText     *string    `json:"consent_text,omitempty" validate:"omitempty,max=2000"`
	VerifyTCPA      bool       `json:"verify_tcpa" default:"true"`
	VerifyGDPR      bool       `json:"verify_gdpr" default:"false"`
}

// DNC Handler Helper Types

// ListDNCParams represents query parameters for listing DNC entries
type ListDNCParams struct {
	Page      int     `json:"page"`
	Limit     int     `json:"limit"`
	Source    *string `json:"source,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Status    *string `json:"status,omitempty"`
	SortBy    string  `json:"sort_by"`
	SortOrder string  `json:"sort_order"`
}

// ComplianceReportParams represents query parameters for compliance reports
type ComplianceReportParams struct {
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	Format             string     `json:"format"`
	IncludeViolations  bool       `json:"include_violations"`
	IncludeStats       bool       `json:"include_stats"`
}

// ClearCacheRequest represents a cache clear request
type ClearCacheRequest struct {
	Pattern string `json:"pattern,omitempty" validate:"omitempty,max=255"`
}