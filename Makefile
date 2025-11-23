.PHONY: test test-verbose test-coverage test-race lint build clean ci help

# Default target
.DEFAULT_GOAL := help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=fan-control
DOCKER_IMAGE=fan-control:latest

## help: Display this help message
help:
	@echo "Fan Controller - Available Make Targets:"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'
	@echo ""

## test: Run all unit tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-verbose: Run tests with verbose output and show individual test results
test-verbose:
	@echo "Running tests with verbose output..."
	$(GOTEST) -v -count=1 ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Coverage Summary:"
	$(GOCMD) tool cover -func=coverage.out | grep total
	@echo ""
	@echo "Generating HTML coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GOTEST) -v -race ./...

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## deps: Download Go module dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## build: Build the fan-control binary
build:
	@echo "Building binary..."
	$(GOBUILD) -o $(BINARY_NAME) -v

## build-docker: Build Docker image
build-docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

## clean: Clean build artifacts and test cache
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

## ci: Run all CI checks (test, lint, build)
ci: deps fmt vet test-coverage lint build
	@echo ""
	@echo "✓ All CI checks passed!"

## check-coverage: Verify coverage meets threshold (60%)
check-coverage:
	@echo "Checking coverage threshold..."
	@$(GOTEST) -coverprofile=coverage.out ./... > /dev/null 2>&1
	@COVERAGE=$$($(GOCMD) tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${COVERAGE}%"; \
	if [ $$(echo "$${COVERAGE} < 60" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage $${COVERAGE}% is below 60% threshold"; \
		exit 1; \
	else \
		echo "✓ Coverage $${COVERAGE}% meets threshold"; \
	fi

## install-tools: Install development tools (golangci-lint)
install-tools:
	@echo "Installing development tools..."
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2
	@echo "✓ Tools installed"
