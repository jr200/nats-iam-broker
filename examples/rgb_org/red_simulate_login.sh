#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../scripts/idp-oidctest-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/red"

JWT_DWHO=`idp_oidctest_login_public dwho dwho`
test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT_64/user-nobody.creds \
    -jwt $JWT_DWHO \
    -run-test pubsub red.public.ZHdobw== hello-from-dwho
