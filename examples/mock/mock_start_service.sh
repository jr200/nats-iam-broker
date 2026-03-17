#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")

export NATS_URL="nats://localhost:${NATS_PORT:-4222}"
export SECRET_STORE="/usr/src/app/secrets/mock"
CONFIG_DIR="/usr/src/app/configs"

export IAM_NAME="mock-minter"
export IAM_VERSION="0.0.11"
export IAM_DESCRIPTION="Mock Jwt Minting Service"

export IAM_ACCOUNT_NAME="MINT"
export IAM_USER_CREDS="${SECRET_STORE}/${IAM_ACCOUNT_NAME}/user-minter.creds"

export IAM_ACCOUNT_SIGNK="${SECRET_STORE}/${IAM_ACCOUNT_NAME}/acct-signing-key.nk"
export IAM_SERVICE_XK="${SECRET_STORE}/${IAM_ACCOUNT_NAME}/acct-encryption-key.xk"


nats-iam-broker serve "${@}" ${CONFIG_DIR}/env_minter.yaml ${CONFIG_DIR}/idp_mock.yaml "${SCRIPT_DIR}"/rbac.yaml &
