#!/usr/bin/env bash
# Runs the HTTP golden report test against a disposable local Compose service.
set -euo pipefail
cd "$(dirname "$0")/.."

compose=(docker compose -f lab/docker-compose.yml -p "nuqs-http-lab-$$")
trap '"${compose[@]}" down' EXIT
"${compose[@]}" up -d --wait --wait-timeout 60
address=$("${compose[@]}" port web 8080)
if [[ ! $address =~ ^127\.0\.0\.1:([1-9][0-9]*)$ ]]; then
  echo "Unexpected HTTP fixture address: $address" >&2
  exit 1
fi
go test -count=1 -v ./internal/cli -run '^TestHTTPReport$' -http-lab-port="${BASH_REMATCH[1]}"
