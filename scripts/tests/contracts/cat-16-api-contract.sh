#!/bin/bash
# cat-16-api-contract.sh — Extension↔server /sync API contract validation.
#
# These tests verify the server side of the contract, so they run on the offline
# daemon and speak as the extension itself. That identity is required: the daemon
# answers kaboom-probe requests with a canned empty envelope and adopts nothing,
# so a probe cannot prove that settings were applied or that a session was
# established. On the offline port no real extension is attached, so claiming the
# extension session here cannot disturb the user's browser.
#
# Every assertion checks an HTTP status or an observable effect. Parsing the
# reply is not an assertion — the daemon answers 400 (invalid JSON), 403 (bad
# client header) and 409 (stale generation) with well-formed JSON bodies.
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

# ── 16.1 — /sync returns a usable envelope and rejects malformed bodies ──
begin_test "16.1" "/sync returns a complete SyncResponse and rejects malformed JSON" \
    "POST a well-formed startup payload, then POST a body that is not JSON" \
    "Contract: the extension's poll loop stalls if ack/commands/next_poll_ms/server_time are missing"

run_test_16_1() {
    if ! wait_for_health 10; then
        fail "Daemon health check failed before API contract test."
        return
    fi

    post_extension "/sync" '{"ext_session_id":"contract-envelope","extension_version":"'"${VERSION}"'","settings":{"pilot_enabled":false}}'
    expect_http_status 200 "valid /sync payload" || return

    if ! check_sync_envelope "$LAST_HTTP_BODY"; then
        fail "/sync reply is missing required envelope fields. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    local reported_version
    reported_version="$(echo "$LAST_HTTP_BODY" | jq -r '.server_version // empty')"
    if [ "$reported_version" != "$VERSION" ]; then
        fail "/sync reported server_version '$reported_version', expected '$VERSION'"
        return
    fi

    post_extension "/sync" 'this is not json'
    expect_http_status 400 "malformed /sync body" || return

    pass "/sync returned a complete envelope (server_version=$reported_version) and rejected malformed JSON with 400"
}
run_test_16_1

# ── 16.2 — settings are applied, not merely accepted ──
begin_test "16.2" "settings the extension reports change server-side capture state" \
    "Report pilot_enabled=true with capture flags, then report pilot_enabled=false" \
    "Contract: a 200 that does not apply the settings silently disables capture"

run_test_16_2() {
    post_extension "/sync" '{
        "ext_session_id":"contract-settings",
        "settings":{
            "pilot_enabled":true,
            "tracking_enabled":true,
            "capture_logs":true,
            "capture_network":true,
            "capture_websocket":true,
            "capture_actions":true
        }
    }'
    expect_http_status 200 "settings payload with capture flags" || return

    local enabled_state
    enabled_state="$(capture_state_field pilot_state)"
    if [ "$enabled_state" != "enabled" ]; then
        fail "After reporting pilot_enabled=true, capture.pilot_state is '$enabled_state', expected 'enabled'"
        return
    fi

    post_extension "/sync" '{"ext_session_id":"contract-settings","settings":{"pilot_enabled":false}}'
    expect_http_status 200 "settings payload disabling pilot" || return

    local disabled_state
    disabled_state="$(capture_state_field pilot_state)"
    if [ "$disabled_state" != "explicitly_disabled" ]; then
        fail "After reporting pilot_enabled=false, capture.pilot_state is '$disabled_state', expected 'explicitly_disabled'"
        return
    fi

    pass "Reported settings were applied: pilot_state tracked enabled → explicitly_disabled"
}
run_test_16_2

# ── 16.3 — X-Kaboom-Client identity is enforced ──
begin_test "16.3" "/sync rejects requests without a recognised X-Kaboom-Client identity" \
    "Send a valid client header, an unrecognised prefix, and no header at all" \
    "Contract: the extension endpoints are unauthenticated apart from this header"

run_test_16_3() {
    post_extension "/sync" '{"ext_session_id":"header-valid","settings":{}}'
    expect_http_status 200 "recognised extension client header" || return

    post_raw "http://localhost:${PORT}/sync" \
        '{"ext_session_id":"header-invalid","settings":{}}' \
        "X-Kaboom-Client: invalid-prefix/5.8.0"
    expect_http_status 403 "unrecognised client header" || return

    post_raw "http://localhost:${PORT}/sync" '{"ext_session_id":"header-missing","settings":{}}'
    expect_http_status 403 "missing client header" || return

    pass "/sync admitted the extension identity and rejected both an unrecognised prefix and a missing header with 403"
}
run_test_16_3

# ── 16.4 — command_results shape is enforced ──
begin_test "16.4" "command_results accepts the documented shape and rejects a malformed one" \
    "POST a complete command result, then POST command_results as a non-array" \
    "Contract: results carry command outcomes back to the waiting MCP caller"

run_test_16_4() {
    post_extension "/sync" '{
        "ext_session_id":"cmd-result-valid",
        "command_results":[{
            "id":"cmd-1",
            "correlation_id":"corr-1",
            "status":"complete",
            "result":{}
        }]
    }'
    expect_http_status 200 "well-formed command_results" || return

    if ! echo "$LAST_HTTP_BODY" | jq -e '.ack == true' >/dev/null 2>&1; then
        fail "Server did not acknowledge command_results. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    post_extension "/sync" '{"ext_session_id":"cmd-result-invalid","command_results":"not-an-array"}'
    expect_http_status 400 "command_results sent as a string" || return

    pass "command_results acknowledged when well-formed and rejected with 400 when the wrong type"
}
run_test_16_4

# ── 16.5 — extension_logs reach the observe tool ──
begin_test "16.5" "extension_logs posted to /sync are retrievable through observe" \
    "POST an extension log with a unique marker, then read it back via observe(extension_logs)" \
    "Contract: accepting logs without storing them makes extension debugging impossible"

run_test_16_5() {
    local marker="UAT_CONTRACT_16_5_$$"
    post_extension "/sync" '{
        "ext_session_id":"ext-logs-contract",
        "extension_logs":[{
            "timestamp":"2026-02-07T12:00:00Z",
            "level":"info",
            "message":"'"${marker}"': Extension started",
            "source":"background",
            "category":"lifecycle"
        }]
    }'
    expect_http_status 200 "extension_logs payload" || return

    sleep 0.3
    local response text
    response="$(call_tool "observe" '{"what":"extension_logs"}')"
    text="$(extract_content_text "$response")"

    if ! check_contains "$text" "$marker"; then
        fail "observe(extension_logs) does not contain the posted marker '$marker'. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "observe(extension_logs) is missing the 'count' field. Content: $(truncate "$text")"
        return
    fi

    pass "extension_logs posted to /sync were stored and returned by observe(extension_logs)"
}
run_test_16_5

finish_category
