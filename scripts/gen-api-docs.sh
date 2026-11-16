#!/usr/bin/env bash
# Builds docs/api/ from the doc comments in the Go code.
# CI runs this before building the website, so the API pages are always
# the same as the code. You can run it locally too: ./scripts/gen-api-docs.sh
set -euo pipefail

cd "$(dirname "$0")/.."
out=docs/api
rm -rf "$out"
mkdir -p "$out"

{
  echo "# API reference"
  echo
  echo "Generated from the comments in the code, so it's always up to date. Don't edit these pages by hand, edit the comments instead."
  echo
} > "$out/index.md"

module=$(go list -m)
for pkg in $(go list ./...); do
  name=${pkg#"$module"/}
  [ "$name" = "$module" ] && name=nuqs
  file="$(echo "$name" | tr '/' '-').md"
  {
    echo "# $name"
    echo
    echo "\`import \"$pkg\"\`"
    echo
    echo '```text'
    go doc -all "$pkg"
    echo '```'
  } > "$out/$file"
  echo "- [$name]($file)" >> "$out/index.md"
done

echo "API docs written to $out/"
