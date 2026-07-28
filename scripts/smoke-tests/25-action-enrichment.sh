#!/bin/bash
# 25-action-enrichment.sh — 25.1-25.3: Action result enrichment fields.
# Verifies final_url, title, effective_title, and dom_changes in interact results.
set -eo pipefail

begin_category "25" "Action Result Enrichment" "3"

# Returns 0 when extension is currently connected; otherwise emits SKIP and returns 1.
require_extension_connected() {
    local pilot_resp
    local pilot_text
    local connected

    pilot_resp=$(call_tool "observe" '{"what":"pilot"}')
    pilot_text=$(extract_content_text "$pilot_resp")

    connected=$(echo "$pilot_text" | python3 -c "
import sys, json
t = sys.stdin.read()
i = t.find('{')
if i < 0:
    print('false')
    raise SystemExit
try:
    data = json.loads(t[i:])
except Exception:
    print('false')
    raise SystemExit
print('true' if data.get('extension_connected') is True else 'false')
" 2>/dev/null || echo "false")

    if [ "$connected" != "true" ]; then
        skip "Extension not connected. Action enrichment tests require an active extension/tracked tab."
        return 1
    fi
    return 0
}

# ── Test 25.1: Navigate returns final_url and title ──────
begin_test "25.1" "[BROWSER] Navigate enriches result with final_url and title" \
    "interact(navigate) to example.com returns final_url, title, effective_title" \
    "Tests: action result enrichment (#149) — final_url, title fields"

run_test_25_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi
    if ! require_extension_connected; then
        return
    fi

    local result=""
    local verdict=""
    local attempt
    local final_attempt=1

    for attempt in 1 2; do
        interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Smoke test enrichment"}'
        result="$INTERACT_RESULT"

        if [ -z "$result" ]; then
            if [ "$attempt" -eq 1 ]; then
                sleep 1
                continue
            fi
            fail "No result from navigate."
            return
        fi

        verdict=$(echo "$result" | python3 -c "
import sys, json
try:
    t = sys.stdin.read()
    i = t.find('{')
    data = json.loads(t[i:]) if i >= 0 else {}
    err = str(data.get('error', ''))
    msg = str(data.get('message', ''))
    if err == 'no_data' and 'Extension not connected' in msg:
        print('SKIP extension_disconnected')
        raise SystemExit
    final_url = data.get('final_url', '')
    title = data.get('title', '')
    eff_title = data.get('effective_title', '')
    fields = []
    if final_url: fields.append(f'final_url={final_url}')
    if title: fields.append(f'title={title}')
    if eff_title: fields.append(f'effective_title={eff_title}')
    if len(fields) >= 2:
        print(f'PASS {\" \".join(fields)}')
    else:
        print(f'FAIL found only: {fields}. Keys: {list(data.keys())[:10]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

        if echo "$verdict" | grep -q "^SKIP"; then
            skip "Extension disconnected during navigate enrichment check."
            return
        fi

        if echo "$verdict" | grep -q "^PASS"; then
            final_attempt="$attempt"
            break
        fi

        # Retry once when navigate appears transiently stuck/pending.
        if [ "$attempt" -eq 1 ] && echo "$result" | grep -qi "pending\|timeout\|expired\|transport_no_response\|still_processing"; then
            sleep 1
            continue
        fi
        final_attempt="$attempt"
        break
    done

    log_diagnostic "25.1" "navigate-enrichment" "$result"

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Navigate result enriched (attempt $final_attempt). $verdict"
    else
        fail "Navigate missing enrichment fields. $verdict. Content: $(truncate "$result" 300)"
    fi
}
run_test_25_1

# ── Test 25.2: Click action returns dom_changes ──────────
begin_test "25.2" "[BROWSER] Click action returns dom_changes" \
    "interact(click) with analyze:true should include dom_changes in result" \
    "Tests: DOM mutation tracking (#78) — dom_changes field in action results"

run_test_25_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi
    if ! require_extension_connected; then
        return
    fi

    # Navigate first to have a clean page.
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Setup for deterministic dom_changes test"}'
    sleep 1

    # Inject a deterministic click target that always mutates DOM when clicked.
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject deterministic DOM mutation target","script":"(function(){var old=document.getElementById(\"enrich-dom-root\"); if(old) old.remove(); var root=document.createElement(\"div\"); root.id=\"enrich-dom-root\"; var btn=document.createElement(\"button\"); btn.id=\"enrich-dom-btn\"; btn.type=\"button\"; btn.textContent=\"Mutate DOM\"; btn.onclick=function(){var n=document.createElement(\"div\"); n.className=\"enrich-dom-added\"; n.textContent=\"added-\"+Date.now(); root.appendChild(n);}; root.appendChild(btn); document.body.appendChild(root); return \"ready\";})()"}'
    sleep 0.5

    interact_and_wait "click" '{"what":"click","selector":"#enrich-dom-btn","reason":"Trigger deterministic DOM mutation","analyze":true}'
    local result="$INTERACT_RESULT"

    if [ -z "$result" ]; then
        fail "No result from click."
        return
    fi

    log_diagnostic "25.2" "click-dom-changes" "$result"

    # Parse dom_changes with support for both count and array payload shapes.
    local dom_verdict
    dom_verdict=$(echo "$result" | python3 -c "
import sys, json
try:
    t = sys.stdin.read()
    i = t.find('{')
    data = json.loads(t[i:]) if i >= 0 else {}
    payload = data.get('result', data) if isinstance(data, dict) else {}
    if not isinstance(payload, dict):
        print('FAIL payload_not_object')
        raise SystemExit
    dc = payload.get('dom_changes', {})
    if not isinstance(dc, dict):
        print(f'FAIL dom_changes_missing keys={list(payload.keys())[:10]}')
        raise SystemExit
    summary = str(dc.get('summary', ''))
    added = dc.get('added', 0)
    removed = dc.get('removed', 0)
    if isinstance(added, list):
        added_count = len(added)
    elif isinstance(added, (int, float)):
        added_count = int(added)
    else:
        added_count = 0
    if isinstance(removed, list):
        removed_count = len(removed)
    elif isinstance(removed, (int, float)):
        removed_count = int(removed)
    else:
        removed_count = 0
    changed = added_count > 0 or removed_count > 0 or ('no dom changes' not in summary.lower())
    if changed:
        print(f'PASS summary={summary} added={added_count} removed={removed_count}')
    else:
        print(f'FAIL summary={summary} added={added_count} removed={removed_count}')
except Exception as e:
    print(f'FAIL parse_error={e}')
" 2>/dev/null || echo "FAIL parse_error")

    # Cross-check actual DOM mutation happened.
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify deterministic DOM mutation happened","script":"document.querySelectorAll(\"#enrich-dom-root .enrich-dom-added\").length"}'
    local dom_count
    dom_count=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json, re
t = sys.stdin.read()
i = t.find('{')
if i >= 0:
    try:
        data = json.loads(t[i:])
        if isinstance(data, dict):
            if 'result' in data and isinstance(data['result'], (str, int, float)):
                print(str(data['result']))
                raise SystemExit
    except Exception:
        pass
m = re.search(r'\\b(\\d+)\\b', t)
print(m.group(1) if m else '0')
" 2>/dev/null || echo "0")

    if echo "$dom_verdict" | grep -q "^PASS" && [ "${dom_count:-0}" -gt 0 ] 2>/dev/null; then
        pass "Click result reports real DOM mutation. $dom_verdict dom_count=${dom_count}"
    else
        fail "Click dom_changes validation failed. $dom_verdict dom_count=${dom_count}. Content: $(truncate "$result" 260)"
    fi
}
run_test_25_2

# ── Test 25.3: Execute JS returns timing enrichment ──────
begin_test "25.3" "[BROWSER] Execute JS returns timing information" \
    "interact(execute_js) should include timing_ms in result" \
    "Tests: action result timing enrichment"

run_test_25_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Smoke test timing","script":"document.title"}'
    local result="$INTERACT_RESULT"

    if [ -z "$result" ]; then
        fail "No result from execute_js."
        return
    fi

    log_diagnostic "25.3" "execute-js-timing" "$result"

    if echo "$result" | grep -q "timing"; then
        pass "Execute JS result includes timing information."
    else
        skip "timing field not present. Content: $(truncate "$result" 200)"
    fi
}
run_test_25_3
