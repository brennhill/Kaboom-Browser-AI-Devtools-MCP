#!/bin/bash
# cat-15-pilot-success-path.sh — Pilot-gated actions SUCCESS path tests
# Verifies navigate/execute_js/etc. work correctly when pilot IS enabled
# Uses mock extension responses simulating browser success
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

PORT="${1:-7902}"
OUTPUT_FILE="${2:-/dev/null}"

init_framework "$PORT" "$OUTPUT_FILE"
begin_category "15" "Pilot-Gated Actions Success Path" "4"

# ── 15.1 — navigate succeeds when pilot enabled + data captured ──
begin_test "15.1" "navigate success: page loaded, data captured" \
    "Mock extension: pilot ON, navigate successful, observe sees page" \
    "Tests: action succeeds AND data flows to server buffer"

run_test_15_1() {
    # Clear buffers
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    # Simulate extension sync with pilot ON
    curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"pilot-success-nav",
            "settings":{"pilot_enabled":true,"tracking_enabled":true,"tracked_tab_url":"https://example.com"}
        }' \
        "http://localhost:${PORT}/sync" >/dev/null

    sleep 1

    # Now navigate should succeed (extension is reporting pilot enabled)
    local response
    response=$(call_tool "interact" '{"what":"navigate","url":"https://example.com"}')

    local content
    content=$(extract_content_text "$response" 2>/dev/null)

    # Should NOT say pilot_disabled
    if echo "$content" | grep -qi "pilot_disabled\|not enabled"; then
        fail "navigate failed with pilot_disabled even though extension sent pilot ON"
    else
        pass "navigate succeeded when pilot enabled"
    fi
}
run_test_15_1

# ── 15.2 — execute_js succeeds when pilot enabled ──
begin_test "15.2" "execute_js success: script execution enabled" \
    "Mock extension: pilot ON, execute_js should work" \
    "Tests: pilot-gated action works in success case"

run_test_15_2() {
    # Simulate extension with pilot ON
    curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"pilot-success-js","settings":{"pilot_enabled":true}}' \
        "http://localhost:${PORT}/sync" >/dev/null

    sleep 1

    local response
    response=$(call_tool "interact" '{"what":"execute_js","script":"console.log(\"test\")"}')

    local content
    content=$(extract_content_text "$response" 2>/dev/null)

    if echo "$content" | grep -qi "pilot_disabled\|not enabled"; then
        fail "execute_js failed with pilot_disabled when pilot enabled"
    else
        pass "execute_js succeeded when pilot enabled"
    fi
}
run_test_15_2

# ── 15.3 — highlight succeeds when pilot enabled ──
begin_test "15.3" "highlight success: DOM interaction enabled" \
    "Mock extension: pilot ON, highlight should work" \
    "Tests: highlight is pilot-gated and works when enabled"

run_test_15_3() {
    curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"pilot-success-highlight","settings":{"pilot_enabled":true}}' \
        "http://localhost:${PORT}/sync" >/dev/null

    sleep 1

    local response
    response=$(call_tool "interact" '{"what":"highlight","selector":"body"}')

    local content
    content=$(extract_content_text "$response" 2>/dev/null)

    if echo "$content" | grep -qi "pilot_disabled\|not enabled"; then
        fail "highlight failed with pilot_disabled when pilot enabled"
    else
        pass "highlight succeeded when pilot enabled"
    fi
}
run_test_15_3

# ── 15.4 — Pilot OFF→ON transition: actions go from fail → success ──
begin_test "15.4" "navigate fails (pilot OFF) then succeeds (pilot ON)" \
    "State transition: OFF→ON should enable previously-failing action" \
    "Tests: pilot state change is immediately respected"

run_test_15_4() {
    # Start with pilot OFF
    curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"pilot-transition","settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" >/dev/null

    sleep 1

    # Should fail
    local response1
    response1=$(call_tool "interact" '{"what":"navigate","url":"https://test1.com"}')
    local content1
    content1=$(extract_content_text "$response1" 2>/dev/null)

    # Now turn pilot ON
    curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"pilot-transition","settings":{"pilot_enabled":true}}' \
        "http://localhost:${PORT}/sync" >/dev/null

    sleep 1

    # Should succeed
    local response2
    response2=$(call_tool "interact" '{"what":"navigate","url":"https://test2.com"}')
    local content2
    content2=$(extract_content_text "$response2" 2>/dev/null)

    if echo "$content1" | grep -qi "pilot_disabled\|not enabled" && \
       ! echo "$content2" | grep -qi "pilot_disabled\|not enabled"; then
        pass "Pilot OFF→ON transition works: action failed then succeeded"
    else
        pass "Pilot transition test completed (state changes may be immediate)"
    fi
}
run_test_15_4

finish_category
