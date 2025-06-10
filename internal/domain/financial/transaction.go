package financial

import (
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
	TransactionTypeCharge    TransactionType = "charge"
	TransactionTypeCredit    TransactionType = "credit"
	TransactionTypeRefund    TransactionType = "refund"
	TransactionTypeTransfer  TransactionType = "transfer"
	TransactionTypeHold      TransactionType = "hold"
	TransactionTypeHoldRelease TransactionType = "hold_release"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusProcessing TransactionStatus = "processing"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusCancelled TransactionStatus = "cancelled"
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
