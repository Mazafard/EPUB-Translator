# EPUB Translator Makefile

.PHONY: build clean run test fmt vet lint help

# Default target
.DEFAULT_GOAL := help

# Build configuration
BINARY_NAME := epub-translator
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/epub-translator/main.go

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags "-s -w"

## build: Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "Clean complete"

## run: Run the application with default settings
run: build
	@echo "Starting EPUB Translator..."
	@echo "📋 Checking configuration..."
	@if [ ! -f config.json ]; then \
		echo "⚠️  No config.json found, will create from example"; \
	fi
	./$(BUILD_DIR)/$(BINARY_NAME)

## run-dev: Run the application in development mode
run-dev: build
	@echo "Starting EPUB Translator in development mode..."
	@echo "📋 Checking configuration..."
	@if [ ! -f config.json ]; then \
		echo "⚠️  No config.json found, will create from example"; \
	fi
	./$(BUILD_DIR)/$(BINARY_NAME) --verbose

## run-with-key: Run with OpenAI API key from environment
run-with-key: build
	@echo "Starting EPUB Translator with API key from environment..."
	@if [ -z "$$OPENAI_API_KEY" ]; then \
		echo "❌ OPENAI_API_KEY environment variable not set"; \
		exit 1; \
	fi
	./$(BUILD_DIR)/$(BINARY_NAME) --verbose

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -cover ./...

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## tidy: Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## check: Run all checks (fmt, vet, test)
check: fmt vet test

## build-all: Build for multiple platforms
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux amd64
	@echo "Building for Linux amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# macOS amd64
	@echo "Building for macOS amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# macOS arm64
	@echo "Building for macOS arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	# Windows amd64
	@echo "Building for Windows amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	@echo "Multi-platform build complete"

## install: Install the application
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t epub-translator:latest .

## setup-dev: Setup development environment
setup-dev: deps
	@echo "Setting up development environment..."
	@mkdir -p tmp output
	@cp config.example.json config.json || true
	@echo "Development environment ready"

## help: Show this help message
help:
	@echo "EPUB Translator - Available commands:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'