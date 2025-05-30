#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/blue"

printf "\nBlue department:\n"

JWT_MSMITH=$(idp_oidctest_login_public msmith msmith)
printf "Running test 1: oidc public msmith\n"
test-client \
    -url ${NATS_URL} \
    -creds=${SECRET_STORE}/MINT_11/user-nobody.creds \
    -jwt "${JWT_MSMITH}" \
    -run-test pubsub blue.public.other hello-from-other
