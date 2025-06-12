package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// callRepository implements CallRepository using PostgreSQL
type callRepository struct {
	db interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	}
}

// NewCallRepository creates a new call repository
func NewCallRepository(db *sql.DB) CallRepository {
	return &callRepository{db: db}
}

// NewCallRepositoryWithTx creates a new call repository with a transaction
func NewCallRepositoryWithTx(tx *sql.Tx) CallRepository {
	return &callRepository{db: tx}
}

// Create inserts a new call into the database
func (r *callRepository) Create(ctx context.Context, c *call.Call) error {
	// Validate required fields
	if c.FromNumber.IsEmpty() || c.ToNumber.IsEmpty() {
		return errors.New("from_number cannot be empty")
	}

	// Note: BuyerID can be nil for marketplace calls awaiting routing

	// Convert direction to string
	directionStr := c.Direction.String()

	// Map status to database enum
	statusStr := mapStatusToEnum(c.Status)

	query := `
		INSERT INTO calls (
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14
		)
	`

	metadataJSON, err := SerializeCallMetadata(c)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Handle nil buyer ID for marketplace calls
	var buyerID any = c.BuyerID
	if c.BuyerID == uuid.Nil {
		buyerID = nil
	}

	// Handle cost conversion
	var cost any
	if c.Cost != nil {
		cost = c.Cost.ToFloat64()
	}

	_, err = r.db.ExecContext(ctx, query,
		c.ID, c.FromNumber.String(), c.ToNumber.String(), statusStr, directionStr,
		buyerID, c.SellerID, c.StartTime, c.EndTime,
		c.Duration, cost, metadataJSON, c.CreatedAt, c.UpdatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return fmt.Errorf("duplicate key: call with ID %s already exists", c.ID)
		}
		return fmt.Errorf("failed to create call: %w", err)
	}

	return nil
}

// GetByID retrieves a call by its ID
func (r *callRepository) GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error) {
	query := `
		SELECT 
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls
		WHERE id = $1
	`

	var c call.Call
	var statusStr, directionStr string
	var metadata []byte
	var buyerIDStr sql.NullString
	var sellerID sql.NullString
	var endTime sql.NullTime
	var duration sql.NullInt32
	var cost sql.NullFloat64
	var fromNumberStr, toNumberStr string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &fromNumberStr, &toNumberStr, &statusStr, &directionStr,
		&buyerIDStr, &sellerID, &c.StartTime, &endTime,
		&duration, &cost, &metadata, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("call not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get call: %w", err)
	}

	// Convert phone numbers to value objects
	fromPhone, err := values.NewPhoneNumber(fromNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid from_number from database: %w", err)
	}
	c.FromNumber = fromPhone

	toPhone, err := values.NewPhoneNumber(toNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid to_number from database: %w", err)
	}
	c.ToNumber = toPhone

	// Convert database values
	c.Status = mapEnumToStatus(statusStr)
	c.Direction = mapTypeToDirection(directionStr)

	// Handle nullable buyer ID (marketplace calls may not have a buyer yet)
	if buyerIDStr.Valid {
		id, _ := uuid.Parse(buyerIDStr.String)
		c.BuyerID = id
	} else {
		c.BuyerID = uuid.Nil
	}

	if sellerID.Valid {
		id, _ := uuid.Parse(sellerID.String)
		c.SellerID = &id
	}

	if endTime.Valid {
		c.EndTime = &endTime.Time
	}

	if duration.Valid {
		d := int(duration.Int32)
		c.Duration = &d
	}

	if cost.Valid {
		money, _ := values.NewMoneyFromFloat(cost.Float64, "USD")
		c.Cost = &money
	}

	// Parse metadata
	parsedMetadata, err := ParseCallMetadata(metadata)
	if err == nil {
		parsedMetadata.ApplyToCall(&c)
	}

	return &c, nil
}

// GetByCallSID retrieves a call by its provider call SID
func (r *callRepository) GetByCallSID(ctx context.Context, callSID string) (*call.Call, error) {
	// For now, we'll search in the metadata for the call_sid
	// In a production system, you might want to add a dedicated call_sid column
	query := `
		SELECT 
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls
		WHERE metadata::jsonb ->> 'call_sid' = $1
	`

	var c call.Call
	var statusStr, directionStr string
	var metadata []byte
	var buyerIDStr sql.NullString
	var sellerID sql.NullString
	var fromNumberStr, toNumberStr string
	var endTime sql.NullTime
	var duration sql.NullInt32
	var cost sql.NullFloat64

	err := r.db.QueryRowContext(ctx, query, callSID).Scan(
		&c.ID, &fromNumberStr, &toNumberStr, &statusStr, &directionStr,
		&buyerIDStr, &sellerID, &c.StartTime, &endTime,
		&duration, &cost, &metadata, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("call not found with SID %s: %w", callSID, err)
		}
		return nil, fmt.Errorf("failed to get call by SID: %w", err)
	}

	// Convert phone numbers to value objects
	fromPhone, err := values.NewPhoneNumber(fromNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid from_number from database: %w", err)
	}
	c.FromNumber = fromPhone

	toPhone, err := values.NewPhoneNumber(toNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid to_number from database: %w", err)
	}
	c.ToNumber = toPhone

	// Convert database values
	c.Status = mapEnumToStatus(statusStr)
	c.Direction = mapTypeToDirection(directionStr)

	// Handle nullable buyer ID
	if buyerIDStr.Valid {
		id, _ := uuid.Parse(buyerIDStr.String)
		c.BuyerID = id
	} else {
		c.BuyerID = uuid.Nil
	}

	if sellerID.Valid {
		id, _ := uuid.Parse(sellerID.String)
		c.SellerID = &id
	}

	if endTime.Valid {
		c.EndTime = &endTime.Time
	}

	if duration.Valid {
		d := int(duration.Int32)
		c.Duration = &d
	}

	if cost.Valid {
		money, _ := values.NewMoneyFromFloat(cost.Float64, "USD")
		c.Cost = &money
	}

	// Parse metadata
	parsedMetadata, err := ParseCallMetadata(metadata)
	if err == nil {
		parsedMetadata.ApplyToCall(&c)
	}

	return &c, nil
}

// UpdateWithStatusCheck updates a call only if it has the expected status
// This is used for concurrent-safe status transitions
func (r *callRepository) UpdateWithStatusCheck(ctx context.Context, c *call.Call, expectedStatus call.Status) error {
	// Map status to database enum
	statusStr := mapStatusToEnum(c.Status)
	expectedStatusStr := mapStatusToEnum(expectedStatus)

	// Update metadata
	metadataJSON, err := SerializeCallMetadata(c)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Handle nil buyer ID for marketplace calls
	var buyerID any = c.BuyerID
	if c.BuyerID == uuid.Nil {
		buyerID = nil
	}

	// Handle seller ID
	var sellerID any
	if c.SellerID != nil {
		sellerID = *c.SellerID
	}

	// Handle cost conversion
	var cost any
	if c.Cost != nil {
		cost = c.Cost.ToFloat64()
	}

	query := `
		UPDATE calls
		SET 
			status = $2,
			buyer_id = $3,
			seller_id = $4,
			ended_at = $5,
			duration = $6,
			cost = $7,
			metadata = $8,
			updated_at = $9
		WHERE id = $1 AND status = $10
	`

	result, err := r.db.ExecContext(ctx, query,
		c.ID, statusStr, buyerID, sellerID, c.EndTime, c.Duration, cost,
		metadataJSON, c.UpdatedAt, expectedStatusStr,
	)

	if err != nil {
		return fmt.Errorf("failed to update call: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// The call status has changed, indicating another process updated it
		return fmt.Errorf("call status has changed, expected %s", expectedStatus)
	}

	return nil
}

// Update updates an existing call
func (r *callRepository) Update(ctx context.Context, c *call.Call) error {
	// Map status to database enum
	statusStr := mapStatusToEnum(c.Status)

	// Update metadata
	metadataJSON, err := SerializeCallMetadata(c)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Handle nil buyer ID for marketplace calls
	var buyerID any = c.BuyerID
	if c.BuyerID == uuid.Nil {
		buyerID = nil
	}

	// Handle seller ID
	var sellerID any
	if c.SellerID != nil {
		sellerID = *c.SellerID
	}

	// Handle cost conversion
	var cost any
	if c.Cost != nil {
		cost = c.Cost.ToFloat64()
	}

	query := `
		UPDATE calls
		SET 
			status = $2,
			buyer_id = $3,
			seller_id = $4,
			ended_at = $5,
			duration = $6,
			cost = $7,
			metadata = $8,
			updated_at = $9
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		c.ID, statusStr, buyerID, sellerID, c.EndTime, c.Duration, cost,
		metadataJSON, c.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update call: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("call with ID %s not found", c.ID)
	}

	return nil
}

// Delete removes a call from the database
func (r *callRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM calls WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete call: %w", err)
	}

	return nil
}

// List returns a list of calls based on filter criteria
func (r *callRepository) List(ctx context.Context, filter CallFilter) ([]*call.Call, error) {
	var conditions []string
	var args []any
	argCount := 0

	// Build WHERE conditions
	if filter.Status != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, mapStatusToEnum(*filter.Status))
	}

	if filter.BuyerID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("buyer_id = $%d", argCount))
		args = append(args, *filter.BuyerID)
	}

	if filter.SellerID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("seller_id = $%d", argCount))
		args = append(args, *filter.SellerID)
	}

	if filter.StartTimeFrom != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("started_at >= $%d", argCount))
		args = append(args, *filter.StartTimeFrom)
	}

	if filter.StartTimeTo != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("started_at <= $%d", argCount))
		args = append(args, *filter.StartTimeTo)
	}

	// Build query
	query := `
		SELECT 
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls
	`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ordering with SQL injection protection
	orderByClause := sanitizeOrderBy(filter.OrderBy)
	query += " ORDER BY " + orderByClause

	// Add pagination
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list calls: %w", err)
	}
	defer rows.Close()

	var calls []*call.Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		calls = append(calls, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return calls, nil
}

// CountByStatus returns the count of calls grouped by status
func (r *callRepository) CountByStatus(ctx context.Context) (map[call.Status]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM calls
		GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count calls by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[call.Status]int)
	for rows.Next() {
		var statusStr string
		var count int

		if err := rows.Scan(&statusStr, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}

		status := mapEnumToStatus(statusStr)
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return counts, nil
}

// scanCall scans a database row into a Call struct
func scanCall(rows *sql.Rows) (*call.Call, error) {
	var c call.Call
	var statusStr, directionStr string
	var metadata []byte
	var buyerIDStr sql.NullString
	var sellerID sql.NullString
	var endTime sql.NullTime
	var duration sql.NullInt32
	var cost sql.NullFloat64

	err := rows.Scan(
		&c.ID, &c.FromNumber, &c.ToNumber, &statusStr, &directionStr,
		&buyerIDStr, &sellerID, &c.StartTime, &endTime,
		&duration, &cost, &metadata, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert database values
	c.Status = mapEnumToStatus(statusStr)
	c.Direction = mapTypeToDirection(directionStr)

	// Handle nullable buyer ID (marketplace calls may not have a buyer yet)
	if buyerIDStr.Valid {
		id, _ := uuid.Parse(buyerIDStr.String)
		c.BuyerID = id
	} else {
		c.BuyerID = uuid.Nil
	}

	if sellerID.Valid {
		id, _ := uuid.Parse(sellerID.String)
		c.SellerID = &id
	}

	if endTime.Valid {
		c.EndTime = &endTime.Time
	}

	if duration.Valid {
		d := int(duration.Int32)
		c.Duration = &d
	}

	if cost.Valid {
		money, _ := values.NewMoneyFromFloat(cost.Float64, "USD")
		c.Cost = &money
	}

	// Parse metadata
	parsedMetadata, err := ParseCallMetadata(metadata)
	if err == nil {
		parsedMetadata.ApplyToCall(&c)
	}

	return &c, nil
}

// Helper functions for mapping between domain and database types

func mapStatusToEnum(status call.Status) string {
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
		return "failed" // No cancelled in DB, map to failed
	case call.StatusNoAnswer:
		return "no_answer"
	case call.StatusBusy:
		return "no_answer" // Busy maps to no_answer
	default:
		return "pending"
	}
}

func mapEnumToStatus(enum string) call.Status {
	switch enum {
	case "pending":
		return call.StatusPending
	case "queued":
		return call.StatusQueued
	case "ringing":
		return call.StatusRinging
	case "in_progress":
		return call.StatusInProgress
	case "completed":
		return call.StatusCompleted
	case "failed":
		return call.StatusFailed
	case "no_answer":
		return call.StatusNoAnswer
	default:
		return call.StatusPending
	}
}

func mapTypeToDirection(directionStr string) call.Direction {
	if directionStr == "outbound" {
		return call.DirectionOutbound
	}
	return call.DirectionInbound
}

// sanitizeOrderBy validates and sanitizes the ORDER BY clause to prevent SQL injection
func sanitizeOrderBy(orderBy string) string {
	// Default order
	const defaultOrder = "created_at DESC"

	// Allowed columns that can be used in ORDER BY
	allowedColumns := map[string]bool{
		"id":         true,
		"status":     true,
		"direction":  true,
		"buyer_id":   true,
		"seller_id":  true,
		"started_at": true,
		"ended_at":   true,
		"duration":   true,
		"cost":       true,
		"created_at": true,
		"updated_at": true,
	}

	// Allowed sort directions
	allowedDirections := map[string]bool{
		"ASC":  true,
		"DESC": true,
		"asc":  true,
		"desc": true,
	}

	if orderBy == "" {
		return defaultOrder
	}

	// Split the order by clause (e.g., "created_at DESC" -> ["created_at", "DESC"])
	parts := strings.Fields(strings.TrimSpace(orderBy))

	if len(parts) == 0 {
		return defaultOrder
	}

	// Validate column name
	column := parts[0]
	if !allowedColumns[column] {
		return defaultOrder
	}

	// If only column is specified, default to DESC
	if len(parts) == 1 {
		return column + " DESC"
	}

	// Validate sort direction
	direction := parts[1]
	if !allowedDirections[direction] {
		return column + " DESC"
	}

	// Return sanitized ORDER BY clause
	return column + " " + strings.ToUpper(direction)
}

// GetActiveCallsForSeller returns active calls owned by a seller
func (r *callRepository) GetActiveCallsForSeller(ctx context.Context, sellerID uuid.UUID) ([]*call.Call, error) {
	filter := CallFilter{
		SellerID: &sellerID,
		Status:   &[]call.Status{call.StatusPending, call.StatusQueued, call.StatusRinging, call.StatusInProgress}[0],
		OrderBy:  "created_at DESC",
		Limit:    100,
	}

	// Use the existing List method with seller filter
	return r.List(ctx, filter)
}

// GetActiveCallsForBuyer returns active calls assigned to a buyer
func (r *callRepository) GetActiveCallsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*call.Call, error) {
	filter := CallFilter{
		BuyerID: &buyerID,
		Status:  &[]call.Status{call.StatusQueued, call.StatusRinging, call.StatusInProgress}[0],
		OrderBy: "created_at DESC",
		Limit:   100,
	}

	// Use the existing List method with buyer filter
	return r.List(ctx, filter)
}

// GetPendingSellerCalls returns pending calls from sellers awaiting routing
func (r *callRepository) GetPendingSellerCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	query := `
		SELECT 
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls 
		WHERE status = 'pending' 
		AND seller_id IS NOT NULL
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending seller calls: %w", err)
	}
	defer rows.Close()

	var calls []*call.Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		calls = append(calls, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return calls, nil
}

// GetIncomingCalls returns calls awaiting seller assignment (for seller distribution)
func (r *callRepository) GetIncomingCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	query := `
		SELECT 
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls 
		WHERE status = 'pending' 
		AND seller_id IS NULL
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query incoming calls: %w", err)
	}
	defer rows.Close()

	var calls []*call.Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		calls = append(calls, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return calls, nil
}
