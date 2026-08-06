#!/bin/bash
set -euo pipefail

# Simulates exactly what Cursor does during connection
# to reproduce the RED connection issue

PORT=7890
WRAPPER="kaboom-agentic-browser"
TEMP_DIR=$(mktemp -d)

echo "========================================"
echo "Cursor Connection Simulation"
echo "========================================"
echo ""

# Kill any existing server
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 1

echo "Step 1: Cursor spawns kaboom-agentic-browser"
echo "===================================="

# Simulate Cursor's connection sequence
OUTPUT_FILE="$TEMP_DIR/cursor_output.txt"

# Send the exact sequence Cursor would send
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"cursor","version":"0.43.6"}}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}'
  sleep 2
) | "$WRAPPER" --port "$PORT" > "$OUTPUT_FILE" 2>&1 &

CURSOR_PID=$!
sleep 5

# Kill the simulated cursor
kill "$CURSOR_PID" 2>/dev/null || true
wait "$CURSOR_PID" 2>/dev/null || true

echo ""
echo "Step 2: Analyze responses"
echo "========================="

# Count messages
MSG_COUNT=$(grep -c '{"jsonrpc' "$OUTPUT_FILE" 2>/dev/null || echo "0")
echo "JSON-RPC messages received: $MSG_COUNT"

# Show all responses
echo ""
echo "=== All Responses ==="
grep '{"jsonrpc' < "$OUTPUT_FILE" | jq -c '{id, has_result: (.result != null), has_error: (.error != null)}' 2>&1 || jq -c . < "$OUTPUT_FILE"
echo ""

# Check for parse errors
PARSE_ERRORS=$(grep -ci "parse error\|unexpected\|invalid" "$OUTPUT_FILE" 2>/dev/null || echo "0")
if [ "$PARSE_ERRORS" -gt 0 ]; then
    echo "⚠️  Found $PARSE_ERRORS potential issues:"
    grep -i "parse error\|unexpected\|invalid" "$OUTPUT_FILE" | head -5
else
    echo "✅ No parse errors"
fi

echo ""

# Check stderr silence
STDERR_LINES=$(grep -c '^\[' "$OUTPUT_FILE" 2>/dev/null || echo "0")
echo "Stderr lines: $STDERR_LINES"
if [ "$STDERR_LINES" -eq 0 ]; then
    echo "✅ Stdio silence maintained"
else
    echo "❌ Found stderr noise:"
    grep '^\[' "$OUTPUT_FILE"
fi

echo ""

# Check if server is still running
if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "✅ Server still running on port $PORT"

    # Check health
    HEALTH=$(curl -s "http://localhost:$PORT/health" 2>/dev/null || echo "{}")
    VERSION=$(echo "$HEALTH" | jq -r '.version' 2>/dev/null || echo "unknown")
    echo "   Version: $VERSION"
else
    echo "❌ Server not running"
fi

echo ""

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

echo "========================================"
echo "Simulation Complete"
echo "========================================"
