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

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

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
	$(MAKE) install-semgrep
	@echo "Note: Trivy must be installed separately. See: https://aquasecurity.github.io/trivy/latest/getting-started/installation/"
	@echo "  macOS: brew install trivy"
	@echo "  Linux: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin"
	@echo "  Docker: docker run aquasec/trivy"

ci: deps fmt vet lint security test ## Run CI pipeline

ci-fast: deps fmt vet lint security-sarif security-deps-ci semgrep-ci test-parallel ## Fast CI pipeline (2025 optimization)

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