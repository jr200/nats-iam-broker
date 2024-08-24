# get target architecture
LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
	TARGET_ARCH_LOCAL=amd64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 5),armv8)
	TARGET_ARCH_LOCAL=arm64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 4),armv)
	TARGET_ARCH_LOCAL=arm
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 5),arm64)
	TARGET_ARCH_LOCAL=arm64
else ifeq ($(shell echo $(LOCAL_ARCH) | head -c 7),aarch64)
	TARGET_ARCH_LOCAL=arm64
else
	TARGET_ARCH_LOCAL=amd64
endif
export GOARCH ?= $(TARGET_ARCH_LOCAL)

# get docker tag
ifeq ($(GOARCH),amd64)
	LATEST_TAG?=latest
else
	LATEST_TAG?=latest-$(GOARCH)
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

DOCKER_REGISTRY ?= ghcr.io/jr200
IMAGE_NAME ?= nats-iam-broker


################################################################################
# Target: all                                                                  #
################################################################################
.PHONY: all
all: fmt build

################################################################################
# Target: fmt                                                                  #
################################################################################
.PHONY: fmt
fmt:
	go fmt $$(go list ./...)

################################################################################
# Target: build                                                                #
################################################################################
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/nats-iam-broker-$(GOOS)-$(GOARCH) -gcflags "all=-N -l" -ldflags '-extldflags "-static"' \
	cmd/nats-iam-broker/main.go

	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -o build/test-client-$(GOOS)-$(GOARCH) -gcflags "all=-N -l" -ldflags '-extldflags "-static"' \
	cmd/test-client/main.go


################################################################################
# Target: docker-build-example                                                 #
################################################################################
.PHONY: docker-build-example
docker-build-example:
	docker build \
		-f Dockerfile \
		--build-arg GOOS=linux --build-arg GOARCH=amd64 \
		-t nats-iam-broker:debug \
		.

################################################################################
# Target: example-shell                                                        #
################################################################################
.PHONY: example-shell
example-shell: docker-build-example
	docker run --rm -it --entrypoint bash nats-iam-broker:debug

################################################################################
# Target: example-basic                                                        #
################################################################################
.PHONY: example-basic
example-basic: docker-build-example
	docker run --rm --entrypoint examples/basic/run.sh nats-iam-broker:debug -log-human -log=info

################################################################################
# Target: example-rgb_org                                                      #
################################################################################
.PHONY: example-rgb_org
example-rgb_org: docker-build-example
	docker run --rm --entrypoint examples/rgb_org/run.sh nats-iam-broker:debug -log-human -log=info
