#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

export NATS_URL="nats://localhost:4222"

export SECRET_STORE="/usr/src/app/secrets/green"
CONFIG_DIR="/usr/src/app/configs"

export SERVICE_NAME="green-minter-01"
export SERVICE_VERSION="0.0.1"
export SERVICE_DESC="Green Department Jwt Minting Service (primary)"
export SERVICE_USER_CREDS="${SECRET_STORE}/MINT_01/user-minter.creds"
export SERVICE_ACCT_SIGNK="${SECRET_STORE}/MINT_01/acct-signing-key.nk"
export SERVICE_ACCT_XK="${SECRET_STORE}/MINT_01/acct-encryption-key.xk"

nats-iam-broker "${@}" ${CONFIG_DIR}/env_minter.yaml ${CONFIG_DIR}/idp_public.yaml "${SCRIPT_DIR}"/green_rbac.yaml &

# start the secondary minter

export SERVICE_NAME="green-minter-02"
export SERVICE_VERSION="0.0.2"
export SERVICE_DESC="Green Department Jwt Minting Service (secondary)"
export SERVICE_USER_CREDS="${SECRET_STORE}/MINT_02/user-minter.creds"
export SERVICE_ACCT_SIGNK="${SECRET_STORE}/MINT_02/acct-signing-key.nk"
export SERVICE_ACCT_XK="${SECRET_STORE}/MINT_02/acct-encryption-key.xk"

nats-iam-broker "${@}" ${CONFIG_DIR}/env_minter.yaml ${CONFIG_DIR}/idp_private.yaml "${SCRIPT_DIR}"/green_rbac.yaml &
