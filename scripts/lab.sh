#!/usr/bin/env bash
# Builds Nuqs and compares its report for the loopback lab services with the golden file.
set -euo pipefail
cd "$(dirname "$0")/.."

workdir=$(mktemp -d)
compose=(docker compose -f lab/docker-compose.yml -p "nuqs-lab-$$")
cleanup() {
  local result=$1 teardown=0 ftp_id
  if (( result != 0 )); then
    echo "Lab failed (exit $result). FTP diagnostics before teardown:" >&2
    "${compose[@]}" ps --all ftp >&2 || true
    ftp_id=$("${compose[@]}" ps --all --quiet ftp) || true
    if [[ -n "$ftp_id" ]]; then
      docker inspect --format 'State={{json .State}} PortBindings={{json .HostConfig.PortBindings}}' "$ftp_id" >&2 || true
    fi
    "${compose[@]}" logs --no-color --tail 100 ftp >&2 || true
  fi
  "${compose[@]}" down --rmi local || teardown=$?
  rm -r -- "$workdir" || teardown=$?
  exit "$((result != 0 ? result : teardown))"
}
trap 'cleanup "$?"' EXIT
go build -o "$workdir/nuqs" ./cmd/nuqs
"${compose[@]}" up -d --build --wait --wait-timeout 60
python3 - <<'PY'
import socket
import sys

try:
    with socket.create_connection(("127.0.0.1", 12121), timeout=3) as conn:
        with conn.makefile("rb") as stream:
            greeting = stream.readline(512)
except OSError as err:
    sys.exit(f"FTP greeting check on 127.0.0.1:12121 failed: {err}")
if not greeting.startswith(b"220 "):
    sys.exit(f"Expected FTP 220 greeting on 127.0.0.1:12121, received {greeting!r}")
print("FTP greeting received on 127.0.0.1:12121 before discovery")
PY
"$workdir/nuqs" scan --json -Pn -p 12121,12323,18080 127.0.0.1 > "$workdir/report.json"
go run ./lab < "$workdir/report.json" > "$workdir/normalized.json"
diff -u lab/expected-report.json "$workdir/normalized.json"
echo "Lab report matches lab/expected-report.json"
