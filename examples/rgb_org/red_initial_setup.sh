#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nsc-toolkit.sh

SECRET_STORE="/usr/src/app/secrets"

# # ### Setup application accounts as per the diagram

# setup red department
create_minting_account red MINT_64
create_plain_account red APP30

# Push the app accounts up to the server.
nsc push -A
