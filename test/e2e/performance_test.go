//go:build e2e

package e2e

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
)

func TestPerformance_HighVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("10K Calls Per Second", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create test accounts
		numBuyers := 100
		buyers := make([]*AuthenticatedUser, numBuyers)
		for i := 0; i < numBuyers; i++ {
			acc := createAccountInDB(t, env, fmt.Sprintf("buyer%d", i), account.TypeBuyer, 10000.00)
			buyers[i] = authenticateAccount(t, client, acc.Email.String())
		}
		
		numSellers := 50
		sellers := make([]*AuthenticatedUser, numSellers)
		for i := 0; i < numSellers; i++ {
			acc := createAccountInDB(t, env, fmt.Sprintf("seller%d", i), account.TypeSeller, 0.00)
			sellers[i] = authenticateAccount(t, client, acc.Email.String())
			
			// Create bid profiles
			localClient := infrastructure.NewAPIClient(t, env.APIURL)
			localClient.SetToken(sellers[i].Token)
			createBidProfile(t, localClient, bid.BidCriteria{
				MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
				CallType:  []string{"sales"},
			})
		}
		
		// Performance metrics
		var totalCalls int64
		var successfulCalls int64
		var failedCalls int64
		var totalLatency int64
		
		// Run load test for 10 seconds
		duration := 10 * time.Second
		callsPerSecond := 10000
		concurrency := 100
		
		start := time.Now()
		deadline := start.Add(duration)
		
		var wg sync.WaitGroup
		callChan := make(chan bool, callsPerSecond)
		
		// Start workers
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				localClient := infrastructure.NewAPIClient(t, env.APIURL)
				buyer := buyers[workerID%len(buyers)]
				localClient.SetToken(buyer.Token)
				
				for time.Now().Before(deadline) {
					select {
					case <-callChan:
						// Make call
						callStart := time.Now()
						
						resp := localClient.Post("/api/v1/calls", map[string]interface{}{
							"from_number": fmt.Sprintf("+1415555%04d", atomic.AddInt64(&totalCalls, 1)%10000),
							"to_number":   "+18005551234",
						})
						
						latency := time.Since(callStart).Milliseconds()
						atomic.AddInt64(&totalLatency, latency)
						
						if resp.StatusCode == 201 {
							atomic.AddInt64(&successfulCalls, 1)
						} else {
							atomic.AddInt64(&failedCalls, 1)
						}
						
						resp.Body.Close()
						
					default:
						// No call to make, brief pause
						time.Sleep(time.Microsecond)
					}
				}
			}(i)
		}
		
		// Feed calls at target rate
		go func() {
			ticker := time.NewTicker(time.Second / time.Duration(callsPerSecond))
			defer ticker.Stop()
			
			for {
				select {
				case <-ticker.C:
					select {
					case callChan <- true:
					default:
						// Channel full, skip
					}
				case <-time.After(duration):
					return
				}
			}
		}()
		
		// Wait for completion
		wg.Wait()
		elapsed := time.Since(start)
		
		// Calculate metrics
		total := atomic.LoadInt64(&totalCalls)
		successful := atomic.LoadInt64(&successfulCalls)
		failed := atomic.LoadInt64(&failedCalls)
		avgLatency := float64(atomic.LoadInt64(&totalLatency)) / float64(total)
		callsPerSec := float64(total) / elapsed.Seconds()
		
		// Log results
		t.Logf("Performance Test Results:")
		t.Logf("  Duration: %v", elapsed)
		t.Logf("  Total Calls: %d", total)
		t.Logf("  Successful: %d (%.2f%%)", successful, float64(successful)/float64(total)*100)
		t.Logf("  Failed: %d (%.2f%%)", failed, float64(failed)/float64(total)*100)
		t.Logf("  Calls/sec: %.2f", callsPerSec)
		t.Logf("  Avg Latency: %.2f ms", avgLatency)
		
		// Assertions
		assert.Greater(t, callsPerSec, float64(8000), "Should handle at least 8000 calls/sec")
		assert.Less(t, avgLatency, float64(50), "Average latency should be under 50ms")
		assert.Greater(t, float64(successful)/float64(total), 0.95, "Success rate should be >95%")
	})
	
	t.Run("Concurrent Auction Performance", func(t *testing.T) {
		env.ResetDatabase()
		
		// Test auction performance with many concurrent bidders
		buyer := createAccountInDB(t, env, "auction-buyer", account.TypeBuyer, 10000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		
		// Create many sellers
		numSellers := 1000
		sellers := make([]*AuthenticatedUser, numSellers)
		for i := 0; i < numSellers; i++ {
			acc := createAccountInDB(t, env, fmt.Sprintf("auction-seller%d", i), account.TypeSeller, 0.00)
			sellers[i] = authenticateAccount(t, client, acc.Email.String())
		}
		
		// Create call and start auction
		client.SetToken(buyerAuth.Token)
		incomingCall := simulateIncomingCall(t, client, "+14155551234", "+18005551234")
		auction := startAuction(t, client, incomingCall.ID)
		
		// Measure bidding performance
		var totalBids int64
		var successfulBids int64
		var bidLatency int64
		
		start := time.Now()
		var wg sync.WaitGroup
		
		// All sellers bid concurrently
		for i, seller := range sellers {
			wg.Add(1)
			go func(idx int, s *AuthenticatedUser) {
				defer wg.Done()
				
				localClient := infrastructure.NewAPIClient(t, env.APIURL)
				localClient.SetToken(s.Token)
				
				bidStart := time.Now()
				amount := 3.00 + float64(idx%100)*0.10 // Varying amounts
				
				_, err := placeBidConcurrent(localClient, auction.ID, amount)
				
				latency := time.Since(bidStart).Milliseconds()
				atomic.AddInt64(&bidLatency, latency)
				atomic.AddInt64(&totalBids, 1)
				
				if err == nil {
					atomic.AddInt64(&successfulBids, 1)
				}
			}(i, seller)
		}
		
		wg.Wait()
		elapsed := time.Since(start)
		
		// Complete auction
		client.SetToken(buyerAuth.Token)
		completeAuction(t, client, auction.ID)
		
		// Calculate metrics
		total := atomic.LoadInt64(&totalBids)
		successful := atomic.LoadInt64(&successfulBids)
		avgBidLatency := float64(atomic.LoadInt64(&bidLatency)) / float64(total)
		bidsPerSec := float64(total) / elapsed.Seconds()
		
		t.Logf("Auction Performance Results:")
		t.Logf("  Total Bidders: %d", numSellers)
		t.Logf("  Successful Bids: %d", successful)
		t.Logf("  Auction Duration: %v", elapsed)
		t.Logf("  Bids/sec: %.2f", bidsPerSec)
		t.Logf("  Avg Bid Latency: %.2f ms", avgBidLatency)
		
		// Assertions
		assert.Greater(t, bidsPerSec, float64(5000), "Should process >5000 bids/sec")
		assert.Less(t, avgBidLatency, float64(100), "Bid latency should be <100ms")
		assert.Equal(t, int64(numSellers), successful, "All bids should be accepted")
	})
}

func TestPerformance_LatencyPercentiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	// Create test data
	env.ResetDatabase()
	buyer := createAccountInDB(t, env, "perf-buyer", account.TypeBuyer, 10000.00)
	buyerAuth := authenticateAccount(t, client, buyer.Email.String())
	
	// Collect latency samples
	numRequests := 10000
	latencies := make([]int64, 0, numRequests)
	var mu sync.Mutex
	
	var wg sync.WaitGroup
	concurrency := 50
	requestsPerWorker := numRequests / concurrency
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			localClient := infrastructure.NewAPIClient(t, env.APIURL)
			localClient.SetToken(buyerAuth.Token)
			
			for j := 0; j < requestsPerWorker; j++ {
				start := time.Now()
				
				resp := localClient.Post("/api/v1/calls", map[string]interface{}{
					"from_number": fmt.Sprintf("+1415555%04d", workerID*1000+j),
					"to_number":   "+18005551234",
				})
				
				latency := time.Since(start).Milliseconds()
				
				if resp.StatusCode == 201 {
					mu.Lock()
					latencies = append(latencies, latency)
					mu.Unlock()
				}
				
				resp.Body.Close()
				
				// Small delay between requests
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Calculate percentiles
	percentiles := calculatePercentiles(latencies)
	
	t.Logf("Latency Percentiles (ms):")
	t.Logf("  P50: %d", percentiles[50])
	t.Logf("  P90: %d", percentiles[90])
	t.Logf("  P95: %d", percentiles[95])
	t.Logf("  P99: %d", percentiles[99])
	t.Logf("  P99.9: %d", percentiles[999])
	
	// Assertions based on SLA requirements
	assert.Less(t, percentiles[50], int64(10), "P50 should be <10ms")
	assert.Less(t, percentiles[90], int64(25), "P90 should be <25ms")
	assert.Less(t, percentiles[95], int64(50), "P95 should be <50ms")
	assert.Less(t, percentiles[99], int64(100), "P99 should be <100ms")
}

func TestPerformance_ResourceUtilization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	// Monitor database connections
	initialStats := env.DB.Stats()
	initialConnections := initialStats.OpenConnections
	
	// Create load
	var wg sync.WaitGroup
	numGoroutines := 1000
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			buyer := createAccountInDB(t, env, 
				fmt.Sprintf("resource-buyer%d", id), account.TypeBuyer, 1000.00)
			buyerAuth := authenticateAccount(t, client, buyer.Email.String())
			
			localClient := infrastructure.NewAPIClient(t, env.APIURL)
			localClient.SetToken(buyerAuth.Token)
			
			// Make several calls
			for j := 0; j < 10; j++ {
				resp := localClient.Post("/api/v1/calls", map[string]interface{}{
					"from_number": fmt.Sprintf("+1415555%04d", id*100+j),
					"to_number":   "+18005551234",
				})
				resp.Body.Close()
			}
		}(i)
	}
	
	// Monitor while load is running
	maxConnections := initialConnections
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				stats := env.DB.Stats()
				if stats.OpenConnections > maxConnections {
					maxConnections = stats.OpenConnections
				}
			case <-done:
				return
			}
		}
	}()
	
	wg.Wait()
	ticker.Stop()
	done <- true
	
	// Final stats
	finalStats := env.DB.Stats()
	
	t.Logf("Resource Utilization:")
	t.Logf("  Initial Connections: %d", initialConnections)
	t.Logf("  Max Connections: %d", maxConnections)
	t.Logf("  Final Connections: %d", finalStats.OpenConnections)
	t.Logf("  In Use: %d", finalStats.InUse)
	t.Logf("  Idle: %d", finalStats.Idle)
	
	// Assertions
	assert.Less(t, maxConnections, 50, "Should not exceed connection pool limit")
	assert.Equal(t, initialConnections, finalStats.OpenConnections, "Should release connections")
}

// Helper function to calculate percentiles
func calculatePercentiles(latencies []int64) map[int]int64 {
	if len(latencies) == 0 {
		return nil
	}
	
	// Sort latencies
	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)
	
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	percentiles := make(map[int]int64)
	percentileValues := []int{50, 90, 95, 99, 999} // 999 = 99.9
	
	for _, p := range percentileValues {
		index := int(math.Ceil(float64(len(sorted)) * float64(p) / 1000.0)) - 1
		if index >= len(sorted) {
			index = len(sorted) - 1
		}
		percentiles[p] = sorted[index]
	}
	
	return percentiles
}
