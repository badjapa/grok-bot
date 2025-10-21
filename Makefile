# Makefile for Grok Bot Project

# Variables
BINARY_NAME=grok-bot
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe
BUILD_DIR=build
TMP_DIR=tmp

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Default target
.PHONY: all
all: clean deps build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_WINDOWS) -v ./main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_WINDOWS)"

# Build for multiple platforms
.PHONY: build-all
build-all: build-windows build-linux build-mac

# Build for Windows
.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_WINDOWS) -v ./main.go

# Build for Linux
.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_UNIX) -v ./main.go

# Build for macOS
.PHONY: build-mac
build-mac:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)_mac -v ./main.go

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)
	@if exist $(TMP_DIR) rmdir /s /q $(TMP_DIR)
	@echo "Clean complete"

# Run the application
.PHONY: run
run: env-setup
	@echo "Running $(BINARY_NAME)..."
	$(GOBUILD) -o $(TMP_DIR)/$(BINARY_WINDOWS) -v ./main.go
	$(TMP_DIR)/$(BINARY_WINDOWS)

# Run with environment variables from env.bat
.PHONY: run-env
run-env:
	@echo "Setting environment variables and running..."
	call env.bat && $(GOBUILD) -o $(TMP_DIR)/$(BINARY_WINDOWS) -v ./main.go && $(TMP_DIR)/$(BINARY_WINDOWS)

# Set up environment variables
.PHONY: env-setup
env-setup:
	@echo "Setting up environment variables..."
	@if not exist .env (echo # Grok Bot Environment Variables > .env && echo DISCORD_TOKEN=your_discord_token_here >> .env && echo GROK_API_KEY=your_grok_api_key_here >> .env)
	@echo "Environment setup complete. Please edit .env file with your actual values."

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies installed"

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "Dependencies updated"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "Code formatted"

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Linting code..."
	@if exist $(GOPATH)/bin/golangci-lint.exe (golangci-lint run) else (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")

# Install golangci-lint
.PHONY: install-lint
install-lint:
	@echo "Installing golangci-lint..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "golangci-lint installed"

# Create build directory
.PHONY: dirs
dirs:
	@if not exist $(BUILD_DIR) mkdir $(BUILD_DIR)
	@if not exist $(TMP_DIR) mkdir $(TMP_DIR)

# Development mode - run with auto-reload (requires air)
.PHONY: dev
dev:
	@echo "Starting development mode..."
	@if exist $(GOPATH)/bin/air.exe (air) else (echo "air not found. Install with: go install github.com/cosmtrek/air@latest" && make run)

# Install air for development
.PHONY: install-air
install-air:
	@echo "Installing air for development..."
	$(GOGET) github.com/cosmtrek/air@latest
	@echo "air installed"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, install deps, and build"
	@echo "  build        - Build the application"
	@echo "  build-all    - Build for Windows, Linux, and macOS"
	@echo "  build-windows- Build for Windows only"
	@echo "  build-linux  - Build for Linux only"
	@echo "  build-mac    - Build for macOS only"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
	@echo "  run-env      - Set environment variables and run"
	@echo "  env-setup    - Create .env template file"
	@echo "  deps         - Install dependencies"
	@echo "  update-deps  - Update dependencies"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"
	@echo "  install-lint - Install golangci-lint"
	@echo "  dev          - Development mode with auto-reload"
	@echo "  install-air  - Install air for development"
	@echo "  help         - Show this help message"

# Ensure directories exist before building
build: dirs
build-windows: dirs
build-linux: dirs
build-mac: dirs
