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
if command -v rg >/dev/null 2>&1; then
  mapfile -t FILES < <(rg --files tests/extension extension/background -g '*.test.js' | sort)
else
  mapfile -t FILES < <(find tests/extension extension/background -name '*.test.js' -type f | sort)
fi
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
for i in $(seq 0 $((SHARDS - 1))); do
  files="${SHARD_FILES[$i]}"
  if [[ -z "$files" ]]; then
    continue
  fi
  # Keep XXXXXX at the end for BSD/macOS mktemp compatibility.
  outfile=$(mktemp "/tmp/js-shard-${i}-XXXXXX")
  OUTPUTS+=("$outfile")

  # Force the spec reporter so ✔/✖ marks appear even when stdout is redirected
  # (node defaults to the TAP reporter off a TTY, which this script cannot count).
  # shellcheck disable=SC2086
  node --experimental-test-module-mocks --test --test-reporter=spec --test-reporter-destination=stdout --test-force-exit --test-timeout="$TIMEOUT" --test-concurrency="$CONCURRENCY" $files > "$outfile" 2>&1 &
  PIDS+=($!)
done

# Wait for all shards
FAILED=0
for i in "${!PIDS[@]}"; do
  if ! wait "${PIDS[$i]}"; then
    FAILED=1
  fi
done

# Aggregate results
TOTAL_PASS=0
TOTAL_FAIL=0
for outfile in "${OUTPUTS[@]}"; do
  pass=$(grep -c '✔' "$outfile" 2>/dev/null || true)
  fail=$(grep -c '✖' "$outfile" 2>/dev/null || true)
  pass=${pass:-0}; pass=${pass//[^0-9]/}; pass=${pass:-0}
  fail=${fail:-0}; fail=${fail//[^0-9]/}; fail=${fail:-0}
  TOTAL_PASS=$((TOTAL_PASS + pass))
  TOTAL_FAIL=$((TOTAL_FAIL + fail))
done

# Show failures if any. Trigger on parsed ✖ marks OR a non-zero shard exit: a shard
# can fail to run at all (unknown node flag, import error, crash) and produce zero ✖
# marks, so dump its raw tail too — otherwise the real cause is invisible in CI logs.
if [[ $TOTAL_FAIL -gt 0 || $FAILED -ne 0 ]]; then
  echo ""
  echo "=== FAILURES ==="
  for i in "${!OUTPUTS[@]}"; do
    outfile="${OUTPUTS[$i]}"
    if grep -q '✖' "$outfile" 2>/dev/null; then
      grep -B1 '✖' "$outfile"
      echo "---"
    elif [[ $FAILED -ne 0 ]]; then
      echo "--- shard $i produced no ✖ but a shard exited non-zero; raw tail ---"
      tail -40 "$outfile"
      echo "---"
    fi
  done
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
