#!/bin/bash
# cat-14-extension-startup.sh — Extension startup sequence contract tests
# Verifies extension initializes correctly and sends proper API payloads
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7901}"
OUTPUT_FILE="${2:-/dev/null}"

init_framework "$PORT" "$OUTPUT_FILE"
begin_category "14" "Extension Startup Sequence" "5"

if ! start_daemon; then
    fail "Failed to start daemon for extension startup contract tests."
    finish_category
fi

# ── 14.1 — Extension sends well-formed /sync payload ──
begin_test "14.1" "Extension /sync payload has required fields" \
    "Simulate extension first sync after startup" \
    "Contract: pilot_enabled, tracking_enabled, session_id must always be present"

run_test_14_1() {
    sleep 2
    if ! wait_for_health 10; then
        fail "Daemon health check failed before /sync payload test."
        return
    fi

    local response
    response=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"ext-startup-test",
            "extension_version":"5.8.0",
            "settings":{
                "pilot_enabled":false,
                "tracking_enabled":false,
                "capture_logs":true,
                "capture_network":true,
                "capture_websocket":true,
                "capture_actions":true
            }
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response" | jq . >/dev/null 2>&1; then
        pass "/sync payload well-formed and accepted"
    else
        fail "/sync rejected startup payload"
    fi
}
run_test_14_1

# ── 14.2 — Extension transitions: pilot OFF → ON ──
begin_test "14.2" "Extension can toggle pilot state in consecutive syncs" \
    "Simulate: initial sync (pilot OFF) → user toggles → second sync (pilot ON)" \
    "Contract: Server must handle pilot state transitions gracefully"

run_test_14_2() {
    # First sync: pilot OFF
    local response1
    response1=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-toggle-test-1","settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Second sync: pilot ON
    local response2
    response2=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-toggle-test-1","settings":{"pilot_enabled":true}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response1" | jq . >/dev/null 2>&1 && echo "$response2" | jq . >/dev/null 2>&1; then
        pass "Pilot state transitions handled correctly"
    else
        fail "Server rejected pilot state transition"
    fi
}
run_test_14_2

# ── 14.3 — Extension sends tracking state correctly ──
begin_test "14.3" "Extension tracking_enabled field changes are accepted" \
    "Simulate: no tab tracked → user tracks tab → server acknowledges" \
    "Contract: Server must accept tracking state updates"

run_test_14_3() {
    # Tab not tracked
    local response1
    response1=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-tracking-test","settings":{"tracking_enabled":false,"tracked_tab_id":0}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Tab is now tracked
    local response2
    response2=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-tracking-test","settings":{"tracking_enabled":true,"tracked_tab_id":42,"tracked_tab_url":"https://example.com"}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response1" | jq . >/dev/null 2>&1 && echo "$response2" | jq . >/dev/null 2>&1; then
        pass "Tracking state changes accepted"
    else
        fail "Server rejected tracking state update"
    fi
}
run_test_14_3

# ── 14.4 — Extension startup with version mismatch handling ──
begin_test "14.4" "Server handles version mismatches gracefully" \
    "Extension may have different version than server" \
    "Contract: Server must accept requests regardless of version"

run_test_14_4() {
    # Old extension version
    local response_old
    response_old=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/5.7.0" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-old-version","extension_version":"5.7.0","settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Future extension version
    local response_new
    response_new=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/5.9.0" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"ext-new-version","extension_version":"5.9.0","settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response_old" | jq . >/dev/null 2>&1 && echo "$response_new" | jq . >/dev/null 2>&1; then
        pass "Version mismatches handled gracefully"
    else
        fail "Server rejected version mismatch"
    fi
}
run_test_14_4

# ── 14.5 — Extension command result payload format ──
begin_test "14.5" "Extension sends command results in correct format" \
    "Extension receives command from server, sends result back" \
    "Contract: command_results array with id, correlation_id, status, result"

run_test_14_5() {
    local response
    response=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"ext-cmd-result-test",
            "command_results":[{
                "id":"cmd-123",
                "correlation_id":"corr-456",
                "status":"complete",
                "result":{"success":true}
            }]
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response" | jq . >/dev/null 2>&1; then
        pass "Command result format accepted"
    else
        fail "Server rejected command result format"
    fi
}
run_test_14_5

finish_category
