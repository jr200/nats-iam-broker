#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-mock-toolkit.sh

NATS_URL="nats://localhost:${NATS_PORT:-4222}"
SECRET_STORE="/usr/src/app/secrets/basic"

sleep 1
printf "\nRunning test cases...\n"

JWT_DWHO=$(idp_mock_login)
printf "Running test 1: mock dwho\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test pubsub basic.mockclientid.ZHdobw== hello-from-dwho

JWT_RTYLER=$(idp_mock_login)
printf "\nRunning test 2: mock rtyler\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_RTYLER}" \
    -run-test pubsub basic.mockclientid.cnR5bGVy hello-from-rtyler

printf "\nRunning test 3: mock jetstream\n"
# jetstream test
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -log trace \
    -run-test stream basic_test_stream basic.mockclientid.ZHdobw==
