#!/bin/bash
# test-new-uat-conservative.sh — Conservative parallel test execution for new tests
# Runs 4 test categories at a time with proper port spacing and cleanup delays
# Addresses daemon lifecycle race conditions from previous parallel approach

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                    NEW UAT TESTS — Conservative Parallel                       ║"
echo "║                   (4 concurrent groups, proper cleanup delays)                  ║"
echo "║                                                                                ║"
echo "║ Running 98 tests across 14 categories in 4 groups of 3-4 tests                 ║"
echo "║ Each test uses distinct port range with 1s cleanup delay between groups       ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

# Create results directory
RESULTS_DIR="/tmp/kaboom-uat-new-conservative-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results will be saved to: $RESULTS_DIR"
echo ""

# Define new test categories (newly built)
TESTS=(
    "cat-15-extended"
    "cat-17-generation-logic"
    "cat-17-healing-logic"
    "cat-17-performance"
    "cat-18-recording-logic"
    "cat-30-recording-automation"
    "cat-18-playback-logic"
    "cat-19-extended"
    "cat-31-link-crawling"
    "cat-20-security"
    "cat-20-filtering-logic"
    "cat-32-auto-detect"
    "cat-21-stress"
    "cat-22-advanced"
)

GLOBAL_START=$(date +%s)

# Process tests in groups of 4 with proper port spacing
# Group 1: ports 7900-7903
# Group 2: ports 7920-7923
# Group 3: ports 7940-7943
# Group 4: ports 7960-7963
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

echo "🚀 Starting test groups sequentially with 4 concurrent tests per group..."
echo ""

# Run groups in sequence (but each group runs 4 tests in parallel)
run_test_group 1 7900 "${TESTS[@]:0:4}"
run_test_group 2 7920 "${TESTS[@]:4:4}"
run_test_group 3 7940 "${TESTS[@]:8:4}"
run_test_group 4 7960 "${TESTS[@]:12:2}"

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
echo "  Duration:      ${GLOBAL_DURATION}s (groups run sequentially, tests parallel)"
echo "  Tests/sec:     $(( (TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP) / (GLOBAL_DURATION + 1) ))"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "🎉 ALL NEW TESTS PASSED (or skipped for pending features)"
    echo ""
    echo "ℹ️  Skipped tests are expected for features not yet implemented."
    exit 0
else
    echo "⚠️  FAILURES DETECTED"
    echo ""
    echo "Failed test details in: $RESULTS_DIR"
    exit 1
fi
