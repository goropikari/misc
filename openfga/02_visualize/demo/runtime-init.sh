#!/bin/sh
set -eu

base=http://openfga:8080
echo "Waiting for OpenFGA..."
until curl -fsS "$base/healthz" >/dev/null; do sleep 1; done

stores=$(curl -fsS "$base/stores?page_size=100")
store_id=$(printf '%s' "$stores" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)
if [ -z "$store_id" ]; then echo "Could not find imported store: $stores" >&2; exit 1; fi

printf '%s\n' "$store_id" > /run/fga/store-id
echo "Demo store=$store_id"
