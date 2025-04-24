#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/idp-mock-toolkit.sh

NATS_URL="nats://localhost:4222"
SECRET_STORE="/usr/src/app/secrets/mock"

JWT_MOCK=`idp_mock_login`

test-client \
    -url=${NATS_URL} \
    -creds=${SECRET_STORE}/MINT/user-nobody.creds \
    -jwt $JWT_MOCK \
    -run-test pubsub mock.mockclientid.Ym9iQGFjbWUuY29t hello-from-bob

