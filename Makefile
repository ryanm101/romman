.PHONY: build test clean install lint fmt run help

# Binary name
BINARY := romman
BUILD_DIR := bin

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Build flags
LDFLAGS := -s -w

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/romman

test: ## Run all tests
	$(GOTEST) -v ./...

test-short: ## Run tests without verbose output
	$(GOTEST) ./...

test-cover: ## Run tests with coverage
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f $(BINARY)

install: build ## Install binary to GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY) $(shell go env GOPATH)/bin/

fmt: ## Format code
	$(GOFMT) -w -s .

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

tidy: ## Tidy go.mod
	$(GOMOD) tidy

run: build ## Build and run with arguments (usage: make run ARGS="systems list")
	./$(BUILD_DIR)/$(BINARY) $(ARGS)

# Development helpers
dev: fmt test build ## Format, test, and build

.DEFAULT_GOAL := help
