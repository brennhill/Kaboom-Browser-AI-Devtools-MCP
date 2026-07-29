#!/bin/bash
# cat-07-concurrency.sh — UAT tests for concurrency and resilience (3 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "7" "Concurrency & Resilience" "3"
ensure_daemon

# ── 7.1 — 10 concurrent clients get valid responses ───────
begin_test "7.1" "10 concurrent clients get valid responses" \
    "Fork 10 background processes each sending tools/list, all must succeed" \
    "Real usage has multiple MCP clients. Must handle concurrency."
run_test_7_1() {
    local concurrent_dir="$TEMP_DIR/concurrent_7_1"
    mkdir -p "$concurrent_dir"
    local total=10
    local request='{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

    # Fork 10 background processes, tracking PIDs to avoid killing daemon
    local pids=()
    for i in $(seq 1 "$total"); do
        (
            echo "$request" | "$TIMEOUT_CMD" 15 "$WRAPPER" --port "$PORT" > "$concurrent_dir/resp_${i}.txt" 2>/dev/null
        ) &
        pids+=($!)
    done

    # Wait for all with a timeout
    local waited=0
    while [ "$waited" -lt 30 ]; do
        local still_running=0
        for pid in "${pids[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                still_running=$((still_running + 1))
            fi
        done
        if [ "$still_running" -eq 0 ]; then
            break
        fi
        sleep 0.5
        waited=$((waited + 1))
    done
    # Kill only the forked test processes (not the daemon), then wait for them
    for pid in "${pids[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    # Count successes
    local success=0
    for i in $(seq 1 "$total"); do
        local resp_file="$concurrent_dir/resp_${i}.txt"
        if [ -f "$resp_file" ]; then
            local last_line
            last_line=$(grep -v '^$' "$resp_file" 2>/dev/null | tail -1)
            if echo "$last_line" | jq -e '.result.tools | length == 5' >/dev/null 2>&1; then
                success=$((success + 1))
            fi
        fi
    done

    if [ "$success" -eq "$total" ]; then
        pass "All $total concurrent clients received valid responses with 5 tools each."
    else
        fail "Only $success/$total concurrent clients received valid responses."
    fi
}
run_test_7_1

# ── 7.2 — 20 rapid sequential tool calls ──────────────────
begin_test "7.2" "20 rapid sequential tool calls" \
    "Send 20 tool calls in a tight loop, all must return valid JSON-RPC" \
    "AI agents make rapid-fire tool calls. Server must not accumulate failure state."
run_test_7_2() {
    local total=20
    local success=0
    # Exclude extension-dependent and deprecated modes.
    local modes=("page" "logs" "errors" "vitals" "actions" "tabs" "pilot" "timeline" "summarized_logs" "recordings")
    local mode_count=${#modes[@]}

    for i in $(seq 1 "$total"); do
        local mode_idx=$(( (i - 1) % mode_count ))
        local mode="${modes[$mode_idx]}"
        local resp
        resp=$(call_tool "observe" "{\"what\":\"$mode\"}")
        if check_valid_jsonrpc "$resp" && check_not_error "$resp"; then
            success=$((success + 1))
        fi
    done

    if [ "$success" -eq "$total" ]; then
        pass "All $total rapid sequential tool calls returned valid JSON-RPC responses."
    else
        fail "Only $success/$total rapid sequential calls returned valid JSON-RPC. Expected all $total."
    fi
}
run_test_7_2

# ── 7.3 — Large limit doesn't crash ───────────────────────
begin_test "7.3" "Large limit does not crash server" \
    "Request network_waterfall with limit:10000, verify valid response (not crash/timeout)" \
    "Large limit values must not cause OOM or buffer overflow."
run_test_7_3() {
    RESPONSE=$(call_tool "observe" '{"what":"network_waterfall","limit":10000}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Large limit request did not return valid JSON-RPC. Raw: $(truncate "$RESPONSE")"
        return
    fi
    if check_bridge_timeout "$RESPONSE"; then
        pass "network_waterfall with limit:10000 returned expected bridge timeout without extension (no crash)."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        fail "Large limit request returned isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "network_waterfall with limit:10000 returned valid non-error response. Content: $(truncate "$text" 200)"
}
run_test_7_3

finish_category
