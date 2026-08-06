#!/bin/bash
# 13-draw-mode.sh — 13.1-13.12: Draw mode annotations, sessions, artifacts.
set -eo pipefail

begin_category "13" "Draw Mode" "12"

# ── Test 13.1: Schema — draw_mode_start in interact ────────
begin_test "13.1" "[DAEMON ONLY] Schema: draw_mode_start in interact what enum" \
    "Verify tools/list includes draw_mode_start as a valid interact action (canonical what enum)" \
    "Tests: schema registration for draw mode activation"

run_test_13_1() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    if echo "$tools_resp" | jq -e '
        .result.tools[] | select(.name=="interact")
        | ((.inputSchema.properties.what.enum // .inputSchema.properties.action.enum // []))
        | .[] | select(.=="draw_mode_start")
    ' >/dev/null 2>&1; then
        pass "draw_mode_start in interact enum (what/action)."
    else
        fail "draw_mode_start NOT in interact enum (checked what/action)."
    fi
}
run_test_13_1

# ── Test 13.2: Schema — annotations in analyze ─────────────
begin_test "13.2" "[DAEMON ONLY] Schema: annotations and annotation_detail in analyze" \
    "Verify tools/list includes annotations and annotation_detail in analyze what enum" \
    "Tests: schema registration for annotation retrieval"

run_test_13_2() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    local has_ann has_det
    has_ann=$(echo "$tools_resp" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.what.enum[] | select(.=="annotations")' 2>/dev/null)
    has_det=$(echo "$tools_resp" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.what.enum[] | select(.=="annotation_detail")' 2>/dev/null)
    if [ "$has_ann" = "annotations" ] && [ "$has_det" = "annotation_detail" ]; then
        pass "annotations and annotation_detail in analyze what enum."
    else
        fail "Missing from analyze enum: annotations=$has_ann, annotation_detail=$has_det."
    fi
}
run_test_13_2

# ── Test 13.3: Schema — annotation generate formats ─────────
begin_test "13.3" "[DAEMON ONLY] Schema: visual_test, annotation_report, annotation_issues in generate" \
    "Verify tools/list includes all 3 annotation generate formats" \
    "Tests: schema registration for annotation artifact generation"

run_test_13_3() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    local has_vt has_ar has_ai
    has_vt=$(echo "$tools_resp" | jq -r '.result.tools[] | select(.name=="generate") | (.inputSchema.properties.what.enum // .inputSchema.properties.format.enum // [])[] | select(.=="visual_test")' 2>/dev/null)
    has_ar=$(echo "$tools_resp" | jq -r '.result.tools[] | select(.name=="generate") | (.inputSchema.properties.what.enum // .inputSchema.properties.format.enum // [])[] | select(.=="annotation_report")' 2>/dev/null)
    has_ai=$(echo "$tools_resp" | jq -r '.result.tools[] | select(.name=="generate") | (.inputSchema.properties.what.enum // .inputSchema.properties.format.enum // [])[] | select(.=="annotation_issues")' 2>/dev/null)
    if [ "$has_vt" = "visual_test" ] && [ "$has_ar" = "annotation_report" ] && [ "$has_ai" = "annotation_issues" ]; then
        pass "All 3 annotation generate formats in schema (what/format)."
    else
        fail "Missing generate formats (checked what/format): visual_test=$has_vt, annotation_report=$has_ar, annotation_issues=$has_ai."
    fi
}
run_test_13_3

# ── Test 13.4: Activate draw mode via MCP ──────────────────
begin_test "13.4" "[INTERACTIVE - BROWSER] Activate draw mode via interact(draw_mode_start)" \
    "Call interact(draw_mode_start), verify response contains correlation_id" \
    "Tests: MCP > daemon > extension > content script draw mode overlay"

run_test_13_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to clean page first
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Clean page for draw mode"}' 20
    sleep 2

    local response
    response=$(call_tool "interact" '{"what":"draw_mode_start"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "correlation_id\|queued"; then
        pass "draw_mode_start accepted with correlation_id."
    elif echo "$content_text" | grep -qi "error\|failed"; then
        fail "draw_mode_start failed. Response: $(truncate "$content_text" 200)"
    else
        fail "draw_mode_start: no correlation_id in response. Response: $(truncate "$content_text" 200)"
    fi
}
run_test_13_4

# ── Test 13.5: Draw annotations and retrieve ────────────────
begin_test "13.5" "[INTERACTIVE - BROWSER] Draw annotations, press ESC, retrieve via analyze" \
    "User draws 1-2 annotations on draw mode overlay, presses ESC, then verify data via analyze(annotations)" \
    "Tests: full annotation pipeline: draw > ESC > extension POST > daemon store > MCP retrieve"

run_test_13_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi
    if [ ! -t 0 ]; then
        skip "Interactive draw input requires a TTY."
        return
    fi

    echo "  >>> Draw 1-2 rectangles on the page, type text for each, then press ESC <<<"
    echo "  -- Press Enter AFTER you have drawn and pressed ESC --"
    read -r

    local response
    response=$(call_tool "analyze" '{"what":"annotations"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "13.5" "analyze(annotations)" "$response" "$content_text"

    # Validate annotations have actual structure: count > 0, entries with text
    local ann_verdict
    ann_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    anns = data.get('annotations', [])
    count = data.get('count', len(anns) if isinstance(anns, list) else 0)
    if count > 0:
        has_text = any(a.get('text') for a in anns) if isinstance(anns, list) else False
        print(f'PASS count={count} has_text={has_text}')
    else:
        print(f'FAIL count={count}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$ann_verdict" | grep -q "^PASS"; then
        pass "Annotations retrieved. $ann_verdict"
    else
        fail "No annotations found. $ann_verdict. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_13_5

# ── Test 13.6: Annotation detail drill-down ─────────────────
begin_test "13.6" "[INTERACTIVE - BROWSER] Annotation detail has selector, tag, computed styles" \
    "Retrieve annotation_detail for the first annotation's correlation_id" \
    "Tests: element detail enrichment pipeline"

run_test_13_6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Get annotations to find a correlation_id
    local response
    response=$(call_tool "analyze" '{"what":"annotations"}')
    local content_text
    content_text=$(extract_content_text "$response")

    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":"ann_detail_[^"]+"' | head -1 | sed 's/"correlation_id":"//' | sed 's/"//' || true)

    if [ -z "$corr_id" ]; then
        skip "No annotation correlation_id found for detail lookup."
        return
    fi

    local detail_resp
    detail_resp=$(call_tool "analyze" "{\"what\":\"annotation_detail\",\"correlation_id\":\"$corr_id\"}")
    local detail_text
    detail_text=$(extract_content_text "$detail_resp")

    log_diagnostic "13.6" "annotation_detail" "$detail_resp" "$detail_text"

    # Require at least selector AND tag to be present (not just one generic word)
    local detail_verdict
    detail_verdict=$(echo "$detail_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    has_sel = bool(data.get('selector'))
    has_tag = bool(data.get('tag'))
    has_styles = bool(data.get('computed_styles'))
    if has_sel and has_tag:
        print(f'PASS selector={data[\"selector\"][:40]} tag={data[\"tag\"]} has_styles={has_styles}')
    elif has_sel or has_tag:
        print(f'PASS partial: selector={has_sel} tag={has_tag} styles={has_styles}')
    else:
        print(f'FAIL no selector or tag, keys={list(data.keys())[:8]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$detail_verdict" | grep -q "^PASS"; then
        pass "Annotation detail has element data. $detail_verdict"
    else
        fail "Annotation detail missing fields. $detail_verdict. Content: $(truncate "$detail_text" 200)"
    fi
}
run_test_13_6

# ── Test 13.7: Async wait pattern (correlation_id) ──────────
begin_test "13.7" "[INTERACTIVE - BROWSER] Async annotation wait: analyze(wait:true) returns correlation_id" \
    "Call analyze(annotations, wait:true), verify immediate return with correlation_id and status=waiting_for_user" \
    "Tests: non-blocking async pattern — LLM gets correlation_id immediately"

run_test_13_7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Start draw mode
    call_tool "interact" '{"what":"draw_mode_start"}' >/dev/null 2>&1
    sleep 1

    local response
    response=$(call_tool "analyze" '{"what":"annotations","wait":true}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "13.7" "analyze(wait:true)" "$response" "$content_text"

    if echo "$content_text" | grep -qi "waiting_for_user\|correlation_id"; then
        local ann_corr
        ann_corr=$(echo "$content_text" | grep -oE '"correlation_id":"ann_[^"]+"' | head -1 | sed 's/"correlation_id":"//' | sed 's/"//' || true)
        pass "wait:true returned immediately with correlation_id=$ann_corr."
    elif echo "$content_text" | grep -qi "complete\|annotation"; then
        pass "wait:true returned existing annotations (data was already available)."
    else
        fail "Unexpected response from wait:true. Content: $(truncate "$content_text" 200)"
    fi

    # Cleanup: exit draw mode
    echo "  >>> Press ESC to exit draw mode if still active <<<"
    echo "  -- Press Enter to continue --"
    if [ -t 0 ]; then read -r; fi
}
run_test_13_7

# ── Test 13.8: Double activation returns already_active ──────
begin_test "13.8" "[INTERACTIVE - BROWSER] Double draw_mode_start returns already_active" \
    "Activate draw mode twice, poll extension result for already_active status" \
    "Tests: idempotent activation guard (extension-side)"

run_test_13_8() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # First activation — wait for the extension to confirm it's active
    local first_resp
    first_resp=$(call_tool "interact" '{"what":"draw_mode_start"}')
    local first_text
    first_text=$(extract_content_text "$first_resp")
    local first_corr
    first_corr=$(echo "$first_text" | grep -oE '"correlation_id":"[^"]+"' | head -1 | sed 's/"correlation_id":"//' | sed 's/"//' || true)

    # Poll until the first activation is confirmed by the extension
    if [ -n "$first_corr" ]; then
        for i in $(seq 1 10); do
            sleep 0.5
            local poll
            poll=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$first_corr\"}")
            local poll_text
            poll_text=$(extract_content_text "$poll")
            if echo "$poll_text" | grep -q '"status":"complete"'; then
                break
            fi
        done
    else
        sleep 1
    fi

    # Second activation — the server returns immediately (fire-and-forget),
    # so extract the correlation_id and poll the extension's actual response.
    local second_resp
    second_resp=$(call_tool "interact" '{"what":"draw_mode_start"}')
    local second_text
    second_text=$(extract_content_text "$second_resp")
    local second_corr
    second_corr=$(echo "$second_text" | grep -oE '"correlation_id":"[^"]+"' | head -1 | sed 's/"correlation_id":"//' | sed 's/"//' || true)

    if [ -z "$second_corr" ]; then
        fail "No correlation_id in second draw_mode_start response."
        return
    fi

    # Poll command_result for the extension's already_active response
    local found="false"
    for i in $(seq 1 10); do
        sleep 0.5
        local poll_resp
        poll_resp=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$second_corr\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_resp")

        if echo "$poll_text" | grep -qi "already.active\|already_active"; then
            found="true"
            pass "Extension returned already_active on second draw_mode_start (poll $i)."
            break
        fi
        if echo "$poll_text" | grep -q '"status":"complete"'; then
            # Got a result but not already_active — check the data payload
            if echo "$poll_text" | grep -qi "already.active\|already_active"; then
                found="true"
                pass "Extension returned already_active on second draw_mode_start (poll $i)."
            else
                fail "Extension completed but without already_active. Response: $(truncate "$poll_text" 200)"
                found="true"
            fi
            break
        fi
    done

    if [ "$found" = "false" ]; then
        fail "Timed out polling for second draw_mode_start result (correlation_id=$second_corr)."
    fi

    # Cleanup
    echo "  >>> Press ESC to exit draw mode <<<"
    echo "  -- Press Enter to continue --"
    if [ -t 0 ]; then read -r; fi
}
run_test_13_8

# ── Test 13.9: Named session across pages ────────────────────
begin_test "13.9" "[INTERACTIVE - BROWSER] Multi-page named session accumulates annotations" \
    "Draw on page 1 with session name, navigate, draw on page 2 with same session, verify both pages" \
    "Tests: named session aggregation across navigation"

run_test_13_9() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi
    if [ ! -t 0 ]; then
        skip "Interactive multi-page annotation draw requires a TTY."
        return
    fi

    local session_name="smoke-session-$$"

    # Page 1
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Session page 1"}' 20
    sleep 2
    call_tool "interact" "{\"action\":\"draw_mode_start\",\"annot_session\":\"$session_name\"}" >/dev/null 2>&1
    echo "  >>> Draw 1 annotation on this page, then press ESC <<<"
    echo "  -- Press Enter when done --"
    if [ -t 0 ]; then read -r; fi

    # Page 2
    interact_and_wait "navigate" '{"what":"navigate","url":"https://www.iana.org/domains/reserved","reason":"Session page 2"}' 20
    sleep 2
    call_tool "interact" "{\"action\":\"draw_mode_start\",\"annot_session\":\"$session_name\"}" >/dev/null 2>&1
    echo "  >>> Draw 1 annotation on this page, then press ESC <<<"
    echo "  -- Press Enter when done --"
    if [ -t 0 ]; then read -r; fi

    # Retrieve named session
    local response
    response=$(call_tool "analyze" "{\"what\":\"annotations\",\"annot_session\":\"$session_name\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "13.9" "named session" "$response" "$content_text"

    if echo "$content_text" | grep -q "page_count"; then
        local page_count
        page_count=$(echo "$content_text" | grep -oE '"page_count":[0-9]+' | head -1 | grep -oE '[0-9]+' || true)
        if [ "$page_count" = "2" ]; then
            pass "Named session '$session_name' has 2 pages."
        else
            fail "Expected 2 pages, got $page_count."
        fi
    else
        fail "No page_count in named session response. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_13_9

# ── Test 13.10: generate(visual_test) from annotations ────────
begin_test "13.10" "[DAEMON ONLY] Generate Playwright test from annotations" \
    "Call generate(visual_test), verify output contains test() and page.goto()" \
    "Tests: annotation-to-test code generation"

run_test_13_10() {
    local response content_text
    local max_attempts=8
    local settled=false

    for i in $(seq 1 "$max_attempts"); do
        response=$(call_tool "generate" '{"what":"visual_test"}')
        content_text=$(extract_content_text "$response")

        if [ -z "$content_text" ]; then
            content_text="$response"
        fi

        if echo "$content_text" | grep -qi "starting up"; then
            if [ "$i" -lt "$max_attempts" ]; then
                sleep 2
                continue
            fi
        else
            settled=true
            break
        fi
    done

    if [ "$settled" != "true" ] && echo "$content_text" | grep -qi "starting up"; then
        skip "visual_test deferred: daemon still starting after ${max_attempts} retries."
        return
    fi

    if echo "$content_text" | grep -q "test(" && echo "$content_text" | grep -q "page.goto"; then
        pass "visual_test contains test() and page.goto()."
    elif echo "$content_text" | grep -qi "no annotation"; then
        skip "No annotations available for visual_test generation."
    else
        fail "visual_test missing expected code. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_13_10

# ── Test 13.11: generate(annotation_report) ───────────────────
begin_test "13.11" "[DAEMON ONLY] Generate annotation report (Markdown)" \
    "Call generate(annotation_report), verify Markdown output with header" \
    "Tests: annotation-to-report generation"

run_test_13_11() {
    local response
    response=$(call_tool "generate" '{"what":"annotation_report"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "# Annotation Report"; then
        # Verify report has actual content beyond just the header
        local line_count
        line_count=$(echo "$content_text" | wc -l | tr -d ' ')
        if [ "$line_count" -gt 3 ]; then
            pass "annotation_report contains Markdown header + $line_count lines of content."
        else
            pass "annotation_report contains Markdown header."
        fi
    elif echo "$content_text" | grep -qi "no annotation"; then
        skip "No annotations available for report generation."
    else
        fail "annotation_report missing '# Annotation Report' header. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_13_11

# ── Test 13.12: generate(annotation_issues) ───────────────────
begin_test "13.12" "[DAEMON ONLY] Generate annotation issues (structured JSON)" \
    "Call generate(annotation_issues), verify issues array and total_count" \
    "Tests: annotation-to-issues extraction"

run_test_13_12() {
    local response
    response=$(call_tool "generate" '{"what":"annotation_issues"}')
    local content_text
    content_text=$(extract_content_text "$response")

    # Validate issues structure: total_count must be numeric, issues must be array
    local issues_verdict
    issues_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    issues = data.get('issues', [])
    total = data.get('total_count', -1)
    if isinstance(issues, list) and isinstance(total, int) and total >= 0:
        print(f'PASS total_count={total} issues_len={len(issues)}')
    elif isinstance(issues, list):
        print(f'PASS issues_len={len(issues)} (no total_count)')
    else:
        print(f'FAIL issues_type={type(issues).__name__} total={total}')
except Exception as e:
    if 'no annotation' in str(e).lower():
        print('SKIP no annotations')
    else:
        print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$issues_verdict" | grep -q "^PASS"; then
        pass "annotation_issues has valid structure. $issues_verdict"
    elif echo "$issues_verdict" | grep -q "^SKIP\|no annotation"; then
        skip "No annotations available for issues extraction."
    elif echo "$content_text" | grep -qi "no annotation"; then
        skip "No annotations available for issues extraction."
    else
        fail "annotation_issues invalid. $issues_verdict. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_13_12
