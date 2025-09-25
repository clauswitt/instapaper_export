# Build variables
BINARY_NAME := instapaper-cli
VERSION := $(shell git describe --tags --abbrev=0 --match=v* 2>/dev/null || echo "v0.0.0")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS := -ldflags "-X 'instapaper-cli/internal/version.Version=$(VERSION)' \
                    -X 'instapaper-cli/internal/version.Commit=$(COMMIT)' \
                    -X 'instapaper-cli/internal/version.Date=$(DATE)'"

# Default target
.PHONY: all
all: build

# Development build (uses git version detection)
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) development version..."
	go build -o $(BINARY_NAME) cmd/instapaper-cli/main.go

# Release build with embedded version info
.PHONY: release
release:
	@echo "Building $(BINARY_NAME) release version $(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) cmd/instapaper-cli/main.go

# Multi-platform release builds (like GitHub Actions)
.PHONY: release-all
release-all:
	@echo "Building release binaries for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 cmd/instapaper-cli/main.go
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 cmd/instapaper-cli/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 cmd/instapaper-cli/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 cmd/instapaper-cli/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe cmd/instapaper-cli/main.go

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf dist/

# Test the build
.PHONY: test
test:
	go test ./...

# Show current version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

# Development workflow
.PHONY: dev
dev: clean build
	./$(BINARY_NAME) version

# Install locally
.PHONY: install
install: release
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	cp $(BINARY_NAME) $(GOPATH)/bin/

# Create dist directory
dist:
	mkdir -p dist

.PHONY: release-all
release-all: dist

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Development build (git version detection)"
	@echo "  release     - Release build with embedded version"
	@echo "  release-all - Multi-platform release builds"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  version     - Show version info"
	@echo "  dev         - Clean build and show version"
	@echo "  install     - Install to GOPATH/bin"
	@echo "  help        - Show this help"