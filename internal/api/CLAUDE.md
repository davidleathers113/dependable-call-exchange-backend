# API Layer Context

## Directory Structure
- `grpc/` - Internal service communication
- `rest/` - External API endpoints  
- `websocket/` - Real-time bidding events

## API Implementation Guidelines

### REST API (when implementing)
- Use chi router with middleware chain
- Standard response format: `{"data": {...}, "error": null}`
- Error format: `{"data": null, "error": {"code": "ERR_CODE", "message": "..."}}`
- Use domain errors from `internal/domain/errors`
- Include request ID in all responses
- Implement rate limiting per endpoint

### WebSocket Implementation
- Use gorilla/websocket for handling connections
- Message format: `{"type": "event_type", "payload": {...}}`
- Implement heartbeat/ping-pong for connection health
- Use channels for broadcasting to multiple clients
- Handle reconnection with message queue

### gRPC Services
- Define `.proto` files in `api/grpc/proto/`
- Use protoc-gen-go for code generation
- Implement interceptors for auth and logging
- Use streaming for real-time updates
- Include deadline/timeout in context

## Authentication & Security
- JWT tokens for REST and WebSocket
- mTLS for internal gRPC communication
- API key authentication for external integrations
- Rate limiting based on account tier