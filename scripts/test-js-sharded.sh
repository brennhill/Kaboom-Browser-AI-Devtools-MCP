#!/usr/bin/env bash
# test-js-sharded.sh — Run all extension JS tests split across N parallel Node processes.
#
# Why: Running 40 test files in a single Node process takes ~30 minutes due to
# serial execution of tests with real setTimeout waits. Splitting across processes
# reduces wall-clock time to ~1/N.

set -euo pipefail

SHARDS="${JS_TEST_SHARDS:-4}"
TIMEOUT="${JS_TEST_TIMEOUT:-15000}"
CONCURRENCY="${JS_TEST_CONCURRENCY:-4}"

usage() {
  cat <<'EOF'
Usage: scripts/test-js-sharded.sh [options]

Options:
  --shards <n>      Number of parallel processes (default: 4, env: JS_TEST_SHARDS)
  --timeout <ms>    Per-test timeout in ms (default: 15000, env: JS_TEST_TIMEOUT)
  -h, --help        Show help

Examples:
  scripts/test-js-sharded.sh
  scripts/test-js-sharded.sh --shards 8
  JS_TEST_SHARDS=6 make test-js
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --shards)   SHARDS="$2";   shift 2 ;;
    --timeout)  TIMEOUT="$2";  shift 2 ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

# Collect test files from both extension test roots.
#
# NOTE: do NOT use `mapfile`/`readarray` here. Those are bash 4+ builtins, and
# macOS ships bash 3.2 as /bin/bash — there `mapfile` is "command not found",
# which aborted this script before a single test ran. Use a portable read loop.
FILES=()
while IFS= read -r test_file; do
  [ -n "$test_file" ] && FILES+=("$test_file")
done < <(
  if command -v rg >/dev/null 2>&1; then
    rg --files tests/extension extension/background -g '*.test.js'
  else
    find tests/extension extension/background -name '*.test.js' -type f
  fi | sort
)
TOTAL=${#FILES[@]}

if [[ $TOTAL -eq 0 ]]; then
  echo "No extension test files found in tests/extension or extension/background" >&2
  exit 1
fi

# Cap shards at file count
if [[ $SHARDS -gt $TOTAL ]]; then
  SHARDS=$TOTAL
fi

echo "Sharding extension JS tests: $TOTAL files across $SHARDS process(es)"

# Distribute files round-robin into shard arrays
declare -a SHARD_FILES
for i in $(seq 0 $((SHARDS - 1))); do
  SHARD_FILES[$i]=""
done

for i in "${!FILES[@]}"; do
  shard=$((i % SHARDS))
  SHARD_FILES[$shard]+="${FILES[$i]} "
done

# Launch shards in parallel, capture PIDs and temp output files
PIDS=()
OUTPUTS=()

# Remove shard temp files even if the run is interrupted.
cleanup_outputs() {
  for f in "${OUTPUTS[@]:-}"; do
    [ -n "$f" ] && rm -f "$f"
  done
}
trap cleanup_outputs EXIT INT TERM
for i in $(seq 0 $((SHARDS - 1))); do
  files="${SHARD_FILES[$i]}"
  if [[ -z "$files" ]]; then
    continue
  fi
  # Keep XXXXXX at the end for BSD/macOS mktemp compatibility.
  outfile=$(mktemp "/tmp/js-shard-${i}-XXXXXX")
  OUTPUTS+=("$outfile")

  # shellcheck disable=SC2086
  node --experimental-test-module-mocks --test --test-force-exit --test-timeout="$TIMEOUT" --test-concurrency="$CONCURRENCY" $files > "$outfile" 2>&1 &
  PIDS+=($!)
done

# Wait for all shards
FAILED=0
for i in "${!PIDS[@]}"; do
  if ! wait "${PIDS[$i]}"; then
    FAILED=1
  fi
done

# Aggregate results.
#
# Parse node's machine-readable run summary ("ℹ pass N" / "ℹ fail N") rather than
# counting ✔/✖ glyph lines — the reporter prints those for suites as well as
# tests, so glyph counting badly inflates the totals.
TOTAL_PASS=0
TOTAL_FAIL=0
for outfile in "${OUTPUTS[@]}"; do
  # Pure awk (no pipeline): awk exits 0 even with zero matches, so `set -o pipefail`
  # cannot abort the run before the missing-summary guard below gets to report.
  pass=$(awk '/^ℹ pass [0-9]+$/ {s+=$3} END {print s+0}' "$outfile" 2>/dev/null)
  fail=$(awk '/^ℹ fail [0-9]+$/ {s+=$3} END {print s+0}' "$outfile" 2>/dev/null)

  # A shard with no summary never actually ran (crash, bad flag, missing binary).
  # Treat that as a hard failure so a broken runner can't report success.
  if [ "$(awk '/^ℹ fail [0-9]+$/ {n++} END {print n+0}' "$outfile" 2>/dev/null)" = "0" ]; then
    echo "ERROR: shard produced no test summary — it did not run. Output:" >&2
    tail -20 "$outfile" >&2
    FAILED=1
  fi

  TOTAL_PASS=$((TOTAL_PASS + ${pass:-0}))
  TOTAL_FAIL=$((TOTAL_FAIL + ${fail:-0}))
done

# Show failures if any
if [[ $TOTAL_FAIL -gt 0 ]]; then
  echo ""
  echo "=== FAILURES ==="
  for outfile in "${OUTPUTS[@]}"; do
    if grep -q '✖' "$outfile" 2>/dev/null; then
      grep -B1 '✖' "$outfile"
      echo "---"
    fi
  done
fi

# A non-zero aggregate failure count must fail the run even if every node
# process happened to exit 0.
if [[ $TOTAL_FAIL -gt 0 ]]; then
  FAILED=1
fi

# Summary
echo ""
echo "JS sharded test run: $TOTAL_PASS passed, $TOTAL_FAIL failed ($SHARDS shards)"

# Cleanup
for outfile in "${OUTPUTS[@]}"; do
  rm -f "$outfile"
done

if [[ $FAILED -ne 0 ]]; then
  echo "FAIL" >&2
  exit 1
fi

echo "OK"
