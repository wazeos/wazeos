.PHONY: test test-verbose test-coverage build clean run lint package package-tar package-zip release dist

# Version and build metadata
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Platform detection
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Build the binary
build:
	go build $(LDFLAGS) -o bin/wazeos ./cmd/wazeos

# Run the server
run:
	go run ./cmd/wazeos

# Run all tests
test:
	go test ./... -v -race

# Run tests with coverage
test-coverage:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with verbose output
test-verbose:
	go test ./... -v -race -count=1

# Run specific package tests
test-pkg:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-pkg PKG=./internal/types"; \
		exit 1; \
	fi
	go test $(PKG) -v -race

# Run linting
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf dist/
	rm -rf data/packages/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate mocks (if using mockery)
mocks:
	mockery --all --output ./internal/mocks --case underscore

# Run tests on file change (requires entr)
watch:
	find . -name '*.go' | entr -c make test

# Check for security vulnerabilities
security:
	gosec ./...

# Format code
fmt:
	go fmt ./...
	gofumpt -l -w .

# Run all checks (test + lint + security)
check: fmt test lint security

# Prepare distribution directory with binary
dist: build
	@echo "Preparing distribution..."
	@mkdir -p dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH)/data
	@cp bin/wazeos dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH)/
	@cp DISTRIBUTION.md dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH)/README.md
	@echo "Distribution prepared in dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH)/"

# Package as tar.gz
package-tar: dist
	@echo "Creating tar.gz package..."
	@cd dist && tar -czf wazeos-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz wazeos-$(VERSION)-$(GOOS)-$(GOARCH)
	@echo "✓ Package created: dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz"

# Package as zip
package-zip: dist
	@echo "Creating zip package..."
	@cd dist && zip -r wazeos-$(VERSION)-$(GOOS)-$(GOARCH).zip wazeos-$(VERSION)-$(GOOS)-$(GOARCH)
	@echo "✓ Package created: dist/wazeos-$(VERSION)-$(GOOS)-$(GOARCH).zip"

# Package both formats
package: package-tar package-zip
	@echo "✓ All packages created"

# Build for multiple platforms
release:
	@echo "Building release binaries for multiple platforms..."
	@mkdir -p dist

	@echo "Building for darwin/amd64..."
	@mkdir -p dist/wazeos-$(VERSION)-darwin-amd64/data
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/wazeos-$(VERSION)-darwin-amd64/wazeos ./cmd/wazeos
	@cp DISTRIBUTION.md dist/wazeos-$(VERSION)-darwin-amd64/README.md
	@cd dist && tar -czf wazeos-$(VERSION)-darwin-amd64.tar.gz wazeos-$(VERSION)-darwin-amd64

	@echo "Building for darwin/arm64..."
	@mkdir -p dist/wazeos-$(VERSION)-darwin-arm64/data
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/wazeos-$(VERSION)-darwin-arm64/wazeos ./cmd/wazeos
	@cp DISTRIBUTION.md dist/wazeos-$(VERSION)-darwin-arm64/README.md
	@cd dist && tar -czf wazeos-$(VERSION)-darwin-arm64.tar.gz wazeos-$(VERSION)-darwin-arm64

	@echo "Building for linux/amd64..."
	@mkdir -p dist/wazeos-$(VERSION)-linux-amd64/data
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/wazeos-$(VERSION)-linux-amd64/wazeos ./cmd/wazeos
	@cp DISTRIBUTION.md dist/wazeos-$(VERSION)-linux-amd64/README.md
	@cd dist && tar -czf wazeos-$(VERSION)-linux-amd64.tar.gz wazeos-$(VERSION)-linux-amd64

	@echo "Building for linux/arm64..."
	@mkdir -p dist/wazeos-$(VERSION)-linux-arm64/data
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/wazeos-$(VERSION)-linux-arm64/wazeos ./cmd/wazeos
	@cp DISTRIBUTION.md dist/wazeos-$(VERSION)-linux-arm64/README.md
	@cd dist && tar -czf wazeos-$(VERSION)-linux-arm64.tar.gz wazeos-$(VERSION)-linux-arm64

	@echo "Generating checksums..."
	@cd dist && shasum -a 256 *.tar.gz > checksums.txt

	@echo ""
	@echo "✓ Release complete! Packages created:"
	@cd dist && ls -lh *.tar.gz
	@echo ""
	@echo "Checksums:"
	@cat dist/checksums.txt

# Install locally (useful for testing)
install: build
	@echo "Installing wazeos to /usr/local/bin/..."
	@mkdir -p /usr/local/bin
	@cp bin/wazeos /usr/local/bin/
	@echo "✓ Installed! Run with: wazeos"

.DEFAULT_GOAL := test
