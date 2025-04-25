#!/bin/bash

export NATS_URL="nats://localhost:4222"

export SECRET_STORE="/usr/src/app/secrets/red"
export CONFIG_DIR="/usr/src/app/configs"

export SERVICE_NAME="red-minter"
export SERVICE_VERSION="0.0.64"
export SERVICE_DESC="Red Department Jwt Minting Service"

export SERVICE_USER_CREDS="${SECRET_STORE}/MINT_64/user-minter.creds"
export SERVICE_ACCT_SIGNK="${SECRET_STORE}/MINT_64/acct-signing-key.nk"
export SERVICE_ACCT_XK="${SECRET_STORE}/MINT_64/acct-encryption-key.xk"

nats-iam-broker "${@}" ${CONFIG_DIR}/*.yaml "${SCRIPT_DIR}"/red_rbac.yaml &
