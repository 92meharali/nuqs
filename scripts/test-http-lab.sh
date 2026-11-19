#!/usr/bin/env bash
# Checks the Compose HTTP fixture and the saved Telnet and FTPS inventories.
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
go test -count=1 -v ./internal/cli -run '^Test(HTTP|Telnet|FTPS)Report$' -http-lab-port="${BASH_REMATCH[1]}"
