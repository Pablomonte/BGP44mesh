.PHONY: test test-coverage test-unit test-race test-integration build clean help
.PHONY: vet fmt lint deps coverage-html install build-race ci-test
.PHONY: deps-tidy deps-update watch

# Variables
GO := go
GOFLAGS := -v
BINARY := bgp-daemon
BUILD_DIR := bin
COVERAGE_FILE := coverage.out

# Build targets
build: ## Build the daemon binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/bgp-daemon

build-race: ## Build with race detector
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -race -o $(BUILD_DIR)/$(BINARY)-race ./cmd/bgp-daemon

install: build ## Install binary to $GOPATH/bin
	$(GO) install ./cmd/bgp-daemon

# Test targets
test: ## Run all tests
	$(GO) test $(GOFLAGS) ./...

test-unit: ## Run unit tests only (exclude integration)
	$(GO) test $(GOFLAGS) -short ./...

test-race: ## Run tests with race detector
	$(GO) test $(GOFLAGS) -race ./...

test-coverage: ## Run tests with coverage report
	$(GO) test $(GOFLAGS) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo "Coverage: $$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}')"

coverage-html: test-coverage ## Generate HTML coverage report
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration: ## Run integration tests (requires Docker)
	$(GO) test $(GOFLAGS) -tags=integration ./...

# Code quality targets
vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format code with gofmt
	$(GO) fmt ./...

lint: ## Run golangci-lint (requires golangci-lint)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

# Dependency management
deps: ## Download dependencies
	$(GO) mod download
	$(GO) mod verify

deps-tidy: ## Tidy dependencies
	$(GO) mod tidy

deps-update: ## Update dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

# Cleanup
clean: ## Clean build artifacts and coverage files
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) coverage.html

# Development helpers
watch: ## Watch for changes and rebuild (requires entr)
	@if command -v entr >/dev/null 2>&1; then \
		find . -name "*.go" | entr -r make build; \
	else \
		echo "entr not installed, install with: apt-get install entr"; \
	fi

# CI targets (used by GitHub Actions)
ci-test: vet test-race test-coverage ## Run CI test suite

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
