package compliance

// TCPARestrictions represents TCPA time-based calling restrictions
type TCPARestrictions struct {
	StartTime string
	EndTime   string
	TimeZone  string
}
