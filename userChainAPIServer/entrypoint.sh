#!/bin/bash
# Generate configuration files from environment variables and start processes
set -e

userChainAPIServer.php >/tmp/config.json

exec userChainAPIServer -config /tmp/config.json "$@"
