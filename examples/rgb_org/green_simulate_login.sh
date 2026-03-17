#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-mock-toolkit.sh

NATS_URL="nats://localhost:${NATS_PORT:-4222}"
SECRET_STORE="/usr/src/app/secrets/green"

# using green-minter-01

printf "\nGreen department:\n"

JWT_RTYLER=$(idp_mock_login)
printf "Running test 1: mock rtyler\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT_01/user-nobody.creds \
    -jwt "${JWT_RTYLER}" \
    -run-test pubsub green.mockclientid.rtyler hello-from-rtyler

# using green-minter-02

JWT_RTYLER=$(idp_mock_login)
printf "\nRunning test 2: mock rtyler\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT_02/user-nobody.creds \
    -jwt "${JWT_RTYLER}" \
    -run-test pubsub green.mockclientid.rtyler hello-from-rtyler
