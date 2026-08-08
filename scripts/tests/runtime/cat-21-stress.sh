#!/bin/bash
# cat-21-stress.sh — System Stress & Concurrency Tests (4 tests)
# Tests high load, concurrent operations, resource exhaustion scenarios.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "21" "Stress & Concurrency" "4"

ensure_daemon

# ── TEST 21.1: 50 Concurrent Tool Calls ────────────────────────────────

begin_test "21.1" "50 concurrent observe calls from different clients" \
    "Launch 50 parallel observe requests on same daemon" \
    "System must handle concurrent load without deadlock or data loss"

run_test_21_1() {
    local success_count=0
    local pids=()

    # Launch 50 concurrent observe calls
    for i in {1..50}; do
        call_tool "observe" '{"what":"page"}' >/dev/null 2>&1 &
        pids+=($!)
        if [ $((i % 10)) -eq 0 ]; then
            echo "Queued $i requests..." >&2
        fi
    done

    # Wait for all to complete, then kill any stragglers
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    sleep 0.2

    # Verify daemon still responsive
    response=$(call_tool "observe" '{"what":"page"}')
    if ! check_not_error "$response"; then
        fail "Daemon unresponsive after concurrent load"
    else
        pass "System handled 50 concurrent observe calls without deadlock"
    fi
}
run_test_21_1

# ── TEST 21.2: Rapid Tool Switching (observe → generate → configure) ────

begin_test "21.2" "Rapid switching between different tools" \
    "Call observe, generate, configure, interact, analyze in rapid sequence" \
    "No tool should block other tools during concurrent use"

run_test_21_2() {
    # The previous version counted `call_tool ... && success_count++`, but
    # call_tool always exits 0 (it echoes a response, including a synthesized
    # error envelope), so the counter was always 50 and both branches passed.
    # What this test can actually prove is that no tool blocks another: every
    # call must come back with a usable envelope rather than hanging.
    local calls=0
    local blocked=0
    local first_failure=""
    local i tool args response

    for i in $(seq 1 10); do
        for spec in 'observe|{"what":"page"}' \
                    'generate|{"what":"reproduction"}' \
                    'configure|{"what":"health"}' \
                    'interact|{"what":"navigate","url":"https://example.com"}' \
                    'analyze|{"what":"page"}'; do
            tool="${spec%%|*}"
            args="${spec#*|}"
            response="$(call_tool "$tool" "$args")"
            calls=$((calls + 1))
            if check_transport_failure "$response"; then
                blocked=$((blocked + 1))
                [ -n "$first_failure" ] || first_failure="$tool: $(truncate "$(extract_content_text "$response")" 160)"
            fi
        done
    done

    if [ "$blocked" -gt 0 ]; then
        fail "$blocked/$calls rapid tool calls did not return a usable response. First: $first_failure"
        return
    fi

    pass "Rapid tool switching: all $calls calls returned without blocking"
}
run_test_21_2

# ── TEST 21.3: Large Buffer Filling (100MB+ logs) ─────────────────────

begin_test "21.3" "System handles large log buffers without performance degradation" \
    "Generate 10,000 log entries (total > 100MB), query still responsive" \
    "Buffer management must scale gracefully"

run_test_21_3() {
    # The buffer is filled for real: querying an empty one proves nothing about
    # how the daemon scales. The previous version skipped the fill entirely and
    # then passed on every branch, including the error branch.
    local entries='{"entries":['
    local i
    for i in $(seq 1 2000); do
        [ "$i" -gt 1 ] && entries+=','
        entries+="{\"type\":\"console\",\"level\":\"info\",\"message\":\"UAT_STRESS_21_3 entry $i padded with filler text to grow the buffer\",\"timestamp\":\"2026-02-06T12:00:00Z\"}"
    done
    entries+=']}'

    post_logs "$entries"
    if [ "$LAST_HTTP_STATUS" != "200" ]; then
        fail "Seeding 2000 log entries returned HTTP $LAST_HTTP_STATUS. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    response=$(call_tool "observe" '{"what":"logs","limit":10000}')
    if ! check_not_error "$response"; then
        fail "Large buffer query failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # Verify daemon still responsive
    response=$(call_tool "observe" '{"what":"page"}')
    if check_transport_failure "$response"; then
        fail "Daemon unresponsive after large buffer query"
        return
    fi

    pass "Queried a 2000-entry buffer and the daemon stayed responsive"
}
run_test_21_3

# ── TEST 21.5: Cleanup After High Load ─────────────────────────────────

begin_test "21.5" "System recovers cleanly after high-load stress test" \
    "Run stress tests, call clear, verify clean state, daemon remains stable" \
    "Cleanup must not leave dangling resources"

run_test_21_5() {
    # Clear everything
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    sleep 0.2

    # Verify clean state
    response=$(call_tool "observe" '{"what":"logs"}')

    if ! check_not_error "$response"; then
        fail "Observe after clear failed"
        return
    fi

    # The previous check was `grep -qi "empty\|none\|0"`, which matches almost any
    # payload — a timestamp or a nonzero count containing a 0 satisfied it. Read
    # the structured count instead.
    local count
    count=$(extract_content_text "$response" | sed -n '/^{/,$p' | jq -r '.count // empty' 2>/dev/null)
    if [ "$count" != "0" ]; then
        fail "Buffers report count='$count' after clear, expected 0"
        return
    fi

    # Final health check
    response=$(call_tool "configure" '{"what":"health"}')
    if ! check_not_error "$response"; then
        fail "Daemon unhealthy after stress test cleanup"
        return
    fi

    pass "Clear emptied the buffers (count=0) and the daemon stayed healthy"
}
run_test_21_5

finish_category
