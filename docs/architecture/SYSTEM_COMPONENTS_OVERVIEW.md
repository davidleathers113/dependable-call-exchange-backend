# System Components Overview

## Service Architecture Matrix

| Layer | Component | Technology | Purpose | Performance Target |
|-------|-----------|------------|---------|-------------------|
| **External** | Web Clients | React/Next.js | Buyer/Seller UI | < 100ms load |
| | Mobile Apps | React Native | iOS/Android apps | < 200ms response |
| | Admin Portal | Vue.js | System management | < 150ms response |
| **API Gateway** | REST API | Go + Gin | HTTP/JSON endpoints | < 10ms overhead |
| | gRPC Gateway | Go + gRPC | High-performance RPC | < 5ms overhead |
| | WebSocket | Go + Gorilla | Real-time updates | < 5ms latency |
| | Auth Service | JWT + OAuth2 | Authentication | < 20ms validation |
| **Core Services** | Call Routing | Go | Route decisions | < 1ms decision |
| | Bidding Engine | Go | Real-time auctions | < 100ms auction |
| | Fraud Detection | Go + Python ML | Security scoring | < 50ms scoring |
| | Telephony | Go + Pion | SIP/WebRTC | < 10ms setup |
| | Analytics | Go | Metrics processing | < 100ms aggregation |
| | Compliance | Go | Rule enforcement | < 50ms check |
| **Domain Layer** | Models | Go structs | Business entities | N/A |
| | Value Objects | Go types | Type safety | N/A |
| | Repositories | Go interfaces | Data access | N/A |
| **Data Storage** | PostgreSQL | v15 + TimescaleDB | Primary database | < 10ms queries |
| | Redis | v7 | Caching layer | < 1ms access |
| | Kafka | v3 | Event streaming | < 10ms publish |
| **Infrastructure** | Kubernetes | v1.28 | Container orchestration | 99.99% uptime |
| | Prometheus | v2.x | Metrics collection | 1s scrape interval |
| | Grafana | v10.x | Visualization | Real-time dashboards |
| | ELK Stack | v8.x | Log aggregation | < 1s indexing |

## Service Dependencies

| Service | Depends On | External APIs | Database Tables |
|---------|------------|---------------|-----------------|
| **Call Routing** | Fraud, Compliance, Analytics | None | calls, bids, accounts |
| **Bidding** | Account, Financial | None | bids, auctions, accounts |
| **Fraud Detection** | Analytics | ML Model API | accounts, calls, fraud_scores |
| **Telephony** | Call Routing | SIP Providers, WebRTC | calls, media_sessions |
| **Compliance** | None | DNC Registry, TCPA DB | compliance_rules, consent_records |
| **Analytics** | None | None | All tables (read-only) |

## API Endpoint Summary

| Endpoint | Method | Purpose | Auth Required | Rate Limit |
|----------|--------|---------|---------------|------------|
| `/api/v1/auth/login` | POST | User authentication | No | 10/min |
| `/api/v1/auth/refresh` | POST | Token refresh | Yes | 100/min |
| `/api/v1/accounts` | GET | List accounts | Yes (Admin) | 100/min |
| `/api/v1/accounts/{id}` | GET | Get account details | Yes | 1000/min |
| `/api/v1/calls` | POST | Initiate call | Yes | 100/min |
| `/api/v1/calls/{id}` | GET | Get call details | Yes | 1000/min |
| `/api/v1/bids` | POST | Place bid | Yes | 500/min |
| `/api/v1/auctions/{id}` | GET | Get auction status | Yes | 1000/min |
| `/api/v1/analytics/dashboard` | GET | Analytics dashboard | Yes | 100/min |
| `/ws/v1/events` | WS | Real-time events | Yes | N/A |

## Database Schema Summary

| Table | Type | Partitioning | Indexes | Size Estimate |
|-------|------|--------------|---------|---------------|
| `accounts` | Regular | By type | email, type, status | ~100K rows |
| `calls` | Hypertable | By time (weekly) | buyer_id, status, time | ~10M rows/month |
| `bids` | Regular | None | call_id, buyer_id, status | ~50M rows |
| `auctions` | Regular | By date | call_id, status, created | ~10M rows/month |
| `transactions` | Regular | By month | account_id, created | ~20M rows/month |
| `compliance_rules` | Regular | None | type, geographic | ~1K rows |
| `fraud_scores` | Regular | By date | account_id, call_id | ~10M rows/month |

## Environment Configuration

| Variable | Development | Staging | Production |
|----------|-------------|---------|------------|
| `DCE_ENVIRONMENT` | development | staging | production |
| `DCE_LOG_LEVEL` | debug | info | warn |
| `DCE_DATABASE_POOL_SIZE` | 5 | 15 | 25 |
| `DCE_REDIS_POOL_SIZE` | 10 | 20 | 50 |
| `DCE_RATE_LIMIT_RPS` | 100 | 500 | 1000 |
| `DCE_JWT_EXPIRY` | 24h | 12h | 6h |
| `DCE_AUCTION_TIMEOUT` | 500ms | 200ms | 100ms |
| `DCE_CALL_TIMEOUT` | 30m | 60m | 120m |

## Monitoring Metrics

| Metric | Type | Target | Alert Threshold |
|--------|------|--------|-----------------|
| `api_request_duration_ms` | Histogram | < 50ms p99 | > 100ms |
| `api_error_rate` | Counter | < 0.1% | > 1% |
| `call_routing_duration_ms` | Histogram | < 1ms p99 | > 5ms |
| `auction_success_rate` | Gauge | > 95% | < 90% |
| `database_connection_pool` | Gauge | < 80% | > 90% |
| `redis_hit_rate` | Gauge | > 90% | < 80% |
| `kafka_lag_ms` | Gauge | < 100ms | > 500ms |
| `revenue_per_hour` | Counter | > $1000 | < $500 |

## Security Controls

| Control | Implementation | Compliance |
|---------|----------------|------------|
| **Authentication** | JWT with refresh tokens | SOC2, ISO27001 |
| **Authorization** | RBAC with permissions | SOC2 |
| **Encryption in Transit** | TLS 1.3 | PCI-DSS, HIPAA |
| **Encryption at Rest** | AES-256 | PCI-DSS, GDPR |
| **API Rate Limiting** | Token bucket per user | DDoS protection |
| **Input Validation** | Go validator v10 | OWASP Top 10 |
| **SQL Injection Prevention** | Prepared statements | OWASP |
| **XSS Prevention** | Content Security Policy | OWASP |
| **Audit Logging** | Immutable event log | SOC2, GDPR |
| **Data Retention** | 90 days active, 7 years archive | GDPR |

## Deployment Targets

| Environment | Infrastructure | Regions | Scaling |
|-------------|----------------|---------|---------|
| **Development** | Docker Compose | Local | 1 instance each |
| **Staging** | Kubernetes (EKS) | us-east-1 | 2-4 pods each |
| **Production** | Kubernetes (EKS) | us-east-1, us-west-2, eu-west-1 | 10-50 pods each |
| **DR Site** | Kubernetes (EKS) | us-central-1 | Cold standby |

## SLA Commitments

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Uptime** | 99.99% | Monthly |
| **API Response Time** | < 50ms p99 | 5-minute window |
| **Call Routing Time** | < 1ms | Per request |
| **Data Durability** | 99.999999% | Annual |
| **RTO (Recovery Time)** | < 1 hour | Per incident |
| **RPO (Recovery Point)** | < 5 minutes | Per incident |
