#!/bin/bash
SCRIPT_DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

VAULT_CMD=bao
VAULT_CMD=vault

# ### Declare some useful helper functions
function render_jq_template() {
  local template_file=$1
  shift
  local kv_pairs=("${@}")
  
  # Read the template file into a variable
  local template
  template=$(<"$template_file")

  # Iterate over the key-value pairs and perform the substitutions
  for kv in "${kv_pairs[@]}"; do
    local key
    key=$(echo "$kv" | cut -d= -f1)
    local value
    value=$(echo "$kv" | cut -d= -f2-)
    
    # Perform the substitution in the template using jq
    # Determine if the value is a JSON array or object
    if [[ "$value" =~ ^\[.*\]$ ]] || [[ "$value" =~ ^\{.*\}$ ]]; then
      template=$(echo "$template" | jq --argjson value "$value" "$key |= \$value")
    else
      template=$(echo "$template" | jq --arg value "$value" "$key |= \$value")
    fi
  done

  # Output the modified template to stdout
  echo "$template"
}

function create_user_minter() {
  local operator=$1
  local account=$2

  template_file="${SCRIPT_DIR}/templates/user.json.j2"

  # shellcheck disable=SC2016
  json_payload=$(render_jq_template "${template_file}" \
    ".useSigningKey=${account}-ask1" \
    '.claims.name=minter' \
    ".claims.user.pub.allow=[\">\"]" \
    '.claims.user.sub.allow=["_INBOX.>", "$SRV.>", "$SYS.REQ.USER.*"]' \
    )
	echo "${json_payload}" | $VAULT_CMD write "nats/issue/operator/${operator}/account/${account}/user/minter" -
}

function create_user_nobody() {
  local operator=$1
  local account=$2

  template_file="${SCRIPT_DIR}/templates/user.json.j2"

  json_payload=$(render_jq_template "${template_file}" \
    ".useSigningKey=${account}-ask1" \
    '.claims.name=nobody' \
    ".claims.user.pub.deny=[\">\"]" \
    ".claims.user.sub.deny=[\">\"]" \
    )
	echo "${json_payload}" | $VAULT_CMD write "nats/issue/operator/${operator}/account/${account}/user/nobody" -
}

function create_user_god() {
  local operator=$1
  local account=$2

  template_file="${SCRIPT_DIR}/templates/user.json.j2"

  json_payload=$(render_jq_template "${template_file}" \
    ".useSigningKey=${account}-ask1" \
    '.claims.name=god' \
    ".claims.user.pub.allow=[\">\"]" \
    ".claims.user.sub.allow=[\">\"]" \
    )
	echo "${json_payload}" | $VAULT_CMD write "nats/issue/operator/${operator}/account/${account}/user/god" -
}

function read_account_pubkey() {
  local operator=$1
  local account=$2

  account_pubkey=$($VAULT_CMD read -format=table -field=publicKey "nats/nkey/operator/${operator}/account/${account}")
  echo "${account_pubkey}"
}

function read_account_signing_key() {
  local operator=$1
  local account=$2

  account_signing_key=$($VAULT_CMD read -format=table -field=seed "nats/nkey/operator/${operator}/account/${account}")
  echo "${account_signing_key}"
}

function read_user_pubkey() {
  local operator=$1
  local account=$2
  local user=$3

  user_pubkey=$($VAULT_CMD read -format=table -field=publicKey "nats/nkey/operator/${operator}/account/${account}/user/${user}")
  echo "${user_pubkey}"
}

function read_user_creds() {
  local operator=$1
  local account=$2
  local user=$3

  local user_creds
  user_creds=$($VAULT_CMD read -format=table -field=creds "nats/creds/operator/${operator}/account/${account}/user/${user}")
  echo "${user_creds}"
}

function save_nats_context() {
  local operator=$1
  local account=$2
  local user=$3
  shift 3 # Shift the positional parameters to the left, so $@ will contain the rest

  local path=${NATS_CREDS_STORE:-~/.nats}
  mkdir -p "${path}"

  creds=$(read_user_creds "${operator}" "${account}" "${user}")
  echo "${creds}" > "${path}/u-${account}-${user}.creds"

  save_name=$(echo "vault-nats_${account}_${user}" | awk '{print tolower($0)}')

  # Pass all additional arguments using "$@"
  # hint: useful for passing --server flag
  nats context save "${save_name}" \
      --creds="${path}/u-${account}-${user}.creds" \
      --description "vault-nats: ${operator}/${account}/${user}" \
      "${@}"
}

function create_plain_account() {
  local operator=$1
  local account=$2
  template_file="${SCRIPT_DIR}/templates/account.json.j2"

  account_json=$(render_jq_template "${template_file}" \
    '.useSigningKey=opsk1' \
    ".claims.name=${account}" \
    ".claims.account.signingKeys=[\"${account}-ask1\"]" \
    )

	echo "${account_json}" | $VAULT_CMD write "nats/issue/operator/${operator}/account/${account}" -

  # account_signing_key=`read_account_signing_key ${operator} ${account}`
}

function create_minting_account() {
  local operator=$1
  local account=$2
  template_file="${SCRIPT_DIR}/templates/account.json.j2"

  create_user_minter "${operator}" "${account}"
  minter_pubkey=$(read_user_pubkey "${operator}" "${account}" minter)
  create_user_nobody "${operator}" "${account}"

	nsc generate nkey --curve > ./tmp/acct-encryption-key.xk
  encryption_pubkey=$(sed -n 2,1p ./tmp/acct-encryption-key.xk)

  account_json=$(render_jq_template "${template_file}" \
    '.useSigningKey=opsk1' \
    ".claims.name=${account}" \
    ".claims.account.signingKeys=[\"${account}-ask1\"]" \
    ".claims.account.authorization.auth_users=[\"${minter_pubkey}\"]" \
    '.claims.account.authorization.allowed_accounts=["*"]' \
    ".claims.account.authorization.xkey=${encryption_pubkey}" \
    )

	echo "${account_json}" | $VAULT_CMD write "nats/issue/operator/${operator}/account/${account}" -

  # account_signing_key=`read_account_signing_key ${operator} ${account}`
}
