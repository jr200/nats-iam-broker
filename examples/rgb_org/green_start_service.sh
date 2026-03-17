#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

export NATS_URL="nats://localhost:4222"

export SECRET_STORE="/usr/src/app/secrets/green"
CONFIG_DIR="/usr/src/app/configs"

export IAM_NAME="green-minter-01"
export IAM_VERSION="0.0.1"
export IAM_DESCRIPTION="Green Department Jwt Minting Service (primary)"
export IAM_USER_CREDS="${SECRET_STORE}/MINT_01/user-minter.creds"
export IAM_ACCOUNT_SIGNK="${SECRET_STORE}/MINT_01/acct-signing-key.nk"
export IAM_SERVICE_XK="${SECRET_STORE}/MINT_01/acct-encryption-key.xk"

nats-iam-broker serve "${@}" ${CONFIG_DIR}/env_minter.yaml ${CONFIG_DIR}/idp_public.yaml "${SCRIPT_DIR}"/green_rbac.yaml &

# start the secondary minter

export IAM_NAME="green-minter-02"
export IAM_VERSION="0.0.2"
export IAM_DESCRIPTION="Green Department Jwt Minting Service (secondary)"
export IAM_USER_CREDS="${SECRET_STORE}/MINT_02/user-minter.creds"
export IAM_ACCOUNT_SIGNK="${SECRET_STORE}/MINT_02/acct-signing-key.nk"
export IAM_SERVICE_XK="${SECRET_STORE}/MINT_02/acct-encryption-key.xk"

nats-iam-broker serve "${@}" ${CONFIG_DIR}/env_minter.yaml ${CONFIG_DIR}/idp_private.yaml "${SCRIPT_DIR}"/green_rbac.yaml &
