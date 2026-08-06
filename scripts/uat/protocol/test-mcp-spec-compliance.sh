#!/bin/bash
set -euo pipefail

# MCP Spec Compliance Test Suite
# Validates all JSON-RPC responses match the MCP specification
# Reference: https://spec.modelcontextprotocol.io/specification/

PORT=$((8000 + RANDOM % 1000))
WRAPPER="kaboom-agentic-browser"
TEMP_DIR=$(mktemp -d)
PASS=0
FAIL=0

echo "========================================"
echo "MCP Spec Compliance Test Suite"
echo "========================================"
echo ""
echo "Port: $PORT"
echo "Temp: $TEMP_DIR"
echo ""

# Kill any existing server
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

# Helper: Send request and get response
send_request() {
    local request="$1"
    local output_file="$TEMP_DIR/response.json"

    (echo "$request"; sleep 0.3) | "$WRAPPER" --port "$PORT" > "$output_file" 2>/dev/null
    cat "$output_file"
}

# Helper: Validate JSON-RPC response structure
validate_response() {
    local test_name="$1"
    local response="$2"
    local expect_result="$3"  # "result", "error", or "none"
    local request_id="$4"

    # Check it's valid JSON
    if ! echo "$response" | jq . >/dev/null 2>&1; then
        echo "  ❌ $test_name: Invalid JSON"
        FAIL=$((FAIL + 1))
        return 1
    fi

    # For notifications, expect empty response
    if [ "$expect_result" = "none" ]; then
        if [ -z "$response" ] || [ "$response" = "" ]; then
            echo "  ✅ $test_name: No response (correct for notification)"
            PASS=$((PASS + 1))
            return 0
        else
            echo "  ❌ $test_name: Got response for notification (should be empty)"
            echo "       Response: $response"
            FAIL=$((FAIL + 1))
            return 1
        fi
    fi

    # Check jsonrpc version
    local jsonrpc
    jsonrpc=$(echo "$response" | jq -r '.jsonrpc')
    if [ "$jsonrpc" != "2.0" ]; then
        echo "  ❌ $test_name: jsonrpc must be '2.0', got '$jsonrpc'"
        FAIL=$((FAIL + 1))
        return 1
    fi

    # Check id matches request id
    local resp_id
    resp_id=$(echo "$response" | jq -r '.id')
    if [ "$resp_id" = "null" ]; then
        echo "  ❌ $test_name: id must not be null"
        FAIL=$((FAIL + 1))
        return 1
    fi
    if [ "$resp_id" != "$request_id" ]; then
        echo "  ❌ $test_name: id mismatch (expected $request_id, got $resp_id)"
        FAIL=$((FAIL + 1))
        return 1
    fi

    # Check result or error (mutually exclusive)
    local has_result
    has_result=$(echo "$response" | jq 'has("result")')
    local has_error
    has_error=$(echo "$response" | jq 'has("error")')

    if [ "$has_result" = "true" ] && [ "$has_error" = "true" ]; then
        echo "  ❌ $test_name: Response has both 'result' and 'error' (invalid)"
        FAIL=$((FAIL + 1))
        return 1
    fi

    if [ "$expect_result" = "result" ]; then
        if [ "$has_result" != "true" ]; then
            echo "  ❌ $test_name: Expected 'result' but got 'error'"
            echo "       Error: $(echo "$response" | jq -r '.error.message')"
            FAIL=$((FAIL + 1))
            return 1
        fi
    elif [ "$expect_result" = "error" ]; then
        if [ "$has_error" != "true" ]; then
            echo "  ❌ $test_name: Expected 'error' but got 'result'"
            FAIL=$((FAIL + 1))
            return 1
        fi
        # Validate error structure
        local error_code
        error_code=$(echo "$response" | jq -r '.error.code')
        local error_message
        error_message=$(echo "$response" | jq -r '.error.message')
        if [ "$error_code" = "null" ] || [ -z "$error_message" ]; then
            echo "  ❌ $test_name: Error must have 'code' and 'message'"
            FAIL=$((FAIL + 1))
            return 1
        fi
    fi

    echo "  ✅ $test_name"
    PASS=$((PASS + 1))
    return 0
}

echo "=== Test 1: initialize ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')
validate_response "initialize returns result" "$RESP" "result" "1"

# Validate initialize result structure
PROTOCOL_VERSION=$(echo "$RESP" | jq -r '.result.protocolVersion')
SERVER_NAME=$(echo "$RESP" | jq -r '.result.serverInfo.name')
HAS_CAPABILITIES=$(echo "$RESP" | jq '.result | has("capabilities")')

if [ "$PROTOCOL_VERSION" != "null" ] && [ -n "$PROTOCOL_VERSION" ]; then
    echo "  ✅ initialize: has protocolVersion ($PROTOCOL_VERSION)"
    PASS=$((PASS + 1))
else
    echo "  ❌ initialize: missing protocolVersion"
    FAIL=$((FAIL + 1))
fi

if [ "$SERVER_NAME" != "null" ] && [ -n "$SERVER_NAME" ]; then
    echo "  ✅ initialize: has serverInfo.name ($SERVER_NAME)"
    PASS=$((PASS + 1))
else
    echo "  ❌ initialize: missing serverInfo.name"
    FAIL=$((FAIL + 1))
fi

if [ "$HAS_CAPABILITIES" = "true" ]; then
    echo "  ✅ initialize: has capabilities object"
    PASS=$((PASS + 1))
else
    echo "  ❌ initialize: missing capabilities"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 2: notifications/initialized (notification - no response expected) ==="
echo ""
# For notifications, we need to check that no response comes back
# Send notification and check output is empty or just whitespace
NOTIF_OUTPUT="$TEMP_DIR/notif_output.txt"
(echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'; sleep 0.3) | "$WRAPPER" --port "$PORT" > "$NOTIF_OUTPUT" 2>/dev/null

# Check if output contains any JSON-RPC response
if grep -q '"jsonrpc"' "$NOTIF_OUTPUT" 2>/dev/null; then
    # There's a response - check if it has id:null (bad) or is something else
    NOTIF_ID=$(grep '"jsonrpc"' < "$NOTIF_OUTPUT" | head -1 | jq -r '.id' 2>/dev/null)
    if [ "$NOTIF_ID" = "null" ]; then
        echo "  ❌ notifications/initialized: Got response with id:null (should be no response)"
        FAIL=$((FAIL + 1))
    else
        echo "  ⚠️  notifications/initialized: Got unexpected response (id=$NOTIF_ID)"
        FAIL=$((FAIL + 1))
    fi
else
    echo "  ✅ notifications/initialized: No response (correct)"
    PASS=$((PASS + 1))
fi

echo ""
echo "=== Test 3: tools/list ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}')
validate_response "tools/list returns result" "$RESP" "result" "2"

# Validate tools structure
TOOLS_ARRAY=$(echo "$RESP" | jq '.result.tools')
if [ "$TOOLS_ARRAY" != "null" ]; then
    TOOL_COUNT=$(echo "$RESP" | jq '.result.tools | length')
    echo "  ✅ tools/list: has tools array ($TOOL_COUNT tools)"
    PASS=$((PASS + 1))

    # Check first tool has required fields
    FIRST_TOOL_NAME=$(echo "$RESP" | jq -r '.result.tools[0].name')
    _FIRST_TOOL_DESC=$(echo "$RESP" | jq -r '.result.tools[0].description')
    HAS_INPUT_SCHEMA=$(echo "$RESP" | jq '.result.tools[0] | has("inputSchema")')

    if [ "$FIRST_TOOL_NAME" != "null" ] && [ -n "$FIRST_TOOL_NAME" ]; then
        echo "  ✅ tools/list: tool has name ($FIRST_TOOL_NAME)"
        PASS=$((PASS + 1))
    else
        echo "  ❌ tools/list: tool missing name"
        FAIL=$((FAIL + 1))
    fi

    if [ "$HAS_INPUT_SCHEMA" = "true" ]; then
        echo "  ✅ tools/list: tool has inputSchema"
        PASS=$((PASS + 1))
    else
        echo "  ❌ tools/list: tool missing inputSchema"
        FAIL=$((FAIL + 1))
    fi

    # Check NO _meta field (not in MCP spec)
    HAS_META=$(echo "$RESP" | jq '.result.tools[0] | has("_meta")')
    if [ "$HAS_META" = "false" ]; then
        echo "  ✅ tools/list: tool has no _meta field (correct)"
        PASS=$((PASS + 1))
    else
        echo "  ❌ tools/list: tool has _meta field (not in MCP spec)"
        FAIL=$((FAIL + 1))
    fi
else
    echo "  ❌ tools/list: missing tools array"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 4: resources/list ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}')
validate_response "resources/list returns result" "$RESP" "result" "3"

# Validate resources structure
HAS_RESOURCES=$(echo "$RESP" | jq '.result | has("resources")')
if [ "$HAS_RESOURCES" = "true" ]; then
    RESOURCE_COUNT=$(echo "$RESP" | jq '.result.resources | length')
    echo "  ✅ resources/list: has resources array ($RESOURCE_COUNT resources)"
    PASS=$((PASS + 1))
else
    echo "  ❌ resources/list: missing resources array"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 5: prompts/list ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}')
validate_response "prompts/list returns result" "$RESP" "result" "4"

# Validate prompts structure
HAS_PROMPTS=$(echo "$RESP" | jq '.result | has("prompts")')
if [ "$HAS_PROMPTS" = "true" ]; then
    PROMPT_COUNT=$(echo "$RESP" | jq '.result.prompts | length')
    echo "  ✅ prompts/list: has prompts array ($PROMPT_COUNT prompts)"
    PASS=$((PASS + 1))
else
    echo "  ❌ prompts/list: missing prompts array"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 6: ping ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":5,"method":"ping","params":{}}')
validate_response "ping returns result" "$RESP" "result" "5"

echo ""
echo "=== Test 7: tools/call (observe) ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}')
# This might return error if no browser extension, but should still be valid JSON-RPC
HAS_RESULT=$(echo "$RESP" | jq 'has("result")')
HAS_ERROR=$(echo "$RESP" | jq 'has("error")')
if [ "$HAS_RESULT" = "true" ] || [ "$HAS_ERROR" = "true" ]; then
    validate_response "tools/call returns result or error" "$RESP" "result" "6" || \
    validate_response "tools/call returns result or error" "$RESP" "error" "6"
else
    echo "  ❌ tools/call: response has neither result nor error"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 8: Unknown method ==="
echo ""
RESP=$(send_request '{"jsonrpc":"2.0","id":7,"method":"unknown/method","params":{}}')
validate_response "unknown method returns error" "$RESP" "error" "7"

# Check error code is -32601 (Method not found)
ERROR_CODE=$(echo "$RESP" | jq -r '.error.code')
if [ "$ERROR_CODE" = "-32601" ]; then
    echo "  ✅ unknown method: error code is -32601 (Method not found)"
    PASS=$((PASS + 1))
else
    echo "  ❌ unknown method: expected error code -32601, got $ERROR_CODE"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 9: Parse error (malformed JSON) ==="
echo ""
RESP=$(send_request 'not valid json at all')
# Should get parse error
HAS_ERROR=$(echo "$RESP" | jq 'has("error")' 2>/dev/null || echo "false")
if [ "$HAS_ERROR" = "true" ]; then
    ERROR_CODE=$(echo "$RESP" | jq -r '.error.code')
    if [ "$ERROR_CODE" = "-32700" ]; then
        echo "  ✅ parse error: error code is -32700 (Parse error)"
        PASS=$((PASS + 1))
    else
        echo "  ❌ parse error: expected error code -32700, got $ERROR_CODE"
        FAIL=$((FAIL + 1))
    fi
    # For parse errors, JSON-RPC requires id to be null
    PARSE_ERROR_ID=$(echo "$RESP" | jq -r '.id')
    if [ "$PARSE_ERROR_ID" = "null" ]; then
        echo "  ✅ parse error: id is null (correct)"
        PASS=$((PASS + 1))
    else
        echo "  ❌ parse error: id should be null, got '$PARSE_ERROR_ID'"
        FAIL=$((FAIL + 1))
    fi
else
    echo "  ❌ parse error: no error response"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Test 10: ID types (string and number) ==="
echo ""

# String ID
RESP=$(send_request '{"jsonrpc":"2.0","id":"string-id-test","method":"ping","params":{}}')
STRING_ID=$(echo "$RESP" | jq -r '.id')
if [ "$STRING_ID" = "string-id-test" ]; then
    echo "  ✅ string id: correctly echoed back"
    PASS=$((PASS + 1))
else
    echo "  ❌ string id: expected 'string-id-test', got '$STRING_ID'"
    FAIL=$((FAIL + 1))
fi

# Number ID
RESP=$(send_request '{"jsonrpc":"2.0","id":12345,"method":"ping","params":{}}')
NUMBER_ID=$(echo "$RESP" | jq -r '.id')
if [ "$NUMBER_ID" = "12345" ]; then
    echo "  ✅ number id: correctly echoed back"
    PASS=$((PASS + 1))
else
    echo "  ❌ number id: expected '12345', got '$NUMBER_ID'"
    FAIL=$((FAIL + 1))
fi

echo ""

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

# Summary
echo "========================================"
echo "SUMMARY"
echo "========================================"
echo ""
echo "  ✅ Passed: $PASS"
echo "  ❌ Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
    echo "🎉 ALL TESTS PASSED - MCP Spec Compliant!"
    exit 0
else
    echo "⚠️  SOME TESTS FAILED - Review above for details"
    exit 1
fi
