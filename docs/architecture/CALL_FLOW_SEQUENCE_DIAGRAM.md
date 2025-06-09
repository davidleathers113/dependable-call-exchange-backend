# Call Flow Sequence Diagram

## Overview
This sequence diagram illustrates the complete lifecycle of a call in the Dependable Call Exchange system, from initial connection through compliance checks, real-time auction, routing, and final billing.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Caller
    participant Gateway as API Gateway
    participant CallSvc as Call Service
    participant Compliance
    participant Bidding
    participant Routing
    participant Buyer

    %% Call Initiation
    Caller->>Gateway: 1. Incoming Call
    activate Gateway
    Gateway->>CallSvc: 2. Authenticate
    activate CallSvc
    
    %% Call Record Creation
    CallSvc->>CallSvc: 3. Create Call Record
    Note over CallSvc: Status: PENDING<br/>Generate Call ID
    
    %% Compliance Checks
    CallSvc->>Compliance: 4. Check Compliance
    activate Compliance
    
    Compliance->>Compliance: 5. DNC Check
    Note over Compliance: Verify against<br/>Do-Not-Call list
    
    Compliance->>Compliance: 6. TCPA Check
    Note over Compliance: Verify time<br/>restrictions
    
    Compliance-->>CallSvc: 7. Compliance OK
    deactivate Compliance
    
    %% Auction Process
    CallSvc->>Bidding: 8. Start Auction
    activate Bidding
    
    Bidding->>Bidding: 9. Find Matching Bids
    Note over Bidding: Match call properties<br/>with bid criteria
    
    Bidding->>Bidding: 10. Execute Auction
    Note over Bidding: Run auction algorithm<br/>(First/Second price)
    
    Bidding->>Bidding: 11. Select Winner
    Note over Bidding: Highest bid wins<br/>Quality adjusted
    
    Bidding-->>CallSvc: 12. Winner: Buyer X
    deactivate Bidding
    
    %% Call Routing
    CallSvc->>Routing: 13. Route to Winner
    activate Routing
    
    Routing->>Routing: 14. Connect Call
    Note over Routing: Establish SIP/WebRTC<br/>connection
    
    Routing-->>Gateway: 15. Call Connected
    deactivate Routing
    
    Gateway-->>Caller: 16. Connected to Buyer
    deactivate Gateway
    
    %% Call in Progress
    rect rgb(230, 243, 255)
        Note over Caller,Buyer: Call in Progress<br/>Real-time monitoring, quality tracking, duration tracking
        Caller<->Buyer: Voice/Data Stream
    end
    
    %% Call Completion
    Caller->>Gateway: 17. Call Ended
    activate Gateway
    Gateway->>CallSvc: 18. Update Status
    
    CallSvc->>CallSvc: 19. Calculate Billing
    Note over CallSvc: Duration × Rate<br/>= Total Cost
    
    CallSvc->>CallSvc: 20. Process Payment
    Note over CallSvc: Debit Buyer<br/>Credit Seller
    
    deactivate CallSvc
    deactivate Gateway
```

## Detailed Flow Description

### Phase 1: Call Initiation (Steps 1-3)
1. **Incoming Call**: Caller initiates a call to the exchange
2. **Authenticate**: API Gateway validates the request and authenticates the caller
3. **Create Call Record**: Call Service creates a new call record with status PENDING

### Phase 2: Compliance Verification (Steps 4-7)
4. **Check Compliance**: Call Service requests compliance verification
5. **DNC Check**: Verify the number is not on the Do-Not-Call list
6. **TCPA Check**: Ensure call complies with time-based restrictions
7. **Compliance OK**: All checks pass, call can proceed

### Phase 3: Real-time Auction (Steps 8-12)
8. **Start Auction**: Initiate the bidding process for this call
9. **Find Matching Bids**: Identify all bids that match the call criteria:
   - Geographic location
   - Call type
   - Time window
   - Quality requirements
10. **Execute Auction**: Run the auction algorithm (first-price or second-price)
11. **Select Winner**: Determine the winning bid based on:
    - Bid amount
    - Quality score
    - Capacity availability
12. **Winner Selected**: Return the winning buyer information

### Phase 4: Call Routing (Steps 13-16)
13. **Route to Winner**: Direct the call to the winning buyer
14. **Connect Call**: Establish the telephony connection (SIP/WebRTC)
15. **Call Connected**: Confirm successful connection
16. **Notify Caller**: Inform the caller they're connected to the buyer

### Phase 5: Call Execution
- **Real-time Monitoring**: Track call quality metrics
- **Duration Tracking**: Record exact call length
- **Quality Scoring**: Monitor conversation quality
- **Fraud Detection**: Continuous monitoring for suspicious activity

### Phase 6: Call Completion (Steps 17-20)
17. **Call Ended**: Caller or buyer terminates the call
18. **Update Status**: Mark call as COMPLETED
19. **Calculate Billing**: Compute total cost based on duration and rate
20. **Process Payment**: 
    - Debit the buyer's account
    - Credit the seller's account
    - Record transaction

## Key Features

### Performance Requirements
- **Auction Execution**: < 100ms total
- **Routing Decision**: < 1ms
- **Compliance Checks**: < 50ms
- **End-to-end Latency**: < 200ms

### Compliance Features
- **DNC List**: Real-time verification against Do-Not-Call registry
- **TCPA Compliance**: Time-zone aware calling restrictions
- **GDPR**: Consent verification for EU numbers
- **Geographic Restrictions**: State/country specific rules

### Financial Processing
- **Real-time Billing**: Immediate cost calculation
- **Prepaid Balance**: Buyers maintain account balance
- **Automatic Settlement**: Instant crediting to sellers
- **Transaction Logging**: Complete audit trail

### Quality Assurance
- **Call Recording**: Optional recording for quality review
- **Duration Tracking**: Minimum call length requirements
- **Quality Scoring**: Post-call quality metrics
- **Fraud Detection**: ML-based anomaly detection

## Error Handling

### Compliance Failures
- **DNC Violation**: Call blocked, caller notified
- **TCPA Violation**: Call blocked with time restriction message
- **Invalid Number**: Format validation failure

### Auction Failures
- **No Matching Bids**: Call rejected, no buyers available
- **Insufficient Balance**: Buyer excluded from auction
- **All Bids Expired**: No active bids for the call

### Technical Failures
- **Connection Timeout**: Retry with backup routing
- **Network Issues**: Failover to secondary carrier
- **System Overload**: Queue management and backpressure

## Monitoring Points

1. **Call Initiation Rate**: Calls per second
2. **Auction Success Rate**: Percentage of calls with winning bids
3. **Average Auction Time**: Milliseconds per auction
4. **Call Success Rate**: Completed vs failed calls
5. **Revenue Per Call**: Average transaction value
6. **Compliance Block Rate**: Percentage blocked by compliance
7. **System Latency**: End-to-end processing time
