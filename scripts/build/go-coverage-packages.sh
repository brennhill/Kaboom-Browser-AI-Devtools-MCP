#!/usr/bin/env bash
# go-coverage-packages.sh — Print the first-party Go packages the coverage gate measures.
#
# `./...` also matches vendored npm content such as
# node_modules/flatted/golang/pkg/flatted (no go.mod of its own, so the go tool
# treats it as part of this module). Whether it exists depends on the local
# npm install, so including it swung the aggregate by ~0.5% between machines
# and runs with identical Go code. Only first-party packages are measured.

set -euo pipefail

cd "$(dirname "$0")/../.."

packages="$(go list ./... | grep -v '/node_modules/' || true)"
if [[ -z "$packages" ]]; then
  echo "FAIL: no first-party Go packages found for coverage" >&2
  exit 1
fi
printf '%s\n' "$packages"
