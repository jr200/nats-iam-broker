#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nsc-toolkit.sh

SECRET_STORE="/usr/src/app/secrets"

# setup mock accounts
create_minting_account mock MINT
create_plain_account mock APP1
account_enable_jetstream mock APP1

create_user mock APP1 debug

# Push the app accounts up to the server.
nsc push -A
