#!/bin/sh
set -eu

if [ -z "${OPENFGA_STORE_ID:-}" ] && [ -f /run/fga/store-id ]; then
  export OPENFGA_STORE_ID=$(cat /run/fga/store-id)
fi
exec /fgalens "$@"
