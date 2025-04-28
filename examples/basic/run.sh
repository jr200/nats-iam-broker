#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../../scripts/nats-toolkit.sh

# bootstrap a new NATS server with operator 'basic'
create_new_nats_config "nats://localhost:4222" basic
start_nats

# create accounts/users
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/basic_initial_setup.sh

# debug: save system_account creds for inspection
nats context save \
    --nsc=nsc://basic/SYS/sys \
    --description "system account" \
    --select \
    system_account

# start the micro-service
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/basic_start_service.sh "${@}"

# test login
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/basic_simulate_login.sh
