# DBF Test Makefile

.PHONY: all test unit integration performance chaos e2e coverage clean

# Default target
all: test

# Run all tests
test: unit integration

# Run unit tests
unit:
	@echo "Running unit tests..."
	go test ./pkg/... ./internal/... -v -coverprofile=coverage.out
	go tool cover -func=coverage.out | grep total

# Run integration tests
integration:
	@echo "Running integration tests..."
	go test ./tests/integration/... -v -timeout=5m

# Run performance tests
performance:
	@echo "Running performance tests..."
	go test ./tests/performance/... -v -timeout=10m

# Run chaos tests
chaos:
	@echo "Running chaos tests..."
	go test ./tests/chaos/... -v -timeout=10m

# Run end-to-end tests
e2e:
	@echo "Running end-to-end tests..."
	go test ./tests/e2e/... -v -timeout=10m

# Generate coverage report
coverage: unit
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
race:
	@echo "Running tests with race detector..."
	go test ./... -race -timeout=10m

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	rm -f coverage.out coverage.html
	rm -rf tests/tmp/*

# Benchmark tests
benchmark:
	@echo "Running benchmark tests..."
	go test ./pkg/bloom -bench=. -benchmem

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Run all tests (default)"
	@echo "  test         - Run unit and integration tests"
	@echo "  unit         - Run unit tests"
	@echo "  integration  - Run integration tests"
	@echo "  performance  - Run performance tests"
	@echo "  chaos        - Run chaos tests"
	@echo "  e2e          - Run end-to-end tests"
	@echo "  coverage     - Generate coverage report"
	@echo "  race         - Run tests with race detector"
	@echo "  clean        - Clean test artifacts"
	@echo "  benchmark    - Run benchmark tests"
	@echo "  help         - Show this help message"
