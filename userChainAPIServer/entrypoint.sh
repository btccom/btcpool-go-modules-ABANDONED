#!/bin/bash
# Generate configuration files from environment variables and start processes
set -e

userChainAPIServer.php >/tmp/config.json

echo "================ config.json ================"
cat /tmp/config.json
echo "============================================="

exec userChainAPIServer -config /tmp/config.json "$@"
