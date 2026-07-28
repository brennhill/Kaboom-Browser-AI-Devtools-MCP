#!/bin/bash
# 02-core-telemetry.sh — 2.1-2.7: Console logs/errors, clicks, forms,
# error clusters, DOM query, full form lifecycle.
# Seeds error buffer used by later modules (3.4 error_bundles).
set -eo pipefail

begin_category "2" "Core Telemetry" "7"

# ── Test 2.1: Trigger console log + error ────────────────
begin_test "2.1" "[BROWSER] Trigger console log and error via JS" \
    "Execute JS to console.log and console.error with markers, verify in observe" \
    "Tests: inject.js console monkey-patch > extension > daemon buffer > MCP observe"

run_test_2_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Precondition: ensure we are on a CSP-safe page where execute_js can run.
    # Without this, a restricted tracked tab can cause false "logs missing" failures.
    local page_before page_before_text
    page_before=$(call_tool "observe" '{"what":"page"}')
    page_before_text=$(extract_content_text "$page_before")
    if ! echo "$page_before_text" | grep -qi "example.com"; then
        interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Core telemetry 2.1 precondition: CSP-safe page"}'
        sleep 1
    fi

    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger console.log\",\"script\":\"console.log('${SMOKE_MARKER}_LOG')\"}"
    local log_exec_result
    log_exec_result="$INTERACT_RESULT"
    if ! echo "$log_exec_result" | grep -q '"success":true'; then
        fail "execute_js(console.log) failed before log assertion. Result: $(truncate "$log_exec_result" 220)"
        return
    fi

    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger console.error\",\"script\":\"console.error('${SMOKE_MARKER}_ERROR')\"}"
    local err_exec_result
    err_exec_result="$INTERACT_RESULT"
    if ! echo "$err_exec_result" | grep -q '"success":true'; then
        fail "execute_js(console.error) failed before error assertion. Result: $(truncate "$err_exec_result" 220)"
        return
    fi

    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger thrown error\",\"script\":\"try { throw new Error('${SMOKE_MARKER}_THROWN') } catch(e) { console.error(e.message, e.stack) }\"}"
    local thrown_exec_result
    thrown_exec_result="$INTERACT_RESULT"
    if ! echo "$thrown_exec_result" | grep -q '"success":true'; then
        fail "execute_js(thrown error) failed before assertion. Result: $(truncate "$thrown_exec_result" 220)"
        return
    fi

    sleep 2

    local log_response
    log_response=$(call_tool "observe" '{"what":"logs","scope":"all","limit":200}')
    local log_text
    log_text=$(extract_content_text "$log_response")

    local log_ok=false
    if echo "$log_text" | grep -q "${SMOKE_MARKER}_LOG"; then
        log_ok=true
    fi

    local err_response
    err_response=$(call_tool "observe" '{"what":"errors","scope":"all","limit":200}')
    local err_text
    err_text=$(extract_content_text "$err_response")

    local err_ok=false
    if echo "$err_text" | grep -q "${SMOKE_MARKER}"; then
        err_ok=true
    fi

    if [ "$log_ok" = "true" ] && [ "$err_ok" = "true" ]; then
        pass "Log marker '${SMOKE_MARKER}_LOG' in observe(logs) AND error marker in observe(errors)."
    elif [ "$log_ok" = "true" ]; then
        fail "Log marker found but error marker '${SMOKE_MARKER}' missing from observe(errors). Errors: $(truncate "$err_text" 200)"
    else
        fail "Log marker '${SMOKE_MARKER}_LOG' NOT found in observe(logs). Console monkey-patch may be broken. Logs: $(truncate "$log_text" 200)"
    fi
}
run_test_2_1

# ── Test 2.2: Click a button ─────────────────────────────
begin_test "2.2" "[BROWSER] Click a button via JS" \
    "Inject a button into the page, click it, verify in observe(actions)" \
    "Tests: user action capture > extension > daemon > MCP observe"

run_test_2_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local js="var btn = document.createElement('button'); btn.id = 'smoke-btn-${SMOKE_MARKER}'; btn.textContent = 'Smoke Test'; document.body.appendChild(btn); btn.click(); 'clicked'"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject + click button\",\"script\":\"$js\"}"

    sleep 1

    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "smoke-btn-${SMOKE_MARKER}\|click.*smoke-btn"; then
        pass "Click action for 'smoke-btn-${SMOKE_MARKER}' captured in observe(actions)."
    elif echo "$content_text" | grep -qi "click"; then
        pass "Click action captured (button ID not in response, but click event present)."
    else
        fail "No click action found. Action capture may be broken. Actions: $(truncate "$content_text" 200)"
    fi
}
run_test_2_2

# ── Test 2.3: Fill a form input ──────────────────────────
begin_test "2.3" "[BROWSER] Fill a form input via JS" \
    "Inject an input, set its value and dispatch input event, verify in observe(actions)" \
    "Tests: form input tracking > extension > daemon > MCP observe"

run_test_2_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local js="var inp = document.createElement('input'); inp.id = 'smoke-input-${SMOKE_MARKER}'; inp.type = 'text'; document.body.appendChild(inp); inp.focus(); inp.value = 'smoke-test-value'; inp.dispatchEvent(new Event('input', {bubbles:true})); 'filled'"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject + fill input\",\"script\":\"$js\"}"

    sleep 1

    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "smoke-input-${SMOKE_MARKER}"; then
        pass "Input action for 'smoke-input-${SMOKE_MARKER}' captured in observe(actions)."
    elif echo "$content_text" | grep -qi "input\|change"; then
        pass "Input/change action captured (element ID not in response, but form event present)."
    else
        fail "No input/change action found. Form tracking may be broken. Actions: $(truncate "$content_text" 200)"
    fi
}
run_test_2_3

# ── Test 2.4: Highlight an element ───────────────────────
begin_test "2.4" "[BROWSER] Highlight an element via interact(highlight)" \
    "Use interact(highlight) to highlight the body element, verify command completes" \
    "Tests: highlight pipeline: MCP > daemon > extension > inject overlay"

run_test_2_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Use 40 polls (20s) — extension sync can take a few seconds after daemon startup
    interact_and_wait "highlight" '{"what":"highlight","selector":"body","duration_ms":2000,"reason":"Highlight page body"}' 40

    if echo "$INTERACT_RESULT" | grep -qi "complete\|success\|highlighted"; then
        pass "Highlight command completed successfully."
    elif echo "$INTERACT_RESULT" | grep -qi "timeout"; then
        fail "Highlight command timed out. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        fail "Highlight command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_2_4

# ── Test 2.5: Error clusters ─────────────────────────────
begin_test "2.5" "[BROWSER] Error clusters aggregate triggered errors" \
    "After 2.1 triggered multiple errors, verify analyze(error_clusters) groups them" \
    "Tests: error dedup and clustering — critical for noise reduction in real apps"

run_test_2_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "analyze" '{"what":"error_clusters"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from analyze(error_clusters)."
        return
    fi

    echo "  [error clusters]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    clusters = data.get('clusters', [])
    print(f'    total clusters: {len(clusters)}')
    for c in clusters[:3]:
        msg = c.get('message', c.get('pattern', ''))[:80]
        count = c.get('count', c.get('occurrences', 1))
        print(f'    [{count}x] {msg}')
except: pass
" 2>/dev/null || true

    # Validate clusters have actual structure: array with count > 0
    local cluster_verdict
    cluster_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    clusters = data.get('clusters', [])
    if not isinstance(clusters, list) or len(clusters) == 0:
        print('FAIL no clusters array or empty')
    else:
        # Verify at least one cluster has a message and count
        valid = [c for c in clusters if c.get('message') or c.get('pattern')]
        if len(valid) > 0:
            print(f'PASS clusters={len(clusters)} with_message={len(valid)}')
        else:
            print(f'FAIL clusters={len(clusters)} but none have message/pattern fields')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$cluster_verdict" | grep -q "^PASS"; then
        pass "Error clusters returned structured data. $cluster_verdict"
    else
        fail "analyze(error_clusters) invalid. $cluster_verdict. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_2_5

# ── Test 2.6: DOM query ─────────────────────────────────
begin_test "2.6" "[BROWSER] DOM query parses page structure" \
    "Use analyze(dom) to query elements on the page, verify DOM data returned" \
    "Tests: page structure analysis"

run_test_2_6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # DOM query helper — issues analyze(dom) and polls for async result.
    # Returns result in dom_text. Retries once if the first attempt fails
    # (content script may not be loaded yet on the target page).
    _dom_query_attempt() {
        local dom_response
        dom_response=$(call_tool "analyze" '{"what":"dom","selector":"h1, a, button, input"}')
        dom_text=$(extract_content_text "$dom_response")

        local corr_id
        corr_id=$(echo "$dom_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

        if [ -n "$corr_id" ]; then
            for i in $(seq 1 15); do
                sleep 0.5
                local poll_response
                poll_response=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
                local poll_text
                poll_text=$(extract_content_text "$poll_response")
                if echo "$poll_text" | grep -q '"status":"complete"'; then
                    dom_text="$poll_text"
                    return 0
                fi
                if echo "$poll_text" | grep -q '"status":"failed"'; then
                    dom_text="$poll_text"
                    return 1
                fi
            done
        fi
    }

    local dom_text=""
    if ! _dom_query_attempt; then
        # First attempt failed — content script may not be loaded yet. Wait and retry.
        echo "  [DOM query attempt 1 failed, retrying after 3s...]"
        sleep 3
        _dom_query_attempt || true
    fi

    echo "  [DOM query: h1, a, button, input]"
    echo "$dom_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    # Unwrap command_result wrapper: actual DOM data is under 'result'
    if 'result' in data and isinstance(data['result'], dict):
        data = data['result']
    # Extension returns 'matches' (not 'elements')
    elements = data.get('matches', data.get('elements', data.get('results', [])))
    if isinstance(elements, list):
        print(f'    found: {len(elements)} element(s)')
        for e in elements[:5]:
            tag = e.get('tag', e.get('tagName', '?'))
            text = e.get('text', e.get('textContent', ''))[:50]
            print(f'    <{tag}> {text}')
    else:
        print(f'    matchCount: {data.get(\"matchCount\", \"?\")}')
        print(f'    response keys: {list(data.keys())[:5]}')
except Exception as ex:
    print(f'    (parse note: {ex})')
" 2>/dev/null || true

    if [ -z "$dom_text" ]; then
        fail "Empty response from analyze(dom)."
        return
    fi

    # Validate DOM query returned actual elements
    local dom_verdict
    dom_verdict=$(echo "$dom_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    # Unwrap command_result wrapper
    if 'result' in data and isinstance(data['result'], dict):
        data = data['result']
    # Extension returns 'matches' array with serialized DOM elements
    elements = data.get('matches', data.get('elements', data.get('results', [])))
    if isinstance(elements, list) and len(elements) > 0:
        has_tag = any(e.get('tag') or e.get('tagName') for e in elements)
        if has_tag:
            print(f'PASS elements={len(elements)} with_tags=true')
        else:
            # Matches may use different field names — check matchCount
            mc = data.get('matchCount', 0)
            if mc > 0:
                print(f'PASS matchCount={mc} elements={len(elements)}')
            else:
                print(f'FAIL elements={len(elements)} but none have tag field')
    elif isinstance(elements, list):
        print(f'FAIL elements array is empty, keys={list(data.keys())[:8]}')
    else:
        print(f'FAIL no elements/matches array, keys={list(data.keys())[:8]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$dom_verdict" | grep -q "^PASS"; then
        pass "DOM query returned page elements. $dom_verdict"
    elif echo "$dom_text" | grep -qi "Failed to execute DOM query\|dom_query_failed"; then
        fail "DOM query execution failed — content script may not be loaded on the target page. Ensure the extension is connected and the page is not a chrome:// or restricted URL. Raw: $(truncate "$dom_text" 200)"
    elif echo "$dom_text" | grep -qi "Receiving end does not exist"; then
        fail "DOM query failed — content script not reachable on the tab. Extension may need to reload. Raw: $(truncate "$dom_text" 200)"
    else
        fail "DOM query invalid. $dom_verdict. Content: $(truncate "$dom_text" 200)"
    fi
}
run_test_2_6

# ── Test 2.7: Full form lifecycle ───────────────────────
begin_test "2.7" "[BROWSER] Full form: create, fill multiple fields, submit" \
    "Inject a complete form with multiple inputs, fill each, submit, verify all actions captured" \
    "Tests: full form lifecycle — creation, multi-field fill, and submit event capture"

run_test_2_7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "configure" '{"what":"clear","buffer":"all"}' >/dev/null
    sleep 0.3

    local form_js="(function() {
        var f = document.createElement('form');
        f.id = 'smoke-form';
        f.innerHTML = '<input name=\"username\" type=\"text\" id=\"sf-user\">' +
            '<input name=\"email\" type=\"email\" id=\"sf-email\">' +
            '<input name=\"password\" type=\"password\" id=\"sf-pass\">' +
            '<select name=\"role\" id=\"sf-role\"><option value=\"user\">User</option><option value=\"admin\">Admin</option></select>' +
            '<button type=\"submit\" id=\"sf-submit\">Submit</button>';
        f.onsubmit = function(e) { e.preventDefault(); window.__SMOKE_FORM_SUBMITTED__ = true; };
        document.body.appendChild(f);
        return 'form-injected';
    })()"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Create form\",\"script\":$(echo "$form_js" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")}"

    sleep 0.5

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill username","script":"var el = document.getElementById(\"sf-user\"); el.focus(); el.value = \"smokeuser\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-user\""}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill email","script":"var el = document.getElementById(\"sf-email\"); el.focus(); el.value = \"smoke@test.com\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-email\""}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill password","script":"var el = document.getElementById(\"sf-pass\"); el.focus(); el.value = \"s3cure!\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-pass\""}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Select role","script":"var el = document.getElementById(\"sf-role\"); el.value = \"admin\"; el.dispatchEvent(new Event(\"change\", {bubbles:true})); \"selected-role\""}'

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Submit form","script":"document.getElementById(\"sf-submit\").click(); \"submitted\""}'

    sleep 1

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify form submitted","script":"window.__SMOKE_FORM_SUBMITTED__ === true ? \"submit-confirmed\" : \"no-submit\""}'

    local submit_confirmed=false
    if echo "$INTERACT_RESULT" | grep -q "submit-confirmed"; then
        submit_confirmed=true
    fi

    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [form actions captured]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    entries = data.get('entries', [])
    form_actions = [e for e in entries if any(k in str(e).lower() for k in ['input', 'change', 'click', 'submit', 'focus'])]
    print(f'    total actions: {len(entries)}, form-related: {len(form_actions)}')
    for a in form_actions[:6]:
        atype = a.get('type', a.get('action', '?'))
        target = a.get('target', a.get('selector', ''))[:50]
        print(f'    {atype}: {target}')
except: pass
" 2>/dev/null || true

    local has_input has_click
    has_input=$(echo "$content_text" | grep -ci "input\|change\|focus" || true)
    has_click=$(echo "$content_text" | grep -ci "click\|submit" || true)

    if [ "$submit_confirmed" != "true" ]; then
        fail "Form submission not confirmed. Form lifecycle test failed. Actions: $(truncate "$content_text" 200)"
    elif [ "${has_input:-0}" -eq 0 ]; then
        fail "Form submitted but no input/change/focus actions captured. Actions: $(truncate "$content_text" 200)"
    elif [ "${has_click:-0}" -eq 0 ]; then
        fail "Form submitted but no click/submit actions captured. Actions: $(truncate "$content_text" 200)"
    else
        pass "Full form lifecycle: submitted, $has_input input events + $has_click click/submit events captured."
    fi
}
run_test_2_7
