//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/financial"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFinancial_BillingCycle tests complete billing cycle
func TestFinancial_BillingCycle(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("End-to-End Billing Flow", func(t *testing.T) {
		// Create accounts with initial balance
		buyer := createTestAccountWithBalance(t, ctx, testDB, "billing-buyer", account.TypeBuyer, 1000.00)
		seller1 := createTestAccountWithBalance(t, ctx, testDB, "billing-seller1", account.TypeSeller, 0.00)
		seller2 := createTestAccountWithBalance(t, ctx, testDB, "billing-seller2", account.TypeSeller, 0.00)
		
		// Track initial balances
		initialBuyerBalance := getAccountBalance(t, server, buyer.ID)
		initialSeller1Balance := getAccountBalance(t, server, seller1.ID)
		initialSeller2Balance := getAccountBalance(t, server, seller2.ID)
		
		// Process multiple calls
		totalCalls := 10
		var totalCost float64
		var seller1Earnings float64
		var seller2Earnings float64
		
		for i := 0; i < totalCalls; i++ {
			// Create call
			call := simulateIncomingCall(t, server, buyer.ID, 
				fmt.Sprintf("+1415555%04d", i), "+18005551234")
			
			// Start auction
			auction := startAuction(t, server, call.ID)
			
			// Sellers bid (alternating winner)
			var winningBid *bid.Bid
			if i%2 == 0 {
				placeBid(t, server, auction.ID, seller2.ID, 4.50)
				winningBid = placeBid(t, server, auction.ID, seller1.ID, 5.00)
			} else {
				placeBid(t, server, auction.ID, seller1.ID, 5.50)
				winningBid = placeBid(t, server, auction.ID, seller2.ID, 6.00)
			}
			
			// Complete auction and route
			completeAuction(t, server, auction.ID)
			routedCall := routeCall(t, server, call.ID)
			
			// Progress call
			updateCallStatus(t, server, routedCall.ID, call.StatusRinging)
			updateCallStatus(t, server, routedCall.ID, call.StatusInProgress)
			
			// Complete with varying durations
			duration := 60 + i*30 // 1-5.5 minutes
			completedCall := completeCall(t, server, routedCall.ID, duration)
			
			// Calculate cost
			callCost := float64(duration/60) * winningBid.Amount
			totalCost += callCost
			
			if i%2 == 0 {
				seller1Earnings += callCost
			} else {
				seller2Earnings += callCost
			}
			
			// Verify call record has correct cost
			assert.InDelta(t, callCost, *completedCall.Cost, 0.01)
		}
		
		// Wait for billing processing
		time.Sleep(100 * time.Millisecond)
		
		// Verify final balances
		finalBuyerBalance := getAccountBalance(t, server, buyer.ID)
		finalSeller1Balance := getAccountBalance(t, server, seller1.ID)
		finalSeller2Balance := getAccountBalance(t, server, seller2.ID)
		
		// Buyer should be charged
		expectedBuyerBalance := initialBuyerBalance - totalCost
		assert.InDelta(t, expectedBuyerBalance, finalBuyerBalance, 0.01)
		
		// Sellers should be credited
		expectedSeller1Balance := initialSeller1Balance + seller1Earnings
		expectedSeller2Balance := initialSeller2Balance + seller2Earnings
		
		assert.InDelta(t, expectedSeller1Balance, finalSeller1Balance, 0.01)
		assert.InDelta(t, expectedSeller2Balance, finalSeller2Balance, 0.01)
		
		// Verify transaction history
		buyerTransactions := getTransactionHistory(t, server, buyer.ID)
		assert.Equal(t, totalCalls, len(buyerTransactions))
		
		for _, tx := range buyerTransactions {
			assert.Equal(t, financial.TransactionTypeDebit, tx.Type)
			assert.Equal(t, financial.TransactionStatusCompleted, tx.Status)
		}
	})
	
	t.Run("Insufficient Funds", func(t *testing.T) {
		// Create buyer with limited balance
		poorBuyer := createTestAccountWithBalance(t, ctx, testDB, "poor-buyer", account.TypeBuyer, 10.00)
		seller := createTestAccountWithBalance(t, ctx, testDB, "funds-seller", account.TypeSeller, 0.00)
		
		// Create expensive call
		call := simulateIncomingCall(t, server, poorBuyer.ID, "+14155551234", "+19005551234")
		auction := startAuction(t, server, call.ID)
		
		// High bid that would exceed balance
		placeBid(t, server, auction.ID, seller.ID, 15.00) // $15/min
		completeAuction(t, server, auction.ID)
		routedCall := routeCall(t, server, call.ID)
		
		// Progress call
		updateCallStatus(t, server, routedCall.ID, call.StatusRinging)
		updateCallStatus(t, server, routedCall.ID, call.StatusInProgress)
		
		// Call should be terminated when balance runs out
		time.Sleep(1 * time.Second)
		
		// Check call status
		callStatus := getCallStatus(t, server, routedCall.ID)
		assert.Equal(t, call.StatusCompleted, callStatus.Status)
		
		// Verify buyer balance is depleted
		finalBalance := getAccountBalance(t, server, poorBuyer.ID)
		assert.Less(t, finalBalance, 1.00)
	})
}

// TestFinancial_PaymentMethods tests payment method management
func TestFinancial_PaymentMethods(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	// Create authenticated user
	user := createAuthenticatedUser(t, server, "payment-test@example.com", "buyer")
	
	t.Run("Add Credit Card", func(t *testing.T) {
		cardReq := map[string]interface{}{
			"card_number": "4242424242424242",
			"exp_month":   12,
			"exp_year":    2030,
			"cvc":         "123",
			"zip":         "94105",
		}
		
		body, _ := json.Marshal(cardReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/payment-methods", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var paymentMethod financial.PaymentMethod
		json.NewDecoder(resp.Body).Decode(&paymentMethod)
		assert.Equal(t, "****4242", paymentMethod.Last4)
		assert.Equal(t, financial.PaymentMethodTypeCard, paymentMethod.Type)
		assert.True(t, paymentMethod.IsDefault)
	})
	
	t.Run("Add Bank Account", func(t *testing.T) {
		bankReq := map[string]interface{}{
			"account_number": "123456789",
			"routing_number": "110000000",
			"account_type":   "checking",
		}
		
		body, _ := json.Marshal(bankReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/payment-methods/bank", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var paymentMethod financial.PaymentMethod
		json.NewDecoder(resp.Body).Decode(&paymentMethod)
		assert.Equal(t, "****6789", paymentMethod.Last4)
		assert.Equal(t, financial.PaymentMethodTypeBank, paymentMethod.Type)
	})
	
	t.Run("Set Default Payment Method", func(t *testing.T) {
		// Get all payment methods
		paymentMethods := getPaymentMethods(t, server, user.Token)
		assert.GreaterOrEqual(t, len(paymentMethods), 2)
		
		// Find non-default method
		var nonDefaultID uuid.UUID
		for _, pm := range paymentMethods {
			if !pm.IsDefault {
				nonDefaultID = pm.ID
				break
			}
		}
		
		// Set as default
		req, _ := http.NewRequest("PUT", server.URL+fmt.Sprintf("/api/v1/payment-methods/%s/default", nonDefaultID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Verify change
		updatedMethods := getPaymentMethods(t, server, user.Token)
		for _, pm := range updatedMethods {
			if pm.ID == nonDefaultID {
				assert.True(t, pm.IsDefault)
			} else {
				assert.False(t, pm.IsDefault)
			}
		}
	})
	
	t.Run("Remove Payment Method", func(t *testing.T) {
		paymentMethods := getPaymentMethods(t, server, user.Token)
		
		// Cannot remove default method
		var defaultMethod *financial.PaymentMethod
		var nonDefaultMethod *financial.PaymentMethod
		
		for _, pm := range paymentMethods {
			if pm.IsDefault {
				defaultMethod = &pm
			} else {
				nonDefaultMethod = &pm
			}
		}
		
		// Try to remove default (should fail)
		req, _ := http.NewRequest("DELETE", server.URL+fmt.Sprintf("/api/v1/payment-methods/%s", defaultMethod.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		
		// Remove non-default (should succeed)
		req, _ = http.NewRequest("DELETE", server.URL+fmt.Sprintf("/api/v1/payment-methods/%s", nonDefaultMethod.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

// TestFinancial_Invoicing tests invoice generation and management
func TestFinancial_Invoicing(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("Monthly Invoice Generation", func(t *testing.T) {
		// Create buyer and process calls
		buyer := createTestAccountWithBalance(t, ctx, testDB, "invoice-buyer", account.TypeBuyer, 5000.00)
		seller := createTestAccountWithBalance(t, ctx, testDB, "invoice-seller", account.TypeSeller, 0.00)
		
		// Process calls over multiple days
		callCount := 50
		var totalAmount float64
		
		for i := 0; i < callCount; i++ {
			call := simulateIncomingCall(t, server, buyer.ID, 
				fmt.Sprintf("+1415555%04d", i), "+18005551234")
			
			auction := startAuction(t, server, call.ID)
			bid := placeBid(t, server, auction.ID, seller.ID, 4.00+float64(i%5))
			completeAuction(t, server, auction.ID)
			
			routedCall := routeCall(t, server, call.ID)
			updateCallStatus(t, server, routedCall.ID, call.StatusRinging)
			updateCallStatus(t, server, routedCall.ID, call.StatusInProgress)
			
			duration := 60 + (i%10)*30
			completedCall := completeCall(t, server, routedCall.ID, duration)
			
			totalAmount += *completedCall.Cost
		}
		
		// Trigger invoice generation
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/invoices/generate", nil)
		req.Header.Set("Authorization", "Bearer "+getAdminToken(t, server))
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Get buyer's invoice
		buyerUser := createAuthenticatedUser(t, server, buyer.Email, "buyer")
		invoices := getInvoices(t, server, buyerUser.Token)
		assert.GreaterOrEqual(t, len(invoices), 1)
		
		latestInvoice := invoices[0]
		assert.Equal(t, financial.InvoiceStatusPending, latestInvoice.Status)
		assert.InDelta(t, totalAmount, latestInvoice.TotalAmount, 0.01)
		assert.Equal(t, callCount, len(latestInvoice.LineItems))
		
		// Verify line items
		var lineItemTotal float64
		for _, item := range latestInvoice.LineItems {
			assert.Equal(t, financial.LineItemTypeCall, item.Type)
			assert.NotEmpty(t, item.Description)
			lineItemTotal += item.Amount
		}
		assert.InDelta(t, totalAmount, lineItemTotal, 0.01)
	})
	
	t.Run("Invoice Payment", func(t *testing.T) {
		// Create buyer with invoice
		buyer := createTestAccountWithBalance(t, ctx, testDB, "pay-buyer", account.TypeBuyer, 1000.00)
		buyerUser := createAuthenticatedUser(t, server, buyer.Email, "buyer")
		
		// Create manual invoice
		invoice := createManualInvoice(t, server, buyer.ID, 250.00)
		
		// Pay invoice
		paymentReq := map[string]interface{}{
			"payment_method_id": getDefaultPaymentMethod(t, server, buyerUser.Token),
		}
		
		body, _ := json.Marshal(paymentReq)
		req, _ := http.NewRequest("POST", server.URL+fmt.Sprintf("/api/v1/invoices/%s/pay", invoice.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+buyerUser.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var payment financial.Payment
		json.NewDecoder(resp.Body).Decode(&payment)
		assert.Equal(t, financial.PaymentStatusCompleted, payment.Status)
		assert.Equal(t, invoice.TotalAmount, payment.Amount)
		
		// Verify invoice is marked as paid
		updatedInvoice := getInvoice(t, server, buyerUser.Token, invoice.ID)
		assert.Equal(t, financial.InvoiceStatusPaid, updatedInvoice.Status)
		assert.NotNil(t, updatedInvoice.PaidAt)
		
		// Verify balance updated
		finalBalance := getAccountBalance(t, server, buyer.ID)
		assert.InDelta(t, 750.00, finalBalance, 0.01)
	})
}

// TestFinancial_Reconciliation tests financial reconciliation
func TestFinancial_Reconciliation(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("Daily Reconciliation", func(t *testing.T) {
		// Create multiple buyers and sellers
		buyers := make([]*account.Account, 5)
		sellers := make([]*account.Account, 3)
		
		for i := 0; i < 5; i++ {
			buyers[i] = createTestAccountWithBalance(t, ctx, testDB, 
				fmt.Sprintf("recon-buyer%d", i), account.TypeBuyer, 1000.00)
		}
		
		for i := 0; i < 3; i++ {
			sellers[i] = createTestAccountWithBalance(t, ctx, testDB, 
				fmt.Sprintf("recon-seller%d", i), account.TypeSeller, 0.00)
		}
		
		// Process many calls concurrently
		var wg sync.WaitGroup
		callCount := 100
		
		for i := 0; i < callCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				buyer := buyers[idx%len(buyers)]
				seller := sellers[idx%len(sellers)]
				
				call := simulateIncomingCall(t, server, buyer.ID, 
					fmt.Sprintf("+1415555%04d", idx), "+18005551234")
				
				auction := startAuction(t, server, call.ID)
				placeBid(t, server, auction.ID, seller.ID, 3.00+float64(idx%7))
				completeAuction(t, server, auction.ID)
				
				routedCall := routeCall(t, server, call.ID)
				updateCallStatus(t, server, routedCall.ID, call.StatusRinging)
				updateCallStatus(t, server, routedCall.ID, call.StatusInProgress)
				
				duration := 60 + (idx%15)*20
				completeCall(t, server, routedCall.ID, duration)
			}(i)
		}
		
		wg.Wait()
		
		// Run reconciliation
		adminToken := getAdminToken(t, server)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/financial/reconcile", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var reconciliation financial.ReconciliationReport
		json.NewDecoder(resp.Body).Decode(&reconciliation)
		
		// Verify reconciliation results
		assert.Equal(t, callCount, reconciliation.TotalTransactions)
		assert.Greater(t, reconciliation.TotalDebits, 0.0)
		assert.Greater(t, reconciliation.TotalCredits, 0.0)
		assert.InDelta(t, 0.0, reconciliation.TotalDebits-reconciliation.TotalCredits, 0.01)
		assert.Empty(t, reconciliation.Discrepancies)
		
		// Verify all accounts balance
		for _, buyer := range buyers {
			balance := getAccountBalance(t, server, buyer.ID)
			assert.Less(t, balance, 1000.00)
			assert.GreaterOrEqual(t, balance, 0.0)
		}
		
		for _, seller := range sellers {
			balance := getAccountBalance(t, server, seller.ID)
			assert.Greater(t, balance, 0.0)
		}
	})
	
	t.Run("Discrepancy Detection", func(t *testing.T) {
		// Create scenario with discrepancy
		buyer := createTestAccountWithBalance(t, ctx, testDB, "disc-buyer", account.TypeBuyer, 500.00)
		
		// Manually create transaction without corresponding call
		tx := &financial.Transaction{
			ID:        uuid.New(),
			AccountID: buyer.ID,
			Type:      financial.TransactionTypeDebit,
			Amount:    50.00,
			Status:    financial.TransactionStatusCompleted,
			CreatedAt: time.Now(),
		}
		
		// Insert directly to database (simulating discrepancy)
		err := testDB.DB().QueryRow(
			`INSERT INTO transactions (id, account_id, type, amount, status, created_at) 
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			tx.ID, tx.AccountID, tx.Type, tx.Amount, tx.Status, tx.CreatedAt,
		).Scan(&tx.ID)
		require.NoError(t, err)
		
		// Run reconciliation
		adminToken := getAdminToken(t, server)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/financial/reconcile", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var reconciliation financial.ReconciliationReport
		json.NewDecoder(resp.Body).Decode(&reconciliation)
		
		// Should detect discrepancy
		assert.NotEmpty(t, reconciliation.Discrepancies)
		assert.Equal(t, 1, len(reconciliation.Discrepancies))
		
		discrepancy := reconciliation.Discrepancies[0]
		assert.Equal(t, tx.ID, discrepancy.TransactionID)
		assert.Equal(t, "orphaned_transaction", discrepancy.Type)
		assert.Equal(t, 50.00, discrepancy.Amount)
	})
}

// Helper functions for financial testing

func createTestAccountWithBalance(t *testing.T, ctx context.Context, db *testutil.TestDB, name string, accType account.Type, balance float64) *account.Account {
	acc := &account.Account{
		ID:        uuid.New(),
		Name:      name,
		Email:     name + "@example.com",
		Type:      accType,
		Status:    account.StatusActive,
		Balance:   balance,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Insert account
	err := db.DB().QueryRow(
		`INSERT INTO accounts (id, name, email, type, status, balance, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		acc.ID, acc.Name, acc.Email, acc.Type, acc.Status, acc.Balance, acc.CreatedAt, acc.UpdatedAt,
	).Scan(&acc.ID)
	require.NoError(t, err)
	
	return acc
}

func getTransactionHistory(t *testing.T, server *httptest.Server, accountID uuid.UUID) []financial.Transaction {
	user := createAuthenticatedUser(t, server, fmt.Sprintf("tx-%s@example.com", accountID), "buyer")
	
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+user.Token)
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var transactions []financial.Transaction
	json.NewDecoder(resp.Body).Decode(&transactions)
	return transactions
}

func getCallStatus(t *testing.T, server *httptest.Server, callID uuid.UUID) *call.Call {
	resp, err := http.Get(server.URL + fmt.Sprintf("/api/v1/calls/%s", callID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result call.Call
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func getPaymentMethods(t *testing.T, server *httptest.Server, token string) []financial.PaymentMethod {
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/payment-methods", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var methods []financial.PaymentMethod
	json.NewDecoder(resp.Body).Decode(&methods)
	return methods
}

func getInvoices(t *testing.T, server *httptest.Server, token string) []financial.Invoice {
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var invoices []financial.Invoice
	json.NewDecoder(resp.Body).Decode(&invoices)
	return invoices
}

func getInvoice(t *testing.T, server *httptest.Server, token string, invoiceID uuid.UUID) *financial.Invoice {
	req, _ := http.NewRequest("GET", server.URL+fmt.Sprintf("/api/v1/invoices/%s", invoiceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var invoice financial.Invoice
	json.NewDecoder(resp.Body).Decode(&invoice)
	return &invoice
}

func createManualInvoice(t *testing.T, server *httptest.Server, accountID uuid.UUID, amount float64) *financial.Invoice {
	adminToken := getAdminToken(t, server)
	
	invoiceReq := map[string]interface{}{
		"account_id": accountID,
		"amount":     amount,
		"due_date":   time.Now().Add(30 * 24 * time.Hour),
		"line_items": []map[string]interface{}{
			{
				"description": "Manual charge",
				"amount":      amount,
				"type":        "manual",
			},
		},
	}
	
	body, _ := json.Marshal(invoiceReq)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/invoices", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var invoice financial.Invoice
	json.NewDecoder(resp.Body).Decode(&invoice)
	return &invoice
}

func getDefaultPaymentMethod(t *testing.T, server *httptest.Server, token string) uuid.UUID {
	methods := getPaymentMethods(t, server, token)
	
	// If no methods, create one
	if len(methods) == 0 {
		cardReq := map[string]interface{}{
			"card_number": "4242424242424242",
			"exp_month":   12,
			"exp_year":    2030,
			"cvc":         "123",
			"zip":         "94105",
		}
		
		body, _ := json.Marshal(cardReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/payment-methods", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var method financial.PaymentMethod
		json.NewDecoder(resp.Body).Decode(&method)
		return method.ID
	}
	
	// Find default
	for _, method := range methods {
		if method.IsDefault {
			return method.ID
		}
	}
	
	// Return first if no default
	return methods[0].ID
}

func getAdminToken(t *testing.T, server *httptest.Server) string {
	admin := createAuthenticatedUser(t, server, "admin@dependablecallexchange.com", "admin")
	return admin.Token
}