#!/bin/bash
# cat-15-pilot-success-path.sh — Pilot-gated actions actually execute in the page.
#
# Runs connected, against the real extension and the daemon-served fixture.
#
# The previous version POSTed /sync as kaboom-probe to "simulate pilot ON" and
# then asserted only that the reply did not contain the string "pilot_disabled".
# Both halves were hollow: the daemon answers probes with a canned envelope and
# adopts nothing, so the simulation never happened; and an absence-of-one-string
# check reports pass for a timeout, a lost tab, or any other failure. Pilot
# gating is now covered offline in cat-13, where pilot state is controllable
# without touching the user's browser.
#
# What remains here is the coverage nothing else provides: cat-33 sweeps every
# MCP mode but only asserts "no error", so a hollow success envelope passes.
# These tests assert values computed inside the page, which a stubbed or
# no-op action cannot produce.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7902}"
OUTPUT_FILE="${2:-/dev/null}"

init_framework "$PORT" "$OUTPUT_FILE"
begin_category "15" "Pilot-Gated Actions Success Path" "4"

if ! start_daemon; then
    fail "Failed to start daemon for pilot success path tests."
    finish_category
fi

# Parses the structured JSON payload an MCP tool appends after its summary line.
tool_payload() {
    extract_content_text "$1" | sed -n '/^{/,$p'
}

# Navigates the tracked tab to the daemon-served fixture so each test starts
# from a known DOM instead of whatever page happened to be tracked.
goto_fixture() {
    local response
    response="$(call_tool "interact" \
        '{"what":"navigate_and_wait_for","url":'"$(json_string "$(connected_fixture_url)")"',"wait_for":"#sf-btn"}')"
    if ! check_valid_jsonrpc "$response" || check_is_error "$response"; then
        fail "Could not reach the interaction fixture: $(truncate "$(command_failure_message "$response")")"
        return 1
    fi
    return 0
}

# ── 15.1 — navigate actually moves the browser ──
begin_test "15.1" "navigate lands the tracked tab on the requested URL" \
    "Navigate to the fixture, then ask observe(page) where the browser actually is" \
    "Contract: a success envelope that did not move the browser is indistinguishable from a no-op"

run_test_15_1() {
    goto_fixture || return

    local response text
    response="$(call_tool "observe" '{"what":"page"}')"
    text="$(extract_content_text "$response")"

    if ! check_contains "$text" "interact.html"; then
        fail "After navigate, observe(page) does not report the fixture URL. Content: $(truncate "$text")"
        return
    fi

    pass "navigate moved the tracked tab to the fixture and observe(page) confirms it"
}
run_test_15_1

# ── 15.2 — execute_js runs in the page and returns a computed value ──
begin_test "15.2" "execute_js returns a value computed inside the page" \
    "Evaluate an expression that only the real page can answer, then check the value" \
    "Contract: proves the script executed rather than the call merely being accepted"

run_test_15_2() {
    goto_fixture || return

    # Combines arithmetic the daemon cannot fake with a DOM read that only the
    # fixture can answer, so a stubbed executor cannot produce this string.
    local response payload value
    response="$(call_tool "interact" \
        '{"what":"execute_js","script":"return (6*7) + \"-\" + (document.querySelector(\"#sf-btn\") ? \"btn\" : \"nobtn\")"}')"
    if check_is_error "$response"; then
        fail "execute_js failed on the fixture: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # The script's value lands in `return_value`. `result` is the execution
    # envelope ({"success":true}) and carries nothing the page computed, so
    # reading it would assert only that the call was accepted.
    payload="$(tool_payload "$response")"
    value="$(echo "$payload" | jq -r '.return_value // empty' 2>/dev/null)"

    if [ "$value" != "42-btn" ]; then
        fail "execute_js return_value was '$value', expected '42-btn' (payload: $(truncate "$payload" 200))"
        return
    fi

    pass "execute_js evaluated in the page and returned the computed value 42-btn"
}
run_test_15_2

# ── 15.3 — highlight queries the real DOM ──
begin_test "15.3" "highlight resolves a real selector and rejects a missing one" \
    "Highlight an element the fixture defines, then highlight a selector that cannot match" \
    "Contract: an action that succeeds for every selector is not reading the DOM"

run_test_15_3() {
    goto_fixture || return

    local present
    present="$(call_tool "interact" '{"what":"highlight","selector":"#sf-btn"}')"
    if check_is_error "$present"; then
        fail "highlight failed on '#sf-btn', which the fixture defines: $(truncate "$(command_failure_message "$present")")"
        return
    fi

    local missing
    missing="$(call_tool "interact" '{"what":"highlight","selector":"#kaboom-absent-'"$$"'"}')"
    if ! check_is_error "$missing"; then
        fail "highlight reported success for a selector that matches nothing; it is not querying the DOM. Content: $(truncate "$(extract_content_text "$missing")")"
        return
    fi

    pass "highlight resolved '#sf-btn' and rejected a selector that matches nothing"
}
run_test_15_3

# ── 15.4 — reported pilot state agrees with observed behaviour ──
begin_test "15.4" "Daemon-reported pilot state agrees with gated actions succeeding" \
    "Run a gated action, then read the pilot state the daemon reports" \
    "Contract: state and behaviour disagreeing means one of them is lying to the operator"

run_test_15_4() {
    goto_fixture || return

    local response
    response="$(call_tool "interact" '{"what":"execute_js","script":"return 1"}')"
    if check_is_error "$response"; then
        fail "Gated action failed while pilot was expected to be enabled: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    local state
    state="$(capture_state_field pilot_state)"
    case "$state" in
        enabled | assumed_enabled) ;;
        *)
            fail "Gated actions are succeeding but the daemon reports pilot_state='$state'"
            return
            ;;
    esac

    pass "Gated actions succeed and the daemon reports a consistent pilot_state=$state"
}
run_test_15_4

finish_category
