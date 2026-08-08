#!/bin/bash
# cat-17-performance.sh — Test Generation Performance & Stress Tests (5 tests)
#
# Every test here previously called generate() with {"format":..., "actions":[...]}
# — parameters the tool does not define. The tool requires "what", so all five
# calls failed with missing_param. Four of the five reported pass anyway, via
# branches like "Template rendering feature pending (planned)", so the category
# looked green while generating nothing. Only 17.19 had a real failure path, and
# it was the only one reporting the breakage.
#
# The generator reads captured actions from the daemon's buffer rather than
# taking them as arguments, so these tests seed the buffer over the extension
# data pipeline and then measure generation against it. Each test enforces the
# latency budget it claims.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "17.performance" "Test Generation: Performance & Stress" "5"

ensure_daemon

# Seeds the action buffer with N synthetic recorded actions.
seed_actions() {
    local count="$1"
    local payload='{"actions":['
    local i
    for i in $(seq 1 "$count"); do
        [ "$i" -gt 1 ] && payload+=','
        payload+="{\"type\":\"click\",\"timestamp\":$((1738843200000 + i)),\"url\":\"https://uat.example.com/step$i\",\"selectors\":{\"css\":\"#btn$i\",\"text\":\"Button $i\"}}"
    done
    payload+=']}'
    post_extension "/enhanced-actions" "$payload"
    if [ "$LAST_HTTP_STATUS" != "200" ]; then
        fail "Seeding $count actions returned HTTP $LAST_HTTP_STATUS. Body: $(truncate "$LAST_HTTP_BODY")"
        return 1
    fi
    return 0
}

# Milliseconds elapsed since a `date +%s%N` reading.
millis_since() {
    echo "$(( ($(date +%s%N) - $1) / 1000000 ))"
}

# Parses the structured payload a tool appends after its summary line.
tool_payload() {
    extract_content_text "$1" | sed -n '/^{/,$p'
}

# ── TEST 17.19: Generate Test from 100-Action Sequence ───────────────────

begin_test "17.19" "generate({what:'test'}) handles 100 recorded actions" \
    "Seed 100 captured actions, generate a test, require completion under 10s" \
    "Performance must scale for complex user journeys"

run_test_17_19() {
    seed_actions 100 || return

    local start
    start=$(date +%s%N)
    response=$(call_tool "generate" '{"what":"test","test_name":"perf-100-actions"}')
    local duration_ms
    duration_ms=$(millis_since "$start")

    if ! check_not_error "$response"; then
        fail "100-action generation failed. Content: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    local included
    included="$(tool_payload "$response" | jq -r '.action_count // .metadata.actions_included // empty' 2>/dev/null)"
    if [ "$included" != "100" ]; then
        fail "Generated test covered '$included' actions, expected 100. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    if [ "$duration_ms" -ge 10000 ]; then
        fail "100-action generation took ${duration_ms}ms, above the 10000ms budget"
        return
    fi

    pass "Generated a 100-action test in ${duration_ms}ms (< 10s)"
}
run_test_17_19

# ── TEST 17.20: Concurrent Test Generation Requests ───────────────────

begin_test "17.20" "5 concurrent generate() calls don't interfere" \
    "Issue 5 generation requests in parallel and check every response" \
    "Concurrency must not cause data corruption or deadlocks"

run_test_17_20() {
    seed_actions 20 || return

    local out_dir="$TEMP_DIR/concurrent-17-20"
    mkdir -p "$out_dir"

    local pids=()
    local i
    for i in $(seq 1 5); do
        call_tool "generate" "{\"what\":\"test\",\"test_name\":\"concurrent-$i\"}" \
            >"$out_dir/$i.json" 2>/dev/null &
        pids+=($!)
    done
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    # Every response is inspected. Discarding them, as this test used to, makes
    # a deadlock or a corrupted envelope indistinguishable from success.
    for i in $(seq 1 5); do
        local response
        response="$(cat "$out_dir/$i.json" 2>/dev/null)"
        if ! check_valid_jsonrpc "$response"; then
            fail "Concurrent generate #$i returned no valid JSON-RPC envelope: $(truncate "$response")"
            return
        fi
        if check_is_error "$response"; then
            fail "Concurrent generate #$i failed: $(truncate "$(command_failure_message "$response")")"
            return
        fi
    done

    pass "All 5 concurrent generate() calls returned successful, well-formed responses"
}
run_test_17_20

# ── TEST 17.21: SARIF Export Over a Large Error Set ────────

begin_test "17.21" "generate({what:'sarif'}) over a large error set" \
    "Seed 200 captured errors, export SARIF, require valid JSON under 10MB and 5s" \
    "Large exports must remain performant"

run_test_17_21() {
    local entries='{"entries":['
    local i
    for i in $(seq 1 200); do
        [ "$i" -gt 1 ] && entries+=','
        entries+="{\"type\":\"console\",\"level\":\"error\",\"message\":\"UAT_PERF_17_21 Error $i\",\"url\":\"https://uat.example.com/bundle.js\",\"source\":\"https://uat.example.com/bundle.js\",\"line\":$i,\"column\":1,\"timestamp\":\"2026-02-06T12:00:00Z\"}"
    done
    entries+=']}'

    post_logs "$entries"
    if [ "$LAST_HTTP_STATUS" != "200" ]; then
        fail "Seeding 200 errors returned HTTP $LAST_HTTP_STATUS. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    local start
    start=$(date +%s%N)
    response=$(call_tool "generate" '{"what":"sarif"}')
    local duration_ms
    duration_ms=$(millis_since "$start")

    if ! check_not_error "$response"; then
        fail "SARIF export failed. Content: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    local payload
    payload="$(tool_payload "$response")"
    if ! echo "$payload" | jq -e '.runs | type == "array"' >/dev/null 2>&1; then
        fail "SARIF export is not a valid SARIF document (no runs array). Content: $(truncate "$payload")"
        return
    fi

    local bytes=${#payload}
    if [ "$bytes" -gt 10485760 ]; then
        fail "SARIF export is ${bytes} bytes, above the 10MB ceiling"
        return
    fi
    if [ "$duration_ms" -ge 5000 ]; then
        fail "SARIF export took ${duration_ms}ms, above the 5000ms budget"
        return
    fi

    pass "Exported valid SARIF (${bytes} bytes) in ${duration_ms}ms"
}
run_test_17_21

# ── TEST 17.22: Reproduction Script Generation ────────────────────────

begin_test "17.22" "generate({what:'reproduction'}) renders quickly" \
    "Generate a reproduction script from the seeded buffer, require completion under 2s" \
    "A second generator path must not add significant overhead"

run_test_17_22() {
    seed_actions 50 || return

    local start
    start=$(date +%s%N)
    response=$(call_tool "generate" '{"what":"reproduction"}')
    local duration_ms
    duration_ms=$(millis_since "$start")

    if ! check_not_error "$response"; then
        fail "Reproduction generation failed. Content: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    if ! echo "$(tool_payload "$response")" | jq -e '.script | type == "string" and length > 0' >/dev/null 2>&1; then
        fail "Reproduction response carries no script. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    if [ "$duration_ms" -ge 2000 ]; then
        fail "Reproduction generation took ${duration_ms}ms, above the 2000ms budget"
        return
    fi

    pass "Rendered a reproduction script in ${duration_ms}ms (< 2s)"
}
run_test_17_22

# ── TEST 17.23: Stability Under Repeated Generation ────────────

begin_test "17.23" "Generate 10 tests in sequence without degradation" \
    "Run 10 sequential generations over a 50-action buffer, require the batch under 20s" \
    "Long-running operations must free memory properly"

run_test_17_23() {
    seed_actions 50 || return

    local start
    start=$(date +%s%N)
    local i
    for i in $(seq 1 10); do
        response=$(call_tool "generate" "{\"what\":\"test\",\"test_name\":\"batch-$i\"}")
        if ! check_not_error "$response"; then
            fail "Sequential generation #$i failed. Content: $(truncate "$(command_failure_message "$response")")"
            return
        fi
    done
    local duration_ms
    duration_ms=$(millis_since "$start")

    if [ "$duration_ms" -ge 20000 ]; then
        fail "10 sequential generations took ${duration_ms}ms, above the 20000ms budget"
        return
    fi

    pass "Completed 10 sequential generations in ${duration_ms}ms with no failures"
}
run_test_17_23

finish_category
