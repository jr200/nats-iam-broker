export GOOS ?= $(shell go env GOOS)
export GOARCH ?= $(shell go env GOARCH)

.DEFAULT_GOAL := all

.PHONY: all fmt test test-integration test-all test-race view-coverage lint build clean docs docker-offical-build chart-install

all: fmt lint build

fmt:
	go fmt ./...

test:
	go test -timeout=10m ./...

test-integration:
	go test -tags=integration -timeout=5m -count=1 ./tests/integration/...

test-race:
	go test -race -cover -coverprofile=coverage.out -timeout=10m ./...

view-coverage: test-race
	go tool cover -html=coverage.out

lint:
	go vet ./...
	golangci-lint run --timeout=5m

build:
	go mod tidy
	go mod download
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/nats-iam-broker-$(GOOS)-$(GOARCH) -ldflags '-extldflags "-static"' \
	./cmd/nats-iam-broker/

clean:
	@echo "Cleaning build artifacts and test cache..."
	rm -rf ./build
	rm -f coverage.out
	go clean -testcache

docs:
	cd docs/site && uv run quarto render
	cd docs/site && uv run quarto preview
