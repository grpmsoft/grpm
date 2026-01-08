# GRPM Makefile
# v0.1.0 - Single binary with daemon + CLI modes

BINARY = grpm
VERSION ?= v0.1.0-dev
GOARCH = amd64
GOOS ?= linux

# Version injection via ldflags
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S' 2>/dev/null || echo "unknown")

LDFLAGS = -ldflags "\
	-X 'main.Version=$(VERSION)' \
	-X 'main.GitCommit=$(GIT_COMMIT)' \
	-X 'main.BuildDate=$(BUILD_DATE)'"

# Default target
.DEFAULT_GOAL := build

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@mkdir -p api/gen
	protoc --go_out=api/gen --go_opt=paths=source_relative \
		--go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
		--proto_path=api/proto \
		api/proto/grpm.proto

# Build single binary
build:
	@echo "Building $(BINARY) $(VERSION)..."
	@mkdir -p bin
	GO111MODULE=on go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/grpm

# Build with debugging symbols
build-debug:
	@echo "Building $(BINARY) $(VERSION) with debug symbols..."
	@mkdir -p bin
	GO111MODULE=on go build $(LDFLAGS) -gcflags="all=-N -l" -o bin/$(BINARY) ./cmd/grpm

# Install to system
install: build
	@echo "Installing $(BINARY)..."
	install -m 0755 bin/$(BINARY) /usr/bin/$(BINARY)

# Run all tests
test:
	@echo "Running tests..."
	go test -v -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
 
# Run tests with race detector (requires CGO, Linux/CI only)
test-race:
	@echo "Running tests with race detector..."
	CGO_ENABLED=1 go test -v -race -coverprofile=coverage.out ./...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run --timeout=5m

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -w -s .
	go mod tidy

# Check code formatting (CI-friendly, no changes)
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "ERROR: The following files are not formatted:"; \
		gofmt -l .; \
		echo ""; \
		echo "Run 'make fmt' to fix formatting issues."; \
		exit 1; \
	fi
	@echo "All files are properly formatted ✓"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Development: run daemon locally
run-daemon: build
	@echo "Starting daemon..."
	./bin/$(BINARY) daemon

# Development: run CLI locally
run-cli: build
	@echo "Running CLI..."
	./bin/$(BINARY) status

# Create release tarball
release: build
	@echo "Creating release package..."
	tar -czf grpm-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz \
		-C bin $(BINARY)
	@echo "Release: grpm-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz"

# Multi-platform builds
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(MAKE) build
	mv bin/$(BINARY) bin/$(BINARY)-linux-amd64

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(MAKE) build
	mv bin/$(BINARY) bin/$(BINARY)-windows-amd64.exe

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(MAKE) build
	mv bin/$(BINARY) bin/$(BINARY)-darwin-amd64

build-all: build-linux build-windows build-darwin

# Development workflow
dev: fmt lint test build
	@echo "Development build complete!"

# CI/CD checks (includes formatting check)
ci: fmt-check test lint
	@echo "CI checks passed!"

# Help
help:
	@echo "GRPM Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make proto         - Generate protobuf code"
	@echo "  make build         - Build single binary"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make test-race     - Run tests with race detector (Linux/CI)"
	@echo "  make benchmark     - Run benchmarks"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install       - Install to /usr/bin"
	@echo "  make run-daemon    - Run daemon locally"
	@echo "  make run-cli       - Run CLI locally"
	@echo "  make release       - Create release tarball"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make dev           - Full development workflow"
	@echo "  make ci            - CI/CD checks"
	@echo ""
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(GIT_COMMIT)"
	@echo "Date:    $(BUILD_DATE)"

.PHONY: proto build build-debug install test test-coverage test-race benchmark lint fmt clean \
	run-daemon run-cli release build-linux build-windows build-darwin build-all \
	dev ci help
