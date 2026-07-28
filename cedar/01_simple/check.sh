#!/usr/bin/env bash
set -euo pipefail

if ! command -v cedar >/dev/null 2>&1; then
  echo "cedar CLI が見つかりません。README.md の手順でインストールしてください。" >&2
  exit 1
fi

authorize() {
  cedar authorize \
    --policies policy.cedar \
    --schema schema.cedarschema \
    --entities entities.json \
    --principal "User::\"$1\"" \
    --action "Action::\"$2\"" \
    --resource 'Document::"report"'
}

assert_decision() {
  local expected="$1"
  shift
  local actual
  local status
  set +e
  actual="$(authorize "$@" 2>&1)"
  status=$?
  set -e
  actual="$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  if [[ "$actual" != "$expected" ]]; then
    echo "期待値=$expected 実際=$actual ($*, exit=$status)" >&2
    exit 1
  fi
  echo "OK: $* => $actual"
}

assert_decision allow alice View report
assert_decision allow alice Edit report
assert_decision allow bob View report
assert_decision deny bob Edit report
