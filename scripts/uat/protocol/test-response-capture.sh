#!/bin/bash
set -euo pipefail

# Test that responses are properly captured by parent process
# Simulates exactly what happens when IDE spawns kaboom-agentic-browser

PORT="$((8000 + RANDOM % 1000))"
WRAPPER="kaboom-agentic-browser"
TEMP_DIR="$(mktemp -d)"

echo "========================================"
echo "Response Capture Test"
echo "========================================"
echo ""
echo "Port: $PORT"
echo ""

# Kill any existing server
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

echo "Test: Send request, capture full output before process exits"
echo "=============================================================="

for i in {1..10}; do
    LOG="$TEMP_DIR/client_$i.log"
    STDERR="$TEMP_DIR/client_${i}_stderr.log"

    # Send request and capture output
    # Important: Don't background, let it complete fully
    (
        echo "{\"jsonrpc\":\"2.0\",\"id\":$i,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"1.0\"}}}"
        sleep 0.2  # Keep stdin open briefly
    ) | "$WRAPPER" --port "$PORT" > "$LOG" 2> "$STDERR"

    # Client has now fully exited
    SIZE="$(wc -c < "$LOG" 2>/dev/null | tr -d ' ')"

    if [ "$SIZE" -gt 0 ]; then
        if grep -q '{"jsonrpc":"2.0"' "$LOG" 2>/dev/null; then
            if grep -q '"result"' "$LOG"; then
                echo "  Client $i: ✅ Success ($SIZE bytes)"
            elif grep -q '"error"' "$LOG"; then
                MSG="$(grep '"error"' "$LOG" | jq -r '.error.message' 2>/dev/null || echo "unknown")"
                echo "  Client $i: ⚠️  Error ($SIZE bytes) - $MSG"
            else
                echo "  Client $i: ⚠️  Response but no result/error ($SIZE bytes)"
            fi
        else
            echo "  Client $i: ❌ Non-JSON response ($SIZE bytes)"
            head -3 "$LOG" | sed 's/^/       /'
        fi
    else
        echo "  Client $i: ❌ EMPTY (process exited without writing)"
        # Check stderr for clues
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

# Cleanup
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
rm -rf "$TEMP_DIR"

if [ "$EMPTY" -eq 0 ] && [ "$INVALID" -eq 0 ]; then
    echo "✅ ALL CLIENTS GOT RESPONSES"
    echo "   No empty responses, no 'Unexpected end of JSON input' possible"
    exit 0
else
    echo "❌ SOME CLIENTS GOT EMPTY/INVALID RESPONSES"
    echo "   This will cause 'Unexpected end of JSON input' errors in IDEs"
    exit 1
fi
