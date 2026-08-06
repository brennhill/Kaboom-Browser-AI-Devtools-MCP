#!/bin/bash
# 28-proof-first.sh — Proof-first smoke validations for ambiguous/CSP/social flows.
#
# Preconditions (documented for nightly use):
# - SMOKE_PROOF_FIRST=1 to enable this module (default: skip entire module)
# - Pilot + extension connected
# - Optional social flows:
#   - SMOKE_SOCIAL=1
#   - SMOKE_LINKEDIN_URL (e.g. https://www.linkedin.com/feed/)
#   - SMOKE_FACEBOOK_URL (e.g. https://www.facebook.com/)
#   - Logged-in session in browser profile used by the extension
#
# Artifacts:
# - Writes screenshot paths and correlation IDs to:
#   ~/.kaboom/smoke-results/proof-first-artifacts.log
set -eo pipefail

begin_category "28" "Proof-First Real-World Flows" "6"

PROOF_FIRST_ENABLED="${SMOKE_PROOF_FIRST:-0}"
PROOF_SOCIAL_ENABLED="${SMOKE_SOCIAL:-0}"
PROOF_ARTIFACT_FILE="${SMOKE_OUTPUT_DIR}/proof-first-artifacts.log"
mkdir -p "$(dirname "$PROOF_ARTIFACT_FILE")"

if [ ! -f "$PROOF_ARTIFACT_FILE" ]; then
    {
        echo "Proof-First Artifacts — $(date)"
        echo "======================================"
    } > "$PROOF_ARTIFACT_FILE"
fi

record_proof_artifact() {
    local key="$1"
    local value="$2"
    local line
    line="$(date +%Y-%m-%dT%H:%M:%S%z) | test=${CURRENT_TEST_ID:-unknown} | ${key}=${value}"
    echo "$line" >> "$PROOF_ARTIFACT_FILE"
    echo "  [artifact] ${key}: ${value}"
}

record_last_corr_id() {
    local corr
    corr=$(echo "$INTERACT_RESULT" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed -E 's/.*"correlation_id":\s*"([^"]+)".*/\1/' || true)
    if [ -n "$corr" ]; then
        record_proof_artifact "corr_id" "$corr"
    fi
}

capture_screenshot_artifact() {
    local label="$1"
    local screenshot_response
    screenshot_response=$(call_tool "observe" '{"what":"screenshot"}')
    local text
    text=$(extract_content_text "$screenshot_response")
    if [ -z "$text" ] && [ -n "$screenshot_response" ]; then
        text="$screenshot_response"
    fi
    local screenshot_path
    screenshot_path=$(echo "$text" | python3 -c 'import sys,json; t=sys.stdin.read(); i=t.find("{"); print(json.loads(t[i:]).get("path","") if i>=0 else "")' 2>/dev/null || true)
    if [ -n "$screenshot_path" ]; then
        record_proof_artifact "screenshot_${label}" "$screenshot_path"
    else
        record_proof_artifact "screenshot_${label}" "missing_path"
    fi
}

module_enabled_or_skip() {
    if [ "$PROOF_FIRST_ENABLED" != "1" ]; then
        skip "Set SMOKE_PROOF_FIRST=1 to enable proof-first flows."
        return 1
    fi
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return 1
    fi
    return 0
}

# ── Test 28.1: Ambiguous selector proof + recovery ───────────────────────────
begin_test "28.1" "[BROWSER] Proof-first ambiguous selector flow" \
    "Broad selector must fail with ambiguous_target, then scoped retry must succeed with screenshot checkpoints" \
    "Tests: real state transition proof + artifacts (screenshots + trace IDs)"

run_test_28_1() {
    module_enabled_or_skip || return

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Proof-first ambiguous flow baseline"}' 20
    record_last_corr_id

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject ambiguous controls","script":"(function(){var old=document.getElementById(\"pf-ambig-root\"); if(old) old.remove(); var root=document.createElement(\"div\"); root.id=\"pf-ambig-root\"; root.innerHTML=\"<div id=\\\"pf-ambig-a\\\"><button class=\\\"pf-ambig-btn\\\" type=\\\"button\\\">Post</button></div><div id=\\\"pf-ambig-b\\\"><button class=\\\"pf-ambig-btn\\\" type=\\\"button\\\">Post</button></div>\"; document.body.appendChild(root); return \"ambig-ready\";})()"}'
    record_last_corr_id
    capture_screenshot_artifact "ambiguous_before"

    interact_and_wait "click" '{"what":"click","selector":".pf-ambig-btn","reason":"Proof-first trigger ambiguous target"}'
    record_last_corr_id
    if ! echo "$INTERACT_RESULT" | grep -qi "ambiguous_target"; then
        fail "Expected ambiguous_target for broad selector. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    interact_and_wait "click" '{"what":"click","selector":".pf-ambig-btn","scope_selector":"#pf-ambig-a","reason":"Proof-first scoped recovery"}'
    record_last_corr_id
    capture_screenshot_artifact "ambiguous_after"
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Scoped recovery failed after ambiguous_target. Result: $(truncate "$INTERACT_RESULT" 240)"
    else
        pass "Ambiguous flow proved: broad selector failed, scoped recovery succeeded, artifacts captured."
    fi
}
run_test_28_1

# ── Test 28.2: CSP execute_js failure + DOM fallback proof ──────────────────
begin_test "28.2" "[BROWSER] Proof-first CSP fallback flow" \
    "execute_js must fail on CSP page, DOM primitive fallback must still succeed, with screenshot checkpoints" \
    "Tests: CSP regression proof with artifacts"

run_test_28_2() {
    module_enabled_or_skip || return

    local csp_url="${SMOKE_CSP_URL:-https://news.google.com/home?hl=en-US}"
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$csp_url\",\"reason\":\"Proof-first CSP target\"}" 30
    record_last_corr_id
    capture_screenshot_artifact "csp_before"

    interact_and_wait "execute_js" '{"what":"execute_js","world":"main","reason":"Expect CSP block in MAIN world","script":"document.title"}'
    record_last_corr_id
    if ! echo "$INTERACT_RESULT" | grep -qi "csp\|content security policy\|trusted type\|unsafe-eval"; then
        skip "CSP block not reproducible on configured URL ($csp_url). Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    interact_and_wait "list_interactive" '{"what":"list_interactive","selector":"main","reason":"CSP-safe fallback via DOM primitive"}'
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "DOM fallback failed after CSP execute_js failure. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    local elem_count
    elem_count=$(echo "$INTERACT_RESULT" | python3 -c '
import sys, json
t = sys.stdin.read()
i = t.find("{")
if i < 0:
    print(0); raise SystemExit
try:
    data = json.loads(t[i:])
except Exception:
    print(0); raise SystemExit
elems = data.get("elements", [])
print(len(elems) if isinstance(elems, list) else 0)
' 2>/dev/null || echo "0")
    capture_screenshot_artifact "csp_after"
    if [ "${elem_count:-0}" -le 0 ] 2>/dev/null; then
        fail "DOM fallback returned no elements after CSP failure. Result: $(truncate "$INTERACT_RESULT" 240)"
    else
        pass "CSP flow proved: execute_js blocked, DOM fallback succeeded with ${elem_count} elements and artifacts."
    fi
}
run_test_28_2

# ── Test 28.3: LinkedIn composer proof (optional) ───────────────────────────
begin_test "28.3" "[INTERACTIVE - BROWSER] Proof-first LinkedIn composer flow (optional)" \
    "Open composer, type, submit, verify dialog closes, with before/after screenshots" \
    "Tests: real social workflow proof; requires logged-in preconditions"

run_test_28_3() {
    module_enabled_or_skip || return

    if [ "$PROOF_SOCIAL_ENABLED" != "1" ]; then
        skip "Set SMOKE_SOCIAL=1 to run social composer flows."
        return
    fi
    local linkedin_url="${SMOKE_LINKEDIN_URL:-}"
    if [ -z "$linkedin_url" ]; then
        skip "SMOKE_LINKEDIN_URL is not set."
        return
    fi

    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$linkedin_url\",\"reason\":\"Proof-first LinkedIn baseline\"}" 30
    record_last_corr_id
    capture_screenshot_artifact "linkedin_before"

    interact_and_wait "open_composer" '{"what":"open_composer","reason":"Open LinkedIn composer"}' 30
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "LinkedIn open_composer failed. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    local post_text="${SMOKE_SOCIAL_POST_TEXT:-This post written with Kaboom MCP}"
    local post_text_json
    post_text_json=$(printf '%s' "$post_text" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
    interact_and_wait "type" "{\"action\":\"type\",\"selector\":\"[role='dialog'] [contenteditable='true']\",\"text\":${post_text_json},\"clear\":true,\"reason\":\"Type LinkedIn post\"}" 30
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "LinkedIn type failed. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    interact_and_wait "submit_active_composer" '{"what":"submit_active_composer","reason":"Submit LinkedIn post"}' 40
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "LinkedIn submit_active_composer failed. Result: $(truncate "$INTERACT_RESULT" 260)"
        return
    fi

    sleep 2
    interact_and_wait "list_interactive" '{"what":"list_interactive","selector":"[role=\"dialog\"][aria-modal=\"true\"]","reason":"Verify LinkedIn composer closed"}'
    record_last_corr_id
    capture_screenshot_artifact "linkedin_after"

    if echo "$INTERACT_RESULT" | grep -qi "scope_not_found\|elements\":\\[\\]"; then
        pass "LinkedIn proof flow succeeded: composer submitted and dialog appears closed."
    else
        fail "LinkedIn composer close/success cue not confirmed. Result: $(truncate "$INTERACT_RESULT" 260)"
    fi
}
run_test_28_3

# ── Test 28.4: Facebook composer proof (optional) ───────────────────────────
begin_test "28.4" "[INTERACTIVE - BROWSER] Proof-first Facebook composer flow (optional)" \
    "Open composer, type, submit, verify dialog closes, with before/after screenshots" \
    "Tests: real social workflow proof; requires logged-in preconditions"

run_test_28_4() {
    module_enabled_or_skip || return

    if [ "$PROOF_SOCIAL_ENABLED" != "1" ]; then
        skip "Set SMOKE_SOCIAL=1 to run social composer flows."
        return
    fi
    local facebook_url="${SMOKE_FACEBOOK_URL:-}"
    if [ -z "$facebook_url" ]; then
        skip "SMOKE_FACEBOOK_URL is not set."
        return
    fi

    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$facebook_url\",\"reason\":\"Proof-first Facebook baseline\"}" 30
    record_last_corr_id
    capture_screenshot_artifact "facebook_before"

    interact_and_wait "open_composer" '{"what":"open_composer","reason":"Open Facebook composer"}' 30
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Facebook open_composer failed. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    local post_text="${SMOKE_SOCIAL_POST_TEXT:-This post written with Kaboom MCP}"
    local post_text_json
    post_text_json=$(printf '%s' "$post_text" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
    interact_and_wait "type" "{\"action\":\"type\",\"selector\":\"[role='dialog'] [contenteditable='true']\",\"text\":${post_text_json},\"clear\":true,\"reason\":\"Type Facebook post\"}" 30
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Facebook type failed. Result: $(truncate "$INTERACT_RESULT" 240)"
        return
    fi

    interact_and_wait "submit_active_composer" '{"what":"submit_active_composer","reason":"Submit Facebook post"}' 40
    record_last_corr_id
    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Facebook submit_active_composer failed. Result: $(truncate "$INTERACT_RESULT" 260)"
        return
    fi

    sleep 2
    interact_and_wait "list_interactive" '{"what":"list_interactive","selector":"[role=\"dialog\"][aria-modal=\"true\"]","reason":"Verify Facebook composer closed"}'
    record_last_corr_id
    capture_screenshot_artifact "facebook_after"

    if echo "$INTERACT_RESULT" | grep -qi "scope_not_found\|elements\":\\[\\]"; then
        pass "Facebook proof flow succeeded: composer submitted and dialog appears closed."
    else
        fail "Facebook composer close/success cue not confirmed. Result: $(truncate "$INTERACT_RESULT" 260)"
    fi
}
run_test_28_4

# ── Test 28.5: Evidence mode returns before/after artifact paths ─────────────
begin_test "28.5" "[BROWSER] Evidence mode captures before/after artifacts" \
    "Run a mutating action with evidence:'always' and verify evidence.before/evidence.after are returned as screenshot paths" \
    "Tests: interact evidence mode state machine and artifact surfacing"

run_test_28_5() {
    module_enabled_or_skip || return

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Evidence mode baseline"}' 20
    record_last_corr_id

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject evidence target button","script":"(function(){var old=document.getElementById(\"pf-evidence-btn\"); if(old) old.remove(); var btn=document.createElement(\"button\"); btn.id=\"pf-evidence-btn\"; btn.type=\"button\"; btn.textContent=\"Evidence Target\"; btn.onclick=function(){document.body.setAttribute(\"data-evidence-clicked\",\"1\")}; document.body.appendChild(btn); return \"evidence-ready\";})()"}'
    record_last_corr_id

    interact_and_wait "click" '{"what":"click","selector":"#pf-evidence-btn","evidence":"always","reason":"Evidence mode validation click"}' 30
    record_last_corr_id

    local parsed
    parsed=$(echo "$INTERACT_RESULT" | python3 -c '
import json,sys
t = sys.stdin.read()
i = t.find("{")
if i < 0:
    print("NO_JSON|||")
    raise SystemExit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print("NO_JSON|||")
    raise SystemExit(0)
e = data.get("evidence")
if not isinstance(e, dict):
    print("NO_EVIDENCE|||")
    raise SystemExit(0)
before = e.get("before", "") or ""
after = e.get("after", "") or ""
if before and after:
    print("OK|%s|%s|" % (before, after))
else:
    errors = e.get("errors", {})
    if isinstance(errors, dict):
        err = ";".join("%s=%s" % (k, errors[k]) for k in sorted(errors.keys()))
    else:
        err = ""
    print("MISSING|%s|%s|%s" % (before, after, err))
')

    local status before_path after_path err_summary
    IFS='|' read -r status before_path after_path err_summary <<< "$parsed"

    if [ "$status" != "OK" ]; then
        fail "Evidence payload missing before/after paths. status=$status before=$before_path after=$after_path errors=$err_summary result=$(truncate "$INTERACT_RESULT" 260)"
        return
    fi

    record_proof_artifact "evidence_before" "$before_path"
    record_proof_artifact "evidence_after" "$after_path"

    local before_exists after_exists
    before_exists="false"
    after_exists="false"
    if [ -f "$before_path" ]; then
        before_exists="true"
    fi
    if [ -f "$after_path" ]; then
        after_exists="true"
    fi

    pass "Evidence mode returned before/after paths (before_exists=$before_exists, after_exists=$after_exists)."
}
run_test_28_5

# ── Test 28.6: Deterministic retry contract terminal stop ───────────────────
begin_test "28.6" "[BROWSER] Retry contract stops after two attempts" \
    "First failure should be retryable with retry_context.attempt=1; second failure chained via correlation_id should return terminal_stop=true" \
    "Tests: deterministic retry contract + terminal evidence summary"

run_test_28_6() {
    module_enabled_or_skip || return

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Retry contract baseline"}' 20
    record_last_corr_id

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject duplicate retry buttons","script":"(function(){var old=document.getElementById(\"pf-retry-root\"); if(old) old.remove(); var root=document.createElement(\"div\"); root.id=\"pf-retry-root\"; root.innerHTML=\"<div id=\\\"pf-retry-a\\\"><button class=\\\"pf-retry-btn\\\" type=\\\"button\\\">Retry Target</button></div><div id=\\\"pf-retry-b\\\"><button class=\\\"pf-retry-btn\\\" type=\\\"button\\\">Retry Target</button></div>\"; document.body.appendChild(root); return \"retry-ready\";})()"}'
    record_last_corr_id

    interact_and_wait "click" '{"what":"click","selector":".pf-retry-btn","reason":"Retry contract first failure"}'
    record_last_corr_id

    local first_eval
    first_eval=$(echo "$INTERACT_RESULT" | python3 -c '
import json,sys
t = sys.stdin.read()
i = t.find("{")
if i < 0:
    print("NO_JSON|")
    raise SystemExit(0)
try:
    d = json.loads(t[i:])
except Exception:
    print("NO_JSON|")
    raise SystemExit(0)
corr = d.get("correlation_id","")
ctx = d.get("retry_context",{}) if isinstance(d.get("retry_context",{}), dict) else {}
attempt = ctx.get("attempt")
terminal = ctx.get("terminal_stop")
print(f"{corr}|{attempt}|{terminal}")
')

    local first_corr first_attempt first_terminal
    IFS='|' read -r first_corr first_attempt first_terminal <<< "$first_eval"

    if [ -z "$first_corr" ] || [ "$first_attempt" != "1" ] || [ "$first_terminal" != "False" ]; then
        fail "First attempt did not return expected retry_context (corr=$first_corr attempt=$first_attempt terminal=$first_terminal). Result: $(truncate "$INTERACT_RESULT" 260)"
        return
    fi

    interact_and_wait "click" "{\"action\":\"click\",\"selector\":\".pf-retry-btn\",\"correlation_id\":\"$first_corr\",\"reason\":\"Retry contract second failure\"}"
    record_last_corr_id

    local second_eval
    second_eval=$(echo "$INTERACT_RESULT" | python3 -c '
import json,sys
t = sys.stdin.read()
i = t.find("{")
if i < 0:
    print("NO_JSON|")
    raise SystemExit(0)
try:
    d = json.loads(t[i:])
except Exception:
    print("NO_JSON|")
    raise SystemExit(0)
ctx = d.get("retry_context",{}) if isinstance(d.get("retry_context",{}), dict) else {}
attempt = ctx.get("attempt")
terminal_stop = ctx.get("terminal_stop")
retryable = d.get("retryable")
terminal = d.get("terminal")
has_summary = isinstance(d.get("evidence_summary"), dict)
print(f"{attempt}|{terminal_stop}|{retryable}|{terminal}|{has_summary}")
')

    local second_attempt second_terminal_stop second_retryable second_terminal has_summary
    IFS='|' read -r second_attempt second_terminal_stop second_retryable second_terminal has_summary <<< "$second_eval"

    if [ "$second_attempt" != "2" ] || [ "$second_terminal_stop" != "True" ] || [ "$second_retryable" != "False" ] || [ "$second_terminal" != "True" ] || [ "$has_summary" != "True" ]; then
        fail "Second attempt terminal contract not met (attempt=$second_attempt terminal_stop=$second_terminal_stop retryable=$second_retryable terminal=$second_terminal evidence_summary=$has_summary). Result: $(truncate "$INTERACT_RESULT" 280)"
        return
    fi

    pass "Retry contract verified: attempt 1 retryable, attempt 2 terminal with evidence summary."
}
run_test_28_6

echo "  Proof-first artifacts file: $PROOF_ARTIFACT_FILE"
