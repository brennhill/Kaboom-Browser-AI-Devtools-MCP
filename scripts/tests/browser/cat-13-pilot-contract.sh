#!/bin/bash
# cat-13-pilot-contract.sh — Contract tests for AI Web Pilot gating.
#
# Runs offline and drives pilot state explicitly by speaking as the extension.
# The earlier version relied on pilot being OFF "by default" and asserted only
# that some error came back — which it always did, because no extension is
# attached offline. That passed for the wrong reason and would not have caught a
# gating regression. These tests distinguish the two failure modes:
#   pilot OFF → error_code pilot_disabled
#   pilot ON  → a different error (extension_error), because no browser is attached
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7900}"
OUTPUT_FILE="${2:-/dev/null}"

init_framework "$PORT" "$OUTPUT_FILE"
begin_category "13" "Pilot State Contract Tests" "3"
ensure_daemon

# Reports the pilot state the extension claims, failing the caller if the daemon
# did not accept or apply it.
report_pilot_state() {
    local enabled="$1"
    post_extension "/sync" '{"ext_session_id":"pilot-contract","settings":{"pilot_enabled":'"${enabled}"'}}'
    expect_http_status 200 "reporting pilot_enabled=${enabled}" || return 1
    local expected="enabled"
    [ "$enabled" = "false" ] && expected="explicitly_disabled"
    local actual
    actual="$(capture_state_field pilot_state)"
    if [ "$actual" != "$expected" ]; then
        fail "Reported pilot_enabled=${enabled} but capture.pilot_state is '$actual', expected '$expected'"
        return 1
    fi
    return 0
}

# Extracts the structured error_code from an MCP tool response.
tool_error_code() {
    extract_content_text "$1" | sed -n '/^{/,$p' | jq -r '.error_code // empty' 2>/dev/null
}

# ── 13.1 — navigate is refused while pilot is OFF ──
begin_test "13.1" "navigate is refused with pilot_disabled when pilot is OFF" \
    "Report pilot_enabled=false as the extension, then call interact(navigate)" \
    "Regression guard: the pilot cache once defaulted to enabled, letting gated actions through"

run_test_13_1() {
    report_pilot_state false || return

    local response code
    response="$(call_tool "interact" '{"what":"navigate","url":"https://example.com"}')"
    code="$(tool_error_code "$response")"

    if [ "$code" != "pilot_disabled" ]; then
        fail "navigate returned error_code '$code' with pilot OFF, expected 'pilot_disabled'. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    pass "navigate refused with error_code=pilot_disabled while pilot is OFF"
}
run_test_13_1

# ── 13.2 — /sync applies both pilot states ──
begin_test "13.2" "/sync applies pilot_enabled in both directions" \
    "Report pilot OFF then ON, reading capture state back after each" \
    "Contract: a 200 that does not apply the state leaves the UI and the server disagreeing"

run_test_13_2() {
    report_pilot_state false || return
    report_pilot_state true || return
    report_pilot_state false || return

    pass "/sync applied pilot_enabled in both directions (explicitly_disabled ⇄ enabled)"
}
run_test_13_2

# ── 13.3 — gating applies to execute_js and lifts when pilot is ON ──
begin_test "13.3" "execute_js is gated with pilot OFF and no longer gated with pilot ON" \
    "Call execute_js with pilot OFF, enable pilot, then call it again" \
    "Contract: proves the refusal tracks pilot state rather than failing for an unrelated reason"

run_test_13_3() {
    report_pilot_state false || return

    local off_response off_code
    off_response="$(call_tool "interact" '{"what":"execute_js","script":"console.log(1)"}')"
    off_code="$(tool_error_code "$off_response")"
    if [ "$off_code" != "pilot_disabled" ]; then
        fail "execute_js returned error_code '$off_code' with pilot OFF, expected 'pilot_disabled'. Content: $(truncate "$(extract_content_text "$off_response")")"
        return
    fi

    report_pilot_state true || return

    # No browser is attached offline, so this cannot succeed — but it must fail
    # for a different reason. Still seeing pilot_disabled would mean the gate
    # ignores the state the extension reported.
    local on_response on_code
    on_response="$(call_tool "interact" '{"what":"execute_js","script":"console.log(1)"}')"
    on_code="$(tool_error_code "$on_response")"
    if [ "$on_code" = "pilot_disabled" ]; then
        fail "execute_js still reports pilot_disabled after the extension enabled pilot; the gate ignores reported state"
        return
    fi

    pass "execute_js gated with pilot OFF (pilot_disabled) and released with pilot ON (error_code=${on_code:-none})"
}
run_test_13_3

finish_category
