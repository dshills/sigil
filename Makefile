# Makefile for Sigil

# Variables
BINARY_NAME := sigil
GO := go
GOLANGCI_LINT := golangci-lint
BUILD_DIR := ./build
DIST_DIR := ./dist

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build $(LDFLAGS) -o ./build/$(BINARY_NAME) cmd/sigil/main.go

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@$(GO) test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -v -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@$(GOLANGCI_LINT) run

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

# Run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy

# Update dependencies
.PHONY: deps-update
deps-update:
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy

# Build for all platforms
.PHONY: build-all
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 cmd/sigil/main.go
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 cmd/sigil/main.go
	# macOS ARM64
	@GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 cmd/sigil/main.go
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe cmd/sigil/main.go
	@echo "Build complete. Binaries in $(DIST_DIR)/"

# Install locally
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	@$(GO) install $(LDFLAGS) ./cmd/sigil

# Run the binary
.PHONY: run
run: build
	@./$(BINARY_NAME)

# Check everything (used in CI)
.PHONY: check
check: fmt vet lint test

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the binary"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deps          - Install dependencies"
	@echo "  make deps-update   - Update dependencies"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make install       - Install locally"
	@echo "  make run           - Build and run"
	@echo "  make check         - Run all checks (fmt, vet, lint, test)"
	@echo "  make help          - Show this help"
