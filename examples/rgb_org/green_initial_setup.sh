#!/bin/bash

SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}"/../../scripts/nsc-toolkit.sh

# shellcheck disable=SC2034
SECRET_STORE="/usr/src/app/secrets"

# # ### Setup application accounts as per the diagram

# setup green department
create_minting_account green MINT_01
create_minting_account green MINT_02
create_plain_account green APP7

# Push the app accounts up to the server.
# Only push if the script is run directly, not when sourced
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  # nsc push -A
  nsc push -A
fi
