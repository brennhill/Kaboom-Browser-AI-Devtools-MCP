#!/usr/bin/env bash
# annotation-parity-benchmark.sh — repeated parity benchmark for framework + annotation smoke gates.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

RUNS="${ANNOTATION_PARITY_BENCH_RUNS:-3}"
PASS_RATE_THRESHOLD="${ANNOTATION_PARITY_PASS_RATE_THRESHOLD:-99}"
FRAMEWORK_REPEATS="${FRAMEWORK_RESILIENCE_FULL_REPEATS:-3}"
FRAMEWORK_CYCLES="${FRAMEWORK_SELECTOR_REFRESH_CYCLES:-3}"
REQUIRE_PILOT="${ANNOTATION_PARITY_REQUIRE_PILOT:-true}"

if ! [[ "$RUNS" =~ ^[0-9]+$ ]] || [ "$RUNS" -lt 1 ]; then
  echo "ERROR: ANNOTATION_PARITY_BENCH_RUNS must be a positive integer (got: $RUNS)" >&2
  exit 1
fi

parse_summary_value() {
  local file="$1"
  local key="$2"
  awk -v key="$key" '$1 == key { print $2 }' "$file" | tail -1
}

run_smoke_module() {
  local module="$1"
  local outfile="$2"
  local status=0

  (
    cd "$REPO_ROOT"
    NO_COLOR=1 \
      FRAMEWORK_RESILIENCE_FULL_REPEATS="$FRAMEWORK_REPEATS" \
      FRAMEWORK_SELECTOR_REFRESH_CYCLES="$FRAMEWORK_CYCLES" \
      bash scripts/uat/runners/smoke-test.sh --only "$module"
  ) >"$outfile" 2>&1 || status=$?

  local passed failed skipped
  passed="$(parse_summary_value "$outfile" "Passed:")"
  failed="$(parse_summary_value "$outfile" "Failed:")"
  skipped="$(parse_summary_value "$outfile" "Skipped:")"

  passed="${passed:-0}"
  failed="${failed:-0}"
  skipped="${skipped:-0}"

  echo "$status|$passed|$failed|$skipped"
}

fmt_pct() {
  local num="$1"
  local den="$2"
  if [ "$den" -eq 0 ]; then
    echo "0.00"
    return 0
  fi
  python3 - <<'PY' "$num" "$den"
import sys
num = float(sys.argv[1])
den = float(sys.argv[2])
print(f"{(num / den) * 100:.2f}")
PY
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

total_pass=0
total_fail=0
total_skip=0
hard_failures=0

echo "Annotation parity benchmark"
echo "  runs: $RUNS"
echo "  threshold: ${PASS_RATE_THRESHOLD}%"
echo "  framework repeats: $FRAMEWORK_REPEATS"
echo "  framework refresh cycles: $FRAMEWORK_CYCLES"
echo "  require pilot: $REQUIRE_PILOT"
echo ""

for run in $(seq 1 "$RUNS"); do
  echo "Run $run/$RUNS"

  mod29_log="$TMP_DIR/run-${run}-m29.log"
  mod31_log="$TMP_DIR/run-${run}-m31.log"

  mod29_result="$(run_smoke_module 29 "$mod29_log")"
  IFS='|' read -r m29_status m29_pass m29_fail m29_skip <<<"$mod29_result"

  mod31_result="$(run_smoke_module 31 "$mod31_log")"
  IFS='|' read -r m31_status m31_pass m31_fail m31_skip <<<"$mod31_result"

  run_pass=$((m29_pass + m31_pass))
  run_fail=$((m29_fail + m31_fail))
  run_skip=$((m29_skip + m31_skip))

  total_pass=$((total_pass + run_pass))
  total_fail=$((total_fail + run_fail))
  total_skip=$((total_skip + run_skip))

  if [ "$m29_status" -ne 0 ] || [ "$m31_status" -ne 0 ]; then
    hard_failures=$((hard_failures + 1))
  fi

  if [ "$REQUIRE_PILOT" = "true" ] && [ "$m29_skip" -gt 0 ]; then
    echo "  FAIL: framework module skipped tests (pilot not available or disabled)."
    hard_failures=$((hard_failures + 1))
  fi

  run_total_executed=$((run_pass + run_fail))
  run_pass_rate="$(fmt_pct "$run_pass" "$run_total_executed")"
  echo "  module29: pass=$m29_pass fail=$m29_fail skip=$m29_skip status=$m29_status"
  echo "  module31: pass=$m31_pass fail=$m31_fail skip=$m31_skip status=$m31_status"
  echo "  run-total: pass=$run_pass fail=$run_fail skip=$run_skip pass_rate=${run_pass_rate}%"
  echo ""
done

executed_total=$((total_pass + total_fail))
overall_pass_rate="$(fmt_pct "$total_pass" "$executed_total")"

echo "Benchmark summary"
echo "  total pass: $total_pass"
echo "  total fail: $total_fail"
echo "  total skip: $total_skip"
echo "  overall pass rate: ${overall_pass_rate}%"
echo "  hard failures: $hard_failures"

threshold_breach=0
if ! python3 - <<'PY' "$overall_pass_rate" "$PASS_RATE_THRESHOLD"
import sys
rate = float(sys.argv[1])
threshold = float(sys.argv[2])
sys.exit(0 if rate >= threshold else 1)
PY
then
  threshold_breach=1
fi

if [ "$hard_failures" -gt 0 ] || [ "$threshold_breach" -ne 0 ] || [ "$total_fail" -gt 0 ]; then
  echo "VERDICT: FAIL"
  if [ "$threshold_breach" -ne 0 ]; then
    echo "  reason: pass rate ${overall_pass_rate}% below threshold ${PASS_RATE_THRESHOLD}%"
  fi
  exit 1
fi

echo "VERDICT: PASS"
