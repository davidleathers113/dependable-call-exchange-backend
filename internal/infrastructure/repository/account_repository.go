package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/seller_distribution"
)

// accountRepository implements AccountRepository using PostgreSQL
type accountRepository struct {
	db    interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	}
	dbConn *sql.DB // Keep reference to original DB for transaction operations
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *sql.DB) bidding.AccountRepository {
	return &accountRepository{db: db, dbConn: db}
}

// NewAccountRepositoryWithTx creates a new account repository with a transaction
func NewAccountRepositoryWithTx(tx *sql.Tx) bidding.AccountRepository {
	return &accountRepository{db: tx, dbConn: nil}
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
	var settingsJSON []byte
	var complianceFlags pq.StringArray
	var company sql.NullString
	var lastLoginAt sql.NullTime
	var emailStr, phoneStr string
	var balanceFloat, creditLimitFloat float64

	// We'll need to handle the address separately since it's a composite type
	addressJSON := []byte("{}")

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &emailStr, &a.Name, &company, &typeStr, &statusStr, &phoneStr,
		&balanceFloat, &creditLimitFloat, &a.PaymentTerms,
		&a.TCPAConsent, &a.GDPRConsent, &complianceFlags,
		&a.QualityMetrics.QualityScore, &a.QualityMetrics.FraudScore, &settingsJSON,
		&lastLoginAt, &a.CreatedAt, &a.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Convert database values to value objects
	email, err := values.NewEmail(emailStr)
	if err != nil {
		return nil, fmt.Errorf("invalid email from database: %w", err)
	}
	a.Email = email

	phone, err := values.NewPhoneNumber(phoneStr)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number from database: %w", err)
	}
	a.PhoneNumber = phone

	// Convert balance and credit limit to Money value objects
	// Assuming USD currency - in production, currency should be stored
	a.Balance = values.MustNewMoneyFromFloat(balanceFloat, "USD")
	a.CreditLimit = values.MustNewMoneyFromFloat(creditLimitFloat, "USD")

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

	// Convert PostgreSQL array to string slice
	a.ComplianceFlags = []string(complianceFlags)

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
	
	// If we're already in a transaction (WithTx), use it directly
	if r.dbConn == nil {
		return r.updateBalanceInTx(ctx, r.db, id, amount)
	}
	
	// Otherwise, create a new transaction
	tx, err := r.dbConn.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	if err := r.updateBalanceInTx(ctx, tx, id, amount); err != nil {
		return err
	}
	
	return tx.Commit()
}

func (r *accountRepository) updateBalanceInTx(ctx context.Context, tx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}, id uuid.UUID, amount float64) error {
	// Lock the account row for update
	var currentBalance float64
	query := `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`
	err := tx.QueryRowContext(ctx, query, id).Scan(&currentBalance)
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
	if a.Email.IsEmpty() {
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

	// Convert ComplianceFlags to PostgreSQL array format
	complianceFlags := pq.Array(a.ComplianceFlags)

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
		a.ID, a.Email.String(), a.Name, company, a.Type.String(), a.Status.String(), a.PhoneNumber.String(),
		a.Balance.ToFloat64(), a.CreditLimit.ToFloat64(), a.PaymentTerms,
		a.TCPAConsent, a.GDPRConsent, complianceFlags,
		a.QualityMetrics.QualityScore, a.QualityMetrics.FraudScore, settingsJSON,
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

// GetBuyerQualityMetrics retrieves quality metrics for a buyer account
func (r *accountRepository) GetBuyerQualityMetrics(ctx context.Context, buyerID uuid.UUID) (*values.QualityMetrics, error) {
	query := `
		SELECT 
			quality_score, fraud_score
		FROM accounts
		WHERE id = $1 AND type = 'buyer'
	`
	
	var qualityScore, fraudScore float64
	err := r.db.QueryRowContext(ctx, query, buyerID).Scan(&qualityScore, &fraudScore)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("buyer not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get buyer quality metrics: %w", err)
	}
	
	return &values.QualityMetrics{
		QualityScore: qualityScore,
		FraudScore:   fraudScore,
	}, nil
}

// GetActiveBuyers returns all active buyer accounts
func (r *accountRepository) GetActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error) {
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
	if err != nil {
		return nil, fmt.Errorf("failed to query active buyers: %w", err)
	}
	defer rows.Close()
	
	var accounts []*account.Account
	for rows.Next() {
		a, err := r.scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	
	return accounts, nil
}

// GetActiveSellers returns all active seller accounts
func (r *accountRepository) GetActiveSellers(ctx context.Context, limit int) ([]*account.Account, error) {
	query := `
		SELECT 
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			last_login_at, created_at, updated_at
		FROM accounts
		WHERE type = 'seller' AND status = 'active'
		ORDER BY quality_score DESC
		LIMIT $1
	`
	
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sellers: %w", err)
	}
	defer rows.Close()
	
	var accounts []*account.Account
	for rows.Next() {
		a, err := r.scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	
	return accounts, nil
}

// scanAccount is a helper to scan account rows
func (r *accountRepository) scanAccount(rows *sql.Rows) (*account.Account, error) {
	var a account.Account
	var typeStr, statusStr string
	var settingsJSON []byte
	var complianceFlags pq.StringArray
	var company sql.NullString
	var lastLoginAt sql.NullTime
	
	err := rows.Scan(
		&a.ID, &a.Email, &a.Name, &company, &typeStr, &statusStr, &a.PhoneNumber,
		&a.Balance, &a.CreditLimit, &a.PaymentTerms,
		&a.TCPAConsent, &a.GDPRConsent, &complianceFlags,
		&a.QualityMetrics.QualityScore, &a.QualityMetrics.FraudScore, &settingsJSON,
		&lastLoginAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
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
	
	// Convert PostgreSQL array to string slice
	a.ComplianceFlags = []string(complianceFlags)
	
	// Set default address for now
	a.Address = account.Address{}
	
	return &a, nil
}

// GetAvailableSellers returns sellers available for call assignment based on criteria
func (r *accountRepository) GetAvailableSellers(ctx context.Context, criteria *seller_distribution.SellerCriteria) ([]*account.Account, error) {
	query := `
		SELECT 
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			last_login_at, created_at, updated_at
		FROM accounts
		WHERE type = 'seller' AND status = 'active'
	`

	args := []interface{}{}
	argIndex := 1

	// Apply criteria filters
	if criteria != nil {
		if criteria.MinQuality > 0 {
			query += fmt.Sprintf(" AND quality_score >= $%d", argIndex)
			args = append(args, criteria.MinQuality)
			argIndex++
		}

		if criteria.AvailableNow {
			query += " AND last_login_at >= NOW() - INTERVAL '1 hour'"
		}

		// Add geography filter if specified
		if criteria.Geography != nil && len(criteria.Geography.States) > 0 {
			// This would require a more complex query with address/location data
			// For now, we'll skip this filter as the schema doesn't include location columns
		}

		// Add skills filter if specified
		if len(criteria.Skills) > 0 {
			// This would require querying against settings JSONB for skills
			// For now, we'll skip this complex filter
		}
	}

	query += " ORDER BY quality_score DESC"

	// Apply capacity limit if specified
	if criteria != nil && criteria.MaxCapacity > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, criteria.MaxCapacity)
	} else {
		query += " LIMIT 50" // Default limit
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query available sellers: %w", err)
	}
	defer rows.Close()

	var accounts []*account.Account
	for rows.Next() {
		a, err := r.scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, a)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return accounts, nil
}

// GetSellerCapacity returns current capacity information for a seller
func (r *accountRepository) GetSellerCapacity(ctx context.Context, sellerID uuid.UUID) (*seller_distribution.SellerCapacity, error) {
	// For now, we'll simulate capacity data since the database schema doesn't include
	// a dedicated capacity table. In production, this would likely be:
	// 1. A separate table tracking real-time capacity
	// 2. Cached data from a real-time system
	// 3. Calculated from active calls

	// First verify the seller exists
	var exists bool
	err := r.db.QueryRowContext(ctx, 
		"SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND type = 'seller' AND status = 'active')",
		sellerID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check seller existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("seller not found or not active")
	}

	// For demonstration, return simulated capacity data
	// In production, this would query actual capacity tables
	return &seller_distribution.SellerCapacity{
		SellerID:           sellerID,
		MaxConcurrentCalls: 5,  // Default capacity
		CurrentCalls:       0,  // Would be calculated from active calls
		AvailableSlots:     5,  // MaxConcurrentCalls - CurrentCalls
		LastUpdated:        time.Now(),
	}, nil
}