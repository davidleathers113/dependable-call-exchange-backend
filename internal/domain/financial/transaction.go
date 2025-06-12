package financial

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Transaction represents a financial transaction
type Transaction struct {
	ID            uuid.UUID
	Type          TransactionType
	Amount        float64
	Currency      string
	FromAccountID uuid.UUID
	ToAccountID   uuid.UUID
	ReferenceType string
	ReferenceID   uuid.UUID
	Description   string
	Status        TransactionStatus
	CreatedAt     time.Time
	ProcessedAt   *time.Time
}

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeCharge      TransactionType = "charge"
	TransactionTypeCredit      TransactionType = "credit"
	TransactionTypeRefund      TransactionType = "refund"
	TransactionTypeTransfer    TransactionType = "transfer"
	TransactionTypeHold        TransactionType = "hold"
	TransactionTypeHoldRelease TransactionType = "hold_release"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusProcessing TransactionStatus = "processing"
	TransactionStatusCompleted  TransactionStatus = "completed"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusCancelled  TransactionStatus = "cancelled"
)

// String returns the string representation of TransactionType
func (t TransactionType) String() string {
	return string(t)
}

// String returns the string representation of TransactionStatus
func (s TransactionStatus) String() string {
	return string(s)
}

// IsValid checks if the transaction type is valid
func (t TransactionType) IsValid() bool {
	switch t {
	case TransactionTypeCharge, TransactionTypeCredit, TransactionTypeRefund,
		TransactionTypeTransfer, TransactionTypeHold, TransactionTypeHoldRelease:
		return true
	default:
		return false
	}
}

// IsValid checks if the transaction status is valid
func (s TransactionStatus) IsValid() bool {
	switch s {
	case TransactionStatusPending, TransactionStatusProcessing, TransactionStatusCompleted,
		TransactionStatusFailed, TransactionStatusCancelled:
		return true
	default:
		return false
	}
}

// NewTransaction creates a new transaction with validation
func NewTransaction(
	txnType TransactionType,
	amount float64,
	currency string,
	fromAccountID, toAccountID uuid.UUID,
	referenceType string,
	referenceID uuid.UUID,
	description string,
) (*Transaction, error) {
	// Validate transaction type
	if !txnType.IsValid() {
		return nil, fmt.Errorf("invalid transaction type: %s", txnType)
	}

	// Validate amount
	if amount < 0 {
		return nil, fmt.Errorf("transaction amount cannot be negative")
	}

	if amount == 0 && txnType != TransactionTypeHold && txnType != TransactionTypeHoldRelease {
		return nil, fmt.Errorf("transaction amount cannot be zero for type %s", txnType)
	}

	// Validate currency
	if currency == "" {
		return nil, fmt.Errorf("currency cannot be empty")
	}

	// Validate account IDs for transfer/charge transactions
	if txnType == TransactionTypeTransfer {
		if fromAccountID == uuid.Nil || toAccountID == uuid.Nil {
			return nil, fmt.Errorf("both from and to accounts required for transfer")
		}
		if fromAccountID == toAccountID {
			return nil, fmt.Errorf("cannot transfer to the same account")
		}
	}

	// Validate reference
	if referenceType == "" {
		return nil, fmt.Errorf("reference type cannot be empty")
	}
	if referenceID == uuid.Nil {
		return nil, fmt.Errorf("reference ID cannot be nil")
	}

	now := time.Now()
	return &Transaction{
		ID:            uuid.New(),
		Type:          txnType,
		Amount:        amount,
		Currency:      currency,
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
		Status:        TransactionStatusPending,
		CreatedAt:     now,
	}, nil
}

// Process marks the transaction as processing
func (t *Transaction) Process() error {
	if t.Status != TransactionStatusPending {
		return fmt.Errorf("can only process pending transactions")
	}

	t.Status = TransactionStatusProcessing
	return nil
}

// Complete marks the transaction as completed
func (t *Transaction) Complete() error {
	if t.Status != TransactionStatusProcessing {
		return fmt.Errorf("can only complete processing transactions")
	}

	now := time.Now()
	t.Status = TransactionStatusCompleted
	t.ProcessedAt = &now
	return nil
}

// Fail marks the transaction as failed
func (t *Transaction) Fail(reason string) error {
	if t.Status == TransactionStatusCompleted {
		return fmt.Errorf("cannot fail completed transaction")
	}

	t.Status = TransactionStatusFailed
	if reason != "" {
		t.Description = fmt.Sprintf("%s - Failed: %s", t.Description, reason)
	}
	return nil
}

// Cancel marks the transaction as cancelled
func (t *Transaction) Cancel() error {
	if t.Status == TransactionStatusCompleted {
		return fmt.Errorf("cannot cancel completed transaction")
	}

	if t.Status == TransactionStatusProcessing {
		return fmt.Errorf("cannot cancel processing transaction")
	}

	t.Status = TransactionStatusCancelled
	return nil
}

// IsCompleted returns true if the transaction is completed
func (t *Transaction) IsCompleted() bool {
	return t.Status == TransactionStatusCompleted
}

// IsPending returns true if the transaction is pending
func (t *Transaction) IsPending() bool {
	return t.Status == TransactionStatusPending
}

// IsProcessing returns true if the transaction is processing
func (t *Transaction) IsProcessing() bool {
	return t.Status == TransactionStatusProcessing
}

// IsFailed returns true if the transaction failed
func (t *Transaction) IsFailed() bool {
	return t.Status == TransactionStatusFailed
}

// IsCancelled returns true if the transaction was cancelled
func (t *Transaction) IsCancelled() bool {
	return t.Status == TransactionStatusCancelled
}

// CanProcess validates if the transaction can be processed
func (t *Transaction) CanProcess() error {
	if t.Status != TransactionStatusPending {
		return fmt.Errorf("transaction is not pending")
	}

	if t.Amount < 0 {
		return fmt.Errorf("cannot process negative amount")
	}

	return nil
}

// GetAge returns how long ago the transaction was created
func (t *Transaction) GetAge() time.Duration {
	return time.Since(t.CreatedAt)
}

// IsStale checks if transaction is older than the specified duration
func (t *Transaction) IsStale(maxAge time.Duration) bool {
	return t.GetAge() > maxAge
}

// GetProcessingTime returns how long the transaction took to process
func (t *Transaction) GetProcessingTime() *time.Duration {
	if t.ProcessedAt == nil {
		return nil
	}

	duration := t.ProcessedAt.Sub(t.CreatedAt)
	return &duration
}

// CreateRefund creates a refund transaction for this transaction
func (t *Transaction) CreateRefund(reason string) (*Transaction, error) {
	if !t.IsCompleted() {
		return nil, fmt.Errorf("can only refund completed transactions")
	}

	if t.Type == TransactionTypeRefund {
		return nil, fmt.Errorf("cannot refund a refund transaction")
	}

	// Create refund with reversed accounts for charges/transfers
	var fromAccount, toAccount uuid.UUID
	switch t.Type {
	case TransactionTypeCharge:
		fromAccount = t.ToAccountID
		toAccount = t.FromAccountID
	case TransactionTypeTransfer:
		fromAccount = t.ToAccountID
		toAccount = t.FromAccountID
	default:
		return nil, fmt.Errorf("refunds not supported for transaction type %s", t.Type)
	}

	description := fmt.Sprintf("Refund for transaction %s", t.ID.String())
	if reason != "" {
		description = fmt.Sprintf("%s - Reason: %s", description, reason)
	}

	return NewTransaction(
		TransactionTypeRefund,
		t.Amount,
		t.Currency,
		fromAccount,
		toAccount,
		"transaction_refund",
		t.ID,
		description,
	)
}
