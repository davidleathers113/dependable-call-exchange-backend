package consent

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Consumer represents a person who can grant or revoke consent
type Consumer struct {
	ID          uuid.UUID
	PhoneNumber *values.PhoneNumber
	Email       *string
	FirstName   string
	LastName    string
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewConsumer creates a new consumer with validation
func NewConsumer(phoneNumber string, email *string, firstName, lastName string) (*Consumer, error) {
	if phoneNumber == "" && (email == nil || *email == "") {
		return nil, errors.NewValidationError("CONTACT_REQUIRED", "either phone number or email is required")
	}

	var phone *values.PhoneNumber
	if phoneNumber != "" {
		p, err := values.NewPhoneNumber(phoneNumber)
		if err != nil {
			return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
		}
		phone = &p
	}

	if email != nil && *email != "" {
		if err := validateEmail(*email); err != nil {
			return nil, errors.NewValidationError("INVALID_EMAIL", "invalid email format").WithCause(err)
		}
	}

	now := time.Now()
	return &Consumer{
		ID:          uuid.New(),
		PhoneNumber: phone,
		Email:       email,
		FirstName:   firstName,
		LastName:    lastName,
		Metadata:    make(map[string]interface{}),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetPrimaryContact returns the primary contact method (phone or email)
func (c *Consumer) GetPrimaryContact() string {
	if c.PhoneNumber != nil {
		return c.PhoneNumber.String()
	}
	if c.Email != nil {
		return *c.Email
	}
	return ""
}

// UpdateContact updates the consumer's contact information
func (c *Consumer) UpdateContact(phoneNumber string, email *string) error {
	if phoneNumber == "" && (email == nil || *email == "") {
		return errors.NewValidationError("CONTACT_REQUIRED", "either phone number or email is required")
	}

	if phoneNumber != "" {
		phone, err := values.NewPhoneNumber(phoneNumber)
		if err != nil {
			return errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
		}
		c.PhoneNumber = &phone
	}

	if email != nil && *email != "" {
		if err := validateEmail(*email); err != nil {
			return errors.NewValidationError("INVALID_EMAIL", "invalid email format").WithCause(err)
		}
		c.Email = email
	}

	c.UpdatedAt = time.Now()
	return nil
}

// ConsumerRepository defines the interface for consumer persistence
type ConsumerRepository interface {
	// Save creates or updates a consumer
	Save(ctx context.Context, consumer *Consumer) error

	// GetByID retrieves a consumer by ID
	GetByID(ctx context.Context, id uuid.UUID) (*Consumer, error)

	// GetByPhoneNumber retrieves a consumer by phone number
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*Consumer, error)

	// GetByEmail retrieves a consumer by email
	GetByEmail(ctx context.Context, email string) (*Consumer, error)

	// FindOrCreate finds an existing consumer or creates a new one
	FindOrCreate(ctx context.Context, phoneNumber string, email *string, firstName, lastName string) (*Consumer, error)
}

// validateEmail performs basic email validation
func validateEmail(email string) error {
	// Basic email validation - in production, use a more robust validator
	if len(email) < 5 || len(email) > 255 {
		return errors.NewValidationError("INVALID_EMAIL_LENGTH", "email must be between 5 and 255 characters")
	}
	
	// Check for @ symbol
	atIndex := -1
	for i, ch := range email {
		if ch == '@' {
			if atIndex != -1 {
				return errors.NewValidationError("MULTIPLE_AT_SYMBOLS", "email contains multiple @ symbols")
			}
			atIndex = i
		}
	}
	
	if atIndex == -1 || atIndex == 0 || atIndex == len(email)-1 {
		return errors.NewValidationError("INVALID_EMAIL_FORMAT", "email must contain @ symbol with characters before and after")
	}
	
	return nil
}