#!/bin/bash

SCRIPT_DIR=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")
source ${SCRIPT_DIR}/../../scripts/nsc-toolkit.sh

SECRET_STORE="/usr/src/app/secrets"

# # ### Setup application accounts as per the diagram
# setup blue department
create_minting_account blue MINT_11
create_plain_account blue APP1
create_user blue APP1 admin
create_plain_account blue APP2
create_plain_account blue APP3

# Push the app accounts up to the server.
nsc push -A

