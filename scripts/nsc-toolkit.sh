#!/bin/bash

# ### Declare some useful helper functions

function extract_signing_key() {
  local account=$1
  sk=$(nsc describe account ${account} --field 'nats.signing_keys[0]' | tr -d '"')
  cat "/root/.local/share/nats/nsc/keys/keys/${sk:0:1}/${sk:1:2}/${sk}.nk"
}

function create_plain_account() {
  local department=$1
  local account=$2
  local path="${SECRET_STORE}/${department}/${account}"
  mkdir -p ${path}

  nsc add account "${account}"
  nsc edit account "${account}" --sk generate

  extract_signing_key ${account} > ${path}/acct-signing-key.nk
  nsc describe account ${account} --field sub | jq -r > ${path}/acct-pubkey.pub
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
  mkdir -p ${path}

  nsc add account "${account}"
  nsc edit account "${account}" --sk generate

  nsc add user --account ${account} --name nobody --deny-pubsub ">"
  nsc add user --account ${account} --name minter

	nsc generate nkey --curve 2>  ${path}/acct-encryption-key.xk

  minter_pubkey=`nsc describe user --account ${account} --name minter --field sub | jq -r`
  encryption_pubkey=`sed -n 2,1p ${path}/acct-encryption-key.xk`

  nsc edit authcallout \
    --account ${account} \
    --auth-user ${minter_pubkey} \
    --allowed-account '*' \
    --curve ${encryption_pubkey}

  extract_signing_key ${account} > ${path}/acct-signing-key.nk
  nsc generate creds --account ${account} --name minter > ${path}/user-minter.creds
  nsc generate creds --account ${account} --name nobody > ${path}/user-nobody.creds
}

function create_user() {
  local department=$1
  local account=$2
  local user=$3
  local path="${SECRET_STORE}/${department}/${account}"
  mkdir -p ${path}

  nsc add user --account "${account}" --name "${user}"
  nsc generate creds --account ${account} --name ${user} > ${path}/user-${user}.creds

  save_name=`echo ${department}_${account}_${user} | awk '{print tolower($0)}'`
  nats context save \
      --creds=${path}/user-${user}.creds \
      --description "${department}: ${account}/${user}" \
      "${save_name}"
}
