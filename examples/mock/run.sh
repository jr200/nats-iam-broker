#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nats-toolkit.sh

# start oidc server
echo starting OIDC-SERVER
oidc-server start -c /usr/src/app/oidc-server/config.yaml -p 5550 &

# bootstrap a new NATS server with operator 'mock'
create_new_nats_config "nats://localhost:4222" mock
start_nats

# create accounts/users
source ./examples/mock/mock_initial_setup.sh

# debug: save system_account creds for inspection
nats context save \
    --nsc=nsc://mock/SYS/sys \
    --description "system account" \
    --select \
    system_account

# start the micro-service
source ./examples/mock/mock_start_service.sh $@

# test login
source ./examples/mock/mock_simulate_login.sh

