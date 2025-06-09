package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// callRepository implements CallRepository using PostgreSQL
type callRepository struct {
	db interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
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
	if c.FromNumber == "" || c.ToNumber == "" {
		return errors.New("from_number cannot be empty")
	}
	
	if c.BuyerID == uuid.Nil {
		return errors.New("buyer_id cannot be nil")
	}

	// Convert call type based on direction
	callType := "inbound"
	if c.Direction == call.DirectionOutbound {
		callType = "outbound"
	}

	// Map status to database enum
	statusStr := mapStatusToEnum(c.Status)

	query := `
		INSERT INTO calls (
			id, from_number, to_number, status, type,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14
		)
	`

	metadata := map[string]interface{}{
		"call_sid":   c.CallSID,
		"session_id": c.SessionID,
		"user_agent": c.UserAgent,
		"ip_address": c.IPAddress,
		"location":   c.Location,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		c.ID, c.FromNumber, c.ToNumber, statusStr, callType,
		c.BuyerID, c.SellerID, c.StartTime, c.EndTime,
		c.Duration, c.Cost, metadataJSON, c.CreatedAt, c.UpdatedAt,
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
			id, from_number, to_number, status, type,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata, created_at, updated_at
		FROM calls
		WHERE id = $1
	`

	var c call.Call
	var statusStr, callType string
	var metadata []byte
	var sellerID sql.NullString
	var endTime sql.NullTime
	var duration sql.NullInt32
	var cost sql.NullFloat64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.FromNumber, &c.ToNumber, &statusStr, &callType,
		&c.BuyerID, &sellerID, &c.StartTime, &endTime,
		&duration, &cost, &metadata, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert database values
	c.Status = mapEnumToStatus(statusStr)
	c.Direction = mapTypeToDirection(callType)

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
		c.Cost = &cost.Float64
	}

	// Parse metadata
	var meta map[string]interface{}
	if err := json.Unmarshal(metadata, &meta); err == nil {
		if v, ok := meta["call_sid"].(string); ok {
			c.CallSID = v
		}
		if v, ok := meta["session_id"].(string); ok {
			c.SessionID = &v
		}
		if v, ok := meta["user_agent"].(string); ok {
			c.UserAgent = &v
		}
		if v, ok := meta["ip_address"].(string); ok {
			c.IPAddress = &v
		}
		if v, ok := meta["location"].(map[string]interface{}); ok {
			loc := &call.Location{}
			if country, ok := v["country"].(string); ok {
				loc.Country = country
			}
			if state, ok := v["state"].(string); ok {
				loc.State = state
			}
			if city, ok := v["city"].(string); ok {
				loc.City = city
			}
			if lat, ok := v["latitude"].(float64); ok {
				loc.Latitude = lat
			}
			if lon, ok := v["longitude"].(float64); ok {
				loc.Longitude = lon
			}
			if tz, ok := v["timezone"].(string); ok {
				loc.Timezone = tz
			}
			c.Location = loc
		}
	}

	return &c, nil
}

// Update updates an existing call
func (r *callRepository) Update(ctx context.Context, c *call.Call) error {
	// Map status to database enum
	statusStr := mapStatusToEnum(c.Status)

	// Update metadata
	metadata := map[string]interface{}{
		"call_sid":   c.CallSID,
		"session_id": c.SessionID,
		"user_agent": c.UserAgent,
		"ip_address": c.IPAddress,
		"location":   c.Location,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE calls
		SET 
			status = $2,
			ended_at = $3,
			duration = $4,
			cost = $5,
			metadata = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		c.ID, statusStr, c.EndTime, c.Duration, c.Cost,
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
	var args []interface{}
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
			id, from_number, to_number, status, type,
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
	var statusStr, callType string
	var metadata []byte
	var sellerID sql.NullString
	var endTime sql.NullTime
	var duration sql.NullInt32
	var cost sql.NullFloat64

	err := rows.Scan(
		&c.ID, &c.FromNumber, &c.ToNumber, &statusStr, &callType,
		&c.BuyerID, &sellerID, &c.StartTime, &endTime,
		&duration, &cost, &metadata, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert database values
	c.Status = mapEnumToStatus(statusStr)
	c.Direction = mapTypeToDirection(callType)

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
		c.Cost = &cost.Float64
	}

	// Parse metadata
	var meta map[string]interface{}
	if err := json.Unmarshal(metadata, &meta); err == nil {
		if v, ok := meta["call_sid"].(string); ok {
			c.CallSID = v
		}
		if v, ok := meta["session_id"].(string); ok {
			c.SessionID = &v
		}
		if v, ok := meta["user_agent"].(string); ok {
			c.UserAgent = &v
		}
		if v, ok := meta["ip_address"].(string); ok {
			c.IPAddress = &v
		}
		if v, ok := meta["location"].(map[string]interface{}); ok {
			loc := &call.Location{}
			if country, ok := v["country"].(string); ok {
				loc.Country = country
			}
			if state, ok := v["state"].(string); ok {
				loc.State = state
			}
			if city, ok := v["city"].(string); ok {
				loc.City = city
			}
			if lat, ok := v["latitude"].(float64); ok {
				loc.Latitude = lat
			}
			if lon, ok := v["longitude"].(float64); ok {
				loc.Longitude = lon
			}
			if tz, ok := v["timezone"].(string); ok {
				loc.Timezone = tz
			}
			c.Location = loc
		}
	}

	return &c, nil
}

// Helper functions for mapping between domain and database types

func mapStatusToEnum(status call.Status) string {
	switch status {
	case call.StatusPending:
		return "pending"
	case call.StatusQueued:
		return "routing" // Map to closest database enum
	case call.StatusRinging:
		return "routing"
	case call.StatusInProgress:
		return "active"
	case call.StatusCompleted:
		return "completed"
	case call.StatusFailed:
		return "failed"
	case call.StatusCanceled:
		return "cancelled"
	case call.StatusNoAnswer:
		return "failed"
	case call.StatusBusy:
		return "failed"
	default:
		return "pending"
	}
}

func mapEnumToStatus(enum string) call.Status {
	switch enum {
	case "pending":
		return call.StatusPending
	case "routing":
		return call.StatusQueued
	case "active":
		return call.StatusInProgress
	case "completed":
		return call.StatusCompleted
	case "failed":
		return call.StatusFailed
	case "cancelled":
		return call.StatusCanceled
	default:
		return call.StatusPending
	}
}

func mapTypeToDirection(callType string) call.Direction {
	if callType == "outbound" {
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
		"type":       true,
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
