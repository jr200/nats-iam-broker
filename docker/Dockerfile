ARG GOVERSION=1.26
FROM golang:${GOVERSION}-alpine AS builder

ARG BUILD_ARCH
ARG BUILD_OS

RUN apk update && apk add --no-cache git bash curl jq make

WORKDIR /usr/src/app

# pre-download all dependencies once for faster builds
COPY go.sum go.mod ./
RUN go mod download

COPY . .

RUN make build && \
    ln -s /usr/src/app/build/nats-iam-broker-${BUILD_OS}-${BUILD_ARCH} /usr/local/bin/nats-iam-broker

# minimal container
FROM alpine:3

LABEL org.opencontainers.image.source="https://github.com/jr200/nats-iam-broker"
LABEL org.opencontainers.image.description="nats-iam-broker runtime stage"

RUN apk add --no-cache bash ca-certificates jq && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /usr/src/app

COPY ./docker/docker-entrypoint.sh /docker-entrypoint.sh
COPY ./docker/config /config/

COPY --from=builder /usr/local/bin/nats-iam-broker /usr/local/bin/nats-iam-broker

USER appuser

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["nats-iam-broker", "version"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
