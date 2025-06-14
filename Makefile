# NNTP History Makefile

.PHONY: build test clean help example

# Default Go build flags
GOFLAGS := -v

# Build with SQLite3 support (always included)
build:
	@echo "Building with SQLite3 support..."
	go build $(GOFLAGS) .

# Test with SQLite3
test:
	@echo "Running tests..."
	go test $(GOFLAGS) .

# Get dependencies
deps:
	@echo "Getting dependencies..."
	go get github.com/mattn/go-sqlite3
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	go clean
	rm -f nntp-history
	rm -rf /tmp/nntp-history-*

# Check Go modules
mod-tidy:
	@echo "Tidying Go modules..."
	go mod tidy

# Run the SQLite3 example
example:
	@echo "Running SQLite3 example..."
	cd examples && go run sqlite3_example.go

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build with SQLite3 support"
	@echo "  test      - Run tests"
	@echo "  deps      - Get dependencies including SQLite3"
	@echo "  clean     - Clean build artifacts"
	@echo "  mod-tidy  - Tidy Go modules"
	@echo "  example   - Run SQLite3 example"
	@echo "  help      - Show this help"

# Default target
default: help
