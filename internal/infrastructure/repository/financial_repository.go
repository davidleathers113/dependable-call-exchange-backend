package repository

import (
	"context"
	"database/sql"
	"fmt"
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
	if invoice == nil {
		return fmt.Errorf("invoice cannot be nil")
	}

	// Generate invoice number if not provided
	if invoice.Number == "" {
		invoice.Number = generateInvoiceNumber()
	}

	// Set ID if not provided
	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}

	// Insert invoice
	query := `
		INSERT INTO invoices (id, account_id, number, total_amount, currency, status, 
			due_date, paid_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		invoice.ID,
		invoice.AccountID,
		invoice.Number,
		invoice.TotalAmount,
		invoice.Currency,
		invoice.Status,
		invoice.DueDate,
		invoice.PaidAt,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create invoice: %w", err)
	}

	// Insert line items
	for _, item := range invoice.LineItems {
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}

		itemQuery := `
			INSERT INTO invoice_line_items (id, invoice_id, type, description, 
				quantity, unit_price, amount, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

		_, err = r.db.Exec(ctx, itemQuery,
			item.ID,
			invoice.ID,
			item.Type,
			item.Description,
			item.Quantity,
			item.UnitPrice,
			item.Amount,
			now,
		)

		if err != nil {
			return fmt.Errorf("failed to create line item: %w", err)
		}
	}

	return nil
}

// GetInvoiceByID retrieves an invoice by ID
func (r *FinancialRepository) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*financial.Invoice, error) {
	invoice := &financial.Invoice{}

	// Get invoice
	query := `
		SELECT id, account_id, number, total_amount, currency, status, 
			due_date, paid_at, created_at, updated_at
		FROM invoices WHERE id = $1`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.AccountID,
		&invoice.Number,
		&invoice.TotalAmount,
		&invoice.Currency,
		&invoice.Status,
		&invoice.DueDate,
		&invoice.PaidAt,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	// Get line items
	itemsQuery := `
		SELECT id, invoice_id, type, description, quantity, unit_price, amount, created_at
		FROM invoice_line_items WHERE invoice_id = $1 ORDER BY created_at`

	rows, err := r.db.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	defer rows.Close()

	var lineItems []financial.LineItem
	for rows.Next() {
		var item financial.LineItem
		err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.Type,
			&item.Description,
			&item.Quantity,
			&item.UnitPrice,
			&item.Amount,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan line item: %w", err)
		}
		lineItems = append(lineItems, item)
	}

	invoice.LineItems = lineItems
	return invoice, nil
}

// CreatePaymentMethod creates a new payment method
func (r *FinancialRepository) CreatePaymentMethod(ctx context.Context, method *financial.PaymentMethod) error {
	if method == nil {
		return fmt.Errorf("payment method cannot be nil")
	}

	// Set ID if not provided
	if method.ID == uuid.Nil {
		method.ID = uuid.New()
	}

	query := `
		INSERT INTO payment_methods (id, account_id, type, last4, expiry_month, expiry_year, 
			brand, is_default, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		method.ID,
		method.AccountID,
		method.Type,
		method.Last4,
		method.ExpiryMonth,
		method.ExpiryYear,
		method.Brand,
		method.IsDefault,
		method.Status,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create payment method: %w", err)
	}

	return nil
}

// GetPaymentMethodsByAccount retrieves payment methods for an account
func (r *FinancialRepository) GetPaymentMethodsByAccount(ctx context.Context, accountID uuid.UUID) ([]*financial.PaymentMethod, error) {
	query := `
		SELECT id, account_id, type, last4, expiry_month, expiry_year, 
			brand, is_default, status, created_at, updated_at
		FROM payment_methods 
		WHERE account_id = $1 AND status = $2
		ORDER BY is_default DESC, created_at DESC`

	rows, err := r.db.Query(ctx, query, accountID, financial.PaymentMethodStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment methods: %w", err)
	}
	defer rows.Close()

	var methods []*financial.PaymentMethod
	for rows.Next() {
		method := &financial.PaymentMethod{}
		err := rows.Scan(
			&method.ID,
			&method.AccountID,
			&method.Type,
			&method.Last4,
			&method.ExpiryMonth,
			&method.ExpiryYear,
			&method.Brand,
			&method.IsDefault,
			&method.Status,
			&method.CreatedAt,
			&method.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment method: %w", err)
		}
		methods = append(methods, method)
	}

	return methods, rows.Err()
}

// GetReconciliationReport generates a reconciliation report for a time period
func (r *FinancialRepository) GetReconciliationReport(ctx context.Context, start, end time.Time) (*financial.ReconciliationReport, error) {
	report := &financial.ReconciliationReport{
		ID:          uuid.New(),
		PeriodStart: start,
		PeriodEnd:   end,
		CreatedAt:   time.Now(),
		Status:      financial.ReconciliationStatusPending,
	}

	// Get transaction summary
	summaryQuery := `
		SELECT 
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN amount < 0 THEN ABS(amount) ELSE 0 END), 0) as total_debits,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0) as total_credits
		FROM transactions 
		WHERE created_at >= $1 AND created_at <= $2 AND status = $3`

	err := r.db.QueryRow(ctx, summaryQuery, start, end, financial.TransactionStatusCompleted).Scan(
		&report.TotalTransactions,
		&report.TotalDebits,
		&report.TotalCredits,
	)

	if err != nil {
		report.Status = financial.ReconciliationStatusFailed
		return report, fmt.Errorf("failed to get transaction summary: %w", err)
	}

	// Check for discrepancies (simplified logic for now)
	discrepancyQuery := `
		SELECT id, amount, description
		FROM transactions 
		WHERE created_at >= $1 AND created_at <= $2 
		AND (status = $3 OR amount = 0)
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, discrepancyQuery, start, end, financial.TransactionStatusFailed)
	if err != nil {
		report.Status = financial.ReconciliationStatusFailed
		return report, fmt.Errorf("failed to get discrepancies: %w", err)
	}
	defer rows.Close()

	var discrepancies []financial.Discrepancy
	for rows.Next() {
		var txID uuid.UUID
		var amount float64
		var description string

		err := rows.Scan(&txID, &amount, &description)
		if err != nil {
			continue // Skip malformed records
		}

		discrepancy := financial.Discrepancy{
			TransactionID: txID,
			Type:          "failed_transaction",
			Amount:        amount,
			Description:   description,
		}
		discrepancies = append(discrepancies, discrepancy)
	}

	report.Discrepancies = discrepancies
	report.Status = financial.ReconciliationStatusCompleted

	return report, nil
}

// generateInvoiceNumber generates a unique invoice number
func generateInvoiceNumber() string {
	// Simple format: INV-YYYYMMDD-XXXX
	now := time.Now()
	dateStr := now.Format("20060102")
	// In production, you'd want to use a proper sequence or counter
	randomSuffix := now.UnixNano() % 10000
	return fmt.Sprintf("INV-%s-%04d", dateStr, randomSuffix)
}
