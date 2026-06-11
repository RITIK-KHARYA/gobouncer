.PHONY: check build run test test-integration lint clean fmt vet

# Binary name
BINARY=gobouncer

## check: Run all quality checks
check: fmt vet test build


## build: Compile the binary
build:
	go build -o $(BINARY) ./cmd/api/...

## run: Build and run the server
run: build
	./$(BINARY)

## test: Run unit tests
test:
	go test -v -count=1 ./...

## test-integration: Run integration tests (requires Redis)
test-integration:
	go test -v -tags=integration -count=1 ./internal/limiter/...

## test-cover: Run tests with coverage report
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## fmt: Format all Go files
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run fmt + vet
lint: fmt vet

## clean: Remove build artifacts
clean:
	go clean
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
