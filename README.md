# Dependable Call Exchange Backend

A high-performance Pay Per Call exchange platform built with Go, following 2025 best practices for real-time bidding, call routing, and compliance management.

## 🏗️ Architecture

This project implements a **modular monolith** architecture designed for high-performance telephony operations with built-in compliance, fraud detection, and real-time bidding capabilities.

### Key Features

- **Real-time Call Routing** - Sub-millisecond call routing with intelligent load balancing
- **Live Bidding Engine** - Real-time auction system for call traffic
- **Compliance-First** - Built-in TCPA, GDPR, and DNC compliance
- **Fraud Detection** - ML-powered fraud prevention with 96%+ accuracy
- **Multi-Protocol Support** - REST, gRPC, and WebSocket APIs
- **Observability** - Comprehensive telemetry with OpenTelemetry integration

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Redis 7+
- Kafka (optional, for high-throughput deployments)

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd DependableCallExchangeBackEnd

# Install dependencies
go mod download

# Copy configuration
cp configs/config.yaml.example configs/config.yaml

# Set environment variables
export DCE_DATABASE_URL="postgres://localhost:5432/dce_dev?sslmode=disable"
export DCE_REDIS_URL="localhost:6379"

# Run the application
go run main.go
```

## 📁 Project Structure

```
├── internal/                   # Private packages (non-exportable)
│   ├── domain/                 # Domain entities & business rules
│   │   ├── call/              # Call lifecycle, routing rules
│   │   ├── bid/               # Bidding entities, auction logic
│   │   ├── account/           # User/buyer/seller management
│   │   ├── compliance/        # TCPA, GDPR, regulatory rules
│   │   └── financial/         # Payment, billing, transactions
│   ├── service/               # Business logic orchestration
│   │   ├── callrouting/       # Real-time call routing
│   │   ├── bidding/           # Auction engine
│   │   ├── fraud/             # ML-based fraud detection
│   │   ├── telephony/         # SIP/WebRTC integration
│   │   └── analytics/         # Real-time metrics
│   ├── infrastructure/        # External system integrations
│   │   ├── database/          # PostgreSQL repositories
│   │   ├── messaging/         # Kafka/NATS for events
│   │   ├── telemetry/         # OpenTelemetry, logging
│   │   ├── cache/             # Redis for sessions
│   │   └── config/            # Configuration management
│   └── api/                   # Protocol handlers
│       ├── rest/              # Management APIs
│       ├── grpc/              # High-performance inter-service
│       └── websocket/         # Real-time bidding interface
├── cmd/                       # Additional binaries
├── configs/                   # Configuration files
└── deployments/               # Docker, Kubernetes manifests
```

## 🔧 Configuration

Configuration uses a hybrid approach with [Koanf](https://github.com/knadh/koanf):

1. **Defaults** - Hard-coded in `internal/infrastructure/config/`
2. **Config Files** - YAML files in `configs/`
3. **Environment Variables** - Prefixed with `DCE_`
4. **Command-line Flags** - Runtime overrides

### Environment Variables

```bash
# Database
DCE_DATABASE_URL=postgres://localhost:5432/dce_dev?sslmode=disable

# Redis
DCE_REDIS_URL=localhost:6379
DCE_REDIS_PASSWORD=
DCE_REDIS_DB=0

# Security
DCE_SECURITY_JWT_SECRET=your-secret-key
DCE_SECURITY_TOKEN_EXPIRY=24h

# Compliance
DCE_COMPLIANCE_TCPA_ENABLED=true
DCE_COMPLIANCE_GDPR_ENABLED=true
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test ./internal/domain/call -v

# Benchmark tests
go test -bench=. ./internal/service/bidding
```

## 📊 Monitoring & Observability

The application includes comprehensive observability:

- **Structured Logging** - JSON logs with slog
- **Metrics** - Prometheus-compatible metrics
- **Tracing** - OpenTelemetry distributed tracing
- **Health Checks** - Kubernetes-ready endpoints

### Key Metrics

- Call routing latency (P50, P95, P99)
- Bid processing time
- Compliance check duration
- Fraud detection accuracy
- System resource utilization

## 🛡️ Security & Compliance

### Built-in Compliance Features

- **TCPA Compliance** - Automated time zone checking and consent validation
- **GDPR Compliance** - Data privacy controls and audit trails
- **DNC Integration** - Real-time Do Not Call list checking
- **Fraud Prevention** - ML-based risk scoring

### Security Features

- JWT-based authentication
- Rate limiting (token bucket algorithm)
- Input validation and sanitization
- SQL injection prevention
- Comprehensive audit logging

## 🔄 Development Workflow

1. **Create Feature Branch** - `git checkout -b feat/new-feature`
2. **Implement Changes** - Follow domain-driven design patterns
3. **Run Tests** - Ensure all tests pass
4. **Check Compliance** - Verify security and compliance requirements
5. **Create Pull Request** - Include comprehensive testing

### Code Quality Tools

```bash
# Linting
golangci-lint run

# Security scanning
gosec ./...

# Vulnerability checking
govulncheck ./...

# Format code
gofmt -w .
```

## 📈 Performance Benchmarks

Target performance metrics:

- **Call Routing**: < 1ms P99 latency
- **Bid Processing**: < 5ms average response time
- **Throughput**: 10,000+ calls per second
- **Availability**: 99.99% uptime SLA

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support, please contact:
- Email: support@dependablecallexchange.com
- Documentation: [docs.dependablecallexchange.com](https://docs.dependablecallexchange.com)
- Issues: [GitHub Issues](https://github.com/your-org/dependable-call-exchange-backend/issues)