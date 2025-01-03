#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/green"

# using green-minter-01

JWT_RTYLER=`idp_oidctest_login_public rtyler rtyler`
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT_01/user-nobody.creds \
    -jwt $JWT_RTYLER \
    -run-test "pubsub green.public.rtyler hello-from-rtyler"

# using green-minter-02

JWT_RTYLER=`idp_oidctest_login_private rtyler rtyler`
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT_02/user-nobody.creds \
    -jwt $JWT_RTYLER \
    -run-test "pubsub green.private.rtyler hello-from-rtyler"
