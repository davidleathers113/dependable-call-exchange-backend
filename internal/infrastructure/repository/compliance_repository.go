package repository

import (
	"context"

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

// GetTCPARestrictions retrieves TCPA time restrictions
func (r *ComplianceRepository) GetTCPARestrictions(ctx context.Context) (*compliance.TCPARestrictions, error) {
	// TODO: Implement TCPA restrictions retrieval
	return &compliance.TCPARestrictions{
		StartTime: "09:00",
		EndTime:   "20:00",
		TimeZone:  "America/New_York",
	}, nil
}

// UpdateTCPARestrictions updates TCPA time restrictions
func (r *ComplianceRepository) UpdateTCPARestrictions(ctx context.Context, restrictions *compliance.TCPARestrictions) error {
	// TODO: Implement TCPA restrictions update
	return nil
}
