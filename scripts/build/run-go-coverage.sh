#!/usr/bin/env bash
# run-go-coverage.sh — Measure package and black-box Go execution in one profile.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COVERAGE_ROOT="$PROJECT_ROOT/coverage/go"
SUBPROCESS_DIR="$COVERAGE_ROOT/subprocess"
PACKAGE_PROFILE="$COVERAGE_ROOT/packages.out"
SUBPROCESS_PROFILE="$COVERAGE_ROOT/subprocess.out"
MERGED_PROFILE="$PROJECT_ROOT/coverage.out"
MINIMUM="${GO_COVERAGE_MINIMUM:-89}"

mkdir -p "$SUBPROCESS_DIR"
find "$SUBPROCESS_DIR" -mindepth 1 -maxdepth 1 -type f -delete

cd "$PROJECT_ROOT"
KABOOM_GO_COVERDIR="$SUBPROCESS_DIR" \
  go test -count=1 -coverpkg=./... -coverprofile="$PACKAGE_PROFILE" ./...

coverage_inputs="$(
  find "$SUBPROCESS_DIR" -type f -name 'covmeta.*' -exec dirname {} \; |
    sort -u |
    paste -sd, -
)"
if [[ -z "$coverage_inputs" ]]; then
  echo "FAIL: black-box Go tests produced no subprocess coverage" >&2
  exit 1
fi

go tool covdata textfmt -i="$coverage_inputs" -o="$SUBPROCESS_PROFILE"
node scripts/build/merge-go-coverage.mjs \
  "$MERGED_PROFILE" "$PACKAGE_PROFILE" "$SUBPROCESS_PROFILE"

coverage="$(
  go tool cover -func="$MERGED_PROFILE" |
    awk '/^total:/{gsub(/%/, "", $3); print $3}'
)"
if [[ -z "$coverage" ]]; then
  echo "FAIL: unable to calculate aggregate Go coverage" >&2
  exit 1
fi

awk -v actual="$coverage" -v minimum="$MINIMUM" 'BEGIN {
  if (actual < minimum) {
    printf "FAIL: Coverage %.1f%% is below %.1f%% threshold\n", actual, minimum
    exit 1
  }
  printf "OK: Coverage %.1f%%\n", actual
}'
