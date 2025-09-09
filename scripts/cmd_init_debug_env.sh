#!/bin/bash

set -e

dotenv () {
  set -a
  [ -f ${ENV_FILE} ] && . ${ENV_FILE}
  set +a
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE=${SCRIPT_DIR}/../.env.local
TMP_DIR=/tmp/nats-iam-broker

mkdir -p ${TMP_DIR}

echo Script Dir: $SCRIPT_DIR
echo ENV File: ${ENV_FILE}
dotenv

jq -n \
    --arg name "${ACCOUNT_NAME}" \
    --arg pub "{{ readFile \"${IAM_SECRETS_DIR}/${ACCOUNT_NAME}-id-1.pub\" }}" \
    --arg sk "{{ readFile \"${IAM_SECRETS_DIR}/${ACCOUNT_NAME}-sk-1.nk\" }}" \
    '{rbac: {user_accounts: [{name: $name, public_key: $pub, signing_nkey: $sk}] }}' \
    > "${TMP_DIR}/user_accounts-${ACCOUNT_NAME}.yaml"

printenv | sort