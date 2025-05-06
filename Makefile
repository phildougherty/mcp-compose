# Makefile
.PHONY: build clean test

# Build variables
BINARY_NAME=mcp-compose
GO=go
BUILD_DIR=build
SRC_MAIN=cmd/mcp-compose/main.go

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

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"
