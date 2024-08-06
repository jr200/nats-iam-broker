#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nats-toolkit.sh

# bootstrap a new NATS server with operator 'basic'
create_new_nats_config "nats://localhost:4222" basic
start_nats

# create accounts/users
source ./examples/basic/basic_initial_setup.sh

# debug: save system_account creds for inspection
nats context save \
    --nsc=nsc://basic/SYS/sys \
    --description "system account" \
    --select \
    system_account

# start the micro-service
source ./examples/basic/basic_start_service.sh $@

# test login
source ./examples/basic/basic_simulate_login.sh

