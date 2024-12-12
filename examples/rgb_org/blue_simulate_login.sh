#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/blue"

JWT_MSMITH=`idp_oidctest_login_public msmith msmith`
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT_11/user-nobody.creds \
    -jwt $JWT_MSMITH \
    -run-test "pubsub blue.public.other hello-from-other"
