ARG GOVERSION=1.22

FROM golang:${GOVERSION}-alpine
LABEL org.opencontainers.image.source "https://github.com/jr200/nats-iam-broker"
LABEL org.opencontainers.image.description "nats-iam-broker alpine image"

ARG GOARCH
ARG GOOS

RUN echo nats-iam-broker-${GOOS}-${GOARCH}

RUN apk update && apk add git bash curl jq make
RUN go install github.com/nats-io/nats-server/v2@v2.10.18
RUN go install github.com/nats-io/natscli/nats@v0.1.5
RUN go install github.com/nats-io/nsc/v2@v2.8.6

# RUN curl -L https://github.com/openbao/openbao/releases/download/v2.0.0/bao_2.0.0_Linux_x86_64.tar.gz | tar xz -C /usr/local/bin bao

WORKDIR /usr/src/app

COPY . .

RUN make build && \
    ln build/nats-iam-broker-${GOOS}-${GOARCH} /usr/local/bin/nats-iam-broker && \
    ln build/test-client-${GOOS}-${GOARCH} /usr/local/bin/test-client

ENTRYPOINT ["bash"]
