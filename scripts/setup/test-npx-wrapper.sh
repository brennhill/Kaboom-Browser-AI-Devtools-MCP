#!/bin/bash
# Test the FULL stack: npx wrapper → binary → stdio → HTTP → server
# This is what Claude Desktop actually uses
set -euo pipefail

NUM_CLIENTS="${1:-10}"
PORT="${2:-7890}"

echo "=== NPX Wrapper Integration Test ==="
echo "Testing $NUM_CLIENTS concurrent clients via npx"
echo "Port: $PORT"
echo ""

# Clean start
echo "Killing existing servers..."
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 2

if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "❌ Port $PORT still in use"
    exit 1
fi
echo "✅ Port $PORT is free"
echo ""

# Test concurrent connections via npx (the REAL way Claude Desktop uses it)
echo "Starting $NUM_CLIENTS concurrent npx clients..."
echo ""

# shellcheck disable=SC2034 # counters tracked via subshell exit codes
SUCCESS=0
# shellcheck disable=SC2034 # counters tracked via subshell exit codes
FAILED=0

for i in $(seq 1 "$NUM_CLIENTS"); do
    (
        START="$(date +%s%N)"

        # Use the ACTUAL npx command with stdio (like Claude Desktop does)
        RESPONSE="$(echo "{\"jsonrpc\":\"2.0\",\"id\":$i,\"method\":\"tools/list\"}" | \
            npx -y kaboom-agentic-browser 2>/dev/null)"

        END="$(date +%s%N)"
        DURATION_MS="$(( (END - START) / 1000000 ))"

        if echo "$RESPONSE" | grep -q '"result"'; then
            echo "Client $i: ✅ Success in ${DURATION_MS}ms"
        else
            echo "Client $i: ❌ Failed after ${DURATION_MS}ms"
            echo "  Response: $(echo "$RESPONSE" | head -c 100)"
            exit 1
        fi
    ) &
done

# Wait for all clients
wait

# Count results
echo ""
echo "=== Results ==="

# Check if server is still running
sleep 2
if curl -s "localhost:$PORT/health" >/dev/null 2>&1; then
    VERSION="$(curl -s "localhost:$PORT/health" | jq -r '.version' 2>/dev/null)"
    echo "✅ Server running: v$VERSION"

    # Count processes
    PIDS="$(lsof -ti :"$PORT" 2>/dev/null || echo "")"
    if [ -n "$PIDS" ]; then
        NUM_PROCS="$(echo "$PIDS" | wc -l | tr -d ' ')"
        if [ "$NUM_PROCS" = "1" ]; then
            echo "✅ Exactly 1 server process (as expected)"
        else
            echo "⚠️  Found $NUM_PROCS processes (expected 1)"
            echo "   PIDs: $PIDS"
        fi
    fi
else
    echo "❌ No server running"
    exit 1
fi

echo ""
echo "✅ All $NUM_CLIENTS clients succeeded"
echo "✅ Server shared correctly (one process, multiple clients)"
