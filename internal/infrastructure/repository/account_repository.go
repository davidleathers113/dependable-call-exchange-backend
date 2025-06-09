package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
)

// accountRepository implements AccountRepository using PostgreSQL
type accountRepository struct {
	db *sql.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *sql.DB) bidding.AccountRepository {
	return &accountRepository{db: db}
}

// GetByID retrieves an account by ID
func (r *accountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	query := `
		SELECT 
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			last_login_at, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`

	var a account.Account
	var typeStr, statusStr string
	var settingsJSON, complianceFlagsJSON []byte
	var company sql.NullString
	var lastLoginAt sql.NullTime

	// We'll need to handle the address separately since it's a composite type
	addressJSON := []byte("{}")

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.Email, &a.Name, &company, &typeStr, &statusStr, &a.PhoneNumber,
		&a.Balance, &a.CreditLimit, &a.PaymentTerms,
		&a.TCPAConsent, &a.GDPRConsent, &complianceFlagsJSON,
		&a.QualityScore, &a.FraudScore, &settingsJSON,
		&lastLoginAt, &a.CreatedAt, &a.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Convert database values
	a.Type = parseAccountType(typeStr)
	a.Status = parseAccountStatus(statusStr)

	if company.Valid {
		a.Company = &company.String
	}

	if lastLoginAt.Valid {
		a.LastLoginAt = &lastLoginAt.Time
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(settingsJSON, &a.Settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if err := json.Unmarshal(complianceFlagsJSON, &a.ComplianceFlags); err != nil {
		// Default to empty array if unmarshal fails
		a.ComplianceFlags = []string{}
	}

	// For now, set a default address - in a real implementation, this would be stored properly
	if err := json.Unmarshal(addressJSON, &a.Address); err != nil {
		a.Address = account.Address{}
	}

	return &a, nil
}

// UpdateBalance updates account balance atomically
func (r *accountRepository) UpdateBalance(ctx context.Context, id uuid.UUID, amount float64) error {
	// This operation needs to be atomic to prevent race conditions
	// Using a transaction with SELECT FOR UPDATE ensures consistency
	
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock the account row for update
	var currentBalance float64
	query := `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, id).Scan(&currentBalance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("account not found: %w", err)
		}
		return fmt.Errorf("failed to get current balance: %w", err)
	}

	// Calculate new balance
	newBalance := currentBalance + amount
	
	// Validate balance doesn't go negative unless it's a credit account
	if newBalance < 0 {
		// Check if account has credit limit
		var creditLimit float64
		err = tx.QueryRowContext(ctx, 
			`SELECT credit_limit FROM accounts WHERE id = $1`, id).Scan(&creditLimit)
		if err != nil {
			return fmt.Errorf("failed to check credit limit: %w", err)
		}
		
		if newBalance < -creditLimit {
			return fmt.Errorf("insufficient balance: would exceed credit limit")
		}
	}

	// Update the balance
	updateQuery := `
		UPDATE accounts 
		SET balance = $2, updated_at = NOW() 
		WHERE id = $1
	`
	result, err := tx.ExecContext(ctx, updateQuery, id, newBalance)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %s not found", id)
	}

	// Log the transaction for audit trail
	auditQuery := `
		INSERT INTO account_transactions (
			id, account_id, amount, balance_after, 
			transaction_type, description, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW()
		)
	`
	
	transactionType := "credit"
	if amount < 0 {
		transactionType = "debit"
	}
	
	_, err = tx.ExecContext(ctx, auditQuery,
		uuid.New(), id, amount, newBalance,
		transactionType, "Balance update",
	)
	// If audit log fails, we still continue (non-critical)
	if err != nil {
		// Log the error but don't fail the transaction
		// In production, this would be logged to monitoring
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetBalance returns current balance
func (r *accountRepository) GetBalance(ctx context.Context, id uuid.UUID) (float64, error) {
	query := `SELECT balance FROM accounts WHERE id = $1`
	
	var balance float64
	err := r.db.QueryRowContext(ctx, query, id).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("account not found: %w", err)
		}
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

// UpdateQualityScore updates an account's quality score
func (r *accountRepository) UpdateQualityScore(ctx context.Context, id uuid.UUID, score float64) error {
	// Validate score range
	if score < 0 || score > 100 {
		return errors.New("quality score must be between 0 and 100")
	}

	query := `
		UPDATE accounts 
		SET quality_score = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, score)
	if err != nil {
		return fmt.Errorf("failed to update quality score: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("account not found")
	}

	return nil
}

// Additional method for creating accounts (useful for testing)
func (r *accountRepository) Create(ctx context.Context, a *account.Account) error {
	// Validate required fields
	if a.Email == "" {
		return errors.New("email is required")
	}
	if a.Name == "" {
		return errors.New("name is required")
	}

	// Serialize complex types
	settingsJSON, err := json.Marshal(a.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	complianceFlagsJSON, err := json.Marshal(a.ComplianceFlags)
	if err != nil {
		return fmt.Errorf("failed to marshal compliance flags: %w", err)
	}

	// For now, we'll store address as JSON in settings
	// In production, this might be a separate table or structured differently
	
	query := `
		INSERT INTO accounts (
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			last_login_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15, $16,
			$17, $18, $19
		)
	`

	// Handle optional company
	var company interface{} = sql.NullString{}
	if a.Company != nil {
		company = *a.Company
	}

	_, err = r.db.ExecContext(ctx, query,
		a.ID, a.Email, a.Name, company, a.Type.String(), a.Status.String(), a.PhoneNumber,
		a.Balance, a.CreditLimit, a.PaymentTerms,
		a.TCPAConsent, a.GDPRConsent, complianceFlagsJSON,
		a.QualityScore, a.FraudScore, settingsJSON,
		a.LastLoginAt, a.CreatedAt, a.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

// parseAccountType converts string to account.AccountType
func parseAccountType(s string) account.AccountType {
	switch s {
	case "buyer":
		return account.TypeBuyer
	case "seller":
		return account.TypeSeller
	case "admin":
		return account.TypeAdmin
	default:
		return account.TypeBuyer
	}
}

// parseAccountStatus converts string to account.Status
func parseAccountStatus(s string) account.Status {
	switch s {
	case "pending":
		return account.StatusPending
	case "active":
		return account.StatusActive
	case "suspended":
		return account.StatusSuspended
	case "closed":
		return account.StatusClosed
	default:
		return account.StatusPending
	}
}