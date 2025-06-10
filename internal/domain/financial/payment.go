package financial

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethod represents a stored payment method
type PaymentMethod struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	Type           PaymentMethodType
	Last4          string
	ExpiryMonth    int
	ExpiryYear     int
	Brand          string
	IsDefault      bool
	Status         PaymentMethodStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PaymentMethodType represents the type of payment method
type PaymentMethodType string

const (
	PaymentMethodTypeCard PaymentMethodType = "card"
	PaymentMethodTypeBank PaymentMethodType = "bank"
)

// PaymentMethodStatus represents the status of a payment method
type PaymentMethodStatus string

const (
	PaymentMethodStatusActive   PaymentMethodStatus = "active"
	PaymentMethodStatusInactive PaymentMethodStatus = "inactive"
	PaymentMethodStatusExpired  PaymentMethodStatus = "expired"
)

// Invoice represents a billing invoice
type Invoice struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	Number      string
	TotalAmount float64
	Currency    string
	Status      InvoiceStatus
	DueDate     time.Time
	PaidAt      *time.Time
	LineItems   []LineItem
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InvoiceStatus represents the status of an invoice
type InvoiceStatus string

const (
	InvoiceStatusDraft    InvoiceStatus = "draft"
	InvoiceStatusPending  InvoiceStatus = "pending"
	InvoiceStatusPaid     InvoiceStatus = "paid"
	InvoiceStatusOverdue  InvoiceStatus = "overdue"
	InvoiceStatusCanceled InvoiceStatus = "canceled"
)

// LineItem represents a line item on an invoice
type LineItem struct {
	ID          uuid.UUID
	InvoiceID   uuid.UUID
	Type        LineItemType
	Description string
	Quantity    float64
	UnitPrice   float64
	Amount      float64
	CreatedAt   time.Time
}

// LineItemType represents the type of line item
type LineItemType string

const (
	LineItemTypeCall      LineItemType = "call"
	LineItemTypeCredit    LineItemType = "credit"
	LineItemTypeDebit     LineItemType = "debit"
	LineItemTypeAdjustment LineItemType = "adjustment"
	LineItemTypeManual    LineItemType = "manual"
)

// Payment represents a payment transaction
type Payment struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	InvoiceID       *uuid.UUID
	PaymentMethodID uuid.UUID
	Amount          float64
	Currency        string
	Status          PaymentStatus
	ProcessorID     string
	ProcessedAt     *time.Time
	FailureReason   *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// ReconciliationReport represents a financial reconciliation report
type ReconciliationReport struct {
	ID                uuid.UUID
	PeriodStart       time.Time
	PeriodEnd         time.Time
	TotalTransactions int
	TotalDebits       float64
	TotalCredits      float64
	Discrepancies     []Discrepancy
	Status            ReconciliationStatus
	CreatedAt         time.Time
}

// Discrepancy represents a reconciliation discrepancy
type Discrepancy struct {
	TransactionID uuid.UUID
	Type          string
	Amount        float64
	Description   string
	ResolvedAt    *time.Time
}

// ReconciliationStatus represents the status of a reconciliation
type ReconciliationStatus string

const (
	ReconciliationStatusPending   ReconciliationStatus = "pending"
	ReconciliationStatusCompleted ReconciliationStatus = "completed"
	ReconciliationStatusFailed    ReconciliationStatus = "failed"
)
