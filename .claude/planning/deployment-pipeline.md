# DCE Deployment Pipeline - Parallel Development Strategy

**Version**: 1.0  
**Created**: January 12, 2025  
**Purpose**: CI/CD pipeline for coordinated 5-team parallel development  
**Target**: Zero-downtime deployments with feature flag control

---

## 🎯 Pipeline Overview

### Strategic Goals
- **Parallel Development**: 5 teams deploying independently without conflicts
- **Feature Isolation**: Feature flags enable safe progressive rollouts
- **Zero Downtime**: Blue-green deployments with automatic rollback
- **Quality Gates**: Automated testing and compliance validation
- **Observability**: Real-time monitoring and alerting

### Pipeline Architecture
```
┌─────────────────────────────────────────────────────────────────┐
│                    SOURCE CODE MANAGEMENT                      │
├─────────┬─────────┬─────────┬─────────┬─────────┬─────────────────┤
│Security │Compliance│Infra   │Financial│Integration│   Main Branch  │
│Team     │Team     │Team    │Team     │Team     │                 │
└─────────┴─────────┴─────────┴─────────┴─────────┴─────────────────┘
         │         │         │         │         │
         ▼         ▼         ▼         ▼         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    BUILD & TEST PIPELINE                       │
│  Unit Tests → Integration Tests → Security Scan → Performance   │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    STAGING DEPLOYMENT                          │
│     Feature Flags → Integration Testing → Performance Testing   │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                  PRODUCTION DEPLOYMENT                         │
│   Canary (5%) → Rolling (25%→50%→100%) → Monitoring → Rollback │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🌿 Git Branching Strategy

### Branch Structure
```
main
├── team/security-main
│   ├── feat/security-auth-middleware
│   ├── feat/security-jwt-service
│   └── feat/security-rbac
├── team/compliance-main
│   ├── feat/compliance-tcpa-validation
│   ├── feat/compliance-consent-mgmt
│   └── feat/compliance-dnc-integration
├── team/infrastructure-main
│   ├── feat/infrastructure-domain-events
│   ├── feat/infrastructure-kafka
│   └── feat/infrastructure-caching
├── team/financial-main
│   ├── feat/financial-billing-service
│   ├── feat/financial-payments
│   └── feat/financial-transactions
└── team/integration-main
    ├── feat/integration-websocket
    ├── feat/integration-api-completion
    └── feat/integration-webhooks
```

### Branch Protection Rules
```yaml
# .github/branch-protection.yml
main:
  required_status_checks:
    - ci/build
    - ci/test-unit
    - ci/test-integration
    - ci/security-scan
    - ci/performance-test
  required_reviews: 2
  required_reviewers_from_teams: ["tech-leads"]
  dismiss_stale_reviews: true
  require_up_to_date_branch: true

team/*-main:
  required_status_checks:
    - ci/build
    - ci/test-unit
    - ci/team-integration
  required_reviews: 1
  required_reviewers_from_teams: ["team-leads"]

feat/*:
  required_status_checks:
    - ci/build
    - ci/test-unit
  required_reviews: 1
```

### Merge Strategy
1. **Feature → Team Branch**: Squash merge with descriptive commit
2. **Team Branch → Main**: Merge commit for traceability
3. **Hotfix**: Direct to main with fast-track approval

---

## 🚀 CI/CD Pipeline Configuration

### GitHub Actions Workflow

#### Multi-Team Pipeline
```yaml
# .github/workflows/multi-team-ci.yml
name: DCE Multi-Team CI/CD

on:
  push:
    branches: [main, 'team/*', 'feat/*']
  pull_request:
    branches: [main, 'team/*']

env:
  GO_VERSION: '1.24'
  REGISTRY: ghcr.io
  IMAGE_NAME: dce-backend

jobs:
  detect-changes:
    runs-on: ubuntu-latest
    outputs:
      security: ${{ steps.changes.outputs.security }}
      compliance: ${{ steps.changes.outputs.compliance }}
      infrastructure: ${{ steps.changes.outputs.infrastructure }}
      financial: ${{ steps.changes.outputs.financial }}
      integration: ${{ steps.changes.outputs.integration }}
      shared: ${{ steps.changes.outputs.shared }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v2
        id: changes
        with:
          filters: |
            security:
              - 'internal/service/auth/**'
              - 'internal/api/rest/auth/**'
              - 'internal/infrastructure/security/**'
            compliance:
              - 'internal/domain/compliance/**'
              - 'internal/service/compliance/**'
              - 'internal/api/rest/compliance/**'
            infrastructure:
              - 'internal/infrastructure/**'
              - 'internal/domain/events/**'
              - 'configs/**'
            financial:
              - 'internal/domain/financial/**'
              - 'internal/service/financial/**'
              - 'internal/api/rest/financial/**'
            integration:
              - 'internal/api/**'
              - 'internal/service/webhook/**'
              - 'cmd/api/**'
            shared:
              - 'internal/domain/values/**'
              - 'internal/testutil/**'
              - 'go.mod'
              - 'go.sum'

  security-team:
    needs: detect-changes
    if: needs.detect-changes.outputs.security == 'true' || needs.detect-changes.outputs.shared == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Security Team Tests
        run: |
          make test-security
          make security-scan
          make test-auth-integration
      
      - name: Build Security Services
        run: |
          make build-auth-service
          make build-security-middleware

  compliance-team:
    needs: detect-changes
    if: needs.detect-changes.outputs.compliance == 'true' || needs.detect-changes.outputs.shared == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Compliance Team Tests
        run: |
          make test-compliance
          make test-tcpa-validation
          make test-dnc-integration
      
      - name: Compliance Validation
        run: |
          make validate-tcpa-rules
          make validate-gdpr-compliance

  infrastructure-team:
    needs: detect-changes
    if: needs.detect-changes.outputs.infrastructure == 'true' || needs.detect-changes.outputs.shared == 'true'
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      kafka:
        image: confluentinc/cp-kafka:latest
        env:
          KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
          KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Infrastructure Tests
        run: |
          make test-infrastructure
          make test-event-store
          make test-kafka-integration
      
      - name: Performance Tests
        run: |
          make test-performance-infrastructure
          make bench-event-processing

  financial-team:
    needs: detect-changes
    if: needs.detect-changes.outputs.financial == 'true' || needs.detect-changes.outputs.shared == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Financial Team Tests
        run: |
          make test-financial
          make test-billing-accuracy
          make test-payment-integration
      
      - name: Financial Compliance
        run: |
          make validate-financial-compliance
          make test-transaction-accuracy

  integration-team:
    needs: detect-changes
    if: needs.detect-changes.outputs.integration == 'true' || needs.detect-changes.outputs.shared == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Integration Tests
        run: |
          make test-integration
          make test-api-contract
          make test-websocket
      
      - name: API Performance Tests
        run: |
          make test-api-performance
          make test-websocket-load

  cross-team-integration:
    needs: [security-team, compliance-team, infrastructure-team, financial-team, integration-team]
    if: always() && (needs.security-team.result == 'success' || needs.security-team.result == 'skipped') && (needs.compliance-team.result == 'success' || needs.compliance-team.result == 'skipped') && (needs.infrastructure-team.result == 'success' || needs.infrastructure-team.result == 'skipped') && (needs.financial-team.result == 'success' || needs.financial-team.result == 'skipped') && (needs.integration-team.result == 'success' || needs.integration-team.result == 'skipped')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Cross-Team Integration Tests
        run: |
          make test-e2e-auth-compliance
          make test-e2e-financial-compliance
          make test-e2e-full-call-flow
      
      - name: System Performance Tests
        run: |
          make test-system-performance
          make test-load-full-system

  build-and-push:
    needs: cross-team-integration
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    outputs:
      image-digest: ${{ steps.build.outputs.digest }}
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=sha,prefix=main-
      
      - name: Build and push Docker image
        id: build
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  deploy-staging:
    needs: build-and-push
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4
      
      - name: Deploy to Staging
        run: |
          # Deploy with feature flags enabled
          ./scripts/deploy-staging.sh ${{ needs.build-and-push.outputs.image-digest }}
      
      - name: Run Staging Tests
        run: |
          make test-staging-environment
          make test-feature-flags
          make test-staging-performance

  deploy-production:
    needs: [build-and-push, deploy-staging]
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
      
      - name: Production Deployment
        run: |
          ./scripts/deploy-production.sh ${{ needs.build-and-push.outputs.image-digest }}
```

### Team-Specific Makefile Targets
```makefile
# Makefile additions for team-specific testing

# Security Team Targets
test-security:
	go test -v ./internal/service/auth/... ./internal/api/rest/auth/...

test-auth-integration:
	go test -v -tags=integration ./test/integration/auth/...

security-scan:
	gosec ./internal/service/auth/...
	govulncheck ./internal/service/auth/...

# Compliance Team Targets
test-compliance:
	go test -v ./internal/domain/compliance/... ./internal/service/compliance/...

test-tcpa-validation:
	go test -v ./internal/domain/compliance/tcpa/...

validate-tcpa-rules:
	go run ./cmd/compliance-validator/main.go --rules=tcpa

# Infrastructure Team Targets
test-infrastructure:
	go test -v ./internal/infrastructure/...

test-event-store:
	go test -v -tags=integration ./internal/infrastructure/events/...

test-kafka-integration:
	go test -v -tags=integration ./internal/infrastructure/messaging/...

bench-event-processing:
	go test -bench=BenchmarkEventProcessing ./internal/infrastructure/events/...

# Financial Team Targets
test-financial:
	go test -v ./internal/domain/financial/... ./internal/service/financial/...

test-billing-accuracy:
	go test -v ./internal/service/financial/billing/... -run TestBillingAccuracy

validate-financial-compliance:
	go run ./cmd/financial-validator/main.go

# Integration Team Targets
test-integration:
	go test -v ./internal/api/... ./cmd/api/...

test-api-contract:
	go test -v -tags=contract ./test/contract/...

test-websocket:
	go test -v ./internal/api/websocket/...

test-api-performance:
	go test -bench=BenchmarkAPI ./internal/api/rest/...

# Cross-Team Integration Targets
test-e2e-auth-compliance:
	go test -v -tags=e2e ./test/e2e/auth_compliance_test.go

test-e2e-financial-compliance:
	go test -v -tags=e2e ./test/e2e/financial_compliance_test.go

test-e2e-full-call-flow:
	go test -v -tags=e2e ./test/e2e/call_flow_test.go

test-system-performance:
	go test -bench=BenchmarkSystem -tags=e2e ./test/performance/...
```

---

## 🏴‍☠️ Feature Flag Strategy

### Feature Flag Management

#### LaunchDarkly Configuration
```yaml
# feature-flags.yml
feature_flags:
  security:
    jwt_authentication:
      enabled: true
      rollout_percentage: 100
      environments: [staging, production]
    
    rbac_authorization:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [security]
    
    rate_limiting:
      enabled: true
      rollout_percentage: 50
      environments: [production]

  compliance:
    tcpa_validation:
      enabled: true
      rollout_percentage: 100
      environments: [staging, production]
    
    advanced_consent_mgmt:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [compliance]
    
    dnc_integration:
      enabled: true
      rollout_percentage: 25
      environments: [production]

  infrastructure:
    domain_events:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [infrastructure]
    
    kafka_messaging:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [infrastructure]

  financial:
    billing_service:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [financial]
    
    payment_processing:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [financial]

  integration:
    websocket_realtime:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [integration]
    
    webhook_platform:
      enabled: false
      rollout_percentage: 0
      environments: [staging]
      teams: [integration]
```

#### Feature Flag Implementation
```go
// internal/infrastructure/featureflags/client.go
package featureflags

import (
    "context"
    "log"
    
    "github.com/launchdarkly/go-server-sdk/v6"
    "github.com/launchdarkly/go-server-sdk/v6/ldcomponents"
)

type Client struct {
    ldClient *ldclient.LDClient
}

func NewClient(sdkKey string) (*Client, error) {
    config := ldclient.Config{
        Events: ldcomponents.SendEvents(),
    }
    
    client, err := ldclient.MakeCustomClient(sdkKey, config, 5*time.Second)
    if err != nil {
        return nil, err
    }
    
    return &Client{ldClient: client}, nil
}

func (c *Client) IsEnabled(ctx context.Context, flagKey string, userContext ldcontext.Context) bool {
    enabled, err := c.ldClient.BoolVariation(flagKey, userContext, false)
    if err != nil {
        log.Printf("Error evaluating feature flag %s: %v", flagKey, err)
        return false
    }
    return enabled
}

// Team-specific flag methods
func (c *Client) IsSecurityFeatureEnabled(ctx context.Context, feature string, user ldcontext.Context) bool {
    return c.IsEnabled(ctx, fmt.Sprintf("security_%s", feature), user)
}

func (c *Client) IsComplianceFeatureEnabled(ctx context.Context, feature string, user ldcontext.Context) bool {
    return c.IsEnabled(ctx, fmt.Sprintf("compliance_%s", feature), user)
}
```

#### Usage in Services
```go
// internal/service/auth/service.go
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*User, error) {
    // Check if JWT authentication is enabled
    if !s.featureFlags.IsSecurityFeatureEnabled(ctx, "jwt_authentication", s.getUserContext(ctx)) {
        return s.legacyValidateToken(ctx, token)
    }
    
    return s.jwtValidateToken(ctx, token)
}
```

### Progressive Rollout Strategy

#### Rollout Phases
1. **Development**: Feature flags off, manual testing
2. **Team Testing**: Feature flags on for team members only
3. **Staging**: Feature flags on for staging environment
4. **Canary (5%)**: Limited production exposure
5. **Rolling (25%→50%→100%)**: Gradual production rollout
6. **Full Release**: Feature flag removal

#### Rollout Automation
```bash
#!/bin/bash
# scripts/progressive-rollout.sh

FEATURE_FLAG=$1
ENVIRONMENT=$2
TARGET_PERCENTAGE=$3

echo "Rolling out $FEATURE_FLAG to $TARGET_PERCENTAGE% in $ENVIRONMENT"

# Update feature flag percentage
curl -X PATCH "https://app.launchdarkly.com/api/v2/flags/default/$FEATURE_FLAG" \
  -H "Authorization: $LAUNCHDARKLY_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"patch\": [
      {
        \"op\": \"replace\",
        \"path\": \"/environments/$ENVIRONMENT/rules/0/rollout/variations/0/weight\",
        \"value\": $TARGET_PERCENTAGE
      }
    ]
  }"

# Monitor metrics for 30 minutes
echo "Monitoring metrics for 30 minutes..."
./scripts/monitor-rollout.sh $FEATURE_FLAG $ENVIRONMENT $TARGET_PERCENTAGE

# Auto-rollback if error rate > 1%
ERROR_RATE=$(./scripts/get-error-rate.sh $FEATURE_FLAG $ENVIRONMENT)
if (( $(echo "$ERROR_RATE > 1.0" | bc -l) )); then
    echo "Error rate too high ($ERROR_RATE%), rolling back..."
    ./scripts/rollback-feature.sh $FEATURE_FLAG $ENVIRONMENT
fi
```

---

## 🧪 Testing Pipeline Coordination

### Test Categories

#### Unit Tests (Individual Teams)
```yaml
unit_testing:
  security_team:
    - auth_service_test.go
    - jwt_validator_test.go
    - rbac_engine_test.go
  
  compliance_team:
    - tcpa_validator_test.go
    - consent_manager_test.go
    - dnc_checker_test.go
  
  infrastructure_team:
    - event_store_test.go
    - kafka_producer_test.go
    - cache_manager_test.go
  
  financial_team:
    - billing_service_test.go
    - payment_processor_test.go
    - transaction_manager_test.go
  
  integration_team:
    - api_handler_test.go
    - websocket_manager_test.go
    - webhook_dispatcher_test.go
```

#### Integration Tests (Cross-Team)
```yaml
integration_testing:
  auth_compliance:
    description: "Test authenticated users can access compliance APIs"
    teams: [security, compliance]
    test_file: test/integration/auth_compliance_test.go
  
  financial_compliance:
    description: "Test billing services respect compliance rules"
    teams: [financial, compliance]
    test_file: test/integration/financial_compliance_test.go
  
  event_infrastructure:
    description: "Test domain events flow through infrastructure"
    teams: [infrastructure, all_domains]
    test_file: test/integration/event_flow_test.go
  
  api_realtime:
    description: "Test REST APIs trigger WebSocket events"
    teams: [integration, infrastructure]
    test_file: test/integration/api_realtime_test.go
```

#### End-to-End Tests (Full System)
```yaml
e2e_testing:
  complete_call_flow:
    description: "Test complete call exchange flow with all features"
    duration: 15 minutes
    test_file: test/e2e/call_flow_test.go
    
  compliance_enforcement:
    description: "Test system prevents violations across all components"
    duration: 10 minutes
    test_file: test/e2e/compliance_enforcement_test.go
    
  performance_benchmarks:
    description: "Test system meets performance requirements"
    duration: 30 minutes
    test_file: test/e2e/performance_test.go
```

### Parallel Testing Strategy

#### Test Execution Matrix
```bash
# Parallel test execution for CI
parallel -j 5 ::: \
  "make test-security" \
  "make test-compliance" \
  "make test-infrastructure" \
  "make test-financial" \
  "make test-integration"

# Wait for all unit tests to complete
wait

# Run integration tests
parallel -j 3 ::: \
  "make test-auth-compliance-integration" \
  "make test-financial-compliance-integration" \
  "make test-event-infrastructure-integration"

# Wait for integration tests
wait

# Run E2E tests sequentially (require full system)
make test-e2e-complete-flow
make test-e2e-compliance-enforcement
make test-e2e-performance
```

### Test Data Management

#### Shared Test Fixtures
```go
// test/fixtures/shared.go
package fixtures

type TestDataSet struct {
    Users       []User
    Accounts    []Account
    Calls       []Call
    Bids        []Bid
}

func NewCompleteTestDataSet() *TestDataSet {
    return &TestDataSet{
        Users: []User{
            NewUser().WithRole("buyer").Build(),
            NewUser().WithRole("seller").Build(),
            NewUser().WithRole("admin").Build(),
        },
        Accounts: []Account{
            NewAccount().WithCompliance(true).Build(),
            NewAccount().WithCompliance(false).Build(),
        },
        // ... more test data
    }
}
```

#### Database State Management
```bash
# scripts/reset-test-db.sh
#!/bin/bash

echo "Resetting test database state..."

# Drop and recreate test database
psql -h localhost -p 5433 -U dce_test << EOF
DROP DATABASE IF EXISTS dce_test_parallel;
CREATE DATABASE dce_test_parallel;
EOF

# Run migrations
./cmd/migrate/migrate -database="postgres://localhost:5433/dce_test_parallel" -up

# Seed with test data
./scripts/seed-test-data.sh

echo "Test database reset complete"
```

---

## 🏭 Production Deployment Strategy

### Environment Architecture

#### Staging Environment
```yaml
# deployments/staging/kustomization.yml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: dce-staging

resources:
  - ../base
  - ingress-staging.yml
  - postgres-staging.yml
  - redis-staging.yml
  - kafka-staging.yml

patchesStrategicMerge:
  - deployment-staging.yml
  - configmap-staging.yml

configMapGenerator:
  - name: dce-config
    files:
      - config.staging.yml

images:
  - name: dce-backend
    newTag: staging-latest
```

#### Production Environment
```yaml
# deployments/production/kustomization.yml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: dce-production

resources:
  - ../base
  - ingress-production.yml
  - postgres-production.yml
  - redis-cluster-production.yml
  - kafka-cluster-production.yml
  - monitoring-production.yml

patchesStrategicMerge:
  - deployment-production.yml
  - configmap-production.yml
  - hpa-production.yml

configMapGenerator:
  - name: dce-config
    files:
      - config.production.yml

images:
  - name: dce-backend
    newTag: latest

replicas:
  - name: dce-api
    count: 5
```

### Blue-Green Deployment

#### Deployment Script
```bash
#!/bin/bash
# scripts/deploy-production.sh

IMAGE_DIGEST=$1
ENVIRONMENT=${2:-production}

echo "Starting blue-green deployment for $IMAGE_DIGEST"

# Determine current active slot
CURRENT_SLOT=$(kubectl get service dce-api-active -o jsonpath='{.spec.selector.slot}')
NEW_SLOT=$([ "$CURRENT_SLOT" = "blue" ] && echo "green" || echo "blue")

echo "Current active slot: $CURRENT_SLOT"
echo "Deploying to slot: $NEW_SLOT"

# Deploy to inactive slot
kubectl set image deployment/dce-api-$NEW_SLOT dce-backend=$IMAGE_DIGEST
kubectl rollout status deployment/dce-api-$NEW_SLOT --timeout=600s

# Wait for health checks
echo "Waiting for health checks..."
sleep 30

# Test new deployment
./scripts/health-check.sh dce-api-$NEW_SLOT
if [ $? -ne 0 ]; then
    echo "Health check failed, aborting deployment"
    exit 1
fi

# Run smoke tests
./scripts/smoke-tests.sh dce-api-$NEW_SLOT
if [ $? -ne 0 ]; then
    echo "Smoke tests failed, aborting deployment"
    exit 1
fi

# Switch traffic to new slot
echo "Switching traffic to $NEW_SLOT slot"
kubectl patch service dce-api-active -p '{"spec":{"selector":{"slot":"'$NEW_SLOT'"}}}'

# Monitor for 5 minutes
echo "Monitoring new deployment for 5 minutes..."
./scripts/monitor-deployment.sh 300

# Check error rates and performance
ERROR_RATE=$(./scripts/get-error-rate.sh)
LATENCY_P99=$(./scripts/get-latency.sh p99)

if (( $(echo "$ERROR_RATE > 1.0" | bc -l) )) || (( $(echo "$LATENCY_P99 > 100" | bc -l) )); then
    echo "Performance degradation detected, rolling back..."
    kubectl patch service dce-api-active -p '{"spec":{"selector":{"slot":"'$CURRENT_SLOT'"}}}'
    exit 1
fi

# Deployment successful
echo "Deployment successful, scaling down old slot"
kubectl scale deployment dce-api-$CURRENT_SLOT --replicas=1

echo "Blue-green deployment completed successfully"
```

### Canary Deployment

#### Canary Strategy
```yaml
# deployments/canary/canary-rollout.yml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: dce-api-canary
spec:
  replicas: 10
  strategy:
    canary:
      steps:
      - setWeight: 5    # 5% traffic
      - pause: {duration: 300s}  # 5 minutes
      - setWeight: 25   # 25% traffic
      - pause: {duration: 600s}  # 10 minutes
      - setWeight: 50   # 50% traffic
      - pause: {duration: 600s}  # 10 minutes
      - setWeight: 100  # 100% traffic
      
      analysis:
        templates:
        - templateName: success-rate
        - templateName: latency-p99
        startingStep: 1
        args:
        - name: service-name
          value: dce-api
          
      trafficRouting:
        nginx:
          stableService: dce-api-stable
          canaryService: dce-api-canary
          annotationPrefix: nginx.ingress.kubernetes.io
          
  selector:
    matchLabels:
      app: dce-api
  template:
    metadata:
      labels:
        app: dce-api
    spec:
      containers:
      - name: dce-backend
        image: ghcr.io/dce-backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: DCE_ENVIRONMENT
          value: "production"
        - name: DCE_FEATURE_FLAGS_ENABLED
          value: "true"
```

#### Automated Rollback
```yaml
# deployments/canary/analysis-template.yml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: success-rate
spec:
  metrics:
  - name: success-rate
    interval: 60s
    count: 5
    successCondition: result[0] >= 99.5
    failureLimit: 2
    provider:
      prometheus:
        address: http://prometheus:9090
        query: |
          sum(rate(http_requests_total{service="dce-api",code!~"5.."}[5m])) /
          sum(rate(http_requests_total{service="dce-api"}[5m])) * 100
  
  - name: latency-p99
    interval: 60s
    count: 5
    successCondition: result[0] <= 100
    failureLimit: 2
    provider:
      prometheus:
        address: http://prometheus:9090
        query: |
          histogram_quantile(0.99,
            sum(rate(http_request_duration_seconds_bucket{service="dce-api"}[5m])) by (le)
          ) * 1000
```

---

## 📊 Monitoring & Observability

### Real-Time Monitoring Dashboard

#### Deployment Health Dashboard
```yaml
# monitoring/grafana-deployment.json
{
  "dashboard": {
    "title": "DCE Deployment Pipeline Health",
    "panels": [
      {
        "title": "Build Pipeline Status",
        "type": "stat",
        "targets": [
          {
            "expr": "github_workflow_runs_total{repository=\"dce-backend\",status=\"success\"} / github_workflow_runs_total{repository=\"dce-backend\"} * 100"
          }
        ]
      },
      {
        "title": "Team Build Success Rate",
        "type": "bargauge",
        "targets": [
          {
            "expr": "github_workflow_runs_total{repository=\"dce-backend\",job=~\".*-team\",status=\"success\"} by (job)"
          }
        ]
      },
      {
        "title": "Deployment Frequency",
        "type": "graph",
        "targets": [
          {
            "expr": "increase(deployments_total{environment=\"production\"}[1d])"
          }
        ]
      },
      {
        "title": "Feature Flag Rollout Status",
        "type": "table",
        "targets": [
          {
            "expr": "launchdarkly_flag_rollout_percentage by (flag_key, environment)"
          }
        ]
      }
    ]
  }
}
```

#### Application Performance Monitoring
```yaml
# monitoring/application-metrics.yml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-rules
data:
  rules.yml: |
    groups:
    - name: dce-deployment
      rules:
      - alert: DeploymentFailed
        expr: increase(deployment_failures_total[10m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "DCE deployment failed"
          description: "Deployment to {{ $labels.environment }} failed"
      
      - alert: CanaryRollbackTriggered
        expr: increase(canary_rollbacks_total[5m]) > 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "Canary deployment rolled back"
          description: "Canary deployment rolled back due to performance issues"
      
      - alert: FeatureFlagToggled
        expr: changes(launchdarkly_flag_rollout_percentage[5m]) > 0
        for: 0m
        labels:
          severity: info
        annotations:
          summary: "Feature flag modified"
          description: "Feature flag {{ $labels.flag_key }} changed to {{ $value }}%"
```

### Performance Monitoring

#### Deployment Performance Metrics
```go
// internal/infrastructure/metrics/deployment.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    DeploymentDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "dce_deployment_duration_seconds",
            Help: "Time spent on deployments",
            Buckets: prometheus.LinearBuckets(30, 30, 10), // 30s to 5min
        },
        []string{"team", "environment", "status"},
    )
    
    FeatureFlagToggle = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dce_feature_flag_toggles_total",
            Help: "Number of feature flag toggles",
        },
        []string{"flag_key", "environment", "action"},
    )
    
    TeamBuildSuccess = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dce_team_build_success_total",
            Help: "Number of successful team builds",
        },
        []string{"team", "branch"},
    )
)
```

---

## 🔄 Rollback Procedures

### Automated Rollback Triggers

#### Performance-Based Rollback
```bash
#!/bin/bash
# scripts/automated-rollback.sh

THRESHOLD_ERROR_RATE=1.0
THRESHOLD_LATENCY_P99=100
MONITORING_DURATION=300  # 5 minutes

echo "Starting automated monitoring for rollback triggers..."

for i in $(seq 1 $MONITORING_DURATION); do
    ERROR_RATE=$(./scripts/get-error-rate.sh)
    LATENCY_P99=$(./scripts/get-latency.sh p99)
    
    echo "Minute $i: Error Rate: $ERROR_RATE%, Latency P99: ${LATENCY_P99}ms"
    
    # Check error rate
    if (( $(echo "$ERROR_RATE > $THRESHOLD_ERROR_RATE" | bc -l) )); then
        echo "ERROR RATE THRESHOLD EXCEEDED: $ERROR_RATE% > $THRESHOLD_ERROR_RATE%"
        ./scripts/emergency-rollback.sh "high_error_rate"
        exit 1
    fi
    
    # Check latency
    if (( $(echo "$LATENCY_P99 > $THRESHOLD_LATENCY_P99" | bc -l) )); then
        echo "LATENCY THRESHOLD EXCEEDED: ${LATENCY_P99}ms > ${THRESHOLD_LATENCY_P99}ms"
        ./scripts/emergency-rollback.sh "high_latency"
        exit 1
    fi
    
    sleep 60
done

echo "Monitoring completed successfully - no rollback needed"
```

#### Manual Rollback Process
```bash
#!/bin/bash
# scripts/manual-rollback.sh

ENVIRONMENT=${1:-production}
REASON=${2:-manual_request}

echo "Initiating manual rollback for $ENVIRONMENT environment"
echo "Reason: $REASON"

# Get current and previous deployments
CURRENT_DEPLOYMENT=$(kubectl get deployment dce-api -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')
PREVIOUS_DEPLOYMENT=$((CURRENT_DEPLOYMENT - 1))

echo "Rolling back from revision $CURRENT_DEPLOYMENT to $PREVIOUS_DEPLOYMENT"

# Perform rollback
kubectl rollout undo deployment/dce-api --to-revision=$PREVIOUS_DEPLOYMENT

# Wait for rollback to complete
kubectl rollout status deployment/dce-api --timeout=300s

# Verify rollback success
./scripts/health-check.sh
if [ $? -eq 0 ]; then
    echo "Rollback completed successfully"
    
    # Notify teams
    ./scripts/notify-rollback.sh "$ENVIRONMENT" "$REASON" "success"
else
    echo "Rollback health check failed"
    ./scripts/notify-rollback.sh "$ENVIRONMENT" "$REASON" "failed"
    exit 1
fi
```

### Emergency Procedures

#### Circuit Breaker Activation
```go
// internal/infrastructure/circuit/breaker.go
package circuit

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type CircuitBreaker struct {
    state         State
    failureCount  int
    successCount  int
    threshold     int
    timeout       time.Duration
    lastFailTime  time.Time
    mu           sync.RWMutex
}

type State int

const (
    Closed State = iota
    Open
    HalfOpen
)

func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if cb.state == Open {
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.state = HalfOpen
            cb.successCount = 0
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }
    
    err := fn()
    
    if err != nil {
        cb.onFailure()
        return err
    }
    
    cb.onSuccess()
    return nil
}

func (cb *CircuitBreaker) onFailure() {
    cb.failureCount++
    cb.lastFailTime = time.Now()
    
    if cb.failureCount >= cb.threshold {
        cb.state = Open
        // Trigger emergency rollback
        go cb.triggerEmergencyRollback()
    }
}

func (cb *CircuitBreaker) triggerEmergencyRollback() {
    // Execute emergency rollback script
    exec.Command("./scripts/emergency-rollback.sh", "circuit_breaker_opened").Run()
}
```

---

## 📋 Deployment Checklist

### Pre-Deployment Checklist
- [ ] All team unit tests passing
- [ ] Cross-team integration tests passing
- [ ] Security scan completed with no critical issues
- [ ] Performance tests meet SLA requirements
- [ ] Feature flags configured correctly
- [ ] Database migrations reviewed and tested
- [ ] Monitoring and alerting configured
- [ ] Rollback plan documented and tested
- [ ] Team leads approve deployment
- [ ] Stakeholders notified of deployment window

### During Deployment Checklist
- [ ] Monitor deployment progress
- [ ] Verify health checks passing
- [ ] Check application metrics
- [ ] Validate feature flag behavior
- [ ] Monitor error rates and latency
- [ ] Verify database connectivity
- [ ] Check external service integrations
- [ ] Monitor system resource usage

### Post-Deployment Checklist
- [ ] All health checks green
- [ ] Performance metrics within SLA
- [ ] Feature flags working as expected
- [ ] No increase in error rates
- [ ] User acceptance testing passed
- [ ] Monitor for 24 hours
- [ ] Document lessons learned
- [ ] Update deployment playbook
- [ ] Celebrate successful deployment! 🎉

---

**Document Version**: 1.0  
**Last Updated**: January 12, 2025  
**Next Review**: Weekly during sprint retrospectives  
**Owner**: Tech Lead & DevOps Team  
**Approvers**: CTO, VP Engineering