#!/bin/bash

# ### Declare some useful helper functions
function create_new_nats_config() {
  local nats_url=$1
  local operator=$2
  local nats_port=${NATS_PORT:-4222}
  local nats_http_port=${NATS_HTTP_PORT:-8222}
  local nats_ws_port=${NATS_WS_PORT:-8080}
  mkdir -p /usr/src/app/jsdata

  nsc add operator --generate-signing-key --sys --name "${operator}"

  # A follow-up edit of the operator enforces signing keys are used for
  # accounts as well. Setting the server URL is a convenience so that
  # it does not need to be specified with call `nsc push`.
  nsc edit operator \
    --require-signing-keys \
    --account-jwt-server-url "${nats_url}"

  # This command generates the bit of configuration to be used by the server
  # to setup the embedded JWT resolver.
  nsc generate config \
    --nats-resolver \
    --sys-account SYS > resolver.conf

  # Create the most basic config that simply includes the generated
  # resolver config.
  cat <<- EOF > server.conf
  server_name: "test_server"
  logtime: true
  debug: false
  trace: false
  port: ${nats_port}
  http_port: ${nats_http_port}
  jetstream {
      store_dir: /usr/src/app/jsdata
      max_mem: 32M
      max_file: 32M
  }
  websocket: {
    port: ${nats_ws_port}
    no_tls: true
  }
  include resolver.conf
EOF

}

function start_nats() {
  local mode=${1:-bg}

  if [[ "$mode" == "fg" ]]; then
    echo "Starting nats-server in foreground mode..."
    nats-server -c server.conf 2>&1
  else
    echo "Starting nats-server in background mode..."
    nats-server -c server.conf 2>&1 &
  fi
}
