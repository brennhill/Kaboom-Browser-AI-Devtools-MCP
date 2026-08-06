#!/bin/bash
# test-original-uat.sh — Original/Proven UAT Tests (54 tests, 20 categories)
# These are the stable, battle-tested test categories

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../orchestration/uat-result-lib.sh"

echo "════════════════════════════════════════════════════════════════════════════════"
echo "ORIGINAL UAT TEST SUITE — 54 Tests, 20 Categories"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

# Create results directory
RESULTS_DIR="/tmp/kaboom-uat-original-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results will be saved to: $RESULTS_DIR"
echo ""
echo "Running in parallel (16 categories at once)..."
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

PORT=7890
# shellcheck disable=SC2034 # used by sourced result files
PASS_COUNT=0
# shellcheck disable=SC2034 # used by sourced result files
FAIL_COUNT=0
START_TIME=$(date +%s)

# Run tests in parallel (max 16 at a time)
for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"

    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &

    PORT=$((PORT + 1))
    if [ $((PORT - 7890)) -ge 16 ]; then
        PORT=7890
    fi

    # Small delay between launches
    sleep 0.1
done

echo "⏳ Waiting for all test categories to complete..."
wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Aggregate results
echo ""
echo "════════════════════════════════════════════════════════════════════════════════"
echo "TEST RESULTS"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0
REPORTED_CATEGORIES=0
MISSING_CATEGORIES=0
CORRUPT_CATEGORIES=0
INVALID_COUNTER_CATEGORIES=0

for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"
    parse_status=0

    if parse_uat_category_result "$RESULT_FILE"; then
        REPORTED_CATEGORIES=$((REPORTED_CATEGORIES + 1))
        PASS="$UAT_RESULT_PASS"
        FAIL="$UAT_RESULT_FAIL"
        SKIP="$UAT_RESULT_SKIP"

        TOTAL_PASS=$((TOTAL_PASS + PASS))
        TOTAL_FAIL=$((TOTAL_FAIL + FAIL))
        TOTAL_SKIP=$((TOTAL_SKIP + SKIP))

        if [ "$FAIL" -eq 0 ]; then
            if [ "$SKIP" -gt 0 ]; then
                echo "⚠️  $test: $PASS passed, $SKIP skipped"
            else
                echo "✅ $test: $PASS passed"
            fi
        else
            echo "❌ $test: $PASS passed, $FAIL FAILED, $SKIP skipped"
        fi
        continue
    else
        parse_status="$?"
    fi

    case "$parse_status" in
        1)
            MISSING_CATEGORIES=$((MISSING_CATEGORIES + 1))
            echo "⚠️  $test: NO RESULT FILE"
            ;;
        2)
            CORRUPT_CATEGORIES=$((CORRUPT_CATEGORIES + 1))
            echo "⚠️  $test: CORRUPTED RESULT FILE"
            ;;
        3)
            INVALID_COUNTER_CATEGORIES=$((INVALID_COUNTER_CATEGORIES + 1))
            echo "⚠️  $test: INVALID RESULT COUNTERS"
            ;;
        *)
            CORRUPT_CATEGORIES=$((CORRUPT_CATEGORIES + 1))
            echo "⚠️  $test: UNKNOWN RESULT PARSE ERROR"
            ;;
    esac
done

TOTAL_ASSERTIONS=$((TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP))
INTEGRITY_ERRORS=$((MISSING_CATEGORIES + CORRUPT_CATEGORIES + INVALID_COUNTER_CATEGORIES))
CATEGORY_TOTAL="${#TESTS[@]}"
CATEGORY_COVERAGE_PCT="$(awk "BEGIN { if ($CATEGORY_TOTAL == 0) { print \"0.0\" } else { printf \"%.1f\", ($REPORTED_CATEGORIES*100)/$CATEGORY_TOTAL } }")"
FAIL_ON_RESULT_INTEGRITY="${KABOOM_UAT_FAIL_ON_RESULT_INTEGRITY:-1}"

if [ "$TOTAL_ASSERTIONS" -eq 0 ]; then
    INTEGRITY_ERRORS=$((INTEGRITY_ERRORS + 1))
    echo ""
    echo "⚠️  Result integrity check: zero assertions collected from all categories."
fi

if [ "$FAIL_ON_RESULT_INTEGRITY" = "1" ] && [ "$INTEGRITY_ERRORS" -gt 0 ]; then
    RESULT_INTEGRITY_FAILED=true
else
    RESULT_INTEGRITY_FAILED=false
fi

if [ -n "${KABOOM_UAT_SUMMARY_FILE:-}" ]; then
    cat > "$KABOOM_UAT_SUMMARY_FILE" <<EOF
RESULTS_DIR=$RESULTS_DIR
TOTAL_PASS=$TOTAL_PASS
TOTAL_FAIL=$TOTAL_FAIL
TOTAL_SKIP=$TOTAL_SKIP
TOTAL_ASSERTIONS=$TOTAL_ASSERTIONS
CATEGORY_TOTAL=$CATEGORY_TOTAL
CATEGORY_REPORTED=$REPORTED_CATEGORIES
CATEGORY_MISSING=$MISSING_CATEGORIES
CATEGORY_CORRUPT=$CORRUPT_CATEGORIES
CATEGORY_INVALID=$INVALID_COUNTER_CATEGORIES
INTEGRITY_ERRORS=$INTEGRITY_ERRORS
DURATION=$DURATION
EOF
fi

echo ""
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Summary:"
echo "  Total Passed:  $TOTAL_PASS"
echo "  Total Failed:  $TOTAL_FAIL"
echo "  Total Skipped: $TOTAL_SKIP"
echo "  Total Checks:  $TOTAL_ASSERTIONS"
echo "  Category Coverage: $REPORTED_CATEGORIES/$CATEGORY_TOTAL (${CATEGORY_COVERAGE_PCT}%)"
echo "  Result Integrity Errors: $INTEGRITY_ERRORS"
echo "  Duration:      ${DURATION}s"
echo "  Tests/sec:     $(( TOTAL_ASSERTIONS / (DURATION + 1) ))"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ] && [ "$RESULT_INTEGRITY_FAILED" = false ]; then
    echo "🎉 ALL ORIGINAL TESTS PASSED"
    exit 0
else
    echo "⚠️  FAILURES DETECTED"
    echo ""
    if [ "$RESULT_INTEGRITY_FAILED" = true ]; then
        echo "Result integrity checks failed (missing/corrupt/invalid category results)."
    fi
    echo "Failed test details in: $RESULTS_DIR"
    exit 1
fi
