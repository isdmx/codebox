# Makefile for Codebox project

.PHONY: build lint test vet fmt clean install-tools

# Install golangci-lint
install-tools:
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w -local github.com/isdmx/codebox .

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Build the project
build: 
	@echo "Building project..."
	go build ./cmd/server

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f server

# Run all quality checks
check: fmt vet lint test
	@echo "All checks passed!"

# Help target
help:
	@echo "Available targets:"
	@echo "  install-tools - Install golangci-lint"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run linter"
	@echo "  test         - Run tests"
	@echo "  build        - Build the project"
	@echo "  clean        - Clean build artifacts"
	@echo "  check        - Run all quality checks (fmt, vet, lint, test)"
	@echo "  help         - Show this help"