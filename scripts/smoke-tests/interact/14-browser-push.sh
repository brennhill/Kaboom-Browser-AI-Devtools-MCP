#!/bin/bash
# 14-browser-push.sh — 14.1-14.6: Browser push pipeline (push endpoints, inbox, piggyback).
set -eo pipefail

begin_category "14" "Browser Push" "6"

PUSH_TEST_CLIENT_HEADER="kaboom-extension/smoke"
PUSH_TEST_SUPPORTS_SAMPLING="false"
PUSH_TEST_SUPPORTS_NOTIFICATIONS="false"

_push_expected_method() {
    if [ "$PUSH_TEST_SUPPORTS_SAMPLING" = "true" ]; then
        echo "sampling"
        return
    fi
    if [ "$PUSH_TEST_SUPPORTS_NOTIFICATIONS" = "true" ]; then
        echo "notification"
        return
    fi
    echo "inbox"
}

_push_expected_status() {
    if [ "$(_push_expected_method)" = "sampling" ]; then
        echo "delivered"
    else
        echo "queued"
    fi
}

_push_http_get() {
    local path="$1"
    curl -sS --connect-timeout 3 --max-time 8 \
        -H "X-Kaboom-Client: ${PUSH_TEST_CLIENT_HEADER}" \
        "http://127.0.0.1:${PORT}${path}"
}

_push_http_post_json() {
    local path="$1"
    local payload="$2"
    curl -sS --connect-timeout 3 --max-time 8 \
        -H "X-Kaboom-Client: ${PUSH_TEST_CLIENT_HEADER}" \
        -H "Content-Type: application/json" \
        -X POST "http://127.0.0.1:${PORT}${path}" \
        -d "$payload"
}

_push_drain_inbox() {
    call_tool "observe" '{"what":"inbox"}' >/dev/null 2>&1 || true
}

# ── Test 14.1: Schema includes observe(inbox) ────────────
begin_test "14.1" "[DAEMON ONLY] Schema: observe supports inbox mode" \
    "Verify tools/list includes inbox in observe what enum" \
    "Tests: push inbox retrieval is discoverable in MCP schema"

run_test_14_1() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    if echo "$tools_resp" | jq -e '.result.tools[] | select(.name=="observe") | .inputSchema.properties.what.enum[] | select(.=="inbox")' >/dev/null 2>&1; then
        pass "observe(inbox) present in schema."
    else
        fail "observe(inbox) missing from schema."
    fi
}
run_test_14_1

# ── Test 14.2: Capabilities endpoint ─────────────────────
begin_test "14.2" "[DAEMON ONLY] /push/capabilities returns push routing flags" \
    "Call extension-facing push capabilities endpoint and cache capability flags for later assertions" \
    "Tests: push capability surface and header gating path"

run_test_14_2() {
    local caps_resp
    caps_resp=$(_push_http_get "/push/capabilities" || true)
    log_diagnostic "14.2" "GET /push/capabilities" "$caps_resp"

    if [ -z "$caps_resp" ] || ! echo "$caps_resp" | jq -e '.supports_sampling != null and .supports_notifications != null and .inbox_count != null' >/dev/null 2>&1; then
        fail "Invalid /push/capabilities response: $(truncate "$caps_resp" 200)"
        return
    fi

    PUSH_TEST_SUPPORTS_SAMPLING=$(echo "$caps_resp" | jq -r '.supports_sampling // false')
    PUSH_TEST_SUPPORTS_NOTIFICATIONS=$(echo "$caps_resp" | jq -r '.supports_notifications // false')

    pass "Capabilities read: sampling=${PUSH_TEST_SUPPORTS_SAMPLING}, notifications=${PUSH_TEST_SUPPORTS_NOTIFICATIONS}."
}
run_test_14_2

# ── Test 14.3: Push message route + inbox delivery ───────
begin_test "14.3" "[DAEMON ONLY] POST /push/message routes and appears in observe(inbox)" \
    "Send a push chat message, verify delivery_method/status and inbox event payload" \
    "Tests: push route wiring + routing contract + inbox retrieval"

run_test_14_3() {
    _push_drain_inbox

    local marker
    marker="SMOKE_PUSH_CHAT_${SMOKE_MARKER}_143"
    local payload
    payload=$(jq -nc --arg message "$marker" --arg page "$SMOKE_EXAMPLE_URL" '{message:$message, page_url:$page, tab_id:1}')

    local post_resp
    post_resp=$(_push_http_post_json "/push/message" "$payload" || true)
    log_diagnostic "14.3" "POST /push/message" "$post_resp"

    local delivery_method expected_method
    delivery_method=$(echo "$post_resp" | jq -r '.delivery_method // empty' 2>/dev/null)
    expected_method=$(_push_expected_method)
    local status expected_status
    status=$(echo "$post_resp" | jq -r '.status // empty' 2>/dev/null)
    expected_status=$(_push_expected_status)

    if [ -z "$delivery_method" ] || [ "$delivery_method" != "$expected_method" ]; then
        fail "Unexpected delivery_method for /push/message: got='$delivery_method' expected='$expected_method'. Response: $(truncate "$post_resp" 200)"
        return
    fi
    if [ -z "$status" ] || [ "$status" != "$expected_status" ]; then
        fail "Unexpected status for /push/message: got='$status' expected='$expected_status'. Response: $(truncate "$post_resp" 200)"
        return
    fi

    local inbox_resp inbox_text
    inbox_resp=$(call_tool "observe" '{"what":"inbox"}')
    inbox_text=$(extract_content_text "$inbox_resp")
    log_diagnostic "14.3" "observe(inbox)" "$inbox_resp" "$inbox_text"

    if echo "$inbox_text" | grep -q "$marker" && echo "$inbox_text" | grep -q '"type":"chat"'; then
        pass "Push message routed via ${delivery_method} and retrieved from inbox."
    else
        fail "Pushed chat event not found in inbox. Inbox content: $(truncate "$inbox_text" 220)"
    fi
}
run_test_14_3

# ── Test 14.4: piggyback hint lifecycle ──────────────────
begin_test "14.4" "[DAEMON ONLY] _push_* piggyback appears when inbox non-empty and clears after drain" \
    "Queue a push event, verify _push_* piggyback on regular tool response, then drain and verify hint disappears" \
    "Tests: inbox piggyback contract for LLM guidance"

run_test_14_4() {
    _push_drain_inbox

    local marker
    marker="SMOKE_PUSH_HINT_${SMOKE_MARKER}_144"
    local payload
    payload=$(jq -nc --arg message "$marker" --arg page "$SMOKE_EXAMPLE_URL" '{message:$message, page_url:$page, tab_id:1}')
    _push_http_post_json "/push/message" "$payload" >/dev/null 2>&1 || true

    local hinted_resp hinted_text
    local hinted_seen=false
    for _hint_poll in $(seq 1 8); do
        hinted_resp=$(call_tool "configure" '{"what":"health"}')
        hinted_text=$(extract_content_text "$hinted_resp")
        if echo "$hinted_resp" | grep -q "_push_"; then
            hinted_seen=true
            break
        fi
        sleep 0.25
    done
    log_diagnostic "14.4" "configure(health) with pending push" "$hinted_resp" "$hinted_text"

    if [ "$hinted_seen" != "true" ]; then
        fail "Expected _push_* piggyback when inbox had pending event. Content: $(truncate "$hinted_text" 220)"
        return
    fi

    _push_drain_inbox

    local clean_resp clean_text
    clean_resp=$(call_tool "configure" '{"what":"health"}')
    clean_text=$(extract_content_text "$clean_resp")
    log_diagnostic "14.4" "configure(health) after inbox drain" "$clean_resp" "$clean_text"

    if echo "$clean_resp" | grep -q "_push_"; then
        fail "Expected no _push_* piggyback after draining inbox. Content: $(truncate "$clean_text" 220)"
    else
        pass "_push_* piggyback appears and clears correctly."
    fi
}
run_test_14_4

# ── Test 14.5: Push screenshot route ─────────────────────
begin_test "14.5" "[DAEMON ONLY] POST /push/screenshot strips data URL and stores screenshot event" \
    "Push a screenshot data URL and verify observe(inbox) exposes a screenshot event with stripped base64 data" \
    "Tests: screenshot push ingest and normalization"

run_test_14_5() {
    _push_drain_inbox

    local screenshot_b64
    screenshot_b64="U01PS0VfUFVTSF9TQ1JFRU5TSE9U"
    local payload
    payload=$(jq -nc --arg b64 "$screenshot_b64" --arg note "smoke screenshot note" --arg page "$SMOKE_EXAMPLE_URL" \
        '{screenshot_data_url:("data:image/png;base64," + $b64), note:$note, page_url:$page, tab_id:1}')

    local post_resp
    post_resp=$(_push_http_post_json "/push/screenshot" "$payload" || true)
    log_diagnostic "14.5" "POST /push/screenshot" "$post_resp"

    local delivery_method expected_method
    delivery_method=$(echo "$post_resp" | jq -r '.delivery_method // empty' 2>/dev/null)
    expected_method=$(_push_expected_method)
    local status expected_status
    status=$(echo "$post_resp" | jq -r '.status // empty' 2>/dev/null)
    expected_status=$(_push_expected_status)

    if [ -z "$delivery_method" ] || [ "$delivery_method" != "$expected_method" ]; then
        fail "Unexpected delivery_method for /push/screenshot: got='$delivery_method' expected='$expected_method'. Response: $(truncate "$post_resp" 200)"
        return
    fi
    if [ -z "$status" ] || [ "$status" != "$expected_status" ]; then
        fail "Unexpected status for /push/screenshot: got='$status' expected='$expected_status'. Response: $(truncate "$post_resp" 200)"
        return
    fi

    local inbox_resp inbox_text
    inbox_resp=$(call_tool "observe" '{"what":"inbox"}')
    inbox_text=$(extract_content_text "$inbox_resp")
    log_diagnostic "14.5" "observe(inbox)" "$inbox_resp" "$inbox_text"

    if echo "$inbox_text" | grep -q '"type":"screenshot"' && echo "$inbox_text" | grep -q "$screenshot_b64"; then
        pass "Screenshot push routed via ${delivery_method} and screenshot_b64 is normalized."
    else
        fail "Screenshot event missing/invalid in inbox. Content: $(truncate "$inbox_text" 220)"
    fi
}
run_test_14_5

# ── Test 14.6: Draw-mode completion auto-push ────────────
begin_test "14.6" "[DAEMON ONLY] /draw-mode/complete auto-pushes annotation events to inbox" \
    "Post a draw-mode completion payload with annotations and verify an annotations push event is retrievable from inbox" \
    "Tests: annotation workflow auto-push integration"

run_test_14_6() {
    _push_drain_inbox

    local ann_marker
    ann_marker="SMOKE_PUSH_ANNOTATION_${SMOKE_MARKER}_146"
    local detail_id
    detail_id="ann_detail_smoke_146"
    local payload
    payload=$(jq -nc \
        --arg page "$SMOKE_EXAMPLE_URL" \
        --arg text "$ann_marker" \
        --arg detail "$detail_id" \
        '{tab_id:1, page_url:$page, annot_session_name:"smoke-push-annotations", annotations:[{id:"ann-smoke-146", rect:{x:12,y:18,width:120,height:36}, text:$text, timestamp:1700000000000, page_url:$page, element_summary:"smoke element", correlation_id:$detail}], element_details:{}}')

    local draw_resp
    draw_resp=$(_push_http_post_json "/draw-mode/complete" "$payload" || true)
    log_diagnostic "14.6" "POST /draw-mode/complete" "$draw_resp"

    if ! echo "$draw_resp" | jq -e '.status == "stored" and .annotation_count == 1' >/dev/null 2>&1; then
        fail "draw-mode completion did not store expected payload. Response: $(truncate "$draw_resp" 220)"
        return
    fi

    local inbox_resp inbox_text
    inbox_resp=$(call_tool "observe" '{"what":"inbox"}')
    inbox_text=$(extract_content_text "$inbox_resp")
    log_diagnostic "14.6" "observe(inbox)" "$inbox_resp" "$inbox_text"

    if echo "$inbox_text" | grep -q '"type":"annotations"' && echo "$inbox_text" | grep -q "$ann_marker"; then
        pass "Draw-mode completion auto-pushed annotation event to inbox."
    else
        fail "Annotation push event missing after draw completion. Content: $(truncate "$inbox_text" 220)"
    fi
}
run_test_14_6
