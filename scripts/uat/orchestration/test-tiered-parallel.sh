#!/bin/bash
# test-tiered-parallel.sh — Three-tier parallel test execution (ALL START AT ONCE)
# All three tiers launch simultaneously and run in parallel for incremental results

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║          KABOOM UAT TIERED TEST SUITE — ALL TIERS IN PARALLEL                 ║"
echo "║                                                                                ║"
echo "║ Tier 1 (1 min):   Protocol + Basic Tools        ▶ STARTS NOW                  ║"
echo "║ Tier 2 (5 min):   Extended Coverage             ▶ STARTS NOW                  ║"
echo "║ Tier 3 (15 min):  Full Suite + Stress           ▶ STARTS NOW                  ║"
echo "║                                                                                ║"
echo "║ Results appear incrementally as each tier completes                            ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

RESULTS_DIR="/tmp/kaboom-uat-tiered-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results: $RESULTS_DIR"
echo ""

GLOBAL_START=$(date +%s)

# Define test tiers
TIER1_TESTS=("cat-01-protocol" "cat-04-configure" "cat-08-security" "cat-09-http")
TIER2_TESTS=("cat-02-observe" "cat-03-generate" "cat-05-interact" "cat-06-lifecycle" "cat-20-security")
TIER3_TESTS=("cat-07-concurrency" "cat-10-regression" "cat-11-data-pipeline" "cat-12-rich-actions" "cat-13-pilot-contract" "cat-14-extension-startup" "cat-15-extended" "cat-15-pilot-success-path" "cat-16-api-contract" "cat-17-generation-logic" "cat-17-healing-logic" "cat-17-performance" "cat-29-reproduction" "cat-18-recording-logic" "cat-30-recording-automation" "cat-18-playback-logic" "cat-18-recording" "cat-19-extended" "cat-31-link-crawling" "cat-19-link-health" "cat-20-filtering-logic" "cat-20-noise-persistence" "cat-21-stress" "cat-22-advanced")

# ============================================================================
# LAUNCH ALL TIERS IN PARALLEL (TIER 1, 2, 3 ALL START NOW)
# ============================================================================

echo "🚀 Launching all three tiers simultaneously..."
echo ""

# TIER 1: Launch all tests
PORT=7890
for test in "${TIER1_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier1-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" >/dev/null 2>&1 &
    PORT=$((PORT + 1))
done

# TIER 2: Launch all tests (different port range)
PORT=7920
for test in "${TIER2_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier2-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" >/dev/null 2>&1 &
    PORT=$((PORT + 1))
done

# TIER 3: Launch all tests (different port range)
PORT=7950
for test in "${TIER3_TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/tier3-${test}.txt"
    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" >/dev/null 2>&1 &
    PORT=$((PORT + 1))
    if [ $((PORT - 7950)) -ge 8 ]; then
        PORT=7950
    fi
done

echo "✅ All tiers launched simultaneously"
echo ""
echo "⏳ Waiting for results..."
echo ""

# Wait for all background jobs
wait

GLOBAL_END=$(date +%s)
GLOBAL_DURATION=$((GLOBAL_END - GLOBAL_START))

# ============================================================================
# COLLECT AND DISPLAY RESULTS (as tiers complete)
# ============================================================================

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                          RESULTS SUMMARY                                       ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

# TIER 1 Results
echo "TIER 1: Protocol + Basic Tools"
echo "────────────────────────────────────────────────────────────────────────────────"
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
            echo "✅ $test: $PASS"
        else
            echo "❌ $test: $FAIL failed"
        fi
    fi
done
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Tier 1 Summary: $TIER1_PASS passed, $TIER1_FAIL failed"
echo ""

# TIER 2 Results
echo "TIER 2: Extended Coverage"
echo "────────────────────────────────────────────────────────────────────────────────"
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
            echo "✅ $test: $PASS"
        else
            echo "❌ $test: $FAIL failed"
        fi
    fi
done
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Tier 2 Summary: $TIER2_PASS passed, $TIER2_FAIL failed"
echo ""

# TIER 3 Results
echo "TIER 3: Full Suite + Stress Tests"
echo "────────────────────────────────────────────────────────────────────────────────"
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
            echo "✅ $test: $PASS"
        elif [ "$FAIL" -eq 0 ]; then
            echo "⚠️  $test: $PASS (skipped $SKIP)"
        else
            echo "❌ $test: $FAIL failed"
        fi
    fi
done
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Tier 3 Summary: $TIER3_PASS passed, $TIER3_FAIL failed, $TIER3_SKIP skipped"
echo ""

# Final Summary
TOTAL_PASS=$((TIER1_PASS + TIER2_PASS + TIER3_PASS))
TOTAL_FAIL=$((TIER1_FAIL + TIER2_FAIL + TIER3_FAIL))
TOTAL_SKIP=$TIER3_SKIP

echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                           OVERALL SUMMARY                                      ║"
echo "╠════════════════════════════════════════════════════════════════════════════════╣"
echo "║                                                                                ║"
echo "║  Tier 1:  $TIER1_PASS passed,  $TIER1_FAIL failed                                      ║"
echo "║  Tier 2:  $TIER2_PASS passed,  $TIER2_FAIL failed                                      ║"
echo "║  Tier 3:  $TIER3_PASS passed,  $TIER3_FAIL failed,  $TIER3_SKIP skipped                   ║"
echo "║  ─────────────────────────────────                                            ║"
echo "║  TOTAL:   $TOTAL_PASS passed,  $TOTAL_FAIL failed,  $TOTAL_SKIP skipped                   ║"
echo "║                                                                                ║"
echo "║  Total Time: ${GLOBAL_DURATION}s (all tiers parallel)                                 ║"
echo "║  Tests/sec:  $(( (TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP) / (GLOBAL_DURATION + 1) ))                                              ║"
echo "║                                                                                ║"
if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "║  🎉 ALL TESTS PASSED                                                          ║"
else
    echo "║  ⚠️  FAILURES DETECTED — Review logs in $RESULTS_DIR                ║"
fi
echo "║                                                                                ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    exit 0
else
    exit 1
fi
