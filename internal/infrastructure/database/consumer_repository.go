package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// ConsumerRepository implements the consent.ConsumerRepository interface
type ConsumerRepository struct {
	db *pgxpool.Pool
}

// NewConsumerRepository creates a new PostgreSQL consumer repository
func NewConsumerRepository(db *pgxpool.Pool) *ConsumerRepository {
	return &ConsumerRepository{db: db}
}

// Save creates or updates a consumer
func (r *ConsumerRepository) Save(ctx context.Context, consumer *consent.Consumer) error {
	metadata, err := json.Marshal(consumer.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	var phoneNumber sql.NullString
	if consumer.PhoneNumber != nil {
		phoneNumber = sql.NullString{String: consumer.PhoneNumber.String(), Valid: true}
	}

	var email sql.NullString
	if consumer.Email != nil && *consumer.Email != "" {
		email = sql.NullString{String: *consumer.Email, Valid: true}
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO consent_consumers (
			id, phone_number, email, first_name, last_name, 
			metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			phone_number = EXCLUDED.phone_number,
			email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, consumer.ID, phoneNumber, email, consumer.FirstName, consumer.LastName,
	   metadata, consumer.CreatedAt, consumer.UpdatedAt)

	if err != nil {
		return errors.NewInternalError("failed to save consumer").WithCause(err)
	}

	return nil
}

// GetByID retrieves a consumer by ID
func (r *ConsumerRepository) GetByID(ctx context.Context, id uuid.UUID) (*consent.Consumer, error) {
	var consumer consent.Consumer
	var phoneNumber sql.NullString
	var email sql.NullString
	var metadata json.RawMessage

	err := r.db.QueryRow(ctx, `
		SELECT id, phone_number, email, first_name, last_name, 
		       metadata, created_at, updated_at
		FROM consent_consumers
		WHERE id = $1
	`, id).Scan(&consumer.ID, &phoneNumber, &email, &consumer.FirstName,
	            &consumer.LastName, &metadata, &consumer.CreatedAt, &consumer.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFoundError("consumer not found")
		}
		return nil, errors.NewInternalError("failed to get consumer").WithCause(err)
	}

	// Handle phone number
	if phoneNumber.Valid {
		phone, err := values.NewPhoneNumber(phoneNumber.String)
		if err != nil {
			return nil, errors.NewInternalError("invalid phone number in database").WithCause(err)
		}
		consumer.PhoneNumber = &phone
	}

	// Handle email
	if email.Valid {
		consumer.Email = &email.String
	}

	// Unmarshal metadata
	if metadata != nil {
		err = json.Unmarshal(metadata, &consumer.Metadata)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}
	} else {
		consumer.Metadata = make(map[string]interface{})
	}

	return &consumer, nil
}

// GetByPhoneNumber retrieves a consumer by phone number
func (r *ConsumerRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*consent.Consumer, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id FROM consent_consumers
		WHERE phone_number = $1
		LIMIT 1
	`, phoneNumber).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found, but not an error
		}
		return nil, errors.NewInternalError("failed to query by phone").WithCause(err)
	}

	return r.GetByID(ctx, id)
}

// GetByEmail retrieves a consumer by email
func (r *ConsumerRepository) GetByEmail(ctx context.Context, email string) (*consent.Consumer, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id FROM consent_consumers
		WHERE email = $1
		LIMIT 1
	`, email).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found, but not an error
		}
		return nil, errors.NewInternalError("failed to query by email").WithCause(err)
	}

	return r.GetByID(ctx, id)
}

// FindOrCreate finds an existing consumer or creates a new one
func (r *ConsumerRepository) FindOrCreate(ctx context.Context, phoneNumber string, email *string, firstName, lastName string) (*consent.Consumer, error) {
	// Start transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.NewInternalError("failed to begin transaction").WithCause(err)
	}
	defer tx.Rollback(ctx)

	// Try to find by phone number first
	if phoneNumber != "" {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id FROM consent_consumers WHERE phone_number = $1
		`, phoneNumber).Scan(&id)
		
		if err == nil {
			// Found by phone number
			if err := tx.Commit(ctx); err != nil {
				return nil, errors.NewInternalError("failed to commit transaction").WithCause(err)
			}
			return r.GetByID(ctx, id)
		} else if err != pgx.ErrNoRows {
			return nil, errors.NewInternalError("failed to query by phone").WithCause(err)
		}
	}

	// Try to find by email if not found by phone
	if email != nil && *email != "" {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id FROM consent_consumers WHERE email = $1
		`, *email).Scan(&id)
		
		if err == nil {
			// Found by email, update phone if provided
			if phoneNumber != "" {
				_, err = tx.Exec(ctx, `
					UPDATE consent_consumers 
					SET phone_number = $2, updated_at = NOW()
					WHERE id = $1
				`, id, phoneNumber)
				if err != nil {
					return nil, errors.NewInternalError("failed to update phone").WithCause(err)
				}
			}
			
			if err := tx.Commit(ctx); err != nil {
				return nil, errors.NewInternalError("failed to commit transaction").WithCause(err)
			}
			return r.GetByID(ctx, id)
		} else if err != pgx.ErrNoRows {
			return nil, errors.NewInternalError("failed to query by email").WithCause(err)
		}
	}

	// Not found, create new consumer
	consumer, err := consent.NewConsumer(phoneNumber, email, firstName, lastName)
	if err != nil {
		return nil, err
	}

	// Save in transaction
	metadata, err := json.Marshal(consumer.Metadata)
	if err != nil {
		return nil, errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	var phoneNumberNull sql.NullString
	if consumer.PhoneNumber != nil {
		phoneNumberNull = sql.NullString{String: consumer.PhoneNumber.String(), Valid: true}
	}

	var emailNull sql.NullString
	if consumer.Email != nil && *consumer.Email != "" {
		emailNull = sql.NullString{String: *consumer.Email, Valid: true}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO consent_consumers (
			id, phone_number, email, first_name, last_name, 
			metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, consumer.ID, phoneNumberNull, emailNull, consumer.FirstName, consumer.LastName,
	   metadata, consumer.CreatedAt, consumer.UpdatedAt)

	if err != nil {
		return nil, errors.NewInternalError("failed to insert consumer").WithCause(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.NewInternalError("failed to commit transaction").WithCause(err)
	}

	return consumer, nil
}