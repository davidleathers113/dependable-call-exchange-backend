package marketplace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	callpkg "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// orchestrator implements the MarketplaceOrchestrator interface
type orchestrator struct {
	// Repository dependencies
	callRepo    CallRepository
	bidRepo     BidRepository
	accountRepo AccountRepository

	// Service dependencies
	buyerRouting BuyerRoutingService
	sellerDist   SellerDistributionService
	bidding      BiddingService
	fraud        FraudService
	telephony    TelephonyService

	// Metrics and monitoring
	metrics MarketplaceMetricsCollector

	// Configuration
	config *OrchestratorConfig

	// Internal state management
	mu              sync.RWMutex
	activeAuctions  map[uuid.UUID]*AuctionState
	processingCalls map[uuid.UUID]*CallState
}

// OrchestratorConfig holds configuration for the marketplace orchestrator
type OrchestratorConfig struct {
	DefaultAuctionDuration time.Duration `json:"default_auction_duration"`
	MaxConcurrentCalls     int           `json:"max_concurrent_calls"`
	MaxConcurrentAuctions  int           `json:"max_concurrent_auctions"`
	CallTimeoutDuration    time.Duration `json:"call_timeout_duration"`
	FraudCheckEnabled      bool          `json:"fraud_check_enabled"`
	MetricsEnabled         bool          `json:"metrics_enabled"`

	// Routing configuration
	PreferDirectAssignment bool `json:"prefer_direct_assignment"`
	EnableSellerFirst      bool `json:"enable_seller_first"`
	EnableAuctionFallback  bool `json:"enable_auction_fallback"`

	// Quality thresholds
	MinBuyerQualityScore  float64 `json:"min_buyer_quality_score"`
	MinSellerQualityScore float64 `json:"min_seller_quality_score"`
	MaxFraudRiskScore     float64 `json:"max_fraud_risk_score"`
}

// Internal state management
type AuctionState struct {
	AuctionID    uuid.UUID
	CallID       uuid.UUID
	StartTime    time.Time
	Duration     time.Duration
	Status       AuctionStatus
	BidCount     int
	HighestBid   *bid.Bid
	Participants []uuid.UUID
}

type CallState struct {
	CallID    uuid.UUID
	Status    CallProcessingStatus
	Path      ProcessingPath
	StartTime time.Time
	SellerID  *uuid.UUID
	BuyerID   *uuid.UUID
	AuctionID *uuid.UUID
	Retries   int
	Errors    []ProcessingError
}

// NewOrchestrator creates a new marketplace orchestrator
func NewOrchestrator(
	callRepo CallRepository,
	bidRepo BidRepository,
	accountRepo AccountRepository,
	buyerRouting BuyerRoutingService,
	sellerDist SellerDistributionService,
	bidding BiddingService,
	fraud FraudService,
	telephony TelephonyService,
	metrics MarketplaceMetricsCollector,
	config *OrchestratorConfig,
) MarketplaceOrchestrator {
	if config == nil {
		config = defaultConfig()
	}

	return &orchestrator{
		callRepo:        callRepo,
		bidRepo:         bidRepo,
		accountRepo:     accountRepo,
		buyerRouting:    buyerRouting,
		sellerDist:      sellerDist,
		bidding:         bidding,
		fraud:           fraud,
		telephony:       telephony,
		metrics:         metrics,
		config:          config,
		activeAuctions:  make(map[uuid.UUID]*AuctionState),
		processingCalls: make(map[uuid.UUID]*CallState),
	}
}

// ProcessIncomingCall handles a new call entering the marketplace system
func (o *orchestrator) ProcessIncomingCall(ctx context.Context, request *IncomingCallRequest) (*CallProcessingResult, error) {
	startTime := time.Now()

	// Validate request
	if err := o.validateIncomingCallRequest(request); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", err.Error())
	}

	// Create the call entity
	newCall, err := o.createCallFromRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create call: %w", err)
	}

	// Store the call
	if err := o.callRepo.Create(ctx, newCall); err != nil {
		return nil, fmt.Errorf("failed to store call: %w", err)
	}

	// Initialize call state tracking
	callState := &CallState{
		CallID:    newCall.ID,
		Status:    ProcessingStatusAccepted,
		StartTime: startTime,
		SellerID:  request.SellerID,
		BuyerID:   request.BuyerID,
		Retries:   0,
		Errors:    []ProcessingError{},
	}

	o.mu.Lock()
	o.processingCalls[newCall.ID] = callState
	o.mu.Unlock()

	// Determine processing path
	path := o.determineProcessingPath(request)
	callState.Path = path

	result := &CallProcessingResult{
		CallID:         newCall.ID,
		Status:         ProcessingStatusAccepted,
		ProcessingPath: path,
		ProcessedAt:    startTime,
		Metadata:       make(map[string]interface{}),
	}

	// Execute processing based on path
	switch path {
	case PathDirectAssignment:
		err = o.processDirectAssignment(ctx, newCall, result)
	case PathSellerDistribution:
		err = o.processSellerDistribution(ctx, newCall, result)
	case PathAuction:
		err = o.processAuction(ctx, newCall, result)
	default:
		err = fmt.Errorf("unknown processing path: %s", path)
	}

	if err != nil {
		result.Status = ProcessingStatusFailed
		result.Errors = append(result.Errors, ProcessingError{
			Code:        "PROCESSING_FAILED",
			Message:     err.Error(),
			Service:     "marketplace_orchestrator",
			Recoverable: true,
			Timestamp:   time.Now(),
		})
	}

	result.EstimatedDelay = time.Since(startTime)

	// Record metrics
	if o.metrics != nil && o.config.MetricsEnabled {
		o.metrics.RecordCallProcessing(ctx, result)
	}

	return result, nil
}

// ProcessSellerCall handles a call from a seller for distribution
func (o *orchestrator) ProcessSellerCall(ctx context.Context, callID uuid.UUID) (*SellerCallResult, error) {
	startTime := time.Now()

	// Get the call
	call, err := o.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to get call: %w", err)
	}

	// Validate call is from a seller
	if call.SellerID == nil {
		return nil, errors.NewValidationError("INVALID_CALL", "call must have a seller ID")
	}

	// Distribute to potential buyers
	distributionResult, err := o.sellerDist.DistributeCall(ctx, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to distribute call: %w", err)
	}

	result := &SellerCallResult{
		CallID:             callID,
		DistributionResult: distributionResult,
		NotifiedBuyers:     distributionResult.SelectedSellers, // These become potential buyers
		ProcessedAt:        startTime,
		Status:             "distributed",
	}

	// Start auction if buyers were found
	if len(distributionResult.SelectedSellers) > 0 {
		auctionInfo, err := o.bidding.StartAuction(ctx, callID, o.config.DefaultAuctionDuration)
		if err == nil {
			result.AuctionStarted = true
			result.AuctionID = &auctionInfo.ID
			result.EstimatedDuration = o.config.DefaultAuctionDuration
			result.Status = "auction_started"

			// Track auction state
			o.mu.Lock()
			o.activeAuctions[auctionInfo.ID] = &AuctionState{
				AuctionID:    auctionInfo.ID,
				CallID:       callID,
				StartTime:    startTime,
				Duration:     o.config.DefaultAuctionDuration,
				Status:       AuctionStatusActive,
				BidCount:     0,
				Participants: distributionResult.SelectedSellers,
			}
			o.mu.Unlock()
		}
	}

	return result, nil
}

// ProcessBuyerBid handles a new bid from a buyer
func (o *orchestrator) ProcessBuyerBid(ctx context.Context, request *BidRequest) (*BidProcessingResult, error) {
	startTime := time.Now()

	// Validate bid request
	if err := o.validateBidRequest(request); err != nil {
		return nil, errors.NewValidationError("INVALID_BID", err.Error())
	}

	// Get buyer account for fraud check
	buyer, err := o.accountRepo.GetByID(ctx, request.BuyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buyer account: %w", err)
	}

	// Create bid entity
	amount, err := values.NewMoneyFromFloat(request.Amount, request.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid bid amount: %w", err)
	}

	// Convert types to match domain
	bidCriteria := bid.BidCriteria{
		CallType: request.Criteria.CallType,
		Geography: bid.GeoCriteria{
			Countries: []string{"US"}, // Default for now
		},
		TimeWindow: bid.TimeWindow{
			StartHour: 9,
			EndHour:   17,
			Days:      []string{"mon", "tue", "wed", "thu", "fri"},
			Timezone:  "UTC",
		},
	}

	// For marketplace bid, we need to determine the seller ID from the call
	callEntity, err := o.callRepo.GetByID(ctx, request.CallID)
	if err != nil {
		return nil, fmt.Errorf("failed to get call for bid: %w", err)
	}

	var sellerID uuid.UUID
	if callEntity.SellerID != nil {
		sellerID = *callEntity.SellerID
	} else {
		return nil, fmt.Errorf("call has no seller ID for marketplace bid")
	}

	newBid, err := bid.NewBid(request.CallID, request.BuyerID, sellerID, amount, bidCriteria)
	if err != nil {
		return nil, fmt.Errorf("failed to create bid: %w", err)
	}

	result := &BidProcessingResult{
		BidID:       newBid.ID,
		Status:      BidStatusAccepted,
		ProcessedAt: startTime,
		Errors:      []ProcessingError{},
	}

	// Fraud check if enabled
	if o.config.FraudCheckEnabled && o.fraud != nil {
		fraudResult, err := o.fraud.CheckBid(ctx, newBid, buyer)
		if err == nil {
			result.FraudCheck = fraudResult
			if !fraudResult.Approved {
				result.Status = BidStatusFraudulent
				return result, nil
			}
		}
	}

	// Store the bid
	if err := o.bidRepo.Create(ctx, newBid); err != nil {
		return nil, fmt.Errorf("failed to store bid: %w", err)
	}

	// Check if this bid wins current auction
	o.mu.RLock()
	var auctionState *AuctionState
	for _, auction := range o.activeAuctions {
		if auction.CallID == request.CallID {
			auctionState = auction
			break
		}
	}
	o.mu.RUnlock()

	if auctionState != nil {
		o.mu.Lock()
		auctionState.BidCount++
		if auctionState.HighestBid == nil || newBid.Amount.Compare(auctionState.HighestBid.Amount) > 0 {
			auctionState.HighestBid = newBid
			result.IsWinning = true
			result.Rank = 1
		} else {
			result.Rank = auctionState.BidCount
		}
		o.mu.Unlock()
	}

	return result, nil
}

// ExecuteCallRouting performs the complete call routing workflow
func (o *orchestrator) ExecuteCallRouting(ctx context.Context, callID uuid.UUID) (*RoutingResult, error) {
	startTime := time.Now()

	// Get the call
	call, err := o.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to get call: %w", err)
	}

	// Execute buyer routing
	routingDecision, err := o.buyerRouting.RouteCall(ctx, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to route call: %w", err)
	}

	result := &RoutingResult{
		CallID:          callID,
		RoutingDecision: routingDecision,
		ProcessingTime:  time.Since(startTime),
		CompletedAt:     time.Now(),
	}

	// If routing found a buyer, initiate telephony
	result.SelectedBuyerID = &routingDecision.BuyerID

	// Update call with buyer assignment
	call.BuyerID = routingDecision.BuyerID
	call.UpdateStatus(callpkg.StatusQueued)

	if err := o.callRepo.Update(ctx, call); err != nil {
		return nil, fmt.Errorf("failed to update call: %w", err)
	}

	// Initiate telephony if service available
	if o.telephony != nil {
		telephonyResult, err := o.telephony.InitiateCall(ctx, call)
		if err == nil {
			result.TelephonyResult = telephonyResult
			result.FinalStatus = callpkg.StatusInProgress
		} else {
			result.FinalStatus = callpkg.StatusFailed
		}
	} else {
		result.FinalStatus = callpkg.StatusQueued
	}

	// Record metrics
	if o.metrics != nil && o.config.MetricsEnabled {
		o.metrics.RecordRoutingDecision(ctx, result)
	}

	return result, nil
}

// HandleAuctionCompletion processes auction results and assigns calls
func (o *orchestrator) HandleAuctionCompletion(ctx context.Context, auctionID uuid.UUID) (*AuctionResult, error) {
	startTime := time.Now()

	// Get auction state
	o.mu.RLock()
	auctionState, exists := o.activeAuctions[auctionID]
	o.mu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("auction not found")
	}

	// Complete the auction through bidding service
	biddingResult, err := o.bidding.CompleteAuction(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to complete auction: %w", err)
	}

	result := &AuctionResult{
		AuctionID:       auctionID,
		CallID:          auctionState.CallID,
		TotalBids:       auctionState.BidCount,
		AuctionDuration: time.Since(auctionState.StartTime),
		Status:          AuctionStatusCompleted,
		CompletedAt:     startTime,
	}

	// Process winning bid if any
	if biddingResult.WinnerID != uuid.Nil {
		// Get the winning bid
		winningBid, err := o.bidRepo.GetByID(ctx, biddingResult.WinningBidID)
		if err == nil && winningBid != nil {
			result.WinningBid = winningBid
			result.WinningBuyerID = &winningBid.BuyerID
			result.FinalPrice = winningBid.Amount.ToFloat64()
			result.Currency = winningBid.Amount.Currency()
		}

		// Route the call to the winning buyer
		_, err = o.ExecuteCallRouting(ctx, auctionState.CallID)
		if err != nil {
			// Log error but don't fail the auction completion
			result.Status = AuctionStatusCompleted // Still completed, routing failed
		}
	} else {
		result.Status = AuctionStatusExpired
	}

	// Cleanup auction state
	o.mu.Lock()
	delete(o.activeAuctions, auctionID)
	o.mu.Unlock()

	// Record metrics
	if o.metrics != nil && o.config.MetricsEnabled {
		o.metrics.RecordAuctionCompletion(ctx, result)
	}

	return result, nil
}

// GetMarketplaceStatus returns current marketplace status and metrics
func (o *orchestrator) GetMarketplaceStatus(ctx context.Context) (*MarketplaceStatus, error) {
	o.mu.RLock()
	activeCalls := len(o.processingCalls)
	pendingAuctions := len(o.activeAuctions)
	o.mu.RUnlock()

	// Get active participants
	buyers, err := o.accountRepo.GetActiveBuyers(ctx, 1000)
	if err != nil {
		buyers = []*account.Account{} // Don't fail on metrics error
	}

	sellers, err := o.accountRepo.GetActiveSellers(ctx, 1000)
	if err != nil {
		sellers = []*account.Account{} // Don't fail on metrics error
	}

	status := &MarketplaceStatus{
		ActiveCalls:     activeCalls,
		PendingAuctions: pendingAuctions,
		ActiveBuyers:    len(buyers),
		ActiveSellers:   len(sellers),
		LastUpdated:     time.Now(),
		SystemHealth:    o.calculateSystemHealth(),
	}

	// Get detailed metrics if available
	if o.metrics != nil && o.config.MetricsEnabled {
		metrics, err := o.metrics.GetCurrentMetrics(ctx)
		if err == nil {
			status.Metrics = metrics
		}
	}

	return status, nil
}

// Helper methods

func (o *orchestrator) validateIncomingCallRequest(request *IncomingCallRequest) error {
	if request.FromNumber == "" {
		return fmt.Errorf("from_number is required")
	}
	if request.ToNumber == "" {
		return fmt.Errorf("to_number is required")
	}
	if request.SellerID == nil && request.BuyerID == nil {
		return fmt.Errorf("either seller_id or buyer_id must be provided")
	}
	return nil
}

func (o *orchestrator) validateBidRequest(request *BidRequest) error {
	if request.CallID == uuid.Nil {
		return fmt.Errorf("call_id is required")
	}
	if request.BuyerID == uuid.Nil {
		return fmt.Errorf("buyer_id is required")
	}
	if request.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if request.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	return nil
}

func (o *orchestrator) createCallFromRequest(request *IncomingCallRequest) (*callpkg.Call, error) {
	var newCall *callpkg.Call
	var err error

	if request.SellerID != nil {
		// Marketplace call from seller
		newCall, err = callpkg.NewMarketplaceCall(request.FromNumber, request.ToNumber, *request.SellerID, request.Direction)
	} else if request.BuyerID != nil {
		// Direct call from buyer
		newCall, err = callpkg.NewCall(request.FromNumber, request.ToNumber, *request.BuyerID, request.Direction)
	} else {
		return nil, fmt.Errorf("invalid call request: no seller or buyer ID")
	}

	return newCall, err
}

func (o *orchestrator) determineProcessingPath(request *IncomingCallRequest) ProcessingPath {
	// Direct assignment if buyer is specified
	if request.BuyerID != nil {
		return PathDirectAssignment
	}

	// Seller distribution if seller is specified
	if request.SellerID != nil && o.config.EnableSellerFirst {
		return PathSellerDistribution
	}

	// Default to auction
	return PathAuction
}

func (o *orchestrator) processDirectAssignment(ctx context.Context, call *callpkg.Call, result *CallProcessingResult) error {
	// Route directly to the specified buyer
	routingResult, err := o.ExecuteCallRouting(ctx, call.ID)
	if err != nil {
		return err
	}

	result.Status = ProcessingStatusAssigned
	result.BuyerDecision = routingResult.RoutingDecision
	return nil
}

func (o *orchestrator) processSellerDistribution(ctx context.Context, call *callpkg.Call, result *CallProcessingResult) error {
	// Distribute call to potential buyers
	sellerResult, err := o.ProcessSellerCall(ctx, call.ID)
	if err != nil {
		return err
	}

	result.Status = ProcessingStatusAuction
	result.SellerDecision = sellerResult.DistributionResult
	result.AuctionID = sellerResult.AuctionID
	return nil
}

func (o *orchestrator) processAuction(ctx context.Context, call *callpkg.Call, result *CallProcessingResult) error {
	// Start auction for the call
	auctionInfo, err := o.bidding.StartAuction(ctx, call.ID, o.config.DefaultAuctionDuration)
	if err != nil {
		return err
	}

	result.Status = ProcessingStatusAuction
	result.AuctionID = &auctionInfo.ID
	result.EstimatedDelay = o.config.DefaultAuctionDuration
	return nil
}

func (o *orchestrator) calculateSystemHealth() SystemHealthStatus {
	o.mu.RLock()
	activeCalls := len(o.processingCalls)
	activeAuctions := len(o.activeAuctions)
	o.mu.RUnlock()

	// Simple health calculation based on load
	if activeCalls > o.config.MaxConcurrentCalls || activeAuctions > o.config.MaxConcurrentAuctions {
		return HealthUnhealthy
	}

	if activeCalls > o.config.MaxConcurrentCalls/2 || activeAuctions > o.config.MaxConcurrentAuctions/2 {
		return HealthDegraded
	}

	return HealthHealthy
}

func defaultConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		DefaultAuctionDuration: 30 * time.Second,
		MaxConcurrentCalls:     1000,
		MaxConcurrentAuctions:  100,
		CallTimeoutDuration:    5 * time.Minute,
		FraudCheckEnabled:      true,
		MetricsEnabled:         true,
		PreferDirectAssignment: false,
		EnableSellerFirst:      true,
		EnableAuctionFallback:  true,
		MinBuyerQualityScore:   5.0,
		MinSellerQualityScore:  5.0,
		MaxFraudRiskScore:      0.7,
	}
}
