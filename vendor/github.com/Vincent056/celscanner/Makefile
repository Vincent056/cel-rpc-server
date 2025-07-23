# Makefile for CEL Go Scanner

# Variables
GO_VERSION = 1.23.0
BINARY_NAME = celscanner
PACKAGE_NAME = github.com/Vincent056/celscanner

# Default target
.PHONY: all
all: test build

# Build the project
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) .

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean: clean-examples
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download

# Run all quality checks
.PHONY: quality
quality: fmt lint test

# Create release
.PHONY: release
release: clean quality build
	@echo "Creating release..."
	mkdir -p dist
	tar -czf dist/$(BINARY_NAME)-$(shell git describe --tags --abbrev=0).tar.gz bin/$(BINARY_NAME)

# Generate documentation
.PHONY: docs
docs:
	@echo "Generating documentation..."
	godoc -http=:6060

# Example targets
.PHONY: examples
examples: example-basic example-complex example-kubernetes example-filesystem example-system-monitoring example-scanner-in-cluster example-http-api-security
	@echo "All examples completed!"

.PHONY: example
example: example-basic

.PHONY: example-basic
example-basic:
	@echo "Running basic example..."
	@cd examples/basic && go run main.go

.PHONY: example-complex
example-complex:
	@echo "Running complex example..."
	@cd examples/complex && go run main.go

.PHONY: example-kubernetes
example-kubernetes:
	@echo "Running kubernetes example..."
	@cd examples/kubernetes && go run main.go

.PHONY: example-filesystem
example-filesystem:
	@echo "Running filesystem example..."
	@cd examples/filesystem && go run main.go

.PHONY: example-live-kubernetes
example-live-kubernetes:
	@echo "Running live kubernetes example..."
	@echo "Note: This requires a live Kubernetes cluster connection"
	@cd examples/live-kubernetes && go run main.go

.PHONY: example-system-monitoring
example-system-monitoring:
	@echo "Running system monitoring example..."
	@cd examples/system-monitoring && go run main.go

.PHONY: example-system-security
example-system-security:
	@echo "Running system security example..."
	@cd examples/system-security && go run main.go

.PHONY: example-scanner-in-cluster
example-scanner-in-cluster:
	@echo "Running in-cluster security scanner example..."
	@echo "Note: This demonstrates container-native security scanning approaches"
	@cd examples/scanner-in-cluster && go run main.go

.PHONY: example-http-api-security
example-http-api-security:
	@echo "Running HTTP API security scanning example..."
	@echo "Note: This demonstrates REST API endpoint security validation"
	@cd examples/http-api-security && go run main.go

# Build all examples
.PHONY: build-examples
build-examples:
	@echo "Building all examples..."
	@cd examples/basic && go build -o basic main.go
	@cd examples/complex && go build -o complex main.go
	@cd examples/kubernetes && go build -o kubernetes main.go
	@cd examples/filesystem && go build -o filesystem main.go
	@cd examples/live-kubernetes && go build -o live-kubernetes main.go
	@cd examples/system-monitoring && go build -o system-monitoring main.go
	@cd examples/system-security && go build -o system-security main.go
	@cd examples/scanner-in-cluster && go build -o scanner-in-cluster main.go
	@cd examples/http-api-security && go build -o http-api-security main.go
	@echo "All examples built successfully!"

# Clean example binaries
.PHONY: clean-examples
clean-examples:
	@echo "Cleaning example binaries..."
	@find examples -name "basic" -type f -delete 2>/dev/null || true
	@find examples -name "complex" -type f -delete 2>/dev/null || true
	@find examples -name "kubernetes" -type f -delete 2>/dev/null || true
	@find examples -name "filesystem" -type f -delete 2>/dev/null || true
	@find examples -name "live-kubernetes" -type f -delete 2>/dev/null || true
	@find examples -name "system-monitoring" -type f -delete 2>/dev/null || true
	@find examples -name "system-security" -type f -delete 2>/dev/null || true
	@find examples -name "container-file-permissions" -type f -delete 2>/dev/null || true
	@find examples -name "scanner-in-cluster" -type f -delete 2>/dev/null || true
	@find examples -name "http-api-security" -type f -delete 2>/dev/null || true
	@echo "Example binaries cleaned!"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Test:"
	@echo "  build         - Build the project"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  benchmark     - Run benchmarks"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  tidy          - Tidy dependencies"
	@echo "  quality       - Run all quality checks"
	@echo ""
	@echo "Examples:"
	@echo "  examples                     - Run all examples"
	@echo "  example                      - Run basic example (alias)"
	@echo "  example-basic                - Run basic example"
	@echo "  example-complex              - Run complex example"
	@echo "  example-kubernetes           - Run kubernetes example"
	@echo "  example-filesystem           - Run filesystem example"
	@echo "  example-live-kubernetes      - Run live kubernetes example (requires cluster)"
	@echo "  example-system-monitoring    - Run system monitoring example"
	@echo "  example-system-security      - Run system security example"
	@echo "  example-scanner-in-cluster   - Run in-cluster security scanner example"
	@echo "  example-http-api-security    - Run HTTP API security scanning example"
	@echo "  build-examples               - Build all examples"
	@echo "  clean-examples               - Clean example binaries"
	@echo ""
	@echo "Other:"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  release       - Create release"
	@echo "  docs          - Generate documentation"
	@echo "  help          - Show this help message" 