package audit

import (
	"time"

	"github.com/google/uuid"
)

// Financial Domain Events
// These events are published when financial transactions occur

// PaymentProcessedEvent is published when a payment is processed
type PaymentProcessedEvent struct {
	*BaseDomainEvent
	PaymentID         uuid.UUID `json:"payment_id"`
	TransactionID     uuid.UUID `json:"transaction_id"`
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	BidID             *uuid.UUID `json:"bid_id,omitempty"`
	PayerID           uuid.UUID `json:"payer_id"`
	PayeeID           uuid.UUID `json:"payee_id"`
	Amount            string    `json:"amount"`
	Currency          string    `json:"currency"`
	PaymentMethod     string    `json:"payment_method"`
	PaymentProvider   string    `json:"payment_provider"`
	PaymentStatus     string    `json:"payment_status"`
	ProcessedAt       time.Time `json:"processed_at"`
	SettledAt         *time.Time `json:"settled_at,omitempty"`
	FeeAmount         string    `json:"fee_amount,omitempty"`
	NetAmount         string    `json:"net_amount"`
	ExchangeRate      *float64  `json:"exchange_rate,omitempty"`
	Reference         string    `json:"reference"`
	Memo              string    `json:"memo,omitempty"`
}

// NewPaymentProcessedEvent creates a new payment processed event
func NewPaymentProcessedEvent(actorID string, paymentID, transactionID, payerID, payeeID uuid.UUID, amount string) *PaymentProcessedEvent {
	base := NewBaseDomainEvent(EventPaymentProcessed, actorID, paymentID.String(), "payment_processed")
	base.TargetType = "payment"
	base.ActorType = "system"

	event := &PaymentProcessedEvent{
		BaseDomainEvent: base,
		PaymentID:       paymentID,
		TransactionID:   transactionID,
		PayerID:         payerID,
		PayeeID:         payeeID,
		Amount:          amount,
		Currency:        "USD",
		PaymentMethod:   "credit_card",
		PaymentProvider: "stripe",
		PaymentStatus:   "completed",
		ProcessedAt:     time.Now().UTC(),
		NetAmount:       amount,
		Reference:       paymentID.String(),
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("financial_data")
	event.AddDataClass("payment_data")
	event.AddDataClass("transaction_data")

	// Set metadata for payment processing
	event.SetMetadata("action_type", "payment_processing")
	event.SetMetadata("payer_id", payerID.String())
	event.SetMetadata("payee_id", payeeID.String())
	event.SetMetadata("amount", amount)
	event.SetMetadata("currency", "USD")

	return event
}

// TransactionCompletedEvent is published when a transaction is completed
type TransactionCompletedEvent struct {
	*BaseDomainEvent
	TransactionID     uuid.UUID  `json:"transaction_id"`
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	BidID             *uuid.UUID `json:"bid_id,omitempty"`
	BuyerID           uuid.UUID  `json:"buyer_id"`
	SellerID          uuid.UUID  `json:"seller_id"`
	TransactionType   string     `json:"transaction_type"`
	Amount            string     `json:"amount"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       time.Time  `json:"completed_at"`
	Duration          int64      `json:"duration_ms"`
	PaymentMethodID   string     `json:"payment_method_id,omitempty"`
	BillingAddress    string     `json:"billing_address,omitempty"`
	TaxAmount         string     `json:"tax_amount,omitempty"`
	FeeAmount         string     `json:"fee_amount,omitempty"`
	NetAmount         string     `json:"net_amount"`
	Reference         string     `json:"reference"`
	InvoiceID         *uuid.UUID `json:"invoice_id,omitempty"`
}

// NewTransactionCompletedEvent creates a new transaction completed event
func NewTransactionCompletedEvent(actorID string, transactionID, buyerID, sellerID uuid.UUID, amount string) *TransactionCompletedEvent {
	base := NewBaseDomainEvent(EventPaymentProcessed, actorID, transactionID.String(), "transaction_completed")
	base.TargetType = "transaction"
	base.ActorType = "system"

	event := &TransactionCompletedEvent{
		BaseDomainEvent: base,
		TransactionID:   transactionID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		TransactionType: "call_payment",
		Amount:          amount,
		Currency:        "USD",
		Status:          "completed",
		StartedAt:       time.Now().UTC().Add(-1 * time.Minute),
		CompletedAt:     time.Now().UTC(),
		Duration:        60000, // 1 minute in milliseconds
		NetAmount:       amount,
		Reference:       transactionID.String(),
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("financial_data")
	event.AddDataClass("transaction_data")
	event.AddDataClass("payment_data")

	// Set metadata for transaction completion
	event.SetMetadata("action_type", "transaction_completion")
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("amount", amount)
	event.SetMetadata("currency", "USD")

	return event
}

// ChargebackInitiatedEvent is published when a chargeback is initiated
type ChargebackInitiatedEvent struct {
	*BaseDomainEvent
	ChargebackID      uuid.UUID  `json:"chargeback_id"`
	PaymentID         uuid.UUID  `json:"payment_id"`
	TransactionID     uuid.UUID  `json:"transaction_id"`
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	PayerID           uuid.UUID  `json:"payer_id"`
	PayeeID           uuid.UUID  `json:"payee_id"`
	Amount            string     `json:"amount"`
	Currency          string     `json:"currency"`
	ReasonCode        string     `json:"reason_code"`
	ReasonDescription string     `json:"reason_description"`
	InitiatedAt       time.Time  `json:"initiated_at"`
	DueDate           time.Time  `json:"due_date"`
	Status            string     `json:"status"`
	PaymentProvider   string     `json:"payment_provider"`
	EvidenceRequired  []string   `json:"evidence_required"`
	DisputeID         string     `json:"dispute_id,omitempty"`
	ChargebackType    string     `json:"chargeback_type"`
}

// NewChargebackInitiatedEvent creates a new chargeback initiated event
func NewChargebackInitiatedEvent(actorID string, chargebackID, paymentID, transactionID, payerID, payeeID uuid.UUID, amount, reasonCode string) *ChargebackInitiatedEvent {
	base := NewBaseDomainEvent(EventPaymentProcessed, actorID, chargebackID.String(), "chargeback_initiated")
	base.TargetType = "chargeback"
	base.ActorType = "system"

	event := &ChargebackInitiatedEvent{
		BaseDomainEvent:   base,
		ChargebackID:      chargebackID,
		PaymentID:         paymentID,
		TransactionID:     transactionID,
		PayerID:           payerID,
		PayeeID:           payeeID,
		Amount:            amount,
		Currency:          "USD",
		ReasonCode:        reasonCode,
		ReasonDescription: getChargebackReasonDescription(reasonCode),
		InitiatedAt:       time.Now().UTC(),
		DueDate:           time.Now().UTC().AddDate(0, 0, 7), // 7 days to respond
		Status:            "open",
		PaymentProvider:   "stripe",
		EvidenceRequired:  []string{"receipt", "communication", "shipping_proof"},
		ChargebackType:    "dispute",
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()
	event.MarkSecuritySensitive()

	// Add relevant data classes
	event.AddDataClass("financial_data")
	event.AddDataClass("chargeback_data")
	event.AddDataClass("dispute_data")

	// Set metadata for chargeback initiation
	event.SetMetadata("action_type", "chargeback_initiation")
	event.SetMetadata("payer_id", payerID.String())
	event.SetMetadata("payee_id", payeeID.String())
	event.SetMetadata("amount", amount)
	event.SetMetadata("reason_code", reasonCode)

	return event
}

// RefundProcessedEvent is published when a refund is processed
type RefundProcessedEvent struct {
	*BaseDomainEvent
	RefundID          uuid.UUID  `json:"refund_id"`
	PaymentID         uuid.UUID  `json:"payment_id"`
	TransactionID     uuid.UUID  `json:"transaction_id"`
	CallID            *uuid.UUID `json:"call_id,omitempty"`
	BidID             *uuid.UUID `json:"bid_id,omitempty"`
	RefundeeID        uuid.UUID  `json:"refundee_id"`
	RefunderID        uuid.UUID  `json:"refunder_id"`
	OriginalAmount    string     `json:"original_amount"`
	RefundAmount      string     `json:"refund_amount"`
	Currency          string     `json:"currency"`
	RefundReason      string     `json:"refund_reason"`
	RefundType        string     `json:"refund_type"`
	RefundMethod      string     `json:"refund_method"`
	ProcessedAt       time.Time  `json:"processed_at"`
	Status            string     `json:"status"`
	PaymentProvider   string     `json:"payment_provider"`
	Reference         string     `json:"reference"`
	IsPartialRefund   bool       `json:"is_partial_refund"`
}

// NewRefundProcessedEvent creates a new refund processed event
func NewRefundProcessedEvent(actorID string, refundID, paymentID, transactionID, refundeeID, refunderID uuid.UUID, refundAmount string) *RefundProcessedEvent {
	base := NewBaseDomainEvent(EventPaymentProcessed, actorID, refundID.String(), "refund_processed")
	base.TargetType = "refund"
	base.ActorType = "system"

	event := &RefundProcessedEvent{
		BaseDomainEvent: base,
		RefundID:        refundID,
		PaymentID:       paymentID,
		TransactionID:   transactionID,
		RefundeeID:      refundeeID,
		RefunderID:      refunderID,
		RefundAmount:    refundAmount,
		Currency:        "USD",
		RefundReason:    "customer_request",
		RefundType:      "full",
		RefundMethod:    "original_payment_method",
		ProcessedAt:     time.Now().UTC(),
		Status:          "completed",
		PaymentProvider: "stripe",
		Reference:       refundID.String(),
		IsPartialRefund: false,
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("financial_data")
	event.AddDataClass("refund_data")
	event.AddDataClass("transaction_data")

	// Set metadata for refund processing
	event.SetMetadata("action_type", "refund_processing")
	event.SetMetadata("refundee_id", refundeeID.String())
	event.SetMetadata("refunder_id", refunderID.String())
	event.SetMetadata("refund_amount", refundAmount)
	event.SetMetadata("currency", "USD")

	return event
}

// PayoutInitiatedEvent is published when a payout to a seller is initiated
type PayoutInitiatedEvent struct {
	*BaseDomainEvent
	PayoutID        uuid.UUID `json:"payout_id"`
	SellerID        uuid.UUID `json:"seller_id"`
	Amount          string    `json:"amount"`
	Currency        string    `json:"currency"`
	PayoutMethod    string    `json:"payout_method"`
	BankAccountID   string    `json:"bank_account_id,omitempty"`
	RoutingNumber   string    `json:"routing_number,omitempty"`
	AccountNumber   string    `json:"account_number_masked,omitempty"`
	InitiatedAt     time.Time `json:"initiated_at"`
	EstimatedArrival time.Time `json:"estimated_arrival"`
	Status          string    `json:"status"`
	PayoutProvider  string    `json:"payout_provider"`
	Reference       string    `json:"reference"`
	TransactionIDs  []uuid.UUID `json:"transaction_ids"`
	TaxWithheld     string    `json:"tax_withheld,omitempty"`
	NetAmount       string    `json:"net_amount"`
}

// NewPayoutInitiatedEvent creates a new payout initiated event
func NewPayoutInitiatedEvent(actorID string, payoutID, sellerID uuid.UUID, amount string) *PayoutInitiatedEvent {
	base := NewBaseDomainEvent(EventPaymentProcessed, actorID, payoutID.String(), "payout_initiated")
	base.TargetType = "payout"
	base.ActorType = "system"

	event := &PayoutInitiatedEvent{
		BaseDomainEvent:  base,
		PayoutID:         payoutID,
		SellerID:         sellerID,
		Amount:           amount,
		Currency:         "USD",
		PayoutMethod:     "bank_transfer",
		InitiatedAt:      time.Now().UTC(),
		EstimatedArrival: time.Now().UTC().AddDate(0, 0, 1), // Next business day
		Status:           "pending",
		PayoutProvider:   "stripe",
		Reference:        payoutID.String(),
		TransactionIDs:   make([]uuid.UUID, 0),
		NetAmount:        amount,
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("financial_data")
	event.AddDataClass("payout_data")
	event.AddDataClass("banking_data")

	// Set metadata for payout initiation
	event.SetMetadata("action_type", "payout_initiation")
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("amount", amount)
	event.SetMetadata("currency", "USD")

	return event
}

// FinancialComplianceCheckEvent is published when financial compliance is checked
type FinancialComplianceCheckEvent struct {
	*BaseDomainEvent
	CheckID           uuid.UUID `json:"check_id"`
	TransactionID     uuid.UUID `json:"transaction_id"`
	PaymentID         *uuid.UUID `json:"payment_id,omitempty"`
	AccountID         uuid.UUID `json:"account_id"`
	CheckType         string    `json:"check_type"`
	Regulation        string    `json:"regulation"`
	Amount            string    `json:"amount"`
	Currency          string    `json:"currency"`
	ComplianceResult  string    `json:"compliance_result"`
	RiskScore         float64   `json:"risk_score"`
	RulesChecked      []string  `json:"rules_checked"`
	RuleResults       map[string]bool `json:"rule_results"`
	Flags             []string  `json:"flags"`
	RecommendedAction string    `json:"recommended_action"`
	CheckedAt         time.Time `json:"checked_at"`
	ProcessorResponse map[string]interface{} `json:"processor_response,omitempty"`
}

// NewFinancialComplianceCheckEvent creates a new financial compliance check event
func NewFinancialComplianceCheckEvent(actorID string, checkID, transactionID, accountID uuid.UUID, checkType string) *FinancialComplianceCheckEvent {
	base := NewBaseDomainEvent(EventAuthSuccess, actorID, checkID.String(), "financial_compliance_checked")
	base.TargetType = "compliance_check"
	base.ActorType = "system"

	event := &FinancialComplianceCheckEvent{
		BaseDomainEvent:   base,
		CheckID:           checkID,
		TransactionID:     transactionID,
		AccountID:         accountID,
		CheckType:         checkType,
		Regulation:        "BSA",
		Currency:          "USD",
		ComplianceResult:  "compliant",
		RiskScore:         0.1,
		RulesChecked:      []string{"aml", "kyc", "sanctions"},
		RuleResults:       make(map[string]bool),
		Flags:             make([]string, 0),
		RecommendedAction: "proceed",
		CheckedAt:         time.Now().UTC(),
	}

	// Initialize rule results
	event.RuleResults["aml"] = true
	event.RuleResults["kyc"] = true
	event.RuleResults["sanctions"] = true

	// Mark as containing financial data
	event.MarkFinancialData()
	event.MarkSecuritySensitive()

	// Add relevant data classes
	event.AddDataClass("compliance_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("risk_data")

	// Set metadata for compliance check
	event.SetMetadata("action_type", "financial_compliance_check")
	event.SetMetadata("transaction_id", transactionID.String())
	event.SetMetadata("account_id", accountID.String())
	event.SetMetadata("check_type", checkType)

	return event
}

// Helper function to get chargeback reason descriptions
func getChargebackReasonDescription(reasonCode string) string {
	descriptions := map[string]string{
		"4855": "Goods or Services Not Provided",
		"4534": "Multiple Processing",
		"4808": "Authorization-Related Chargeback",
		"4841": "Cancelled Recurring Transaction",
		"4849": "Questionable Merchant Activity",
		"4853": "Cardholder Dispute",
		"4854": "Cardholder Dispute - Not Elsewhere Classified",
		"4859": "Addendum, No-show, or ATM Dispute",
		"4860": "Credit Not Processed",
		"4862": "Counterfeit Transaction",
		"4863": "Cardholder Does Not Recognize - Potential Fraud",
		"4870": "Chip Liability Shift",
		"4871": "Chip Liability Shift - Stolen Card",
	}
	
	if desc, exists := descriptions[reasonCode]; exists {
		return desc
	}
	return "Unknown chargeback reason"
}