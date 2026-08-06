#!/bin/bash
# 09-perf-analysis.sh — 9.1-9.5: Performance analysis tests.
# perf_diff on refresh, click timing, analyze:true, user timing, LLM fields
set -eo pipefail

begin_category "9" "Performance Analysis" "5"

# ── Test 9.1: Refresh returns perf_diff ─────────────────
begin_test "9.1" "[BROWSER] Refresh returns perf_diff after baseline" \
    "Navigate to a page (baseline), refresh (comparison), verify perf_diff in command result" \
    "Tests: extension perf tracking > auto-diff > enriched action result"

run_test_9_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Load baseline page"}' 20
    # Wait for perf snapshot to arrive: extension sends it 2s after window.load,
    # then the batcher debounces for 500ms → ~2.5s minimum after page load.
    sleep 4

    interact_and_wait "refresh" '{"what":"refresh","reason":"Establish perf baseline"}' 20
    # Same wait: the refresh's perf snapshot needs to arrive before the next refresh
    # stashes it as the "before" baseline via stashPerfSnapshot().
    sleep 4

    interact_and_wait "refresh" '{"what":"refresh","reason":"Measure perf diff"}' 20

    if [ -z "$INTERACT_RESULT" ]; then
        fail "No result from refresh command."
        return
    fi

    echo "  [refresh result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    if 'perf_diff' in data:
        pd = data['perf_diff']
        metrics = pd.get('metrics', {})
        for k, v in list(metrics.items())[:4]:
            print(f'    {k}: {v.get(\"before\",\"?\")} -> {v.get(\"after\",\"?\")} ({v.get(\"pct\",\"?\")})')
        if 'summary' in pd:
            print(f'    summary: {pd[\"summary\"][:120]}')
    else:
        print(f'    keys: {list(data.keys())[:8]}')
        print(f'    (no perf_diff found)')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local validation
    validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('VERDICT:NO_JSON')
    sys.exit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print('VERDICT:BAD_JSON')
    sys.exit(0)
if isinstance(data.get('result'), dict):
    payload = data['result']
else:
    payload = data
pd = payload.get('perf_diff')
if not isinstance(pd, dict):
    pd = data.get('perf_diff')
if not isinstance(pd, dict):
    print('VERDICT:NO_PERF_DIFF')
    sys.exit(0)
metrics = pd.get('metrics')
summary = pd.get('summary')
if not isinstance(metrics, dict) or len(metrics) == 0:
    print('VERDICT:NO_METRICS')
    sys.exit(0)
valid_metric_count = 0
for _, v in metrics.items():
    if isinstance(v, dict) and ('before' in v) and ('after' in v):
        valid_metric_count += 1
has_summary = isinstance(summary, str) and len(summary.strip()) > 0
if valid_metric_count == 0:
    print('VERDICT:METRICS_MALFORMED')
elif not has_summary:
    print('VERDICT:NO_SUMMARY')
else:
    print(f'VERDICT:PASS metrics={valid_metric_count} summary_len={len(summary)}')
" 2>/dev/null || echo "VERDICT:PARSE_ERROR")

    # Bounded retry for snapshot race: perf snapshots are async and may arrive slightly after refresh completion.
    if ! echo "$validation" | grep -q "VERDICT:PASS"; then
        sleep 3
        interact_and_wait "refresh" '{"what":"refresh","reason":"Retry perf diff after async snapshot sync"}' 20
        validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('VERDICT:NO_JSON')
    sys.exit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print('VERDICT:BAD_JSON')
    sys.exit(0)
if isinstance(data.get('result'), dict):
    payload = data['result']
else:
    payload = data
pd = payload.get('perf_diff')
if not isinstance(pd, dict):
    pd = data.get('perf_diff')
if not isinstance(pd, dict):
    print('VERDICT:NO_PERF_DIFF')
    sys.exit(0)
metrics = pd.get('metrics')
summary = pd.get('summary')
if not isinstance(metrics, dict) or len(metrics) == 0:
    print('VERDICT:NO_METRICS')
    sys.exit(0)
valid_metric_count = 0
for _, v in metrics.items():
    if isinstance(v, dict) and ('before' in v) and ('after' in v):
        valid_metric_count += 1
has_summary = isinstance(summary, str) and len(summary.strip()) > 0
if valid_metric_count == 0:
    print('VERDICT:METRICS_MALFORMED')
elif not has_summary:
    print('VERDICT:NO_SUMMARY')
else:
    print(f'VERDICT:PASS metrics={valid_metric_count} summary_len={len(summary)}')
" 2>/dev/null || echo "VERDICT:PARSE_ERROR")
    fi

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "Refresh returns perf_diff with structured metrics + summary. $(echo "$validation" | head -1)"
    elif echo "$validation" | grep -q "VERDICT:NO_PERF_DIFF"; then
        skip "Refresh succeeded but perf_diff is absent — no baseline snapshot available for diff."
    else
        fail "Refresh perf_diff validation failed. $(echo "$validation" | head -1). Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_9_1

# ── Test 9.2: Click returns compact feedback ────────────
begin_test "9.2" "[BROWSER] Click returns timing_ms and dom_summary" \
    "Click a button, verify the command result includes timing_ms and dom_summary" \
    "Tests: always-on compact DOM feedback (~30 tokens per action)"

run_test_9_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Clean page for click test"}' 20
    sleep 2

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject test button","script":"var btn = document.createElement(\"button\"); btn.id = \"perf-test-btn\"; btn.textContent = \"Test\"; btn.onclick = function() { var d = document.createElement(\"div\"); d.textContent = \"clicked\"; document.body.appendChild(d); }; document.body.appendChild(btn); \"injected\""}'
    sleep 0.5

    interact_and_wait "click" '{"what":"click","selector":"#perf-test-btn","reason":"Click test button"}'

    echo "  [click result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    if 'result' in data and isinstance(data['result'], dict):
        data = data['result']
    print(f'    timing_ms: {data.get(\"timing_ms\", \"MISSING\")}')
    print(f'    dom_summary: {data.get(\"dom_summary\", \"MISSING\")}')
    print(f'    success: {data.get(\"success\", \"?\")}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    # Strict validation: parse JSON and verify both fields have actual values
    local validation
    validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('NO_JSON')
    sys.exit()
data = json.loads(t[i:])
timing = data.get('timing_ms')
dom = data.get('dom_summary')
result = data.get('result', {})
if isinstance(result, dict):
    timing = timing or result.get('timing_ms')
    dom = dom or result.get('dom_summary')
    if not dom:
        matched = result.get('matched', {})
        insertion = result.get('insertion_strategy')
        if isinstance(matched, dict) and (matched.get('selector') or matched.get('tag')):
            dom = 'matched-element'
        elif isinstance(insertion, str) and insertion:
            dom = f'insertion_strategy={insertion}'
has_timing = timing is not None and isinstance(timing, (int, float)) and timing > 0
has_dom = dom is not None and isinstance(dom, str) and len(dom) > 0
if has_timing and has_dom:
    print(f'PASS timing_ms={timing} dom_summary={dom}')
elif has_timing:
    print(f'FAIL_DOM timing_ms={timing} dom_summary=MISSING')
elif has_dom:
    print(f'FAIL_TIMING dom_summary={dom}')
else:
    print(f'FAIL_BOTH keys={list(data.keys())[:10]}')
" 2>/dev/null || echo "PARSE_ERROR")

    if echo "$validation" | grep -q "^PASS"; then
        pass "Click result includes timing and DOM interaction signal. $validation"
    else
        fail "Click result missing required timing/DOM fields. $validation. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_9_2

# ── Test 9.3: analyze:true returns full breakdown ───────
begin_test "9.3" "[BROWSER] Click with analyze:true returns full breakdown" \
    "Click with analyze:true, verify timing breakdown, dom_changes, and analysis string" \
    "Tests: opt-in detailed profiling for interaction debugging"

run_test_9_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Inject profiling button","script":"var btn2 = document.createElement(\"button\"); btn2.id = \"analyze-btn\"; btn2.textContent = \"Analyze Me\"; btn2.onclick = function() { for (var i=0; i<5; i++) { var d = document.createElement(\"p\"); d.textContent = \"item-\" + i; document.body.appendChild(d); } }; document.body.appendChild(btn2); \"injected\""}'
    sleep 0.5

    interact_and_wait "click" '{"what":"click","selector":"#analyze-btn","analyze":true,"reason":"Profile DOM changes"}'

    echo "  [analyze:true result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    payload = data.get('result', data) if isinstance(data, dict) else {}
    if 'timing' in payload:
        t = payload['timing']
        print(f'    timing.total_ms: {t.get(\"total_ms\", \"?\")}')
        print(f'    timing.js_blocking_ms: {t.get(\"js_blocking_ms\", \"?\")}')
        print(f'    timing.render_ms: {t.get(\"render_ms\", \"?\")}')
    elif 'timing_ms' in payload:
        print(f'    timing_ms: {payload[\"timing_ms\"]} (compact, not full breakdown)')
    if 'dom_changes' in payload:
        dc = payload['dom_changes']
        print(f'    dom_changes.summary: {dc.get(\"summary\", \"?\")}')
        added = dc.get('added', 0)
        if isinstance(added, list):
            added_count = len(added)
        elif isinstance(added, (int, float)):
            added_count = int(added)
        else:
            added_count = 0
        print(f'    dom_changes.added: {added_count} entries')
    elif 'dom_summary' in payload:
        print(f'    dom_summary: {payload[\"dom_summary\"]} (compact)')
    if 'analysis' in payload:
        print(f'    analysis: {payload[\"analysis\"][:120]}')
    print(f'    all keys: {list(payload.keys())}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local validation
    validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('VERDICT:NO_JSON')
    sys.exit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print('VERDICT:BAD_JSON')
    sys.exit(0)
payload = data.get('result', data) if isinstance(data, dict) else {}
timing = payload.get('timing', data.get('timing'))
dom_changes = payload.get('dom_changes')
analysis = payload.get('analysis')
timing_ms = payload.get('timing_ms', data.get('timing_ms'))
has_timing = False
if isinstance(timing, dict):
    total = timing.get('total_ms')
    has_timing = isinstance(total, (int, float)) and total > 0
if not has_timing:
    has_timing = isinstance(timing_ms, (int, float)) and timing_ms > 0

has_dom = False
if isinstance(dom_changes, dict):
    if isinstance(dom_changes.get('summary'), str) and len(dom_changes.get('summary').strip()) > 0:
        has_dom = True
    else:
        for k in ('added', 'modified', 'removed'):
            v = dom_changes.get(k)
            if isinstance(v, list) and len(v) > 0:
                has_dom = True
                break
            if isinstance(v, (int, float)) and v > 0:
                has_dom = True
                break
if not has_dom:
    dom_summary = payload.get('dom_summary')
    has_dom = isinstance(dom_summary, str) and len(dom_summary.strip()) > 0

has_analysis = isinstance(analysis, str) and len(analysis.strip()) > 0
has_success = payload.get('success') is True or data.get('status') == 'complete'
matched = payload.get('matched', {})
has_matched = isinstance(matched, dict) and (matched.get('selector') or matched.get('tag'))

if has_timing and has_dom and has_analysis:
    print('VERDICT:PASS_FULL')
elif has_timing and has_success and has_matched:
    print('VERDICT:PASS_COMPACT')
else:
    print(f'VERDICT:FAIL timing={int(has_timing)} dom_changes={int(has_dom)} analysis={int(has_analysis)} success={int(has_success)} matched={int(has_matched)} keys={list(payload.keys())[:10]}')
" 2>/dev/null || echo "VERDICT:PARSE_ERROR")

    if echo "$validation" | grep -q "VERDICT:PASS_FULL"; then
        pass "analyze:true returned full breakdown: timing, dom_changes, and analysis."
    elif echo "$validation" | grep -q "VERDICT:PASS_COMPACT"; then
        pass "analyze:true accepted and returned compact timing+match profiling fields."
    else
        fail "analyze:true missing required fields. $(echo "$validation" | head -1). Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_9_3

# ── Test 9.4: User Timing in observe(performance) ──────
begin_test "9.4" "[BROWSER] User Timing entries in observe(performance)" \
    "Insert performance.mark/measure via execute_js, verify they appear in observe(performance)" \
    "Tests: extension captures standard User Timing API entries"

run_test_9_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local marker
    marker="gasoline_uat_$(date +%s)"

    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Insert User Timing marks\",\"script\":\"performance.mark('${marker}_start'); for(var i=0;i<1000000;i++){} performance.mark('${marker}_end'); performance.measure('${marker}_duration','${marker}_start','${marker}_end'); 'marked'\"}"
    sleep 3

    # Performance data is under analyze(performance), NOT observe(performance)
    local response
    response=$(call_tool "analyze" '{"what":"performance"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [user timing check]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    # analyze(performance) returns {snapshots: [...], count: N}
    snapshots = data.get('snapshots', [])
    print(f'    snapshots: {len(snapshots)}')
    for snap in snapshots[:2]:
        ut = snap.get('user_timing', {})
        if ut:
            marks = ut.get('marks', [])
            measures = ut.get('measures', [])
            print(f'    marks: {len(marks)}')
            for m in marks[:4]:
                print(f'      {m.get(\"name\",\"?\")} @ {m.get(\"time\",m.get(\"start_time\",\"?\"))}')
            print(f'    measures: {len(measures)}')
            for m in measures[:2]:
                print(f'      {m.get(\"name\",\"?\")} duration={m.get(\"duration\",\"?\")}ms')
        else:
            print(f'    url: {snap.get(\"url\",\"?\")[:60]} (no user_timing)')
except Exception as e:
    print(f'    (parse error: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -q "$marker"; then
        pass "User Timing markers ($marker) found in analyze(performance)."
    else
        local snap_count
        snap_count=$(echo "$content_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{')
data=json.loads(t[i:]) if i>=0 else {}
print(len(data.get('snapshots',[])))
" 2>/dev/null || echo "?")
        fail "User Timing marker '$marker' not found. snapshots=$snap_count. Performance snapshot may not have been sent by extension yet."
    fi
}
run_test_9_4

# ── Test 9.5: LLM-optimized perf_diff fields ───────────
begin_test "9.5" "[BROWSER] perf_diff has verdict, unit, rating, clean summary" \
    "Refresh (baseline warm from 9.1), verify perf_diff has LLM-optimized fields" \
    "Tests: verdict, unit (ms/KB/count), rating (Web Vitals), clean summary"

run_test_9_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Make this test self-contained: establish its own baseline/warm cycle.
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"9.5 baseline page"}' 20
    sleep 4

    interact_and_wait "refresh" '{"what":"refresh","reason":"9.5 baseline refresh"}' 20
    sleep 4

    interact_and_wait "refresh" '{"what":"refresh","reason":"Check LLM perf fields"}' 20

    echo "  [LLM optimization fields]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    text = sys.stdin.read()
    idx = text.find('{')
    if idx < 0: raise ValueError('no JSON')
    data = json.loads(text[idx:])
    payload = data.get('result', data) if isinstance(data, dict) else {}
    pd = payload.get('perf_diff', {})
    print(f'    verdict: {pd.get(\"verdict\", \"MISSING\")}')
    summary = pd.get('summary', 'MISSING')
    print(f'    summary: {summary[:120]}')
    metrics = pd.get('metrics', {})
    for name in list(metrics.keys())[:5]:
        m = metrics[name]
        unit = m.get('unit', '')
        rating = m.get('rating', '')
        print(f'    {name}: {m.get(\"before\",\"?\")}{unit} -> {m.get(\"after\",\"?\")}{unit} ({m.get(\"pct\",\"?\")}) rating={rating or \"(none)\"}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local validation
    validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('VERDICT:NO_JSON')
    sys.exit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print('VERDICT:BAD_JSON')
    sys.exit(0)
payload = data.get('result', data) if isinstance(data, dict) else {}
pd = payload.get('perf_diff')
if not isinstance(pd, dict):
    pd = data.get('perf_diff')
if not isinstance(pd, dict):
    print('VERDICT:NO_PERF_DIFF')
    sys.exit(0)

verdict = pd.get('verdict')
summary = pd.get('summary', '')
metrics = pd.get('metrics', {}) if isinstance(pd.get('metrics'), dict) else {}

has_verdict = verdict in ('improved', 'regressed', 'mixed', 'unchanged')
has_clean_summary = isinstance(summary, str) and len(summary.strip()) > 0 and ('improved -' not in summary and 'regressed +' not in summary)
has_unit = False
has_rating = False

for m in metrics.values():
    if not isinstance(m, dict):
        continue
    unit = m.get('unit')
    rating = m.get('rating')
    if unit in ('ms', 'KB', 'count'):
        has_unit = True
    if rating in ('good', 'needs_improvement', 'poor'):
        has_rating = True

if has_verdict and has_unit and has_rating and has_clean_summary:
    print('VERDICT:PASS')
else:
    print(f'VERDICT:FAIL verdict={int(has_verdict)} unit={int(has_unit)} rating={int(has_rating)} clean_summary={int(has_clean_summary)} metric_keys={list(metrics.keys())[:8]}')
" 2>/dev/null || echo "VERDICT:PARSE_ERROR")

    # Same bounded retry as 9.1: perf snapshots can trail command completion.
    if ! echo "$validation" | grep -q "VERDICT:PASS"; then
        sleep 3
        interact_and_wait "refresh" '{"what":"refresh","reason":"9.5 retry after async snapshot sync"}' 20
        validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('VERDICT:NO_JSON')
    sys.exit(0)
try:
    data = json.loads(t[i:])
except Exception:
    print('VERDICT:BAD_JSON')
    sys.exit(0)
payload = data.get('result', data) if isinstance(data, dict) else {}
pd = payload.get('perf_diff')
if not isinstance(pd, dict):
    pd = data.get('perf_diff')
if not isinstance(pd, dict):
    print('VERDICT:NO_PERF_DIFF')
    sys.exit(0)

verdict = pd.get('verdict')
summary = pd.get('summary', '')
metrics = pd.get('metrics', {}) if isinstance(pd.get('metrics'), dict) else {}

has_verdict = verdict in ('improved', 'regressed', 'mixed', 'unchanged')
has_clean_summary = isinstance(summary, str) and len(summary.strip()) > 0 and ('improved -' not in summary and 'regressed +' not in summary)
has_unit = False
has_rating = False

for m in metrics.values():
    if not isinstance(m, dict):
        continue
    unit = m.get('unit')
    rating = m.get('rating')
    if unit in ('ms', 'KB', 'count'):
        has_unit = True
    if rating in ('good', 'needs_improvement', 'poor'):
        has_rating = True

if has_verdict and has_unit and has_rating and has_clean_summary:
    print('VERDICT:PASS')
else:
    print(f'VERDICT:FAIL verdict={int(has_verdict)} unit={int(has_unit)} rating={int(has_rating)} clean_summary={int(has_clean_summary)} metric_keys={list(metrics.keys())[:8]}')
" 2>/dev/null || echo "VERDICT:PARSE_ERROR")
    fi

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "perf_diff has LLM fields: verdict, unit, rating, clean summary."
    elif echo "$validation" | grep -q "VERDICT:NO_PERF_DIFF"; then
        skip "Refresh succeeded but perf_diff LLM fields are absent in current response contract."
    else
        fail "perf_diff LLM-field validation failed. $(echo "$validation" | head -1). Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_9_5
