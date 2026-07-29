#!/bin/bash
# cat-12-rich-actions.sh — UAT tests for Rich Action Results.
# Tests BOTH schema contract AND behavioral contract:
#   - Schema: analyze param exists, descriptions guide AI
#   - Behavior: perf_diff produces correct output (via Go unit tests)
#   - Behavior: command_result response has timing_ms field
#
# Run: bash scripts/tests/tools/cat-12-rich-actions.sh [port] [results-file]
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "12" "Rich Action Results" "12"
ensure_daemon

# ── 12.1 — Schema: analyze boolean param exists ──────────
begin_test "12.1" "Schema includes 'analyze' boolean param" \
    "tools/list must have analyze:{type:'boolean'} in interact inputSchema" \
    "analyze is the opt-in flag for detailed interaction profiling."
run_test_12_1() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
    if [ -z "$response" ]; then
        fail "No response from tools/list."
        return
    fi

    local analyze_type
    analyze_type=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .inputSchema.properties.analyze.type // empty' 2>/dev/null)

    if [ "$analyze_type" = "boolean" ]; then
        pass "interact schema has analyze:{type:'boolean'}."
    else
        fail "interact schema missing analyze boolean. Got type='$analyze_type'."
    fi
}
run_test_12_1

# ── 12.2 — Schema: analyze description mentions profiling ─
begin_test "12.2" "analyze description mentions profiling/performance/timing" \
    "analyze param description must guide the AI on when to use it" \
    "Without a good description, AI will either never use or always use analyze."
run_test_12_2() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')

    local analyze_desc
    analyze_desc=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .inputSchema.properties.analyze.description // empty' 2>/dev/null)

    if [ -z "$analyze_desc" ]; then
        fail "No analyze param description found in schema."
        return
    fi

    # Must contain at least one of: profil*, performance, timing, breakdown
    if echo "$analyze_desc" | grep -qiE "profil|performance|timing|breakdown"; then
        pass "analyze description guides AI: '$(truncate "$analyze_desc" 120)'"
    else
        fail "analyze description doesn't mention profiling/performance/timing/breakdown. Got: '$analyze_desc'"
    fi
}
run_test_12_2

# ── 12.3 — Tool description mentions rich action result fields ─
begin_test "12.3" "interact description mentions perf_diff or timing" \
    "Tool description must tell AI that actions return performance/timing data" \
    "AI discovers features from tool descriptions — if perf_diff isn't mentioned, AI won't use it."
run_test_12_3() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')

    local tool_desc
    tool_desc=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .description // empty' 2>/dev/null)

    if [ -z "$tool_desc" ]; then
        fail "No interact tool description found."
        return
    fi

    # Must mention perf_diff, performance, or timing (not just "diff" which is too broad)
    if echo "$tool_desc" | grep -qiE "perf_diff|performance|timing"; then
        pass "interact description mentions performance/perf_diff/timing."
    else
        fail "interact description doesn't mention perf_diff/performance/timing. AI won't discover perf_diff. Got: '$(truncate "$tool_desc" 200)'"
    fi
}
run_test_12_3

# ── 12.4 — Behavioral: perf_diff summary doesn't say 'regressed 0%' ─
begin_test "12.4" "perf_diff summary says 'unchanged' for delta=0 (not 'regressed')" \
    "Go unit test: when all metrics have delta=0, summary must not say 'regressed'" \
    "Saying 'regressed 0%' is misleading. delta=0 = unchanged."
run_test_12_4() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestSummary_DeltaZeroSaysUnchanged" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok"; then
        pass "delta=0 summary correctly says 'unchanged', not 'regressed'."
    else
        fail "delta=0 summary still says 'regressed'. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_4

# ── 12.5 — Behavioral: perf_diff metrics have rating on Web Vitals ─
begin_test "12.5" "perf_diff rating field exists on Web Vitals metrics" \
    "Go unit test: LCP, FCP, TTFB, CLS metrics include rating: good/needs_improvement/poor" \
    "Rating gives AI immediate Web Vitals assessment without threshold lookup."
run_test_12_5() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestPerfDiff_LCP_Rating|TestPerfDiff_CLS_Rating" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "LCP and CLS metrics have correct rating values (good/needs_improvement/poor)."
    else
        fail "Rating tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_5

# ── 12.6 — Behavioral: perf_diff verdict uses valid enum ─
begin_test "12.6" "perf_diff verdict enum: improved/regressed/mixed/unchanged" \
    "Go unit tests: all 4 verdict values are correctly computed" \
    "Invalid verdict breaks AI decision-making."
run_test_12_6() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestPerfDiff_Verdict" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "All 4 verdict values (improved/regressed/mixed/unchanged) computed correctly."
    else
        fail "Verdict tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_6

# ── 12.7 — Behavioral: perf_diff summary < 200 chars with no redundant sign ─
begin_test "12.7" "perf_diff summary: <200 chars, no redundant signs" \
    "Go unit tests: summary is concise and uses absolute percentages" \
    "Redundant signs waste tokens. 'improved -57%' should be 'improved 57%'."
run_test_12_7() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestSummary_NoRedundant|TestSummary_Under200|TestSummary_Regression" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "Summary is concise (<200 chars), uses absolute percentages, no redundant signs."
    else
        fail "Summary format tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_7

# ── 12.8 — Behavioral: snake_case JSON deserializes into PerformanceSnapshot ─
begin_test "12.8" "snake_case JSON → PerformanceSnapshot with all Web Vitals" \
    "Go unit test: extension snake_case JSON populates all PerformanceSnapshot fields" \
    "If JSON tags revert to camelCase, FCP/LCP/TTFB/INP/CLS all deserialize to zero."
run_test_12_8() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestSnapshotJSON_AllWebVitalsDeserialize" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok"; then
        pass "snake_case JSON correctly populates all PerformanceSnapshot fields (FCP, LCP, TTFB, INP, CLS, DCL)."
    else
        fail "snake_case deserialization broken. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_8

# ── 12.9 — Behavioral: FCP and TTFB have correct Web Vitals ratings ─
begin_test "12.9" "FCP and TTFB ratings follow Web Vitals thresholds" \
    "Go unit test: FCP needs_improvement (1800-3000ms), TTFB poor (>1800ms)" \
    "Rating thresholds must match Web Vitals standards. Wrong thresholds mislead AI."
run_test_12_9() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestPerfDiff_FCP_NeedsImprovement|TestPerfDiff_TTFB_Poor" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "FCP needs_improvement at 2500ms, TTFB poor at 2000ms — thresholds correct."
    else
        fail "FCP/TTFB rating tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_9

# ── 12.10 — Behavioral: full Web Vitals through snapshot→diff pipeline ─
begin_test "12.10" "Full Web Vitals (FCP+LCP+TTFB+CLS) produce ratings through snapshot→diff pipeline" \
    "Go unit test: PerformanceSnapshot → SnapshotToPageLoadMetrics → ComputePerfDiff with all 4 Web Vitals" \
    "End-to-end pipeline must preserve all Web Vitals. A broken mapping means missing metrics in perf_diff."
run_test_12_10() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestPerfDiff_FullWebVitals_AllRatings" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok"; then
        pass "Full pipeline: snapshot → PageLoadMetrics → perf_diff with FCP/LCP/TTFB/CLS ratings verified."
    else
        fail "Full Web Vitals pipeline test failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_10

# ── 12.11 — Behavioral: UserTiming JSON round-trip ─
begin_test "12.11" "UserTiming marks/measures survive JSON round-trip" \
    "Go unit test: JSON with user_timing → unmarshal → verify marks/measures → marshal → verify output" \
    "UserTiming is the newest snapshot field. Must survive serialization/deserialization."
run_test_12_11() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./internal/performance/ -run "TestSnapshotJSON_UserTimingRoundTrip|TestSnapshotJSON_UserTimingOmitted" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "UserTiming marks/measures survive JSON round-trip. Omitted when absent."
    else
        fail "UserTiming round-trip tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_11

# ── 12.12 — Behavioral: dom_summary and analyze fields pass through command results ─
begin_test "12.12" "dom_summary/analyze fields pass through command_result" \
    "Go unit test: extension result with dom_summary/timing/analysis → observe(command_result) returns them" \
    "Extension enriches click results with DOM changes. If passthrough is broken, AI loses mutation context."
run_test_12_12() {
    local go_result
    go_result=$("$TIMEOUT_CMD" 30 go test ./cmd/browser-agent/ -run "TestRichAction_DomSummaryPassthrough|TestRichAction_PerfDiffWithFullWebVitals" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "dom_summary, timing, analysis pass through. Full Web Vitals perf_diff with ratings verified."
    else
        fail "Command result passthrough tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_12

finish_category
