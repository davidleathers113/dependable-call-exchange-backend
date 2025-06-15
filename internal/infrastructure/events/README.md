# Audit Event Publisher

The Audit Event Publisher provides a scalable, real-time event streaming system for audit events in the Dependable Call Exchange Backend. It supports multiple transport protocols and implements sophisticated filtering, routing, and delivery guarantees.

## Features

### Core Capabilities
- **Multi-Transport Support**: WebSocket, Kafka, gRPC, HTTP
- **Real-time Streaming**: Low-latency event delivery
- **Event Filtering**: Topic-based subscriptions with fine-grained filters
- **Guaranteed Delivery**: Retry logic for critical events
- **Backpressure Handling**: Circuit breaker and flow control
- **Performance Monitoring**: Comprehensive metrics and tracing

### Transport Protocols

#### WebSocket Transport
- Real-time bidirectional communication
- Automatic reconnection handling
- Ping/pong health checks
- Message batching for efficiency

#### Kafka Transport
- High-throughput event streaming
- Topic-based routing by severity/type
- Guaranteed ordering per entity
- Configurable partitioning strategy

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Audit Event Publisher                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │Event Router │  │ Backpressure │  │ Worker Pool      │  │
│  │             │  │ Controller   │  │ - Regular: 10    │  │
│  │ • Filtering │  │              │  │ - Critical: 5    │  │
│  │ • Caching   │  │ • Flow Control│  │ - Batch: async  │  │
│  │ • Indexing  │  │ • Circuit     │  │                 │  │
│  └─────────────┘  │   Breaker    │  └──────────────────┘  │
│                   └──────────────┘                         │
├─────────────────────────────────────────────────────────────┤
│                      Transport Layer                         │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  WebSocket  │  │    Kafka     │  │      gRPC       │  │
│  │  Transport  │  │  Transport   │  │   Transport     │  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Creating the Publisher

```go
// Create with factory
factory := events.NewFactory(logger, config)
publisher, err := factory.CreateAuditPublisher(ctx)
if err != nil {
    return err
}
defer publisher.Close()

// Or create manually
transports := map[events.TransportType]events.EventTransport{
    events.TransportWebSocket: wsTransport,
    events.TransportKafka:     kafkaTransport,
}

publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
```

### Publishing Events

```go
// Publish single event
event := &audit.Event{
    ID:         uuid.New(),
    Type:       audit.EventTypeCallCreated,
    Severity:   audit.SeverityInfo,
    Timestamp:  time.Now(),
    UserID:     userID,
    EntityType: "call",
    EntityID:   callID,
    Action:     "create",
    Result:     audit.ResultSuccess,
    Metadata: map[string]interface{}{
        "from_number": "+1234567890",
        "to_number":   "+0987654321",
    },
}

err := publisher.Publish(ctx, event)

// Critical events get priority
criticalEvent := &audit.Event{
    Type:     audit.EventTypeSecurityBreach,
    Severity: audit.SeverityCritical,
    // ...
}
err := publisher.Publish(ctx, criticalEvent)
```

### Subscribing to Events

```go
// Subscribe with filters
filters := events.EventFilters{
    EventTypes: []audit.EventType{
        audit.EventTypeCallCreated,
        audit.EventTypeCallCompleted,
    },
    Severity: []audit.Severity{
        audit.SeverityHigh,
        audit.SeverityCritical,
    },
    EntityTypes: []string{"call", "bid"},
    TimeRange: &events.TimeRange{
        Start: time.Now().Add(-24 * time.Hour),
        End:   time.Now(),
    },
}

subscription, err := publisher.Subscribe(
    ctx,
    userID,
    events.TransportWebSocket,
    filters,
    connectionData, // Transport-specific data
)

// Later: unsubscribe
err = publisher.Unsubscribe(ctx, subscription.ID)
```

### WebSocket Integration

```go
// In your WebSocket handler
func HandleAuditStream(w http.ResponseWriter, r *http.Request) {
    // Upgrade connection
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    // Add to transport
    wsTransport.AddConnection(connectionID, conn, userID)
    
    // Connection will now receive filtered audit events
}
```

### Monitoring

```go
// Get publisher metrics
metrics := publisher.GetMetrics()
// Returns:
// - events_published
// - events_failed
// - events_dropped
// - queue_size
// - active_subscriptions

// Get subscription stats
stats, err := publisher.GetSubscriptionStats(subscriptionID)
// Returns delivery statistics per subscription

// Health check
err := publisher.Health()
```

## Configuration

### Publisher Configuration

```yaml
audit:
  publisher:
    event_queue_size: 10000
    critical_queue_size: 1000
    worker_count: 10
    critical_workers: 5
    batch_size: 100
    batch_timeout: 50ms
    max_retries: 3
    retry_delay: 100ms
    retry_backoff: 2.0
    max_queue_depth: 5000
    backpressure_delay: 10ms
    send_timeout: 5s
    shutdown_timeout: 30s
```

### WebSocket Configuration

```yaml
websocket:
  write_timeout: 10s
  ping_interval: 30s
  pong_timeout: 60s
  max_message_size: 1048576  # 1MB
  send_buffer_size: 256
```

### Kafka Configuration

```yaml
kafka:
  enabled: true
  brokers:
    - kafka1:9092
    - kafka2:9092
  topic: audit-events
  compression_type: snappy
  batch_size: 100
  linger_ms: 10
  retry_max: 3
  required_acks: 1
  idempotent_writes: true
  enable_topic_routing: true
  topic_prefix: audit
  
  # Security
  enable_tls: true
  enable_sasl: true
  sasl_mechanism: SCRAM-SHA-512
  sasl_username: ${KAFKA_USERNAME}
  sasl_password: ${KAFKA_PASSWORD}
```

## Event Filtering

The event router supports sophisticated filtering:

### Filter Types

1. **Event Type Filtering**
   - Filter by specific event types (e.g., CallCreated, BidAccepted)

2. **Severity Filtering**
   - Critical, High, Medium, Low, Info

3. **Entity Filtering**
   - By entity type (call, bid, account)
   - By specific entity IDs

4. **User Filtering**
   - Events related to specific users

5. **Time Range Filtering**
   - Events within a specific time window

6. **Custom Filters**
   - Extensible filter system for complex criteria

### Filter Examples

```go
// Get only critical security events
securityFilters := events.EventFilters{
    EventTypes: []audit.EventType{
        audit.EventTypeSecurityBreach,
        audit.EventTypeSuspiciousActivity,
    },
    Severity: []audit.Severity{audit.SeverityCritical},
}

// Get all events for specific entities
entityFilters := events.EventFilters{
    EntityIDs: []uuid.UUID{callID1, callID2, callID3},
}

// Complex filter combining multiple criteria
complexFilters := events.EventFilters{
    EventTypes: []audit.EventType{audit.EventTypeCallCompleted},
    Severity:   []audit.Severity{audit.SeverityHigh, audit.SeverityCritical},
    UserIDs:    []uuid.UUID{buyerID},
    TimeRange: &events.TimeRange{
        Start: startTime,
        End:   endTime,
    },
}
```

## Performance Optimization

### Batching
- Events are automatically batched for efficiency
- Configurable batch size and timeout
- Separate batching per transport

### Caching
- Route decisions are cached to reduce computation
- LRU cache with configurable TTL
- Cache invalidation on subscription changes

### Worker Pools
- Separate worker pools for regular and critical events
- Configurable worker counts
- Non-blocking event queuing

### Backpressure
- Circuit breaker prevents system overload
- Configurable queue depth limits
- Graceful degradation under load

## Monitoring and Observability

### Metrics
- OpenTelemetry metrics for all operations
- Prometheus-compatible metric endpoints
- Per-transport and per-subscription metrics

### Tracing
- Distributed tracing with OpenTelemetry
- Trace context propagation across transports
- Detailed span attributes for debugging

### Health Checks
- Transport-level health monitoring
- Queue depth monitoring
- Circuit breaker state tracking

## Error Handling

### Retry Logic
- Exponential backoff for transient failures
- Configurable retry limits
- Dead letter queue for persistent failures

### Circuit Breaker
- Protects against cascading failures
- Automatic recovery testing
- Configurable thresholds

### Graceful Shutdown
- Drains event queues on shutdown
- Waits for in-flight events
- Configurable shutdown timeout

## Security Considerations

1. **Transport Security**
   - TLS for all network communications
   - Authentication per subscription
   - Authorization checks on event access

2. **Data Privacy**
   - PII filtering in audit events
   - Encryption at rest for Kafka
   - Secure credential management

3. **Rate Limiting**
   - Per-user subscription limits
   - Event publishing rate limits
   - Connection throttling

## Future Enhancements

1. **Additional Transports**
   - gRPC streaming support
   - HTTP/2 Server-Sent Events
   - Message queue integrations (RabbitMQ, AWS SQS)

2. **Advanced Filtering**
   - Complex event processing (CEP)
   - Pattern matching across events
   - Machine learning-based filtering

3. **Enhanced Reliability**
   - Persistent event storage
   - Exactly-once delivery guarantees
   - Cross-region replication

4. **Performance Improvements**
   - Zero-copy event routing
   - SIMD-optimized filtering
   - Hardware acceleration support