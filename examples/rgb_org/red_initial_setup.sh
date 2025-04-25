#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../../scripts/nsc-toolkit.sh

# shellcheck disable=SC2034
SECRET_STORE="/usr/src/app/secrets"

# # ### Setup application accounts as per the diagram

# setup red department
create_minting_account red MINT_64
create_plain_account red APP30

# Push the app accounts up to the server.
nsc push -A
