#!/bin/bash
# cat-33-connected-action-coverage.sh — Invoke every live MCP mode against an attached test page.
# Docs: docs/features/feature/self-testing/index.md
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7890}"
OUTPUT_FILE="${2:-/dev/null}"
init_framework "$PORT" "$OUTPUT_FILE"

TOOLS="observe generate configure interact analyze"
ACTION_FILTER="${KABOOM_UAT_ACTION:-}"
begin_category "33" "Connected Action Coverage" "schema-derived"

# json_string and connected_fixture_url are canonical framework helpers.

# uat_connected_tracked_tab returns "id<TAB>url"; only the id belongs in JSON.
# Interpolating the raw value emits malformed JSON, which silently stops the
# action under test from being exercised at all.
tracked_tab_id() {
    printf '%s' "${HEALTH_TRACKED_TAB_ID:-0}" | cut -f1
}

# Chrome refuses chrome.debugger access to a target it considers another
# extension's page, and refuses to expose some targets to the debugger at all.
# That is a browser security boundary working as intended — an extension being
# able to attach a debugger to another extension's page would be the bug — so an
# action blocked by it is reported as skipped, not failed. Anything else that
# goes wrong with the same action still fails.
is_debugger_refusal() {
    printf '%s' "$1" |
        grep -qE 'chrome-extension:// URL of different extension|Cannot attach to this target|performance_trace_target_not_debuggable'
}

action_expectation() {
    case "$1/$2" in
        observe/command_result) echo "expected_error:no_data" ;;
        observe/recording_actions|observe/log_diff_report) echo "expected_error:internal_error" ;;
        observe/playback_results) echo "expected_error:no_data" ;;
        configure/playback|configure/log_diff) echo "expected_error:internal_error" ;;
        configure/setup_quality_gates) echo "destructive" ;;
        interact/screen_recording_start|interact/screen_recording_stop) echo "user_mediated" ;;
        interact/clipboard_read) echo "permission_gated" ;;
        analyze/annotation_detail|analyze/draw_session) echo "expected_error:no_data" ;;
        observe/errors|observe/logs|observe/extension_logs|observe/network_waterfall|observe/network_bodies|\
        observe/websocket_events|observe/websocket_status|observe/actions|observe/vitals|observe/page|\
        observe/tabs|observe/history|observe/pilot|observe/timeline|observe/error_bundles|observe/screenshot|\
        observe/storage|observe/indexeddb|observe/pending_commands|observe/failed_commands|observe/saved_videos|\
        observe/recordings|observe/summarized_logs|observe/page_inventory|observe/transients|observe/inbox|\
        observe/site_menus|\
        generate/reproduction|generate/test|generate/pr_summary|generate/har|generate/csp|generate/sri|\
        generate/sarif|generate/visual_test|generate/annotation_report|generate/annotation_issues|\
        generate/test_from_context|generate/test_heal|generate/test_classify|\
        configure/store|configure/load|configure/noise_rule|configure/clear|configure/health|configure/tutorial|\
        configure/streaming|configure/test_boundary_start|configure/test_boundary_end|\
        configure/event_recording_start|configure/event_recording_stop|configure/telemetry|\
        configure/describe_capabilities|configure/diff_sessions|configure/audit_log|configure/restart|\
        configure/save_sequence|configure/get_sequence|configure/list_sequences|configure/delete_sequence|\
        configure/replay_sequence|configure/doctor|configure/security_mode|configure/network_recording|\
        configure/action_jitter|configure/report_issue|configure/qa_fixture|\
        interact/highlight|interact/subtitle|interact/save_state|interact/load_state|interact/list_states|\
        interact/delete_state|interact/set_storage|interact/delete_storage|interact/clear_storage|\
        interact/set_cookie|interact/delete_cookie|interact/execute_js|interact/navigate|interact/refresh|\
        interact/back|interact/forward|interact/new_tab|interact/switch_tab|interact/close_tab|interact/click|\
        interact/type|interact/select|interact/check|interact/get_text|interact/get_value|\
        interact/get_attribute|interact/query|interact/set_attribute|interact/focus|interact/scroll_to|\
        interact/wait_for|interact/key_press|interact/paste|interact/open_composer|\
        interact/submit_active_composer|interact/confirm_top_dialog|interact/dismiss_top_overlay|\
        interact/hover|interact/auto_dismiss_overlays|interact/wait_for_stable|interact/list_interactive|\
        interact/get_readable|interact/get_markdown|interact/navigate_and_wait_for|\
        interact/navigate_and_document|interact/fill_form_and_submit|interact/fill_form|\
        interact/run_a11y_and_export_sarif|interact/upload|interact/draw_mode_start|interact/hardware_click|\
        interact/activate_tab|interact/explore_page|interact/batch|\
        interact/clipboard_write|\
        analyze/dom|analyze/performance|analyze/accessibility|analyze/error_clusters|\
        analyze/navigation_patterns|analyze/security_audit|analyze/third_party_audit|analyze/link_health|\
        analyze/link_validation|analyze/page_summary|analyze/annotations|analyze/api_validation|\
        analyze/draw_history|analyze/computed_styles|analyze/forms|analyze/form_state|\
        analyze/form_validation|analyze/data_table|analyze/visual_baseline|analyze/visual_diff|\
        analyze/visual_baselines|analyze/navigation|analyze/page_structure|analyze/audit|\
        analyze/feature_gates|analyze/page_issues|analyze/performance_trace|analyze/react_profile|\
        analyze/verification) echo "success" ;;
        *) echo "unclassified" ;;
    esac
}

action_args() {
    local tool="$1"
    local mode="$2"
    local base_url="http://127.0.0.1:${PORT}/tests/interact.html"
    case "$tool/$mode" in
        observe/storage) echo '{"what":"storage","storage_type":"local"}' ;;
        observe/indexeddb) echo '{"what":"indexeddb","database":"kaboom_uat","store":"items"}' ;;
        observe/command_result) echo '{"what":"command_result","correlation_id":"uat-missing-command"}' ;;
        observe/recording_actions) echo '{"what":"recording_actions","recording_id":"uat-missing-recording"}' ;;
        observe/playback_results) echo '{"what":"playback_results","recording_id":"uat-missing-recording"}' ;;
        observe/log_diff_report) echo '{"what":"log_diff_report","original_id":"uat-original","replay_id":"uat-replay"}' ;;
        generate/test) echo '{"what":"test","test_name":"connected-action-coverage"}' ;;
        generate/visual_test) echo '{"what":"visual_test","test_name":"connected-visual-coverage"}' ;;
        generate/annotation_report|generate/annotation_issues) echo '{"what":"'"$mode"'","annot_session":"uat-missing-annotations"}' ;;
        generate/test_from_context) echo '{"what":"test_from_context","context":"interaction","output_format":"inline"}' ;;
        generate/test_heal) echo '{"what":"test_heal","action":"batch","test_dir":"tests/extension"}' ;;
        generate/test_classify) echo '{"what":"test_classify","action":"failure","failure":{"test_name":"connected action coverage","error":"Expected button to be visible"}}' ;;
        configure/store) echo '{"what":"store","store_action":"save","key":"active_codebase","data":'"$(json_string "$PROJECT_ROOT")"'}' ;;
        configure/load) echo '{"what":"load","namespace":"connected-coverage"}' ;;
        configure/diff_sessions) echo '{"what":"diff_sessions","session_a":"uat-a","session_b":"uat-b"}' ;;
        configure/noise_rule) echo '{"what":"noise_rule","noise_action":"list"}' ;;
        configure/streaming) echo '{"what":"streaming","streaming_action":"status"}' ;;
        configure/test_boundary_start) echo '{"what":"test_boundary_start","test_id":"connected-action-coverage"}' ;;
        configure/test_boundary_end) echo '{"what":"test_boundary_end","test_id":"connected-action-coverage"}' ;;
        configure/event_recording_start) echo '{"what":"event_recording_start","name":"connected-action-coverage"}' ;;
        configure/event_recording_stop) echo '{"what":"event_recording_stop","recording_id":"'"${HEALTH_RECORDING_ID:-missing}"'"}' ;;
        configure/playback) echo '{"what":"playback","recording_id":"uat-missing-recording"}' ;;
        configure/log_diff) echo '{"what":"log_diff","original_id":"uat-original","replay_id":"uat-replay"}' ;;
        configure/telemetry) echo '{"what":"telemetry","mode":"off"}' ;;
        configure/describe_capabilities) echo '{"what":"describe_capabilities","tool":"interact","mode":"click"}' ;;
        configure/tutorial) echo '{"what":"tutorial","topic":"getting_started"}' ;;
        configure/save_sequence) echo '{"what":"save_sequence","name":"connected-action-coverage","steps":[{"what":"get_text","selector":"body"}]}' ;;
        configure/get_sequence|configure/delete_sequence|configure/replay_sequence) echo '{"what":"'"$mode"'","name":"connected-action-coverage"}' ;;
        configure/security_mode) echo '{"what":"security_mode","mode":"normal"}' ;;
        configure/network_recording) echo '{"what":"network_recording","operation":"status"}' ;;
        configure/action_jitter) echo '{"what":"action_jitter","enabled":false}' ;;
        configure/report_issue) echo '{"what":"report_issue","operation":"preview","title":"Connected action coverage"}' ;;
        configure/qa_fixture) echo '{"what":"qa_fixture","fixture_action":"validate","fixture":{"version":1}}' ;;
        configure/setup_quality_gates) echo '{"what":"setup_quality_gates","target_dir":'"$(json_string "$PROJECT_ROOT")"'}' ;;
        interact/highlight) echo '{"what":"highlight","selector":"#sf-btn"}' ;;
        interact/subtitle) echo '{"what":"subtitle","text":"Connected action coverage"}' ;;
        interact/save_state) echo '{"what":"save_state","snapshot_name":"connected-action-coverage"}' ;;
        interact/load_state|interact/delete_state) echo '{"what":"'"$mode"'","snapshot_name":"connected-action-coverage"}' ;;
        interact/set_storage) echo '{"what":"set_storage","storage_type":"localStorage","key":"kaboom_uat","value":"ok"}' ;;
        interact/delete_storage) echo '{"what":"delete_storage","storage_type":"localStorage","key":"kaboom_uat"}' ;;
        interact/clear_storage) echo '{"what":"clear_storage","storage_type":"sessionStorage"}' ;;
        interact/set_cookie) echo '{"what":"set_cookie","name":"kaboom_uat","value":"ok"}' ;;
        interact/delete_cookie) echo '{"what":"delete_cookie","name":"kaboom_uat"}' ;;
        interact/execute_js) echo '{"what":"execute_js","script":"({title: document.title, ready: document.readyState})"}' ;;
        interact/navigate) echo '{"what":"navigate","url":'"$(json_string "${base_url}?navigation=1")"'}' ;;
        interact/new_tab) echo '{"what":"new_tab","url":'"$(json_string "$base_url")"'}' ;;
        interact/switch_tab) echo '{"what":"switch_tab","tab_id":'"$(tracked_tab_id)"'}' ;;
        interact/close_tab) echo '{"what":"close_tab","tab_id":'"${HEALTH_EXTRA_TAB_ID:-0}"'}' ;;
        interact/activate_tab) echo '{"what":"activate_tab"}' ;;
        interact/click|interact/focus|interact/hover) echo '{"what":"'"$mode"'","selector":"#sf-btn"}' ;;
        interact/open_composer) echo '{"what":"open_composer","scope_selector":"#uat-composer-scope"}' ;;
        interact/submit_active_composer) echo '{"what":"submit_active_composer","scope_selector":"#uat-composer-active-scope"}' ;;
        interact/type) echo '{"what":"type","selector":"#sf-name","text":"Kaboom UAT"}' ;;
        interact/select) echo '{"what":"select","selector":"#sf-role","value":"admin"}' ;;
        interact/check) echo '{"what":"check","selector":"#sf-agree","checked":true}' ;;
        interact/get_text) echo '{"what":"get_text","selector":"#sf-result"}' ;;
        interact/get_value) echo '{"what":"get_value","selector":"#sf-name"}' ;;
        interact/get_attribute) echo '{"what":"get_attribute","selector":"#sf-btn","name":"type"}' ;;
        interact/set_attribute) echo '{"what":"set_attribute","selector":"#sf-btn","name":"data-uat","value":"ok"}' ;;
        interact/scroll_to) echo '{"what":"scroll_to","selector":"#sf-scroll-target"}' ;;
        interact/wait_for) echo '{"what":"wait_for","selector":"#sf-btn","timeout_ms":2000}' ;;
        interact/key_press) echo '{"what":"key_press","key":"Tab"}' ;;
        interact/paste) echo '{"what":"paste","selector":"#sf-name","text":"Kaboom paste"}' ;;
        interact/query) echo '{"what":"query","selector":"#sf-btn","query_type":"exists"}' ;;
        interact/navigate_and_wait_for) echo '{"what":"navigate_and_wait_for","url":'"$(json_string "$base_url")"', "wait_for":"#sf-btn"}' ;;
        interact/navigate_and_document) echo '{"what":"navigate_and_document","selector":"#sf-link","wait_for_url_change":true,"timeout_ms":15000}' ;;
        interact/fill_form) echo '{"what":"fill_form","fields":[{"selector":"#sf-user","value":"kaboom"},{"selector":"#sf-email2","value":"uat@example.com"}]}' ;;
        interact/fill_form_and_submit) echo '{"what":"fill_form_and_submit","fields":[{"selector":"#sf-user","value":"kaboom"},{"selector":"#sf-email2","value":"uat@example.com"}],"submit_selector":"#sf-submit"}' ;;
        interact/upload) echo '{"what":"upload","selector":"#file-input","file_path":"/tmp/kaboom-connected-action-coverage.txt"}' ;;
        interact/draw_mode_start) echo '{"what":"draw_mode_start","annot_session":"connected-action-coverage"}' ;;
        interact/hardware_click) echo '{"what":"hardware_click","x":10,"y":10,"tab_id":'"$(tracked_tab_id)"'}' ;;
        interact/batch) echo '{"what":"batch","steps":[{"what":"get_text","selector":"body"}]}' ;;
        interact/clipboard_write) echo '{"what":"clipboard_write","text":"Kaboom connected coverage"}' ;;
        analyze/dom|analyze/computed_styles) echo '{"what":"'"$mode"'","selector":"#sf-btn"}' ;;
        analyze/link_validation) echo '{"what":"link_validation","urls":['"$(json_string "$base_url")"']}' ;;
        analyze/link_health) echo '{"what":"link_health","domain":'"$(json_string "$base_url")"',"max_workers":1}' ;;
        analyze/annotation_detail) echo '{"what":"annotation_detail","correlation_id":"uat-missing-annotation"}' ;;
        analyze/api_validation) echo '{"what":"api_validation","operation":"analyze","url":'"$(json_string "$base_url")"'}' ;;
        analyze/draw_session) echo '{"what":"draw_session","file":"uat-missing-session.json"}' ;;
        analyze/form_state|analyze/form_validation) echo '{"what":"'"$mode"'","selector":"#smoke-form"}' ;;
        analyze/data_table) echo '{"what":"data_table","selector":"table","max_rows":5}' ;;
        analyze/visual_baseline) echo '{"what":"visual_baseline","name":"connected-action-coverage"}' ;;
        analyze/visual_diff) echo '{"what":"visual_diff","name":"connected-action-coverage","baseline":"connected-action-coverage"}' ;;
        analyze/audit) echo '{"what":"audit","categories":["accessibility","performance"]}' ;;
        analyze/performance_trace|analyze/react_profile) echo '{"what":"'"$mode"'","action":"stop","tab_id":'"$(tracked_tab_id)"'}' ;;
        analyze/verification) echo '{"what":"verification","operation":"define","contract":{"schema_version":"1","contract_id":"connected-action-coverage","assertions":[{"assertion_id":"reachable","description":"The connected verification handler is reachable","required_evidence":["dom"]}]}}' ;;
        *) echo '{"what":"'"$mode"'"}' ;;
    esac
}

# recording_id_from_response is a canonical framework helper.

ensure_event_recording() {
    [ -n "${HEALTH_RECORDING_ID:-}" ] && return 0

    local response
    response="$(call_tool "configure" '{"what":"event_recording_start","name":"connected-action-coverage"}')"
    HEALTH_RECORDING_ID="$(recording_id_from_response "$response")"
    if [ -z "$HEALTH_RECORDING_ID" ]; then
        fail "Event recording start returned no recording_id: $(truncate "$(extract_content_text "$response")")"
        return 1
    fi
}

ensure_fixture_page() {
    local fixture_url
    fixture_url="$(connected_fixture_url)"
    local fixture_attempt=1
    local response
    # Wait for the extension only. navigate_and_wait_for is an escape action that
    # re-tracks the tab it lands on, so demanding a tracked tab up front would turn
    # any transient tracking loss — a closed tab, a released undebuggable target —
    # into an unrecoverable failure for every later action that needs a target.
    if ! uat_wait_for_extension "$PORT"; then
        fail "Connected browser was unavailable before fixture navigation"
        return 1
    fi
    while [ "$fixture_attempt" -le 2 ]; do
        response="$(call_tool "interact" '{"what":"navigate_and_wait_for","url":'"$(json_string "$fixture_url")"',"wait_for":"#sf-btn"}')"
        if check_valid_jsonrpc "$response" && ! check_is_error "$response"; then
            break
        fi
        if [ "$fixture_attempt" -eq 1 ]; then
            uat_wait_for_connected_browser "$PORT" "$WRAPPER" || return 1
        fi
        fixture_attempt=$((fixture_attempt + 1))
    done
    if [ "$fixture_attempt" -gt 2 ]; then
        fail "Could not navigate to the connected action fixture: $(truncate "$(extract_content_text "$response")")"
        return 1
    fi
    if ! uat_wait_for_connected_browser "$PORT" "$WRAPPER"; then
        fail "Connected action fixture did not acknowledge readiness"
        return 1
    fi
    HEALTH_TRACKED_TAB_ID="$(uat_connected_tracked_tab "$PORT" "$WRAPPER")"
}

report_target_drift() {
    local tool="$1"
    local mode="$2"
    local tracked=""
    local tracked_id=""
    local tracked_url=""
    tracked="$(uat_connected_tracked_tab "$PORT" "$WRAPPER")"
    tracked_id="$(printf '%s' "$tracked" | cut -f1)"
    tracked_url="$(printf '%s' "$tracked" | cut -f2-)"
    case "$tracked_url" in
        http://*|https://*)
            if [ -n "$UAT_DISPOSABLE_TAB_ID" ] && [ "$tracked_id" != "$UAT_DISPOSABLE_TAB_ID" ]; then
                echo "  TARGET DRIFT after $tool/$mode: tab_id=$tracked_id expected=$UAT_DISPOSABLE_TAB_ID url=$tracked_url"
            fi
            ;;
        *)
            echo "  TARGET DRIFT after $tool/$mode: tab_id=$tracked_id expected=$UAT_DISPOSABLE_TAB_ID url=$tracked_url"
            ;;
    esac
}

# Bounded, self-classifying outcomes a browser-permission-gated action may report.
# The browser alone decides whether the permission is granted, so the contract is
# that the action always names its outcome — never hangs into a generic timeout.
CLIPBOARD_BOUNDED_OUTCOMES='clipboard_permission_denied|clipboard_permission_prompt_required'
CLIPBOARD_BOUNDED_OUTCOMES="$CLIPBOARD_BOUNDED_OUTCOMES|clipboard_read_navigation_cancelled"
CLIPBOARD_BOUNDED_OUTCOMES="$CLIPBOARD_BOUNDED_OUTCOMES|clipboard_read_context_destroyed"
CLIPBOARD_BOUNDED_OUTCOMES="$CLIPBOARD_BOUNDED_OUTCOMES|clipboard_document_not_focused|clipboard_read_timeout"

evaluate_permission_gated() {
    local tool="$1"
    local mode="$2"
    local body=""
    body="$(extract_content_text "$3")"
    if check_is_error "$3"; then
        fail "$tool/$mode rejected its connected health payload: $(command_failure_message "$3")"
    elif printf '%s' "$body" | grep -q 'execution_timeout'; then
        fail "$tool/$mode hung into a generic execution_timeout instead of naming its permission outcome"
    elif printf '%s' "$body" | grep -qE "\"($CLIPBOARD_BOUNDED_OUTCOMES)\""; then
        pass "$tool/$mode reported a bounded, classified permission outcome"
    elif printf '%s' "$body" | grep -q '"text"'; then
        pass "$tool/$mode completed against a granted browser permission"
    else
        fail "$tool/$mode returned an unclassified permission outcome: $(truncate "$body")"
    fi
}

prepare_action() {
    local action="$1/$2"
    local response=""
    local script=""
    local visual_baseline_attempt=1
    case "$action" in
        observe/indexeddb)
            script='new Promise((resolve,reject)=>{const r=indexedDB.open("kaboom_uat",1);r.onupgradeneeded=()=>r.result.createObjectStore("items",{keyPath:"id"});r.onsuccess=()=>{const db=r.result;const tx=db.transaction("items","readwrite");tx.objectStore("items").put({id:1,value:"ready"});tx.oncomplete=()=>{db.close();resolve("ready")};tx.onerror=()=>reject(tx.error)}})'
            ;;
        interact/open_composer)
            ensure_fixture_page || return 1
            script='document.body.insertAdjacentHTML("beforeend","<section id=\"uat-composer-scope\"><button type=\"button\" aria-label=\"Open composer\">Compose</button></section>"); "ready"'
            ;;
        interact/submit_active_composer)
            ensure_fixture_page || return 1
            script='document.body.insertAdjacentHTML("beforeend","<section id=\"uat-composer-active-scope\" role=\"dialog\"><div contenteditable=\"true\" role=\"textbox\">ready</div><button type=\"button\" aria-label=\"Send\">Send</button></section>"); document.querySelector("#uat-composer-active-scope [contenteditable]").focus(); "ready"'
            ;;
        interact/navigate_and_document)
            ensure_fixture_page || return 1
            script='document.getElementById("sf-link").href="/tests/interact.html?documented=1"; "ready"'
            ;;
        interact/hardware_click)
            ensure_fixture_page || return 1
            ;;
        interact/back)
            call_tool "interact" \
                '{"what":"navigate","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html?history=1")"'}' \
                >/dev/null
            ;;
        interact/forward)
            call_tool "interact" \
                '{"what":"navigate","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html?history=1")"'}' \
                >/dev/null
            call_tool "interact" '{"what":"back"}' >/dev/null
            ;;
        interact/close_tab)
            if [ -z "${HEALTH_EXTRA_TAB_ID:-}" ]; then
                response="$(call_tool "interact" '{"what":"new_tab","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html")"'}')"
                HEALTH_EXTRA_TAB_ID="$(uat_new_tab_id "$response")"
            fi
            if [ -z "$HEALTH_EXTRA_TAB_ID" ]; then
                fail "Could not create a disposable close_tab target"
                return 1
            fi
            ;;
        interact/confirm_top_dialog)
            script='document.body.insertAdjacentHTML("beforeend","<div role=\"dialog\" id=\"uat-dialog\"><button>Confirm</button></div>"); "ready"'
            ;;
        interact/dismiss_top_overlay|interact/auto_dismiss_overlays)
            script='document.body.insertAdjacentHTML("beforeend","<div role=\"dialog\" id=\"uat-overlay\"><button aria-label=\"Close\">Close</button></div>"); "ready"'
            ;;
        interact/fill_form|interact/fill_form_and_submit|analyze/visual_baseline)
            ensure_fixture_page || return 1
            ;;
        analyze/visual_diff)
            ensure_fixture_page || return 1
            while [ "$visual_baseline_attempt" -le 2 ]; do
                response="$(call_tool "analyze" '{"what":"visual_baseline","name":"connected-action-coverage"}')"
                if check_valid_jsonrpc "$response" && ! check_is_error "$response"; then
                    break
                fi
                if [ "$visual_baseline_attempt" -eq 1 ]; then
                    uat_wait_for_connected_browser "$PORT" "$WRAPPER" || return 1
                    ensure_fixture_page || return 1
                fi
                visual_baseline_attempt=$((visual_baseline_attempt + 1))
            done
            if [ "$visual_baseline_attempt" -gt 2 ]; then
                fail "Could not capture the visual diff baseline: $(truncate "$(extract_content_text "$response")")"
                return 1
            fi
            ;;
        analyze/performance_trace|analyze/react_profile)
            # A profile describes one tab. Name the fixture explicitly so start and
            # stop cannot land on different targets, and so a refusal is reported
            # against the tab we meant to profile.
            ensure_fixture_page || return 1
            response="$(call_tool "analyze" '{"what":"'"$2"'","action":"start","tab_id":'"$(tracked_tab_id)"'}')"
            if is_debugger_refusal "$(command_failure_message "$response")"; then
                skip "$action: Chrome refused debugger access to the target (browser security boundary, not a product failure)"
                return 1
            fi
            if ! check_valid_jsonrpc "$response" || check_is_error "$response"; then
                fail "Could not start $action lifecycle: $(command_failure_message "$response")"
                return 1
            fi
            ;;
        configure/event_recording_stop)
            ensure_event_recording
            ;;
    esac
    [ -z "$script" ] || call_tool "interact" '{"what":"execute_js","script":'"$(json_string "$script")"'}' >/dev/null
}

call_action_with_retry() {
    local tool="$1"
    local mode="$2"
    local args="$3"
    local attempt=1
    local response=""
    local response_text=""
    while [ "$attempt" -le 3 ]; do
        response="$(call_tool "$tool" "$args")"
        response_text="$(extract_content_text "$response")"
        # 'No tab is being tracked' is a recoverable target loss, not an action
        # defect: the fixture is re-established below and the action retried.
        if ! printf '%s\n%s' "$response" "$response_text" |
                grep -qE 'context deadline exceeded|extension_timeout|no_result|dismiss_loop_detected|extension_lost_command|screenshot_failed|No tab is being tracked'; then
            printf '%s' "$response"
            return 0
        fi
        if [ "$attempt" -eq 3 ]; then
            break
        fi
        if [ "$tool/$mode" = "interact/close_tab" ]; then
            HEALTH_EXTRA_TAB_ID=""
        fi
        # ensure_fixture_page waits for the extension, re-navigates, and re-tracks,
        # so it recovers targets that uat_wait_for_connected_browser can only observe.
        if ! ensure_fixture_page || ! prepare_action "$tool" "$mode"; then
            break
        fi
        args="$(action_args "$tool" "$mode")"
        attempt=$((attempt + 1))
    done
    printf '%s' "$response"
}

touch /tmp/kaboom-connected-action-coverage.txt
start_daemon || {
    fail "Connected action coverage could not start with an attached browser"
    finish_category
}

tools_response="$(send_mcp '{"jsonrpc":"2.0","id":33,"method":"tools/list","params":{}}' "connected_action_schema")"
if ! check_valid_jsonrpc "$tools_response"; then
    fail "tools/list did not return a valid JSON-RPC envelope"
    finish_category
fi

ensure_fixture_page || finish_category

schema_count="$(printf '%s' "$tools_response" | jq --argjson tools '["observe","generate","configure","interact","analyze"]' '
    [.result.tools[] | select(.name as $name | $tools | index($name)) | .inputSchema.properties.what.enum[]] | length
')"
executed_count=0
classified_count=0
selected_count=0
HEALTH_TRACKED_TAB_ID="$(uat_connected_tracked_tab "$PORT" "$WRAPPER")"
HEALTH_RECORDING_ID=""

for tool in $TOOLS; do
    modes="$(printf '%s' "$tools_response" | jq -r --arg tool "$tool" '
        .result.tools[] |
        select(.name == $tool) |
        .inputSchema.properties.what.enum[]
    ')"
    if [ -z "$modes" ]; then
        fail "$tool exposes no discoverable what modes"
        continue
    fi
    for mode in $modes; do
        expectation="$(action_expectation "$tool" "$mode")"
        if [ "$expectation" = "unclassified" ]; then
            fail "Unclassified live schema action: $tool/$mode"
            continue
        fi
        classified_count=$((classified_count + 1))
        if [ -n "$ACTION_FILTER" ] && [ "$ACTION_FILTER" != "$tool/$mode" ]; then
            continue
        fi
        selected_count=$((selected_count + 1))
        begin_test "33.$((executed_count + 1))" "$tool/$mode" \
            "Invoke the live schema action against the attached deterministic page" \
            "A valid MCP response proves the canonical dispatcher and connected execution path are reachable"
        case "$expectation" in
            user_mediated)
                skip "$tool/$mode requires an explicit browser user gesture and is covered by recording UAT"
                continue
                ;;
            destructive)
                skip "$tool/$mode mutates the checked-out project and is explicitly excluded from connected health"
                continue
                ;;
        esac
        if ! prepare_action "$tool" "$mode"; then
            continue
        fi
        args="$(action_args "$tool" "$mode")"
        if [ "$tool/$mode" = "configure/replay_sequence" ]; then
            call_tool "configure" \
                '{"what":"save_sequence","name":"connected-action-coverage","steps":[{"what":"get_text","selector":"body"}]}' \
                >/dev/null
        fi
        response="$(call_action_with_retry "$tool" "$mode" "$args")"
        executed_count=$((executed_count + 1))
        report_target_drift "$tool" "$mode"
        if [ "$tool/$mode" = "interact/new_tab" ]; then
            HEALTH_EXTRA_TAB_ID="$(uat_new_tab_id "$response")"
        fi
        if [ "$tool/$mode" = "configure/event_recording_start" ]; then
            HEALTH_RECORDING_ID="$(recording_id_from_response "$response")"
        fi
        if ! check_valid_jsonrpc "$response"; then
            fail "$tool/$mode returned an invalid JSON-RPC envelope: $(truncate "$response")"
        elif echo "$expectation" | grep -q '^expected_error:'; then
            expected_code="${expectation#expected_error:}"
            if ! check_is_error "$response"; then
                fail "$tool/$mode should return classified $expected_code without fixture data"
            elif ! extract_content_text "$response" | grep -q "\"error_code\":\"$expected_code\""; then
                fail "$tool/$mode returned the wrong classified error: $(truncate "$(extract_content_text "$response")")"
            else
                pass "$tool/$mode reached its expected $expected_code contract"
            fi
        elif [ "$expectation" = "permission_gated" ]; then
            evaluate_permission_gated "$tool" "$mode" "$response"
        elif is_debugger_refusal "$(extract_content_text "$response")"; then
            skip "$tool/$mode: Chrome refused debugger access to the target (browser security boundary, not a product failure)"
        elif check_is_error "$response"; then
            fail "$tool/$mode rejected its connected health payload: $(truncate "$(extract_content_text "$response")")"
        else
            pass "$tool/$mode returned a successful connected response"
        fi
    done
done

if [ -n "$ACTION_FILTER" ] && [ "$selected_count" -eq 0 ]; then
    fail "No live schema action matched KABOOM_UAT_ACTION=$ACTION_FILTER"
fi

if [ "$classified_count" -ne "$schema_count" ]; then
    fail "Action coverage mismatch: schema_count=$schema_count classified_count=$classified_count"
else
    pass "Action coverage matched the live schema: $classified_count classified, $executed_count invoked across five tools"
fi

rm -f /tmp/kaboom-connected-action-coverage.txt
finish_category
