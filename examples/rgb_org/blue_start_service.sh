#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")

export NATS_URL="nats://localhost:4222"

export SECRET_STORE="/usr/src/app/secrets/blue"
CONFIG_DIR="/usr/src/app/configs"

export SERVICE_NAME="blue-minter"
export SERVICE_VERSION="0.0.11"
export SERVICE_DESC="Blue Department Jwt Minting Service"

export SERVICE_USER_CREDS="${SECRET_STORE}/MINT_11/user-minter.creds"
export SERVICE_ACCT_SIGNK="${SECRET_STORE}/MINT_11/acct-signing-key.nk"
export SERVICE_ACCT_XK="${SECRET_STORE}/MINT_11/acct-encryption-key.xk"

nats-iam-broker $@ ${CONFIG_DIR}/*.yaml  ${SCRIPT_DIR}/blue_rbac.yaml &
