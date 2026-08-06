#!/bin/bash
# test-tiered.sh — Three-tier parallel test execution for increasing clarity
# Tier 1 (1min):   Core protocol, basic tools
# Tier 2 (5min):   Extended coverage, edge cases
# Tier 3 (15min):  Full suite with stress, concurrency

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║               KABOOM UAT TIERED TEST SUITE — Parallel Execution                ║"
echo "║                                                                                ║"
echo "║ Tier 1 (1 min):   Protocol + Basic Tools (fastest feedback)                   ║"
echo "║ Tier 2 (5 min):   Extended Coverage (more confidence)                         ║"
echo "║ Tier 3 (15 min):  Full Suite (complete validation)                            ║"
echo "║                                                                                ║"
echo "║ All tiers run in PARALLEL — results show incrementally                        ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

RESULTS_DIR="/tmp/kaboom-uat-tiered-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results directory: $RESULTS_DIR"
echo ""

# Define test tiers
# Tier 1: Quick smoke tests (fastest)
TIER1_TESTS=(
    "cat-01-protocol"        # 7 tests
    "cat-04-configure"       # 11 tests
    "cat-08-security"        # 4 tests
    "cat-09-http"            # 4 tests
)

# Tier 2: Extended coverage (medium)
TIER2_TESTS=(
    "cat-02-observe"         # 26 tests
    "cat-03-generate"        # 9 tests
    "cat-05-interact"        # 19 tests
    "cat-06-lifecycle"       # 5 tests
    "cat-20-security"        # 5 tests (new critical tests)
)

# Tier 3: Full suite (comprehensive)
TIER3_TESTS=(
    "cat-07-concurrency"     # 3 tests
    "cat-10-regression"      # 3 tests
    "cat-11-data-pipeline"   # 31 tests
    "cat-12-rich-actions"    # 12 tests
    "cat-13-pilot-contract"  # 3 tests
    "cat-14-extension-startup" # 5 tests
    "cat-15-extended"        # 8 tests (new)
    "cat-15-pilot-success-path" # 4 tests
    "cat-16-api-contract"    # 5 tests
    "cat-17-generation-logic" # 6 tests (new)
    "cat-17-healing-logic"   # 7 tests (new)
    "cat-17-performance"     # 6 tests (new)
    "cat-29-reproduction"    # 6 tests
    "cat-18-recording-logic" # 6 tests (new)
    "cat-30-recording-automation" # 7 tests (new)
    "cat-18-playback-logic"  # 7 tests (new)
    "cat-18-recording"       # 7 tests
    "cat-19-extended"        # 10 tests (new)
    "cat-31-link-crawling"   # 6 tests (new)
    "cat-19-link-health"     # 19 tests
    "cat-20-filtering-logic" # 5 tests (new)
    "cat-32-auto-detect"     # 8 tests (new)
    "cat-20-noise-persistence" # 10 tests
    "cat-21-stress"          # 5 tests (new)
    "cat-22-advanced"        # 5 tests (new)
)

echo "🚀 Starting Tier 1, Tier 2, and Tier 3 in parallel..."
echo ""
echo "Tier 1: $(echo "${TIER1_TESTS[@]}" | wc -w) tests (expected ~1 min)"
echo "Tier 2: $(echo "${TIER2_TESTS[@]}" | wc -w) tests (expected ~5 min)"
echo "Tier 3: $(echo "${TIER3_TESTS[@]}" | wc -w) tests (expected ~15 min)"
echo ""

TIER1_START=$(date +%s)
TIER2_START=""
TIER3_START=""

# ============================================================================
# TIER 1: Run in parallel
# ============================================================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TIER 1: Protocol + Basic Tools (Fast Feedback)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

PORT=7890
for test in "${TIER1_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier1-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &
    PORT=$((PORT + 1))
    sleep 0.1
done

wait

TIER1_END=$(date +%s)
TIER1_DURATION=$((TIER1_END - TIER1_START))

# Show Tier 1 results
echo ""
TIER1_PASS=0
TIER1_FAIL=0
for test in "${TIER1_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier1-${test}.txt"
    if [ -f "$RESULT_FILE" ]; then
        PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
        FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")
        TIER1_PASS=$((TIER1_PASS + PASS))
        TIER1_FAIL=$((TIER1_FAIL + FAIL))

        if [ "$FAIL" -eq 0 ]; then
            echo "✅ $test: $PASS passed"
        else
            echo "❌ $test: $FAIL FAILED, $PASS passed"
        fi
    fi
done

echo ""
echo "TIER 1 COMPLETE: ${TIER1_DURATION}s | Passed: $TIER1_PASS | Failed: $TIER1_FAIL"
echo ""

if [ "$TIER1_FAIL" -eq 0 ]; then
    echo "✅ Tier 1 passed - daemon is healthy, basic tools work"
else
    echo "⚠️  Tier 1 had failures - core functionality may be broken"
fi

echo ""

# ============================================================================
# TIER 2: Run in parallel (while we wait for Tier 3)
# ============================================================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TIER 2: Extended Coverage (More Confidence)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

TIER2_START=$(date +%s)
PORT=7890
for test in "${TIER2_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier2-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &
    PORT=$((PORT + 1))
    sleep 0.1
done

wait

TIER2_END=$(date +%s)
TIER2_DURATION=$((TIER2_END - TIER2_START))

# Show Tier 2 results
echo ""
TIER2_PASS=0
TIER2_FAIL=0
for test in "${TIER2_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier2-${test}.txt"
    if [ -f "$RESULT_FILE" ]; then
        PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
        FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")
        TIER2_PASS=$((TIER2_PASS + PASS))
        TIER2_FAIL=$((TIER2_FAIL + FAIL))

        if [ "$FAIL" -eq 0 ]; then
            echo "✅ $test: $PASS passed"
        else
            echo "❌ $test: $FAIL FAILED, $PASS passed"
        fi
    fi
done

echo ""
echo "TIER 2 COMPLETE: ${TIER2_DURATION}s | Passed: $TIER2_PASS | Failed: $TIER2_FAIL"
echo ""

# ============================================================================
# TIER 3: Run in parallel
# ============================================================================

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TIER 3: Full Suite (Complete Validation)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

TIER3_START=$(date +%s)
PORT=7890
for test in "${TIER3_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier3-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &
    PORT=$((PORT + 1))
    if [ $((PORT - 7890)) -ge 8 ]; then
        PORT=7890
    fi
    sleep 0.1
done

wait

TIER3_END=$(date +%s)
TIER3_DURATION=$((TIER3_END - TIER3_START))

# Show Tier 3 results
echo ""
TIER3_PASS=0
TIER3_FAIL=0
TIER3_SKIP=0
for test in "${TIER3_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier3-${test}.txt"
    if [ -f "$RESULT_FILE" ]; then
        PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
        FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")
        SKIP=$(grep -c "^  SKIP:" "$RESULT_FILE" 2>/dev/null || echo "0")
        TIER3_PASS=$((TIER3_PASS + PASS))
        TIER3_FAIL=$((TIER3_FAIL + FAIL))
        TIER3_SKIP=$((TIER3_SKIP + SKIP))

        if [ "$FAIL" -eq 0 ] && [ "$SKIP" -eq 0 ]; then
            echo "✅ $test: $PASS passed"
        elif [ "$FAIL" -eq 0 ]; then
            echo "⚠️  $test: $PASS passed, $SKIP skipped"
        else
            echo "❌ $test: $FAIL FAILED, $PASS passed"
        fi
    fi
done

echo ""
echo "TIER 3 COMPLETE: ${TIER3_DURATION}s | Passed: $TIER3_PASS | Failed: $TIER3_FAIL | Skipped: $TIER3_SKIP"
echo ""

# ============================================================================
# FINAL SUMMARY
# ============================================================================

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                          FINAL RESULTS SUMMARY                                 ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

TOTAL_PASS=$((TIER1_PASS + TIER2_PASS + TIER3_PASS))
TOTAL_FAIL=$((TIER1_FAIL + TIER2_FAIL + TIER3_FAIL))
TOTAL_SKIP=$TIER3_SKIP
TOTAL_DURATION=$((TIER3_END - TIER1_START))

echo "┌────────────────────────────────────────────────────────────────────────────────┐"
echo "│ Tier 1 (1 min):      $TIER1_PASS passed, $TIER1_FAIL failed — ${TIER1_DURATION}s"
echo "│ Tier 2 (5 min):      $TIER2_PASS passed, $TIER2_FAIL failed — ${TIER2_DURATION}s"
echo "│ Tier 3 (15 min):     $TIER3_PASS passed, $TIER3_FAIL failed, $TIER3_SKIP skipped — ${TIER3_DURATION}s"
echo "├────────────────────────────────────────────────────────────────────────────────┤"
echo "│ TOTAL:               $TOTAL_PASS passed, $TOTAL_FAIL failed, $TOTAL_SKIP skipped"
echo "│ Total Time:          ${TOTAL_DURATION}s"
echo "│ Tests/sec:           $(( (TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP) / (TOTAL_DURATION + 1) ))"
echo "└────────────────────────────────────────────────────────────────────────────────┘"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "🎉 ALL TIERS PASSED"
    exit 0
else
    echo "⚠️  FAILURES DETECTED - Check $RESULTS_DIR for details"
    exit 1
fi
