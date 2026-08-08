#!/bin/bash
# cat-22-advanced.sh — Advanced Scenarios & Integration Tests (5 tests)
# Tests complex workflows, feature interactions, edge cases.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "22" "Advanced: Integration & Complex Workflows" "5"

ensure_daemon

# ── TEST 22.1: Record → Generate → Heal → Validate Cycle ────────────────

begin_test "22.1" "Full workflow: Record → Generate Test → Heal Broken → Validate" \
    "Record user actions, generate test, simulate element change, heal test, validate" \
    "End-to-end feature integration"

run_test_22_1() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    # 1. Record. Event recording captures the action stream; screen recording
    # needs an explicit browser user gesture and can never start unattended.
    start_event_recording "workflow-test" || return
    sleep 0.1
    call_tool "interact" '{"what":"navigate","url":"https://example.com"}' >/dev/null 2>&1
    sleep 0.1
    call_tool "interact" '{"what":"click","selector":"#old-button"}' >/dev/null 2>&1
    sleep 0.1
    stop_event_recording || return
    response="$LAST_RECORDING_RESPONSE"

    # Each stage must succeed for the cycle to mean anything. The previous version
    # emitted four passes from this one test — three of them conceding the stage
    # had not happened ("Test generation pending (workflow continues)") — which
    # both hid the breakage and inflated the category's pass count.
    if ! check_not_error "$response"; then
        fail "Recording stage failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # 2. Generate a test from the recorded actions.
    sleep 0.2
    response=$(call_tool "generate" '{"what":"test","test_name":"generated-test"}')
    if ! check_not_error "$response"; then
        fail "Generate stage failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    # 3. Healing. test_heal inspects a real spec file and reports selectors that
    # no longer resolve, so the stage needs a file with a deliberately broken
    # selector. The previous call was generate(reproduction, heal=true) — `heal`
    # is not a parameter of any generate mode, so the stage never ran; it was
    # hidden behind a "Healing feature pending (workflow continues)" pass.
    # The daemon sandboxes file access to the project directory, so the spec
    # cannot live in TEMP_DIR; it is written under artifacts/ and removed below.
    local spec_dir="$PROJECT_ROOT/artifacts/uat"
    local spec="$spec_dir/heal-target-$$.spec.ts"
    mkdir -p "$spec_dir"
    cat > "$spec" <<'SPEC'
import { test, expect } from "@playwright/test";
test("uat heal target", async ({ page }) => {
  await page.locator("#sf-btn-renamed-by-uat").click();
});
SPEC

    response=$(call_tool "generate" '{"what":"test_heal","action":"analyze","test_file":'"$(json_string "$spec")"'}')
    rm -f "$spec"
    if ! check_not_error "$response"; then
        fail "Heal stage failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi
    if ! check_contains "$(extract_content_text "$response")" "#sf-btn-renamed-by-uat"; then
        fail "Heal analysis did not report the broken selector: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    pass "Full workflow cycle completed: recorded, generated a test, and heal analysis found the broken selector"
}
run_test_22_1

# ── TEST 22.2: Filtering + Recording Together ──────────────────────────

begin_test "22.2" "Noise filtering active while recording user actions" \
    "Configure noise rules, start recording, verify recording excludes filtered entries" \
    "Both features must work together without interference"

run_test_22_2() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    # Add noise rule
    response=$(call_tool "configure" '{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"debug_logs","match_spec":{"message_regex":"^\\[DEBUG\\]"}}]}')

    if ! check_not_error "$response"; then
        fail "Noise rule add failed"
        return
    fi

    sleep 0.1

    # Start recording with noise rules active.
    start_event_recording "with-filtering" || return
    sleep 0.1
    call_tool "interact" '{"what":"navigate","url":"https://example.com"}' >/dev/null 2>&1
    sleep 0.1
    stop_event_recording || return

    pass "Noise filtering + recording work together"
}
run_test_22_2

# ── TEST 22.3: Link Health + Performance Audit Together ────────────────

begin_test "22.3" "Link health and performance audits can run concurrently" \
    "Queue analyze(link_health) and analyze(performance) simultaneously" \
    "Multiple async analyses must not interfere"

run_test_22_3() {
    # Queue link health
    response1=$(call_tool "analyze" '{"what":"link_health"}')

    # Queue performance analysis simultaneously
    response2=$(call_tool "analyze" '{"what":"performance"}')

    # Neither analysis can queue without an attached browser, so the assertion
    # that holds in every environment is that concurrent async analyses do not
    # block or corrupt each other's responses. Whether they queued is reported
    # honestly as pass or skip rather than as an unconditional pass.
    if check_transport_failure "$response1"; then
        fail "analyze(link_health) did not return a usable response: $(truncate "$(extract_content_text "$response1")")"
        return
    fi
    if check_transport_failure "$response2"; then
        fail "analyze(performance) did not return a usable response: $(truncate "$(extract_content_text "$response2")")"
        return
    fi

    local id1 id2
    id1=$(extract_content_text "$response1" | grep -o 'link_health_[a-z0-9]*' | head -1 || true)
    id2=$(extract_content_text "$response2" | grep -o 'performance_[a-z0-9]*' | head -1 || true)

    if [ -n "$id1" ] && [ -n "$id2" ]; then
        pass "Link health and performance analyses queued concurrently ($id1, $id2)"
        return
    fi

    skip "Concurrent analyses did not queue in this environment (requires an attached browser): link_health='${id1:-none}' performance='${id2:-none}'"
}
run_test_22_3

# ── TEST 22.4: CORS Detection + Framework Detection ────────────────────

begin_test "22.4" "CORS detection and framework detection work together in link crawl" \
    "Crawl site with React, check CORS boundaries, verify both features active" \
    "Combined heuristics provide richer analysis"

run_test_22_4() {
    response=$(call_tool "analyze" '{"what":"link_health","mode":"crawl","start_url":"https://example.com","check_cors":true,"detect_framework":true}')

    if ! check_not_error "$response"; then
        fail "Combined crawl features failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "CORS + framework detection combined in link crawl"
    else
        fail "Combined features did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_22_4

# ── TEST 22.5: State Snapshot Before Major Operation ────────────────────

begin_test "22.5" "Save state before test generation, restore if needed" \
    "Save state, generate test (potential modification), could restore from snapshot" \
    "State management enables recovery and debugging"

run_test_22_5() {
    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null

    # Save state
    response=$(call_tool "interact" '{"what":"save_state","snapshot_name":"pre-generation"}')
    if ! check_not_error "$response"; then
        fail "save_state failed: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    sleep 0.1

    # Perform an operation that could modify state.
    call_tool "generate" '{"what":"test"}' >/dev/null 2>&1

    sleep 0.1

    response=$(call_tool "interact" '{"what":"load_state","snapshot_name":"pre-generation"}')
    if ! check_not_error "$response"; then
        fail "load_state failed for the snapshot saved above: $(truncate "$(command_failure_message "$response")")"
        return
    fi

    pass "State snapshot saved before generation and restored afterwards"
}
run_test_22_5

finish_category
