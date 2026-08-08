#!/bin/bash
# cat-14-extension-startup.sh — Extension startup sequence contract tests.
#
# These verify the server's half of the startup handshake, so they run on the
# offline daemon and speak as the extension itself. The daemon answers
# kaboom-probe requests with a canned empty envelope and adopts nothing, so a
# probe cannot prove a session was established or that settings were applied.
# On the offline port no real extension is attached, so claiming the extension
# session here cannot disturb the user's browser.
#
# Every assertion checks an HTTP status or an observable effect. Parsing the
# reply proves nothing: 400, 403 and 409 all carry well-formed JSON bodies.
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

# Reads connection_generation from the last /sync reply.
sync_generation() {
    echo "$LAST_HTTP_BODY" | jq -r '.connection_generation // empty' 2>/dev/null
}

# ── 14.1 — First sync establishes the extension session ──
begin_test "14.1" "First /sync registers the extension as connected" \
    "Send the startup payload, then read capture state back from /health" \
    "Contract: if the first sync does not register the session, every tool reports no extension"

run_test_14_1() {
    if ! wait_for_health 10; then
        fail "Daemon health check failed before /sync payload test."
        return
    fi

    post_extension "/sync" '{
        "ext_session_id":"ext-startup-test",
        "extension_version":"'"${VERSION}"'",
        "settings":{
            "pilot_enabled":false,
            "tracking_enabled":false,
            "capture_logs":true,
            "capture_network":true,
            "capture_websocket":true,
            "capture_actions":true
        }
    }'
    expect_http_status 200 "extension startup payload" || return

    if ! check_sync_envelope "$LAST_HTTP_BODY"; then
        fail "Startup /sync reply is missing required envelope fields. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    local connected client_id
    connected="$(capture_state_field extension_connected)"
    client_id="$(capture_state_field extension_client_id)"
    if [ "$connected" != "true" ]; then
        fail "After the startup sync, capture.extension_connected is '$connected', expected 'true'"
        return
    fi
    if [ "$client_id" != "kaboom-extension/${VERSION}" ]; then
        fail "capture.extension_client_id is '$client_id', expected 'kaboom-extension/${VERSION}'"
        return
    fi

    pass "Startup /sync registered the session: extension_connected=true, client_id=$client_id"
}
run_test_14_1

# ── 14.2 — Pilot toggles are honoured across consecutive syncs ──
begin_test "14.2" "Consecutive syncs move pilot state OFF → ON" \
    "Report pilot_enabled=false, then report pilot_enabled=true on the same session" \
    "Contract: a toggle the server accepts but ignores leaves pilot-gated actions refusing to run"

run_test_14_2() {
    post_extension "/sync" '{"ext_session_id":"ext-toggle-test","settings":{"pilot_enabled":false}}'
    expect_http_status 200 "pilot OFF sync" || return

    local off_state
    off_state="$(capture_state_field pilot_state)"
    if [ "$off_state" != "explicitly_disabled" ]; then
        fail "After reporting pilot_enabled=false, capture.pilot_state is '$off_state', expected 'explicitly_disabled'"
        return
    fi

    post_extension "/sync" '{"ext_session_id":"ext-toggle-test","settings":{"pilot_enabled":true}}'
    expect_http_status 200 "pilot ON sync" || return

    local on_state
    on_state="$(capture_state_field pilot_state)"
    if [ "$on_state" != "enabled" ]; then
        fail "After reporting pilot_enabled=true, capture.pilot_state is '$on_state', expected 'enabled'"
        return
    fi

    pass "Pilot transition applied server-side: explicitly_disabled → enabled"
}
run_test_14_2

# ── 14.3 — Tracking updates keep the same connection ──
begin_test "14.3" "Tracking state updates are accepted without churning the connection" \
    "Report no tracked tab, then report a tracked tab on the same session" \
    "Contract: a generation bump mid-session supersedes the extension's in-flight poll"

run_test_14_3() {
    post_extension "/sync" '{"ext_session_id":"ext-tracking-test","settings":{"tracking_enabled":false,"tracked_tab_id":0}}'
    expect_http_status 200 "tracking disabled sync" || return
    local first_generation
    first_generation="$(sync_generation)"

    post_extension "/sync" '{"ext_session_id":"ext-tracking-test","settings":{"tracking_enabled":true,"tracked_tab_id":42,"tracked_tab_url":"https://example.com"}}'
    expect_http_status 200 "tracking enabled sync" || return
    local second_generation
    second_generation="$(sync_generation)"

    if [ -z "$first_generation" ] || [ "$first_generation" != "$second_generation" ]; then
        fail "Same-session tracking update changed connection_generation ($first_generation → $second_generation); the extension's in-flight poll would be superseded"
        return
    fi

    pass "Tracking updates accepted on a stable connection (generation $second_generation held across both syncs)"
}
run_test_14_3

# ── 14.4 — Session identity rules ──
begin_test "14.4" "Any extension version is accepted, but the session id is required" \
    "Sync as an older and a newer extension, then sync with no ext_session_id" \
    "Contract: an omitted session id is a protocol error, not a silently accepted heartbeat"

run_test_14_4() {
    post_extension "/sync" '{"ext_session_id":"ext-old-version","extension_version":"5.7.0","settings":{"pilot_enabled":false}}'
    expect_http_status 200 "older extension version" || return

    post_extension "/sync" '{"ext_session_id":"ext-new-version","extension_version":"5.9.0","settings":{"pilot_enabled":false}}'
    expect_http_status 200 "newer extension version" || return

    # An absent ext_session_id cannot be matched against the live generation, so
    # the daemon must reject it rather than apply its settings. UAT payloads that
    # used the wrong field name ("session_id") landed here and were silently
    # counted as passes for as long as the assertion was "the reply is JSON".
    post_extension "/sync" '{"settings":{"pilot_enabled":true}}'
    expect_http_status 409 "sync with no ext_session_id" || return

    if ! echo "$LAST_HTTP_BODY" | jq -e '.error == "stale_connection_generation"' >/dev/null 2>&1; then
        fail "Session-less sync was rejected without the stale_connection_generation error. Body: $(truncate "$LAST_HTTP_BODY")"
        return
    fi

    pass "Both extension versions accepted; a sync with no ext_session_id was rejected with 409 stale_connection_generation"
}
run_test_14_4

# ── 14.5 — In-flight command state reaches the MCP surface ──
begin_test "14.5" "Commands the extension reports in flight are visible to observe" \
    "Report an in-progress command via /sync, then read observe(pending_commands)" \
    "Contract: without this the caller cannot tell a running command from a lost one"

run_test_14_5() {
    local marker="uat-inflight-14-5-$$"
    post_extension "/sync" '{
        "ext_session_id":"ext-inflight-test",
        "in_progress":[{
            "id":"'"${marker}"'",
            "correlation_id":"corr-14-5",
            "type":"navigate",
            "status":"running"
        }]
    }'
    expect_http_status 200 "in-progress report" || return

    sleep 0.3
    local response text
    response="$(call_tool "observe" '{"what":"pending_commands"}')"
    text="$(extract_content_text "$response")"

    if ! check_contains "$text" "$marker"; then
        fail "observe(pending_commands) does not report the in-flight command '$marker'. Content: $(truncate "$text")"
        return
    fi
    if ! echo "$text" | sed -n '/^{/,$p' | jq -e '.extension_in_progress_count >= 1' >/dev/null 2>&1; then
        fail "observe(pending_commands) reported extension_in_progress_count of zero. Content: $(truncate "$text")"
        return
    fi

    pass "In-flight command '$marker' reported via /sync surfaced through observe(pending_commands)"
}
run_test_14_5

finish_category
