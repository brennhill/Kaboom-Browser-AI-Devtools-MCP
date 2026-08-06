#!/bin/bash
# 32-metrics-collection.sh — 32.1-32.3: Verify analytics pipeline end-to-end.
# Proves that tool calls flow through UsageCounter → beacon payload with install ID.
set -eo pipefail

begin_category "32" "Metrics Collection" "3"

ensure_daemon

USAGE_URL="http://127.0.0.1:${PORT}/debug/usage"
FLUSH_URL="http://127.0.0.1:${PORT}/debug/beacon-flush"

# Debug endpoints require KABOOM_DEBUG=1. Check if they're available.
_debug_probe=$(curl -s --connect-timeout 2 "$USAGE_URL" 2>/dev/null || echo "")
if echo "$_debug_probe" | grep -q '"error"' || [ -z "$_debug_probe" ]; then
    echo "  NOTE: Debug endpoints not available (start daemon with KABOOM_DEBUG=1)."
    echo "  Skipping metrics collection tests."
    skip "Debug endpoints require KABOOM_DEBUG=1. Restart daemon with: KABOOM_DEBUG=1 kaboom-agentic-browser --daemon --port $PORT"
    skip "Debug endpoints require KABOOM_DEBUG=1."
    skip "Debug endpoints require KABOOM_DEBUG=1."
    return 0 2>/dev/null || exit 0
fi

# ── Test 32.1: Tool calls populate UsageCounter ──────────
begin_test "32.1" "[DAEMON ONLY] All 5 tools populate UsageCounter" \
    "Call each tool once, verify /debug/usage has tool:mode keys for all 5" \
    "Tests: UsageCounter.Increment wiring in tools_core.go HandleToolCall"

run_test_32_1() {
    # Flush any stale counts from prior tests
    curl -s -X POST "$FLUSH_URL" >/dev/null 2>&1

    # Call each tool once with a known "what" param
    call_tool "observe" '{"what":"page"}' >/dev/null 2>&1
    call_tool "configure" '{"what":"describe_capabilities"}' >/dev/null 2>&1
    call_tool "analyze" '{"what":"page_structure"}' >/dev/null 2>&1
    call_tool "generate" '{"what":"har"}' >/dev/null 2>&1
    call_tool "interact" '{"what":"describe_capabilities"}' >/dev/null 2>&1

    local usage
    usage=$(curl -s --connect-timeout 3 "$USAGE_URL" 2>/dev/null || echo "{}")

    log_diagnostic "32.1" "usage-snapshot" "$usage"

    local validation
    validation=$(echo "$usage" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
counts = data.get('counts', {})
expected = ['observe:page', 'configure:describe_capabilities', 'analyze:page_structure', 'generate:har', 'interact:describe_capabilities']
found = []
missing = []
for key in expected:
    if counts.get(key, 0) > 0:
        found.append(key)
    else:
        missing.append(key)
if missing:
    print(f'FAIL missing={missing} found={found} counts={dict(list(counts.items())[:10])}')
else:
    print(f'PASS found={len(found)} keys={sorted(counts.keys())}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$validation" | grep -q "^PASS"; then
        pass "All 5 tools populated UsageCounter. $validation"
    else
        fail "UsageCounter missing expected tool keys. $validation"
    fi
}
run_test_32_1

# ── Test 32.2: Beacon flush returns valid payload ────────
begin_test "32.2" "[DAEMON ONLY] Beacon flush returns payload with iid, sid, props" \
    "POST /debug/beacon-flush, verify payload has event, iid, sid, and tool:mode props" \
    "Tests: BeaconUsageSummary envelope — install ID, session ID, usage props"

run_test_32_2() {
    # Ensure there's activity to flush (32.1 may have been flushed already by the peek)
    call_tool "observe" '{"what":"errors"}' >/dev/null 2>&1
    call_tool "configure" '{"what":"health"}' >/dev/null 2>&1

    local flush_response
    flush_response=$(curl -s -X POST --connect-timeout 3 "$FLUSH_URL" 2>/dev/null || echo "{}")

    log_diagnostic "32.2" "beacon-flush" "$flush_response"

    local validation
    validation=$(echo "$flush_response" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
flushed = data.get('flushed', 0)
payload = data.get('payload')
if not payload or not isinstance(payload, dict):
    print(f'FAIL no_payload flushed={flushed} data_keys={list(data.keys())}')
    sys.exit(0)

errors = []
# Must have event=usage_summary
if payload.get('event') != 'usage_summary':
    errors.append(f'event={payload.get(\"event\")}')
# Must have install ID (12-char hex)
iid = payload.get('iid', '')
if not iid or len(iid) < 8:
    errors.append(f'iid_missing_or_short={repr(iid)}')
# Must have session ID (16-char hex)
sid = payload.get('sid', '')
if not sid or len(sid) < 8:
    errors.append(f'sid_missing_or_short={repr(sid)}')
# Must have version
if not payload.get('v'):
    errors.append('version_missing')
# Must have OS
if not payload.get('os'):
    errors.append('os_missing')
# Must have props with tool:mode keys
props = payload.get('props', {})
if not isinstance(props, dict) or len(props) == 0:
    errors.append(f'props_empty_or_missing')
else:
    # At least one tool:mode key
    has_tool_key = any(':' in k for k in props)
    if not has_tool_key:
        errors.append(f'no_tool_mode_keys props={list(props.keys())[:5]}')

if errors:
    print(f'FAIL {\" \".join(errors)}')
else:
    print(f'PASS iid={iid[:6]}... sid={sid[:6]}... props={sorted(props.keys())} flushed={flushed}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$validation" | grep -q "^PASS"; then
        pass "Beacon payload has valid envelope. $validation"
    else
        fail "Beacon payload validation failed. $validation"
    fi
}
run_test_32_2

# ── Test 32.3: Flush resets UsageCounter ─────────────────
begin_test "32.3" "[DAEMON ONLY] Beacon flush resets UsageCounter to zero" \
    "After flush, /debug/usage should return empty counts (SwapAndReset)" \
    "Tests: UsageCounter.SwapAndReset atomicity — counters reset after beacon fire"

run_test_32_3() {
    # Seed some activity
    call_tool "observe" '{"what":"page"}' >/dev/null 2>&1

    # Verify non-empty before flush
    local before
    before=$(curl -s --connect-timeout 3 "$USAGE_URL" 2>/dev/null || echo "{}")
    local before_count
    before_count=$(echo "$before" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
print(len(data.get('counts', {})))
" 2>/dev/null || echo "0")

    if [ "$before_count" -eq 0 ]; then
        fail "UsageCounter was already empty before flush — cannot test reset."
        return
    fi

    # Flush
    curl -s -X POST "$FLUSH_URL" >/dev/null 2>&1

    # Verify empty after flush
    local after
    after=$(curl -s --connect-timeout 3 "$USAGE_URL" 2>/dev/null || echo "{}")

    log_diagnostic "32.3" "post-flush-usage" "$after"

    local after_count
    after_count=$(echo "$after" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
print(len(data.get('counts', {})))
" 2>/dev/null || echo "0")

    if [ "$after_count" -eq 0 ]; then
        pass "UsageCounter reset to empty after flush. Had ${before_count} keys before."
    else
        fail "UsageCounter still has ${after_count} keys after flush (expected 0). Before: ${before_count}."
    fi
}
run_test_32_3
