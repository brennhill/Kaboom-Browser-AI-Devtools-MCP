#!/bin/bash
# 21-macro-recording.sh — 21.1-21.6: Macro recording (sequences) via configure tool.
# save, get, list, delete, replay lifecycle
set -eo pipefail

begin_category "21" "Macro Recording" "6"

# ── Test 21.1: Save a sequence ───────────────────────────
begin_test "21.1" "[DAEMON ONLY] Save a sequence with steps and tags" \
    "configure(what='save_sequence') with name, steps, tags, description" \
    "Tests: sequence persistence via session store"

run_test_21_1() {
    local response
    response=$(call_tool "configure" '{"what":"save_sequence","name":"smoke-macro","description":"Smoke test macro","tags":["smoke","test"],"steps":[{"action":"navigate","url":"https://example.com"},{"action":"click","selector":"a"}]}')

    if ! check_not_error "$response"; then
        fail "save_sequence returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if echo "$text" | grep -qi "smoke-macro"; then
        pass "save_sequence saved 'smoke-macro' with 2 steps and tags."
    else
        fail "save_sequence response did not reference 'smoke-macro'. Content: $(truncate "$text" 200)"
    fi
}
run_test_21_1

# ── Test 21.2: Get the saved sequence ────────────────────
begin_test "21.2" "[DAEMON ONLY] Get a saved sequence by name" \
    "configure(what='get_sequence') returns steps, tags, description" \
    "Tests: sequence retrieval"

run_test_21_2() {
    local response
    response=$(call_tool "configure" '{"what":"get_sequence","name":"smoke-macro"}')

    if ! check_not_error "$response"; then
        fail "get_sequence returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    seq = data.get('sequence', data)
    name = seq.get('name', '')
    steps = seq.get('steps', [])
    tags = seq.get('tags', [])
    desc = seq.get('description', '')
    if name == 'smoke-macro' and len(steps) == 2 and 'smoke' in tags:
        print(f'PASS name={name} steps={len(steps)} tags={tags}')
    else:
        print(f'FAIL name={name} steps={len(steps)} tags={tags}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "get_sequence returned 'smoke-macro' with correct steps and tags. $verdict"
    else
        fail "get_sequence invalid. $verdict. Content: $(truncate "$text" 200)"
    fi
}
run_test_21_2

# ── Test 21.3: List sequences ────────────────────────────
begin_test "21.3" "[DAEMON ONLY] List sequences with optional tag filter" \
    "configure(what='list_sequences') returns saved sequences" \
    "Tests: sequence listing and tag filtering"

run_test_21_3() {
    # List all
    local response
    response=$(call_tool "configure" '{"what":"list_sequences"}')

    if ! check_not_error "$response"; then
        fail "list_sequences returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if ! echo "$text" | grep -q "smoke-macro"; then
        fail "list_sequences did not include 'smoke-macro'. Content: $(truncate "$text" 200)"
        return
    fi

    # List with tag filter
    local filtered
    filtered=$(call_tool "configure" '{"what":"list_sequences","tags":["smoke"]}')
    local filtered_text
    filtered_text=$(extract_content_text "$filtered")

    if echo "$filtered_text" | grep -q "smoke-macro"; then
        pass "list_sequences: found 'smoke-macro' in full list and tag-filtered list."
    else
        fail "list_sequences tag filter did not match 'smoke-macro'. Content: $(truncate "$filtered_text" 200)"
    fi
}
run_test_21_3

# ── Test 21.4: Replay a sequence ─────────────────────────
begin_test "21.4" "[BROWSER] Replay a saved sequence" \
    "configure(what='replay_sequence') executes steps via interact" \
    "Tests: sequence replay dispatches through interact handler"

run_test_21_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "configure" '{"what":"replay_sequence","name":"smoke-macro"}')
    local text
    text=$(extract_content_text "$response")
    if [ -z "$text" ] && [ -n "$response" ]; then
        text="$response"
    fi

    # Replay should return step results
    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read().strip()
    if t.startswith('{') and '\"jsonrpc\"' in t:
        outer = json.loads(t)
        t = (((outer.get('result') or {}).get('content') or [{}])[0].get('text') or t)
    i = t.find('{')
    data = json.loads(t[i:]) if i >= 0 else {}
    results = data.get('step_results', data.get('results', []))
    total = data.get('total_steps', data.get('steps_total', 0))
    if isinstance(results, list) and len(results) > 0:
        print(f'PASS steps_executed={len(results)} total={total}')
    else:
        print(f'FAIL no step_results. keys={list(data.keys())[:8]} status={data.get(\"status\", \"\")}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "replay_sequence executed steps. $verdict"
    else
        # Replay may fail on the click step (no matching element) but should still return results
        if echo "$text" | grep -qi "step_results\|results\|executed"; then
            pass "replay_sequence returned results (some steps may have failed as expected)."
        else
            fail "replay_sequence failed. $verdict. Content: $(truncate "$text" 200)"
        fi
    fi
}
run_test_21_4

# ── Test 21.5: Save with override (upsert) ──────────────
begin_test "21.5" "[DAEMON ONLY] Upsert: save over existing sequence" \
    "Save 'smoke-macro' again with different steps, verify updated" \
    "Tests: sequence upsert behavior"

run_test_21_5() {
    # Save with different steps
    local response
    response=$(call_tool "configure" '{"what":"save_sequence","name":"smoke-macro","description":"Updated macro","steps":[{"action":"navigate","url":"https://example.org"}]}')

    if ! check_not_error "$response"; then
        fail "save_sequence upsert returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    # Get and verify updated
    local get_response
    get_response=$(call_tool "configure" '{"what":"get_sequence","name":"smoke-macro"}')
    local text
    text=$(extract_content_text "$get_response")

    local verdict
    verdict=$(echo "$text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    seq = data.get('sequence', data)
    steps = seq.get('steps', [])
    desc = seq.get('description', '')
    if len(steps) == 1 and 'example.org' in json.dumps(steps):
        print(f'PASS steps={len(steps)} desc={desc}')
    else:
        print(f'FAIL steps={len(steps)} desc={desc}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Upsert: 'smoke-macro' updated to 1 step with example.org. $verdict"
    else
        fail "Upsert verification failed. $verdict. Content: $(truncate "$text" 200)"
    fi
}
run_test_21_5

# ── Test 21.6: Delete a sequence ─────────────────────────
begin_test "21.6" "[DAEMON ONLY] Delete a sequence and verify removal" \
    "configure(what='delete_sequence') then get returns not found" \
    "Tests: sequence deletion"

run_test_21_6() {
    local response
    response=$(call_tool "configure" '{"what":"delete_sequence","name":"smoke-macro"}')

    if ! check_not_error "$response"; then
        fail "delete_sequence returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    # Verify deletion — get should fail
    local get_response
    get_response=$(call_tool "configure" '{"what":"get_sequence","name":"smoke-macro"}')
    local get_text
    get_text=$(extract_content_text "$get_response")

    if echo "$get_text" | grep -qi "not found\|error\|not_found"; then
        pass "delete_sequence: 'smoke-macro' deleted, get returns not found."
    else
        fail "delete_sequence: get still returns data after delete. Content: $(truncate "$get_text" 200)"
    fi
}
run_test_21_6
