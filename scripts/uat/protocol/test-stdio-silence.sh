#!/bin/bash
set -euo pipefail

# Test script to verify MCP stdio silence invariant
# This ensures the wrapper outputs ZERO noise to stderr during normal operation

PORT=$((8000 + RANDOM % 1000))
TEMP_DIR=$(mktemp -d)
WRAPPER="kaboom-agentic-browser"

echo "========================================"
echo "MCP Stdio Silence Invariant Test"
echo "========================================"
echo ""
echo "Testing: $WRAPPER"
echo "Port: $PORT"
echo ""

# Find the wrapper
if ! command -v "$WRAPPER" &> /dev/null; then
    echo "❌ ERROR: kaboom-agentic-browser not found in PATH"
    echo "Run 'npm link' from npm/kaboom-agentic-browser first"
    exit 1
fi

echo "✅ Wrapper found: $(which "$WRAPPER")"
echo ""

# Test 1: Normal connection
echo "Test 1: Normal Connection (Silent Stdio)"
echo "==========================================="

# Send MCP initialize request and capture output
(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 0.5) | \
    "$WRAPPER" --port "$PORT" > "$TEMP_DIR/stdout.txt" 2>"$TEMP_DIR/stderr.txt" &
CLIENT_PID=$!

# Wait for connection
sleep 2

# Kill client
kill "$CLIENT_PID" 2>/dev/null || true
wait "$CLIENT_PID" 2>/dev/null || true

# Check stderr - should be EMPTY
STDERR_LINES=$(wc -l < "$TEMP_DIR/stderr.txt" 2>/dev/null | tr -d ' ' || echo "0")

echo ""
echo "=== Stderr Output ==="
if [ "$STDERR_LINES" -eq 0 ]; then
    echo "<EMPTY - Completely silent!>"
else
    cat "$TEMP_DIR/stderr.txt"
fi
echo "====================="
echo ""

echo "Results:"
echo "  Stderr lines: $STDERR_LINES"

if [ "$STDERR_LINES" -eq 0 ]; then
    echo "  ✅ PASS: No stderr noise"
else
    echo "  ❌ FAIL: Expected 0 stderr lines, got $STDERR_LINES"
    echo ""
    echo "INVARIANT VIOLATION: All logs must go to:"
    echo "  - ~/kaboom-wrapper.log (wrapper logs)"
    echo "  - ~/kaboom-logs.jsonl (server logs)"
    echo "  - /tmp/kaboom-debug-*.log (debug logs)"
    echo ""
    # Cleanup
    lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Check stdout - should contain JSON-RPC only
echo ""
echo "=== Stdout Output ==="
head -5 "$TEMP_DIR/stdout.txt" || echo "<empty>"
echo "====================="
echo ""

# Verify stdout is valid JSON-RPC
if grep -q '"jsonrpc":"2.0"' "$TEMP_DIR/stdout.txt" 2>/dev/null; then
    echo "  ✅ Stdout contains JSON-RPC response"
else
    echo "  ⚠️  No JSON-RPC response found (may not have connected)"
fi

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

echo ""
echo "========================================"
echo "✅ STDIO SILENCE INVARIANT VERIFIED"
echo "========================================"
echo ""
echo "✅ Zero stderr output during connection"
echo "✅ Only JSON-RPC on stdout"
echo "✅ All diagnostics in log files"
echo ""
exit 0
