#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-mock-toolkit.sh

NATS_URL="nats://localhost:${NATS_PORT:-4222}"
SECRET_STORE="/usr/src/app/secrets/red"

printf "Red department:\n"

JWT_DWHO=$(idp_mock_login)
printf "Running test 1: mock dwho\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT_64/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test pubsub red.mockclientid.ZHdobw== hello-from-dwho
