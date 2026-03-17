#!/bin/bash

export NATS_URL="nats://localhost:${NATS_PORT:-4222}"

export SECRET_STORE="/usr/src/app/secrets/red"
export CONFIG_DIR="/usr/src/app/configs"

export IAM_NAME="red-minter"
export IAM_VERSION="0.0.64"
export IAM_DESCRIPTION="Red Department Jwt Minting Service"

export IAM_USER_CREDS="${SECRET_STORE}/MINT_64/user-minter.creds"
export IAM_ACCOUNT_SIGNK="${SECRET_STORE}/MINT_64/acct-signing-key.nk"
export IAM_SERVICE_XK="${SECRET_STORE}/MINT_64/acct-encryption-key.xk"

nats-iam-broker serve "${@}" ${CONFIG_DIR}/*.yaml "${SCRIPT_DIR}"/red_rbac.yaml &
