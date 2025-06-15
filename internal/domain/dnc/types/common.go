package types

// ComplianceFlag represents regulatory compliance requirements
type ComplianceFlag struct {
    TCPA  bool
    GDPR  bool
    SOX   bool
    CCPA  bool
}

// CheckContext provides context for DNC checks
type CheckContext struct {
    RequestID     string
    UserID        string
    CallerTimeZone string
    CallTime      string
}

// PerformanceMetrics tracks DNC operation performance
type PerformanceMetrics struct {
    LatencyMs     int
    CacheHits     int
    SourceCount   int
    ErrorCount    int
}

