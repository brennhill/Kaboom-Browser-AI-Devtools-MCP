#!/bin/bash
set -euo pipefail

# Test concurrent clients all starting simultaneously
# Verifies no race conditions cause empty responses

PORT="$((8000 + RANDOM % 1000))"
WRAPPER="kaboom-agentic-browser"
TEMP_DIR="$(mktemp -d)"

echo "========================================"
echo "Concurrent Response Test"
echo "========================================"
echo ""
echo "Port: $PORT"
echo "Testing 10 clients starting simultaneously"
echo ""

# Kill any existing server
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

# Launch all clients simultaneously
PIDS=()
for i in {1..10}; do
    LOG="$TEMP_DIR/client_$i.log"
    STDERR="$TEMP_DIR/client_${i}_stderr.log"

    # Launch in background
    (
        echo "{\"jsonrpc\":\"2.0\",\"id\":$i,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"1.0\"}}}"
        sleep 0.3  # Keep stdin open
    ) | "$WRAPPER" --port "$PORT" > "$LOG" 2> "$STDERR" &

    PIDS+=("$!")
done

echo "Launched 10 clients (PIDs: ${PIDS[*]+"${PIDS[*]}"})"
echo "Waiting for all to complete..."

# Wait for ALL clients to finish
for pid in "${PIDS[@]}"; do
    wait "$pid" 2>/dev/null || true
done

echo "All clients completed"
echo ""

# Analyze results
echo "Results:"
for i in {1..10}; do
    LOG="$TEMP_DIR/client_$i.log"
    STDERR="$TEMP_DIR/client_${i}_stderr.log"
    SIZE="$(wc -c < "$LOG" 2>/dev/null | tr -d ' ')"

    if [ "$SIZE" -gt 0 ]; then
        if grep -q '{"jsonrpc":"2.0"' "$LOG" 2>/dev/null; then
            if grep -q '"result"' "$LOG"; then
                echo "  Client $i: ✅ Success ($SIZE bytes)"
            elif grep -q '"error"' "$LOG"; then
                MSG="$(grep '"error"' "$LOG" | jq -r '.error.message' 2>/dev/null || echo "unknown")"
                echo "  Client $i: ⚠️  Error ($SIZE bytes) - $MSG"
            else
                echo "  Client $i: ⚠️  Response without result/error ($SIZE bytes)"
            fi
        else
            echo "  Client $i: ❌ Non-JSON ($SIZE bytes)"
            head -2 "$LOG" | sed 's/^/       /'
        fi
    else
        echo "  Client $i: ❌ EMPTY"
        if [ -s "$STDERR" ]; then
            echo "       stderr: $(head -1 "$STDERR")"
        fi
    fi
done

echo ""

# Count results
SUCCESS=0
ERROR=0
EMPTY=0
INVALID=0

for i in {1..10}; do
    LOG="$TEMP_DIR/client_$i.log"
    SIZE="$(wc -c < "$LOG" 2>/dev/null | tr -d ' ')"

    if [ "$SIZE" -eq 0 ]; then
        EMPTY="$((EMPTY + 1))"
    elif grep -q '{"jsonrpc":"2.0"' "$LOG" 2>/dev/null; then
        if grep -q '"result"' "$LOG"; then
            SUCCESS="$((SUCCESS + 1))"
        elif grep -q '"error"' "$LOG"; then
            ERROR="$((ERROR + 1))"
        fi
    else
        INVALID="$((INVALID + 1))"
    fi
done

echo "Summary:"
echo "  ✅ Success:       $SUCCESS/10"
echo "  ⚠️  Error:         $ERROR/10"
echo "  ❌ Empty:         $EMPTY/10"
echo "  ❌ Invalid:       $INVALID/10"
echo ""

# Check server still running
if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "  ✅ Server still running after all clients"
else
    echo "  ⚠️  Server not running (may have been killed)"
fi

echo ""

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

if [ "$EMPTY" -eq 0 ] && [ "$INVALID" -eq 0 ]; then
    echo "✅ SUCCESS - All $((SUCCESS + ERROR)) clients got valid JSON-RPC responses"
    echo "   No 'Unexpected end of JSON input' errors possible"
    exit 0
else
    echo "❌ FAILURE - $EMPTY empty, $INVALID invalid out of 10 clients"
    echo "   Will cause 'Unexpected end of JSON input' errors"
    exit 1
fi
