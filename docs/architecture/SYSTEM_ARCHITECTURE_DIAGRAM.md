# System Architecture Diagram

## Overview
This diagram illustrates the complete system architecture of the Dependable Call Exchange Backend, showing how different components interact to provide a scalable pay-per-call marketplace.

## Architecture Diagram

```mermaid
graph TB
    %% External Users
    subgraph "External Users"
        Buyers[Buyers]
        Sellers[Sellers]
        Admins[Admins]
    end

    %% API Gateway
    subgraph "API Gateway"
        Gateway[API Gateway<br/>• REST API<br/>• gRPC<br/>• WebSocket<br/>• Auth/Rate Limit]
    end

    %% Core Services
    subgraph "Core Services"
        CallRouting[Call Routing<br/>• Route Decision<br/>• Load Balance]
        Bidding[Bidding<br/>• Auction Engine<br/>• Bid Matching]
        Fraud[Fraud Detection<br/>• ML Scoring<br/>• Pattern Analysis]
        Telephony[Telephony<br/>• SIP/WebRTC<br/>• Call Control]
        Analytics[Analytics<br/>• Real-time Metrics<br/>• Reporting]
        Compliance[Compliance<br/>• TCPA/GDPR<br/>• DNC Check]
    end

    %% Domain Layer
    subgraph "Domain Layer"
        Account[Account]
        Call[Call]
        Bid[Bid]
        ComplianceDomain[Compliance]
        Financial[Financial]
    end

    %% Infrastructure Layer
    subgraph "Infrastructure"
        PostgreSQL[PostgreSQL<br/>• TimescaleDB<br/>• Partitioning<br/>• Read Replicas<br/>• Connection Pool]
        Redis[Redis<br/>• Session Cache<br/>• Rate Limiting<br/>• Real-time Data]
        Kafka[Kafka<br/>• Event Stream<br/>• Audit Log<br/>• Analytics]
        Observability[Observability<br/>• OpenTelemetry<br/>• Prometheus<br/>• Grafana<br/>• ELK Stack]
    end

    %% Connections
    Buyers --> Gateway
    Sellers --> Gateway
    Admins --> Gateway
    
    Gateway --> CallRouting
    Gateway --> Bidding
    Gateway --> Fraud
    Gateway --> Telephony
    Gateway --> Analytics
    Gateway --> Compliance
    
    CallRouting --> Account
    CallRouting --> Call
    CallRouting --> Bid
    
    Bidding --> Account
    Bidding --> Bid
    Bidding --> Financial
    
    Fraud --> Account
    Fraud --> Call
    
    Compliance --> ComplianceDomain
    Compliance --> Call
    
    Analytics --> Call
    Analytics --> Bid
    Analytics --> Financial
    
    Telephony --> Call
    
    Account --> PostgreSQL
    Call --> PostgreSQL
    Bid --> PostgreSQL
    ComplianceDomain --> PostgreSQL
    Financial --> PostgreSQL
    
    Account --> Redis
    Call --> Redis
    Bid --> Redis
    
    Call --> Kafka
    Bid --> Kafka
    Financial --> Kafka
    
    CallRouting --> Observability
    Bidding --> Observability
    Fraud --> Observability
    Compliance --> Observability
    Analytics --> Observability
    Telephony --> Observability

    %% Styling
    classDef userStyle fill:#4A90E2,stroke:#2E5C8A,color:#fff
    classDef apiStyle fill:#7ED321,stroke:#5A9E18,color:#fff
    classDef serviceStyle fill:#FF9500,stroke:#CC7700,color:#fff
    classDef domainStyle fill:#E6F3FF,stroke:#4A90E2,color:#2E5C8A
    classDef infraStyle fill:#50C878,stroke:#3A9B5C,color:#fff

    class Buyers,Sellers,Admins userStyle
    class Gateway apiStyle
    class CallRouting,Bidding,Fraud,Telephony,Analytics,Compliance serviceStyle
    class Account,Call,Bid,ComplianceDomain,Financial domainStyle
    class PostgreSQL,Redis,Kafka,Observability infraStyle
```

## Component Descriptions

### External Users
- **Buyers**: Businesses that pay for incoming calls
- **Sellers**: Lead generators and affiliates who route calls
- **Admins**: System administrators managing the platform

### API Gateway Layer
Single entry point for all external requests:
- **REST API**: Standard HTTP/JSON endpoints
- **gRPC**: High-performance RPC for internal services
- **WebSocket**: Real-time bidirectional communication
- **Auth/Rate Limit**: Security and traffic management

### Core Services

#### Call Routing Service
- Makes intelligent routing decisions in < 1ms
- Implements multiple routing algorithms
- Handles load balancing across buyers

#### Bidding Service
- Executes real-time auctions
- Matches calls with bid profiles
- Processes 100K+ bids per second

#### Fraud Detection Service
- ML-powered fraud scoring
- Real-time pattern analysis
- Behavioral anomaly detection

#### Telephony Service
- SIP/WebRTC protocol handling
- Call state management
- Media streaming control

#### Analytics Service
- Real-time metrics collection
- Business intelligence reporting
- Performance monitoring

#### Compliance Service
- TCPA time-based restrictions
- GDPR data protection
- DNC list checking

### Domain Layer
Core business entities:
- **Account**: User accounts (buyers, sellers, admins)
- **Call**: Call records and state
- **Bid**: Auction bids and profiles
- **Compliance**: Rules and violations
- **Financial**: Transactions and billing

### Infrastructure Layer

#### PostgreSQL
- Primary data store
- TimescaleDB for time-series data
- Table partitioning for scale
- Read replicas for analytics
- Connection pooling with PgBouncer

#### Redis
- Session caching
- Rate limiting counters
- Real-time data storage
- Pub/sub for events

#### Kafka
- Event streaming platform
- Audit log persistence
- Analytics data pipeline
- Asynchronous processing

#### Observability Stack
- **OpenTelemetry**: Distributed tracing
- **Prometheus**: Metrics collection
- **Grafana**: Visualization dashboards
- **ELK Stack**: Log aggregation and search

## Data Flow

1. **Request Flow**: Users → API Gateway → Services
2. **Business Logic**: Services → Domain Layer
3. **Persistence**: Domain → Infrastructure
4. **Events**: All layers → Kafka → Analytics/Audit
5. **Monitoring**: All components → Observability Stack

## Scalability Features

- Stateless services for horizontal scaling
- Database sharding by account/region
- Multi-level caching strategy
- Event-driven architecture for decoupling
- Read replicas for analytics workloads
