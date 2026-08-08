#!/bin/bash
# cat-30-recording-automation.sh — Recording UI Automation Tests (4 tests)
#
# Runs connected, against the daemon-served interaction fixture.
#
# These tests used to wrap every action in interact/screen_recording_start.
# Screen recording captures video through getDisplayMedia and cannot begin
# without an explicit browser user gesture — the extension answers "awaiting
# user gesture" until someone clicks Approve in the popup — so an unattended run
# could never get past the first step. (cat-33 skips those two modes for exactly
# this reason.) Recording *user actions* is what these tests actually need, and
# that is event recording, which requires no gesture.
#
# They also acted on elements the fixture does not define (`.modal`), so the
# waits could only ever time out. Each test now drives real fixture behaviour.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "30" "Flow Recording: UI Automation" "4"

if ! start_daemon; then
    fail "Failed to start daemon for recording automation tests."
    finish_category
fi

# Puts the tracked tab on the fixture so each test starts from a known DOM.
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

# ── TEST 30.19: Wait for an element that appears later ───────────────────

begin_test "30.19" "record_wait_for: Wait for element to appear before clicking" \
    "Schedule the fixture's delayed element, wait for it, then read it back" \
    "Waiting prevents race conditions with dynamic content"

run_test_30_19() {
    goto_fixture || return
    start_event_recording "wait-test" || return

    # The fixture creates #delayed-el 800ms after scheduleDelayedEl runs, so the
    # wait has something real to resolve rather than a selector that never exists.
    call_tool "interact" '{"what":"execute_js","script":"scheduleDelayedEl(800); return \"scheduled\""}' >/dev/null 2>&1

    local response
    response=$(call_tool "interact" '{"what":"wait_for","selector":"#delayed-el","timeout":5000}')
    if ! check_not_error "$response"; then
        fail "wait_for did not resolve the delayed element: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    local text
    text=$(call_tool "interact" '{"what":"get_text","selector":"#delayed-el"}')
    if ! check_contains "$(extract_content_text "$text")" "I appeared"; then
        fail "Delayed element resolved but its text was not read back: $(truncate "$(extract_content_text "$text")")"
        return
    fi

    stop_event_recording || return
    pass "wait_for resolved the delayed element and its text was read back"
}
run_test_30_19

# ── TEST 30.21: Form Filling with a Result Wait ──────────────────────────

begin_test "30.21" "Recording form fills with validation waits" \
    "Fill the fixture form, submit it, and read the result element back" \
    "Proper sequencing prevents validation errors"

run_test_30_21() {
    goto_fixture || return
    start_event_recording "form-test" || return

    local marker="uat30_21_$$"
    call_tool "interact" '{"what":"type","selector":"#sf-name","text":"'"$marker"'"}' >/dev/null 2>&1
    call_tool "interact" '{"what":"type","selector":"#sf-email","text":"uat@example.com"}' >/dev/null 2>&1

    local response
    response=$(call_tool "interact" '{"what":"click","selector":"#sf-btn"}')
    if ! check_not_error "$response"; then
        fail "Submitting the fixture form failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # The fixture writes the submitted values into #sf-result, so reading the
    # marker back proves the fill and the submit both landed, in order.
    local text
    text=$(call_tool "interact" '{"what":"get_text","selector":"#sf-result"}')
    if ! check_contains "$(extract_content_text "$text")" "$marker"; then
        fail "Form result does not contain the typed value '$marker': $(truncate "$(extract_content_text "$text")")"
        return
    fi

    stop_event_recording || return
    pass "Form fill and submit sequenced correctly; the result reflected the typed value"
}
run_test_30_21

# ── TEST 30.23: Keyboard Navigation ─────────────────────

begin_test "30.23" "Recording keyboard navigation (Tab, Enter, Escape)" \
    "Record key presses during an event recording and verify the recording closes cleanly" \
    "Keyboard interactions essential for accessibility testing"

run_test_30_23() {
    goto_fixture || return
    start_event_recording "keyboard-test" || return

    local key response
    for key in Tab Enter Escape; do
        response=$(call_tool "interact" '{"what":"key_press","text":"'"$key"'"}')
        if ! check_not_error "$response"; then
            fail "key_press '$key' failed: $(truncate "$(command_failure_message "$response")")"
            return
        fi
    done

    stop_event_recording || return
    pass "Recorded Tab, Enter and Escape key presses and closed the recording"
}
run_test_30_23

# ── TEST 30.24: Screenshot During Recording ──────────────────────────

begin_test "30.24" "Recording includes screenshot at key moments" \
    "Capture a screenshot while an event recording is active" \
    "Screenshots aid debugging and visual regression detection"

run_test_30_24() {
    goto_fixture || return
    start_event_recording "screenshot-test" || return

    call_tool "interact" '{"what":"click","selector":"#sf-btn"}' >/dev/null 2>&1

    local response
    response=$(call_tool "observe" '{"what":"screenshot"}')
    if ! check_not_error "$response"; then
        fail "observe(screenshot) failed during recording: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # A success envelope with no image would leave nothing to debug with.
    local text
    text=$(extract_content_text "$response")
    if ! check_matches "$text" "image|screenshot|png|data:"; then
        fail "Screenshot response carries no image payload: $(truncate "$text")"
        return
    fi

    stop_event_recording || return
    pass "Captured a screenshot while an event recording was active"
}
run_test_30_24

finish_category
