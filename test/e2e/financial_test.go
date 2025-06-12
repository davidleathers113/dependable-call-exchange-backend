//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/financial"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancial_BillingCycle(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("End-to-End Billing Flow", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create accounts with initial balance
		buyer := createAccountInDB(t, env, "billing-buyer", account.TypeBuyer, 1000.00)
		seller1 := createAccountInDB(t, env, "billing-seller1", account.TypeSeller, 0.00)
		seller2 := createAccountInDB(t, env, "billing-seller2", account.TypeSeller, 0.00)
		
		// Authenticate accounts
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		seller1Auth := authenticateAccount(t, client, seller1.Email.String())
		seller2Auth := authenticateAccount(t, client, seller2.Email.String())
		
		// Track initial balances
		initialBuyerBalance := getAccountBalance(t, client, buyerAuth.Token)
		initialSeller1Balance := getAccountBalance(t, client, seller1Auth.Token)
		initialSeller2Balance := getAccountBalance(t, client, seller2Auth.Token)
		
		// Process multiple calls
		totalCalls := 10
		var totalCost float64
		var seller1Earnings float64
		var seller2Earnings float64
		
		for i := 0; i < totalCalls; i++ {
			// Create call
			client.SetToken(buyerAuth.Token)
			incomingCall := simulateIncomingCall(t, client, 
				fmt.Sprintf("+1415555%04d", i), "+18005551234")
			
			// Start auction
			auction := startAuction(t, client, incomingCall.ID)
			
			// Sellers bid (alternating winner)
			var winningBid *bid.Bid
			if i%2 == 0 {
				client.SetToken(seller2Auth.Token)
				placeBid(t, client, auction.ID, 4.50)
				
				client.SetToken(seller1Auth.Token)
				winningBid = placeBid(t, client, auction.ID, 5.00)
			} else {
				client.SetToken(seller1Auth.Token)
				placeBid(t, client, auction.ID, 5.50)
				
				client.SetToken(seller2Auth.Token)
				winningBid = placeBid(t, client, auction.ID, 6.00)
			}
			
			// Complete auction and route
			client.SetToken(buyerAuth.Token)
			completeAuction(t, client, auction.ID)
			routedCall := routeCall(t, client, incomingCall.ID)
			
			// Progress call
			updateCallStatus(t, client, routedCall.ID, call.StatusRinging)
			updateCallStatus(t, client, routedCall.ID, call.StatusInProgress)
			
			// Complete with varying durations
			duration := 60 + i*30 // 1-5.5 minutes
			completedCall := completeCall(t, client, routedCall.ID, duration)
			
			// Calculate cost
			callCost := float64(duration/60) * winningBid.Amount.ToFloat64()
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
		finalBuyerBalance := getAccountBalance(t, client, buyerAuth.Token)
		finalSeller1Balance := getAccountBalance(t, client, seller1Auth.Token)
		finalSeller2Balance := getAccountBalance(t, client, seller2Auth.Token)
		
		// Buyer should be charged
		expectedBuyerBalance := initialBuyerBalance - totalCost
		assert.InDelta(t, expectedBuyerBalance, finalBuyerBalance, 0.01)
		
		// Sellers should be credited
		expectedSeller1Balance := initialSeller1Balance + seller1Earnings
		expectedSeller2Balance := initialSeller2Balance + seller2Earnings
		
		assert.InDelta(t, expectedSeller1Balance, finalSeller1Balance, 0.01)
		assert.InDelta(t, expectedSeller2Balance, finalSeller2Balance, 0.01)
		
		// Verify transaction history
		buyerTransactions := getTransactionHistory(t, client, buyerAuth.Token)
		assert.Equal(t, totalCalls, len(buyerTransactions))
		
		for _, tx := range buyerTransactions {
			assert.Equal(t, financial.TransactionTypeCharge, tx.Type)
			assert.Equal(t, financial.TransactionStatusCompleted, tx.Status)
		}
	})
	
	t.Run("Insufficient Funds", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create buyer with limited balance
		poorBuyer := createAccountInDB(t, env, "poor-buyer", account.TypeBuyer, 10.00)
		seller := createAccountInDB(t, env, "funds-seller", account.TypeSeller, 0.00)
		
		poorBuyerAuth := authenticateAccount(t, client, poorBuyer.Email.String())
		sellerAuth := authenticateAccount(t, client, seller.Email.String())
		
		// Create expensive call
		client.SetToken(poorBuyerAuth.Token)
		incomingCall := simulateIncomingCall(t, client, "+14155551234", "+19005551234")
		auction := startAuction(t, client, incomingCall.ID)
		
		// High bid that would exceed balance
		client.SetToken(sellerAuth.Token)
		placeBid(t, client, auction.ID, 15.00) // $15/min
		
		client.SetToken(poorBuyerAuth.Token)
		completeAuction(t, client, auction.ID)
		routedCall := routeCall(t, client, incomingCall.ID)
		
		// Progress call
		updateCallStatus(t, client, routedCall.ID, call.StatusRinging)
		updateCallStatus(t, client, routedCall.ID, call.StatusInProgress)
		
		// Call should be terminated when balance runs out
		time.Sleep(1 * time.Second)
		
		// Check call status
		callStatus := getCallStatus(t, client, routedCall.ID)
		assert.Equal(t, call.StatusCompleted, callStatus.Status)
		
		// Verify buyer balance is depleted
		finalBalance := getAccountBalance(t, client, poorBuyerAuth.Token)
		assert.Less(t, finalBalance, 1.00)
	})
}

func TestFinancial_PaymentMethods(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Add Credit Card", func(t *testing.T) {
		env.ResetDatabase()
		user := createAuthenticatedUser(t, client, "payment-test@example.com", "buyer")
		client.SetToken(user.Token)
		
		cardReq := map[string]interface{}{
			"card_number": "4242424242424242",
			"exp_month":   12,
			"exp_year":    2030,
			"cvc":         "123",
			"zip":         "94105",
		}
		
		resp := client.Post("/api/v1/payment-methods", cardReq)
		assert.Equal(t, 201, resp.StatusCode)
		
		var paymentMethod financial.PaymentMethod
		client.DecodeResponse(resp, &paymentMethod)
		assert.Equal(t, "****4242", paymentMethod.Last4)
		assert.Equal(t, financial.PaymentMethodTypeCard, paymentMethod.Type)
		assert.True(t, paymentMethod.IsDefault)
	})
	
	t.Run("Add Bank Account", func(t *testing.T) {
		env.ResetDatabase()
		user := createAuthenticatedUser(t, client, "bank-test@example.com", "buyer")
		client.SetToken(user.Token)
		
		bankReq := map[string]interface{}{
			"account_number": "123456789",
			"routing_number": "110000000",
			"account_type":   "checking",
		}
		
		resp := client.Post("/api/v1/payment-methods/bank", bankReq)
		assert.Equal(t, 201, resp.StatusCode)
		
		var paymentMethod financial.PaymentMethod
		client.DecodeResponse(resp, &paymentMethod)
		assert.Equal(t, "****6789", paymentMethod.Last4)
		assert.Equal(t, financial.PaymentMethodTypeBank, paymentMethod.Type)
	})
}

func TestFinancial_Invoicing(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Monthly Invoice Generation", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create buyer and process calls
		buyer := createAccountInDB(t, env, "invoice-buyer", account.TypeBuyer, 5000.00)
		seller := createAccountInDB(t, env, "invoice-seller", account.TypeSeller, 0.00)
		
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		sellerAuth := authenticateAccount(t, client, seller.Email.String())
		
		// Process calls over multiple days
		callCount := 50
		var totalAmount float64
		
		for i := 0; i < callCount; i++ {
			client.SetToken(buyerAuth.Token)
			incomingCall := simulateIncomingCall(t, client, 
				fmt.Sprintf("+1415555%04d", i), "+18005551234")
			
			auction := startAuction(t, client, incomingCall.ID)
			
			client.SetToken(sellerAuth.Token)
			_ = placeBid(t, client, auction.ID, 4.00+float64(i%5))
			
			client.SetToken(buyerAuth.Token)
			completeAuction(t, client, auction.ID)
			
			routedCall := routeCall(t, client, incomingCall.ID)
			updateCallStatus(t, client, routedCall.ID, call.StatusRinging)
			updateCallStatus(t, client, routedCall.ID, call.StatusInProgress)
			
			duration := 60 + (i%10)*30
			completedCall := completeCall(t, client, routedCall.ID, duration)
			
			totalAmount += completedCall.Cost.ToFloat64()
		}
		
		// Trigger invoice generation (admin endpoint)
		adminAuth := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(adminAuth.Token)
		
		resp := client.Post("/api/v1/invoices/generate", nil)
		assert.Equal(t, 200, resp.StatusCode)
		
		// Get buyer's invoice
		client.SetToken(buyerAuth.Token)
		invoices := getInvoices(t, client)
		assert.GreaterOrEqual(t, len(invoices), 1)
		
		latestInvoice := invoices[0]
		assert.Equal(t, financial.InvoiceStatusPending, latestInvoice.Status)
		assert.InDelta(t, totalAmount, latestInvoice.TotalAmount, 0.01)
		
		// Verify line items
		var lineItemTotal float64
		for _, item := range latestInvoice.LineItems {
			assert.Equal(t, financial.LineItemTypeCall, item.Type)
			assert.NotEmpty(t, item.Description)
			lineItemTotal += item.Amount
		}
		assert.InDelta(t, totalAmount, lineItemTotal, 0.01)
	})
}

func TestFinancial_Reconciliation(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Daily Reconciliation", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create multiple buyers and sellers
		buyers := make([]*account.Account, 5)
		buyerAuths := make([]*AuthenticatedUser, 5)
		sellers := make([]*account.Account, 3)
		sellerAuths := make([]*AuthenticatedUser, 3)
		
		for i := 0; i < 5; i++ {
			buyers[i] = createAccountInDB(t, env, 
				fmt.Sprintf("recon-buyer%d", i), account.TypeBuyer, 1000.00)
			buyerAuths[i] = authenticateAccount(t, client, buyers[i].Email.String())
		}
		
		for i := 0; i < 3; i++ {
			sellers[i] = createAccountInDB(t, env, 
				fmt.Sprintf("recon-seller%d", i), account.TypeSeller, 0.00)
			sellerAuths[i] = authenticateAccount(t, client, sellers[i].Email.String())
		}
		
		// Process many calls concurrently
		var wg sync.WaitGroup
		callCount := 100
		
		for i := 0; i < callCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				buyerAuth := buyerAuths[idx%len(buyerAuths)]
				sellerAuth := sellerAuths[idx%len(sellerAuths)]
				
				localClient := infrastructure.NewAPIClient(t, env.APIURL)
				
				// Create call as buyer
				localClient.SetToken(buyerAuth.Token)
				incomingCall := simulateIncomingCall(t, localClient, 
					fmt.Sprintf("+1415555%04d", idx), "+18005551234")
				
				auction := startAuction(t, localClient, incomingCall.ID)
				
				// Place bid as seller
				localClient.SetToken(sellerAuth.Token)
				placeBid(t, localClient, auction.ID, 3.00+float64(idx%7))
				
				// Complete as buyer
				localClient.SetToken(buyerAuth.Token)
				completeAuction(t, localClient, auction.ID)
				
				routedCall := routeCall(t, localClient, incomingCall.ID)
				updateCallStatus(t, localClient, routedCall.ID, call.StatusRinging)
				updateCallStatus(t, localClient, routedCall.ID, call.StatusInProgress)
				
				duration := 60 + (idx%15)*20
				completeCall(t, localClient, routedCall.ID, duration)
			}(i)
		}
		
		wg.Wait()
		
		// Run reconciliation (admin endpoint)
		adminAuth := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(adminAuth.Token)
		
		resp := client.Post("/api/v1/financial/reconcile", nil)
		assert.Equal(t, 200, resp.StatusCode)
		
		var reconciliation financial.ReconciliationReport
		client.DecodeResponse(resp, &reconciliation)
		
		// Verify reconciliation results
		assert.Equal(t, callCount, reconciliation.TotalTransactions)
		assert.Greater(t, reconciliation.TotalDebits, 0.0)
		assert.Greater(t, reconciliation.TotalCredits, 0.0)
		assert.InDelta(t, 0.0, reconciliation.TotalDebits-reconciliation.TotalCredits, 0.01)
		assert.Empty(t, reconciliation.Discrepancies)
		
		// Verify all accounts balance
		for i, buyerAuth := range buyerAuths {
			balance := getAccountBalance(t, client, buyerAuth.Token)
			assert.Less(t, balance, 1000.00)
			assert.GreaterOrEqual(t, balance, 0.0)
			t.Logf("Buyer %d balance: %.2f", i, balance)
		}
		
		for i, sellerAuth := range sellerAuths {
			balance := getAccountBalance(t, client, sellerAuth.Token)
			assert.Greater(t, balance, 0.0)
			t.Logf("Seller %d balance: %.2f", i, balance)
		}
	})
}

// Helper functions
func getTransactionHistory(t *testing.T, client *infrastructure.APIClient, token string) []financial.Transaction {
	client.SetToken(token)
	resp := client.Get("/api/v1/transactions")
	require.Equal(t, 200, resp.StatusCode)
	
	var transactions []financial.Transaction
	client.DecodeResponse(resp, &transactions)
	return transactions
}

func getCallStatus(t *testing.T, client *infrastructure.APIClient, callID uuid.UUID) *call.Call {
	resp := client.Get(fmt.Sprintf("/api/v1/calls/%s", callID))
	require.Equal(t, 200, resp.StatusCode)
	
	var result call.Call
	client.DecodeResponse(resp, &result)
	return &result
}

func getInvoices(t *testing.T, client *infrastructure.APIClient) []financial.Invoice {
	resp := client.Get("/api/v1/invoices")
	require.Equal(t, 200, resp.StatusCode)
	
	var invoices []financial.Invoice
	client.DecodeResponse(resp, &invoices)
	return invoices
}
