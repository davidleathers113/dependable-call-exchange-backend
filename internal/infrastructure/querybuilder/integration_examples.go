package querybuilder

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Examples of how to integrate the QueryBuilder with existing repository patterns
// These examples demonstrate replacing handcrafted SQL with type-safe query builders

// Repository interface that repositories can implement to use the query builder
type QueryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Enhanced repository examples using QueryBuilder

// ExampleAccountRepository demonstrates QueryBuilder integration
type ExampleAccountRepository struct {
	db QueryExecutor
}

// GetActiveBuyers replaces handcrafted SQL with type-safe QueryBuilder
func (r *ExampleAccountRepository) GetActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error) {
	// Old approach: handcrafted SQL
	// query := `SELECT id, email, name... FROM accounts WHERE type = 'buyer' AND status = 'active' ORDER BY quality_score DESC LIMIT $1`

	// New approach: type-safe QueryBuilder
	sql, params, err := NewAccountQuery().
		SelectAccounts().
		WhereActiveBuyers().
		OrderByQualityScore(true). // desc = true
		Limit(limit).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Scanning logic remains the same
	var accounts []*account.Account
	// ... scanning implementation
	return accounts, nil
}

// GetHighQualityAccounts demonstrates complex filtering
func (r *ExampleAccountRepository) GetHighQualityAccounts(ctx context.Context, qualityThreshold, fraudThreshold float64) ([]*account.Account, error) {
	sql, params, err := NewAccountQuery().
		SelectAccounts().
		WhereAccountStatus(account.StatusActive).
		WhereQualityScoreAbove(qualityThreshold).
		WhereFraudScoreBelow(fraudThreshold).
		OrderByQualityScore(true).
		Limit(50).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Implementation continues...
	return nil, nil
}

// UpdateAccountStatus demonstrates UPDATE with QueryBuilder
func (r *ExampleAccountRepository) UpdateAccountStatus(ctx context.Context, accountID uuid.UUID, status account.Status) error {
	sql, params, err := New().
		Update("accounts").
		Set("status", status.String()).
		SetUpdatedAt(time.Now()).
		WhereID(accountID).
		Returning("id", "updated_at").
		ToSQL()

	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	var updatedID uuid.UUID
	var updatedAt time.Time
	err = r.db.QueryRowContext(ctx, sql, params...).Scan(&updatedID, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to update account status: %w", err)
	}

	return nil
}

// ExampleBidRepository demonstrates bid-specific queries
type ExampleBidRepository struct {
	db QueryExecutor
}

// GetActiveBidsForCall replaces handcrafted SQL
func (r *ExampleBidRepository) GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	// Old approach: handcrafted SQL with manual parameter management
	// query := `SELECT ... FROM bids WHERE call_id = $1 AND status IN ('active', 'winning') AND expires_at > NOW() ORDER BY amount DESC, placed_at ASC`

	// New approach: type-safe and readable
	sql, params, err := NewBidQuery().
		SelectBids().
		WhereCallID(callID).
		WhereActiveBids().
		WhereNotExpired().
		OrderByAmount(true).    // highest first
		OrderByPlacedAt(false). // earliest first for tie-breaking
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Scanning logic remains the same
	var bids []*bid.Bid
	// ... scanning implementation
	return bids, nil
}

// GetTopBiddersAnalytics demonstrates analytics queries
func (r *ExampleBidRepository) GetTopBiddersAnalytics(ctx context.Context, days, limit int) ([]BidderStats, error) {
	sql, params, err := NewBidAnalytics().
		TopBuyersByVolume(limit).
		Where("created_at", GreaterThanOrEqual, time.Now().AddDate(0, 0, -days)).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build analytics query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute analytics query: %w", err)
	}
	defer rows.Close()

	var stats []BidderStats
	for rows.Next() {
		var stat BidderStats
		err := rows.Scan(&stat.BuyerID, &stat.BidCount, &stat.WonCount, &stat.AvgBid, &stat.TotalAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bidder stats: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// BidderStats represents analytics data
type BidderStats struct {
	BuyerID     uuid.UUID
	BidCount    int
	WonCount    int
	AvgBid      float64
	TotalAmount float64
}

// ExampleCallRepository demonstrates call-specific queries
type ExampleCallRepository struct {
	db QueryExecutor
}

// GetPendingSellerCalls demonstrates filtering with business logic
func (r *ExampleCallRepository) GetPendingSellerCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	callQuery := NewCallQuery().
		SelectCalls().
		WherePendingCalls().
		WhereNotNull("seller_id").     // Must have a seller
		Where("buyer_id", IsNull, nil) // Not yet assigned to buyer

	sql, params, err := callQuery.
		OrderByStartedAt(false). // Oldest first (FIFO)
		Limit(limit).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Scanning logic
	var calls []*call.Call
	// ... implementation
	return calls, nil
}

// GetCallPerformanceReport demonstrates complex analytics
func (r *ExampleCallRepository) GetCallPerformanceReport(ctx context.Context, sellerID uuid.UUID, days int) (*CallPerformanceReport, error) {
	sql, params, err := NewCallAnalytics().
		SellerPerformance(days).
		WhereEqual("seller_id", sellerID).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build analytics query: %w", err)
	}

	var report CallPerformanceReport
	err = r.db.QueryRowContext(ctx, sql, params...).Scan(
		&report.SellerID,
		&report.TotalCalls,
		&report.CompletedCalls,
		&report.AvgDuration,
		&report.TotalRevenue,
		&report.CompletionRate,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get performance report: %w", err)
	}

	return &report, nil
}

// CallPerformanceReport represents call analytics
type CallPerformanceReport struct {
	SellerID       uuid.UUID
	TotalCalls     int
	CompletedCalls int
	AvgDuration    float64
	TotalRevenue   float64
	CompletionRate float64
}

// Transaction example with QueryBuilder
func (r *ExampleAccountRepository) TransferBalance(ctx context.Context, fromID, toID uuid.UUID, amount float64) error {
	// This would typically be wrapped in a transaction

	// Note: In a real implementation, balance updates would need proper Money value object handling
	// This is simplified for demonstration purposes

	// Debit from source account - simplified approach
	debitSQL, debitParams, err := New().
		Update("accounts").
		Set("updated_at", time.Now()).
		WhereID(fromID).
		Where("(balance->>'amount')::float", GreaterThanOrEqual, amount). // Ensure sufficient balance
		Returning("id").
		ToSQL()

	if err != nil {
		return fmt.Errorf("failed to build debit query: %w", err)
	}

	// Credit to destination account - simplified approach
	creditSQL, creditParams, err := New().
		Update("accounts").
		Set("updated_at", time.Now()).
		WhereID(toID).
		Returning("id").
		ToSQL()

	if err != nil {
		return fmt.Errorf("failed to build credit query: %w", err)
	}

	// Execute in transaction (implementation depends on your transaction pattern)
	var debitedID, creditedID uuid.UUID
	err = r.db.QueryRowContext(ctx, debitSQL, debitParams...).Scan(&debitedID)
	if err != nil {
		return fmt.Errorf("failed to debit account: %w", err)
	}

	err = r.db.QueryRowContext(ctx, creditSQL, creditParams...).Scan(&creditedID)
	if err != nil {
		return fmt.Errorf("failed to credit account: %w", err)
	}

	return nil
}

// Bulk operations example
func (r *ExampleBidRepository) MarkExpiredBids(ctx context.Context, cutoffTime time.Time) (int, error) {
	sql, params, err := New().
		Update("bids").
		Set("status", "expired").
		SetUpdatedAt(time.Now()).
		Where("expires_at", LessThan, cutoffTime).
		WhereIn("status", []interface{}{"pending", "active", "winning"}).
		Returning("id").
		ToSQL()

	if err != nil {
		return 0, fmt.Errorf("failed to build bulk update query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute bulk update: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return count, fmt.Errorf("failed to scan expired bid ID: %w", err)
		}
		count++
	}

	return count, nil
}

// Complex JOIN example with QueryBuilder
func (r *ExampleBidRepository) GetBidsWithAccountInfo(ctx context.Context, callID uuid.UUID) ([]BidWithAccount, error) {
	sql, params, err := New().
		Select(
			"b.id", "b.amount", "b.status", "b.placed_at",
			"a.name", "a.email", "a.quality_metrics",
		).
		From("bids b").
		InnerJoin("accounts a", "a.id = b.buyer_id").
		WhereEqual("b.call_id", callID).
		WhereEqual("a.status", "active").
		OrderByDesc("b.amount").
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build join query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute join query: %w", err)
	}
	defer rows.Close()

	var results []BidWithAccount
	for rows.Next() {
		var result BidWithAccount
		err := rows.Scan(
			&result.BidID, &result.Amount, &result.Status, &result.PlacedAt,
			&result.BuyerName, &result.BuyerEmail, &result.QualityMetrics,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid with account: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// BidWithAccount represents joined data
type BidWithAccount struct {
	BidID          uuid.UUID
	Amount         float64
	Status         string
	PlacedAt       time.Time
	BuyerName      string
	BuyerEmail     string
	QualityMetrics string // JSON
}

// Migration examples showing old vs new patterns

// OLD PATTERN: Handcrafted SQL with manual parameter management
/*
func (r *oldRepository) getActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error) {
	query := `
		SELECT
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			last_login_at, created_at, updated_at
		FROM accounts
		WHERE type = 'buyer' AND status = 'active'
		ORDER BY quality_score DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	// ... error-prone parameter management and scanning
}
*/

// NEW PATTERN: Type-safe QueryBuilder
/*
func (r *newRepository) getActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error) {
	sql, params, err := NewAccountQuery().
		SelectAccounts().
		WhereActiveBuyers().
		OrderByQualityScore(true).
		Limit(limit).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("query build error: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, params...)
	// ... same scanning logic, but with guaranteed parameter safety
}
*/
