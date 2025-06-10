# DevOps Tools Implementation Todo

This document outlines the implementation plan for enhancing the Dependable Call Exchange Backend with essential DevOps tools for security, observability, and performance monitoring.

## Phase 1: Immediate Security Hardening (Week 1)

### Nancy - Dependency Vulnerability Scanning

- [ ] **Install Nancy**
  ```bash
  go install github.com/sonatype-nexus-community/nancy@latest
  ```

- [ ] **Add to Makefile**
  ```makefile
  # Add to existing Makefile
  .PHONY: security-deps
  security-deps:
  	@echo "Scanning dependencies for vulnerabilities..."
  	@nancy sleuth -p go.sum || (echo "Vulnerabilities found!" && exit 1)
  
  # Update existing security target
  security: lint security-deps
  ```

- [ ] **Configure CI Pipeline**
  - Add Nancy to CI workflow after `go mod download`
  - Set up failure conditions for high/critical vulnerabilities
  - Create `.nancy-ignore` for false positives with justification

- [ ] **Create Security Policy**
  - Document vulnerability response times (Critical: 24h, High: 7d, Medium: 30d)
  - Define approval process for ignoring vulnerabilities
  - Add to `docs/security/vulnerability-management.md`

### Trivy - Container and File System Scanning

- [ ] **Install Trivy**
  ```bash
  # For macOS (development)
  brew install aquasecurity/trivy/trivy
  
  # For CI/Linux
  curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
  ```

- [ ] **Add to Makefile**
  ```makefile
  .PHONY: security-scan security-container
  security-scan:
  	@echo "Scanning filesystem for vulnerabilities..."
  	@trivy fs . --severity HIGH,CRITICAL --exit-code 1
  
  security-container: docker-build
  	@echo "Scanning container image..."
  	@trivy image dce-backend:latest --severity HIGH,CRITICAL --exit-code 1
  
  # Update security target
  security: lint security-deps security-scan
  ```

- [ ] **Configure Trivy**
  - Create `.trivyignore` for accepted risks
  - Add trivy cache to `.gitignore`
  - Configure severity thresholds for different environments

- [ ] **Enhance Dockerfile Security**
  - Verify multi-stage build uses minimal base image
  - Ensure non-root user execution
  - Add Trivy scan to docker-compose workflow

## Phase 2: Enhanced Observability (Week 2-3)

### OpenTelemetry - Comprehensive Instrumentation

- [ ] **Audit Current Coverage**
  - [ ] Create instrumentation coverage report
  - [ ] Identify uninstrumented critical paths:
    - [ ] Bid auction execution (<100ms target)
    - [ ] Call routing decisions (<1ms target)
    - [ ] Compliance checks (DNC, TCPA)
    - [ ] Fraud detection scoring

- [ ] **Enhance Domain Instrumentation**
  
  ```go
  // internal/services/callrouting/tracer.go
  package callrouting
  
  import (
      "go.opentelemetry.io/otel"
      "go.opentelemetry.io/otel/attribute"
      "go.opentelemetry.io/otel/metric"
  )
  
  var (
      tracer = otel.Tracer("dce.callrouting")
      meter  = otel.Meter("dce.callrouting")
      
      routingDuration metric.Float64Histogram
      routingCounter  metric.Int64Counter
  )
  
  func init() {
      routingDuration, _ = meter.Float64Histogram(
          "dce.callrouting.duration",
          metric.WithDescription("Call routing decision duration in milliseconds"),
          metric.WithUnit("ms"),
      )
      
      routingCounter, _ = meter.Int64Counter(
          "dce.callrouting.total",
          metric.WithDescription("Total routing decisions"),
      )
  }
  ```

- [ ] **Add Context Propagation**
  - [ ] Ensure all HTTP handlers use `otelhttp.NewHandler`
  - [ ] Add trace context to domain events
  - [ ] Implement trace ID in structured logs

- [ ] **Create Instrumentation Standards**
  - [ ] Document span naming conventions
  - [ ] Define required attributes per domain
  - [ ] Create helper functions for common patterns

### Prometheus + Grafana - Metrics Infrastructure

- [ ] **Set Up Prometheus**
  
  ```yaml
  # docker-compose.monitoring.yml
  version: '3.8'
  services:
    prometheus:
      image: prom/prometheus:latest
      ports:
        - "9090:9090"
      volumes:
        - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
        - prometheus-data:/prometheus
      command:
        - '--config.file=/etc/prometheus/prometheus.yml'
        - '--storage.tsdb.path=/prometheus'
        - '--storage.tsdb.retention.time=30d'
        - '--web.enable-lifecycle'
  
    grafana:
      image: grafana/grafana:latest
      ports:
        - "3000:3000"
      volumes:
        - ./monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards
        - ./monitoring/grafana/datasources:/etc/grafana/provisioning/datasources
        - grafana-data:/var/lib/grafana
      environment:
        - GF_SECURITY_ADMIN_PASSWORD=admin
        - GF_USERS_ALLOW_SIGN_UP=false
  
  volumes:
    prometheus-data:
    grafana-data:
  ```

- [ ] **Implement Domain Metrics**
  
  ```go
  // internal/metrics/registry.go
  package metrics
  
  import (
      "github.com/prometheus/client_golang/prometheus"
      "github.com/prometheus/client_golang/prometheus/promauto"
  )
  
  var (
      // Bid Domain Metrics
      BidProcessingDuration = promauto.NewHistogramVec(
          prometheus.HistogramOpts{
              Namespace: "dce",
              Subsystem: "bid",
              Name:      "processing_duration_seconds",
              Help:      "Duration of bid processing",
              Buckets:   prometheus.ExponentialBuckets(0.00001, 2, 15), // 10μs to 160ms
          },
          []string{"auction_type", "status"},
      )
      
      BidsPerSecond = promauto.NewGaugeVec(
          prometheus.GaugeOpts{
              Namespace: "dce",
              Subsystem: "bid",
              Name:      "throughput_per_second",
              Help:      "Current bid processing throughput",
          },
          []string{"auction_type"},
      )
      
      // Call Domain Metrics
      CallRoutingLatency = promauto.NewHistogramVec(
          prometheus.HistogramOpts{
              Namespace: "dce",
              Subsystem: "call",
              Name:      "routing_latency_seconds",
              Help:      "Call routing decision latency",
              Buckets:   prometheus.ExponentialBuckets(0.000001, 2, 20), // 1μs to 1s
          },
          []string{"algorithm", "result"},
      )
      
      ActiveCalls = promauto.NewGauge(
          prometheus.GaugeOpts{
              Namespace: "dce",
              Subsystem: "call",
              Name:      "active_total",
              Help:      "Number of active calls",
          },
      )
      
      // Compliance Domain Metrics
      ComplianceCheckDuration = promauto.NewHistogramVec(
          prometheus.HistogramOpts{
              Namespace: "dce",
              Subsystem: "compliance",
              Name:      "check_duration_seconds",
              Help:      "Compliance check duration",
          },
          []string{"check_type", "result"},
      )
      
      DNCListSize = promauto.NewGauge(
          prometheus.GaugeOpts{
              Namespace: "dce",
              Subsystem: "compliance",
              Name:      "dnc_list_size",
              Help:      "Current DNC list size",
          },
      )
  )
  ```

- [ ] **Add Metrics Endpoint**
  ```go
  // cmd/api/metrics.go
  import "github.com/prometheus/client_golang/prometheus/promhttp"
  
  // In your router setup
  router.Handle("/metrics", promhttp.Handler())
  ```

- [ ] **Create Grafana Dashboards**
  - [ ] System Overview Dashboard
    - API response times (p50, p90, p99)
    - Request rate by endpoint
    - Error rate by domain
    - Go runtime metrics (goroutines, memory, GC)
  
  - [ ] Bid Processing Dashboard
    - Bid throughput (target: 100K/sec)
    - Auction processing latency histogram
    - Success/failure rates by auction type
    - Queue depths and backpressure indicators
  
  - [ ] Call Routing Dashboard
    - Routing decision latency (target: <1ms)
    - Algorithm performance comparison
    - Geographic distribution of calls
    - Routing success rates
  
  - [ ] Compliance Dashboard
    - TCPA check latency
    - DNC list hit rate
    - Geographic compliance violations
    - Consent verification metrics

- [ ] **Set Up Alerts**
  ```yaml
  # monitoring/prometheus/alerts.yml
  groups:
    - name: dce_performance
      rules:
        - alert: HighCallRoutingLatency
          expr: histogram_quantile(0.99, dce_call_routing_latency_seconds) > 0.001
          for: 5m
          labels:
            severity: critical
            team: platform
          annotations:
            summary: "Call routing p99 latency exceeds 1ms target"
            
        - alert: LowBidThroughput
          expr: rate(dce_bid_processing_total[1m]) < 50000
          for: 5m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Bid processing below 50K/sec threshold"
  ```

## Phase 3: Code Quality and Security Analysis (Week 4)

### Semgrep - Custom Security and Compliance Rules

- [ ] **Install Semgrep**
  ```bash
  pip install semgrep
  # or
  brew install semgrep
  ```

- [ ] **Create Rule Categories**
  
  ```yaml
  # .semgrep/rules/telephony-security.yml
  rules:
    - id: phone-number-validation
      pattern-either:
        - pattern: |
            $PHONE := $INPUT
            ...
            $FUNC($PHONE, ...)
        - pattern: |
            $FUNC($INPUT, ...)
      pattern-not: |
        $PHONE := $INPUT
        ...
        domain.ValidatePhoneNumber($PHONE)
        ...
        $FUNC($PHONE, ...)
      message: "Phone number used without validation"
      languages: [go]
      severity: ERROR
      metadata:
        category: security
        compliance: TCPA
    
    - id: tcpa-time-check-required
      patterns:
        - pattern-either:
          - pattern: MakeCall(...)
          - pattern: InitiateCall(...)
          - pattern: QueueCall(...)
        - pattern-not-inside: |
            if compliance.IsCallTimeAllowed(...) {
              ...
            }
      message: "Call initiated without TCPA time restriction check"
      languages: [go]
      severity: ERROR
      
    - id: pii-logging
      pattern-either:
        - pattern: log.$METHOD(..., $PHONE, ...)
        - pattern: logger.$METHOD(..., $PHONE, ...)
      metavariable-regex:
        metavariable: $PHONE
        regex: '.*[Pp]hone.*|.*[Nn]umber.*'
      message: "Potential PII (phone number) in logs"
      languages: [go]
      severity: WARNING
  ```

- [ ] **Create Domain-Specific Rules**
  
  ```yaml
  # .semgrep/rules/domain-patterns.yml
  rules:
    - id: repository-transaction-required
      patterns:
        - pattern-inside: |
            func ($REPO *$TYPE) $METHOD(...) error {
              ...
            }
        - metavariable-regex:
            metavariable: $TYPE
            regex: '.*Repository'
        - pattern: |
            $REPO.$WRITE(...)
        - metavariable-regex:
            metavariable: $WRITE
            regex: 'Create|Update|Delete|Save'
        - pattern-not-inside: |
            tx := ...
            ...
            $REPO.$WRITE(..., tx, ...)
      message: "Repository write operation without transaction"
      languages: [go]
      severity: ERROR
      
    - id: money-arithmetic-safety
      pattern-either:
        - pattern: $MONEY1 + $MONEY2
        - pattern: $MONEY1 - $MONEY2
        - pattern: $MONEY1 * $FACTOR
      metavariable-type:
        metavariable: $MONEY1
        type: Money
      message: "Direct arithmetic on Money type - use Money.Add(), Money.Subtract(), etc."
      languages: [go]
      severity: ERROR
  ```

- [ ] **Integrate with CI/CD**
  ```makefile
  .PHONY: semgrep semgrep-ci
  semgrep:
  	@echo "Running Semgrep security analysis..."
  	@semgrep --config=.semgrep/rules --severity=ERROR --error
  
  semgrep-ci:
  	@semgrep ci --config=.semgrep/rules
  
  # Update security target
  security: lint security-deps security-scan semgrep
  ```

- [ ] **Configure Semgrep**
  - Create `.semgrepignore` for vendored code
  - Set up rule severity levels
  - Document rule writing guidelines

## Phase 4: Performance Testing and Monitoring (Week 5-6)

### Performance Benchmarking Framework

- [ ] **Create Benchmark Suite**
  ```go
  // internal/benchmark/routing_bench_test.go
  package benchmark
  
  func BenchmarkCallRouting(b *testing.B) {
      scenarios := []struct {
          name      string
          algorithm string
          buyers    int
      }{
          {"RoundRobin_10Buyers", "round-robin", 10},
          {"RoundRobin_100Buyers", "round-robin", 100},
          {"SkillBased_10Buyers", "skill-based", 10},
          {"Geographic_100Buyers", "geographic", 100},
      }
      
      for _, sc := range scenarios {
          b.Run(sc.name, func(b *testing.B) {
              // Setup
              router := setupRouter(sc.algorithm, sc.buyers)
              call := generateTestCall()
              
              b.ResetTimer()
              b.ReportAllocs()
              
              for i := 0; i < b.N; i++ {
                  _, err := router.Route(call)
                  if err != nil {
                      b.Fatal(err)
                  }
              }
              
              b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "ns/op")
          })
      }
  }
  ```

- [ ] **Add Continuous Benchmarking**
  ```makefile
  .PHONY: bench bench-compare
  bench:
  	@echo "Running performance benchmarks..."
  	@go test -bench=. -benchmem -benchtime=10s ./internal/benchmark/... | tee bench_current.txt
  
  bench-compare: bench
  	@if [ -f bench_baseline.txt ]; then \
  		benchstat bench_baseline.txt bench_current.txt; \
  	else \
  		echo "No baseline found. Run 'make bench-baseline' first."; \
  	fi
  
  bench-baseline:
  	@cp bench_current.txt bench_baseline.txt
  	@echo "Baseline updated"
  ```

### Load Testing Infrastructure

- [ ] **Create Load Test Scenarios**
  ```go
  // internal/loadtest/scenarios.go
  package loadtest
  
  type Scenario struct {
      Name            string
      Duration        time.Duration
      TargetRPS       int
      BidVolumePerSec int
      CallPattern     string // "steady", "spike", "gradual"
  }
  
  var ProductionScenarios = []Scenario{
      {
          Name:            "SteadyState",
          Duration:        10 * time.Minute,
          TargetRPS:       1000,
          BidVolumePerSec: 50000,
          CallPattern:     "steady",
      },
      {
          Name:            "PeakHour",
          Duration:        30 * time.Minute,
          TargetRPS:       5000,
          BidVolumePerSec: 100000,
          CallPattern:     "spike",
      },
  }
  ```

## Phase 5: CI/CD Enhancement (Week 7-8)

### GitLab CI Integration (if using GitLab)

- [ ] **Create .gitlab-ci.yml**
  ```yaml
  image: golang:1.21-alpine
  
  stages:
    - validate
    - test
    - security
    - build
    - deploy
  
  variables:
    GO_MODULE_CACHE: "${CI_PROJECT_DIR}/.go/pkg/mod"
  
  cache:
    key:
      files:
        - go.sum
    paths:
      - .go/pkg/mod/
  
  before_script:
    - apk add --no-cache make gcc musl-dev
    - go mod download
    - go mod verify
  
  lint:
    stage: validate
    script:
      - make lint
  
  test:
    stage: test
    script:
      - make test
      - make test-synctest
    coverage: '/total:\s+\(statements\)\s+(\d+.\d+)%/'
    artifacts:
      reports:
        coverage_report:
          coverage_format: cobertura
          path: coverage.xml
  
  property-tests:
    stage: test
    script:
      - make test-property
    timeout: 30m
  
  security-scan:
    stage: security
    script:
      - make security
    allow_failure: false
  
  benchmarks:
    stage: test
    script:
      - make bench-compare
    artifacts:
      paths:
        - bench_current.txt
      when: always
  
  build:
    stage: build
    script:
      - make docker-build
      - make security-container
    only:
      - main
      - develop
  ```

### GitHub Actions Integration (if using GitHub)

- [ ] **Create .github/workflows/ci.yml**
  ```yaml
  name: CI Pipeline
  
  on:
    push:
      branches: [ main, develop ]
    pull_request:
      branches: [ main ]
  
  jobs:
    test:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v3
        
        - name: Set up Go
          uses: actions/setup-go@v4
          with:
            go-version: '1.21'
            cache: true
        
        - name: Run tests
          run: |
            make test
            make test-synctest
        
        - name: Upload coverage
          uses: codecov/codecov-action@v3
          with:
            file: ./coverage.out
    
    security:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v3
        
        - name: Run Nancy
          uses: sonatype-nexus-community/nancy-github-action@main
        
        - name: Run Trivy
          uses: aquasecurity/trivy-action@master
          with:
            scan-type: 'fs'
            severity: 'HIGH,CRITICAL'
            exit-code: '1'
        
        - name: Run Semgrep
          uses: returntocorp/semgrep-action@v1
          with:
            config: .semgrep/rules
  ```

## Phase 6: Documentation and Training (Week 9-10)

### Documentation Updates

- [ ] **Create Security Runbook**
  - Document vulnerability response procedures
  - Create incident response templates
  - Define security contact escalation

- [ ] **Update Development Guide**
  - Add instrumentation best practices
  - Document metric naming conventions
  - Include security checklist for PRs

- [ ] **Create Operations Playbook**
  - Grafana dashboard usage guide
  - Alert response procedures
  - Performance tuning guidelines

### Team Training

- [ ] **Security Tools Workshop**
  - Nancy usage and vulnerability management
  - Trivy container scanning workflow
  - Semgrep rule writing basics

- [ ] **Observability Workshop**
  - OpenTelemetry instrumentation patterns
  - Creating effective Grafana dashboards
  - Setting meaningful alerts

- [ ] **Performance Workshop**
  - Writing effective benchmarks
  - Interpreting benchmark results
  - Load testing procedures

## Rollout Schedule

### Week 1-2: Security Foundations
- Nancy and Trivy implementation
- Basic security scanning in CI

### Week 3-4: Observability Core
- Prometheus and Grafana setup
- Critical path instrumentation
- Basic dashboards

### Week 5-6: Advanced Security
- Semgrep custom rules
- Domain-specific security patterns
- Compliance automation

### Week 7-8: CI/CD Enhancement
- Pipeline optimization
- Automated benchmarking
- Performance regression detection

### Week 9-10: Polish and Training
- Documentation completion
- Team training sessions
- Process refinement

## Success Metrics

- [ ] Zero high/critical vulnerabilities in production dependencies
- [ ] 100% of critical paths instrumented with OpenTelemetry
- [ ] Sub-1ms p99 latency for call routing visible in Grafana
- [ ] 100K+ bids/second sustained load verified
- [ ] Zero TCPA/GDPR compliance violations detected by Semgrep
- [ ] < 5 minute mean time to detection for performance regressions
- [ ] All team members trained on new tools

## Maintenance Tasks (Post-Implementation)

- [ ] Weekly dependency vulnerability review
- [ ] Monthly Grafana dashboard optimization
- [ ] Quarterly Semgrep rule updates
- [ ] Semi-annual benchmark baseline updates
- [ ] Annual security tool evaluation

## Resources

- [Nancy Documentation](https://github.com/sonatype-nexus-community/nancy)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Semgrep Rules](https://semgrep.dev/docs/writing-rules/)
- [Grafana Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)

## Notes

- All performance targets are based on production requirements
- Security scans should never be skipped, even for hotfixes
- Observability is not optional for production deployments
- Regular benchmarking prevents performance regression
