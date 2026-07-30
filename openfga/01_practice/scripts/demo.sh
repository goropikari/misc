#!/usr/bin/env bash
set -euo pipefail

api_url="${OPENFGA_API_URL:-http://localhost:8080}"
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

wait_for_server() {
  echo "Waiting for OpenFGA at ${api_url} ..."
  for _ in {1..30}; do
    if curl --silent --fail "${api_url}/healthz" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "OpenFGA did not become ready" >&2
  exit 1
}

json_value() {
  python3 -c 'import json, sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

wait_for_server

store_id="$(curl --silent --fail --request POST "${api_url}/stores" \
  --header 'Content-Type: application/json' \
  --data '{"name":"openfga-sample"}' | json_value id)"
echo "Created store: ${store_id}"

model_json="$(docker run --rm \
  --volume "${root_dir}:/workspace:ro" \
  openfga/cli:v0.7.14 \
  model transform --file /workspace/model.fga --output-format json)"

model_id="$(curl --silent --fail --request POST "${api_url}/stores/${store_id}/authorization-models" \
  --header 'Content-Type: application/json' \
  --data-binary "${model_json}" | json_value authorization_model_id)"
echo "Created authorization model: ${model_id}"

curl --silent --fail --request POST "${api_url}/stores/${store_id}/write" \
  --header 'Content-Type: application/json' \
  --data '{
    "writes": {
      "tuple_keys": [
        {"user":"user:alice","relation":"viewer","object":"folder:engineering"},
        {"user":"user:bob","relation":"editor","object":"folder:engineering"},
        {"user":"folder:engineering","relation":"parent","object":"document:roadmap"}
      ]
    },
    "authorization_model_id": "'"${model_id}"'"
  }' >/dev/null

check() {
  local user="$1" relation="$2" object="$3"
  local result
  result="$(curl --silent --fail --request POST "${api_url}/stores/${store_id}/check" \
    --header 'Content-Type: application/json' \
    --data '{"tuple_key":{"user":"'"${user}"'","relation":"'"${relation}"'","object":"'"${object}"'"},"authorization_model_id":"'"${model_id}"'"}' \
    | json_value allowed)"
  printf '%-24s %-8s %-22s => %s\n' "${user}" "${relation}" "${object}" "${result}"
}

echo
echo "Authorization checks:"
check user:alice viewer document:roadmap
check user:bob editor document:roadmap
check user:charlie viewer document:roadmap
