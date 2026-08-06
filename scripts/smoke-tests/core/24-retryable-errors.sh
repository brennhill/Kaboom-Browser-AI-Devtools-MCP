#!/bin/bash
# 24-retryable-errors.sh — 24.1-24.2: ErrNoData retryable field in error responses.
# Verifies that errors include retryable hints for MCP clients.
set -eo pipefail

begin_category "24" "Retryable Errors" "2"

# ── Test 24.1: Extension-disconnected observe returns retryable ──
begin_test "24.1" "[DAEMON ONLY] Observe without extension returns retryable error" \
    "When extension is not connected, observe should fail with retryable hint" \
    "Tests: ErrNoData includes retryable field for client auto-retry (#151)"

run_test_24_1() {
    # Disconnect extension by stopping and restarting daemon fresh
    kill_server
    sleep 1
    start_daemon

    # Call observe immediately before extension can reconnect
    local response
    response=$(call_tool "observe" '{"what":"page"}')

    if [ -z "$response" ]; then
        fail "No response from observe."
        return
    fi

    log_diagnostic "24.1" "observe-no-ext" "$response"

    local text
    text=$(extract_content_text "$response")

    # Check if the response indicates extension not connected (could be error or structured error)
    local has_retryable
    has_retryable=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read()
    i = t.find('{')
    if i < 0:
        # Not JSON — check for retryable in raw text
        print('FOUND' if 'retryable' in t.lower() else 'NOT_FOUND')
    else:
        data = json.loads(t[i:])
        # Walk nested structures looking for retryable
        def find_retryable(obj):
            if isinstance(obj, dict):
                if 'retryable' in obj:
                    return obj['retryable']
                for v in obj.values():
                    r = find_retryable(v)
                    if r is not None:
                        return r
            return None
        r = find_retryable(data)
        if r is not None:
            print(f'FOUND:{r}')
        else:
            print('NOT_FOUND')
except Exception as e:
    print(f'ERROR:{e}')
" 2>/dev/null || echo "ERROR")

    if echo "$has_retryable" | grep -q "^FOUND"; then
        pass "Error response includes retryable field. $has_retryable"
    elif echo "$text" | grep -qi "extension.*not.*connected\|no.*data\|not.*connected"; then
        # The error message indicates extension disconnect — check retryable in full response JSON
        local full_retryable
        full_retryable=$(echo "$response" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
result = data.get('result', {})
if isinstance(result, str):
    result = json.loads(result) if result.startswith('{') else {}
content = result.get('content', [])
for c in content:
    t = c.get('text', '')
    try:
        d = json.loads(t[t.find('{'):]) if '{' in t else {}
        if 'retryable' in d:
            print(f'FOUND:{d[\"retryable\"]}')
            break
    except: pass
else:
    print('NOT_FOUND')
" 2>/dev/null || echo "NOT_FOUND")
        if echo "$full_retryable" | grep -q "^FOUND"; then
            pass "Extension-disconnect error includes retryable field. $full_retryable"
        else
            fail "Error indicates extension disconnect but missing retryable field. Content: $(truncate "$text" 200)"
        fi
    else
        # Extension might have reconnected fast enough — skip rather than fail
        skip "Extension reconnected before test could capture disconnect error. Content: $(truncate "$text" 100)"
    fi
}
run_test_24_1

# ── Test 24.2: Structured error format includes retryable ──
begin_test "24.2" "[DAEMON ONLY] Structured errors always include retryable field" \
    "Call a tool with invalid params to trigger a structured error, verify retryable is present" \
    "Tests: contract enforcement — all structured errors carry retryable (#151)"

run_test_24_2() {
    # Call interact with missing required fields to trigger an error
    local response
    response=$(call_tool "interact" '{}')

    if [ -z "$response" ]; then
        fail "No response from interact with empty params."
        return
    fi

    log_diagnostic "24.2" "structured-error" "$response"

    local text
    text=$(extract_content_text "$response")

    # The response should be an error with retryable field
    if echo "$text" | grep -q "retryable"; then
        pass "Structured error includes retryable field."
    elif echo "$response" | grep -q "retryable"; then
        pass "Retryable field present in response JSON."
    else
        fail "Structured error missing retryable field. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_2

# Test 24.1 kills and restarts the daemon, which disconnects the extension.
# Wait for the extension to reconnect so subsequent tests aren't poisoned.
if type wait_for_extension &>/dev/null; then
    wait_for_extension 10 || true
fi
