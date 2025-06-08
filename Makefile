# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=dce-backend
BINARY_UNIX=$(BINARY_NAME)_unix
MAIN_PATH=./main.go

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

coverage: ## Run tests with coverage
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

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

all: clean deps fmt vet lint security test build ## Build everything