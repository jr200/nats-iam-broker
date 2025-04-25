#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../../scripts/nats-toolkit.sh

# bootstrap a new NATS server
create_new_nats_config "nats://localhost:4222" rgb_org
start_nats

# create accounts/users
# shellcheck source=/dev/null
source ./examples/rgb_org/red_initial_setup.sh
# shellcheck source=/dev/null
source ./examples/rgb_org/green_initial_setup.sh
# shellcheck source=/dev/null
source ./examples/rgb_org/blue_initial_setup.sh

# debug: save system_account creds for inspection
nats context save \
    --nsc=nsc://rgb_org/SYS/sys \
    --description "system account" \
    --select \
    system_account

# start auth callout micro-services
# shellcheck source=/dev/null
source ./examples/rgb_org/red_start_service.sh "${@}"
# shellcheck source=/dev/null
source ./examples/rgb_org/green_start_service.sh "${@}"
# shellcheck source=/dev/null
source ./examples/rgb_org/blue_start_service.sh "${@}"

sleep 2

# test login
# shellcheck source=/dev/null
source ./examples/rgb_org/red_simulate_login.sh

# shellcheck source=/dev/null
source ./examples/rgb_org/green_simulate_login.sh

# shellcheck source=/dev/null
source ./examples/rgb_org/blue_simulate_login.sh
