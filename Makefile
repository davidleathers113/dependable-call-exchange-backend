# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFUZZ=$(GOCMD) test -fuzz
GOTOOL=$(GOCMD) tool
GOWORK=$(GOCMD) work
BINARY_NAME=dce-backend
BINARY_UNIX=$(BINARY_NAME)_unix
MAIN_PATH=./main.go

# Modern toolchain parameters (2025)
GOVULN=$(shell which govulncheck 2>/dev/null || echo $(HOME)/go/bin/govulncheck)
GOSEC=$(shell which gosec 2>/dev/null || echo $(HOME)/go/bin/gosec)
NANCY=$(shell which nancy 2>/dev/null || echo $(HOME)/go/bin/nancy)
TRIVY=$(shell which trivy 2>/dev/null || echo trivy)
JOBS?=$(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# Docker parameters
DOCKER_IMAGE=dependable-call-exchange-backend
DOCKER_TAG=latest

# Version and build info
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u +%Y%m%d.%H%M%S)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: help build clean test coverage deps fmt lint vet security docker-build docker-run dev

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) -v $(MAIN_PATH)

build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_UNIX) -v $(MAIN_PATH)

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.*

test: ## Run tests
	$(GOTEST) -v ./...

test-race: ## Run tests with race detection
	$(GOTEST) -race -v ./...

test-synctest: ## Run tests with Go 1.24 synctest (requires GOEXPERIMENT=synctest)
	GOEXPERIMENT=synctest $(GOTEST) -v ./...

test-integration: ## Run integration tests
	$(GOTEST) -tags=integration -v ./test/...

test-contract: ## Run contract tests
	@echo "Running OpenAPI contract validation tests..."
	$(GOTEST) -v -tags=contract ./internal/api/rest/ -run TestContract
	$(GOTEST) -v -tags=contract ./test/contract/ -run TestAPIContractCompliance

test-contract-validate: ## Validate OpenAPI specification
	@echo "Validating OpenAPI specification..."
	@if [ -f "api/openapi.yaml" ]; then \
		echo "OpenAPI spec found, validating..."; \
		if command -v swagger-codegen >/dev/null 2>&1; then \
			swagger-codegen validate -i api/openapi.yaml; \
		elif [ -f "swagger-codegen-cli.jar" ]; then \
			java -jar swagger-codegen-cli.jar validate -i api/openapi.yaml; \
		else \
			echo "Installing swagger-codegen-cli for validation..."; \
			wget -q https://repo1.maven.org/maven2/io/swagger/codegen/v3/swagger-codegen-cli/3.0.46/swagger-codegen-cli-3.0.46.jar -O swagger-codegen-cli.jar; \
			java -jar swagger-codegen-cli.jar validate -i api/openapi.yaml; \
		fi; \
	else \
		echo "Warning: OpenAPI specification not found at api/openapi.yaml"; \
	fi

test-contract-full: test-contract-validate test-contract ## Run full contract testing suite

test-property: ## Run property-based tests  
	$(GOTEST) -v -run="Property" ./...

test-parallel: ## Run tests in parallel with optimal job count
	$(GOTEST) -parallel=$(JOBS) -v ./...

test-short: ## Run short tests only
	$(GOTEST) -short -v ./...

test-verbose: ## Run tests with verbose output and timing  
	$(GOTEST) -v -count=1 -timeout=30m ./...

test-fuzz: ## Run fuzzing tests (Go 1.24)
	@echo "Running fuzzing tests for 30 seconds..."
	$(GOFUZZ)=. -fuzztime=30s ./...

coverage: ## Run tests with coverage
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

coverage-synctest: ## Run coverage with synctest
	GOEXPERIMENT=synctest $(GOTEST) -coverprofile=coverage-synctest.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage-synctest.out -o coverage-synctest.html

coverage-detailed: ## Generate detailed coverage with function-level analysis
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic -coverpkg=./... ./...
	$(GOTOOL) cover -func=coverage.out
	$(GOTOOL) cover -html=coverage.out -o coverage.html

coverage-merge: ## Merge coverage from multiple test runs (Go 1.24)
	@echo "Merging coverage data..."
	$(GOTOOL) covdata textfmt -i=coverage -o coverage-merged.out 2>/dev/null || echo "No coverage data to merge"

# E2E Testing with Testcontainers
test-e2e: ## Run E2E tests with Testcontainers
	@echo "Running E2E tests with Testcontainers..."
	$(GOTEST) -tags=e2e -timeout=20m -v ./test/e2e/...

test-e2e-short: ## Run E2E tests excluding performance tests
	@echo "Running E2E tests (short mode)..."
	$(GOTEST) -tags=e2e -short -timeout=10m -v ./test/e2e/...

test-e2e-parallel: ## Run E2E tests in parallel
	@echo "Running E2E tests in parallel..."
	$(GOTEST) -tags=e2e -timeout=20m -v -p 4 ./test/e2e/...

test-e2e-coverage: ## Run E2E tests with coverage
	@echo "Running E2E tests with coverage..."
	$(GOTEST) -tags=e2e -timeout=20m -v -coverprofile=coverage-e2e.out -covermode=atomic ./test/e2e/...
	$(GOTOOL) cover -html=coverage-e2e.out -o coverage-e2e.html
	@echo "Coverage report generated: coverage-e2e.html"

test-e2e-deps: ## Install E2E test dependencies
	@echo "Installing E2E test dependencies..."
	$(GOCMD) get github.com/testcontainers/testcontainers-go
	$(GOCMD) get github.com/testcontainers/testcontainers-go/modules/postgres
	$(GOCMD) get github.com/testcontainers/testcontainers-go/modules/redis
	$(GOCMD) get github.com/docker/go-connections/nat
	$(GOMOD) tidy

test-e2e-auth: ## Run only auth E2E tests
	@echo "Running auth E2E tests..."
	$(GOTEST) -tags=e2e -timeout=5m -v -run TestAuth ./test/e2e/

test-e2e-flow: ## Run only call flow E2E tests
	@echo "Running call flow E2E tests..."
	$(GOTEST) -tags=e2e -timeout=10m -v -run TestCallExchangeFlow ./test/e2e/

test-e2e-financial: ## Run only financial E2E tests
	@echo "Running financial E2E tests..."
	$(GOTEST) -tags=e2e -timeout=10m -v -run TestFinancial ./test/e2e/

test-e2e-performance: ## Run only performance E2E tests
	@echo "Running performance E2E tests..."
	$(GOTEST) -tags=e2e -timeout=30m -v -run TestPerformance ./test/e2e/

test-e2e-realtime: ## Run only real-time E2E tests
	@echo "Running real-time E2E tests..."
	$(GOTEST) -tags=e2e -timeout=10m -v -run TestRealTimeEvents ./test/e2e/

test-security: ## Run security tests (authentication, authorization, input validation, rate limiting)
	@echo "Running security tests..."
	$(GOTEST) -tags=security -timeout=15m -v ./test/security/...

test-security-auth: ## Run only authentication/authorization security tests
	@echo "Running authentication and authorization security tests..."
	$(GOTEST) -tags=security -timeout=5m -v -run TestSecurity_Authentication ./test/security/

test-security-input: ## Run only input validation security tests
	@echo "Running input validation security tests..."
	$(GOTEST) -tags=security -timeout=5m -v -run TestSecurity_InputValidation ./test/security/

test-security-rate: ## Run only rate limiting security tests
	@echo "Running rate limiting security tests..."
	$(GOTEST) -tags=security -timeout=5m -v -run TestSecurity_RateLimiting ./test/security/

test-security-data: ## Run only data protection security tests
	@echo "Running data protection security tests..."
	$(GOTEST) -tags=security -timeout=5m -v -run TestSecurity_DataProtection ./test/security/

test-security-suite: ## Run complete security test suite
	@echo "Running complete security test suite..."
	$(GOTEST) -tags=security -timeout=20m -v -run TestSecuritySuite ./test/security/

# Compliance Testing Targets

test-compliance: ## Run all compliance validation tests (GDPR, TCPA, SOX, CCPA)
	@echo "Running comprehensive compliance validation tests..."
	$(GOTEST) -tags=compliance -timeout=30m -v ./test/compliance/...

test-compliance-gdpr: ## Run GDPR compliance validation tests
	@echo "Running GDPR compliance tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestImmutableAuditComplianceTestSuite/TestGDPRComplianceValidation ./test/compliance/

test-compliance-tcpa: ## Run TCPA compliance validation tests
	@echo "Running TCPA compliance tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestImmutableAuditComplianceTestSuite/TestTCPAComplianceValidation ./test/compliance/

test-compliance-sox: ## Run SOX audit trail verification tests
	@echo "Running SOX audit trail tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestImmutableAuditComplianceTestSuite/TestSOXAuditTrailVerification ./test/compliance/

test-compliance-ccpa: ## Run CCPA privacy controls tests
	@echo "Running CCPA privacy controls tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestImmutableAuditComplianceTestSuite/TestCCPAPrivacyControlsTesting ./test/compliance/

test-compliance-retention: ## Run data retention policy validation tests
	@echo "Running retention policy validation tests..."
	$(GOTEST) -tags=compliance -timeout=10m -v -run TestRetentionPolicyValidation ./test/compliance/

test-compliance-rights: ## Run data subject rights testing
	@echo "Running data subject rights tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestDataSubjectRightsTesting ./test/compliance/

test-compliance-cross-regulation: ## Run cross-regulation compatibility tests
	@echo "Running cross-regulation compatibility tests..."
	$(GOTEST) -tags=compliance -timeout=15m -v -run TestCrossRegulationCompatibility ./test/compliance/

test-compliance-audit-trail: ## Run audit trail completeness tests
	@echo "Running audit trail completeness tests..."
	$(GOTEST) -tags=compliance -timeout=10m -v -run TestAuditTrailCompleteness ./test/compliance/

test-compliance-coverage: ## Run compliance tests with coverage reporting
	@echo "Running compliance tests with coverage..."
	$(GOTEST) -tags=compliance -timeout=30m -v -coverprofile=coverage-compliance.out -covermode=atomic ./test/compliance/...
	$(GOTOOL) cover -html=coverage-compliance.out -o coverage-compliance.html
	@echo "Compliance coverage report generated: coverage-compliance.html"

test-compliance-quick: ## Run quick compliance validation (subset of tests)
	@echo "Running quick compliance validation..."
	$(GOTEST) -tags=compliance -short -timeout=10m -v ./test/compliance/

test-compliance-parallel: ## Run compliance tests in parallel
	@echo "Running compliance tests in parallel..."
	$(GOTEST) -tags=compliance -timeout=30m -v -p 4 ./test/compliance/...

docker-clean: ## Clean up test containers and volumes
	@echo "Cleaning up Docker containers..."
	@docker ps -a | grep "dce-test" | awk '{print $$1}' | xargs -r docker rm -f || true
	@docker volume ls | grep "dce-test" | awk '{print $$2}' | xargs -r docker volume rm || true
	@docker network ls | grep "dce-test" | awk '{print $$1}' | xargs -r docker network rm || true

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

bench-audit: ## Run IMMUTABLE_AUDIT performance benchmarks with validation
	@echo "Running IMMUTABLE_AUDIT performance benchmarks..."
	./scripts/run-audit-benchmarks.sh

bench-audit-quick: ## Run audit benchmarks quickly for development
	@echo "Running quick audit benchmarks..."
	$(GOTEST) -bench=BenchmarkLogger_SingleEventLogging -benchtime=5s ./internal/service/audit/
	$(GOTEST) -bench=BenchmarkQuery_1MillionEvents -benchtime=5s ./internal/service/audit/
	$(GOTEST) -bench=BenchmarkExport_Throughput -benchtime=5s ./internal/service/audit/

bench-audit-logger: ## Run audit logger benchmarks (< 5ms write latency validation)
	@echo "Running audit logger performance benchmarks..."
	$(GOTEST) -bench=BenchmarkLogger -benchtime=10s -benchmem ./internal/service/audit/

bench-audit-query: ## Run audit query benchmarks (< 1s for 1M events validation)
	@echo "Running audit query performance benchmarks..."
	$(GOTEST) -bench=BenchmarkQuery -benchtime=10s -benchmem ./internal/service/audit/

bench-audit-export: ## Run audit export benchmarks (> 10K events/sec validation)
	@echo "Running audit export performance benchmarks..."
	$(GOTEST) -bench=BenchmarkExport -benchtime=10s -benchmem ./internal/service/audit/

bench-audit-integrity: ## Run audit integrity benchmarks
	@echo "Running audit integrity performance benchmarks..."
	$(GOTEST) -bench=BenchmarkIntegrity -benchtime=10s -benchmem ./internal/service/audit/

bench-audit-cache: ## Run audit cache benchmarks
	@echo "Running audit cache performance benchmarks..."
	$(GOTEST) -bench=BenchmarkCache -benchtime=10s -benchmem ./internal/service/audit/

bench-audit-regression: ## Run audit performance regression tests
	@echo "Running audit performance regression benchmarks..."
	$(GOTEST) -bench=.*PerformanceRegression -benchtime=30s -benchmem ./internal/service/audit/

bench-property: ## Run property-based benchmarks
	$(GOTEST) -bench=Property -benchmem ./...

bench-cpu: ## Run benchmarks with CPU profiling
	$(GOTEST) -bench=. -benchmem -cpuprofile=cpu.prof ./...

bench-mem: ## Run benchmarks with memory profiling  
	$(GOTEST) -bench=. -benchmem -memprofile=mem.prof ./...

profile-cpu: ## Analyze CPU profile
	$(GOTOOL) pprof cpu.prof

profile-mem: ## Analyze memory profile
	$(GOTOOL) pprof mem.prof

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy
	$(GOMOD) verify

fmt: ## Format code
	$(GOCMD) fmt ./...

lint: ## Run linter
	golangci-lint run

vet: ## Run go vet
	$(GOCMD) vet ./...

security: ## Run security checks
	$(GOSEC) ./...
	$(GOVULN) ./...
	$(MAKE) security-deps
	$(MAKE) semgrep

security-deps: ## Run Nancy dependency vulnerability scan
	@echo "Running Nancy dependency vulnerability scan..."
	$(GOCMD) list -json -deps ./... | $(NANCY) sleuth

security-deps-json: ## Generate JSON dependency vulnerability report
	$(GOCMD) list -json -deps ./... | $(NANCY) sleuth -o json > nancy-report.json

security-deps-ci: ## Run Nancy with CI mode (fails on vulnerabilities)
	@echo "Running Nancy in CI mode (will fail on vulnerabilities)..."
	@$(GOCMD) list -json -deps ./... | $(NANCY) sleuth --quiet || (echo "Vulnerability found!" && exit 1)

security-scan: ## Run Trivy filesystem scan for vulnerabilities
	@echo "Running Trivy filesystem scan..."
	$(TRIVY) fs --scanners vuln,misconfig,secret .
	$(TRIVY) fs --format json --output trivy-fs-report.json .

security-container: ## Run Trivy container scan on Docker image
	@echo "Running Trivy container scan on $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	$(TRIVY) image --scanners vuln,misconfig,secret $(DOCKER_IMAGE):$(DOCKER_TAG)
	$(TRIVY) image --format json --output trivy-container-report.json $(DOCKER_IMAGE):$(DOCKER_TAG)

security-sbom: ## Generate SBOM (Software Bill of Materials) with Trivy
	@echo "Generating SBOM with Trivy..."
	$(TRIVY) fs --format cyclonedx --output sbom-cyclonedx.json .
	$(TRIVY) fs --format spdx-json --output sbom-spdx.json .

security-config: ## Scan for misconfigurations with Trivy
	@echo "Scanning for misconfigurations..."
	$(TRIVY) config .
	$(TRIVY) config --format json --output trivy-config-report.json .

security-license: ## Scan licenses with Trivy
	@echo "Scanning licenses..."
	$(TRIVY) fs --scanners license --format table .
	$(TRIVY) fs --scanners license --format json --output trivy-license-report.json .

security-quick: ## Quick security scan (vulnerabilities only, no reports)
	@echo "Running quick security scan..."
	$(TRIVY) fs --scanners vuln --severity HIGH,CRITICAL .

security-all: ## Run all security checks (comprehensive)
	@echo "Running comprehensive security audit..."
	$(MAKE) security
	$(MAKE) security-sarif
	$(MAKE) vuln-json
	$(MAKE) security-deps-json
	$(MAKE) security-scan
	@echo "Security audit complete. Check reports: gosec-report.sarif, vuln-report.json, nancy-report.json, trivy-fs-report.json"

security-sarif: ## Generate SARIF security report (2025 CI/CD integration)
	$(GOSEC) -fmt sarif -out gosec-report.sarif ./...

vuln-json: ## Generate JSON vulnerability report (2025 automation)
	$(GOVULN) -json ./... > vuln-report.json

# Code Quality & Smell Testing Targets
.PHONY: smell-test smell-test-full smell-test-quick smell-test-report smell-test-fix smell-test-baseline smell-test-diff

smell-test: smell-test-full ## Run complete code smell analysis

smell-test-quick: ## Quick smell test (pre-commit)
	@echo "=== Running quick smell test ==="
	@golangci-lint run --fast
	@if command -v gocyclo &> /dev/null; then \
		gocyclo -over 15 -avg ./...; \
	else \
		echo "⚠️  gocyclo not installed, skipping complexity check"; \
	fi

smell-test-full: ## Full smell test with all tools
	@echo "=== Running full smell test ==="
	@mkdir -p analysis/reports
	@golangci-lint run --out-format json > analysis/reports/golangci-$(shell date +%Y%m%d-%H%M%S).json || true
	@if command -v staticcheck &> /dev/null; then \
		staticcheck -f json ./... > analysis/reports/staticcheck-$(shell date +%Y%m%d-%H%M%S).json 2>&1 || true; \
	fi
	@if command -v gosec &> /dev/null; then \
		gosec -fmt json -out analysis/reports/gosec-$(shell date +%Y%m%d-%H%M%S).json ./... || true; \
	fi
	@go test ./test/architecture/... -v || true

smell-test-report: smell-test-full ## Generate HTML report
	@if [ -f scripts/generate-report/generate-report.go ]; then \
		go run scripts/generate-report/generate-report.go; \
		echo "Report generated at: analysis/reports/code-smell-report.html"; \
	else \
		echo "Report generator not found"; \
	fi

smell-test-fix: ## Auto-fix what can be fixed
	@golangci-lint run --fix
	@goimports -w .
	@gofmt -w .

smell-test-baseline: ## Create baseline for incremental analysis
	@mkdir -p analysis/baseline
	@golangci-lint run --out-format json > analysis/baseline/golangci.json
	@echo "Baseline created at: analysis/baseline/"

smell-test-diff: ## Compare against baseline
	@golangci-lint run --new-from-rev=HEAD~1

smell-test-antipatterns: ## Detect Go anti-patterns
	@echo "=== Detecting Go Anti-Patterns ==="
	@if [ -x scripts/detect-antipatterns.sh ]; then \
		./scripts/detect-antipatterns.sh; \
	else \
		echo "Anti-pattern detection script not found"; \
	fi

smell-test-ddd: ## Detect DDD smells
	@echo "=== Detecting DDD Smells ==="
	@if [ -f scripts/detect-ddd-smells/detect-ddd-smells.go ]; then \
		go run scripts/detect-ddd-smells/detect-ddd-smells.go > analysis/ddd-smells.txt; \
	else \
		echo "DDD smell detection script not found"; \
	fi

smell-test-boundaries: ## Check domain boundaries
	@echo "=== Checking Domain Boundaries ==="
	@if [ -x scripts/check-domain-boundaries.sh ]; then \
		./scripts/check-domain-boundaries.sh; \
	else \
		echo "Domain boundary check script not found"; \
	fi

smell-test-ci: ## Run smell tests for CI/CD
	@if [ -x scripts/ci-check.sh ]; then \
		./scripts/ci-check.sh; \
	else \
		echo "CI check script not found"; \
	fi

docker-build: ## Build docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-build-scan: ## Build docker image and scan with Trivy
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	$(MAKE) security-container

docker-run: ## Run docker container
	docker run -p 8080:8080 -p 9090:9090 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-push: ## Push docker image
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

dev: ## Run in development mode
	$(GOCMD) run $(MAIN_PATH)

dev-watch: ## Run with file watching (requires air)
	air

install-tools: ## Install development tools
	$(GOCMD) install github.com/air-verse/air@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/sonatype-nexus-community/nancy@latest
	@if [ -x scripts/install-smell-tools.sh ]; then \
		./scripts/install-smell-tools.sh; \
	fi
	$(MAKE) install-semgrep
	@echo "Note: Trivy must be installed separately. See: https://aquasecurity.github.io/trivy/latest/getting-started/installation/"
	@echo "  macOS: brew install trivy"
	@echo "  Linux: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin"
	@echo "  Docker: docker run aquasec/trivy"

ci: deps fmt vet lint security test test-contract ## Run CI pipeline

ci-contract: deps fmt vet lint security test test-contract-full ## Run CI pipeline with full contract validation

ci-compliance: deps fmt vet lint security test test-contract test-compliance ## Run CI pipeline with compliance validation

ci-fast: deps fmt vet lint security-sarif security-deps-ci semgrep-ci test-parallel ## Fast CI pipeline (2025 optimization)

ci-performance: ## Performance-focused CI pipeline with audit benchmarks
	@echo "Running performance-focused CI pipeline..."
	$(MAKE) deps
	$(MAKE) fmt
	$(MAKE) vet
	$(MAKE) test-short
	$(MAKE) bench-audit-quick
	@echo "Performance CI complete."

ci-audit-validation: ## Full audit performance validation for releases
	@echo "Running comprehensive audit performance validation..."
	$(MAKE) deps
	$(MAKE) fmt
	$(MAKE) vet
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) bench-audit
	@echo "Audit validation complete - ready for release."

ci-security: ## Security-focused CI pipeline with comprehensive scanning
	@echo "Running security-focused CI pipeline..."
	$(MAKE) security-all
	$(MAKE) semgrep-ci
	$(MAKE) semgrep-sarif
	$(MAKE) security-config
	$(MAKE) security-license
	$(MAKE) security-sbom
	@echo "Security CI complete. Check all security reports."

ci-full: ci-fast test-race test-synctest test-integration coverage docker-build-scan ## Complete CI pipeline with container scanning

quality-gate: ## Run quality checks before commit (2025 developer workflow)
	@echo "Running quality gate..."
	@$(MAKE) fmt
	@$(MAKE) vet  
	@$(MAKE) lint
	@$(MAKE) test-short
	@echo "Quality gate passed!"

workspace-init: ## Initialize Go workspace (future multi-module support)
	$(GOWORK) init .

workspace-sync: ## Sync workspace modules
	$(GOWORK) sync

clean-all: ## Clean all generated files including profiles (2025 comprehensive cleanup)
	$(GOCLEAN)
	rm -rf bin/ dist/ build/
	rm -f coverage.* *.prof *.trace *.out *.sarif
	rm -f vuln-report.json gosec-report.sarif nancy-report.json
	rm -f trivy-fs-report.json trivy-container-report.json trivy-config-report.json trivy-license-report.json
	rm -f sbom-cyclonedx.json sbom-spdx.json
	rm -f semgrep-report.json semgrep-report.sarif
	rm -f build-errors.log

all: clean deps fmt vet lint security test build ## Build everything

# Repomix targets
repomix: ## Generate both compressed and full repomix outputs
	./scripts/generate-repomix.sh

repomix-compress: ## Generate compressed repomix (structure-focused)
	./scripts/generate-repomix.sh compress

repomix-full: ## Generate full repomix (complete implementation)
	./scripts/generate-repomix.sh full

repomix-clean: ## Clean repomix outputs
	./scripts/generate-repomix.sh clean

repomix-archive: ## Archive current repomix outputs with timestamp
	./scripts/generate-repomix.sh archive

# Monitoring targets
monitoring-up: ## Start Prometheus and Grafana monitoring stack
	docker-compose -f docker-compose.monitoring.yml up -d
	@echo "Monitoring stack started:"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana: http://localhost:3000 (admin/admin)"
	@echo "  Alertmanager: http://localhost:9093"

monitoring-down: ## Stop monitoring stack
	docker-compose -f docker-compose.monitoring.yml down

monitoring-restart: ## Restart monitoring stack
	docker-compose -f docker-compose.monitoring.yml restart

monitoring-logs: ## View monitoring stack logs
	docker-compose -f docker-compose.monitoring.yml logs -f

monitoring-status: ## Check monitoring stack status
	docker-compose -f docker-compose.monitoring.yml ps

monitoring-clean: ## Clean monitoring data volumes
	docker-compose -f docker-compose.monitoring.yml down -v
	@echo "Warning: All monitoring data has been deleted"

monitoring-reload-prometheus: ## Reload Prometheus configuration
	curl -X POST http://localhost:9090/-/reload
	@echo "Prometheus configuration reloaded"

monitoring-check-config: ## Validate Prometheus configuration
	docker run --rm -v $(PWD)/monitoring:/monitoring prom/prometheus:latest promtool check config /monitoring/prometheus.yml

monitoring-check-alerts: ## Validate Prometheus alert rules
	docker run --rm -v $(PWD)/monitoring/prometheus:/prometheus prom/prometheus:latest promtool check rules /prometheus/alerts.yml

monitoring-backup: ## Backup Grafana dashboards
	@mkdir -p backups/grafana
	@echo "Backing up Grafana dashboards..."
	@docker exec -it dce-grafana grafana-cli admin export-dashboard --dir /var/lib/grafana/dashboards || true
	@docker cp dce-grafana:/var/lib/grafana/dashboards backups/grafana/$(shell date +%Y%m%d_%H%M%S)
	@echo "Backup completed to backups/grafana/"

monitoring-import-dashboards: ## Import Grafana dashboards from monitoring/grafana/dashboards
	@echo "Importing dashboards to Grafana..."
	docker exec -it dce-grafana grafana-cli dashboards import /var/lib/grafana/dashboards/*.json

monitoring-full: monitoring-up ## Start monitoring with full instrumentation check
	@sleep 5
	@echo "Checking Prometheus targets..."
	@curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
	@echo ""
	@echo "To view metrics:"
	@echo "  - Application metrics: http://localhost:8080/metrics"
	@echo "  - Prometheus targets: http://localhost:9090/targets"
	@echo "  - Grafana dashboards: http://localhost:3000"

# Semgrep targets
SEMGREP=$(shell which semgrep 2>/dev/null || echo semgrep)

semgrep: ## Run Semgrep with custom rules
	@echo "Running Semgrep security and compliance analysis..."
	$(SEMGREP) --config=.semgrep/config.yml --metrics=off

semgrep-ci: ## Run Semgrep in CI mode (JSON output, exit on findings)
	$(SEMGREP) --config=.semgrep/config.yml --json --output=semgrep-report.json --metrics=off
	@if [ -s semgrep-report.json ] && [ "$$(jq '.results | length' semgrep-report.json)" -gt 0 ]; then \
		echo "Semgrep found issues!"; \
		jq '.results[] | {rule: .check_id, file: .path, line: .start.line, message: .extra.message}' semgrep-report.json; \
		exit 1; \
	fi

semgrep-security: ## Run only security rules
	$(SEMGREP) --config=.semgrep/rules/telephony-security.yml --config=.semgrep/rules/compliance.yml --metrics=off

semgrep-performance: ## Run only performance rules
	$(SEMGREP) --config=.semgrep/rules/performance.yml --metrics=off

semgrep-domain: ## Run only domain pattern rules
	$(SEMGREP) --config=.semgrep/rules/domain-patterns.yml --metrics=off

semgrep-sarif: ## Generate SARIF report for CI/CD integration
	$(SEMGREP) --config=.semgrep/config.yml --sarif --output=semgrep-report.sarif --metrics=off

semgrep-autofix: ## Run Semgrep with autofix enabled (applies fixes)
	$(SEMGREP) --config=.semgrep/config.yml --autofix --metrics=off

semgrep-test: ## Test Semgrep rules on example code
	@echo "Testing Semgrep rules..."
	$(SEMGREP) --config=.semgrep/config.yml --test

semgrep-validate: ## Validate Semgrep rule syntax
	@echo "Validating Semgrep rules..."
	$(SEMGREP) --validate --config=.semgrep/rules/

install-semgrep: ## Install Semgrep
	@echo "Installing Semgrep..."
	@if command -v python3 >/dev/null 2>&1; then \
		python3 -m pip install semgrep; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install semgrep; \
	else \
		echo "Please install Python 3 or Homebrew first"; \
		exit 1; \
	fi