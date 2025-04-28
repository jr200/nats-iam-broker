#!/bin/bash

# ### Declare some useful helper functions

function extract_signing_key() {
  local account=$1
  sk=$(nsc describe account "${account}" --field 'nats.signing_keys[0]' | tr -d '"')
  cat "/root/.local/share/nats/nsc/keys/keys/${sk:0:1}/${sk:1:2}/${sk}.nk"
}

function create_plain_account() {
  local department=$1
  local account=$2
  local path="${SECRET_STORE}/${department}/${account}"
  mkdir -p "${path}"

  nsc add account "${account}"
  nsc edit account "${account}" --sk generate

  extract_signing_key "${account}" > "${path}/acct-signing-key.nk"
  nsc describe account "${account}" --field sub | jq -r > "${path}/acct-pubkey.pub"
}

function account_enable_jetstream() {
  local department=$1
  local account=$2

  nsc edit account "${account}" \
    --js-mem-storage 1M \
    --js-disk-storage 1M  \
    --js-streams 10 \
    --js-consumer 100
}

function create_minting_account() {
  local department=$1
  local account=$2
  local path="${SECRET_STORE}/${department}/${account}"
  mkdir -p "${path}"

  nsc add account "${account}"
  nsc edit account "${account}" --sk generate

  nsc add user --account "${account}" --name nobody --deny-pubsub ">"
  nsc add user --account "${account}" --name minter

	nsc generate nkey --curve 2>  "${path}"/acct-encryption-key.xk

  minter_pubkey=$(nsc describe user --account "${account}" --name minter --field sub | jq -r)
  encryption_pubkey=$(sed -n 2,1p "${path}"/acct-encryption-key.xk)

  nsc edit authcallout \
    --account "${account}" \
    --auth-user "${minter_pubkey}" \
    --allowed-account '*' \
    --curve "${encryption_pubkey}"

  extract_signing_key "${account}" > "${path}/acct-signing-key.nk"
  nsc generate creds --account "${account}" --name minter > "${path}/user-minter.creds"
  nsc generate creds --account "${account}" --name nobody > "${path}/user-nobody.creds"
}

function create_user() {
  local department=$1
  local account=$2
  local user=$3
  local path="${SECRET_STORE}/${department}/${account}"
  mkdir -p "${path}"

  nsc add user --account "${account}" --name "${user}"
  nsc generate creds --account "${account}" --name "${user}" > "${path}/user-${user}.creds"

  save_name=$(echo "${department}_${account}_${user}" | awk '{print tolower($0)}')
  nats context save \
      --creds="${path}/user-${user}.creds" \
      --description "${department}: ${account}/${user}" \
      "${save_name}"
}

function show_context_creds() {
  local creds_name=$1
  local context_info
  local creds_file
  local exit_code

  # Attempt to get context info and capture exit code
  context_info=$(nats context info "${creds_name}" -j 2>/dev/null)
  exit_code=$?

  if [[ ${exit_code} -ne 0 ]]; then
    echo "Error: Failed to get info for context '${creds_name}'. Does it exist?" >&2
    return 1
  fi

  # Extract creds file path
  creds_file=$(echo "${context_info}" | jq -r .creds 2>/dev/null)
  exit_code=$?

   if [[ ${exit_code} -ne 0 ]]; then
    echo "Error: Failed to parse context info with jq for '${creds_name}'." >&2
    return 1
  fi

  # Check if creds file path is empty or null
  if [[ -z "$creds_file" || "$creds_file" == "null" ]]; then
    echo "Error: No credentials file found for context '${creds_name}'." >&2
    return 1
  fi

  # Check if the creds file actually exists
  if [[ ! -f "$creds_file" ]]; then
      echo "Error: Credentials file '${creds_file}' for context '${creds_name}' not found." >&2
      return 1
  fi

  # If all checks pass, output the credentials
  cat "${creds_file}"
}
