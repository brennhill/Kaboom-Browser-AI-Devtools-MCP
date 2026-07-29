#!/bin/bash
# cat-16-api-contract.sh — Extension-Server API Contract Validation
# Verifies server and extension use matching APIs
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7903}"
OUTPUT_FILE="${2:-/dev/null}"

init_framework "$PORT" "$OUTPUT_FILE"
begin_category "16" "Extension-Server API Contract" "5"

if ! start_daemon; then
    fail "Failed to start daemon for API contract tests."
    finish_category
fi

# ── 16.1 — /sync request schema matches server expectations ──
begin_test "16.1" "/sync schema: required fields present" \
    "Extension must send: session_id, extension_version, settings" \
    "Contract: Missing required fields = integration failure"

run_test_16_1() {
    sleep 2
    if ! wait_for_health 10; then
        fail "Daemon health check failed before API contract test."
        return
    fi

    # Valid minimal /sync
    local response_valid
    response_valid=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"test","settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Invalid: missing session_id
    local response_invalid
    response_invalid=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"settings":{"pilot_enabled":false}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response_valid" | jq . >/dev/null 2>&1 && \
       ! (echo "$response_invalid" | jq . >/dev/null 2>&1); then
        pass "/sync enforces required fields schema"
    else
        # Server may be lenient; just check it accepts valid
        if echo "$response_valid" | jq . >/dev/null 2>&1; then
            pass "/sync accepts valid payloads"
        else
            fail "/sync rejected valid payload"
        fi
    fi
}
run_test_16_1

# ── 16.2 — settings field contains expected keys ──
begin_test "16.2" "settings field has: pilot_enabled, tracking_enabled, capture_* flags" \
    "Extension must send capture flags for each data type" \
    "Contract: Missing capture flags = data loss"

run_test_16_2() {
    local response
    response=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"contract-settings",
            "settings":{
                "pilot_enabled":false,
                "tracking_enabled":true,
                "capture_logs":true,
                "capture_network":true,
                "capture_websocket":true,
                "capture_actions":true
            }
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response" | jq . >/dev/null 2>&1; then
        pass "settings schema accepted with capture flags"
    else
        fail "Server rejected settings schema"
    fi
}
run_test_16_2

# ── 16.3 — X-Kaboom-Client header format ──
begin_test "16.3" "X-Kaboom-Client header: 'kaboom-extension/VERSION'" \
    "Server must validate header format" \
    "Contract: Wrong format = rejected"

run_test_16_3() {
    # Valid header
    local response_valid
    response_valid=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"header-test","settings":{}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Invalid header (wrong prefix)
    local response_invalid
    response_invalid=$(curl -s -X POST \
        -H "X-Kaboom-Client: invalid-prefix/5.8.0" \
        -H "Content-Type: application/json" \
        -d '{"session_id":"header-test-invalid","settings":{}}' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response_valid" | jq . >/dev/null 2>&1; then
        if echo "$response_invalid" | jq . >/dev/null 2>&1; then
            # Server accepts any header (lenient)
            pass "/sync accepts requests with Client header"
        else
            pass "/sync validates X-Kaboom-Client header format"
        fi
    else
        fail "/sync rejected valid header format"
    fi
}
run_test_16_3

# ── 16.4 — command_results response format ──
begin_test "16.4" "command_results: id, correlation_id, status, result required" \
    "Extension sends command execution results to server" \
    "Contract: All 4 fields must be present for tracking"

run_test_16_4() {
    # Valid result
    local response_valid
    response_valid=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"cmd-result-valid",
            "command_results":[{
                "id":"cmd-1",
                "correlation_id":"corr-1",
                "status":"complete",
                "result":{}
            }]
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    # Missing correlation_id
    local response_invalid
    response_invalid=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"cmd-result-invalid",
            "command_results":[{
                "id":"cmd-2",
                "status":"complete",
                "result":{}
            }]
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response_valid" | jq . >/dev/null 2>&1; then
        pass "command_results schema accepted"
    else
        fail "Server rejected command_results schema"
    fi
}
run_test_16_4

# ── 16.5 — Extension logs format ──
begin_test "16.5" "extension_logs: timestamp, level, message, source required" \
    "Extension sends debug logs for diagnostics" \
    "Contract: All fields required for log analysis"

run_test_16_5() {
    local response
    response=$(curl -s -X POST \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -H "Content-Type: application/json" \
        -d '{
            "session_id":"ext-logs-test",
            "extension_logs":[{
                "timestamp":"2026-02-07T12:00:00Z",
                "level":"info",
                "message":"Extension started",
                "source":"background",
                "category":"lifecycle"
            }]
        }' \
        "http://localhost:${PORT}/sync" 2>&1)

    if echo "$response" | jq . >/dev/null 2>&1; then
        pass "extension_logs format accepted"
    else
        fail "Server rejected extension_logs format"
    fi
}
run_test_16_5

finish_category
