#!/bin/bash
# cat-35-qa-fixtures.sh — Connected QA fixture transaction and rollback verification.
# Docs: docs/features/feature/environment-manipulation/index.md
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7890}"
OUTPUT_FILE="${2:-/dev/null}"
init_framework "$PORT" "$OUTPUT_FILE"
begin_category "35" "QA Fixture Transactions" "4"

fixture_call() {
    call_tool "configure" '{"what":"qa_fixture","fixture_action":"apply","fixture":'"$1"'}'
}

execute_script() {
    call_tool "interact" '{"what":"execute_js","script":'"$(printf '%s' "$1" | jq -Rs .)"'}'
}

assert_success() {
    check_valid_jsonrpc "$1" && ! check_is_error "$1"
}

start_daemon || {
    fail "QA fixture UAT could not start with an attached browser"
    finish_category
}

fixture_url="http://127.0.0.1:${PORT}/tests/interact.html"
navigate_response="$(call_tool "interact" '{"what":"navigate_and_wait_for","url":"'"$fixture_url"'","wait_for":"#sf-btn"}')"
if ! assert_success "$navigate_response" || ! uat_wait_for_connected_browser "$PORT" "$WRAPPER"; then
    fail "QA fixture UAT could not prepare its disposable target"
    finish_category
fi

begin_test "35.1" "Apply deterministic page state" \
    "Snapshot and apply local/session storage, flags, and seed data on the connected disposable tab" \
    "A real extension round trip proves the declared fixture reaches the tracked page"
run_test_35_1() {
    local response verify cleanup
    execute_script 'localStorage.setItem("kaboom_fixture","old"); sessionStorage.removeItem("kaboom_session"); "prepared"' >/dev/null
    response="$(fixture_call '{"version":1,"local_storage":{"kaboom_fixture":"new"},"session_storage":{"kaboom_session":"ready"},"feature_flags":{"kaboom_flag":true},"seed_data":{"kaboom_seed":{"value":7}}}')"
    verify="$(execute_script '({local:localStorage.getItem("kaboom_fixture"),session:sessionStorage.getItem("kaboom_session"),flag:localStorage.getItem("kaboom_flag"),seed:localStorage.getItem("kaboom_seed")})')"
    cleanup="$(execute_script 'localStorage.setItem("kaboom_fixture","old"); sessionStorage.removeItem("kaboom_session"); localStorage.removeItem("kaboom_flag"); localStorage.removeItem("kaboom_seed"); "restored"')"
    if ! assert_success "$response" || ! assert_success "$cleanup"; then
        fail "Fixture apply or cleanup failed: $(truncate "$(extract_content_text "$response")")"
    elif ! extract_content_text "$verify" | grep -q 'new' || ! extract_content_text "$verify" | grep -q 'ready'; then
        fail "Applied fixture state was not observable on the connected page"
    else
        pass "Connected fixture state applied and the synthetic page state was restored"
    fi
}
run_test_35_1

begin_test "35.2" "Snapshot failure is redacted and mutation-free" \
    "Inject a storage read failure before fixture mutation" \
    "The transaction must stop at snapshot_failed without exposing the synthetic secret"
run_test_35_2() {
    local response verify
    execute_script 'localStorage.setItem("kaboom_snapshot_fault","old"); "prepared"' >/dev/null
    response="$(fixture_call '{"version":1,"target":{"url":"https://example.com/"},"local_storage":{"kaboom_snapshot_fault":"private-fixture-secret"}}')"
    verify="$(execute_script 'localStorage.getItem("kaboom_snapshot_fault")')"
    execute_script 'localStorage.removeItem("kaboom_snapshot_fault"); "restored"' >/dev/null
    if ! extract_content_text "$response" | grep -q 'snapshot_failed'; then
        fail "Injected snapshot failure was not classified: $(truncate "$(extract_content_text "$response")")"
    elif printf '%s' "$response" | grep -q 'private-fixture-secret'; then
        fail "Snapshot failure leaked fixture data"
    elif ! extract_content_text "$verify" | grep -q 'old'; then
        fail "Snapshot failure mutated page state"
    else
        pass "Snapshot failure was stable, redacted, and mutation-free"
    fi
}
run_test_35_2

begin_test "35.3" "Partial apply rolls back exact state" \
    "Inject a write failure after one deterministic storage mutation" \
    "Rollback must restore every captured value and redact the driver cause"
run_test_35_3() {
    local response verify
    execute_script 'document.cookie="kaboom_partial=old; path=/"; "prepared"' >/dev/null
    response="$(fixture_call '{"version":1,"cookies":[{"name":"kaboom_partial","value":"new","path":"/"},{"name":"kaboom_fault","value":"private-fixture-secret","domain":"definitely.invalid","path":"/"}]}')"
    verify="$(execute_script 'document.cookie')"
    execute_script 'document.cookie="kaboom_partial=; Max-Age=0; path=/"; document.cookie="kaboom_fault=; Max-Age=0; path=/"; "restored"' >/dev/null
    if ! extract_content_text "$response" | grep -q 'apply_failed_rolled_back'; then
        fail "Partial failure did not report rollback: $(truncate "$(extract_content_text "$response")")"
    elif printf '%s' "$response" | grep -q 'private-fixture-secret'; then
        fail "Partial apply failure leaked fixture data"
    elif ! extract_content_text "$verify" | grep -q 'kaboom_partial=old' || extract_content_text "$verify" | grep -q 'kaboom_fault'; then
        fail "Rollback did not restore exact prior state"
    else
        pass "Partial apply rolled back exact state without leaking its cause"
    fi
}
run_test_35_3

begin_test "35.4" "Unsupported capability fails before mutation" \
    "Request locale emulation together with a storage sentinel" \
    "Preflight must reject the unsupported capability before touching storage"
run_test_35_4() {
    local response verify
    execute_script 'localStorage.setItem("kaboom_preflight","old"); "prepared"' >/dev/null
    response="$(fixture_call '{"version":1,"locale":"de-DE","local_storage":{"kaboom_preflight":"private-fixture-secret"}}')"
    verify="$(execute_script 'localStorage.getItem("kaboom_preflight")')"
    execute_script 'localStorage.removeItem("kaboom_preflight"); "restored"' >/dev/null
    if ! extract_content_text "$response" | grep -q 'snapshot_failed'; then
        fail "Unsupported fixture was not rejected before apply"
    elif printf '%s' "$response" | grep -q 'private-fixture-secret'; then
        fail "Unsupported fixture response leaked state"
    elif ! extract_content_text "$verify" | grep -q 'old'; then
        fail "Unsupported fixture mutated its sentinel"
    else
        pass "Unsupported capability failed before mutation with a redacted response"
    fi
}
run_test_35_4

finish_category
