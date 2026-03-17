#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

export NATS_URL="nats://localhost:4222"

export SECRET_STORE="/usr/src/app/secrets/blue"
CONFIG_DIR="/usr/src/app/configs"

export IAM_NAME="blue-minter"
export IAM_VERSION="0.0.11"
export IAM_DESCRIPTION="Blue Department Jwt Minting Service"

export IAM_USER_CREDS="${SECRET_STORE}/MINT_11/user-minter.creds"
export IAM_ACCOUNT_SIGNK="${SECRET_STORE}/MINT_11/acct-signing-key.nk"
export IAM_SERVICE_XK="${SECRET_STORE}/MINT_11/acct-encryption-key.xk"

nats-iam-broker "${@}" ${CONFIG_DIR}/*.yaml "${SCRIPT_DIR}"/blue_rbac.yaml &
