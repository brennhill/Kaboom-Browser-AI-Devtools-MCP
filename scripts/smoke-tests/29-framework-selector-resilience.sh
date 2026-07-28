#!/bin/bash
# 29-framework-selector-resilience.sh — real-framework selector resilience smoke coverage.
# Runs React/Vue/Svelte/Next fixtures and proves resilience on hard automation cases:
# hydration delay, overlay blocking, route remount churn, stale element handles,
# async content, and lazy/virtualized list expansion.
set -eo pipefail

begin_category "29" "Framework Selector Resilience" "4"

FRAMEWORK_FIXTURE_BUILD_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/build-framework-fixtures.mjs"
FRAMEWORK_FIXTURES_READY=false
FRAMEWORK_RESILIENCE_FULL_REPEATS="${FRAMEWORK_RESILIENCE_FULL_REPEATS:-1}"
FRAMEWORK_SELECTOR_REFRESH_CYCLES="${FRAMEWORK_SELECTOR_REFRESH_CYCLES:-3}"

normalize_positive_int() {
    local raw="$1"
    local fallback="$2"
    if [[ "$raw" =~ ^[0-9]+$ ]] && [ "$raw" -ge 1 ]; then
        echo "$raw"
        return 0
    fi
    echo "$fallback"
}

FRAMEWORK_RESILIENCE_FULL_REPEATS="$(normalize_positive_int "$FRAMEWORK_RESILIENCE_FULL_REPEATS" "1")"
FRAMEWORK_SELECTOR_REFRESH_CYCLES="$(normalize_positive_int "$FRAMEWORK_SELECTOR_REFRESH_CYCLES" "3")"

ensure_framework_fixtures() {
    if [ "$FRAMEWORK_FIXTURES_READY" = "true" ]; then
        return 0
    fi

    if [ ! -f "$FRAMEWORK_FIXTURE_BUILD_SCRIPT" ]; then
        fail "Framework fixture build script missing: $FRAMEWORK_FIXTURE_BUILD_SCRIPT"
        return 1
    fi

    if ! node "$FRAMEWORK_FIXTURE_BUILD_SCRIPT" >> "$DIAGNOSTICS_FILE" 2>&1; then
        fail "Framework fixture build failed. See diagnostics for details."
        return 1
    fi

    FRAMEWORK_FIXTURES_READY=true
    return 0
}

interact_failed() {
    local raw="$1"

    if echo "$raw" | grep -qi '^timeout waiting for '; then
        return 0
    fi

    local payload
    payload="$(extract_embedded_json "$raw" 2>/dev/null || true)"
    if [ -z "$payload" ]; then
        if echo "$raw" | grep -qiE '"status":"(failed|error|timeout)"|"lifecycle_status":"(failed|error|timeout)"|"isError":true'; then
            return 0
        fi
        return 1
    fi

    local verdict
    verdict="$(
        printf '%s' "$payload" | jq -r '
            def norm(v): (v // "" | tostring | ascii_downcase);

            (norm(.status)) as $status |
            (norm(.lifecycle_status)) as $lifecycle |
            (.isError == true or .result.isError == true) as $is_error |
            (.result.success == false) as $success_false |
            ((.error // .error_code // .failure_cause // "") | tostring | length > 0) as $has_error_signal |

            if ($status == "failed" or $status == "error" or $status == "timeout" or
                $lifecycle == "failed" or $lifecycle == "error" or $lifecycle == "timeout" or
                $is_error or $success_false or
                ($has_error_signal and $status != "complete"))
            then "fail"
            else "ok"
            end
        ' 2>/dev/null || echo "ok"
    )"

    [ "$verdict" = "fail" ]
}

is_transient_transport_issue() {
    local raw="$1"
    echo "$raw" | grep -qiE "transport_no_response|Server connection error|deadline exceeded|connection refused|EOF|starting up|Extension is not connected|no_data"
}

extract_first_submit_selector() {
    local text="$1"
    local payload
    payload="$(extract_embedded_json "$text" 2>/dev/null || true)"
    if [ -z "$payload" ]; then
        return 1
    fi

    printf '%s' "$payload" | jq -r '
        (
            (.result.elements // [])
            | map(select((.label // "") == "Submit Profile"))
            | .[0].selector
        ) // empty
    ' 2>/dev/null | head -1
}

extract_first_submit_element_id() {
    local text="$1"
    local payload
    payload="$(extract_embedded_json "$text" 2>/dev/null || true)"
    if [ -z "$payload" ]; then
        return 1
    fi

    printf '%s' "$payload" | jq -r '
        (
            (.result.elements // [])
            | map(select((.label // "") == "Submit Profile"))
            | .[0].element_id
        ) // empty
    ' 2>/dev/null | head -1
}

extract_token_text() {
    local text="$1"
    echo "$text" | tr '\n' ' ' | grep -oE 'token:[a-z0-9]+' | head -1 | cut -d: -f2
}

framework_url() {
    local framework_key="$1"
    case "$framework_key" in
        next)
            echo "${SMOKE_BASE_URL}/frameworks/next/"
            ;;
        *)
            echo "${SMOKE_BASE_URL}/frameworks/${framework_key}.html"
            ;;
    esac
}

json_quote() {
    local value="$1"
    printf '%s' "$value" | jq -Rs .
}

wait_for_text_contains() {
    local selector="$1"
    local expected_text="$2"
    local max_polls="${3:-20}"
    local poll_sleep="${4:-0.5}"
    local context="${5:-state check}"

    for _ in $(seq 1 "$max_polls"); do
        interact_and_wait "get_text" "{\"action\":\"get_text\",\"selector\":\"${selector}\",\"reason\":\"${context}\"}" 12
        if ! interact_failed "$INTERACT_RESULT" && echo "$INTERACT_RESULT" | grep -q "$expected_text"; then
            return 0
        fi
        sleep "$poll_sleep"
    done
    return 1
}

ensure_hydrated_ready() {
    local framework_key="$1"
    local attempt
    for attempt in 1 2 3; do
        interact_and_wait "wait_for" '{"what":"wait_for","selector":"#hydrated-ready","timeout_ms":8000,"reason":"Wait for hydration to complete"}' 30
        if ! interact_failed "$INTERACT_RESULT"; then
            return 0
        fi

        if ! is_transient_transport_issue "$INTERACT_RESULT"; then
            fail "Hydration wait failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
            return 1
        fi

        if ! wait_for_extension 8; then
            fail "Extension disconnected during hydration check for ${framework_key}."
            return 1
        fi
        sleep 0.5
    done

    fail "Hydration wait failed for ${framework_key} after retries. Result: $(truncate "$INTERACT_RESULT" 240)"
    return 1
}

dismiss_overlay_if_present() {
    local framework_key="$1"
    interact_and_wait "click" '{"what":"click","selector":"text=Accept Cookies","reason":"Dismiss consent overlay if present"}' 12
    if interact_failed "$INTERACT_RESULT"; then
        if echo "$INTERACT_RESULT" | grep -qiE 'element_not_found|ambiguous_target|No matching element'; then
            return 0
        fi
        fail "Consent overlay dismissal failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi
    return 0
}

collect_submit_targets() {
    interact_and_wait "list_interactive" '{"what":"list_interactive","role":"button","text_contains":"Submit Profile","visible_only":true,"reason":"Find submit button by label"}'
    if interact_failed "$INTERACT_RESULT"; then
        return 1
    fi
    return 0
}

exercise_stale_handle_recovery() {
    local framework_key="$1"

    if ! collect_submit_targets; then
        fail "list_interactive failed before stale-handle check for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    local stale_element_id
    stale_element_id="$(extract_first_submit_element_id "$INTERACT_RESULT")"
    if [ -z "$stale_element_id" ]; then
        fail "Missing submit element_id before stale-handle check for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Switch route to settings via helper","script":"(function(){ if (typeof window.__SMOKE_SHOW_SETTINGS__ === \"function\") { window.__SMOKE_SHOW_SETTINGS__(); return \"helper\"; } var b = Array.from(document.querySelectorAll(\"button\")).find(function(x){ return /Settings Tab/.test((x.textContent||\"\")); }); if (b) { b.click(); return \"dom-click\"; } return \"missing\"; })()"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Settings route switch failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Switch route to profile via helper","script":"(function(){ if (typeof window.__SMOKE_SHOW_PROFILE__ === \"function\") { window.__SMOKE_SHOW_PROFILE__(); return \"helper\"; } var b = Array.from(document.querySelectorAll(\"button\")).find(function(x){ return /Profile Tab/.test((x.textContent||\"\")); }); if (b) { b.click(); return \"dom-click\"; } return \"missing\"; })()"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Profile route switch failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    ensure_hydrated_ready "$framework_key" || return 1
    dismiss_overlay_if_present "$framework_key" || return 1

    interact_and_wait "click" "{\"action\":\"click\",\"element_id\":\"${stale_element_id}\",\"reason\":\"Use stale element_id after remount\"}"
    if ! interact_failed "$INTERACT_RESULT"; then
        return 0
    fi

    if ! echo "$INTERACT_RESULT" | grep -qi "stale_element_id"; then
        fail "Stale handle click failed with unexpected error for ${framework_key}. element_id=${stale_element_id}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    if ! collect_submit_targets; then
        fail "Recovery list_interactive failed after stale handle for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    local recovery_selector
    recovery_selector="$(extract_first_submit_selector "$INTERACT_RESULT")"
    if [ -z "$recovery_selector" ]; then
        fail "Recovery selector missing after stale handle for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    local recovery_selector_json
    recovery_selector_json="$(json_quote "$recovery_selector")"
    interact_and_wait "click" "{\"action\":\"click\",\"selector\":${recovery_selector_json},\"reason\":\"Recover after stale element_id\"}"
    if interact_failed "$INTERACT_RESULT"; then
        fail "Recovery click failed after stale handle for ${framework_key}. selector='${recovery_selector}'. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi
    return 0
}

exercise_async_content_flow() {
    local framework_key="$1"

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Trigger delayed async panel via framework helper","script":"(function(){ if (typeof window.__SMOKE_LOAD_ASYNC__ === \"function\") { window.__SMOKE_LOAD_ASYNC__(); return \"helper\"; } var btn = Array.from(document.querySelectorAll(\"button\")).find(function(b){ return /Load Async Panel/.test((b.textContent||\"\")); }); if (btn) { btn.click(); return \"dom-click\"; } return \"missing\"; })()"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Async panel trigger failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    if ! wait_for_text_contains "#async-result" "ready" 20 0.5 "Wait for async ready state"; then
        fail "Async ready state did not appear for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "list_interactive" '{"what":"list_interactive","role":"button","text_contains":"Async Save","visible_only":true,"reason":"Find delayed async button"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Delayed async button discovery failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi
    if ! echo "$INTERACT_RESULT" | grep -q "Async Save"; then
        fail "Delayed async button missing from list_interactive for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    dismiss_overlay_if_present "$framework_key" || return 1

    interact_and_wait "click" '{"what":"click","selector":"#async-panel button","auto_dismiss":true,"wait_for_stable":true,"reason":"Click async action"}'
    if interact_failed "$INTERACT_RESULT" && echo "$INTERACT_RESULT" | grep -qi "blocked_by_overlay"; then
        dismiss_overlay_if_present "$framework_key" || return 1
        interact_and_wait "execute_js" '{"what":"execute_js","reason":"Force-dismiss consent overlay if still present","script":"(function(){ var btn=document.querySelector(\"#consent-modal button\"); if(btn){ btn.click(); return \"dismissed\"; } return \"not_present\"; })()"}'
        interact_and_wait "click" '{"what":"click","selector":"#async-panel button","auto_dismiss":true,"wait_for_stable":true,"reason":"Retry async action after overlay dismissal"}'
    fi
    if interact_failed "$INTERACT_RESULT"; then
        fail "Async Save click failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    if ! wait_for_text_contains "#async-result" "async:clicked" 10 0.3 "Wait for async click acknowledgement"; then
        fail "Async Save click did not update async-result for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    return 0
}

exercise_virtualized_content_flow() {
    local framework_key="$1"

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Expand virtual list via helper + scroll","script":"(function(){ if (typeof window.__SMOKE_EXPAND_VIRTUAL__ === \"function\") { window.__SMOKE_EXPAND_VIRTUAL__(); } var el=document.getElementById(\"virtual-list\"); if(!el) return \"missing\"; el.scrollTop=el.scrollHeight; return \"expanded\"; })()"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Virtual list scroll script failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "wait_for" '{"what":"wait_for","selector":"#deep-target","timeout_ms":6000,"reason":"Wait for late target in virtualized list"}' 25
    if interact_failed "$INTERACT_RESULT"; then
        fail "Deep virtual target did not appear for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "click" '{"what":"click","selector":"#deep-target","reason":"Click deep virtualized target"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Deep virtual target click failed for ${framework_key}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    return 0
}

exercise_submit_roundtrip() {
    local framework_key="$1"
    local iteration="$2"
    local run_value="${framework_key}-run-${iteration}"

    interact_and_wait "type" "{\"action\":\"type\",\"selector\":\"placeholder=Enter name\",\"text\":\"${run_value}\",\"clear\":true,\"reason\":\"Semantic type: ${framework_key} iteration ${iteration}\"}"
    if interact_failed "$INTERACT_RESULT"; then
        fail "Semantic type failed for ${framework_key} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    interact_and_wait "click" '{"what":"click","selector":"text=Submit Profile","reason":"Semantic click submit button"}'
    if interact_failed "$INTERACT_RESULT"; then
        fail "Semantic click failed for ${framework_key} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    if ! collect_submit_targets; then
        fail "list_interactive failed for ${framework_key} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    local roundtrip_selector
    roundtrip_selector="$(extract_first_submit_selector "$INTERACT_RESULT")"
    if [ -z "$roundtrip_selector" ]; then
        fail "list_interactive returned no selector for Submit Profile on ${framework_key} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    local selector_json
    selector_json="$(json_quote "$roundtrip_selector")"
    interact_and_wait "click" "{\"action\":\"click\",\"selector\":${selector_json},\"reason\":\"Roundtrip selector click\"}"
    if interact_failed "$INTERACT_RESULT"; then
        fail "Roundtrip selector click failed for ${framework_key} iteration ${iteration}. selector='${roundtrip_selector}'. Result: $(truncate "$INTERACT_RESULT" 240)"
        return 1
    fi

    return 0
}

run_framework_resilience_test() {
    local framework_key="$1"
    local expected_framework_name="$2"
    local full_repeat
    local page_url
    page_url="$(framework_url "$framework_key")"

    for full_repeat in $(seq 1 "$FRAMEWORK_RESILIENCE_FULL_REPEATS"); do
        if ! wait_for_extension 8; then
            fail "Extension disconnected before ${framework_key} run ${full_repeat}."
            return
        fi

        local previous_token=""
        local nav_attempt
        for nav_attempt in 1 2 3; do
            interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"${page_url}\",\"reason\":\"Framework smoke: open ${framework_key} fixture (run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS})\"}" 30
            if ! interact_failed "$INTERACT_RESULT"; then
                break
            fi
            if ! is_transient_transport_issue "$INTERACT_RESULT"; then
                fail "Navigate to ${framework_key} fixture failed on run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS}. Result: $(truncate "$INTERACT_RESULT" 240)"
                return
            fi
            if ! wait_for_extension 8; then
                fail "Extension disconnected while navigating ${framework_key} run ${full_repeat}."
                return
            fi
            sleep 0.5
        done
        if interact_failed "$INTERACT_RESULT"; then
            fail "Navigate to ${framework_key} fixture failed on run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS} after retries. Result: $(truncate "$INTERACT_RESULT" 240)"
            return
        fi

        local analyze_response analyze_text
        local analyze_ok=false
        local analyze_attempt
        for analyze_attempt in 1 2 3; do
            analyze_response=$(call_tool "analyze" '{"what":"page_structure"}')
            analyze_text=$(extract_content_text "$analyze_response")
            if [ -z "$analyze_text" ] && [ -n "$analyze_response" ]; then
                analyze_text="$analyze_response"
            fi

            if [ -n "$analyze_text" ] && echo "$analyze_text" | grep -q "\"name\":\"${expected_framework_name}\""; then
                analyze_ok=true
                break
            fi

            if is_transient_transport_issue "$analyze_text"; then
                if ! wait_for_extension 8; then
                    fail "Extension disconnected during framework detection for ${framework_key}."
                    return
                fi
                sleep 0.5
                continue
            fi

            if [ -z "$analyze_text" ]; then
                fail "analyze(page_structure) returned empty content for ${framework_key} on run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS}."
            else
                fail "Framework detection mismatch for ${framework_key} on run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS}. Expected '${expected_framework_name}'. Result: $(truncate "$analyze_text" 240)"
            fi
            return
        done
        if [ "$analyze_ok" != "true" ]; then
            fail "Framework detection for ${framework_key} did not stabilize after retries. Last result: $(truncate "$analyze_text" 240)"
            return
        fi

        ensure_hydrated_ready "$framework_key" || return
        dismiss_overlay_if_present "$framework_key" || return
        exercise_async_content_flow "$framework_key" || return
        exercise_virtualized_content_flow "$framework_key" || return
        exercise_stale_handle_recovery "$framework_key" || return

        local iteration
        for iteration in $(seq 1 "$FRAMEWORK_SELECTOR_REFRESH_CYCLES"); do
            if [ "$iteration" -gt 1 ]; then
                interact_and_wait "refresh" "{\"action\":\"refresh\",\"reason\":\"Framework smoke: refresh ${framework_key} run ${full_repeat} iteration ${iteration}\"}" 30
                if interact_failed "$INTERACT_RESULT"; then
                    fail "Refresh failed for ${framework_key} run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
                    return
                fi
            fi

            ensure_hydrated_ready "$framework_key" || return
            dismiss_overlay_if_present "$framework_key" || return

            interact_and_wait "get_text" '{"what":"get_text","selector":"#selector-token","reason":"Read selector churn token"}'
            if interact_failed "$INTERACT_RESULT"; then
                fail "Token read failed for ${framework_key} run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
                return
            fi

            local token
            token="$(extract_token_text "$INTERACT_RESULT")"
            if [ -z "$token" ]; then
                fail "Missing selector token on ${framework_key} run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS} iteration ${iteration}. Result: $(truncate "$INTERACT_RESULT" 240)"
                return
            fi
            if [ "$iteration" -gt 1 ] && [ "$token" = "$previous_token" ]; then
                fail "Selector token did not change after refresh for ${framework_key} on run ${full_repeat}/${FRAMEWORK_RESILIENCE_FULL_REPEATS}. token=${token}"
                return
            fi
            previous_token="$token"

            exercise_submit_roundtrip "$framework_key" "run${full_repeat}-iter${iteration}" || return
        done
    done

    pass "${framework_key}: hydration/overlay/remount/async/virtualized + selector-resilience checks passed (${FRAMEWORK_RESILIENCE_FULL_REPEATS} full run(s), ${FRAMEWORK_SELECTOR_REFRESH_CYCLES} refresh cycle(s) each)."
}

# ── Test 29.1: React selector resilience ─────────────────────
begin_test "29.1" "[BROWSER] React fixture hard-case resilience" \
    "real React fixture validates hydration timing, overlay handling, remount churn, async content, virtualized targets, and selector resilience" \
    "Tests: framework detection + hard-case automation reliability"

run_test_29_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    ensure_framework_fixtures || return
    run_framework_resilience_test "react" "React"
}
run_test_29_1

# ── Test 29.2: Vue selector resilience ───────────────────────
begin_test "29.2" "[BROWSER] Vue fixture hard-case resilience" \
    "real Vue fixture validates hydration timing, overlay handling, remount churn, async content, virtualized targets, and selector resilience" \
    "Tests: framework detection + hard-case automation reliability"

run_test_29_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    ensure_framework_fixtures || return
    run_framework_resilience_test "vue" "Vue"
}
run_test_29_2

# ── Test 29.3: Svelte selector resilience ────────────────────
begin_test "29.3" "[BROWSER] Svelte fixture hard-case resilience" \
    "real Svelte fixture validates hydration timing, overlay handling, remount churn, async content, virtualized targets, and selector resilience" \
    "Tests: framework detection + hard-case automation reliability"

run_test_29_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    ensure_framework_fixtures || return
    run_framework_resilience_test "svelte" "Svelte"
}
run_test_29_3

# ── Test 29.4: Next.js selector resilience ──────────────────
begin_test "29.4" "[BROWSER] Next fixture hard-case resilience" \
    "real Next fixture validates hydration timing, overlay handling, remount churn, async content, virtualized targets, and selector resilience" \
    "Tests: framework detection + hard-case automation reliability"

run_test_29_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    ensure_framework_fixtures || return
    run_framework_resilience_test "next" "Next.js"
}
run_test_29_4
