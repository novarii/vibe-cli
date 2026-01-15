.PHONY: build install clean release test fmt lint

# Binary name
BINARY=vibe

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Version info
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/novari/vibe-cli/internal/cli.Version=$(VERSION)"

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) cmd/vibe/main.go

# Install to /usr/local/bin
install: build
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Format code
fmt:
	$(GOFMT) ./...

# Update dependencies
deps:
	$(GOMOD) tidy

# Build release binaries for multiple platforms
release: clean
	@mkdir -p $(BUILD_DIR)
	@echo "Building darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 cmd/vibe/main.go
	@echo "Building darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 cmd/vibe/main.go
	@echo "Building linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 cmd/vibe/main.go
	@echo "Building linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 cmd/vibe/main.go
	@echo "Done!"
	@ls -la $(BUILD_DIR)/

# Development build and run
dev: build
	./$(BUILD_DIR)/$(BINARY)

# Show help
help:
	@echo "Available targets:"
	@echo "  build    - Build the binary"
	@echo "  install  - Build and install to /usr/local/bin"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"
	@echo "  fmt      - Format code"
	@echo "  deps     - Update dependencies"
	@echo "  release  - Build release binaries for all platforms"
	@echo "  dev      - Build and run"
	@echo "  help     - Show this help"
