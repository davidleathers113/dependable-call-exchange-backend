package rest

import (
	"time"

	"github.com/google/uuid"
)

// Service interface types for audit handlers
// These are temporary interfaces to support the audit handlers
// In a real implementation, these would be in the service layer

// QueryService interface placeholder
type QueryService interface {
	QueryEvents(ctx interface{}, req EventQueryRequest) (*EventQueryResult, error)
	GetEvent(ctx interface{}, id uuid.UUID) (*AuditEvent, error)
	AdvancedSearch(ctx interface{}, req AdvancedSearchRequest) (*SearchResult, error)
	GetEventStatistics(ctx interface{}, req StatsRequest) (*StatsResult, error)
	StreamEvents(ctx interface{}, req StreamRequest) (*StreamResult, error)
	PerformIntegrityCheck(ctx interface{}, req IntegrityCheckRequest) (*IntegrityResult, error)
}

// ExportService interface placeholder
type ExportService interface {
	GenerateReport(ctx interface{}, req ExportRequest) (*ExportResult, error)
	StartStreamingExport(ctx interface{}, req StreamingExportRequest) (*StreamingResult, error)
}

// ComplianceService interface placeholder
type ComplianceService interface {
	GenerateGDPRReport(ctx interface{}, req GDPRRequest) (*GDPRResult, error)
	GenerateTCPAReport(ctx interface{}, req TCPARequest) (*TCPAResult, error)
}

// Service request/response types

type EventQueryRequest struct {
	Filters    map[string]interface{}
	Pagination PaginationRequest
	SortBy     string
	SortOrder  string
}

type EventQueryResult struct {
	Events           []AuditEvent
	Pagination       PaginationResponse
	QueryDurationMs  int64
	CacheHit         bool
}

type AuditEvent struct {
	ID        uuid.UUID
	EventType string
	Actor     string
	Resource  string
	Action    string
	Outcome   string
	Severity  string
	IPAddress string
	UserAgent string
	Timestamp time.Time
	Data      map[string]interface{}
	Metadata  map[string]interface{}
}

type AdvancedSearchRequest struct {
	Query      string
	Fields     []string
	Filters    map[string]interface{}
	Facets     []string
	Highlight  bool
	Pagination PaginationRequest
	TimeRange  *TimeRange
}

type SearchResult struct {
	Events       []AuditEvent
	Pagination   PaginationResponse
	Facets       map[string]interface{}
	Highlights   map[string]interface{}
	SearchTimeMs int64
	TotalHits    int64
}

type StatsRequest struct {
	TimeRange *TimeRange
	GroupBy   string
	Metrics   []string
}

type StatsResult struct {
	TotalEvents  int64
	EventsByType map[string]int64
	Timeline     []TimelinePoint
	TopActors    []ActorStats
	ErrorRate    float64
	CacheStatus  string
}

type StreamRequest struct {
	Filters        map[string]interface{}
	ChunkSize      int
	Format         ExportFormat
}

type StreamResult struct {
	ID             uuid.UUID
	Status         string
	EstimatedTotal int64
}

type ExportRequest struct {
	ReportType ReportType
	Options    ExportOptions
	RequestID  string
}

type ExportResult struct {
	ID          uuid.UUID
	Status      string
	Format      ExportFormat
	Size        int64
	RecordCount int64
	GeneratedAt time.Time
	ExpiresAt   *time.Time
	DownloadURL string
	Checksum    string
	Metadata    map[string]interface{}
}

type StreamingExportRequest struct {
	ReportType ReportType
	Options    ExportOptions
	ChunkSize  int
}

type StreamingResult struct {
	ID             uuid.UUID
	Status         string
	EstimatedTotal int64
	ChunkSize      int
}

type GDPRRequest struct {
	SubjectID    string
	TimeRange    *TimeRange
	IncludePII   bool
	ExportFormat ExportFormat
}

type GDPRResult struct {
	ID              uuid.UUID
	GeneratedAt     time.Time
	DataPoints      []interface{}
	ProcessingBases []string
	RetentionPolicy map[string]interface{}
	RightsExercised []interface{}
	ConsentHistory  []interface{}
	DataTransfers   []interface{}
	Metadata        map[string]interface{}
}

type TCPARequest struct {
	PhoneNumber string
	TimeRange   *TimeRange
	Detailed    bool
}

type TCPAResult struct {
	ID                uuid.UUID
	GeneratedAt       time.Time
	ConsentStatus     interface{}
	CallHistory       []interface{}
	ViolationHistory  []interface{}
	OptOutHistory     []interface{}
	CallingTimeChecks []interface{}
	Metadata          map[string]interface{}
}

type IntegrityCheckRequest struct {
	CheckType string
	TimeRange *TimeRange
	Deep      bool
}

type IntegrityResult struct {
	ID              uuid.UUID
	Status          string
	StartedAt       time.Time
	CompletedAt     *time.Time
	EventsChecked   int64
	IssuesFound     int64
	IntegrityScore  float64
	Issues          []interface{}
	Recommendations []string
	Metadata        map[string]interface{}
}

// Additional types

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type ExportFormat string

const (
	ExportFormatJSON    ExportFormat = "json"
	ExportFormatCSV     ExportFormat = "csv"
	ExportFormatParquet ExportFormat = "parquet"
)

type ReportType string

const (
	ReportTypeGDPR          ReportType = "gdpr_data_subject"
	ReportTypeTCPA          ReportType = "tcpa_consent_trail"
	ReportTypeSOX           ReportType = "sox_financial_audit"
	ReportTypeSecurityAudit ReportType = "security_incident"
	ReportTypeCustom        ReportType = "custom_query"
)

type ExportOptions struct {
	Format          ExportFormat
	ReportType      ReportType
	RedactPII       bool
	IncludeMetadata bool
	ChunkSize       int
	Filters         map[string]interface{}
	TimeRange       *TimeRange
	CustomTemplate  string
}