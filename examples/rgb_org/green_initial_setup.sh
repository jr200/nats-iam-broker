#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nsc-toolkit.sh


SECRET_STORE="/usr/src/app/secrets"

# # ### Setup application accounts as per the diagram

# setup green department
create_minting_account green MINT_01
create_minting_account green MINT_02
create_plain_account green APP7

# Push the app accounts up to the server.
nsc push -A
