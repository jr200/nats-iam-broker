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

# get docker tag
ifeq ($(GOARCH),amd64)
	LATEST_TAG?=latest
	OIDC_SERVER_ARCH?=''
else
	LATEST_TAG?=latest-$(GOARCH)
	OIDC_SERVER_ARCH?='-arm64'
endif

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
	go build -o build/nats-iam-broker-$(GOOS)-$(GOARCH) -gcflags "all=-N -l" -ldflags '-extldflags "-static"' \
	cmd/nats-iam-broker/main.go

	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/test-client-$(GOOS)-$(GOARCH) -gcflags "all=-N -l" -ldflags '-extldflags "-static"' \
	cmd/test-client/main.go

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
# Target: docker-offical-build                                                 #
################################################################################
.PHONY: docker-offical-build
docker-offical-build:
	echo GOARCH=$(GOARCH)
	podman build \
	    --layers \
		-f docker/Dockerfile \
		--build-arg BUILD_OS=linux \
		--build-arg BUILD_ARCH=$(GOARCH) \
		-t ghcr.io/jr200/nats-iam-broker:local \
		.

################################################################################
# Target: docker-example-build                                                 #
################################################################################
.PHONY: docker-example-build
docker-example-build:
	echo GOARCH=$(GOARCH)
	podman build \
	    --layers \
		-f docker/Dockerfile.example \
		--build-arg BUILD_OS=linux \
		--build-arg BUILD_ARCH=$(GOARCH) \
		--build-arg OIDC_SERVER_ARCH=$(OIDC_SERVER_ARCH) \
		-t ghcr.io/jr200/nats-iam-broker:debug \
		.

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
		--set vault-actions.bootstrapToken=$(VAULT_TOKEN) \
		-f $(VALUES_PATH)

################################################################################
# Target: helm template
################################################################################
.PHONY: chart-template
chart-template: chart-deps
	helm template -n $(K8S_NAMESPACE) nats-iam-broker charts/nats-iam-broker \
		--set vault-actions.bootstrapToken=$(VAULT_TOKEN) \
		-f $(VALUES_PATH)

################################################################################
# Target: helm template
################################################################################
.PHONY: chart-dry-run
chart-dry-run:
	helm install \
		-n $(K8S_NAMESPACE) 
		-f $(VALUES_PATH) \
		--generate-name \
		--dry-run \
		--debug \
		--set vault-actions.bootstrapToken=$(VAULT_TOKEN) \
		charts/nats-iam-broker

################################################################################
# Target: example-shell                                                        #
################################################################################
.PHONY: example-shell
example-shell: docker-example-build
	docker run --rm -it --entrypoint bash ghcr.io/jr200/nats-iam-broker:debug

################################################################################
# Target: example-mock                                                        #
################################################################################
.PHONY: example-mock
example-mock: docker-example-build
	docker run --network=host --rm --entrypoint examples/mock/run.sh ghcr.io/jr200/nats-iam-broker:debug -log-human -log=info

################################################################################
# Target: example-basic                                                        #
################################################################################
.PHONY: example-basic
example-basic: docker-example-build
	docker run --rm --entrypoint examples/basic/run.sh ghcr.io/jr200/nats-iam-broker:debug -log-human -log=info

################################################################################
# Target: example-rgb_org                                                      #
################################################################################
.PHONY: example-rgb_org
example-rgb_org: docker-example-build
	docker run --rm --entrypoint examples/rgb_org/run.sh ghcr.io/jr200/nats-iam-broker:debug -log-human -log=info
