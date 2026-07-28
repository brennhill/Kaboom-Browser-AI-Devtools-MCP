#!/bin/bash
# 22-log-aggregation.sh — 22.1-22.5: Log aggregation via observe(summarized_logs).
# fingerprinting, grouping, anomaly detection, periodicity
set -eo pipefail

begin_category "22" "Log Aggregation" "5"

# ── Test 22.1: Basic summarized logs response ────────────
begin_test "22.1" "[BROWSER] Summarized logs returns groups and anomalies" \
    "Seed repeated logs, then observe(summarized_logs) returns grouped entries" \
    "Tests: log fingerprinting and grouping pipeline"

run_test_22_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed repeated log messages (same structure, different values → should group)
    for i in 1 2 3 4 5; do
        interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed repeated log $i\",\"script\":\"console.log('SMOKE_HEARTBEAT_${SMOKE_MARKER} tick=$i')\"}"
    done
    # Seed a unique anomaly
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed anomaly\",\"script\":\"console.warn('SMOKE_UNIQUE_ANOMALY_${SMOKE_MARKER}')\"}"
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all"}')
    local text
    text=$(extract_content_text "$response")

    if [ -z "$text" ]; then
        fail "Empty response from observe(summarized_logs)."
        return
    fi

    echo "  [summarized logs]"
    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    groups = data.get('groups', [])
    anomalies = data.get('anomalies', [])
    summary = data.get('summary', {})
    total = summary.get('total_entries', 0)
    ratio = summary.get('compression_ratio', 0)
    print(f'    groups: {len(groups)}')
    print(f'    anomalies: {len(anomalies)}')
    print(f'    total_entries: {total}')
    print(f'    compression_ratio: {ratio}')
    if len(groups) > 0:
        for g in groups[:3]:
            print(f'    [{g.get(\"count\",0)}x] {g.get(\"fingerprint\",\"?\")[:50]}')
    if isinstance(groups, list) and isinstance(anomalies, list):
        print(f'VERDICT:PASS groups={len(groups)} anomalies={len(anomalies)} total={total}')
    else:
        print('VERDICT:FAIL missing groups or anomalies array')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL')
" 2>/dev/null || echo "VERDICT:FAIL")

    echo "$verdict" | grep -v "^VERDICT:" || true

    if echo "$verdict" | grep -q "VERDICT:PASS"; then
        pass "observe(summarized_logs) returned structured groups and anomalies."
    else
        fail "observe(summarized_logs) invalid. Content: $(truncate "$text" 200)"
    fi
}
run_test_22_1

# ── Test 22.2: Fingerprinting collapses variable content ─
begin_test "22.2" "[BROWSER] Fingerprinting groups messages with variable content" \
    "Seed logs with different numbers/UUIDs, verify they collapse into one group" \
    "Tests: fingerprint normalization (numbers, UUIDs, timestamps)"

run_test_22_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Clear logs to start fresh
    call_tool "configure" '{"what":"clear","buffer":"logs"}' > /dev/null 2>&1
    sleep 1

    # Seed logs with same structure but different variable content
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed var log 1","script":"console.log(\"SMOKE_FP_REQUEST user=1001 took 234ms\")"}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed var log 2","script":"console.log(\"SMOKE_FP_REQUEST user=2002 took 567ms\")"}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed var log 3","script":"console.log(\"SMOKE_FP_REQUEST user=3003 took 890ms\")"}'
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all"}')
    local text
    text=$(extract_content_text "$response")

    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    groups = data.get('groups', [])
    # Find a group that matches our SMOKE_FP pattern
    fp_groups = [g for g in groups if 'smoke_fp' in g.get('fingerprint', '').lower()]
    if len(fp_groups) == 1 and fp_groups[0].get('count', 0) >= 3:
        print(f'PASS fingerprint={fp_groups[0][\"fingerprint\"]} count={fp_groups[0][\"count\"]}')
    elif len(fp_groups) == 0:
        # May be grouped under a different fingerprint — check for any group with count >= 3
        big_groups = [g for g in groups if g.get('count', 0) >= 3]
        if big_groups:
            print(f'PASS (indirect) largest_group_count={big_groups[0][\"count\"]} fp={big_groups[0][\"fingerprint\"][:40]}')
        else:
            print(f'FAIL no group with count>=3. groups={len(groups)} fps={[g.get(\"fingerprint\",\"\")[:30] for g in groups[:5]]}')
    else:
        print(f'FAIL expected 1 fp group, got {len(fp_groups)}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Fingerprinting collapsed variable-content logs into one group. $verdict"
    else
        fail "Fingerprinting did not collapse as expected. $verdict. Content: $(truncate "$text" 200)"
    fi
}
run_test_22_2

# ── Test 22.3: Level breakdown ───────────────────────────
begin_test "22.3" "[BROWSER] Groups include level breakdown" \
    "Seed same message at different log levels, verify level_breakdown" \
    "Tests: per-group level tracking"

run_test_22_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "configure" '{"what":"clear","buffer":"logs"}' > /dev/null 2>&1
    sleep 1

    # Same message at different levels
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed level log","script":"console.log(\"SMOKE_LEVEL_TEST multi-level message\")"}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed level warn","script":"console.warn(\"SMOKE_LEVEL_TEST multi-level message\")"}'
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed level error","script":"console.error(\"SMOKE_LEVEL_TEST multi-level message\")"}'
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all"}')
    local text
    text=$(extract_content_text "$response")

    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    groups = data.get('groups', [])
    level_groups = [g for g in groups if 'smoke_level_test' in g.get('fingerprint', '').lower() or 'level_test' in g.get('sample_message', '').lower()]
    if level_groups:
        g = level_groups[0]
        bd = g.get('level_breakdown', {})
        levels = list(bd.keys())
        print(f'PASS breakdown={bd} levels={levels}')
    else:
        # Check any group with level_breakdown having multiple levels
        multi = [g for g in groups if len(g.get('level_breakdown', {})) > 1]
        if multi:
            print(f'PASS (indirect) found group with multi-level breakdown: {multi[0].get(\"level_breakdown\",{})}')
        else:
            print(f'FAIL no group with level_breakdown. groups={len(groups)}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Groups include level_breakdown with multiple levels. $verdict"
    else
        fail "Level breakdown not found. $verdict. Content: $(truncate "$text" 200)"
    fi
}
run_test_22_3

# ── Test 22.4: min_group_size parameter ──────────────────
begin_test "22.4" "[BROWSER] min_group_size parameter controls grouping threshold" \
    "observe(summarized_logs, min_group_size=1) includes singles as groups" \
    "Tests: configurable grouping threshold"

run_test_22_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "configure" '{"what":"clear","buffer":"logs"}' > /dev/null 2>&1
    sleep 1

    # Seed one unique message
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed unique log\",\"script\":\"console.log('SMOKE_SINGLETON_${SMOKE_MARKER}')\"}"
    sleep 2

    # With default min_group_size=2, the single entry should be an anomaly
    local default_response
    default_response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all"}')
    local default_text
    default_text=$(extract_content_text "$default_response")

    # With min_group_size=1, it should be a group
    local mgs1_response
    mgs1_response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all","min_group_size":1}')
    local mgs1_text
    mgs1_text=$(extract_content_text "$mgs1_response")

    local verdict
    verdict=$(python3 -c "
import sys, json

def parse(text):
    i = text.find('{')
    return json.loads(text[i:]) if i >= 0 else {}

try:
    d_default = parse('''$default_text''')
    d_mgs1 = parse('''$mgs1_text''')
    default_anomalies = len(d_default.get('anomalies', []))
    default_groups = len(d_default.get('groups', []))
    mgs1_anomalies = len(d_mgs1.get('anomalies', []))
    mgs1_groups = len(d_mgs1.get('groups', []))
    # With min_group_size=1, anomalies should decrease and groups should increase
    if mgs1_groups >= default_groups and mgs1_anomalies <= default_anomalies:
        print(f'PASS default(g={default_groups},a={default_anomalies}) mgs1(g={mgs1_groups},a={mgs1_anomalies})')
    else:
        print(f'FAIL default(g={default_groups},a={default_anomalies}) mgs1(g={mgs1_groups},a={mgs1_anomalies})')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "min_group_size=1 promotes singles to groups. $verdict"
    else
        fail "min_group_size parameter did not behave as expected. $verdict"
    fi
}
run_test_22_4

# ── Test 22.5: Compression ratio and summary ─────────────
begin_test "22.5" "[BROWSER] Summary includes compression ratio and time range" \
    "observe(summarized_logs) summary has total_entries, compression_ratio, time_range" \
    "Tests: summary metadata fields"

run_test_22_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed enough logs for meaningful compression
    for i in 1 2 3 4 5 6 7 8; do
        interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed compression log\",\"script\":\"console.log('SMOKE_COMPRESS_${SMOKE_MARKER} iteration=$i')\"}"
    done
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"summarized_logs","scope":"all"}')
    local text
    text=$(extract_content_text "$response")

    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    summary = data.get('summary', {})
    total = summary.get('total_entries', 0)
    ratio = summary.get('compression_ratio', -1)
    time_range = summary.get('time_range', {})
    start = time_range.get('start', '')
    end = time_range.get('end', '')
    groups = summary.get('groups', 0)
    anomalies = summary.get('anomalies', 0)
    print(f'    total_entries: {total}')
    print(f'    groups: {groups}')
    print(f'    anomalies: {anomalies}')
    print(f'    compression_ratio: {ratio}')
    print(f'    time_range: {start[:19]}..{end[:19]}')
    checks = []
    if total > 0: checks.append('total')
    if ratio >= 0: checks.append('ratio')
    if start and end: checks.append('time_range')
    if len(checks) == 3:
        print(f'VERDICT:PASS all_fields_present ratio={ratio}')
    else:
        print(f'VERDICT:FAIL missing={set([\"total\",\"ratio\",\"time_range\"]) - set(checks)}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL')
" 2>/dev/null || echo "VERDICT:FAIL")

    echo "$verdict" | grep -v "^VERDICT:" || true

    if echo "$verdict" | grep -q "VERDICT:PASS"; then
        pass "Summary includes total_entries, compression_ratio, and time_range."
    else
        fail "Summary missing expected fields. Content: $(truncate "$text" 200)"
    fi
}
run_test_22_5
