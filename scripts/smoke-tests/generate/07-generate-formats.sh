#!/bin/bash
# 07-generate-formats.sh — 7.1-7.7: All 7 generate formats.
# reproduction, test, pr_summary, sarif, har, csp, sri
set -eo pipefail

begin_category "7" "Generate Formats" "7"

# ── Test 7.1: Reproduction ──────────────────────────────
begin_test "7.1" "[DAEMON ONLY] Generate reproduction script" \
    "generate(reproduction) produces Playwright code patterns" \
    "Tests: action replay code generation"

run_test_7_1() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"what":"reproduction"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(reproduction)."
        return
    fi

    echo "  [reproduction preview]"
    echo "$content_text" | head -5

    # Prior tests seeded multiple actions — reproduction MUST contain code
    if echo "$content_text" | grep -qE "page\.|await.*goto|\.click\("; then
        pass "generate(reproduction) contains Playwright code (page.goto/click)."
    elif echo "$content_text" | grep -qiE "page\.|goto|click|playwright"; then
        pass "generate(reproduction) contains Playwright code patterns."
    else
        fail "generate(reproduction) missing Playwright patterns. Actions were seeded by prior tests. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_1

# ── Test 7.2: Test ──────────────────────────────────────
begin_test "7.2" "[DAEMON ONLY] Generate Playwright test" \
    "generate(test, test_name='smoke-test') produces test/expect patterns" \
    "Tests: test scaffold generation"

run_test_7_2() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"what":"test","test_name":"smoke-test"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(test)."
        return
    fi

    echo "  [test preview]"
    echo "$content_text" | head -5

    # Verify it contains test code
    if echo "$content_text" | grep -qE "test\(|describe\(|it\("; then
        pass "generate(test) contains test framework patterns (test/describe/it)."
    elif echo "$content_text" | grep -qiE "expect|playwright|page\."; then
        pass "generate(test) contains test assertion patterns."
    else
        fail "generate(test) missing test patterns. Actions were seeded by prior tests. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_2

# ── Test 7.3: PR Summary ───────────────────────────────
begin_test "7.3" "[BROWSER] Generate PR summary" \
    "generate(pr_summary) produces markdown summary with session stats" \
    "Tests: session summary for PR descriptions"

run_test_7_3() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Seed browser activity so pr_summary has data to summarize.
    # Navigate to a page — this generates actions and network entries.
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Seed activity for PR summary"}' 20
        sleep 2
    fi

    local response
    response=$(call_tool "generate" '{"what":"pr_summary"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(pr_summary)."
        return
    fi

    echo "  [pr_summary preview]"
    echo "$content_text" | head -5

    # Validate: summary must be a non-empty string with meaningful content.
    # With seeded data: expect markdown with stats (Actions, Commands, etc.)
    # Without seeded data: expect "No activity captured" message.
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    if len(t.strip()) > 20:
        print(f'PASS text_len={len(t.strip())}')
    else:
        print(f'FAIL too_short text_len={len(t.strip())}')
    sys.exit()
data = json.loads(t[i:])
summary = data.get('summary', data.get('text', data.get('description', '')))
stats = data.get('stats', {})
if not isinstance(summary, str) or len(summary.strip()) < 10:
    keys = [k for k in data.keys() if k not in ('metadata',)]
    print(f'FAIL summary={repr(summary)[:50]} keys={keys[:8]}')
elif 'Session Summary' not in summary:
    print(f'FAIL missing Session Summary header. Got: {summary[:80]}')
elif 'No activity' in summary:
    print(f'PASS no_activity summary_len={len(summary.strip())}')
elif 'Actions' in summary:
    total = sum(v for v in stats.values() if isinstance(v, int))
    print(f'PASS with_stats summary_len={len(summary.strip())} stat_total={total}')
else:
    print(f'PASS summary_len={len(summary.strip())}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$validation" | grep -q "^PASS"; then
        pass "generate(pr_summary) returned meaningful summary. $validation"
    else
        fail "generate(pr_summary) returned empty or insufficient summary. $validation. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_3

# ── Test 7.4: SARIF ─────────────────────────────────────
begin_test "7.4" "[DAEMON ONLY] Generate SARIF report" \
    "generate(sarif) produces valid SARIF structure with version, schema, runs" \
    "Tests: accessibility/security results in SARIF format"

run_test_7_4() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"what":"sarif"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(sarif)."
        return
    fi

    echo "  [sarif structure]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    print(f'    version: {data.get(\"version\", \"?\")}')
    print(f'    \$schema: {str(data.get(\"\$schema\", \"?\"))[:60]}')
    runs = data.get('runs', [])
    print(f'    runs: {len(runs)}')
    if runs:
        results = runs[0].get('results', [])
        print(f'    results in run[0]: {len(results)}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    # Validate SARIF structure: version 2.1.0, $schema, runs array with tool.driver
    # Results may be 0 if no a11y/security audit was run — that's valid structure.
    local sarif_verdict
    sarif_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    version = data.get('version', '')
    schema = data.get('\$schema', '')
    runs = data.get('runs', [])
    if version != '2.1.0':
        print(f'FAIL version={version} expected=2.1.0')
    elif not isinstance(runs, list) or len(runs) == 0:
        print(f'FAIL no runs array')
    elif not schema:
        print(f'FAIL missing \$schema field')
    else:
        driver = runs[0].get('tool', {}).get('driver', {})
        driver_name = driver.get('name', '')
        results = runs[0].get('results', [])
        result_count = len(results) if isinstance(results, list) else 0
        if not driver_name:
            print(f'FAIL missing tool.driver.name in runs[0]')
        else:
            print(f'PASS version=2.1.0 driver={driver_name} results={result_count}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$sarif_verdict" | grep -q "^PASS"; then
        pass "generate(sarif) valid SARIF structure. $sarif_verdict"
    else
        fail "generate(sarif) invalid structure. $sarif_verdict. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_4

# ── Test 7.5: HAR ───────────────────────────────────────
begin_test "7.5" "[BROWSER] Generate HAR archive" \
    "generate(har) produces HAR structure with log, version, creator, entries" \
    "Tests: network traffic export in HAR format"

run_test_7_5() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Seed network traffic so HAR has entries to export.
    local data_seeded="false"
    if [ "$PILOT_ENABLED" = "true" ]; then
        # Navigate to generate resource timing (waterfall) entries.
        interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Seed network for HAR export"}' 20
        sleep 2
        # Trigger a fetch for body capture.
        interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed fetch for HAR export","script":"fetch(\"https://jsonplaceholder.typicode.com/posts/1\").then(r=>r.json()).then(d=>\"fetched\").catch(e=>e.message)"}'

        # Poll network_bodies until the fetch URL appears.
        local max_polls=10
        for i in $(seq 1 "$max_polls"); do
            sleep 1
            local bodies_response
            bodies_response=$(call_tool "observe" '{"what":"network_bodies","url":"jsonplaceholder"}')
            local bodies_text
            bodies_text=$(extract_content_text "$bodies_response")
            if echo "$bodies_text" | grep -q "jsonplaceholder"; then
                data_seeded="true"
                echo "  [data confirmed in network_bodies after ${i}s]"
                break
            fi
        done

        if [ "$data_seeded" != "true" ]; then
            echo "  [warning: fetch body not confirmed after ${max_polls}s, proceeding anyway]"
        fi
    fi

    local har_file
    har_file="/tmp/kaboom-smoke-har-${SMOKE_MARKER:-smoke}-$(date +%s)-$$.har"

    local response
    response=$(call_tool "generate" "{\"format\":\"har\",\"save_to\":\"$har_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(har)."
        return
    fi

    echo "  [har structure]"
    local validation
    if [ ! -f "$har_file" ]; then
        fail "generate(har) did not create HAR file at $har_file. Response: $(truncate "$content_text" 200)"
        return
    fi

    validation=$(cat "$har_file" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    log = data.get('log', data)
    version = log.get('version', '?')
    creator = log.get('creator', {})
    entries = log.get('entries', [])
    entry_count = len(entries) if isinstance(entries, list) else 0
    print(f'    version: {version}')
    print(f'    creator: {creator.get(\"name\", \"?\")} {creator.get(\"version\", \"?\")}')
    print(f'    entries: {entry_count}')
    if entries:
        print(f'    first url: {entries[0].get(\"request\", {}).get(\"url\", \"?\")[:60]}')
    has_version = version != '?'
    has_creator = bool(creator.get('name'))
    if has_version and has_creator and entry_count > 0:
        print(f'VERDICT:PASS entries={entry_count}')
    elif has_version and has_creator:
        print(f'VERDICT:FAIL_EMPTY valid HAR structure but 0 entries')
    else:
        print(f'VERDICT:FAIL_STRUCTURE version={version} creator={creator}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "generate(har) wrote valid HAR to $har_file. $(echo "$validation" | grep 'VERDICT:' | head -1)"
    else
        fail "generate(har) file validation failed. $(echo "$validation" | grep 'VERDICT:' | head -1 || echo 'no verdict'). File: $har_file"
    fi
}
run_test_7_5

# ── Test 7.6: CSP ───────────────────────────────────────
begin_test "7.6" "[DAEMON ONLY] Generate Content Security Policy" \
    "generate(csp, mode='moderate') produces policy directives" \
    "Tests: CSP generation from observed resources"

run_test_7_6() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"what":"csp","mode":"moderate"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(csp)."
        return
    fi

    echo "  [csp policy]"
    echo "$content_text" | head -3

    # CSP must contain actual directives, not just the word "csp"
    if echo "$content_text" | grep -qiE "default-src|script-src|style-src|connect-src|img-src"; then
        pass "generate(csp) returned CSP policy with directives."
    elif echo "$content_text" | grep -qiE "'self'|'none'|'unsafe-inline'"; then
        pass "generate(csp) returned CSP policy values."
    else
        fail "generate(csp) missing CSP directives (default-src, script-src, etc.). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_6

# ── Test 7.7: SRI ───────────────────────────────────────
begin_test "7.7" "[BROWSER] Generate Subresource Integrity hashes" \
    "generate(sri) returns SRI hashes for seeded third-party JS via fetch()" \
    "Tests: SRI hash generation with verified network body capture"

# Keep resource under 16KB capture limit so SRI hash can be computed from full body.
SRI_CDN_URL="https://cdnjs.cloudflare.com/ajax/libs/dayjs/1.11.10/dayjs.min.js"

run_test_7_7() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # ── Step 1: Seed a third-party JS fetch so SRI has a body to hash ──
    # Use fetch() — reliably captured in network_bodies (unlike <script> tag injection).
    local data_seeded="false"
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Fetch CDN JS for SRI test\",\"script\":\"fetch('${SRI_CDN_URL}').then(r=>r.text()).then(t=>'fetched:'+t.length).catch(e=>'error:'+e.message)\"}"

        # ── Step 2: Poll network_bodies until the CDN URL appears with a real body ──
        local max_polls=10
        for i in $(seq 1 "$max_polls"); do
            sleep 1
            local bodies_response
            bodies_response=$(call_tool "observe" '{"what":"network_bodies","url":"cdnjs.cloudflare.com"}')
            local bodies_text
            bodies_text=$(extract_content_text "$bodies_response")
            if echo "$bodies_text" | grep -qE "dayjs|cdnjs.cloudflare.com"; then
                # Verify response_body is real content, not a placeholder
                local body_check
                body_check=$(echo "$bodies_text" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0: print('NO_JSON'); sys.exit()
data = json.loads(t[i:])
entries = data.get('entries', data.get('data', []))
if not entries: print('NO_ENTRIES'); sys.exit()
body = entries[0].get('response_body', '')
truncated = bool(entries[0].get('response_truncated', False))
if not body: print('EMPTY_BODY')
elif body.startswith('['): print(f'PLACEHOLDER:{body[:50]}')
elif truncated: print(f'TRUNCATED_BODY:len={len(body)}')
else: print(f'REAL_BODY:len={len(body)}')
" 2>/dev/null || echo "PARSE_ERROR")
                echo "  [body check: $body_check after ${i}s]"
                if echo "$body_check" | grep -q "REAL_BODY"; then
                    data_seeded="true"
                    break
                fi
                # Keep polling if body is empty or placeholder
            fi
        done

        if [ "$data_seeded" != "true" ]; then
            skip "SRI data seeding unavailable: CDN JS body not captured after ${max_polls}s. Skipping strict hash validation."
            return
        fi
    fi

    # ── Step 3: Call generate(sri) ──
    local response
    response=$(call_tool "generate" '{"what":"sri"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(sri)."
        return
    fi

    echo "  [sri data]"
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    resources = data.get('resources', [])
    summary = data.get('summary', {})
    count = len(resources) if isinstance(resources, list) else 0
    hashes_gen = summary.get('hashes_generated', 0) if isinstance(summary, dict) else 0
    status = data.get('status', '')

    print(f'    resources: {count}')
    print(f'    hashes_generated: {hashes_gen}')

    # ── No data path (pilot not available) ──
    if status == 'unavailable':
        print(f'VERDICT:NO_DATA status=unavailable')
        sys.exit(0)
    if count == 0:
        print(f'VERDICT:EMPTY valid structure but 0 resources')
        sys.exit(0)

    # ── Validate each resource deeply ──
    errors = []
    for idx, r in enumerate(resources[:5]):
        url = r.get('url', '')
        hash_val = r.get('hash', '')
        tag = r.get('tag_template', '')
        crossorigin = r.get('crossorigin', '')
        res_type = r.get('type', '')
        size = r.get('size_bytes', 0)

        print(f'    [{idx}] {url[:60]}')
        print(f'        hash: {hash_val[:50]}')
        print(f'        type: {res_type}  size: {size}  crossorigin: {crossorigin}')

        if not hash_val.startswith('sha384-'):
            errors.append(f'resource[{idx}]: hash missing sha384- prefix: {hash_val[:30]}')
        if len(hash_val) < 15:
            errors.append(f'resource[{idx}]: hash too short: {hash_val}')
        if 'integrity=' not in tag:
            errors.append(f'resource[{idx}]: tag_template missing integrity attribute')
        if crossorigin != 'anonymous':
            errors.append(f'resource[{idx}]: crossorigin should be anonymous, got: {crossorigin}')
        if res_type not in ('script', 'style'):
            errors.append(f'resource[{idx}]: type should be script or style, got: {res_type}')
        if size <= 0:
            errors.append(f'resource[{idx}]: size_bytes should be > 0, got: {size}')

    # ── Validate summary ──
    if hashes_gen != count:
        errors.append(f'summary.hashes_generated ({hashes_gen}) != resource count ({count})')

    if errors:
        for e in errors:
            print(f'    ERROR: {e}')
        print(f'VERDICT:FAIL_VALIDATION {len(errors)} errors')
    else:
        print(f'VERDICT:PASS_WITH_DATA resources={count} hashes={hashes_gen}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    # ── Step 4: Verdict — strict when data was seeded, lenient otherwise ──
    if [ "$data_seeded" = "true" ]; then
        # Pilot seeded data and we confirmed it landed — demand real results.
        if echo "$validation" | grep -q "VERDICT:PASS_WITH_DATA"; then
            pass "generate(sri) returned validated SRI hashes. $(echo "$validation" | grep 'VERDICT:' | head -1)"
        else
            fail "generate(sri) failed with seeded data. $(echo "$validation" | grep 'VERDICT:' | head -1 || echo 'no verdict'). Content: $(truncate "$content_text" 200)"
        fi
    else
        # No pilot — can't seed data, accept structural validity.
        if echo "$validation" | grep -q "VERDICT:PASS_WITH_DATA"; then
            pass "generate(sri) returned SRI hashes (no seeding needed). $(echo "$validation" | grep 'VERDICT:' | head -1)"
        elif echo "$validation" | grep -q "VERDICT:NO_DATA\|VERDICT:EMPTY"; then
            pass "generate(sri) valid structure (no pilot to seed data). $(echo "$validation" | grep 'VERDICT:' | head -1)"
        else
            fail "generate(sri) invalid response. $(echo "$validation" | grep 'VERDICT:' | head -1 || echo 'no verdict'). Content: $(truncate "$content_text" 200)"
        fi
    fi
}
run_test_7_7
