package repository

import (
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Repositories holds all repository instances
type Repositories struct {
	Account    bidding.AccountRepository
	Bid        bidding.BidRepository
	Call       CallRepository
	Compliance *ComplianceRepository
	Financial  *FinancialRepository
}

// NewRepositories creates a new repository collection
func NewRepositories(pool *pgxpool.Pool) *Repositories {
	// Convert pgxpool to sql.DB for repositories that require it
	// This creates a standard database/sql compatible connection
	db := stdlib.OpenDB(*pool.Config().ConnConfig)

	return &Repositories{
		Account:    NewAccountRepository(db),
		Bid:        NewBidRepository(db),
		Call:       NewCallRepository(db),
		Compliance: NewComplianceRepository(pool),
		Financial:  NewFinancialRepository(pool),
	}
}
