#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../../scripts/nsc-toolkit.sh

# shellcheck disable=SC2034
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

