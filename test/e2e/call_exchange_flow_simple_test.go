//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallExchangeFlow_Simplified shows the cleaner approach
func TestCallExchangeFlow_Simplified(t *testing.T) {
	// ONE LINE to set up entire test environment!
	env := infrastructure.NewSimpleTestEnvironment(t)
	
	// Create test data using helper functions
	buyer := createTestAccount(t, env, "buyer", "buyer@example.com")
	_ = createTestAccount(t, env, "seller", "seller@example.com") // seller will be used in future tests
	
	// Test the actual flow
	t.Run("incoming call creates auction", func(t *testing.T) {
		// Create incoming call
		call := IncomingCall{
			FromNumber: "+1234567890",
			ToNumber:   "+0987654321",
			BuyerID:    buyer.ID,
		}
		
		resp, err := postJSON(env.APIURL+"/v1/calls/incoming", call)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var created CallResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
		assert.NotEmpty(t, created.ID)
	})
	
	t.Run("bid submission", func(t *testing.T) {
		// Simplified bid submission test
		bid := BidRequest{
			CallID:   "test-call-id",
			BuyerID:  buyer.ID,
			Amount:   10.50,
		}
		
		resp, err := postJSON(env.APIURL+"/v1/bids", bid)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

// Helper functions make tests cleaner
func createTestAccount(t *testing.T, env *infrastructure.SimpleTestEnvironment, accountType, email string) Account {
	account := Account{
		Type:        accountType,
		Email:       email,
		CompanyName: fmt.Sprintf("Test %s Corp", accountType),
	}
	
	resp, err := postJSON(env.APIURL+"/v1/accounts", account)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var created Account
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	return created
}

func postJSON(url string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	return http.Post(url, "application/json", bytes.NewReader(jsonBody))
}

// Data structures
type Account struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Email       string `json:"email"`
	CompanyName string `json:"company_name"`
}

type IncomingCall struct {
	FromNumber string `json:"from_number"`
	ToNumber   string `json:"to_number"`
	BuyerID    string `json:"buyer_id"`
}

type CallResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type BidRequest struct {
	CallID  string  `json:"call_id"`
	BuyerID string  `json:"buyer_id"`
	Amount  float64 `json:"amount"`
}