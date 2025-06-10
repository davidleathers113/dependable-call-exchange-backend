package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/financial"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FinancialRepository handles financial data persistence
type FinancialRepository struct {
	db *pgxpool.Pool
}

// NewFinancialRepository creates a new financial repository
func NewFinancialRepository(db *pgxpool.Pool) *FinancialRepository {
	return &FinancialRepository{
		db: db,
	}
}

// CreateTransaction creates a new financial transaction
func (r *FinancialRepository) CreateTransaction(ctx context.Context, tx *financial.Transaction) error {
	query := `
		INSERT INTO transactions (id, type, amount, currency, from_account_id, to_account_id, 
			reference_type, reference_id, description, status, created_at, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.Exec(ctx, query,
		tx.ID, tx.Type, tx.Amount, tx.Currency, tx.FromAccountID, tx.ToAccountID,
		tx.ReferenceType, tx.ReferenceID, tx.Description, tx.Status, tx.CreatedAt, tx.ProcessedAt)
	
	return err
}

// GetTransactionByID retrieves a transaction by ID
func (r *FinancialRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*financial.Transaction, error) {
	tx := &financial.Transaction{}
	query := `
		SELECT id, type, amount, currency, from_account_id, to_account_id, 
			reference_type, reference_id, description, status, created_at, processed_at
		FROM transactions WHERE id = $1`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&tx.ID, &tx.Type, &tx.Amount, &tx.Currency, &tx.FromAccountID, &tx.ToAccountID,
		&tx.ReferenceType, &tx.ReferenceID, &tx.Description, &tx.Status, &tx.CreatedAt, &tx.ProcessedAt)
	
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	
	return tx, err
}

// GetTransactionsByAccount retrieves all transactions for an account
func (r *FinancialRepository) GetTransactionsByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*financial.Transaction, error) {
	query := `
		SELECT id, type, amount, currency, from_account_id, to_account_id, 
			reference_type, reference_id, description, status, created_at, processed_at
		FROM transactions 
		WHERE from_account_id = $1 OR to_account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*financial.Transaction
	for rows.Next() {
		tx := &financial.Transaction{}
		err := rows.Scan(
			&tx.ID, &tx.Type, &tx.Amount, &tx.Currency, &tx.FromAccountID, &tx.ToAccountID,
			&tx.ReferenceType, &tx.ReferenceID, &tx.Description, &tx.Status, &tx.CreatedAt, &tx.ProcessedAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// CreateInvoice creates a new invoice
func (r *FinancialRepository) CreateInvoice(ctx context.Context, invoice *financial.Invoice) error {
	// TODO: Implement invoice creation
	return nil
}

// GetInvoiceByID retrieves an invoice by ID
func (r *FinancialRepository) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*financial.Invoice, error) {
	// TODO: Implement invoice retrieval
	return nil, ErrNotFound
}

// CreatePaymentMethod creates a new payment method
func (r *FinancialRepository) CreatePaymentMethod(ctx context.Context, method *financial.PaymentMethod) error {
	// TODO: Implement payment method creation
	return nil
}

// GetPaymentMethodsByAccount retrieves payment methods for an account
func (r *FinancialRepository) GetPaymentMethodsByAccount(ctx context.Context, accountID uuid.UUID) ([]*financial.PaymentMethod, error) {
	// TODO: Implement payment methods retrieval
	return []*financial.PaymentMethod{}, nil
}

// GetReconciliationReport generates a reconciliation report for a time period
func (r *FinancialRepository) GetReconciliationReport(ctx context.Context, start, end time.Time) (*financial.ReconciliationReport, error) {
	// TODO: Implement reconciliation report generation
	return &financial.ReconciliationReport{
		ID:                uuid.New(),
		PeriodStart:       start,
		PeriodEnd:         end,
		TotalTransactions: 0,
		TotalDebits:       0,
		TotalCredits:      0,
		Discrepancies:     []financial.Discrepancy{},
		Status:            financial.ReconciliationStatusCompleted,
		CreatedAt:         time.Now(),
	}, nil
}
