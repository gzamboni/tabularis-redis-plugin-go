.PHONY: help build build-all install install-force clean test test-unit test-e2e test-pubsub test-coverage \
        redis-start redis-stop redis-restart redis-status redis-cli redis-clean \
        release-patch release-minor release-major tag-push \
        lint fmt vet mod-tidy mod-download mod-verify \
        dev-setup seed-redis run-local

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME := tabularis-redis-plugin-go
MAIN_PATH := ./cmd/$(BINARY_NAME)
BUILD_DIR := .
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Platform detection
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    PLUGIN_DIR := $(HOME)/Library/Application Support/com.debba.tabularis/plugins/redis
else ifeq ($(UNAME_S),Linux)
    PLUGIN_DIR := $(HOME)/.local/share/tabularis/plugins/redis
else
    PLUGIN_DIR := $(APPDATA)/com.debba.tabularis/plugins/redis
endif

# Container runtime detection (Docker or Podman)
DOCKER := $(shell command -v docker 2>/dev/null)
PODMAN := $(shell command -v podman 2>/dev/null)

ifdef DOCKER
    CONTAINER_CMD := docker
else ifdef PODMAN
    CONTAINER_CMD := podman
else
    CONTAINER_CMD := echo "Error: Neither docker nor podman found. Please install one of them." && false &&
endif

# Redis configuration
REDIS_PORT := 6379
REDIS_CONTAINER := tabularis-redis-test
REDIS_IMAGE := redis:7-alpine

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

##@ General

help: ## Display this help message
	@echo "$(COLOR_BOLD)Tabularis Redis Plugin - Makefile Commands$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

build: ## Build the plugin binary
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME)...$(COLOR_RESET)"
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(COLOR_GREEN)✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(COLOR_RESET)"

build-all: ## Build for all platforms (Linux, macOS, Windows)
	@echo "$(COLOR_GREEN)Building for all platforms...$(COLOR_RESET)"
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PATH)
	@echo "$(COLOR_GREEN)✓ Cross-platform builds complete in dist/$(COLOR_RESET)"

install: build ## Build and install plugin to Tabularis plugins directory
	@echo "$(COLOR_GREEN)Installing plugin to: $(PLUGIN_DIR)$(COLOR_RESET)"
	@mkdir -p "$(PLUGIN_DIR)"
	@cp $(BUILD_DIR)/$(BINARY_NAME) "$(PLUGIN_DIR)/"
	@cp manifest.json "$(PLUGIN_DIR)/"
	@cp README.md "$(PLUGIN_DIR)/"
	@cp LICENSE "$(PLUGIN_DIR)/"
	@chmod +x "$(PLUGIN_DIR)/$(BINARY_NAME)"
	@echo "$(COLOR_GREEN)✓ Plugin installed successfully$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Restart Tabularis to use the updated plugin$(COLOR_RESET)"

install-force: clean build install ## Clean, build, and install plugin

uninstall: ## Remove plugin from Tabularis plugins directory
	@echo "$(COLOR_YELLOW)Uninstalling plugin from: $(PLUGIN_DIR)$(COLOR_RESET)"
	@rm -rf "$(PLUGIN_DIR)"
	@echo "$(COLOR_GREEN)✓ Plugin uninstalled$(COLOR_RESET)"

clean: ## Remove build artifacts
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -rf dist/
	@rm -f test_output.log
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

run-local: build ## Build and run plugin locally (for manual testing)
	@echo "$(COLOR_GREEN)Running plugin locally (pipe JSON-RPC requests to stdin)...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Example: echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test_connection\",\"params\":{\"params\":{\"driver\":\"redis\",\"host\":\"localhost\",\"port\":6379,\"database\":\"0\"}}}' | ./$(BINARY_NAME)$(COLOR_RESET)"
	./$(BINARY_NAME)

##@ Testing

test: test-unit ## Run all tests (alias for test-unit)

test-unit: ## Run unit tests
	@echo "$(COLOR_GREEN)Running unit tests...$(COLOR_RESET)"
	go test -v -race -timeout 30s ./...

test-coverage: ## Run tests with coverage report
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report generated: coverage.html$(COLOR_RESET)"

test-e2e: build ## Run end-to-end tests (requires Docker/Podman)
	@echo "$(COLOR_GREEN)Running E2E tests...$(COLOR_RESET)"
	@chmod +x tests/run_e2e.sh
	./tests/run_e2e.sh

test-pubsub: build ## Run Pub/Sub tests
	@echo "$(COLOR_GREEN)Running Pub/Sub tests...$(COLOR_RESET)"
	@chmod +x tests/test_pubsub_local.sh
	./tests/test_pubsub_local.sh

test-pubsub-comprehensive: build ## Run comprehensive Pub/Sub tests
	@echo "$(COLOR_GREEN)Running comprehensive Pub/Sub tests...$(COLOR_RESET)"
	@chmod +x tests/test_pubsub_comprehensive.sh
	./tests/test_pubsub_comprehensive.sh

test-all: test-unit test-e2e test-pubsub ## Run all tests (unit, E2E, Pub/Sub)

##@ Redis Server Management

redis-start: ## Start Redis server in Docker/Podman container
	@echo "$(COLOR_GREEN)Starting Redis server (using $(CONTAINER_CMD))...$(COLOR_RESET)"
	@if $(CONTAINER_CMD) ps -a --format '{{.Names}}' | grep -q "^$(REDIS_CONTAINER)$$"; then \
		echo "$(COLOR_YELLOW)Container $(REDIS_CONTAINER) already exists, starting...$(COLOR_RESET)"; \
		$(CONTAINER_CMD) start $(REDIS_CONTAINER); \
	else \
		$(CONTAINER_CMD) run -d --name $(REDIS_CONTAINER) -p $(REDIS_PORT):6379 $(REDIS_IMAGE); \
	fi
	@echo "$(COLOR_GREEN)✓ Redis server started on port $(REDIS_PORT)$(COLOR_RESET)"

redis-stop: ## Stop Redis server
	@echo "$(COLOR_YELLOW)Stopping Redis server...$(COLOR_RESET)"
	@$(CONTAINER_CMD) stop $(REDIS_CONTAINER) 2>/dev/null || echo "Container not running"
	@echo "$(COLOR_GREEN)✓ Redis server stopped$(COLOR_RESET)"

redis-restart: redis-stop redis-start ## Restart Redis server

redis-status: ## Check Redis server status
	@echo "$(COLOR_BLUE)Redis server status (using $(CONTAINER_CMD)):$(COLOR_RESET)"
	@$(CONTAINER_CMD) ps -a --filter name=$(REDIS_CONTAINER) --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

redis-cli: ## Connect to Redis CLI
	@echo "$(COLOR_GREEN)Connecting to Redis CLI...$(COLOR_RESET)"
	$(CONTAINER_CMD) exec -it $(REDIS_CONTAINER) redis-cli

redis-clean: redis-stop ## Stop and remove Redis container
	@echo "$(COLOR_YELLOW)Removing Redis container...$(COLOR_RESET)"
	@$(CONTAINER_CMD) rm $(REDIS_CONTAINER) 2>/dev/null || echo "Container already removed"
	@echo "$(COLOR_GREEN)✓ Redis container removed$(COLOR_RESET)"

redis-logs: ## Show Redis server logs
	@$(CONTAINER_CMD) logs -f $(REDIS_CONTAINER)

##@ Code Quality

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "$(COLOR_GREEN)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not installed. Install with: brew install golangci-lint$(COLOR_RESET)"; \
	fi

fmt: ## Format code with gofmt
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	gofmt -s -w .
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	go vet ./...

check: fmt vet lint test-unit ## Run all code quality checks

##@ Dependencies

mod-download: ## Download Go module dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	go mod download

mod-tidy: ## Tidy Go module dependencies
	@echo "$(COLOR_GREEN)Tidying dependencies...$(COLOR_RESET)"
	go mod tidy

mod-verify: ## Verify Go module dependencies
	@echo "$(COLOR_GREEN)Verifying dependencies...$(COLOR_RESET)"
	go mod verify

mod-update: ## Update all dependencies to latest versions
	@echo "$(COLOR_GREEN)Updating dependencies...$(COLOR_RESET)"
	go get -u ./...
	go mod tidy
	@echo "$(COLOR_GREEN)✓ Dependencies updated$(COLOR_RESET)"

##@ Release Management

release-patch: ## Create a new patch release (v0.0.X)
	@echo "$(COLOR_GREEN)Creating patch release...$(COLOR_RESET)"
	@./scripts/release.sh patch

release-minor: ## Create a new minor release (v0.X.0)
	@echo "$(COLOR_GREEN)Creating minor release...$(COLOR_RESET)"
	@./scripts/release.sh minor

release-major: ## Create a new major release (vX.0.0)
	@echo "$(COLOR_GREEN)Creating major release...$(COLOR_RESET)"
	@./scripts/release.sh major

tag-push: ## Push current tag to trigger release workflow
	@echo "$(COLOR_GREEN)Pushing tags to remote...$(COLOR_RESET)"
	git push origin --tags
	@echo "$(COLOR_GREEN)✓ Tags pushed. GitHub Actions will build and release.$(COLOR_RESET)"

tag-list: ## List all git tags
	@echo "$(COLOR_BLUE)Git tags:$(COLOR_RESET)"
	@git tag -l -n1

##@ Development Setup

dev-setup: mod-download ## Setup development environment
	@echo "$(COLOR_GREEN)Setting up development environment...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Installing development tools...$(COLOR_RESET)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(COLOR_YELLOW)Installing golangci-lint...$(COLOR_RESET)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "$(COLOR_GREEN)✓ Development environment ready$(COLOR_RESET)"

seed-redis: ## Seed Redis with test data (requires Redis running)
	@echo "$(COLOR_GREEN)Seeding Redis with test data...$(COLOR_RESET)"
	go run seed_redis.go
	@echo "$(COLOR_GREEN)✓ Redis seeded with test data$(COLOR_RESET)"

##@ Information

version: ## Show current version
	@echo "$(COLOR_BLUE)Version:$(COLOR_RESET) $(VERSION)"
	@echo "$(COLOR_BLUE)Commit:$(COLOR_RESET)  $(COMMIT)"
	@echo "$(COLOR_BLUE)Built:$(COLOR_RESET)   $(BUILD_TIME)"

info: ## Show project information
	@echo "$(COLOR_BOLD)Project Information$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Binary:$(COLOR_RESET)         $(BINARY_NAME)"
	@echo "$(COLOR_BLUE)Version:$(COLOR_RESET)        $(VERSION)"
	@echo "$(COLOR_BLUE)Plugin Dir:$(COLOR_RESET)     $(PLUGIN_DIR)"
	@echo "$(COLOR_BLUE)Redis Port:$(COLOR_RESET)     $(REDIS_PORT)"
	@echo "$(COLOR_BLUE)Container Cmd:$(COLOR_RESET)  $(CONTAINER_CMD)"
	@echo "$(COLOR_BLUE)Go Version:$(COLOR_RESET)     $(shell go version)"
