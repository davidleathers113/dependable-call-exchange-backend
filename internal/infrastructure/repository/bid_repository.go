package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database/adapters"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/google/uuid"
)

// bidRepository implements BidRepository using PostgreSQL
type bidRepository struct {
	db interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	}
	adapters *adapters.Adapters
}

// NewBidRepository creates a new bid repository
func NewBidRepository(db *sql.DB) bidding.BidRepository {
	return &bidRepository{
		db:       db,
		adapters: adapters.NewAdapters(),
	}
}

// NewBidRepositoryWithTx creates a new bid repository with a transaction
func NewBidRepositoryWithTx(tx *sql.Tx) bidding.BidRepository {
	return &bidRepository{
		db:       tx,
		adapters: adapters.NewAdapters(),
	}
}

// Create stores a new bid
func (r *bidRepository) Create(ctx context.Context, b *bid.Bid) error {
	// Validate required fields
	if b.CallID == uuid.Nil {
		return errors.New("call_id cannot be nil")
	}
	if b.BuyerID == uuid.Nil {
		return errors.New("buyer_id cannot be nil")
	}
	if b.Amount.IsZero() || b.Amount.IsNegative() {
		return errors.New("amount must be positive")
	}

	// Serialize complex types to JSON
	criteriaJSON, err := json.Marshal(b.Criteria)
	if err != nil {
		return fmt.Errorf("failed to marshal criteria: %w", err)
	}

	qualityJSON, err := json.Marshal(b.Quality)
	if err != nil {
		return fmt.Errorf("failed to marshal quality metrics: %w", err)
	}

	query := `
		INSERT INTO bids (
			id, call_id, buyer_id, seller_id, amount, status,
			auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
	`

	// Handle optional seller_id
	var sellerID interface{}
	if b.SellerID != uuid.Nil {
		sellerID = b.SellerID
	} else {
		sellerID = nil
	}

	// Handle optional auction_id
	var auctionID interface{}
	if b.AuctionID != uuid.Nil {
		auctionID = b.AuctionID
	} else {
		auctionID = nil
	}

	_, err = r.db.ExecContext(ctx, query,
		b.ID, b.CallID, b.BuyerID, sellerID, r.adapters.Money.ValueAsFloat64(b.Amount), b.Status.String(),
		auctionID, b.Rank, criteriaJSON, qualityJSON,
		b.PlacedAt, b.ExpiresAt, b.AcceptedAt, b.CreatedAt, b.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}

	return nil
}

// GetByID retrieves a bid by ID
func (r *bidRepository) GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount, status,
			auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE id = $1
	`

	var b bid.Bid
	var statusStr string
	var criteriaJSON, qualityJSON []byte
	var sellerID, auctionID sql.NullString
	var acceptedAt sql.NullTime
	var amountFloat float64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.CallID, &b.BuyerID, &sellerID, &amountFloat, &statusStr,
		&auctionID, &b.Rank, &criteriaJSON, &qualityJSON,
		&b.PlacedAt, &b.ExpiresAt, &acceptedAt, &b.CreatedAt, &b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("bid not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get bid: %w", err)
	}

	// Convert amount to Money value object using adapter
	if err := r.adapters.Money.ScanFromFloat64(&b.Amount, amountFloat, values.USD); err != nil {
		return nil, fmt.Errorf("failed to scan amount: %w", err)
	}

	// Convert database values
	b.Status = parseBidStatus(statusStr)

	if sellerID.Valid {
		id, _ := uuid.Parse(sellerID.String)
		b.SellerID = id
	}

	if auctionID.Valid {
		id, _ := uuid.Parse(auctionID.String)
		b.AuctionID = id
	}

	if acceptedAt.Valid {
		b.AcceptedAt = &acceptedAt.Time
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(criteriaJSON, &b.Criteria); err != nil {
		return nil, fmt.Errorf("failed to unmarshal criteria: %w", err)
	}

	if err := json.Unmarshal(qualityJSON, &b.Quality); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quality metrics: %w", err)
	}

	return &b, nil
}

// Update modifies an existing bid
func (r *bidRepository) Update(ctx context.Context, b *bid.Bid) error {
	// Serialize complex types
	criteriaJSON, err := json.Marshal(b.Criteria)
	if err != nil {
		return fmt.Errorf("failed to marshal criteria: %w", err)
	}

	qualityJSON, err := json.Marshal(b.Quality)
	if err != nil {
		return fmt.Errorf("failed to marshal quality metrics: %w", err)
	}

	query := `
		UPDATE bids
		SET 
			amount = $2,
			status = $3,
			rank = $4,
			criteria = $5,
			quality_metrics = $6,
			expires_at = $7,
			accepted_at = $8,
			updated_at = $9
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		b.ID, b.Amount.ToFloat64(), b.Status.String(), b.Rank,
		criteriaJSON, qualityJSON,
		b.ExpiresAt, b.AcceptedAt, b.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update bid: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("bid with ID %s not found", b.ID)
	}

	return nil
}

// Delete removes a bid
func (r *bidRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM bids WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete bid: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("bid with ID %s not found", id)
	}

	return nil
}

// GetActiveBidsForCall returns active bids for a call
func (r *bidRepository) GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount, status,
			auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE call_id = $1 
		AND status IN ('active', 'winning')
		AND expires_at > NOW()
		ORDER BY amount DESC, placed_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active bids: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		b, err := r.scanBid(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return bids, nil
}

// GetByBuyer returns bids by buyer
func (r *bidRepository) GetByBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount, status,
			auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE buyer_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query, buyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids by buyer: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		b, err := r.scanBid(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return bids, nil
}

// GetExpiredBids returns bids past expiration
func (r *bidRepository) GetExpiredBids(ctx context.Context, before time.Time) ([]*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount, status,
			auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE expires_at < $1
		AND status IN ('pending', 'active', 'winning')
		ORDER BY expires_at ASC
		LIMIT 1000
	`

	rows, err := r.db.QueryContext(ctx, query, before)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired bids: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		b, err := r.scanBid(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return bids, nil
}

// scanBid scans a database row into a Bid struct
func (r *bidRepository) scanBid(rows *sql.Rows) (*bid.Bid, error) {
	var b bid.Bid
	var statusStr string
	var criteriaJSON, qualityJSON []byte
	var sellerID, auctionID sql.NullString
	var acceptedAt sql.NullTime
	var amountFloat float64

	err := rows.Scan(
		&b.ID, &b.CallID, &b.BuyerID, &sellerID, &amountFloat, &statusStr,
		&auctionID, &b.Rank, &criteriaJSON, &qualityJSON,
		&b.PlacedAt, &b.ExpiresAt, &acceptedAt, &b.CreatedAt, &b.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert amount to Money value object using adapter
	if err := r.adapters.Money.ScanFromFloat64(&b.Amount, amountFloat, values.USD); err != nil {
		return nil, fmt.Errorf("failed to scan amount: %w", err)
	}

	// Convert database values
	b.Status = parseBidStatus(statusStr)

	if sellerID.Valid {
		id, _ := uuid.Parse(sellerID.String)
		b.SellerID = id
	}

	if auctionID.Valid {
		id, _ := uuid.Parse(auctionID.String)
		b.AuctionID = id
	}

	if acceptedAt.Valid {
		b.AcceptedAt = &acceptedAt.Time
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(criteriaJSON, &b.Criteria); err != nil {
		return nil, fmt.Errorf("failed to unmarshal criteria: %w", err)
	}

	if err := json.Unmarshal(qualityJSON, &b.Quality); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quality metrics: %w", err)
	}

	return &b, nil
}

// parseBidStatus converts string to bid.Status
func parseBidStatus(s string) bid.Status {
	switch s {
	case "pending":
		return bid.StatusPending
	case "active":
		return bid.StatusActive
	case "winning":
		return bid.StatusWinning
	case "won":
		return bid.StatusWon
	case "lost":
		return bid.StatusLost
	case "expired":
		return bid.StatusExpired
	case "canceled":
		return bid.StatusCanceled
	default:
		return bid.StatusPending
	}
}

// GetBidByID is an alias for GetByID to satisfy callrouting.BidRepository interface
func (r *bidRepository) GetBidByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	return r.GetByID(ctx, id)
}

// GetActiveBuyerBids returns all active bids for a specific buyer
func (r *bidRepository) GetActiveBuyerBids(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount,
			status, auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE buyer_id = $1 AND status = 'active'
		ORDER BY placed_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, buyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active buyer bids: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		b, err := r.scanBid(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return bids, nil
}

// GetWonBidsByBuyer returns won bids for a specific buyer
func (r *bidRepository) GetWonBidsByBuyer(ctx context.Context, buyerID uuid.UUID, limit int) ([]*bid.Bid, error) {
	query := `
		SELECT 
			id, call_id, buyer_id, seller_id, amount,
			status, auction_id, rank, criteria, quality_metrics,
			placed_at, expires_at, accepted_at, created_at, updated_at
		FROM bids
		WHERE buyer_id = $1 AND status = 'won'
		ORDER BY accepted_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, buyerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query won bids: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		b, err := r.scanBid(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return bids, nil
}
