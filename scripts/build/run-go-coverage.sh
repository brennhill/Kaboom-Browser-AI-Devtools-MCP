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
# A missing baseline (first run) floors to 0; anything else — unreadable file,
# invalid JSON, wrong version, non-numeric percent — fails loud rather than
# silently degrading the floor to 0.
BASELINE_COVERAGE="$(
  node -e '
    const fs = require("node:fs");
    const path = process.argv[1];
    let raw;
    try {
      raw = fs.readFileSync(path, "utf8");
    } catch (error) {
      if (error.code === "ENOENT") { console.log(0); process.exit(0); }
      process.stderr.write(`FAIL: cannot read coverage baseline ${path}: ${error.message}\n`);
      process.exit(1);
    }
    let parsed;
    try {
      parsed = JSON.parse(raw);
    } catch (error) {
      process.stderr.write(`FAIL: coverage baseline ${path} is not valid JSON: ${error.message}\n`);
      process.exit(1);
    }
    if (parsed === null || typeof parsed !== "object" || parsed.version !== 1) {
      process.stderr.write(`FAIL: coverage baseline ${path} must have version 1\n`);
      process.exit(1);
    }
    if (typeof parsed.go_total_percent !== "number" || !Number.isFinite(parsed.go_total_percent)) {
      process.stderr.write(`FAIL: coverage baseline ${path} needs a finite numeric go_total_percent\n`);
      process.exit(1);
    }
    console.log(parsed.go_total_percent);
  ' "$BASELINE_PATH"
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
# run-to-run variance never moves the floor. The update itself is also
# upward-only: a lower measurement refuses to move the baseline (the run
# already cleared the floor above it) instead of weakening it.
if [[ "${KABOOM_COVERAGE_UPDATE_BASELINE:-0}" == "1" ]]; then
  node -e '
    const fs = require("node:fs");
    const path = process.argv[1];
    const next = Number(process.argv[2]);
    if (!Number.isFinite(next)) {
      process.stderr.write(`FAIL: refusing to write non-numeric coverage value ${process.argv[2]}\n`);
      process.exit(1);
    }
    let existing = null;
    if (fs.existsSync(path)) {
      let parsed;
      try {
        parsed = JSON.parse(fs.readFileSync(path, "utf8"));
      } catch (error) {
        process.stderr.write(`FAIL: coverage baseline ${path} is not valid JSON: ${error.message}\n`);
        process.exit(1);
      }
      if (parsed === null || typeof parsed !== "object" || parsed.version !== 1 ||
          typeof parsed.go_total_percent !== "number" || !Number.isFinite(parsed.go_total_percent)) {
        process.stderr.write(`FAIL: coverage baseline ${path} is invalid; refusing to update it\n`);
        process.exit(1);
      }
      existing = parsed.go_total_percent;
    }
    if (existing !== null && next < existing) {
      console.log(`refusing to lower coverage baseline from ${existing} to ${next}`);
      process.exit(0);
    }
    fs.writeFileSync(path, JSON.stringify({ version: 1, go_total_percent: next }, null, 2) + "\n");
    console.log(`Coverage baseline ratcheted to ${next}%.`);
  ' "$BASELINE_PATH" "$coverage"
elif awk -v actual="$coverage" -v baseline="$BASELINE_COVERAGE" 'BEGIN { exit !(actual > baseline + 0.5) }'; then
  echo "Coverage improved past the baseline — run \`make coverage-baseline-update\` to lock it in."
fi
