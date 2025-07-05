# Makefile
.PHONY: build clean test test-coverage test-race lint fmt vet security-scan docker-build help

# Build variables
BINARY_NAME=mcp-compose
GO=go
BUILD_DIR=build
SRC_MAIN=cmd/mcp-compose/main.go
COVERAGE_DIR=coverage

# Build the application
build:
	@echo "Building mcp-compose..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_MAIN)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Install the application
install: build
	@echo "Installing mcp-compose..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installation complete"

# Run tests
test:
	$(GO) test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GO) test -race ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Run vet
vet:
	@echo "Running vet..."
	$(GO) vet ./...

# Run security scan
security-scan:
	@echo "Running security scan..."
	gosec ./...

# Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build -f Dockerfile.proxy -t mcp-compose-proxy:latest .
	docker build -f Dockerfile.stdio-bridge -t mcp-compose-stdio-bridge:latest .

# Run all quality checks
quality: fmt vet lint test-race test-coverage security-scan

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(COVERAGE_DIR)
	@echo "Clean complete"

# Help
help:
	@echo "Available targets:"
	@echo "  build           - Build the application"
	@echo "  install         - Install the application"
	@echo "  test            - Run tests"
	@echo "  test-coverage   - Run tests with coverage"
	@echo "  test-race       - Run tests with race detection"
	@echo "  lint            - Run linter"
	@echo "  fmt             - Format code"
	@echo "  vet             - Run vet"
	@echo "  security-scan   - Run security scan"
	@echo "  docker-build    - Build Docker images"
	@echo "  quality         - Run all quality checks"
	@echo "  clean           - Clean build artifacts"
	@echo "  help            - Show this help"

