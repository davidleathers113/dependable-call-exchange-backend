# Makefile additions for E2E testing with Testcontainers

# E2E Testing with Testcontainers
.PHONY: test-e2e
test-e2e: ## Run E2E tests with Testcontainers
	@echo "Running E2E tests with Testcontainers..."
	@go test -tags=e2e -timeout=20m -v .

.PHONY: test-e2e-short
test-e2e-short: ## Run E2E tests excluding performance tests
	@echo "Running E2E tests (short mode)..."
	@go test -tags=e2e -short -timeout=10m -v .

.PHONY: test-e2e-parallel
test-e2e-parallel: ## Run E2E tests in parallel
	@echo "Running E2E tests in parallel..."
	@go test -tags=e2e -timeout=20m -v -p 4 .

.PHONY: test-e2e-auth
test-e2e-auth: ## Run only auth E2E tests
	@echo "Running auth E2E tests..."
	@go test -tags=e2e -timeout=5m -v -run TestAuth .

.PHONY: test-e2e-flow
test-e2e-flow: ## Run only call flow E2E tests
	@echo "Running call flow E2E tests..."
	@go test -tags=e2e -timeout=10m -v -run TestCallExchangeFlow .

.PHONY: test-e2e-financial
test-e2e-financial: ## Run only financial E2E tests
	@echo "Running financial E2E tests..."
	@go test -tags=e2e -timeout=10m -v -run TestFinancial .

.PHONY: test-e2e-performance
test-e2e-performance: ## Run only performance E2E tests
	@echo "Running performance E2E tests..."
	@go test -tags=e2e -timeout=30m -v -run TestPerformance .

.PHONY: test-e2e-realtime
test-e2e-realtime: ## Run only real-time E2E tests
	@echo "Running real-time E2E tests..."
	@go test -tags=e2e -timeout=10m -v -run TestRealTimeEvents .

.PHONY: test-e2e-coverage
test-e2e-coverage: ## Run E2E tests with coverage
	@echo "Running E2E tests with coverage..."
	@go test -tags=e2e -timeout=20m -v -coverprofile=coverage-e2e.out -covermode=atomic .
	@go tool cover -html=coverage-e2e.out -o coverage-e2e.html
	@echo "Coverage report generated: coverage-e2e.html"

.PHONY: test-e2e-race
test-e2e-race: ## Run E2E tests with race detector
	@echo "Running E2E tests with race detector..."
	@go test -tags=e2e -timeout=30m -v -race .

.PHONY: test-e2e-benchmark
test-e2e-benchmark: ## Run E2E performance benchmarks
	@echo "Running E2E benchmarks..."
	@go test -tags=e2e -timeout=30m -bench=. -benchmem -run=^$ ./performance_test.go

.PHONY: docker-clean
docker-clean: ## Clean up test containers and volumes
	@echo "Cleaning up Docker containers..."
	@docker ps -a | grep "dce-test" | awk '{print $$1}' | xargs -r docker rm -f
	@docker volume ls | grep "dce-test" | awk '{print $$2}' | xargs -r docker volume rm
	@docker network ls | grep "dce-test" | awk '{print $$1}' | xargs -r docker network rm

.PHONY: test-e2e-deps
test-e2e-deps: ## Install E2E test dependencies
	@echo "Installing E2E test dependencies..."
	@go get github.com/testcontainers/testcontainers-go
	@go get github.com/testcontainers/testcontainers-go/modules/postgres
	@go get github.com/testcontainers/testcontainers-go/modules/redis
	@go get github.com/docker/go-connections/nat
	@go mod tidy

.PHONY: test-e2e-ci
test-e2e-ci: docker-clean test-e2e-deps ## Run E2E tests in CI environment
	@echo "Running E2E tests for CI..."
	@go test -tags=e2e -timeout=30m -v \
		-coverprofile=coverage-e2e.out \
		-json \
		. | tee test-e2e-results.json

# Help target update
.PHONY: help-e2e
help-e2e: ## Show E2E testing commands
	@echo "E2E Testing Commands:"
	@grep -E '^test-e2e[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
