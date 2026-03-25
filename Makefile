export GOOS ?= $(shell go env GOOS)
export GOARCH ?= $(shell go env GOARCH)

VERSION := $(shell grep '^version' pyproject.toml | head -1 | sed 's/.*"\(.*\)"/\1/')

.DEFAULT_GOAL := all

.PHONY: all fmt test test-integration test-all test-race view-coverage lint build clean docs docker-offical-build chart-install bump release

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

bump:
	@if [ -z "$(PART)" ]; then echo "Usage: make bump PART=major|minor|patch"; exit 1; fi
	@IFS='.' read -r major minor patch <<< "$(VERSION)"; \
	case "$(PART)" in \
		major) major=$$((major + 1)); minor=0; patch=0;; \
		minor) minor=$$((minor + 1)); patch=0;; \
		patch) patch=$$((patch + 1));; \
		*) echo "PART must be major, minor, or patch"; exit 1;; \
	esac; \
	new_version="$$major.$$minor.$$patch"; \
	sed -i '' "s/^version = \"$(VERSION)\"/version = \"$$new_version\"/" pyproject.toml; \
	uv sync; \
	echo "Bumped version: $(VERSION) -> $$new_version"

release: lint test test-integration
	@echo "Creating release v$(VERSION)..."
	git tag "v$(VERSION)"
	git push origin "v$(VERSION)"
	gh release create "v$(VERSION)" --generate-notes
