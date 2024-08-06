#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")

export NATS_URL="nats://localhost:4222"
export SECRET_STORE="/usr/src/app/secrets/basic"
CONFIG_DIR="/usr/src/app/configs"

export SERVICE_NAME="basic-minter"
export SERVICE_VERSION="0.0.11"
export SERVICE_DESC="Basic Jwt Minting Service"

export SERVICE_ACCT="MINT"
export SERVICE_USER_CREDS="${SECRET_STORE}/${SERVICE_ACCT}/user-minter.creds"

export SERVICE_ACCT_SIGNK="${SECRET_STORE}/${SERVICE_ACCT}/acct-signing-key.nk"
export SERVICE_ACCT_XK="${SECRET_STORE}/${SERVICE_ACCT}/acct-encryption-key.xk"


nats-iam-broker $@ ${CONFIG_DIR}/*.yaml  ${SCRIPT_DIR}/rbac.yaml &

