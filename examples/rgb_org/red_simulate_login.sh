#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/red"

printf "Red department:\n"

JWT_DWHO=$(idp_oidctest_login_public dwho dwho)
printf "Running test 1: oidc public dwho\n"
test-client \
    -url ${NATS_URL} \
    -creds ${SECRET_STORE}/MINT_64/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test pubsub red.public.ZHdobw== hello-from-dwho
