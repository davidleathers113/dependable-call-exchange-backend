package querybuilder

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Domain-specific query builders that provide type-safe, semantic query construction
// These builders encapsulate common query patterns for each domain entity

// AccountQueryBuilder provides type-safe query building for Account entities
type AccountQueryBuilder struct {
	*QueryBuilder
}

// NewAccountQuery creates a new AccountQueryBuilder
func NewAccountQuery() *AccountQueryBuilder {
	return &AccountQueryBuilder{
		QueryBuilder: New(),
	}
}

// SelectAccounts starts a SELECT query for accounts
func (aqb *AccountQueryBuilder) SelectAccounts(columns ...string) *AccountQueryBuilder {
	if len(columns) == 0 {
		columns = []string{
			"id", "email", "name", "company", "type", "status", "phone_number",
			"balance", "credit_limit", "payment_terms", "tcpa_consent", "gdpr_consent",
			"compliance_flags", "quality_metrics", "settings", "last_login_at",
			"created_at", "updated_at",
		}
	}
	aqb.Select(columns...).From("accounts")
	return aqb
}

// WhereAccountType filters by account type
func (aqb *AccountQueryBuilder) WhereAccountType(accountType account.AccountType) *AccountQueryBuilder {
	aqb.WhereEqual("type", accountType.String())
	return aqb
}

// WhereAccountStatus filters by account status
func (aqb *AccountQueryBuilder) WhereAccountStatus(status account.Status) *AccountQueryBuilder {
	aqb.WhereEqual("status", status.String())
	return aqb
}

// WhereActiveBuyers filters for active buyer accounts
func (aqb *AccountQueryBuilder) WhereActiveBuyers() *AccountQueryBuilder {
	aqb.WhereEqual("type", "buyer").WhereEqual("status", "active")
	return aqb
}

// WhereActiveSellers filters for active seller accounts
func (aqb *AccountQueryBuilder) WhereActiveSellers() *AccountQueryBuilder {
	aqb.WhereEqual("type", "seller").WhereEqual("status", "active")
	return aqb
}

// WhereQualityScoreAbove filters accounts with quality score above threshold
func (aqb *AccountQueryBuilder) WhereQualityScoreAbove(threshold float64) *AccountQueryBuilder {
	aqb.Where("(quality_metrics->>'quality_score')::float", GreaterThan, threshold)
	return aqb
}

// WhereFraudScoreBelow filters accounts with fraud score below threshold
func (aqb *AccountQueryBuilder) WhereFraudScoreBelow(threshold float64) *AccountQueryBuilder {
	aqb.Where("(quality_metrics->>'fraud_score')::float", LessThan, threshold)
	return aqb
}

// WhereBalanceAbove filters accounts with balance above amount
func (aqb *AccountQueryBuilder) WhereBalanceAbove(amount float64) *AccountQueryBuilder {
	aqb.Where("(balance->>'amount')::float", GreaterThan, amount)
	return aqb
}

// WhereLastLoginBefore filters accounts last logged in before specified time
func (aqb *AccountQueryBuilder) WhereLastLoginBefore(before time.Time) *AccountQueryBuilder {
	aqb.Where("last_login_at", LessThan, before)
	return aqb
}

// OrderByQualityScore orders by quality score
func (aqb *AccountQueryBuilder) OrderByQualityScore(desc bool) *AccountQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	aqb.OrderBy("(quality_metrics->>'quality_score')::float", direction)
	return aqb
}

// OrderByBalance orders by account balance
func (aqb *AccountQueryBuilder) OrderByBalance(desc bool) *AccountQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	aqb.OrderBy("(balance->>'amount')::float", direction)
	return aqb
}

// BidQueryBuilder provides type-safe query building for Bid entities
type BidQueryBuilder struct {
	*QueryBuilder
}

// NewBidQuery creates a new BidQueryBuilder
func NewBidQuery() *BidQueryBuilder {
	return &BidQueryBuilder{
		QueryBuilder: New(),
	}
}

// SelectBids starts a SELECT query for bids
func (bqb *BidQueryBuilder) SelectBids(columns ...string) *BidQueryBuilder {
	if len(columns) == 0 {
		columns = []string{
			"id", "call_id", "buyer_id", "seller_id", "amount", "status",
			"auction_id", "rank", "criteria", "quality_metrics",
			"placed_at", "expires_at", "accepted_at", "created_at", "updated_at",
		}
	}
	bqb.Select(columns...).From("bids")
	return bqb
}

// WhereCallID filters bids for a specific call
func (bqb *BidQueryBuilder) WhereCallID(callID uuid.UUID) *BidQueryBuilder {
	bqb.WhereEqual("call_id", callID)
	return bqb
}

// WhereBuyerID filters bids by buyer
func (bqb *BidQueryBuilder) WhereBuyerID(buyerID uuid.UUID) *BidQueryBuilder {
	bqb.WhereEqual("buyer_id", buyerID)
	return bqb
}

// WhereSellerID filters bids by seller
func (bqb *BidQueryBuilder) WhereSellerID(sellerID uuid.UUID) *BidQueryBuilder {
	bqb.WhereEqual("seller_id", sellerID)
	return bqb
}

// WhereBidStatus filters by bid status
func (bqb *BidQueryBuilder) WhereBidStatus(status bid.Status) *BidQueryBuilder {
	bqb.WhereEqual("status", status.String())
	return bqb
}

// WhereActiveBids filters for active bids
func (bqb *BidQueryBuilder) WhereActiveBids() *BidQueryBuilder {
	bqb.WhereIn("status", []interface{}{"active", "winning"})
	return bqb
}

// WhereNotExpired filters out expired bids
func (bqb *BidQueryBuilder) WhereNotExpired() *BidQueryBuilder {
	bqb.Where("expires_at", GreaterThan, time.Now())
	return bqb
}

// WhereExpiredBefore filters bids expired before a specific time
func (bqb *BidQueryBuilder) WhereExpiredBefore(before time.Time) *BidQueryBuilder {
	bqb.Where("expires_at", LessThan, before)
	return bqb
}

// WhereAmountAbove filters bids with amount above threshold
func (bqb *BidQueryBuilder) WhereAmountAbove(amount float64) *BidQueryBuilder {
	bqb.Where("(amount->>'amount')::float", GreaterThan, amount)
	return bqb
}

// WhereQualityScoreAbove filters bids with quality score above threshold
func (bqb *BidQueryBuilder) WhereQualityScoreAbove(threshold float64) *BidQueryBuilder {
	bqb.Where("(quality_metrics->>'quality_score')::float", GreaterThan, threshold)
	return bqb
}

// OrderByAmount orders by bid amount
func (bqb *BidQueryBuilder) OrderByAmount(desc bool) *BidQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	bqb.OrderBy("(amount->>'amount')::float", direction)
	return bqb
}

// OrderByPlacedAt orders by placement time
func (bqb *BidQueryBuilder) OrderByPlacedAt(desc bool) *BidQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	bqb.OrderBy("placed_at", direction)
	return bqb
}

// OrderByRank orders by bid rank (auction ranking)
func (bqb *BidQueryBuilder) OrderByRank(desc bool) *BidQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	bqb.OrderBy("rank", direction)
	return bqb
}

// CallQueryBuilder provides type-safe query building for Call entities
type CallQueryBuilder struct {
	*QueryBuilder
}

// NewCallQuery creates a new CallQueryBuilder
func NewCallQuery() *CallQueryBuilder {
	return &CallQueryBuilder{
		QueryBuilder: New(),
	}
}

// SelectCalls starts a SELECT query for calls
func (cqb *CallQueryBuilder) SelectCalls(columns ...string) *CallQueryBuilder {
	if len(columns) == 0 {
		columns = []string{
			"id", "from_number", "to_number", "status", "type",
			"buyer_id", "seller_id", "started_at", "ended_at",
			"duration", "cost", "metadata", "created_at", "updated_at",
		}
	}
	cqb.Select(columns...).From("calls")
	return cqb
}

// WhereBuyerID filters calls by buyer
func (cqb *CallQueryBuilder) WhereBuyerID(buyerID uuid.UUID) *CallQueryBuilder {
	cqb.WhereEqual("buyer_id", buyerID)
	return cqb
}

// WhereSellerID filters calls by seller
func (cqb *CallQueryBuilder) WhereSellerID(sellerID uuid.UUID) *CallQueryBuilder {
	cqb.WhereEqual("seller_id", sellerID)
	return cqb
}

// WhereCallStatus filters by call status
func (cqb *CallQueryBuilder) WhereCallStatus(status call.Status) *CallQueryBuilder {
	statusStr := mapCallStatusToString(status)
	cqb.WhereEqual("status", statusStr)
	return cqb
}

// WhereActiveCalls filters for active calls (pending, queued, ringing, in_progress)
func (cqb *CallQueryBuilder) WhereActiveCalls() *CallQueryBuilder {
	cqb.WhereIn("status", []interface{}{"pending", "queued", "ringing", "in_progress"})
	return cqb
}

// WherePendingCalls filters for pending calls awaiting routing
func (cqb *CallQueryBuilder) WherePendingCalls() *CallQueryBuilder {
	cqb.WhereEqual("status", "pending")
	return cqb
}

// WhereCallType filters by call type (inbound/outbound)
func (cqb *CallQueryBuilder) WhereCallType(callType string) *CallQueryBuilder {
	cqb.WhereEqual("type", callType)
	return cqb
}

// WhereStartedAfter filters calls started after specific time
func (cqb *CallQueryBuilder) WhereStartedAfter(after time.Time) *CallQueryBuilder {
	cqb.Where("started_at", GreaterThan, after)
	return cqb
}

// WhereStartedBefore filters calls started before specific time
func (cqb *CallQueryBuilder) WhereStartedBefore(before time.Time) *CallQueryBuilder {
	cqb.Where("started_at", LessThan, before)
	return cqb
}

// WhereDurationAbove filters calls with duration above threshold (in seconds)
func (cqb *CallQueryBuilder) WhereDurationAbove(seconds int) *CallQueryBuilder {
	cqb.Where("duration", GreaterThan, seconds)
	return cqb
}

// WhereCostAbove filters calls with cost above threshold
func (cqb *CallQueryBuilder) WhereCostAbove(amount float64) *CallQueryBuilder {
	cqb.Where("cost", GreaterThan, amount)
	return cqb
}

// WhereUnassigned filters calls not yet assigned to a buyer
func (cqb *CallQueryBuilder) WhereUnassigned() *CallQueryBuilder {
	cqb.Where("buyer_id", IsNull, nil)
	return cqb
}

// OrderByStartedAt orders by call start time
func (cqb *CallQueryBuilder) OrderByStartedAt(desc bool) *CallQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	cqb.OrderBy("started_at", direction)
	return cqb
}

// OrderByDuration orders by call duration
func (cqb *CallQueryBuilder) OrderByDuration(desc bool) *CallQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	cqb.OrderBy("duration", direction)
	return cqb
}

// OrderByCost orders by call cost
func (cqb *CallQueryBuilder) OrderByCost(desc bool) *CallQueryBuilder {
	direction := Asc
	if desc {
		direction = Desc
	}
	cqb.OrderBy("cost", direction)
	return cqb
}

// Additional wrapper methods for CallQueryBuilder to maintain type safety

// Where adds a WHERE condition with AND logic for CallQueryBuilder
func (cqb *CallQueryBuilder) Where(column string, operator Operator, value interface{}) *CallQueryBuilder {
	cqb.QueryBuilder.addCondition(column, operator, value, And)
	return cqb
}

// WhereNotNull adds an IS NOT NULL condition for CallQueryBuilder
func (cqb *CallQueryBuilder) WhereNotNull(column string) *CallQueryBuilder {
	cqb.QueryBuilder.Where(column, IsNotNull, nil)
	return cqb
}

// Limit sets the LIMIT clause for CallQueryBuilder
func (cqb *CallQueryBuilder) Limit(limit int) *CallQueryBuilder {
	cqb.QueryBuilder.Limit(limit)
	return cqb
}

// Helper function to map call status to database string
func mapCallStatusToString(status call.Status) string {
	switch status {
	case call.StatusPending:
		return "pending"
	case call.StatusQueued:
		return "queued"
	case call.StatusRinging:
		return "ringing"
	case call.StatusInProgress:
		return "in_progress"
	case call.StatusCompleted:
		return "completed"
	case call.StatusFailed:
		return "failed"
	case call.StatusCanceled:
		return "failed" // Map to failed in DB
	case call.StatusNoAnswer:
		return "no_answer"
	case call.StatusBusy:
		return "no_answer" // Map busy to no_answer in DB
	default:
		return "pending"
	}
}

// Aggregate query builders for common analytics patterns

// BidAnalyticsBuilder provides analytics queries for bidding metrics
type BidAnalyticsBuilder struct {
	*QueryBuilder
}

// NewBidAnalytics creates a new BidAnalyticsBuilder
func NewBidAnalytics() *BidAnalyticsBuilder {
	return &BidAnalyticsBuilder{
		QueryBuilder: New(),
	}
}

// BidVolumeByDay returns daily bid volume
func (bab *BidAnalyticsBuilder) BidVolumeByDay(days int) *BidAnalyticsBuilder {
	bab.Select(
		"DATE(created_at) as date",
		"COUNT(*) as bid_count",
		"AVG((amount->>'amount')::float) as avg_amount",
		"SUM((amount->>'amount')::float) as total_amount",
	).
		From("bids").
		Where("created_at", GreaterThanOrEqual, time.Now().AddDate(0, 0, -days)).
		GroupBy("DATE(created_at)").
		OrderByAsc("date")
	return bab
}

// TopBuyersByVolume returns buyers ranked by bid volume
func (bab *BidAnalyticsBuilder) TopBuyersByVolume(limit int) *BidAnalyticsBuilder {
	bab.Select(
		"buyer_id",
		"COUNT(*) as bid_count",
		"COUNT(CASE WHEN status = 'won' THEN 1 END) as won_count",
		"AVG((amount->>'amount')::float) as avg_bid",
		"SUM((amount->>'amount')::float) as total_bid_amount",
	).
		From("bids").
		GroupBy("buyer_id").
		OrderByDesc("total_bid_amount").
		Limit(limit)
	return bab
}

// CallAnalyticsBuilder provides analytics queries for call metrics
type CallAnalyticsBuilder struct {
	*QueryBuilder
}

// NewCallAnalytics creates a new CallAnalyticsBuilder
func NewCallAnalytics() *CallAnalyticsBuilder {
	return &CallAnalyticsBuilder{
		QueryBuilder: New(),
	}
}

// CallVolumeByHour returns hourly call volume for the last 24 hours
func (cab *CallAnalyticsBuilder) CallVolumeByHour() *CallAnalyticsBuilder {
	cab.Select(
		"EXTRACT(hour FROM started_at) as hour",
		"COUNT(*) as call_count",
		"COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count",
		"AVG(duration) as avg_duration",
	).
		From("calls").
		Where("started_at", GreaterThanOrEqual, time.Now().Add(-24*time.Hour)).
		GroupBy("EXTRACT(hour FROM started_at)").
		OrderByAsc("hour")
	return cab
}

// SellerPerformance returns seller performance metrics
func (cab *CallAnalyticsBuilder) SellerPerformance(days int) *CallAnalyticsBuilder {
	cab.Select(
		"seller_id",
		"COUNT(*) as total_calls",
		"COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_calls",
		"AVG(duration) as avg_duration",
		"SUM(cost) as total_revenue",
		"(COUNT(CASE WHEN status = 'completed' THEN 1 END)::float / COUNT(*)::float) as completion_rate",
	).
		From("calls").
		WhereNotNull("seller_id").
		Where("started_at", GreaterThanOrEqual, time.Now().AddDate(0, 0, -days)).
		GroupBy("seller_id").
		OrderByDesc("total_revenue")
	return cab
}
