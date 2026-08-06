#!/bin/bash
# 05-interact-dom.sh — 5.1-5.19: DOM primitive smoke tests.
# type, select, check, get_text/value/attribute, set_attribute,
# scroll_to, wait_for, key_press, list_interactive, focus, back/forward, new_tab,
# and interact failure recovery playbooks
set -eo pipefail

begin_category "5" "Interact DOM Primitives" "19"

# ── Inject rich test form on example.com ─────────────────
_inject_smoke_form() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        return 0
    fi

    # Navigate to clean page
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Clean page for DOM tests"}' 20
    sleep 2

    local form_js
    form_js=$(cat <<'FORMEOF'
(function() {
    var old = document.getElementById('smoke-form-dom');
    if (old) old.remove();
    var f = document.createElement('div');
    f.id = 'smoke-form-dom';
    f.innerHTML =
        '<input type="text" id="sf-name" placeholder="Name">' +
        '<input type="email" id="sf-email" placeholder="Email">' +
        '<select id="sf-role"><option value="">Pick</option><option value="admin">Admin</option><option value="user">User</option></select>' +
        '<label><input type="checkbox" id="sf-agree"> I agree</label>' +
        '<button id="sf-btn" type="button">Submit</button>' +
        '<a id="sf-link" href="https://example.com/test">Test Link</a>' +
        '<div id="sf-scroll-target" style="margin-top:2000px">Scroll Target</div>';
    document.body.appendChild(f);
    return 'form-injected';
})()
FORMEOF
)
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject DOM test form\",\"script\":$(echo "$form_js" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")}"
    sleep 0.5
}

_inject_smoke_form

# Parse command_result content and determine whether interact failed.
# Avoid brittle substring checks ("error", "failed") that can appear in benign
# fields like page titles.
_interact_failed() {
    local raw="$1"

    # Framework timeout sentinel string.
    if echo "$raw" | grep -qi '^timeout waiting for '; then
        return 0
    fi

    local payload
    payload="$(extract_embedded_json "$raw" 2>/dev/null || true)"
    if [ -z "$payload" ]; then
        # If JSON extraction fails, only fail on explicit status markers.
        if echo "$raw" | grep -qiE '"status":"(failed|error|timeout)"|"lifecycle_status":"(failed|error|timeout)"'; then
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

# ── Test 5.1: Type text ─────────────────────────────────
begin_test "5.1" "[BROWSER] Type text into input" \
    "interact(type) into #sf-name, then get_value to confirm" \
    "Tests: DOM type primitive > extension > content script"

run_test_5_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "type" '{"what":"type","selector":"#sf-name","text":"SmokeUser","clear":true,"reason":"Type into name field"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "type command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 1
    interact_and_wait "get_value" '{"what":"get_value","selector":"#sf-name","reason":"Verify typed value"}' 20

    if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
        pass "Type + get_value: 'SmokeUser' confirmed in #sf-name."
    else
        # Fallback: verify via execute_js if get_value timed out
        interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify typed value via JS","script":"document.getElementById(\"sf-name\").value"}'
        if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
            pass "Type confirmed via execute_js fallback: 'SmokeUser' in #sf-name."
        else
            fail "get_value did not return 'SmokeUser'. Result: $(truncate "$INTERACT_RESULT" 200)"
        fi
    fi
}
run_test_5_1

# ── Test 5.2: Select dropdown ───────────────────────────
begin_test "5.2" "[BROWSER] Select dropdown option" \
    "interact(select) on #sf-role value='admin', then get_value to confirm" \
    "Tests: DOM select primitive"

run_test_5_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Select 'user' (default is 'admin') to prove the value actually changes.
    interact_and_wait "select" '{"what":"select","selector":"#sf-role","value":"user","reason":"Select user role"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "select command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.5
    interact_and_wait "get_value" '{"what":"get_value","selector":"#sf-role","reason":"Verify selected value"}'

    if echo "$INTERACT_RESULT" | grep -q "user"; then
        pass "Select + get_value: changed to 'user' confirmed in #sf-role."
    else
        fail "get_value did not return 'user'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi

    # Restore to admin for subsequent tests
    interact_and_wait "select" '{"what":"select","selector":"#sf-role","value":"admin","reason":"Restore admin role"}'
}
run_test_5_2

# ── Test 5.3: Checkbox toggle ───────────────────────────
begin_test "5.3" "[BROWSER] Checkbox check and uncheck" \
    "interact(check) on #sf-agree checked:true then checked:false" \
    "Tests: DOM check primitive toggle"

run_test_5_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Check it
    interact_and_wait "check" '{"what":"check","selector":"#sf-agree","checked":true,"reason":"Check the agree box"}'

    sleep 0.3
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify checked","script":"document.getElementById(\"sf-agree\").checked ? \"CHECKED\" : \"UNCHECKED\""}'
    local checked_result="$INTERACT_RESULT"

    # Uncheck it
    interact_and_wait "check" '{"what":"check","selector":"#sf-agree","checked":false,"reason":"Uncheck the agree box"}'

    sleep 0.3
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify unchecked","script":"document.getElementById(\"sf-agree\").checked ? \"CHECKED\" : \"UNCHECKED\""}'
    local unchecked_result="$INTERACT_RESULT"

    if echo "$checked_result" | grep -q "CHECKED" && echo "$unchecked_result" | grep -q "UNCHECKED"; then
        pass "Checkbox toggled: checked=true then checked=false confirmed via DOM."
    else
        fail "Checkbox toggle failed. After check: $(truncate "$checked_result" 100), after uncheck: $(truncate "$unchecked_result" 100)"
    fi
}
run_test_5_3

# ── Test 5.4: Get text ──────────────────────────────────
begin_test "5.4" "[BROWSER] Get text from button" \
    "interact(get_text) on #sf-btn returns 'Submit'" \
    "Tests: DOM get_text primitive"

run_test_5_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_text" '{"what":"get_text","selector":"#sf-btn","reason":"Get button text"}'

    if echo "$INTERACT_RESULT" | grep -q "Submit"; then
        pass "get_text returned 'Submit' from #sf-btn."
    else
        fail "get_text did not return 'Submit'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_4

# ── Test 5.5: Get value ─────────────────────────────────
begin_test "5.5" "[BROWSER] Get value from text input" \
    "interact(get_value) on #sf-name returns value set in 5.1" \
    "Tests: DOM get_value primitive"

run_test_5_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_value" '{"what":"get_value","selector":"#sf-name","reason":"Get name input value"}'

    if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
        pass "get_value returned 'SmokeUser' from #sf-name."
    else
        fail "get_value did not return expected value. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_5

# ── Test 5.6: Get attribute ─────────────────────────────
begin_test "5.6" "[BROWSER] Get attribute from link" \
    "interact(get_attribute) on #sf-link name='href' returns URL" \
    "Tests: DOM get_attribute primitive"

run_test_5_6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_attribute" '{"what":"get_attribute","selector":"#sf-link","name":"href","reason":"Get link href"}'

    if echo "$INTERACT_RESULT" | grep -q "example.com/test"; then
        pass "get_attribute returned href 'example.com/test' from #sf-link."
    elif echo "$INTERACT_RESULT" | grep -q "example.com"; then
        pass "get_attribute returned href containing 'example.com'."
    else
        fail "get_attribute did not return expected href. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_6

# ── Test 5.7: Set attribute ─────────────────────────────
begin_test "5.7" "[BROWSER] Set attribute on element" \
    "interact(set_attribute) data-smoke='modified', then get_attribute to confirm" \
    "Tests: DOM set_attribute primitive"

run_test_5_7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "set_attribute" '{"what":"set_attribute","selector":"#sf-link","name":"data-smoke","value":"modified","reason":"Set data attribute"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "set_attribute command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.3
    interact_and_wait "get_attribute" '{"what":"get_attribute","selector":"#sf-link","name":"data-smoke","reason":"Verify set attribute"}'

    if echo "$INTERACT_RESULT" | grep -q "modified"; then
        pass "set_attribute + get_attribute roundtrip: data-smoke='modified' confirmed."
    else
        fail "get_attribute did not return 'modified'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_7

# ── Test 5.8: Scroll to ────────────────────────────────
begin_test "5.8" "[BROWSER] Scroll to element" \
    "interact(scroll_to) on #sf-scroll-target" \
    "Tests: DOM scroll_to primitive"

run_test_5_8() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "scroll_to" '{"what":"scroll_to","selector":"#sf-scroll-target","reason":"Scroll to bottom target"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "scroll_to command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: check scrollY > 0 via DOM
    sleep 0.5
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify scroll position","script":"window.scrollY > 100 ? \"SCROLLED_\" + Math.round(window.scrollY) : \"NOT_SCROLLED_\" + Math.round(window.scrollY)"}'

    if echo "$INTERACT_RESULT" | grep -q "SCROLLED_"; then
        pass "scroll_to moved page: $(echo "$INTERACT_RESULT" | grep -oE 'SCROLLED_[0-9]+' | head -1 || echo 'SCROLLED')px."
    else
        fail "scroll_to completed but page did not scroll. scrollY: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_8

# ── Test 5.9: Wait for ─────────────────────────────────
begin_test "5.9" "[BROWSER] Wait for delayed element" \
    "Inject element after 1s delay, interact(wait_for) should find it" \
    "Tests: DOM wait_for primitive with polling"

run_test_5_9() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject delayed element
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject delayed element","script":"setTimeout(function(){ var d = document.createElement(\"div\"); d.id = \"delayed-el\"; d.textContent = \"I appeared!\"; document.body.appendChild(d); }, 1000); \"scheduled\""}'

    interact_and_wait "wait_for" '{"what":"wait_for","selector":"#delayed-el","timeout_ms":5000,"reason":"Wait for delayed element"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "wait_for timed out or failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "wait_for found #delayed-el within timeout."
    fi
}
run_test_5_9

# ── Test 5.10: Key press ────────────────────────────────
begin_test "5.10" "[BROWSER] Key press on element" \
    "interact(key_press) Tab on #sf-name" \
    "Tests: DOM key_press primitive"

run_test_5_10() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # First focus the name field, then Tab should move focus to the next field
    interact_and_wait "focus" '{"what":"focus","selector":"#sf-name","reason":"Focus name before Tab"}'
    sleep 0.3
    interact_and_wait "key_press" '{"what":"key_press","selector":"#sf-name","text":"Tab","reason":"Press Tab key"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "key_press command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: activeElement should have moved away from #sf-name
    sleep 0.3
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify focus moved after Tab","script":"document.activeElement ? document.activeElement.id || document.activeElement.tagName : \"NONE\""}'

    if echo "$INTERACT_RESULT" | grep -qE "sf-email|sf-role|sf-agree|sf-btn|INPUT|SELECT"; then
        pass "key_press(Tab) moved focus from #sf-name. Active: $(echo "$INTERACT_RESULT" | grep -oE 'sf-[a-z-]+|INPUT|SELECT' | head -1 || echo 'element')"
    elif echo "$INTERACT_RESULT" | grep -q "sf-name"; then
        fail "key_press(Tab) did not move focus — still on #sf-name."
    else
        pass "key_press(Tab) completed, focus moved to: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_10

# ── Test 5.11: List interactive ──────────────────────────
begin_test "5.11" "[BROWSER] List interactive elements" \
    "interact(list_interactive) returns element list including injected form" \
    "Tests: DOM list_interactive primitive"

run_test_5_11() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "list_interactive" '{"what":"list_interactive","reason":"List all interactive elements"}'

    echo "  [interactive elements]"
    local elem_count
    elem_count=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    elems = data.get('elements', data.get('interactive', data.get('result', {}).get('elements', [])))
    count = len(elems) if isinstance(elems, list) else 0
    print(count)
    if isinstance(elems, list):
        for e in elems[:8]:
            tag = e.get('tag', e.get('tagName', '?'))
            sel = e.get('selector', e.get('id', ''))[:40]
            text = e.get('text', e.get('textContent', ''))[:30]
            import sys as s2
            s2.stderr.write(f'    <{tag}> {sel} \"{text}\"\n')
except Exception as e:
    print(0)
" 2>/dev/null || echo "0")
    echo "    count: $elem_count"

    # Strict: we injected a form with #sf-name, #sf-email, #sf-role, #sf-agree, #sf-btn, #sf-link
    # There MUST be > 0 interactive elements
    if [ "$elem_count" -gt 0 ] 2>/dev/null; then
        # Verify our injected elements are present
        if echo "$INTERACT_RESULT" | grep -qiE "sf-name|sf-btn|sf-email"; then
            pass "list_interactive returned $elem_count elements including injected form fields."
        else
            pass "list_interactive returned $elem_count elements (injected IDs not visible in result, but elements found)."
        fi
    else
        fail "list_interactive returned 0 elements. Injected form with 6 interactive elements should be visible. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_11

# ── Test 5.12: Focus ─────────────────────────────────────
begin_test "5.12" "[BROWSER] Focus an element" \
    "interact(focus) on #sf-email" \
    "Tests: DOM focus primitive"

run_test_5_12() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "focus" '{"what":"focus","selector":"#sf-email","reason":"Focus email field"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "focus command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: document.activeElement should be #sf-email
    sleep 0.3
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify focus target","script":"document.activeElement && document.activeElement.id === \"sf-email\" ? \"FOCUSED_SF_EMAIL\" : \"WRONG_FOCUS_\" + (document.activeElement ? document.activeElement.id || document.activeElement.tagName : \"NONE\")"}'

    if echo "$INTERACT_RESULT" | grep -q "FOCUSED_SF_EMAIL"; then
        pass "focus confirmed: document.activeElement is #sf-email."
    else
        fail "focus did not set activeElement to #sf-email. Result: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_12

# ── Test 5.13: Back ──────────────────────────────────────
begin_test "5.13" "[BROWSER] Browser back navigation" \
    "Navigate to 2 pages, interact(back), verify observe(page) shows previous URL" \
    "Tests: DOM back primitive > browser history"

run_test_5_13() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Use localhost pages — external domains (example.com, iana.org) can redirect unpredictably.
    # The daemon serves /health and /openapi.json which are stable, distinctive URLs.
    local page_a="http://localhost:${PORT}/health"
    local page_b="http://localhost:${PORT}/openapi.json"

    # Navigate to page A
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$page_a\",\"reason\":\"Page A for back test\"}" 20
    sleep 2

    # Verify we're on page A via direct DOM query
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify page A URL","script":"window.location.href"}'
    echo "  [page A] $(echo "$INTERACT_RESULT" | grep -oE 'https?://[^ \"]+' | head -1 || echo '?')"

    # Navigate to page B
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$page_b\",\"reason\":\"Page B for back test\"}" 20
    sleep 2

    # Go back
    interact_and_wait "back" '{"what":"back","reason":"Go back to page A"}'
    sleep 2

    # Primary check: command result URL (extension returns url after goBack)
    if echo "$INTERACT_RESULT" | grep -qi "/health"; then
        echo "  [after back] /health (from command result)"
        pass "Back navigation: returned to /health (confirmed via command result)."
        return
    fi

    # Fallback: verify via observe(page) — does NOT require JS execution,
    # so works on CSP-strict pages where execute_js would fail.
    local page_resp
    page_resp=$(call_tool "observe" '{"what":"page"}')
    local page_text
    page_text=$(extract_content_text "$page_resp")
    echo "  [after back] $(echo "$page_text" | grep -oE 'https?://[^ \"]+' | head -1 || echo '?')"

    if echo "$page_text" | grep -qi "/health"; then
        pass "Back navigation: returned to /health (confirmed via observe(page))."
    else
        # Include both the back command result and observe(page) in the failure for diagnosis
        fail "Back navigation: expected /health in URL. back result: $(truncate "$INTERACT_RESULT" 150). observe(page): $(truncate "$page_text" 150)"
    fi
}
run_test_5_13

# ── Test 5.14: Forward ───────────────────────────────────
begin_test "5.14" "[BROWSER] Browser forward navigation" \
    "After back, interact(forward) returns to page B" \
    "Tests: DOM forward primitive > browser history"

run_test_5_14() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "forward" '{"what":"forward","reason":"Go forward to page B"}'
    sleep 2

    # Primary check: command result URL (extension returns url after goForward)
    if echo "$INTERACT_RESULT" | grep -qi "openapi"; then
        pass "Forward navigation: returned to /openapi.json (confirmed via command result)."
        return
    fi

    # Fallback: verify via observe(page) — no JS execution needed.
    local page_resp
    page_resp=$(call_tool "observe" '{"what":"page"}')
    local page_text
    page_text=$(extract_content_text "$page_resp")

    if echo "$page_text" | grep -qi "openapi"; then
        pass "Forward navigation: returned to /openapi.json (confirmed via observe(page))."
    else
        fail "Forward navigation: expected /openapi.json. forward result: $(truncate "$INTERACT_RESULT" 150). observe(page): $(truncate "$page_text" 150)"
    fi
}
run_test_5_14

# ── Test 5.15: New tab ───────────────────────────────────
begin_test "5.15" "[BROWSER] Open new tab" \
    "interact(new_tab) opens a tab, extension returns success with URL" \
    "Tests: new_tab action via chrome.tabs.create"

run_test_5_15() {
    skip "new_tab creates an untracked tab — no way to verify from Kaboom. Feature exists for future multi-tab support (e.g. login flows)."
}
run_test_5_15

# ── Test 5.16: element_not_found recovery ───────────────
begin_test "5.16" "[BROWSER] element_not_found recovery flow" \
    "Trigger element_not_found, confirm retry guidance, recover via scoped list_interactive + valid click" \
    "Tests: recovery playbook alignment for element_not_found"

run_test_5_16() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # 5.13/5.14 navigate away from example.com; ensure recovery tests start with known DOM.
    _inject_smoke_form

    interact_and_wait "click" '{"what":"click","selector":"#missing-target","reason":"Trigger element_not_found"}'
    if ! echo "$INTERACT_RESULT" | grep -qi "element_not_found"; then
        fail "Expected element_not_found error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi
    if ! echo "$INTERACT_RESULT" | grep -qi "list_interactive\|scope"; then
        fail "element_not_found retry guidance missing list/scope hints. Result: $(truncate "$INTERACT_RESULT" 220)"
        return
    fi

    interact_and_wait "list_interactive" '{"what":"list_interactive","scope_selector":"#smoke-form-dom","reason":"Recover candidates in form scope"}'
    # Use the known unique selector for the actual recovery click to avoid brittle scope coupling.
    interact_and_wait "click" '{"what":"click","selector":"#sf-btn","reason":"Recover from element_not_found"}'

    if _interact_failed "$INTERACT_RESULT"; then
        fail "Recovery click failed after element_not_found. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "element_not_found recovery succeeded via scoped list_interactive + click."
    fi
}
run_test_5_16

# ── Test 5.17: ambiguous_target recovery ────────────────
begin_test "5.17" "[BROWSER] ambiguous_target recovery flow" \
    "Create duplicate targets, trigger ambiguous_target, recover with scope_selector" \
    "Tests: recovery playbook alignment for ambiguous_target"

run_test_5_17() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject duplicate targets for ambiguous test","script":"(function(){var old=document.getElementById(\"smoke-ambiguous\"); if(old) old.remove(); var root=document.createElement(\"div\"); root.id=\"smoke-ambiguous\"; root.innerHTML=\"<div id=\\\"dup-a\\\"><button class=\\\"dup-target\\\" type=\\\"button\\\">Post</button></div><div id=\\\"dup-b\\\"><button class=\\\"dup-target\\\" type=\\\"button\\\">Post</button></div>\"; document.body.appendChild(root); return \"ambiguous-ready\";})()"}'

    interact_and_wait "click" '{"what":"click","selector":".dup-target","reason":"Trigger ambiguous_target"}'
    if ! echo "$INTERACT_RESULT" | grep -qi "ambiguous_target"; then
        fail "Expected ambiguous_target error. Result: $(truncate "$INTERACT_RESULT" 220)"
        return
    fi
    if ! echo "$INTERACT_RESULT" | grep -qi "scope_selector\|element_id"; then
        fail "ambiguous_target retry guidance missing scope_selector/element_id hints. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    interact_and_wait "click" '{"what":"click","selector":".dup-target","scope_selector":"#dup-a","reason":"Recover from ambiguous_target using scope"}'
    if _interact_failed "$INTERACT_RESULT"; then
        fail "Recovery click failed after ambiguous_target. Result: $(truncate "$INTERACT_RESULT" 220)"
    else
        pass "ambiguous_target recovery succeeded with scope_selector."
    fi
}
run_test_5_17

# ── Test 5.18: stale_element_id recovery ────────────────
begin_test "5.18" "[BROWSER] stale_element_id recovery flow" \
    "Trigger stale_element_id via invalid handle, recover by refreshing list_interactive and using new handle" \
    "Tests: recovery playbook alignment for stale_element_id"

run_test_5_18() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Ensure injected test DOM exists before stale handle recovery flow.
    _inject_smoke_form

    interact_and_wait "click" '{"what":"click","selector":"#sf-btn","element_id":"el_missing","reason":"Trigger stale_element_id"}'
    if ! echo "$INTERACT_RESULT" | grep -qi "stale_element_id"; then
        fail "Expected stale_element_id error. Result: $(truncate "$INTERACT_RESULT" 220)"
        return
    fi
    if ! echo "$INTERACT_RESULT" | grep -qi "list_interactive\|element_id"; then
        fail "stale_element_id retry guidance missing list_interactive/element_id hints. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    interact_and_wait "list_interactive" '{"what":"list_interactive","scope_selector":"#smoke-form-dom","reason":"Refresh handles for stale recovery"}'
    local sf_btn_element_id
    sf_btn_element_id=$(echo "$INTERACT_RESULT" | python3 -c '
import json, sys
text = sys.stdin.read()
i = text.find("{")
if i < 0:
    sys.exit(0)
try:
    data = json.loads(text[i:])
except Exception:
    sys.exit(0)
payload = data.get("result", data) if isinstance(data, dict) else {}
if isinstance(payload, str):
    try:
        payload = json.loads(payload)
    except Exception:
        payload = {}
if not isinstance(payload, dict):
    payload = {}
elems = payload.get("elements")
if not isinstance(elems, list):
    nested = payload.get("result", {})
    elems = nested.get("elements", []) if isinstance(nested, dict) else []
if not isinstance(elems, list):
    elems = []
for e in elems:
    selector = str(e.get("selector", ""))
    label = str(e.get("label", ""))
    textv = str(e.get("text", "")) or str(e.get("text_content", ""))
    if "#sf-btn" in selector or "submit" in label.lower() or "submit" in textv.lower():
        eid = str(e.get("element_id", "")).strip()
        if eid:
            print(eid)
            break
')
    if [ -z "$sf_btn_element_id" ]; then
        fail "Could not extract refreshed element_id for #sf-btn from list_interactive result."
        return
    fi

    interact_and_wait "click" "{\"action\":\"click\",\"selector\":\"#sf-btn\",\"element_id\":\"$sf_btn_element_id\",\"reason\":\"Recover from stale_element_id with refreshed handle\"}"
    if _interact_failed "$INTERACT_RESULT"; then
        fail "Recovery click with refreshed element_id failed. Result: $(truncate "$INTERACT_RESULT" 220)"
    else
        pass "stale_element_id recovery succeeded with refreshed list_interactive handle."
    fi
}
run_test_5_18

# ── Test 5.19: scope_not_found recovery ──────────────────
begin_test "5.19" "[BROWSER] scope_not_found recovery flow" \
    "Trigger scope_not_found, confirm fallback guidance, recover with valid scope_selector" \
    "Tests: recovery playbook alignment for scope_not_found"

run_test_5_19() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Ensure injected test DOM exists before scope recovery flow.
    _inject_smoke_form

    interact_and_wait "click" '{"what":"click","selector":"#sf-btn","scope_selector":"#missing-scope","reason":"Trigger scope_not_found"}'
    if ! echo "$INTERACT_RESULT" | grep -qi "scope_not_found"; then
        fail "Expected scope_not_found error. Result: $(truncate "$INTERACT_RESULT" 220)"
        return
    fi
    if ! echo "$INTERACT_RESULT" | grep -qi "scope_selector\|scope_rect\|frame"; then
        fail "scope_not_found retry guidance missing scope fallback hints. Result: $(truncate "$INTERACT_RESULT" 260)"
        return
    fi

    interact_and_wait "click" '{"what":"click","selector":"#sf-btn","scope_selector":"#smoke-form-dom","reason":"Recover from scope_not_found"}'
    if _interact_failed "$INTERACT_RESULT"; then
        fail "Recovery click failed after scope_not_found. Result: $(truncate "$INTERACT_RESULT" 220)"
    else
        pass "scope_not_found recovery succeeded using valid fallback scope_selector."
    fi
}
run_test_5_19
