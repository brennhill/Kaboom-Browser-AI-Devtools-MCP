#!/bin/bash
set -euo pipefail

# Test that all exit points send JSON-RPC responses
# Verifies fix for "Unexpected end of JSON input" errors

PORT="$((8000 + RANDOM % 1000))"
WRAPPER="kaboom-agentic-browser"
TEMP_DIR="$(mktemp -d)"

echo "========================================"
echo "Exit Gate Test - All Paths Send Response"
echo "========================================"
echo ""
echo "Port: $PORT"
echo ""

# Kill any existing server
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

echo "Test 1: Normal Connection (should get valid response)"
echo "===================================================="

OUTPUT_FILE="$TEMP_DIR/test1.log"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | "$WRAPPER" --port "$PORT" > "$OUTPUT_FILE" 2>&1 &
WRAPPER_PID="$!"

# Wait for response
sleep 2
kill "$WRAPPER_PID" 2>/dev/null || true
wait "$WRAPPER_PID" 2>/dev/null || true

# Check response
if grep -q '{"jsonrpc":"2.0"' "$OUTPUT_FILE" 2>/dev/null; then
    MSG_COUNT="$(grep -c '{"jsonrpc":"2.0"' "$OUTPUT_FILE" || echo "0")"
    echo "  ✅ Got $MSG_COUNT JSON-RPC response(s)"

    # Check for valid result or error
    if grep -q '"result"' "$OUTPUT_FILE"; then
        echo "  ✅ Response has 'result' field"
    elif grep -q '"error"' "$OUTPUT_FILE"; then
        echo "  ⚠️  Response has 'error' field (expected for cold start)"
        grep '"error"' "$OUTPUT_FILE" | jq -r '.error.message' 2>/dev/null | sed 's/^/       /' || true
    fi
else
    echo "  ❌ No JSON-RPC response found"
    echo "  Output:"
    sed 's/^/       /' < "$OUTPUT_FILE"
fi

echo ""

echo "Test 2: Concurrent Clients (10 simultaneous)"
echo "============================================="

SUCCESS_COUNT=0
ERROR_COUNT=0
EMPTY_COUNT=0

for i in {1..10}; do
    LOG_FILE="$TEMP_DIR/concurrent_$i.log"

    # Send initialize request
    (
        echo "{\"jsonrpc\":\"2.0\",\"id\":$i,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"1.0\"}}}"
        sleep 0.3
    ) | "$WRAPPER" --port "$PORT" > "$LOG_FILE" 2>&1 &
done

# Wait for all clients
sleep 3

# Analyze results
for i in {1..10}; do
    LOG_FILE="$TEMP_DIR/concurrent_$i.log"

    if [ ! -s "$LOG_FILE" ]; then
        # Empty file
        EMPTY_COUNT="$((EMPTY_COUNT + 1))"
    elif grep -q '{"jsonrpc":"2.0"' "$LOG_FILE" 2>/dev/null; then
        if grep -q '"result"' "$LOG_FILE"; then
            SUCCESS_COUNT="$((SUCCESS_COUNT + 1))"
        elif grep -q '"error"' "$LOG_FILE"; then
            ERROR_COUNT="$((ERROR_COUNT + 1))"
        fi
    else
        # Non-JSON response
        EMPTY_COUNT="$((EMPTY_COUNT + 1))"
    fi
done

echo "  Results:"
echo "    ✅ Success responses: $SUCCESS_COUNT"
echo "    ⚠️  Error responses:   $ERROR_COUNT"
echo "    ❌ Empty/invalid:      $EMPTY_COUNT"
echo ""

if [ "$EMPTY_COUNT" -eq 0 ]; then
    echo "  ✅ ALL clients got JSON-RPC responses (no empty responses)"
else
    echo "  ❌ Some clients got empty responses"
    echo "  Example empty response:"
    for i in {1..10}; do
        LOG_FILE="$TEMP_DIR/concurrent_$i.log"
        if [ ! -s "$LOG_FILE" ] || ! grep -q '{"jsonrpc":"2.0"' "$LOG_FILE" 2>/dev/null; then
            echo "    Client $i output:"
            sed 's/^/      /' < "$LOG_FILE" || echo "      (empty file)"
            break
        fi
    done
fi

echo ""

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

echo "========================================"
echo "Summary"
echo "========================================"
echo ""

if [ "$EMPTY_COUNT" -eq 0 ]; then
    echo "✅ EXIT GATE WORKING"
    echo "   All clients received JSON-RPC responses"
    echo "   No 'Unexpected end of JSON input' errors possible"
    echo ""
    exit 0
else
    echo "❌ EXIT GATE NOT WORKING"
    echo "   $EMPTY_COUNT/$((SUCCESS_COUNT + ERROR_COUNT + EMPTY_COUNT)) clients got empty responses"
    echo "   Will cause 'Unexpected end of JSON input' errors"
    echo ""
    exit 1
fi
