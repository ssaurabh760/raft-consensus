.PHONY: build test test-race lint fmt proto clean cluster-start cluster-stop demo vet coverage

# Build variables
BINARY_NAME=raft-node
BUILD_DIR=bin
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

# Build the raft node binary
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/raft/

# Run all tests
test:
	$(GO) test $(GOFLAGS) ./...

# Run tests with race detection
test-race:
	$(GO) test -race $(GOFLAGS) ./...

# Run tests with coverage
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run go vet
vet:
	$(GO) vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format all Go files
fmt:
	gofmt -s -w .

# Generate protobuf code (requires protoc and protoc-gen-go)
proto:
	protoc --go_out=. --go-grpc_out=. internal/rpc/proto/raft.proto

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Start a 5-node cluster via Docker Compose
cluster-start:
	./scripts/cluster-start.sh

# Stop the cluster
cluster-stop:
	./scripts/cluster-stop.sh

# Run the demo
demo:
	./scripts/demo.sh

# Docker build
docker-build:
	docker compose build

# Docker up
docker-up:
	docker compose up -d

# Docker down
docker-down:
	docker compose down
