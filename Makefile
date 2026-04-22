export GOOS ?= $(shell go env GOOS)
export GOARCH ?= $(shell go env GOARCH)

# Version is maintained by release-please in .release-please-manifest.json.
# Never edit manifest by hand — push conventional commits and let release-please open a PR.
VERSION := $(shell sed -n 's/.*"\.": *"\([^"]*\)".*/\1/p' .release-please-manifest.json)

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
	# -X internal/version.Version: bake the VERSION (from the release-please
	# manifest) into the binary so it self-reports correctly. The broker
	# startup log line, NATS micro registration, and OTel resource attributes
	# all read this through internal/broker.ResolveServiceVersion. See
	# internal/version/version.go for the precedence chain.
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/nats-iam-broker-$(GOOS)-$(GOARCH) \
	-ldflags '-extldflags "-static" -X github.com/jr200-labs/nats-iam-broker/internal/version.Version=v$(VERSION)' \
	./cmd/nats-iam-broker/

clean:
	@echo "Cleaning build artifacts and test cache..."
	rm -rf ./build
	rm -f coverage.out
	go clean -testcache

docs:
	cd docs/site && uv run quarto render
	cd docs/site && uv run quarto preview

# NOTE: releases are fully automated via release-please — see .github/workflows/release-please.yaml.
# Do not add bump/release targets here; they will drift from the CI flow.
