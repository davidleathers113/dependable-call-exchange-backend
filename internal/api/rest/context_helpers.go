package rest

import (
	"context"
	"errors"
	
	"github.com/google/uuid"
)

// getUserFromContext extracts authenticated user information from context
func getUserFromContext(ctx context.Context) (userID uuid.UUID, accountType string, err error) {
	// Extract user ID
	userIDVal := ctx.Value(contextKeyUserID)
	if userIDVal == nil {
		return uuid.Nil, "", errors.New("user ID not found in context")
	}
	
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, "", errors.New("invalid user ID type in context")
	}
	
	// Extract account type
	accountTypeVal := ctx.Value(contextKeyAccountType)
	if accountTypeVal == nil {
		return uuid.Nil, "", errors.New("account type not found in context")
	}
	
	accountType, ok = accountTypeVal.(string)
	if !ok {
		return uuid.Nil, "", errors.New("invalid account type in context")
	}
	
	return userID, accountType, nil
}

// getAccountIDFromContext extracts the account ID from context
func getAccountIDFromContext(ctx context.Context) (uuid.UUID, error) {
	accountIDVal := ctx.Value(contextKey("account_id"))
	if accountIDVal == nil {
		return uuid.Nil, errors.New("account ID not found in context")
	}
	
	accountID, ok := accountIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid account ID type in context")
	}
	
	return accountID, nil
}

// getPermissionsFromContext extracts user permissions from context
func getPermissionsFromContext(ctx context.Context) []string {
	permissionsVal := ctx.Value(contextKey("permissions"))
	if permissionsVal == nil {
		return []string{}
	}
	
	permissions, ok := permissionsVal.([]string)
	if !ok {
		return []string{}
	}
	
	return permissions
}

// hasPermission checks if the user has a specific permission
func hasPermission(ctx context.Context, permission string) bool {
	permissions := getPermissionsFromContext(ctx)
	for _, p := range permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// requireBuyer ensures the user is a buyer
func requireBuyer(ctx context.Context) error {
	_, accountType, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}
	
	if accountType != "buyer" {
		return errors.New("buyer account required")
	}
	
	return nil
}

// requireSeller ensures the user is a seller
func requireSeller(ctx context.Context) error {
	_, accountType, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}
	
	if accountType != "seller" {
		return errors.New("seller account required")
	}
	
	return nil
}