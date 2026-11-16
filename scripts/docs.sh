#!/usr/bin/env bash
# Builds the docs website. The first run installs what it needs into .venv.
#
#   ./scripts/docs.sh          preview at http://127.0.0.1:8000 (reloads on save)
#   ./scripts/docs.sh build    build into site/, the same way CI does
set -euo pipefail
cd "$(dirname "$0")/.."

# Install (or update) the docs tools when requirements.txt is new or changed.
if [ ! -f .venv/.installed ] || [ docs/requirements.txt -nt .venv/.installed ]; then
  echo "Installing docs tools into .venv (only needed once)..."
  python3 -m venv .venv
  .venv/bin/pip install --quiet --upgrade pip
  .venv/bin/pip install --quiet -r docs/requirements.txt
  touch .venv/.installed
fi

./scripts/gen-api-docs.sh
go run ./tools/archdiagram

if [ "${1:-serve}" = "build" ]; then
  .venv/bin/mkdocs build --strict
else
  .venv/bin/mkdocs serve
fi
