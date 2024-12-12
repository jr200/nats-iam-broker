ARG GOVERSION=1.22
FROM golang:${GOVERSION}-alpine AS builder

ARG BUILD_ARCH
ARG BUILD_OS

RUN apk update && apk add --no-cache git bash curl jq make

RUN go install github.com/nats-io/nats-server/v2@v2.10.20
RUN go install github.com/nats-io/natscli/nats@v0.1.5
RUN go install github.com/nats-io/nsc/v2@v2.9.0

WORKDIR /usr/src/app

# pre-download all dependencies once for faster builds
COPY go.sum go.mod ./
RUN go mod download

COPY . .

RUN make build && \
    ln -s /usr/src/app/build/nats-iam-broker-${BUILD_OS}-${BUILD_ARCH} /usr/local/bin/nats-iam-broker && \
    ln -s /usr/src/app/build/test-client-${BUILD_OS}-${BUILD_ARCH} /usr/local/bin/test-client

# minimal container
FROM alpine:3

LABEL org.opencontainers.image.source="https://github.com/jr200/nats-iam-broker"
LABEL org.opencontainers.image.description="nats-iam-broker runtime stage"

RUN apk add --no-cache bash

WORKDIR /usr/src/app

COPY --from=builder /usr/local/bin/nats-iam-broker /usr/local/bin/nats-iam-broker
COPY --from=builder /usr/local/bin/test-client /usr/local/bin/test-client

ENTRYPOINT ["bash"]
