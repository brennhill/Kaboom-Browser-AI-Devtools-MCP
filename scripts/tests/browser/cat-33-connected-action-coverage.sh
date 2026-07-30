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

json_string() {
    printf '%s' "$1" | jq -Rs .
}

action_expectation() {
    case "$1/$2" in
        observe/command_result) echo "expected_error:no_data" ;;
        observe/recording_actions|observe/log_diff_report) echo "expected_error:internal_error" ;;
        observe/playback_results) echo "expected_error:no_data" ;;
        configure/playback|configure/log_diff) echo "expected_error:internal_error" ;;
        configure/setup_quality_gates) echo "destructive" ;;
        interact/screen_recording_start|interact/screen_recording_stop) echo "user_mediated" ;;
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
        configure/action_jitter|configure/report_issue|\
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
        interact/activate_tab|interact/explore_page|interact/batch|interact/clipboard_read|\
        interact/clipboard_write|\
        analyze/dom|analyze/performance|analyze/accessibility|analyze/error_clusters|\
        analyze/navigation_patterns|analyze/security_audit|analyze/third_party_audit|analyze/link_health|\
        analyze/link_validation|analyze/page_summary|analyze/annotations|analyze/api_validation|\
        analyze/draw_history|analyze/computed_styles|analyze/forms|analyze/form_state|\
        analyze/form_validation|analyze/data_table|analyze/visual_baseline|analyze/visual_diff|\
        analyze/visual_baselines|analyze/navigation|analyze/page_structure|analyze/audit|\
        analyze/feature_gates|analyze/page_issues) echo "success" ;;
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
        interact/switch_tab) echo '{"what":"switch_tab","tab_id":'"${HEALTH_TRACKED_TAB_ID:-0}"'}' ;;
        interact/close_tab) echo '{"what":"close_tab","tab_id":'"${HEALTH_EXTRA_TAB_ID:-0}"'}' ;;
        interact/activate_tab) echo '{"what":"activate_tab"}' ;;
        interact/click|interact/focus|interact/hover) echo '{"what":"'"$mode"'","selector":"#sf-btn"}' ;;
        interact/open_composer) echo '{"what":"open_composer","selector":"#uat-composer"}' ;;
        interact/submit_active_composer) echo '{"what":"submit_active_composer","selector":"#uat-composer-active"}' ;;
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
        interact/navigate_and_document) echo '{"what":"navigate_and_document","selector":"#sf-link"}' ;;
        interact/fill_form) echo '{"what":"fill_form","fields":[{"selector":"#sf-user","value":"kaboom"},{"selector":"#sf-email2","value":"uat@example.com"}]}' ;;
        interact/fill_form_and_submit) echo '{"what":"fill_form_and_submit","fields":[{"selector":"#sf-user","value":"kaboom"},{"selector":"#sf-email2","value":"uat@example.com"}],"submit_selector":"#sf-submit"}' ;;
        interact/upload) echo '{"what":"upload","selector":"#file-input","file_path":"/tmp/kaboom-connected-action-coverage.txt"}' ;;
        interact/draw_mode_start) echo '{"what":"draw_mode_start","annot_session":"connected-action-coverage"}' ;;
        interact/hardware_click) echo '{"what":"hardware_click","x":10,"y":10}' ;;
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
        *) echo '{"what":"'"$mode"'"}' ;;
    esac
}

prepare_action() {
    local action="$1/$2"
    local script=""
    case "$action" in
        observe/indexeddb)
            script='new Promise((resolve,reject)=>{const r=indexedDB.open("kaboom_uat",1);r.onupgradeneeded=()=>r.result.createObjectStore("items",{keyPath:"id"});r.onsuccess=()=>{const db=r.result;const tx=db.transaction("items","readwrite");tx.objectStore("items").put({id:1,value:"ready"});tx.oncomplete=()=>{db.close();resolve("ready")};tx.onerror=()=>reject(tx.error)}})'
            ;;
        interact/open_composer)
            call_tool "interact" '{"what":"navigate","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html")"'}' >/dev/null
            script='document.body.insertAdjacentHTML("beforeend","<div id=\"uat-composer\" contenteditable=\"true\" role=\"textbox\"></div>"); "ready"'
            ;;
        interact/submit_active_composer)
            call_tool "interact" '{"what":"navigate","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html")"'}' >/dev/null
            script='document.body.insertAdjacentHTML("beforeend","<div id=\"uat-composer-active\" contenteditable=\"true\" role=\"textbox\">ready</div><button id=\"uat-send\" aria-label=\"Send\">Send</button>"); document.getElementById("uat-composer-active").focus(); "ready"'
            ;;
        interact/back)
            script='history.pushState({uat:true},"","?history=1"); "ready"'
            ;;
        interact/confirm_top_dialog)
            script='document.body.insertAdjacentHTML("beforeend","<div role=\"dialog\" id=\"uat-dialog\"><button>Confirm</button></div>"); "ready"'
            ;;
        interact/dismiss_top_overlay|interact/auto_dismiss_overlays)
            script='document.body.insertAdjacentHTML("beforeend","<div role=\"dialog\" id=\"uat-overlay\"><button aria-label=\"Close\">Close</button></div>"); "ready"'
            ;;
        interact/fill_form|interact/fill_form_and_submit|analyze/visual_baseline)
            call_tool "interact" '{"what":"navigate","url":'"$(json_string "http://127.0.0.1:${PORT}/tests/interact.html")"'}' >/dev/null
            ;;
    esac
    [ -z "$script" ] || call_tool "interact" '{"what":"execute_js","script":'"$(json_string "$script")"'}' >/dev/null
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
        .result.tools[] | select(.name == $tool) | .inputSchema.properties.what.enum[]
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
        args="$(action_args "$tool" "$mode")"
        prepare_action "$tool" "$mode"
        if [ "$tool/$mode" = "configure/replay_sequence" ]; then
            call_tool "configure" \
                '{"what":"save_sequence","name":"connected-action-coverage","steps":[{"what":"get_text","selector":"body"}]}' \
                >/dev/null
        fi
        response="$(call_tool "$tool" "$args")"
        executed_count=$((executed_count + 1))
        if [ "$tool/$mode" = "interact/new_tab" ]; then
            HEALTH_EXTRA_TAB_ID="$(uat_new_tab_id "$response")"
        fi
        if [ "$tool/$mode" = "configure/event_recording_start" ]; then
            HEALTH_RECORDING_ID="$(extract_content_text "$response" | sed -n 's/.*"recording_id"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -n 1)"
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
