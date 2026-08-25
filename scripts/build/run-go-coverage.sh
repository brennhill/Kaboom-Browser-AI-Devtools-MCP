#!/usr/bin/env bash
# run-go-coverage.sh — Measure package and black-box Go execution in one profile.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COVERAGE_ROOT="$PROJECT_ROOT/coverage/go"
SUBPROCESS_DIR="$COVERAGE_ROOT/subprocess"
PACKAGE_PROFILE="$COVERAGE_ROOT/packages.out"
SUBPROCESS_PROFILE="$COVERAGE_ROOT/subprocess.out"
MERGED_PROFILE="$PROJECT_ROOT/coverage.out"
BASELINE_PATH="$PROJECT_ROOT/.coverage-baseline.json"
MINIMUM="${GO_COVERAGE_MINIMUM:-89}"

# The baseline is an upward-only ratchet: the enforced floor is the max of the
# historical minimum and the recorded baseline, so GO_COVERAGE_MINIMUM can only
# RAISE the bar, never lower it below what the repo has already demonstrated.
BASELINE_COVERAGE="$(
  node -e '
    const fs = require("node:fs");
    try {
      const parsed = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      if (parsed.version !== 1 || typeof parsed.go_total_percent !== "number") process.exit(1);
      console.log(parsed.go_total_percent);
    } catch { console.log(0) }
  ' "$BASELINE_PATH" 2>/dev/null || echo 0
)"
FLOOR="$(
  awk -v minimum="$MINIMUM" -v baseline="$BASELINE_COVERAGE" 'BEGIN {
    print (baseline > minimum) ? baseline : minimum
  }'
)"

rm -rf "$SUBPROCESS_DIR"
mkdir -p "$SUBPROCESS_DIR"

cd "$PROJECT_ROOT"
KABOOM_GO_COVERDIR="$SUBPROCESS_DIR" \
  go test -count=1 -coverpkg=./... -coverprofile="$PACKAGE_PROFILE" ./...
KABOOM_GO_COVERDIR="$SUBPROCESS_DIR" \
  scripts/build/run-go-integration.sh -count=1

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

awk -v actual="$coverage" -v floor="$FLOOR" 'BEGIN {
  if (actual < floor) {
    printf "FAIL: Coverage %.1f%% is below the %.1f%% floor (historical minimum %s%%, ratcheted baseline %s%%)\n", actual, floor, "'"${MINIMUM}"'", "'"${BASELINE_COVERAGE}"'"
    exit 1
  }
  printf "OK: Coverage %.1f%% (floor %.1f%%)\n", actual, floor
}'

# Ratchet up: a run that beats the recorded baseline by a margin locks in only
# when explicitly requested (make coverage-baseline-update), so ordinary
# run-to-run variance never moves the floor.
if [[ "${KABOOM_COVERAGE_UPDATE_BASELINE:-0}" == "1" ]]; then
  node -e '
    const fs = require("node:fs");
    fs.writeFileSync(process.argv[1], JSON.stringify({ version: 1, go_total_percent: Number(process.argv[2]) }, null, 2) + "\n");
  ' "$BASELINE_PATH" "$coverage"
  echo "Coverage baseline ratcheted to ${coverage}%."
elif awk -v actual="$coverage" -v baseline="$BASELINE_COVERAGE" 'BEGIN { exit !(actual > baseline + 0.5) }'; then
  echo "Coverage improved past the baseline — run \`make coverage-baseline-update\` to lock it in."
fi
