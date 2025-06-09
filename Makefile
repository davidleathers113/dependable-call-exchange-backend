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
GOVULN=govulncheck
GOSEC=gosec
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
	gosec ./...
	govulncheck ./...

security-sarif: ## Generate SARIF security report (2025 CI/CD integration)
	$(GOSEC) -fmt sarif -out gosec-report.sarif ./...

vuln-json: ## Generate JSON vulnerability report (2025 automation)
	$(GOVULN) -json ./... > vuln-report.json

docker-build: ## Build docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

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
	$(GOCMD) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest

ci: deps fmt vet lint security test ## Run CI pipeline

ci-fast: deps fmt vet lint security-sarif test-parallel ## Fast CI pipeline (2025 optimization)

ci-full: ci-fast test-race test-synctest test-integration coverage ## Complete CI pipeline

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
	rm -f vuln-report.json gosec-report.sarif
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