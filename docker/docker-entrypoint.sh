#!/bin/bash

# function to set env, or default if not specified
set_env_or_default() {
    local var_name=$1
    local default_value=$2
    if [ -z "${!var_name}" ]; then
        export "$var_name=$default_value"
    fi
}

# Help function to display environment variable descriptions
help() {
    echo "Environment Variables:"
    echo "Required:"
    echo "  IAM_ACCOUNT_NAME    Account name for the IAM service"
    echo "  IAM_VERSION         Version of the IAM service"
    echo "  IDP_ISSUER_URL      Primary Idp Issuer URL"
    echo "  IDP_CLIENT_ID       Primary Idp ClientIds"
    echo ""
    echo "Optional:"
    echo "  NATS_URL            NATS server URL (default: nats://localhost:4222)"
    echo "  IDP_DESCRIPTION     Primary Idp Description (default: 'Primary Identity Provider')"
    echo "  IAM_STARTUP_DELAY   Delay before starting service (default: 10s)"
    echo "  IAM_PORT_FORWARDS   CSVs of port-forwards; item format: <ext_host>:<ext_port>:<container_port>"
    echo "  IAM_CONFIGS         Glob for config files (default: /config/*.yaml)"
    echo "  IAM_NAME            Service name (default: <iam_account_name>-iam-broker-svc)"
    echo "  IAM_DESCRIPTION     Service description (default: <iam_account_name> IAM Broker)"
    echo "  IAM_SECRETS_DIR     Directory containing secrets (default: /secrets)"
    echo "  IAM_SERVICE_XK      Path to encryption key (default: /secrets/<iam_account_name>-enc.xk)"
    echo "  IAM_JWT_EXP         Expiration time of minted NATS JWT (default: 1h15m)"
    echo "  IAM_ACCOUNT_SIGNK   Path to signing key (default: /secrets/<iam_account_name>-sk-1.nk)"
    echo "  IAM_USER_CREDS      Path to user credentials (default: /secrets/<iam_account_name>-ac-user.creds)"
    echo "  APP_AUTO_ACCOUNTS   CSVs of accounts to be minted. Sets env-vars APP_ACCT_<idx>, APP_SK_<idx>, APP_ID_<idx>"
}

# Show help if requested
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    help
    exit 0
fi

if [ -z "${IAM_ACCOUNT_NAME}" ] || [ -z "${IAM_VERSION}" ]; then
    echo "Error: Required environment variables must be set"
    echo "Run with --help for more information"
    exit 1
fi

IAM_CONFIGS=${IAM_CONFIGS:-'/config/*.yaml'}

if ! compgen -G "${IAM_CONFIGS}" > /dev/null; then
    echo "Error: No configuration files found matching ${IAM_CONFIGS}"
    echo "Run with --help for more information"
    exit 1
fi

# Initialise optional environment variables with defaults
set_env_or_default NATS_URL "nats://localhost:4222"
set_env_or_default IDP_DESCRIPTION "Primary Identity Provider"
set_env_or_default IAM_STARTUP_DELAY "10s"
set_env_or_default IAM_JWT_EXP "1h15m"
set_env_or_default IAM_NAME "${IAM_ACCOUNT_NAME}-iam-broker-svc"
set_env_or_default IAM_DESCRIPTION "${IAM_ACCOUNT_NAME} JWT IAM Service"
set_env_or_default IAM_SECRETS_DIR "/secrets"
set_env_or_default IAM_SERVICE_XK "${IAM_SECRETS_DIR}/${IAM_ACCOUNT_NAME}-enc.xk"
set_env_or_default IAM_ACCOUNT_SIGNK "${IAM_SECRETS_DIR}/${IAM_ACCOUNT_NAME}-sk-1.nk"
set_env_or_default IAM_USER_CREDS "${IAM_SECRETS_DIR}/${IAM_ACCOUNT_NAME}-ac-user.creds"

# Auto-generate accounts if APP_AUTO_ACCOUNTS is set

# Create temp directory for all account configs
AUTOGEN_CONFIGS_DIR=$(mktemp -d)
num_auto_accounts=0

if [ -n "${APP_AUTO_ACCOUNTS}" ]; then
    
    IFS=',' read -ra ACCOUNTS <<< "${APP_AUTO_ACCOUNTS}"
    for account in "${ACCOUNTS[@]}"; do
        echo "Setting expanded environment variables for ${account}"

        # create user_accounts config for account using jq
        jq -n \
           --arg name "$account" \
           --arg pub "{{ readFile \"${IAM_SECRETS_DIR}/${account}-id-1.pub\" | trim }}" \
           --arg sk "{{ readFile \"${IAM_SECRETS_DIR}/${account}-sk-1.nk\" | trim }}" \
           '{rbac: {user_accounts: [{name: $name, public_key: $pub, signing_nkey: $sk}]}}' \
           > "${AUTOGEN_CONFIGS_DIR}/user_accounts-${account}.yaml"
        num_auto_accounts=$((num_auto_accounts + 1))
    done

    echo "Wrote ${num_auto_accounts} account configuration files to ${AUTOGEN_CONFIGS_DIR}"
fi

echo "Waiting ${IAM_STARTUP_DELAY} before startup..."
sleep "${IAM_STARTUP_DELAY}"

# port-forward endpoints defined in IAM_PORT_FORWARDS
# csvs of: external_hostname:external_port:container_port)
# e.g., nats:4222:4222,oidc:443:443
IFS=',' read -ra ENDPOINTS <<< "${IAM_PORT_FORWARDS}"
for endpoint in "${ENDPOINTS[@]}"; do
    IFS=':' read -r external_host external_port container_port <<< "${endpoint}"
    echo "Port-forwarding ${external_host}:${external_port} to localhost:${container_port}"
    nc -lk -p "${container_port}" -e nc "${external_host}" "${external_port}" &
done

if [ "${DEBUG}" = "1" ]; then
    env | sort
fi

# start the nats-iam-broker
echo "[CMD]" "nats-iam-broker" "$@" "${IAM_CONFIGS}" "${AUTOGEN_CONFIGS_DIR}/*.yaml"

if [ ${num_auto_accounts} -gt 0 ]; then
    # shellcheck disable=SC2086 # Allow word splitting and glob expansion for user args ($@) and config path (${IAM_CONFIGS})
    nats-iam-broker $@ ${IAM_CONFIGS} ${AUTOGEN_CONFIGS_DIR}/*.yaml
else
    # shellcheck disable=SC2086 # Allow word splitting and glob expansion for user args ($@) and config path (${IAM_CONFIGS})
    nats-iam-broker $@ ${IAM_CONFIGS}
fi
