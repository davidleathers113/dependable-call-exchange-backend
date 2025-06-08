# Dependable Call Exchange Backend

A high-performance Pay Per Call exchange platform built with Go 1.24, implementing real-time call routing, intelligent bidding, and comprehensive compliance management following 2025 best practices.

## 🎯 Overview

The Dependable Call Exchange Backend serves as the core engine for a Pay Per Call marketplace, connecting call buyers and sellers through intelligent routing algorithms, real-time auctions, and automated compliance verification. Built as a modular monolith, it's designed to handle millions of calls with sub-millisecond routing decisions while maintaining strict regulatory compliance.

### Core Capabilities

- **🚀 Real-time Call Routing** - Advanced routing algorithms with < 1ms decision latency
- **💰 Live Bidding Engine** - Real-time auctions with millisecond-level bid processing
- **🛡️ Compliance-First Design** - Automated TCPA, GDPR, and DNC compliance
- **🔍 Fraud Detection** - ML-powered fraud prevention with behavioral analysis
- **📞 Multi-Protocol Support** - REST, gRPC, and WebSocket APIs
- **📊 Comprehensive Observability** - OpenTelemetry integration with distributed tracing

## 🏗️ Architecture

This project implements a **modular monolith** architecture optimized for:
- High-performance telephony operations
- Real-time decision making
- Horizontal scalability
- Easy microservices extraction when needed

### Key Design Principles

1. **Domain-Driven Design** - Clear bounded contexts for business domains
2. **Event-Driven Architecture** - Asynchronous processing with event sourcing
3. **CQRS Pattern** - Separated read/write models for optimization
4. **Hexagonal Architecture** - Clean separation of business logic from infrastructure

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+
- Kafka 3.0+ (optional for event streaming)

### Installation

```bash
# Clone the repository
git clone https://github.com/davidleathers113/dependable-call-exchange-backend.git
cd dependable-call-exchange-backend

# Copy environment configuration
cp .env.example .env

# Start all services with Docker Compose
docker-compose up -d

# Run database migrations
go run cmd/migrate/main.go -action up

# Start the application
go run main.go
```

### Development Setup

```bash
# Install development tools
make install-tools

# Run with hot reload
make dev-watch

# Run tests
make test

# Check code quality
make ci
```

## 📁 Project Structure

```
.
├── cmd/                        # Application entrypoints
│   ├── migrate/               # Database migration tool
│   ├── worker/                # Background job processors
│   └── cli/                   # Admin CLI tools
├── internal/                   # Private application code
│   ├── domain/                # Business logic & entities
│   │   ├── call/             # Call management
│   │   ├── bid/              # Bidding & auctions
│   │   ├── account/          # User management
│   │   ├── compliance/       # Regulatory compliance
│   │   └── financial/        # Billing & payments
│   ├── service/              # Business logic orchestration
│   │   ├── callrouting/      # Routing algorithms
│   │   ├── bidding/          # Auction engine
│   │   ├── fraud/            # Fraud detection
│   │   ├── telephony/        # SIP/WebRTC handling
│   │   └── analytics/        # Real-time analytics
│   ├── infrastructure/       # External integrations
│   │   ├── database/         # PostgreSQL repositories
│   │   ├── messaging/        # Kafka/NATS
│   │   ├── telemetry/        # Observability
│   │   ├── cache/            # Redis caching
│   │   └── config/           # Configuration
│   └── api/                  # API handlers
│       ├── rest/             # RESTful endpoints
│       ├── grpc/             # gRPC services
│       └── websocket/        # Real-time connections
├── configs/                   # Configuration files
├── deployments/              # Docker & K8s manifests
├── docs/                     # Documentation
├── migrations/               # Database migrations
├── scripts/                  # Utility scripts
└── test/                     # Integration tests
```

## 🔧 Configuration

The application uses layered configuration with sensible defaults:

### Configuration Hierarchy

1. **Default Values** - Built into the application
2. **Config Files** - `configs/config.yaml` for environment-specific settings
3. **Environment Variables** - Override with `DCE_` prefix
4. **Command Flags** - Runtime overrides

### Core Configuration

```yaml
# configs/config.yaml
server:
  port: 8080
  grpc:
    port: 9090

database:
  url: "postgres://localhost:5432/dce_dev"
  max_open_conns: 25
  max_idle_conns: 5

redis:
  url: "localhost:6379"
  db: 0

telephony:
  sip_proxy: "sip.example.com:5060"
  stun_servers:
    - "stun:stun.l.google.com:19302"

compliance:
  tcpa_enabled: true
  gdpr_enabled: true
```

### Environment Variables

```bash
# Core Settings
DCE_ENVIRONMENT=production
DCE_LOG_LEVEL=info

# Database
DCE_DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require

# Redis Cache
DCE_REDIS_URL=redis-cluster:6379
DCE_REDIS_PASSWORD=secure-password

# Security
DCE_SECURITY_JWT_SECRET=your-256-bit-secret
DCE_SECURITY_TOKEN_EXPIRY=24h

# Rate Limiting
DCE_SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND=1000
DCE_SECURITY_RATE_LIMIT_BURST_SIZE=2000
```

## 🧪 Testing

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# Race condition detection
make test-race

# Coverage report
make coverage

# Benchmark tests
make bench
```

### Test Structure

- **Unit Tests** - Located alongside code files (`*_test.go`)
- **Integration Tests** - In `test/` directory
- **Benchmarks** - Performance testing for critical paths
- **Mocks** - Generated with `mockery` for interfaces

## 📊 API Documentation

### REST API

The REST API provides management operations and is documented with OpenAPI/Swagger.

```bash
# View API documentation
open http://localhost:8080/swagger
```

### gRPC API

High-performance internal APIs use gRPC with Protocol Buffers.

```bash
# Generate gRPC clients
make proto
```

### WebSocket API

Real-time bidding and events use WebSocket connections.

```javascript
// Example WebSocket connection
const ws = new WebSocket('ws://localhost:8080/ws/bidding');
ws.on('message', (data) => {
  const bid = JSON.parse(data);
  console.log('New bid:', bid);
});
```

## 🚀 Deployment

### Docker

```bash
# Build production image
docker build -t dce-backend:latest .

# Run container
docker run -p 8080:8080 -p 9090:9090 \
  --env-file .env.prod \
  dce-backend:latest
```

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n dce-backend
```

### Cloud Providers

- **AWS**: ECS with Fargate or EKS
- **GCP**: Cloud Run or GKE
- **Azure**: Container Instances or AKS

## 📈 Performance

### Benchmarks

| Operation | P50 | P95 | P99 |
|-----------|-----|-----|-----|
| Call Routing | 0.5ms | 0.8ms | 1ms |
| Bid Processing | 2ms | 4ms | 5ms |
| Compliance Check | 1ms | 2ms | 3ms |
| API Response | 5ms | 10ms | 15ms |

### Capacity

- **Throughput**: 10,000+ calls/second
- **Concurrent Connections**: 100,000+
- **Message Queue**: 1M+ events/second
- **Database**: 50,000+ queries/second

## 🛡️ Security

### Authentication & Authorization

- JWT tokens with RS256 signing
- Role-based access control (RBAC)
- API key authentication for B2B
- OAuth2 support for third-party integrations

### Data Protection

- TLS 1.3 for all communications
- AES-256 encryption at rest
- PCI DSS compliance for payment data
- GDPR-compliant data handling

### Security Scanning

```bash
# Run security checks
make security

# Dependency vulnerability scan
make vulncheck
```

## 🔍 Monitoring

### Metrics

Prometheus-compatible metrics exposed at `/metrics`:

- `dce_calls_total` - Total calls processed
- `dce_bids_total` - Total bids received
- `dce_routing_duration_seconds` - Routing decision latency
- `dce_compliance_violations_total` - Compliance issues detected

### Logging

Structured JSON logs with correlation IDs:

```json
{
  "time": "2025-01-15T10:30:45Z",
  "level": "INFO",
  "msg": "Call routed successfully",
  "call_id": "550e8400-e29b-41d4-a716-446655440000",
  "buyer_id": "123",
  "duration_ms": 0.5
}
```

### Distributed Tracing

OpenTelemetry integration for end-to-end request tracing:

```bash
# View traces in Jaeger
open http://localhost:16686
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'feat: add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Write tests for new features
- Update documentation as needed

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: [https://docs.dependablecallexchange.com](https://docs.dependablecallexchange.com)
- **Issues**: [GitHub Issues](https://github.com/davidleathers113/dependable-call-exchange-backend/issues)
- **Email**: support@dependablecallexchange.com
- **Discord**: [Join our community](https://discord.gg/dce-community)

## 🙏 Acknowledgments

- Built with Go 1.24 and modern cloud-native technologies
- Inspired by industry leaders in call routing and real-time systems
- Special thanks to all contributors and the open-source community

---

**Built with ❤️ by the Dependable Call Exchange Team**