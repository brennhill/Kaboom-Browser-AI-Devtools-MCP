#!/bin/bash
# cat-01-protocol.sh — MCP JSON-RPC 2.0 protocol compliance tests.
# Tests the fast-start bridge: initialize, tools/list, ID matching,
# stdout purity, error codes. No daemon needed for these.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"

begin_category "1" "Protocol Compliance" "7"

# Kill any daemon so we're testing fast-start bridge only
kill_server

# ── Test 1.1: Initialize returns capabilities ─────────────
begin_test "1.1" "Initialize returns capabilities" \
    "Send initialize request, verify capabilities and version" \
    "If capabilities are wrong, Claude/Cursor won't know what tools exist"

run_test_1_1() {
    local request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"uat","version":"1.0"}}}'
    local response
    response=$(send_mcp "$request" "init")

    if [ -z "$response" ]; then
        fail "No response received from initialize request."
        return
    fi

    # Check serverInfo.name
    local server_name
    server_name=$(echo "$response" | jq -r '.result.serverInfo.name // empty' 2>/dev/null)
    if [ -z "$server_name" ]; then
        fail "result.serverInfo.name missing. Response: $(truncate "$response")"
        return
    fi

    # Check capabilities.tools
    if ! check_json_has "$response" '.result.capabilities.tools'; then
        fail "result.capabilities.tools missing. Response: $(truncate "$response")"
        return
    fi

    # Check version matches VERSION file
    local server_version
    server_version=$(echo "$response" | jq -r '.result.serverInfo.version // empty' 2>/dev/null)
    if [ "$server_version" != "$VERSION" ]; then
        fail "Version mismatch: serverInfo.version='$server_version', VERSION file='$VERSION'. Response: $(truncate "$response")"
        return
    fi

    pass "Sent initialize, received capabilities. serverInfo.name='$server_name', version='$server_version' (matches VERSION file), capabilities.tools present."
}
run_test_1_1

# ── Test 1.2: tools/list returns exactly 4 tools ──────────
begin_test "1.2" "tools/list returns exactly 4 tools" \
    "Send tools/list, verify exactly observe/generate/configure/interact" \
    "Exact-match means no extras sneak in (we shipped stub tools before)"

run_test_1_2() {
    local request='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
    local response
    response=$(send_mcp "$request" "tools_list")

    if [ -z "$response" ]; then
        fail "No response received from tools/list."
        return
    fi

    # Count tools
    local tool_count
    tool_count=$(echo "$response" | jq -r '.result.tools | length' 2>/dev/null)
    if [ "$tool_count" != "5" ]; then
        fail "Expected exactly 5 tools, got $tool_count. Response: $(truncate "$response")"
        return
    fi

    # Extract sorted tool names
    local tool_names
    tool_names=$(echo "$response" | jq -r '.result.tools[].name' 2>/dev/null | sort | tr '\n' ',' | sed 's/,$//')
    local expected="analyze,configure,generate,interact,observe"
    if [ "$tool_names" != "$expected" ]; then
        fail "Expected tools [$expected], got [$tool_names]. Response: $(truncate "$response")"
        return
    fi

    pass "tools/list returned exactly 5 tools: $tool_names."
}
run_test_1_2

# ── Test 1.3: tools/list schema shapes are valid ──────────
begin_test "1.3" "tools/list schema shapes are valid" \
    "For each tool, verify inputSchema.type=='object' and required fields" \
    "Schema errors mean MCP clients send wrong params"

run_test_1_3() {
    local request='{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}'
    local response
    response=$(send_mcp "$request" "tools_schema")

    if [ -z "$response" ]; then
        fail "No response received from tools/list."
        return
    fi

    # Current schema uses unified `what` across all tools. (No associative array:
    # `declare -A` is bash 4+, and macOS — the primary platform — ships bash 3.2.)
    local expected_required_param="what"

    local verified=0
    local errors=""

    for tool_name in observe analyze generate configure interact; do
        local tool_def
        tool_def=$(echo "$response" | jq -c ".result.tools[] | select(.name == \"$tool_name\")" 2>/dev/null)

        if [ -z "$tool_def" ]; then
            errors="${errors}Tool '$tool_name' not found in response. "
            continue
        fi

        # Check inputSchema.type == "object"
        local schema_type
        schema_type=$(echo "$tool_def" | jq -r '.inputSchema.type // empty' 2>/dev/null)
        if [ "$schema_type" != "object" ]; then
            errors="${errors}$tool_name: inputSchema.type='$schema_type', expected 'object'. "
            continue
        fi

        # Check required field contains expected param
        local req_param="$expected_required_param"
        if ! echo "$tool_def" | jq -e ".inputSchema.required | index(\"$req_param\")" >/dev/null 2>&1; then
            errors="${errors}$tool_name: required field does not contain '$req_param'. "
            continue
        fi

        verified=$((verified + 1))
    done

    if [ -n "$errors" ]; then
        fail "Schema validation errors: $errors Response: $(truncate "$response")"
        return
    fi

    pass "All 5 tool schemas valid: inputSchema.type='object' and required includes 'what'."
}
run_test_1_3

# ── Test 1.4: Response IDs match request IDs ──────────────
begin_test "1.4" "Response IDs match request IDs" \
    "Send 3 requests with IDs 101,102,103 in one pipe, verify each response ID matches" \
    "ID mismatch means responses get routed to wrong callers (JSON-RPC 2.0 requirement)"

run_test_1_4() {
    local requests
    requests='{"jsonrpc":"2.0","id":101,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"uat","version":"1.0"}}}
{"jsonrpc":"2.0","id":102,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":103,"method":"tools/list","params":{}}'

    local lines
    lines=$(send_mcp_multi "$requests" "id_match")

    if [ -z "$lines" ]; then
        fail "No responses received from multi-request pipe."
        return
    fi

    local line_count
    line_count=$(echo "$lines" | wc -l | tr -d ' ')
    if [ "$line_count" -lt 3 ]; then
        fail "Expected 3 response lines, got $line_count. Output: $(truncate "$lines")"
        return
    fi

    # Check each expected ID appears
    local missing=""
    for expected_id in 101 102 103; do
        local found
        found=$(echo "$lines" | jq -r "select(.id == $expected_id) | .id" 2>/dev/null | head -1)
        if [ "$found" != "$expected_id" ]; then
            missing="${missing}ID $expected_id not found. "
        fi
    done

    if [ -n "$missing" ]; then
        fail "Response ID mismatch: $missing Output: $(truncate "$lines")"
        return
    fi

    pass "Sent 3 requests with IDs 101, 102, 103. All 3 responses had matching IDs."
}
run_test_1_4

# ── Test 1.5: Stdout purity (only valid JSON-RPC) ─────────
begin_test "1.5" "Stdout purity (only valid JSON-RPC)" \
    "Send initialize + 3 tool calls, verify every stdout line has jsonrpc=='2.0'" \
    "Any non-JSON on stdout breaks the MCP transport — #1 production failure mode"

run_test_1_5() {
    local requests
    requests='{"jsonrpc":"2.0","id":201,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"uat-purity","version":"1.0"}}}
{"jsonrpc":"2.0","id":202,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":203,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}
{"jsonrpc":"2.0","id":204,"method":"tools/call","params":{"name":"configure","arguments":{"action":"health"}}}'

    local stdout_file="$TEMP_DIR/purity_stdout.txt"
    local stderr_file="$TEMP_DIR/purity_stderr.txt"

    echo "$requests" | "$TIMEOUT_CMD" 20 "$WRAPPER" --port "$PORT" > "$stdout_file" 2>"$stderr_file"

    local total_lines=0
    local bad_lines=0
    local bad_content=""

    while IFS= read -r line; do
        [ -z "$line" ] && continue
        total_lines=$((total_lines + 1))
        if ! check_valid_jsonrpc "$line"; then
            bad_lines=$((bad_lines + 1))
            bad_content="${bad_content}Line $total_lines: $(truncate "$line" 100) "
        fi
    done < "$stdout_file"

    if [ "$total_lines" -eq 0 ]; then
        fail "No output on stdout at all. Expected JSON-RPC responses."
        return
    fi

    if [ "$bad_lines" -gt 0 ]; then
        fail "$bad_lines of $total_lines stdout lines were not valid JSON-RPC: $bad_content"
        return
    fi

    pass "All $total_lines stdout lines are valid JSON-RPC 2.0. Zero non-JSON lines."
}
run_test_1_5

# ── Test 1.6: Unknown method returns error -32601 ─────────
begin_test "1.6" "Unknown method returns error -32601" \
    "Send bogus/method, verify error.code == -32601" \
    "Without this, unknown methods silently succeed or crash the server"

run_test_1_6() {
    local request='{"jsonrpc":"2.0","id":99,"method":"bogus/method","params":{}}'
    local response
    response=$(send_mcp "$request" "unknown_method")

    if [ -z "$response" ]; then
        fail "No response received for unknown method request."
        return
    fi

    # Check error code
    if ! check_protocol_error "$response" "-32601"; then
        local actual_code
        actual_code=$(echo "$response" | jq -r '.error.code // empty' 2>/dev/null)
        fail "Expected error.code=-32601, got '$actual_code'. Response: $(truncate "$response")"
        return
    fi

    # Check id matches
    local resp_id
    resp_id=$(echo "$response" | jq -r '.id // empty' 2>/dev/null)
    if [ "$resp_id" != "99" ]; then
        fail "Response ID mismatch: expected 99, got '$resp_id'. Response: $(truncate "$response")"
        return
    fi

    pass "Unknown method 'bogus/method' returned error.code=-32601 with correct id=99."
}
run_test_1_6

# ── Test 1.7: Malformed JSON returns parse error ──────────
begin_test "1.7" "Malformed JSON returns parse error" \
    "Send '{not valid json', verify error.code == -32700" \
    "Malformed input from buggy clients must not crash the server"

run_test_1_7() {
    local request='{not valid json'
    local response
    response=$(send_mcp "$request" "malformed")

    if [ -z "$response" ]; then
        fail "No response received for malformed JSON. Server may have crashed."
        return
    fi

    if ! check_protocol_error "$response" "-32700"; then
        local actual_code
        actual_code=$(echo "$response" | jq -r '.error.code // empty' 2>/dev/null)
        fail "Expected error.code=-32700 (parse error), got '$actual_code'. Response: $(truncate "$response")"
        return
    fi

    pass "Malformed JSON returned error.code=-32700 (parse error). Server did not crash."
}
run_test_1_7

# ── Done ──────────────────────────────────────────────────
finish_category
