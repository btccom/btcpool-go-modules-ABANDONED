#!/bin/bash
# Generate configuration files from environment variables and start processes
set -e

chainSwitcher.php >/tmp/config.json

echo "================ config.json ================"
cat /tmp/config.json
echo "============================================="

exec chainSwitcher -config /tmp/config.json "$@"
