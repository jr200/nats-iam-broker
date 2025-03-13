#!/bin/bash
set -e

# Function to show usage
show_usage() {
  cat <<EOF
Usage: debug.sh [command]
Commands:
  shell                      Start a bash shell
  nsc [args]                 Run 'nsc' with provided arguments
  nats [args]                Run 'nats' with provided arguments
EOF
  exit 1
}


# Ensure a command is provided
if [ -z "$1" ]; then
  echo "Error: No command provided."
  show_usage
fi

# Parse the first argument as the command
case "$1" in
  shell)
    echo "Starting a bash shell..."
    exec bash
    ;;

  nsc)
    if [ $# -lt 2 ]; then
      echo "Error: Missing arguments for 'nsc'"
      show_usage
    fi
    exec nsc "${@:2}"
    ;;

  nats)
    if [ $# -lt 2 ]; then
      echo "Error: Missing arguments for 'nats'"
      show_usage
    fi
    exec nats "${@:2}"
    ;;

esac
