#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source ${SCRIPT_DIR}/cmd_init_debug_env.sh

BIN="${SCRIPT_DIR}/../build/nats-iam-broker-darwin-arm64"

ARGS=( \
    -log trace \
    -log-sensitive \
    -log-human \
    "$@"
)

if [[ -n "${DLV:-}" ]]; then
echo "Starting with Delve headless at ${DLV_ADDR:-127.0.0.1:38697}"
exec dlv --headless --listen="${DLV_ADDR:-127.0.0.1:38697}" --api-version=2 --accept-multiclient exec "${BIN}" -- "${ARGS[@]}"
else
exec "${BIN}" "${ARGS[@]}"
fi
