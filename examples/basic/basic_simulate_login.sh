#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/basic"

sleep 1
printf "\nRunning test cases...\n"

JWT_DWHO=$(idp_oidctest_login_public dwho dwho)
printf "Running test 1: oidc public dwho\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test pubsub basic.public.ZHdobw== hello-from-dwho

JWT_RTYLER=$(idp_oidctest_login_public rtyler rtyler)
printf "\nRunning test 2: oidc public rtyler\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_RTYLER}" \
    -run-test pubsub basic.public.cnR5bGVy hello-from-rtyler

printf "\nRunning test 3: oidc public jetstream\n"
# jetstream test
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -log trace \
    -run-test stream basic_test_stream basic.public.ZHdobw==
