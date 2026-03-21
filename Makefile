# get target architecture
LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
	TARGET_ARCH_LOCAL=amd64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 5),armv8)
	TARGET_ARCH_LOCAL=arm64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 4),armv)
	TARGET_ARCH_LOCAL=arm
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 6),arm64)
	TARGET_ARCH_LOCAL=arm64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 7),aarch64)
	TARGET_ARCH_LOCAL=arm64
else
	echo "Unknown architecture"
	exit -1
endif
export GOARCH ?= $(TARGET_ARCH_LOCAL)

# get target os
LOCAL_OS := $(shell uname -s)
ifeq ($(LOCAL_OS),Linux)
   TARGET_OS_LOCAL = linux
else ifeq ($(LOCAL_OS),Darwin)
   TARGET_OS_LOCAL = darwin
   PATH := $(PATH):$(HOME)/go/bin/darwin_$(GOARCH)
else
   echo "Not Supported"
   TARGET_OS_LOCAL = windows
endif
export GOOS ?= $(TARGET_OS_LOCAL)

# Default docker container and e2e test target.
TARGET_OS ?= linux
TARGET_ARCH ?= amd64

OUT_DIR := ./dist

.DEFAULT_GOAL := all

ifneq ($(wildcard ./private/charts/nats-iam-broker),)
VALUES_PATH := ./private/charts/nats-iam-broker/values.yaml
else
VALUES_PATH := ./charts/nats-iam-broker/values.yaml
endif


DOCKER_REGISTRY ?= ghcr.io/jr200
IMAGE_NAME ?= nats-iam-broker
K8S_NAMESPACE ?= nats-iam-broker

################################################################################
# Target: all                                                                  #
################################################################################
.PHONY: all
all: fmt lint build

################################################################################
# Target: fmt                                                                  #
################################################################################
.PHONY: fmt
fmt:
	go fmt ./...

################################################################################
# Target: test                                                                  #
################################################################################
.PHONY: test
test:
	go test -timeout=10m ./...

################################################################################
# Target: test-integration                                                     #
################################################################################
.PHONY: test-integration
test-integration:
	go test -tags=integration -timeout=5m -count=1 ./tests/integration/...

################################################################################
# Target: test-all                                                             #
################################################################################
.PHONY: test-all
test-all: test test-integration

################################################################################
# Target: test-race                                                            #
################################################################################
.PHONY: test-race
test-race:
	go test -race -cover -coverprofile=coverage.out -timeout=10m ./...

################################################################################
# Target: view-coverage                                                        #
################################################################################
.PHONY: view-coverage
view-coverage: test-race
	go tool cover -html=coverage.out

################################################################################
# Target: lint                                                                 #
################################################################################
.PHONY: lint
lint:
	go vet $$(go list ./...)
	@if command -v golangci-lint > /dev/null; then \
		echo "Running golangci-lint..."; \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not found, skipping"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

################################################################################
# Target: build                                                                #
################################################################################
.PHONY: build
build:
	# tidy up go.mod before building
	go mod tidy
	go mod download

	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/nats-iam-broker-$(GOOS)-$(GOARCH) -ldflags '-extldflags "-static"' \
	./cmd/nats-iam-broker/

################################################################################
# Target: clean                                                                #
################################################################################
.PHONY: clean
clean:
	@echo "Cleaning build artifacts and test cache..."
	rm -rf ./build
	rm -f coverage.out
	go clean -testcache

################################################################################
# Target: docs-preview                                                         #
################################################################################
.PHONY: docs-preview
docs-preview:
	cd docs/site && uv run quarto preview

################################################################################
# Target: docs-render                                                          #
################################################################################
.PHONY: docs-render
docs-render: docs-generate-config
	cd docs/site && uv run quarto render

################################################################################
# Target: docker-offical-build                                                 #
################################################################################
.PHONY: docker-offical-build
docker-offical-build:
	echo GOARCH=$(GOARCH)
	docker build \
		-f docker/Dockerfile \
		--build-arg BUILD_OS=linux \
		--build-arg BUILD_ARCH=$(GOARCH) \
		-t ghcr.io/jr200/nats-iam-broker:local \
		.

################################################################################
# Target: helm chart dependency update (regenerates Chart.lock)
################################################################################
.PHONY: chart-update-deps
chart-update-deps:
	helm dependency update charts/nats-iam-broker

################################################################################
# Target: helm chart dependencies
################################################################################
.PHONY: chart-deps
chart-deps:
	helm dependency build charts/nats-iam-broker --skip-refresh
	kubectl create namespace $(K8S_NAMESPACE) || echo "OK"

################################################################################
# Target: helm chart install
################################################################################
.PHONY: chart-install
chart-install: chart-deps
	helm upgrade -n $(K8S_NAMESPACE) nats-iam-broker charts/nats-iam-broker \
		--install \
		-f $(VALUES_PATH)

################################################################################
# Target: helm template
################################################################################
.PHONY: chart-template
chart-template: chart-deps
	helm template -n $(K8S_NAMESPACE) nats-iam-broker charts/nats-iam-broker \
		-f $(VALUES_PATH)

################################################################################
# Target: helm template
################################################################################
.PHONY: chart-dry-run
chart-dry-run:
	helm install \
		-n $(K8S_NAMESPACE) \
		-f $(VALUES_PATH) \
		--generate-name \
		--dry-run \
		--debug \
		charts/nats-iam-broker

