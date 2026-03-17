#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-mock-toolkit.sh

NATS_URL="nats://localhost:${NATS_PORT:-4222}"
SECRET_STORE="/usr/src/app/secrets/blue"

printf "\nBlue department:\n"

JWT_MSMITH=$(idp_mock_login)
printf "Running test 1: mock msmith\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT_11/user-nobody.creds \
    -jwt "${JWT_MSMITH}" \
    -run-test pubsub blue.mockclientid.other hello-from-other
