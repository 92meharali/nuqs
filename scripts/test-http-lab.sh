#!/usr/bin/env bash
# Builds Nuqs and compares its report for the loopback lab services with the golden file.
set -euo pipefail
cd "$(dirname "$0")/.."

workdir=$(mktemp -d)
compose=(docker compose -f lab/docker-compose.yml -p "nuqs-http-lab-$$")
cleanup() {
  "${compose[@]}" down
  rm -r -- "$workdir"
}
trap cleanup EXIT
go build -o "$workdir/nuqs" ./cmd/nuqs
"${compose[@]}" up -d --wait --wait-timeout 60
"$workdir/nuqs" scan --json -Pn -p 12323,18080 127.0.0.1 > "$workdir/report.json"
go run ./lab < "$workdir/report.json" > "$workdir/normalized.json"
diff -u lab/expected-report.json "$workdir/normalized.json"
echo "Lab report matches lab/expected-report.json"
