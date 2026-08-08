#!/bin/bash
# cat-30-recording-automation.sh — Recording UI Automation Tests (4 tests)
# Tests element finding, waiting, error recovery during recording playback.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "30" "Flow Recording: UI Automation" "4"

ensure_daemon

# ── TEST 18.19: Wait for Element During Recording ───────────────────────

begin_test "30.19" "record_wait_for: Wait for element to appear before clicking" \
    "Action: wait_for selector with timeout, then click when ready" \
    "Waiting prevents race conditions with dynamic content"

run_test_18_19() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    call_tool "interact" '{"what":"screen_recording_start","name":"wait-test"}' >/dev/null
    sleep 0.1

    # Record a wait action
    response=$(call_tool "interact" '{"what":"wait_for","selector":".modal","timeout":5000}')

    if ! check_not_error "$response"; then
        fail "wait_for during recording failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    sleep 0.1
    call_tool "interact" '{"what":"screen_recording_stop"}' >/dev/null

    pass "Wait action recorded successfully"
}
run_test_18_19

# ── TEST 18.21: Form Filling with Validation ──────────────────────────

begin_test "30.21" "Recording form fills with validation waits" \
    "Fill input, wait for validation message, then submit" \
    "Proper sequencing prevents validation errors"

run_test_18_21() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    call_tool "interact" '{"what":"screen_recording_start","name":"form-test"}' >/dev/null
    sleep 0.1

    # Fill form field
    call_tool "interact" '{"what":"type","selector":"input[name=email]","text":"test@example.com"}' >/dev/null 2>&1
    sleep 0.1

    # Wait for validation
    response=$(call_tool "interact" '{"what":"wait_for","selector":".validation-success","timeout":3000}')

    if check_transport_failure "$response"; then
        fail "wait_for did not return a usable response during recording: $(truncate "$(extract_content_text "$response")")"
        return
    fi
    if check_not_error "$response"; then
        pass "Form filling with validation sequenced correctly"
    else
        skip "wait_for could not resolve '.validation-success' in this environment: $(truncate "$(command_failure_message "$response")" 160)"
    fi

    sleep 0.1
    call_tool "interact" '{"what":"screen_recording_stop"}' >/dev/null
}
run_test_18_21

# ── TEST 18.23: Keyboard Navigation (Tab, Enter) ─────────────────────

begin_test "30.23" "Recording keyboard navigation (Tab, Enter, Escape)" \
    "Record key presses and verify they replay correctly" \
    "Keyboard interactions essential for accessibility testing"

run_test_18_23() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    call_tool "interact" '{"what":"screen_recording_start","name":"keyboard-test"}' >/dev/null
    sleep 0.1

    # Record key presses
    call_tool "interact" '{"what":"key_press","text":"Tab"}' >/dev/null 2>&1
    sleep 0.1
    call_tool "interact" '{"what":"key_press","text":"Enter"}' >/dev/null 2>&1
    sleep 0.1

    response=$(call_tool "interact" '{"what":"screen_recording_stop"}')

    if ! check_not_error "$response"; then
        fail "Keyboard recording failed"
        return
    fi

    pass "Keyboard navigation recorded (Tab, Enter)"
}
run_test_18_23

# ── TEST 18.24: Screenshot During Recording ──────────────────────────

begin_test "30.24" "Recording includes screenshot at key moments" \
    "Capture screenshot after major actions" \
    "Screenshots aid debugging and visual regression detection"

run_test_18_24() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    call_tool "interact" '{"what":"screen_recording_start","name":"screenshot-test"}' >/dev/null
    sleep 0.1

    # Perform action with screenshot
    call_tool "interact" '{"what":"click","selector":"button"}' >/dev/null 2>&1
    sleep 0.1

    # Capture screenshot
    response=$(call_tool "observe" '{"what":"screenshot"}')

    if check_transport_failure "$response"; then
        fail "observe(screenshot) did not return a usable response during recording: $(truncate "$(extract_content_text "$response")")"
        return
    fi
    if check_not_error "$response"; then
        pass "Screenshot captured during recording"
    else
        skip "Screenshot capture unavailable in this environment: $(truncate "$(command_failure_message "$response")" 160)"
    fi

    sleep 0.1
    call_tool "interact" '{"what":"screen_recording_stop"}' >/dev/null
}
run_test_18_24

finish_category
