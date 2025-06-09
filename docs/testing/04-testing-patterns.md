# Testing Patterns

**Version:** 1.0.0  
**Date:** June 9, 2025  
**Status:** Active

## Table of Contents
- [Introduction](#introduction)
- [Test Data Builders](#test-data-builders)
- [Table-Driven Tests](#table-driven-tests)
- [Test Fixtures](#test-fixtures)
- [Mocking Strategies](#mocking-strategies)
- [Assertion Patterns](#assertion-patterns)
- [Error Testing](#error-testing)
- [Concurrent Testing](#concurrent-testing)
- [Test Organization](#test-organization)

## Introduction

This guide documents common testing patterns and best practices used throughout the Dependable Call Exchange Backend. These patterns promote consistency, readability, and maintainability across our test suite.

## Test Data Builders

### Builder Pattern Implementation

```go
// internal/testutil/builders/call_builder.go
package builders

import (
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain"
)

type CallBuilder struct {
    call *domain.Call
}

func NewCallBuilder() *CallBuilder {
    // Sensible defaults
    return &CallBuilder{
        call: &domain.Call{
            ID:         uuid.New(),
            FromNumber: "+15551234567",
            ToNumber:   "+15559876543",
            Status:     domain.CallStatusPending,
            Type:       domain.CallTypeInbound,
            CreatedAt:  time.Now(),
            UpdatedAt:  time.Now(),
        },
    }
}

// Fluent interface methods
func (b *CallBuilder) WithID(id uuid.UUID) *CallBuilder {
    b.call.ID = id
    return b
}

func (b *CallBuilder) WithPhoneNumbers(from, to string) *CallBuilder {
    b.call.FromNumber = from
    b.call.ToNumber = to
    return b
}

func (b *CallBuilder) WithStatus(status domain.CallStatus) *CallBuilder {
    b.call.Status = status
    return b
}

func (b *CallBuilder) WithBuyer(buyerID uuid.UUID) *CallBuilder {
    b.call.BuyerID = &buyerID
    return b
}

func (b *CallBuilder) WithDuration(seconds int) *CallBuilder {
    b.call.Duration = &seconds
    b.call.StartedAt = &b.call.CreatedAt
    endTime := b.call.CreatedAt.Add(time.Duration(seconds) * time.Second)
    b.call.EndedAt = &endTime
    return b
}

func (b *CallBuilder) Build() *domain.Call {
    return b.call
}

// Usage in tests
func TestCallService(t *testing.T) {
    // Simple case with defaults
    call1 := builders.NewCallBuilder().Build()
    
    // Complex case with customization
    call2 := builders.NewCallBuilder().
        WithStatus(domain.CallStatusActive).
        WithBuyer(testBuyerID).
        WithDuration(180).
        Build()
}
```

### Account Builder

```go
type AccountBuilder struct {
    account *domain.Account
}

func NewAccountBuilder() *AccountBuilder {
    return &AccountBuilder{
        account: &domain.Account{
            ID:               uuid.New(),
            Email:           "test@example.com",
            Company:         "Test Company",
            Type:            domain.AccountTypeBuyer,
            Status:          domain.AccountStatusActive,
            Balance:         0,
            CreditLimit:     1000.00,
            QualityScore:    0.85,
            EmailVerified:   true,
            PhoneVerified:   true,
            ComplianceVerified: true,
            CreatedAt:       time.Now(),
            UpdatedAt:       time.Now(),
        },
    }
}

func (b *AccountBuilder) WithType(accountType domain.AccountType) *AccountBuilder {
    b.account.Type = accountType
    return b
}

func (b *AccountBuilder) WithBalance(balance float64) *AccountBuilder {
    b.account.Balance = balance
    return b
}

func (b *AccountBuilder) AsSeller() *AccountBuilder {
    b.account.Type = domain.AccountTypeSeller
    return b
}

func (b *AccountBuilder) Build() *domain.Account {
    return b.account
}
```

## Table-Driven Tests

### Basic Table-Driven Pattern

```go
func TestPhoneNumberValidation(t *testing.T) {
    tests := []struct {
        name        string
        phoneNumber string
        wantValid   bool
        wantErr     string
    }{
        {
            name:        "valid US number",
            phoneNumber: "+15551234567",
            wantValid:   true,
        },
        {
            name:        "missing country code",
            phoneNumber: "5551234567",
            wantValid:   false,
            wantErr:     "missing country code",
        },
        {
            name:        "invalid format",
            phoneNumber: "555-123-4567",
            wantValid:   false,
            wantErr:     "invalid format",
        },
        {
            name:        "too short",
            phoneNumber: "+1555123",
            wantValid:   false,
            wantErr:     "invalid length",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            valid, err := ValidatePhoneNumber(tt.phoneNumber)
            
            if tt.wantValid {
                require.NoError(t, err)
                assert.True(t, valid)
            } else {
                require.Error(t, err)
                assert.False(t, valid)
                assert.Contains(t, err.Error(), tt.wantErr)
            }
        })
    }
}
```

### Advanced Table-Driven Pattern

```go
func TestCallRepository_Operations(t *testing.T) {
    ctx := context.Background()
    db := setupTestDB(t)
    repo := repository.NewCallRepository(db)
    
    tests := []struct {
        name     string
        setup    func(t *testing.T)
        execute  func(t *testing.T) error
        validate func(t *testing.T)
        wantErr  bool
    }{
        {
            name: "create call successfully",
            setup: func(t *testing.T) {
                // Clean database
                truncateTables(t, db)
            },
            execute: func(t *testing.T) error {
                call := builders.NewCallBuilder().Build()
                return repo.Create(ctx, call)
            },
            validate: func(t *testing.T) {
                count := getTableCount(t, db, "calls")
                assert.Equal(t, 1, count)
            },
            wantErr: false,
        },
        {
            name: "create duplicate call fails",
            setup: func(t *testing.T) {
                call := builders.NewCallBuilder().
                    WithID(testCallID).
                    Build()
                err := repo.Create(ctx, call)
                require.NoError(t, err)
            },
            execute: func(t *testing.T) error {
                call := builders.NewCallBuilder().
                    WithID(testCallID).
                    Build()
                return repo.Create(ctx, call)
            },
            validate: func(t *testing.T) {
                // Still only one call in database
                count := getTableCount(t, db, "calls")
                assert.Equal(t, 1, count)
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup(t)
            
            err := tt.execute(t)
            
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
            
            tt.validate(t)
        })
    }
}
```

## Test Fixtures

### Fixture Factory Pattern

```go
// internal/testutil/fixtures/factory.go
package fixtures
