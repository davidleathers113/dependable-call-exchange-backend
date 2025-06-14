# Audit Events WebSocket Implementation

## Overview

This implementation provides real-time audit event streaming via WebSocket connections for the IMMUTABLE_AUDIT feature of the Dependable Call Exchange Backend. It supports 1000+ concurrent connections with advanced filtering, rate limiting, and role-based access control.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐      │
│  │ Admin   │  │Security │  │Compliance│  │Operators│      │
│  │Dashboard│  │ Console │  │ Monitor  │  │ Center  │      │
│  └────┬────┘  └────┬────┘  └────┬─────┘  └────┬────┘      │
│       │            │            │             │           │
│       └─────────┬──┴────────────┴─────────────┘           │
└─────────────────┼─────────────────────────────────────────┘
                  │ WebSocket Connections (ws://host/ws/audit)
┌─────────────────┼─────────────────────────────────────────┐
│                 ▼               API Layer                  │
│  ┌──────────────────────────────────────────────────────┐ │
│  │           WebSocket Handler                          │ │
│  │  • Authentication & Authorization                   │ │
│  │  • Connection Upgrade                               │ │
│  │  • Client Management                                │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────┼─────────────────────────────────────────┘
                  │
┌─────────────────┼─────────────────────────────────────────┐
│                 ▼            Core Streaming Engine        │
│  ┌──────────────────────────────────────────────────────┐ │
│  │              AuditEventHub                          │ │
│  │  • 1000+ Concurrent Connections                     │ │
│  │  • Real-time Event Broadcasting                     │ │
│  │  • Advanced Filtering Engine                        │ │
│  │  • Rate Limiting & Backpressure                     │ │
│  │  • Connection Health Monitoring                     │ │
│  │  • Performance Metrics Collection                   │ │
│  └──────────────────────────────────────────────────────┘ │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐ │
│  │              AuditClient Pool                       │ │
│  │  • Per-Client Event Filtering                       │ │
│  │  • Role-Based Access Control                        │ │
│  │  • Individual Rate Limiting                         │ │
│  │  • Connection Lifecycle Management                  │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────┼─────────────────────────────────────────┘
                  │ Event Subscription
┌─────────────────┼─────────────────────────────────────────┐
│                 ▼          Event Publishing Layer         │
│  ┌──────────────────────────────────────────────────────┐ │
│  │           AuditEventPublisher                        │ │
│  │  • Multi-Transport Support                          │ │
│  │  • Event Routing & Filtering                        │ │
│  │  • Batch Processing                                 │ │
│  │  • Delivery Guarantees                              │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────┼─────────────────────────────────────────┘
                  │ Audit Events
┌─────────────────┼─────────────────────────────────────────┐
│                 ▼           Business Logic Layer          │
│  ┌──────────────────────────────────────────────────────┐ │
│  │              Audit Logger                           │ │
│  │  • High-Performance Logging                         │ │
│  │  • Hash Chain Validation                            │ │
│  │  • Compliance Integration                           │ │
│  │  • Event Enrichment                                 │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

### 🚀 High-Scale Performance
- **1000+ Concurrent Connections**: Optimized for enterprise-scale real-time monitoring
- **Sub-millisecond Latency**: Event broadcasting with <1ms overhead
- **Efficient Resource Usage**: Memory-optimized client management with connection pooling
- **Horizontal Scaling**: Ready for multi-instance deployment

### 🔒 Security & Access Control
- **Role-Based Access Control**: Fine-grained permissions (admin, security, compliance, operator, auditor)
- **Authentication Integration**: Seamless integration with existing auth middleware
- **Permission Validation**: Event-level access control based on user roles
- **Secure WebSocket Upgrade**: Proper origin checking and security headers

### 🎯 Advanced Filtering
- **Multi-Dimensional Filters**: Filter by event type, severity, category, actor, target, time range
- **Real-Time Filter Updates**: Dynamic filter modification without reconnection
- **Compliance Filtering**: Built-in GDPR/TCPA relevance filtering
- **Custom Filter Support**: Extensible filtering with custom criteria

### ⚡ Rate Limiting & Flow Control
- **Per-Client Rate Limiting**: Configurable limits per second/minute
- **Backpressure Handling**: Graceful degradation under high load
- **Connection Management**: Automatic cleanup of stale connections
- **Buffer Management**: Multi-level buffering with overflow protection

### 📊 Monitoring & Observability
- **Real-Time Metrics**: Connection count, event throughput, latency tracking
- **Health Monitoring**: Connection health checks and automated recovery
- **Performance Analytics**: Detailed statistics and trending data
- **Debug Information**: Comprehensive logging and troubleshooting support

## Configuration

### Default Configuration (Optimized for 1000+ Connections)

```go
AuditHubConfig{
    MaxClients:          2000,              // Maximum concurrent connections
    BroadcastBufferSize: 10000,             // Event broadcast buffer size
    ClientBufferSize:    256,               // Per-client event buffer
    PingInterval:        30 * time.Second,  // WebSocket ping interval
    PongTimeout:         60 * time.Second,  // Pong response timeout
    ReadTimeout:         60 * time.Second,  // Client read timeout
    WriteTimeout:        10 * time.Second,  // Client write timeout
    MaxMessageSize:      32 * 1024,         // Maximum message size (32KB)
    RateLimitPerSecond:  100,               // Events per second per client
    RateLimitPerMinute:  1000,              // Events per minute per client
    CleanupInterval:     5 * time.Minute,   // Stale connection cleanup
    MetricsInterval:     30 * time.Second,  // Metrics collection interval
    EnableCompression:   true,              // WebSocket compression
    MaxFiltersPerClient: 50,                // Maximum filters per client
}
```

### Environment-Specific Tuning

#### Development
```go
config := DefaultAuditHubConfig()
config.MaxClients = 100
config.BroadcastBufferSize = 1000
config.RateLimitPerSecond = 10
```

#### Staging
```go
config := DefaultAuditHubConfig()
config.MaxClients = 500
config.BroadcastBufferSize = 5000
```

#### Production
```go
config := DefaultAuditHubConfig()
config.MaxClients = 2000
config.BroadcastBufferSize = 10000
config.EnableCompression = true
```

## Event Types

### Primary Event Types
- **`audit.event.published`**: Regular audit events from the audit logger
- **`audit.security.alert`**: Real-time security alerts
- **`audit.compliance.alert`**: Compliance violation alerts  
- **`audit.system.alert`**: System health and operational alerts

### Connection Management
- **`audit.connection.established`**: Client connected successfully
- **`audit.connection.ping`**: Ping message from client
- **`audit.connection.pong`**: Pong response from server

## Filtering System

### Filter Categories

#### Event Type Filters
```json
{
  "event_types": [
    "consent.granted",
    "consent.revoked", 
    "data.accessed",
    "call.initiated",
    "auth.failure",
    "security.incident"
  ]
}
```

#### Severity Filters
```json
{
  "severities": ["critical", "error", "warning", "info"]
}
```

#### Category Filters
```json
{
  "categories": ["security", "compliance", "call", "financial"]
}
```

#### Actor/Target Filters
```json
{
  "actor_ids": ["user123", "system"],
  "target_ids": ["call456", "bid789"]
}
```

#### Time Range Filters
```json
{
  "time_range": {
    "start": "2025-01-15T10:00:00Z",
    "end": "2025-01-15T18:00:00Z"
  }
}
```

#### Relative Time Filters
```json
{
  "time_range": {
    "relative": "1h"  // "1h", "24h", "7d", "30d"
  }
}
```

#### Compliance Filters
```json
{
  "compliance_only": true,  // Only GDPR/TCPA relevant events
  "security_only": true     // Only security-related events
}
```

### Dynamic Filter Updates

Clients can update filters in real-time without reconnection:

```javascript
ws.send(JSON.stringify({
  type: 'update_filters',
  filters: {
    event_types: ['security.incident', 'compliance.violation'],
    severities: ['critical', 'error'],
    time_range: { relative: '1h' }
  }
}));
```

## Role-Based Access Control

### Role Permissions Matrix

| Role | Audit Events | Security Alerts | Compliance Alerts | System Alerts | Connection Events |
|------|-------------|-----------------|------------------|---------------|------------------|
| **admin** | ✅ All | ✅ All | ✅ All | ✅ All | ✅ All |
| **security** | ✅ All | ✅ All | ✅ All | ❌ None | ✅ All |
| **compliance** | ✅ All | ❌ None | ✅ All | ❌ None | ✅ All |
| **operator** | ✅ All | ❌ None | ❌ None | ✅ All | ✅ All |
| **auditor** | ✅ All | ❌ None | ❌ None | ❌ None | ✅ All |
| **user** | ❌ None | ❌ None | ❌ None | ❌ None | ✅ Connection Only |

### Permission Checking

```go
func (h *AuditEventHub) hasPermissionForEvent(client *AuditClient, event *AuditStreamEvent) bool {
    // Admin can see everything
    if client.role == "admin" {
        return true
    }
    
    // Security personnel can see security and compliance alerts
    if client.role == "security" {
        return event.Type == AuditEventSecurityAlert || 
               event.Type == AuditEventComplianceAlert ||
               event.Type == AuditEventPublished
    }
    
    // Additional role checks...
}
```

## Rate Limiting

### Per-Client Rate Limiting

Each client has individual rate limits to prevent abuse:

```go
type ClientRateLimiter struct {
    maxEventsPerSecond int       // 100 events/second default
    maxEventsPerMinute int       // 1000 events/minute default
    windowSize         time.Duration  // 1 minute sliding window
    events             []time.Time    // Event timestamps
}
```

### Backpressure Handling

When buffers fill up, the system implements graceful degradation:

1. **Client Buffer Full**: Disconnect slow clients
2. **Broadcast Buffer Full**: Drop events with metrics
3. **Memory Pressure**: Apply connection limits
4. **Rate Limit Exceeded**: Temporarily block client

## Client Integration

### JavaScript/TypeScript Client

```typescript
interface AuditEventFilter {
  event_types?: string[];
  severities?: string[];
  categories?: string[];
  actor_ids?: string[];
  target_ids?: string[];
  time_range?: {
    start?: string;
    end?: string;
    relative?: string;
  };
  compliance_only?: boolean;
  security_only?: boolean;
}

class AuditEventClient {
  private ws: WebSocket;
  private filters: AuditEventFilter = {};
  
  constructor(private url: string, private token: string) {
    this.connect();
  }
  
  private connect() {
    this.ws = new WebSocket(`${this.url}?token=${this.token}`);
    
    this.ws.onopen = () => {
      console.log('Connected to audit event stream');
      this.applyFilters();
    };
    
    this.ws.onmessage = (event) => {
      const auditEvent = JSON.parse(event.data);
      this.handleAuditEvent(auditEvent);
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.ws.onclose = () => {
      // Implement reconnection logic
      setTimeout(() => this.connect(), 5000);
    };
  }
  
  public setFilters(filters: AuditEventFilter) {
    this.filters = filters;
    this.applyFilters();
  }
  
  private applyFilters() {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        type: 'update_filters',
        filters: this.filters
      }));
    }
  }
  
  private handleAuditEvent(event: any) {
    switch (event.type) {
      case 'audit.event.published':
        this.onAuditEvent(event.audit_event);
        break;
      case 'audit.security.alert':
        this.onSecurityAlert(event);
        break;
      case 'audit.compliance.alert':
        this.onComplianceAlert(event);
        break;
    }
  }
  
  protected onAuditEvent(event: any) {
    // Override in subclass
  }
  
  protected onSecurityAlert(alert: any) {
    // Override in subclass
  }
  
  protected onComplianceAlert(alert: any) {
    // Override in subclass
  }
}
```

### Go Client

```go
type AuditEventClient struct {
    conn    *websocket.Conn
    filters AuditEventFilters
    events  chan *AuditStreamEvent
}

func NewAuditEventClient(url string, token string) (*AuditEventClient, error) {
    headers := http.Header{}
    headers.Set("Authorization", "Bearer "+token)
    
    conn, _, err := websocket.DefaultDialer.Dial(url, headers)
    if err != nil {
        return nil, err
    }
    
    client := &AuditEventClient{
        conn:   conn,
        events: make(chan *AuditStreamEvent, 100),
    }
    
    go client.readPump()
    return client, nil
}

func (c *AuditEventClient) SetFilters(filters AuditEventFilters) error {
    message := map[string]interface{}{
        "type":    "update_filters",
        "filters": filters,
    }
    return c.conn.WriteJSON(message)
}

func (c *AuditEventClient) Events() <-chan *AuditStreamEvent {
    return c.events
}
```

## Performance Metrics

### Key Performance Indicators

#### Connection Metrics
- **Active Connections**: Current number of connected clients
- **Peak Connections**: Maximum concurrent connections reached
- **Connection Rate**: New connections per minute
- **Disconnect Rate**: Disconnections per minute

#### Event Metrics
- **Events Published**: Total events sent to clients
- **Events Filtered**: Events filtered out by client filters
- **Events Dropped**: Events dropped due to buffer overflow
- **Broadcast Latency**: Time to broadcast event to all clients

#### Resource Metrics
- **Memory Usage**: Total memory used by the hub
- **CPU Usage**: Processing overhead
- **Network Bandwidth**: Outbound data transfer
- **Buffer Utilization**: Current buffer usage percentage

### Metrics Collection

```go
type AuditHubMetrics struct {
    TotalConnections      int64
    ActiveConnections     int64
    TotalEventsPublished  int64
    TotalEventsFiltered   int64
    TotalEventsDropped    int64
    TotalBytesTransferred int64
    AverageLatency        time.Duration
    PeakConnections       int64
    ErrorCount            int64
    StartTime             time.Time
}

// Expose metrics via HTTP endpoint
func (h *AuditEventHub) GetMetrics() AuditHubMetrics {
    h.metrics.mu.RLock()
    defer h.metrics.mu.RUnlock()
    
    return h.metrics.copy()
}
```

## Error Handling

### Client-Side Error Recovery

```javascript
class RobustAuditClient extends AuditEventClient {
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 1000;
  
  private connect() {
    this.ws = new WebSocket(this.url);
    
    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.reconnectDelay = 1000;
      console.log('Connected to audit event stream');
    };
    
    this.ws.onclose = () => {
      this.handleReconnection();
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }
  
  private handleReconnection() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      setTimeout(() => {
        console.log(`Reconnection attempt ${this.reconnectAttempts + 1}`);
        this.reconnectAttempts++;
        this.connect();
      }, this.reconnectDelay);
      
      // Exponential backoff
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
    }
  }
}
```

### Server-Side Error Handling

```go
func (h *AuditEventHub) processEventBroadcast(event *AuditStreamEvent) {
    h.clientsLock.RLock()
    defer h.clientsLock.RUnlock()

    for _, client := range h.clients {
        select {
        case client.send <- event:
            // Success
        default:
            // Client channel full - mark for disconnection
            h.logger.Warn("Client channel full, scheduling disconnection",
                zap.String("client_id", client.ID.String()),
            )
            go func(c *AuditClient) {
                h.unregister <- c
            }(client)
        }
    }
}
```

## Deployment Considerations

### Horizontal Scaling

For large deployments, consider:

1. **Load Balancer Configuration**: Sticky sessions for WebSocket connections
2. **Event Distribution**: Redis pub/sub for multi-instance event distribution
3. **Connection Sharding**: Distribute clients across multiple instances
4. **Health Checks**: Load balancer health checks for WebSocket endpoints

### Resource Planning

#### Memory Requirements
- **Base Memory**: ~100MB for hub infrastructure
- **Per Connection**: ~1KB per client connection
- **Buffer Memory**: ~10MB for 10K event buffer
- **Total for 1000 clients**: ~111MB

#### CPU Requirements
- **Base CPU**: 0.1 CPU cores for hub management
- **Per 1000 connections**: 0.2 CPU cores
- **Event processing**: 0.1 CPU cores per 1000 events/second

### Monitoring Setup

```yaml
# Prometheus metrics
audit_websocket_connections_total{state="active"}
audit_websocket_events_published_total
audit_websocket_events_dropped_total
audit_websocket_latency_seconds{quantile="0.99"}
audit_websocket_memory_usage_bytes
```

## Troubleshooting

### Common Issues

#### High Memory Usage
```bash
# Check connection count
curl http://localhost:8080/debug/websocket/audit/stats

# Monitor memory usage
curl http://localhost:8080/debug/pprof/heap
```

#### Slow Event Delivery
```bash
# Check buffer utilization
curl http://localhost:8080/debug/websocket/audit/metrics

# Enable debug logging
DCE_LOG_LEVEL=debug ./dce-backend
```

#### Connection Drops
```bash
# Check client error rates
curl http://localhost:8080/debug/websocket/audit/clients

# Monitor network connectivity
netstat -an | grep :8080
```

### Debug Endpoints

- **`/debug/websocket/audit/stats`**: Connection statistics
- **`/debug/websocket/audit/metrics`**: Performance metrics  
- **`/debug/websocket/audit/clients`**: Connected client details
- **`/debug/websocket/audit/config`**: Current configuration
- **`/debug/pprof/goroutine`**: Goroutine analysis
- **`/debug/pprof/heap`**: Memory analysis

## Security Considerations

### WebSocket Security
- **Origin Validation**: Restrict allowed origins
- **TLS Encryption**: Use WSS in production
- **Rate Limiting**: Prevent abuse and DoS attacks
- **Authentication**: Validate JWT tokens on connection

### Data Protection
- **Sensitive Data Filtering**: Remove PII from event streams
- **Access Logging**: Log all connection attempts
- **Audit Trail**: Track filter changes and access patterns
- **Encryption**: Encrypt sensitive metadata in events

### Production Checklist

- [ ] Enable TLS/WSS encryption
- [ ] Configure proper CORS/Origin policies
- [ ] Set up authentication middleware
- [ ] Configure rate limiting
- [ ] Enable access logging
- [ ] Set up monitoring and alerting
- [ ] Configure automatic reconnection
- [ ] Test failover scenarios
- [ ] Validate performance under load
- [ ] Review security settings

This implementation provides a robust, scalable, and secure foundation for real-time audit event streaming in the Dependable Call Exchange Backend, supporting the IMMUTABLE_AUDIT feature requirements with enterprise-grade performance and reliability.