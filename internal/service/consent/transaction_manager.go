package consent

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TransactionManager manages database transactions for consent operations
type TransactionManager interface {
	// ExecuteInTransaction executes a function within a database transaction
	ExecuteInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	
	// ExecuteInTransactionWithResult executes a function within a transaction and returns a result
	ExecuteInTransactionWithResult(ctx context.Context, fn func(ctx context.Context) (interface{}, error)) (interface{}, error)
}

// TransactionalService wraps the consent service with transaction management
type TransactionalService struct {
	service Service
	txMgr   TransactionManager
	logger  *zap.Logger
}

// NewTransactionalService creates a new transactional consent service
func NewTransactionalService(service Service, txMgr TransactionManager, logger *zap.Logger) Service {
	return &TransactionalService{
		service: service,
		txMgr:   txMgr,
		logger:  logger,
	}
}

// GrantConsent grants consent within a transaction
func (ts *TransactionalService) GrantConsent(ctx context.Context, req GrantConsentRequest) (*ConsentResponse, error) {
	result, err := ts.txMgr.ExecuteInTransactionWithResult(ctx, func(txCtx context.Context) (interface{}, error) {
		return ts.service.GrantConsent(txCtx, req)
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.(*ConsentResponse), nil
}

// RevokeConsent revokes consent within a transaction
func (ts *TransactionalService) RevokeConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) error {
	return ts.txMgr.ExecuteInTransaction(ctx, func(txCtx context.Context) error {
		return ts.service.RevokeConsent(txCtx, consumerID, consentType)
	})
}

// UpdateConsent updates consent within a transaction
func (ts *TransactionalService) UpdateConsent(ctx context.Context, req UpdateConsentRequest) (*ConsentResponse, error) {
	result, err := ts.txMgr.ExecuteInTransactionWithResult(ctx, func(txCtx context.Context) (interface{}, error) {
		return ts.service.UpdateConsent(txCtx, req)
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.(*ConsentResponse), nil
}

// GetConsent retrieves consent (read-only, no transaction needed)
func (ts *TransactionalService) GetConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) (*ConsentResponse, error) {
	return ts.service.GetConsent(ctx, consumerID, consentType)
}

// GetActiveConsents retrieves active consents (read-only, no transaction needed)
func (ts *TransactionalService) GetActiveConsents(ctx context.Context, consumerID uuid.UUID) ([]*ConsentResponse, error) {
	return ts.service.GetActiveConsents(ctx, consumerID)
}

// CheckConsent checks consent status (read-only, no transaction needed)
func (ts *TransactionalService) CheckConsent(ctx context.Context, phoneNumber string, consentType consent.Type) (*ConsentStatus, error) {
	return ts.service.CheckConsent(ctx, phoneNumber, consentType)
}

// CreateConsumer creates a consumer within a transaction
func (ts *TransactionalService) CreateConsumer(ctx context.Context, req CreateConsumerRequest) (*ConsumerResponse, error) {
	result, err := ts.txMgr.ExecuteInTransactionWithResult(ctx, func(txCtx context.Context) (interface{}, error) {
		return ts.service.CreateConsumer(txCtx, req)
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.(*ConsumerResponse), nil
}

// GetConsumerByPhone retrieves consumer by phone (read-only, no transaction needed)
func (ts *TransactionalService) GetConsumerByPhone(ctx context.Context, phoneNumber string) (*ConsumerResponse, error) {
	return ts.service.GetConsumerByPhone(ctx, phoneNumber)
}

// GetConsumerByEmail retrieves consumer by email (read-only, no transaction needed)
func (ts *TransactionalService) GetConsumerByEmail(ctx context.Context, email string) (*ConsumerResponse, error) {
	return ts.service.GetConsumerByEmail(ctx, email)
}

// ImportConsents imports consents within a transaction
func (ts *TransactionalService) ImportConsents(ctx context.Context, req ImportConsentsRequest) (*ImportResult, error) {
	// For large imports, we might want to batch transactions
	// For now, execute the entire import in one transaction
	result, err := ts.txMgr.ExecuteInTransactionWithResult(ctx, func(txCtx context.Context) (interface{}, error) {
		return ts.service.ImportConsents(txCtx, req)
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.(*ImportResult), nil
}

// ExportConsents exports consents (read-only, no transaction needed)
func (ts *TransactionalService) ExportConsents(ctx context.Context, req ExportConsentsRequest) (*ExportResult, error) {
	return ts.service.ExportConsents(ctx, req)
}

// GetConsentMetrics retrieves metrics (read-only, no transaction needed)
func (ts *TransactionalService) GetConsentMetrics(ctx context.Context, req MetricsRequest) (*ConsentMetrics, error) {
	return ts.service.GetConsentMetrics(ctx, req)
}

// SQLTransactionManager implements TransactionManager using database/sql
type SQLTransactionManager struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewSQLTransactionManager creates a new SQL transaction manager
func NewSQLTransactionManager(db *sql.DB, logger *zap.Logger) TransactionManager {
	return &SQLTransactionManager{
		db:     db,
		logger: logger,
	}
}

// ExecuteInTransaction executes a function within a database transaction
func (tm *SQLTransactionManager) ExecuteInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.NewInternalError("failed to begin transaction").WithCause(err)
	}

	// Store transaction in context
	txCtx := context.WithValue(ctx, transactionKey{}, tx)

	// Execute the function
	if err := fn(txCtx); err != nil {
		tm.logger.Debug("rolling back transaction due to error", zap.Error(err))
		if rbErr := tx.Rollback(); rbErr != nil {
			tm.logger.Error("failed to rollback transaction", zap.Error(rbErr))
			return errors.NewInternalError("transaction rollback failed").WithCause(rbErr)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		tm.logger.Error("failed to commit transaction", zap.Error(err))
		return errors.NewInternalError("transaction commit failed").WithCause(err)
	}

	tm.logger.Debug("transaction committed successfully")
	return nil
}

// ExecuteInTransactionWithResult executes a function within a transaction and returns a result
func (tm *SQLTransactionManager) ExecuteInTransactionWithResult(ctx context.Context, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.NewInternalError("failed to begin transaction").WithCause(err)
	}

	// Store transaction in context
	txCtx := context.WithValue(ctx, transactionKey{}, tx)

	// Execute the function
	result, err := fn(txCtx)
	if err != nil {
		tm.logger.Debug("rolling back transaction due to error", zap.Error(err))
		if rbErr := tx.Rollback(); rbErr != nil {
			tm.logger.Error("failed to rollback transaction", zap.Error(rbErr))
			return nil, errors.NewInternalError("transaction rollback failed").WithCause(rbErr)
		}
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		tm.logger.Error("failed to commit transaction", zap.Error(err))
		return nil, errors.NewInternalError("transaction commit failed").WithCause(err)
	}

	tm.logger.Debug("transaction committed successfully")
	return result, nil
}

// transactionKey is used to store transaction in context
type transactionKey struct{}

// GetTransaction retrieves a transaction from context
func GetTransaction(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(transactionKey{}).(*sql.Tx)
	return tx, ok
}

// BatchTransactionManager handles large batch operations with chunked transactions
type BatchTransactionManager struct {
	txMgr     TransactionManager
	batchSize int
	logger    *zap.Logger
}

// NewBatchTransactionManager creates a new batch transaction manager
func NewBatchTransactionManager(txMgr TransactionManager, batchSize int, logger *zap.Logger) *BatchTransactionManager {
	return &BatchTransactionManager{
		txMgr:     txMgr,
		batchSize: batchSize,
		logger:    logger,
	}
}

// ExecuteBatch executes operations in batches with separate transactions
func (btm *BatchTransactionManager) ExecuteBatch(ctx context.Context, items []interface{}, processor func(ctx context.Context, batch []interface{}) error) error {
	totalItems := len(items)
	btm.logger.Info("starting batch operation", 
		zap.Int("total_items", totalItems),
		zap.Int("batch_size", btm.batchSize),
	)

	for i := 0; i < totalItems; i += btm.batchSize {
		end := i + btm.batchSize
		if end > totalItems {
			end = totalItems
		}

		batch := items[i:end]
		batchNum := (i / btm.batchSize) + 1
		totalBatches := (totalItems + btm.batchSize - 1) / btm.batchSize

		btm.logger.Debug("processing batch",
			zap.Int("batch_num", batchNum),
			zap.Int("total_batches", totalBatches),
			zap.Int("batch_size", len(batch)),
		)

		// Execute batch in transaction
		if err := btm.txMgr.ExecuteInTransaction(ctx, func(txCtx context.Context) error {
			return processor(txCtx, batch)
		}); err != nil {
			return fmt.Errorf("batch %d failed: %w", batchNum, err)
		}
	}

	btm.logger.Info("batch operation completed successfully", zap.Int("total_items", totalItems))
	return nil
}