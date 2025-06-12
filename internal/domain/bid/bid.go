package bid

import (
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// Bid represents a buyer's offer to purchase a call from a seller
// IMPORTANT: Only buyers place bids on seller calls in the marketplace
type Bid struct {
	ID       uuid.UUID    `json:"id"`
	CallID   uuid.UUID    `json:"call_id"`   // The seller's call being bid on
	BuyerID  uuid.UUID    `json:"buyer_id"`  // The buyer placing this bid
	SellerID uuid.UUID    `json:"seller_id"` // The seller who owns the call (redundant with Call.SellerID)
	Amount   values.Money `json:"amount"`    // Amount buyer is willing to pay per call
	Status   Status       `json:"status"`    // Active, Won, Lost, Expired

	// Auction details
	AuctionID uuid.UUID `json:"auction_id"`
	Rank      int       `json:"rank"`

	// Targeting criteria
	Criteria BidCriteria `json:"criteria"`

	// Quality metrics
	Quality values.QualityMetrics `json:"quality"`

	// Timestamps
	PlacedAt   time.Time  `json:"placed_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Status int

const (
	StatusPending Status = iota
	StatusActive
	StatusWinning
	StatusWon
	StatusLost
	StatusExpired
	StatusCanceled
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusActive:
		return "active"
	case StatusWinning:
		return "winning"
	case StatusWon:
		return "won"
	case StatusLost:
		return "lost"
	case StatusExpired:
		return "expired"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

type BidCriteria struct {
	Geography   GeoCriteria  `json:"geography"`
	TimeWindow  TimeWindow   `json:"time_window"`
	CallType    []string     `json:"call_type"`
	Keywords    []string     `json:"keywords"`
	ExcludeList []string     `json:"exclude_list"`
	MaxBudget   values.Money `json:"max_budget"`
}

type GeoCriteria struct {
	Countries []string `json:"countries"`
	States    []string `json:"states"`
	Cities    []string `json:"cities"`
	ZipCodes  []string `json:"zip_codes"`
	Radius    *float64 `json:"radius,omitempty"`
}

type TimeWindow struct {
	StartHour int      `json:"start_hour"`
	EndHour   int      `json:"end_hour"`
	Days      []string `json:"days"`
	Timezone  string   `json:"timezone"`
}

type Auction struct {
	ID         uuid.UUID     `json:"id"`
	CallID     uuid.UUID     `json:"call_id"`
	Status     AuctionStatus `json:"status"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	WinningBid *uuid.UUID    `json:"winning_bid,omitempty"`
	Bids       []Bid         `json:"bids"`

	// Auction parameters
	ReservePrice values.Money `json:"reserve_price"`
	BidIncrement values.Money `json:"bid_increment"`
	MaxDuration  int          `json:"max_duration"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuctionStatus int

const (
	AuctionStatusPending AuctionStatus = iota
	AuctionStatusActive
	AuctionStatusCompleted
	AuctionStatusCanceled
	AuctionStatusExpired
)

func (s AuctionStatus) String() string {
	switch s {
	case AuctionStatusPending:
		return "pending"
	case AuctionStatusActive:
		return "active"
	case AuctionStatusCompleted:
		return "completed"
	case AuctionStatusCanceled:
		return "canceled"
	case AuctionStatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// NewBidCriteriaFromMap converts a map[string]any to BidCriteria
func NewBidCriteriaFromMap(data map[string]any) (BidCriteria, error) {
	criteria := BidCriteria{}

	// Handle geography criteria
	if geo, ok := data["geography"]; ok {
		if geoMap, ok := geo.(map[string]any); ok {
			criteria.Geography = parseGeoCriteria(geoMap)
		}
	}

	// Handle individual geography fields for backward compatibility
	if countries, ok := data["countries"]; ok {
		criteria.Geography.Countries = parseStringSlice(countries)
	}
	if states, ok := data["states"]; ok {
		criteria.Geography.States = parseStringSlice(states)
	}
	if cities, ok := data["cities"]; ok {
		criteria.Geography.Cities = parseStringSlice(cities)
	}
	if zipCodes, ok := data["zip_codes"]; ok {
		criteria.Geography.ZipCodes = parseStringSlice(zipCodes)
	}
	if radius, ok := data["radius"]; ok {
		if radiusFloat, ok := radius.(float64); ok {
			criteria.Geography.Radius = &radiusFloat
		}
	}

	// Handle location (legacy field name)
	if location, ok := data["location"]; ok {
		if locationStr, ok := location.(string); ok {
			criteria.Geography.Countries = []string{locationStr}
		}
	}

	// Handle time window
	if timeWindow, ok := data["time_window"]; ok {
		if twMap, ok := timeWindow.(map[string]any); ok {
			criteria.TimeWindow = parseTimeWindow(twMap)
		}
	}

	// Handle call type
	if callType, ok := data["call_type"]; ok {
		criteria.CallType = parseStringSlice(callType)
	}

	// Handle keywords
	if keywords, ok := data["keywords"]; ok {
		criteria.Keywords = parseStringSlice(keywords)
	}

	// Handle language (common field)
	if language, ok := data["language"]; ok {
		if langStr, ok := language.(string); ok {
			criteria.Keywords = append(criteria.Keywords, "lang:"+langStr)
		}
	}

	// Handle exclude list
	if excludeList, ok := data["exclude_list"]; ok {
		criteria.ExcludeList = parseStringSlice(excludeList)
	}

	// Handle max budget
	if maxBudget, ok := data["max_budget"]; ok {
		if budget, err := parseMoneyValue(maxBudget); err == nil {
			criteria.MaxBudget = budget
		}
	}

	return criteria, nil
}

func parseGeoCriteria(data map[string]any) GeoCriteria {
	geo := GeoCriteria{}

	if countries, ok := data["countries"]; ok {
		geo.Countries = parseStringSlice(countries)
	}
	if states, ok := data["states"]; ok {
		geo.States = parseStringSlice(states)
	}
	if cities, ok := data["cities"]; ok {
		geo.Cities = parseStringSlice(cities)
	}
	if zipCodes, ok := data["zip_codes"]; ok {
		geo.ZipCodes = parseStringSlice(zipCodes)
	}
	if radius, ok := data["radius"]; ok {
		if radiusFloat, ok := radius.(float64); ok {
			geo.Radius = &radiusFloat
		}
	}

	return geo
}

func parseTimeWindow(data map[string]any) TimeWindow {
	tw := TimeWindow{}

	if startHour, ok := data["start_hour"]; ok {
		if sh, ok := startHour.(float64); ok {
			tw.StartHour = int(sh)
		}
	}
	if endHour, ok := data["end_hour"]; ok {
		if eh, ok := endHour.(float64); ok {
			tw.EndHour = int(eh)
		}
	}
	if days, ok := data["days"]; ok {
		tw.Days = parseStringSlice(days)
	}
	if timezone, ok := data["timezone"]; ok {
		if tz, ok := timezone.(string); ok {
			tw.Timezone = tz
		}
	}

	return tw
}

func parseStringSlice(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return []string{}
	}
}

func parseMoneyValue(value any) (values.Money, error) {
	switch v := value.(type) {
	case float64:
		return values.NewMoneyFromFloat(v, "USD")
	case string:
		return values.NewMoneyFromString(v, "USD")
	case map[string]any:
		if amount, ok := v["amount"]; ok {
			if currency, ok := v["currency"].(string); ok {
				if amountFloat, ok := amount.(float64); ok {
					return values.NewMoneyFromFloat(amountFloat, currency)
				}
				if amountStr, ok := amount.(string); ok {
					return values.NewMoneyFromString(amountStr, currency)
				}
			}
		}
		return values.Zero("USD"), fmt.Errorf("invalid money map structure")
	default:
		return values.Zero("USD"), fmt.Errorf("unsupported money value type: %T", value)
	}
}

func NewBid(callID, buyerID, sellerID uuid.UUID, amount values.Money, criteria BidCriteria) (*Bid, error) {
	// Validate UUIDs
	if callID == uuid.Nil {
		return nil, fmt.Errorf("call ID cannot be nil")
	}
	if buyerID == uuid.Nil {
		return nil, fmt.Errorf("buyer ID cannot be nil")
	}
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("seller ID cannot be nil")
	}

	// Validate amount
	if amount.IsZero() {
		return nil, fmt.Errorf("bid amount cannot be zero")
	}

	// Minimum bid amount
	minBidAmount, _ := values.NewMoneyFromFloat(0.01, amount.Currency())
	if amount.Compare(minBidAmount) < 0 {
		return nil, fmt.Errorf("bid amount must be at least %s", minBidAmount.String())
	}

	// Validate criteria
	if err := validateBidCriteria(criteria); err != nil {
		return nil, fmt.Errorf("invalid bid criteria: %w", err)
	}

	now := time.Now()
	return &Bid{
		ID:        uuid.New(),
		CallID:    callID,
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Amount:    amount,
		Status:    StatusPending,
		Criteria:  criteria,
		Quality:   values.NewDefaultQualityMetrics(),
		PlacedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute), // 5-minute expiry
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func validateBidCriteria(criteria BidCriteria) error {
	// Validate time window
	if criteria.TimeWindow.StartHour < 0 || criteria.TimeWindow.StartHour > 23 {
		return fmt.Errorf("invalid start hour: must be between 0 and 23")
	}
	if criteria.TimeWindow.EndHour < 0 || criteria.TimeWindow.EndHour > 23 {
		return fmt.Errorf("invalid end hour: must be between 0 and 23")
	}

	// Validate max budget if set
	if !criteria.MaxBudget.IsZero() {
		if criteria.MaxBudget.IsNegative() {
			return fmt.Errorf("max budget cannot be negative")
		}
	}

	// Validate geography radius if set
	if criteria.Geography.Radius != nil && *criteria.Geography.Radius < 0 {
		return fmt.Errorf("radius cannot be negative")
	}

	return nil
}

func (b *Bid) Accept() {
	now := time.Now()
	b.Status = StatusWon
	b.AcceptedAt = &now
	b.UpdatedAt = now
}

func (b *Bid) Reject() {
	b.Status = StatusLost
	b.UpdatedAt = time.Now()
}

// IsExpired checks if the bid has expired
func (b *Bid) IsExpired() bool {
	return time.Now().After(b.ExpiresAt)
}

// IsActive returns true if the bid is active and not expired
func (b *Bid) IsActive() bool {
	return (b.Status == StatusActive || b.Status == StatusPending) && !b.IsExpired()
}

// CanAccept validates if the bid can be accepted
func (b *Bid) CanAccept() error {
	if b.IsExpired() {
		return fmt.Errorf("bid has expired")
	}
	if b.Status != StatusActive && b.Status != StatusPending {
		return fmt.Errorf("bid is not in active status")
	}
	return nil
}

// Activate transitions bid from pending to active
func (b *Bid) Activate() error {
	if b.Status != StatusPending {
		return fmt.Errorf("can only activate pending bids")
	}
	if b.IsExpired() {
		return fmt.Errorf("cannot activate expired bid")
	}
	b.Status = StatusActive
	b.UpdatedAt = time.Now()
	return nil
}

// Cancel cancels the bid
func (b *Bid) Cancel() error {
	if b.Status == StatusWon || b.Status == StatusLost {
		return fmt.Errorf("cannot cancel completed bid")
	}
	b.Status = StatusCanceled
	b.UpdatedAt = time.Now()
	return nil
}

// GetTimeRemaining returns time until bid expires
func (b *Bid) GetTimeRemaining() time.Duration {
	if b.IsExpired() {
		return 0
	}
	return time.Until(b.ExpiresAt)
}

// MatchesCriteria checks if a call matches this bid's criteria
func (b *Bid) MatchesCriteria(callLocation Location, callTime time.Time, callType string) bool {
	// Check time window
	if !b.matchesTimeWindow(callTime) {
		return false
	}

	// Check geography
	if !b.matchesGeography(callLocation) {
		return false
	}

	// Check call type
	if !b.matchesCallType(callType) {
		return false
	}

	return true
}

func (b *Bid) matchesTimeWindow(callTime time.Time) bool {
	if b.Criteria.TimeWindow.StartHour == 0 && b.Criteria.TimeWindow.EndHour == 0 {
		return true // No time restriction
	}

	callHour := callTime.Hour()
	if b.Criteria.TimeWindow.StartHour <= b.Criteria.TimeWindow.EndHour {
		// Same day window (e.g., 9 AM to 5 PM)
		return callHour >= b.Criteria.TimeWindow.StartHour && callHour <= b.Criteria.TimeWindow.EndHour
	} else {
		// Overnight window (e.g., 10 PM to 6 AM)
		return callHour >= b.Criteria.TimeWindow.StartHour || callHour <= b.Criteria.TimeWindow.EndHour
	}
}

func (b *Bid) matchesGeography(location Location) bool {
	// Check countries
	if len(b.Criteria.Geography.Countries) > 0 {
		found := false
		for _, country := range b.Criteria.Geography.Countries {
			if country == location.Country {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check states
	if len(b.Criteria.Geography.States) > 0 {
		found := false
		for _, state := range b.Criteria.Geography.States {
			if state == location.State {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check cities
	if len(b.Criteria.Geography.Cities) > 0 {
		found := false
		for _, city := range b.Criteria.Geography.Cities {
			if city == location.City {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (b *Bid) matchesCallType(callType string) bool {
	if len(b.Criteria.CallType) == 0 {
		return true // No call type restriction
	}

	for _, allowedType := range b.Criteria.CallType {
		if allowedType == callType {
			return true
		}
	}
	return false
}

// Location represents a geographic location for matching
type Location struct {
	Country string
	State   string
	City    string
	ZipCode string
}

func NewAuction(callID uuid.UUID, reservePrice values.Money) (*Auction, error) {
	// Validate call ID
	if callID == uuid.Nil {
		return nil, fmt.Errorf("call ID cannot be nil")
	}

	// Validate reserve price
	if reservePrice.IsNegative() {
		return nil, fmt.Errorf("reserve price cannot be negative")
	}

	// Create bid increment
	bidIncrement, err := values.NewMoneyFromFloat(0.01, reservePrice.Currency())
	if err != nil {
		return nil, fmt.Errorf("failed to create bid increment: %w", err)
	}

	now := time.Now()
	return &Auction{
		ID:           uuid.New(),
		CallID:       callID,
		Status:       AuctionStatusPending,
		StartTime:    now,
		EndTime:      now.Add(30 * time.Second), // 30-second auction window
		ReservePrice: reservePrice,
		BidIncrement: bidIncrement,
		MaxDuration:  30,
		Bids:         []Bid{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Start begins the auction
func (a *Auction) Start() error {
	if a.Status != AuctionStatusPending {
		return fmt.Errorf("auction must be pending to start")
	}

	now := time.Now()
	a.Status = AuctionStatusActive
	a.StartTime = now
	a.EndTime = now.Add(time.Duration(a.MaxDuration) * time.Second)
	a.UpdatedAt = now

	return nil
}

// AddBid adds a bid to the auction
func (a *Auction) AddBid(bid *Bid) error {
	if a.Status != AuctionStatusActive {
		return fmt.Errorf("auction is not active")
	}

	if a.IsExpired() {
		return fmt.Errorf("auction has expired")
	}

	// Validate bid amount against reserve price
	if bid.Amount.Compare(a.ReservePrice) < 0 {
		return fmt.Errorf("bid amount %s is below reserve price %s", bid.Amount.String(), a.ReservePrice.String())
	}

	// Set auction ID on bid
	bid.AuctionID = a.ID

	// Add to bids slice
	a.Bids = append(a.Bids, *bid)

	// Update rank
	a.rankBids()

	a.UpdatedAt = time.Now()
	return nil
}

// GetWinningBid returns the current highest bid
func (a *Auction) GetWinningBid() *Bid {
	if len(a.Bids) == 0 {
		return nil
	}

	a.rankBids()

	// Return the first (highest) bid
	for i := range a.Bids {
		if a.Bids[i].Rank == 1 {
			return &a.Bids[i]
		}
	}

	return nil
}

// GetBidCount returns the number of active bids
func (a *Auction) GetBidCount() int {
	count := 0
	for _, bid := range a.Bids {
		if bid.IsActive() {
			count++
		}
	}
	return count
}

// IsExpired checks if the auction has expired
func (a *Auction) IsExpired() bool {
	return time.Now().After(a.EndTime)
}

// GetTimeRemaining returns time until auction ends
func (a *Auction) GetTimeRemaining() time.Duration {
	if a.IsExpired() {
		return 0
	}
	return time.Until(a.EndTime)
}

// Close finalizes the auction and determines winner
func (a *Auction) Close() error {
	if a.Status == AuctionStatusCompleted || a.Status == AuctionStatusCanceled {
		return fmt.Errorf("auction is already closed")
	}

	now := time.Now()

	// Determine winner
	winner := a.GetWinningBid()
	if winner != nil {
		a.WinningBid = &winner.ID
		winner.Accept()

		// Mark other bids as lost
		for i := range a.Bids {
			if a.Bids[i].ID != winner.ID && a.Bids[i].IsActive() {
				a.Bids[i].Reject()
			}
		}

		a.Status = AuctionStatusCompleted
	} else {
		// No bids or no valid bids
		a.Status = AuctionStatusExpired
	}

	a.UpdatedAt = now
	return nil
}

// Cancel cancels the auction
func (a *Auction) Cancel() error {
	if a.Status == AuctionStatusCompleted {
		return fmt.Errorf("cannot cancel completed auction")
	}

	// Mark all active bids as canceled
	for i := range a.Bids {
		if a.Bids[i].IsActive() {
			a.Bids[i].Cancel()
		}
	}

	a.Status = AuctionStatusCanceled
	a.UpdatedAt = time.Now()
	return nil
}

// ExtendTime extends the auction by the specified duration
func (a *Auction) ExtendTime(duration time.Duration) error {
	if a.Status != AuctionStatusActive {
		return fmt.Errorf("can only extend active auctions")
	}

	if duration <= 0 {
		return fmt.Errorf("extension duration must be positive")
	}

	a.EndTime = a.EndTime.Add(duration)
	a.UpdatedAt = time.Now()
	return nil
}

// rankBids sorts bids by amount (highest first) and assigns ranks
func (a *Auction) rankBids() {
	// Only rank active bids
	activeBids := make([]*Bid, 0, len(a.Bids))
	for i := range a.Bids {
		if a.Bids[i].IsActive() {
			activeBids = append(activeBids, &a.Bids[i])
		}
	}

	// Sort by amount (highest first), then by placement time (earliest first)
	for i := 0; i < len(activeBids); i++ {
		for j := i + 1; j < len(activeBids); j++ {
			// Compare amounts first
			cmp := activeBids[i].Amount.Compare(activeBids[j].Amount)
			if cmp < 0 || (cmp == 0 && activeBids[i].PlacedAt.After(activeBids[j].PlacedAt)) {
				// Swap
				activeBids[i], activeBids[j] = activeBids[j], activeBids[i]
			}
		}
	}

	// Assign ranks
	for i, bid := range activeBids {
		bid.Rank = i + 1
	}

	// Reset ranks for inactive bids
	for i := range a.Bids {
		if !a.Bids[i].IsActive() {
			a.Bids[i].Rank = 0
		}
	}
}
