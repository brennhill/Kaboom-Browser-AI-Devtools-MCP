#!/bin/bash
# test-original-uat-safe.sh — Safe sequential execution of original UAT tests
# Runs tests 3-4 at a time with proper cleanup between groups
# Avoids daemon lifecycle race conditions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                    ORIGINAL UAT TESTS — Safe Sequential                        ║"
echo "║                  (3-4 concurrent tests, proper cleanup delays)                  ║"
echo "║                                                                                ║"
echo "║ Running 54 tests across 20 categories in 6 groups of 3-4 tests                 ║"
echo "║ Each test uses distinct port range with 1s cleanup delay between groups       ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

# Create results directory
RESULTS_DIR="/tmp/kaboom-uat-original-safe-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results will be saved to: $RESULTS_DIR"
echo ""

# Original test categories (proven stable)
TESTS=(
    "cat-01-protocol"
    "cat-02-observe"
    "cat-03-generate"
    "cat-04-configure"
    "cat-05-interact"
    "cat-06-lifecycle"
    "cat-07-concurrency"
    "cat-08-security"
    "cat-09-http"
    "cat-10-regression"
    "cat-11-data-pipeline"
    "cat-12-rich-actions"
    "cat-13-pilot-contract"
    "cat-14-extension-startup"
    "cat-15-pilot-success-path"
    "cat-16-api-contract"
    "cat-29-reproduction"
    "cat-18-recording"
    "cat-19-link-health"
    "cat-20-noise-persistence"
)

GLOBAL_START=$(date +%s)

# Process tests in groups of 3-4 with proper port spacing
# Group 1: ports 7900-7902
# Group 2: ports 7920-7922
# Group 3: ports 7940-7942
# Group 4: ports 7960-7962
# Group 5: ports 7980-7982
# Group 6: ports 7900-7902 (reuse, after cleanup)
run_test_group() {
    local group_num=$1
    local port_start=$2
    shift 2
    local tests=("$@")

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "GROUP $group_num: Running ${#tests[@]} tests on ports $port_start-$((port_start + ${#tests[@]} - 1))"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    local port=$port_start
    local pids=()

    # Launch tests in this group
    for test in "${tests[@]}"; do
        RESULT_FILE="$RESULTS_DIR/group${group_num}-${test}.txt"
        bash "$TEST_DIR/${test}.sh" "$port" "$RESULT_FILE" >/dev/null 2>&1 &
        local pid=$!
        pids+=("$pid")
        port=$((port + 1))
    done

    # Wait for all tests in this group to complete
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    echo "✅ Group $group_num complete"
    echo ""

    # Give OS time to clean up TCP connections (TIME_WAIT)
    sleep 1

    return 0
}

echo "🚀 Starting test groups sequentially with 3-4 concurrent tests per group..."
echo ""

# Run groups in sequence (but each group runs 3-4 tests in parallel)
run_test_group 1 7900 "${TESTS[@]:0:3}"
run_test_group 2 7920 "${TESTS[@]:3:3}"
run_test_group 3 7940 "${TESTS[@]:6:3}"
run_test_group 4 7960 "${TESTS[@]:9:3}"
run_test_group 5 7980 "${TESTS[@]:12:3}"
run_test_group 6 7900 "${TESTS[@]:15:5}"

GLOBAL_END=$(date +%s)
GLOBAL_DURATION=$((GLOBAL_END - GLOBAL_START))

# ============================================================================
# COLLECT AND DISPLAY RESULTS
# ============================================================================

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                          TEST RESULTS SUMMARY                                  ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0

for test in "${TESTS[@]}"; do
    # Find result file (could be in any group)
    RESULT_FILE=$(find "$RESULTS_DIR" -name "*-${test}.txt" | head -1)

    if [ -f "$RESULT_FILE" ]; then
        # Source the result file to get counts
        # shellcheck source=/dev/null
        source "$RESULT_FILE" 2>/dev/null || {
            echo "⚠️  $test: CORRUPTED RESULT FILE"
            continue
        }

        TOTAL_PASS=$((TOTAL_PASS + PASS_COUNT))
        TOTAL_FAIL=$((TOTAL_FAIL + FAIL_COUNT))
        TOTAL_SKIP=$((TOTAL_SKIP + SKIP_COUNT))

        if [ "$FAIL_COUNT" -eq 0 ]; then
            if [ "$SKIP_COUNT" -gt 0 ]; then
                echo "⚠️  $test: $PASS_COUNT passed, $SKIP_COUNT skipped"
            else
                echo "✅ $test: $PASS_COUNT passed"
            fi
        else
            echo "❌ $test: $PASS_COUNT passed, $FAIL_COUNT FAILED, $SKIP_COUNT skipped"
        fi
    else
        echo "⚠️  $test: NO RESULT FILE"
    fi
done

echo ""
echo "════════════════════════════════════════════════════════════════════════════════"
echo "Summary:"
echo "  Total Passed:  $TOTAL_PASS"
echo "  Total Failed:  $TOTAL_FAIL"
echo "  Total Skipped: $TOTAL_SKIP"
echo "  Duration:      ${GLOBAL_DURATION}s (groups sequential, tests parallel)"
echo "  Tests/sec:     $(( (TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP) / (GLOBAL_DURATION + 1) ))"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "🎉 ALL ORIGINAL TESTS PASSED"
    exit 0
else
    echo "⚠️  FAILURES DETECTED"
    echo ""
    echo "Failed test details in: $RESULTS_DIR"
    exit 1
fi
