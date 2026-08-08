#!/bin/bash
# test-multi-tier.sh — 6-tier parallel test execution for maximum granularity
# Tier 0 (30s):   Health check only
# Tier 1 (1min):  Core protocol + basic tools
# Tier 2 (3min):  Extended tools (observe, generate, etc)
# Tier 3 (5min):  Feature-specific (pilot, recording)
# Tier 4 (10min): New tests only
# Tier 5 (20min): Stress, concurrency, advanced

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/../../tests"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║            KABOOM UAT 6-TIER PARALLEL TEST SUITE — ALL START NOW              ║"
echo "║                                                                                ║"
echo "║ Tier 0 (30s):  Health Check                          ▶ STARTS NOW             ║"
echo "║ Tier 1 (1min): Core Protocol + Basic Tools           ▶ STARTS NOW             ║"
echo "║ Tier 2 (3min): Extended Tools (Observe, Generate)    ▶ STARTS NOW             ║"
echo "║ Tier 3 (5min): Features (Pilot, Recording, Healing)  ▶ STARTS NOW             ║"
echo "║ Tier 4 (10min): New Tests Only                       ▶ STARTS NOW             ║"
echo "║ Tier 5 (20min): Stress, Concurrency, Advanced        ▶ STARTS NOW             ║"
echo "║                                                                                ║"
echo "║ Results appear incrementally • All tiers run in parallel • Fine-grained clarity║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

RESULTS_DIR="/tmp/kaboom-uat-multitier-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results: $RESULTS_DIR"
echo ""

GLOBAL_START=$(date +%s)

# Define test tiers - EACH STARTS ON ITS OWN PORT RANGE
TIER0_TESTS=("cat-01-protocol" "cat-09-http")                              # Health/basic
TIER1_TESTS=("cat-04-configure" "cat-08-security")                         # Core safety
TIER2_TESTS=("cat-02-observe" "cat-03-generate" "cat-05-interact")         # Main tools
TIER3_TESTS=("cat-06-lifecycle" "cat-13-pilot-contract" "cat-15-pilot-success-path" "cat-18-recording" "cat-19-link-health" "cat-20-noise-persistence") # Features
TIER4_TESTS=("cat-15-extended" "cat-17-generation-logic" "cat-17-healing-logic" "cat-17-performance" "cat-18-recording-logic" "cat-30-recording-automation" "cat-18-playback-logic" "cat-19-extended" "cat-31-link-crawling" "cat-20-security" "cat-20-filtering-logic") # All new
TIER5_TESTS=("cat-07-concurrency" "cat-10-regression" "cat-11-data-pipeline" "cat-12-rich-actions" "cat-14-extension-startup" "cat-16-api-contract" "cat-29-reproduction" "cat-21-stress" "cat-22-advanced") # Heavy

echo "🚀 Launching all 6 tiers simultaneously..."
echo ""

# Launch each tier on its own port range
run_tier() {
    local tier_num=$1
    local port_start=$2
    shift 2
    local tests=("$@")

    local port=$port_start
    for test in "${tests[@]}"; do
        RESULT_FILE="$RESULTS_DIR/tier${tier_num}-${test}.txt"
        bash "$TEST_DIR/${test}.sh" "$port" "$RESULT_FILE" >/dev/null 2>&1 &
        port=$((port + 1))
    done
}

# Launch all tiers in parallel on different port ranges
run_tier 0 7800 "${TIER0_TESTS[@]}"   # Ports 7800-7801
run_tier 1 7810 "${TIER1_TESTS[@]}"   # Ports 7810-7811
run_tier 2 7820 "${TIER2_TESTS[@]}"   # Ports 7820-7822
run_tier 3 7830 "${TIER3_TESTS[@]}"   # Ports 7830-7835
run_tier 4 7850 "${TIER4_TESTS[@]}"   # Ports 7850-7861
run_tier 5 7880 "${TIER5_TESTS[@]}"   # Ports 7880-7888

echo "✅ All 6 tiers launched on separate port ranges"
echo "⏳ Waiting for results..."
echo ""

# Wait for all to complete
wait

GLOBAL_END=$(date +%s)
GLOBAL_DURATION=$((GLOBAL_END - GLOBAL_START))

# Process and display results
process_tier() {
    local tier_num=$1
    local tier_name=$2
    shift 2
    local tests=("$@")

    local tier_pass=0
    local tier_fail=0
    local tier_skip=0

    for test in "${tests[@]}"; do
        RESULT_FILE="$RESULTS_DIR/tier${tier_num}-${test}.txt"
        if [ -f "$RESULT_FILE" ]; then
            PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
            FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")
            SKIP=$(grep -c "^  SKIP:" "$RESULT_FILE" 2>/dev/null || echo "0")
            tier_pass=$((tier_pass + PASS))
            tier_fail=$((tier_fail + FAIL))
            tier_skip=$((tier_skip + SKIP))
        fi
    done

    echo "Tier $tier_num: $tier_name"
    echo "  ✅ $tier_pass passed  ❌ $tier_fail failed  ⏭️  $tier_skip skipped"
    echo ""

    echo "$tier_pass:$tier_fail:$tier_skip"
}

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                          RESULTS BY TIER                                       ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

RESULTS=$(process_tier 0 "Health Check" "${TIER0_TESTS[@]}")
IFS=':' read -r T0P T0F T0S <<< "$RESULTS"

RESULTS=$(process_tier 1 "Core Protocol + Safety" "${TIER1_TESTS[@]}")
IFS=':' read -r T1P T1F T1S <<< "$RESULTS"

RESULTS=$(process_tier 2 "Extended Tools" "${TIER2_TESTS[@]}")
IFS=':' read -r T2P T2F T2S <<< "$RESULTS"

RESULTS=$(process_tier 3 "Features (Pilot/Recording)" "${TIER3_TESTS[@]}")
IFS=':' read -r T3P T3F T3S <<< "$RESULTS"

RESULTS=$(process_tier 4 "New Tests" "${TIER4_TESTS[@]}")
IFS=':' read -r T4P T4F T4S <<< "$RESULTS"

RESULTS=$(process_tier 5 "Stress/Concurrency/Advanced" "${TIER5_TESTS[@]}")
IFS=':' read -r T5P T5F T5S <<< "$RESULTS"

# Calculate totals
TOTAL_PASS=$((T0P + T1P + T2P + T3P + T4P + T5P))
TOTAL_FAIL=$((T0F + T1F + T2F + T3F + T4F + T5F))
TOTAL_SKIP=$((T0S + T1S + T2S + T3S + T4S + T5S))
TOTAL_TESTS=$((TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP))

echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                        OVERALL SUMMARY                                         ║"
echo "╠════════════════════════════════════════════════════════════════════════════════╣"
echo "║                                                                                ║"
echo "║  Total Tests:     $TOTAL_TESTS                                                     ║"
echo "║  ✅ Passed:       $TOTAL_PASS                                                      ║"
echo "║  ❌ Failed:       $TOTAL_FAIL                                                      ║"
echo "║  ⏭️  Skipped:      $TOTAL_SKIP                                                      ║"
echo "║                                                                                ║"
echo "║  Total Time:      ${GLOBAL_DURATION}s (all 6 tiers parallel)                           ║"
echo "║  Tests/sec:       $(( TOTAL_TESTS / (GLOBAL_DURATION + 1) ))                                              ║"
echo "║                                                                                ║"

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "║  🎉 ALL TESTS PASSED!                                                       ║"
    STATUS=0
else
    echo "║  ⚠️  FAILURES DETECTED — Details in: $RESULTS_DIR        ║"
    STATUS=1
fi

echo "║                                                                                ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

exit "$STATUS"
