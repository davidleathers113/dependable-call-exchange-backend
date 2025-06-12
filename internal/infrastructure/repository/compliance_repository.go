package repository

import (
	"context"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ComplianceRepository handles compliance data persistence
type ComplianceRepository struct {
	db *pgxpool.Pool
}

// NewComplianceRepository creates a new compliance repository
func NewComplianceRepository(db *pgxpool.Pool) *ComplianceRepository {
	return &ComplianceRepository{
		db: db,
	}
}

// AddToDNCList adds a phone number to the Do Not Call list
func (r *ComplianceRepository) AddToDNCList(ctx context.Context, phoneNumber, reason string) error {
	query := `
		INSERT INTO dnc_list (id, phone_number, reason, created_at)
		VALUES ($1, $2, $3, NOW())`

	_, err := r.db.Exec(ctx, query, uuid.New(), phoneNumber, reason)
	return err
}

// IsOnDNCList checks if a phone number is on the Do Not Call list
func (r *ComplianceRepository) IsOnDNCList(ctx context.Context, phoneNumber string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM dnc_list WHERE phone_number = $1 AND deleted_at IS NULL)`

	err := r.db.QueryRow(ctx, query, phoneNumber).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetTCPARestrictions retrieves TCPA time restrictions from database
func (r *ComplianceRepository) GetTCPARestrictions(ctx context.Context) (*compliance.TCPARestrictions, error) {
	var restrictions compliance.TCPARestrictions

	query := `
		SELECT start_time, end_time, timezone 
		FROM tcpa_restrictions 
		WHERE active = true 
		ORDER BY updated_at DESC 
		LIMIT 1`

	err := r.db.QueryRow(ctx, query).Scan(
		&restrictions.StartTime,
		&restrictions.EndTime,
		&restrictions.TimeZone,
	)

	if err != nil {
		// Return default restrictions if none configured
		return &compliance.TCPARestrictions{
			StartTime: "08:00",
			EndTime:   "21:00",
			TimeZone:  "America/New_York",
		}, nil
	}

	return &restrictions, nil
}

// UpdateTCPARestrictions updates TCPA time restrictions
func (r *ComplianceRepository) UpdateTCPARestrictions(ctx context.Context, restrictions *compliance.TCPARestrictions) error {
	if restrictions == nil {
		return fmt.Errorf("restrictions cannot be nil")
	}

	// Validate time format (HH:MM)
	if !isValidTimeFormat(restrictions.StartTime) || !isValidTimeFormat(restrictions.EndTime) {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	// Validate timezone
	if restrictions.TimeZone == "" {
		restrictions.TimeZone = "America/New_York" // Default to Eastern Time
	}

	// Deactivate current restrictions
	deactivateQuery := `UPDATE tcpa_restrictions SET active = false WHERE active = true`
	_, err := r.db.Exec(ctx, deactivateQuery)
	if err != nil {
		return fmt.Errorf("failed to deactivate current restrictions: %w", err)
	}

	// Insert new restrictions
	insertQuery := `
		INSERT INTO tcpa_restrictions (id, start_time, end_time, timezone, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`

	_, err = r.db.Exec(ctx, insertQuery,
		uuid.New(),
		restrictions.StartTime,
		restrictions.EndTime,
		restrictions.TimeZone,
		true,
	)

	if err != nil {
		return fmt.Errorf("failed to insert new restrictions: %w", err)
	}

	return nil
}

// isValidTimeFormat validates HH:MM time format
func isValidTimeFormat(timeStr string) bool {
	if len(timeStr) != 5 {
		return false
	}

	if timeStr[2] != ':' {
		return false
	}

	// Parse hour and minute
	hour := timeStr[:2]
	minute := timeStr[3:]

	// Validate hour (00-23)
	if hour < "00" || hour > "23" {
		return false
	}

	// Validate minute (00-59)
	if minute < "00" || minute > "59" {
		return false
	}

	return true
}
