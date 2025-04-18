#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/basic"

JWT_DWHO=$(idp_oidctest_login_public dwho dwho)
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test "pubsub basic.public.ZHdobw== hello-from-dwho"

JWT_RTYLER=$(idp_oidctest_login_public rtyler rtyler)
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_RTYLER}" \
    -run-test "pubsub basic.public.cnR5bGVy hello-from-rtyler"

# jetstream test
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt "${JWT_DWHO}" \
    -run-test "stream basic_test_stream basic.public.ZHdobw==" \
    -log=trace
