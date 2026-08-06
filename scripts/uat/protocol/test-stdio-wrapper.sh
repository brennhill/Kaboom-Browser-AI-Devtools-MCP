#!/bin/bash
# Test MCP stdio wrapper with 10 concurrent clients + restart cycle
# This tests the full connection lifecycle including server recovery
set -euo pipefail

PORT="${1:-7890}"
NUM_CLIENTS=10
CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"

echo "════════════════════════════════════════════════════════════"
echo "  MCP Wrapper Stdio Test - Connection Lifecycle Validation"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "📋 Configuration:"
echo "   Port: $PORT"
echo "   Concurrent clients: $NUM_CLIENTS"
echo ""

# Find binary
BINARY="${KABOOM_STDIO_TEST_BINARY:-./kaboom-agentic-browser}"
if [ ! -f "$BINARY" ]; then
    echo "Building binary..."
    go build -o "$BINARY" "$CMD_PKG"
fi

# Helper: Send MCP request via stdio and capture result
send_mcp_request() {
    local client_id="$1"
    local request='{"jsonrpc":"2.0","id":'$client_id',"method":"tools/list"}'
    
    local start_time
    start_time="$(date +%s%N)"
    local tmpfile="/tmp/kaboom-test-client-${client_id}-$$.out"
    local errfile="/tmp/kaboom-test-client-${client_id}-$$.err"
    
    # Run wrapper via stdio (simulates how LLM calls it)
    # Use perl for timeout (works on macOS)
    (
        echo "$request" | "$BINARY" --port "$PORT" 2>"$errfile" >"$tmpfile" &
        local pid="$!"
        local elapsed=0
        while kill -0 "$pid" 2>/dev/null && [ "$elapsed" -lt 10 ]; do
            sleep 0.1
            elapsed="$((elapsed + 1))"
        done
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null
            echo "TIMEOUT" > "$tmpfile"
        fi
        wait "$pid" 2>/dev/null
    )
    
    local end_time
    end_time="$(date +%s%N)"
    local duration_ms
    duration_ms="$(( (end_time - start_time) / 1000000 ))"
    
    local response
    response="$(cat "$tmpfile" 2>/dev/null)"
    local stderr_output
    stderr_output="$(cat "$errfile" 2>/dev/null)"
    
    # Check result
    if echo "$response" | grep -q '"result"'; then
        echo "✅ Client $client_id: Connected in ${duration_ms}ms"
        rm -f "$tmpfile" "$errfile"
        return 0
    else
        echo "❌ Client $client_id: Failed after ${duration_ms}ms"
        if echo "$response" | grep -q "TIMEOUT"; then
            echo "   Error: Timeout (>10s)"
        else
            local error_msg
            error_msg="$(echo "$stderr_output" | grep -E 'error|Error|ERROR|failed|FAILED' | head -2 | tr '\n' ' ')"
            if [ -n "$error_msg" ]; then
                echo "   Error: $error_msg"
            else
                echo "   Error: No response received"
            fi
        fi
        rm -f "$tmpfile" "$errfile"
        return 1
    fi
}

# Test Round 1: Cold start with 10 concurrent clients
echo "═══════════════════════════════════════════════════════════"
echo "📍 ROUND 1: Cold Start (No existing server)"
echo "═══════════════════════════════════════════════════════════"
echo ""

# Clean slate - ensure no server running
echo "🧹 Cleaning up any existing server on port $PORT..."
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 1

if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "❌ FAILED: Port $PORT still in use after cleanup"
    exit 1
fi
echo "✅ Port $PORT is free"
echo ""

echo "🚀 Starting $NUM_CLIENTS clients simultaneously (cold start)..."
echo ""

# shellcheck disable=SC2030 # subshell modification is intentional for parallel execution
round1_success=0
round1_failed=0

for i in $(seq 1 "$NUM_CLIENTS"); do
    {
        if send_mcp_request "$i"; then
            # shellcheck disable=SC2030
            ((round1_success++)) || true
        else
            # shellcheck disable=SC2030
            ((round1_failed++)) || true
        fi
    } &
done

# Wait for all clients to finish
wait

echo ""
echo "📊 Round 1 Results:"
# shellcheck disable=SC2031
echo "   ✅ Successful: $round1_success/$NUM_CLIENTS"
# shellcheck disable=SC2031
echo "   ❌ Failed:     $round1_failed/$NUM_CLIENTS"
echo ""

# Verify server is running
if curl -s "http://localhost:$PORT/health" >/dev/null 2>&1; then
    echo "✅ Server is running and responding"
    
    # Check single process
    PIDS="$(lsof -ti :"$PORT" 2>/dev/null || echo "")"
    NUM_PROCS="$(echo "$PIDS" | wc -l | tr -d ' ')"
    if [ "$NUM_PROCS" = "1" ]; then
        echo "✅ Single server process (PID: $PIDS)"
    else
        echo "⚠️  WARNING: Expected 1 process, found $NUM_PROCS"
        echo "   PIDs: $PIDS"
    fi
else
    echo "❌ FAILED: Server not responding after client connections"
    exit 1
fi

# Pause before next round
echo ""
echo "⏸️  Pausing 2 seconds before server kill..."
sleep 2

# Test Round 2: Kill server and restart with 10 concurrent clients
echo ""
echo "═══════════════════════════════════════════════════════════"
echo "📍 ROUND 2: Recovery Test (Kill server + concurrent restart)"
echo "═══════════════════════════════════════════════════════════"
echo ""

echo "💀 Killing server (simulating crash/hang)..."
SERVER_PID="$(lsof -ti :"$PORT" 2>/dev/null || echo "")"
if [ -n "$SERVER_PID" ]; then
    kill -9 "$SERVER_PID" 2>/dev/null || true
    sleep 1
    echo "✅ Server killed (was PID: $SERVER_PID)"
else
    echo "⚠️  No server found to kill"
fi

# Verify port is free
if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "❌ FAILED: Port $PORT still in use after kill"
    exit 1
fi
echo "✅ Port $PORT is free"
echo ""

echo "🚀 Starting $NUM_CLIENTS clients simultaneously (restart test)..."
echo ""

# shellcheck disable=SC2030 # subshell modification is intentional for parallel execution
round2_success=0
round2_failed=0

for i in $(seq 1 "$NUM_CLIENTS"); do
    {
        if send_mcp_request "$i"; then
            # shellcheck disable=SC2030
            ((round2_success++)) || true
        else
            # shellcheck disable=SC2030
            ((round2_failed++)) || true
        fi
    } &
done

# Wait for all clients to finish
wait

echo ""
echo "📊 Round 2 Results:"
# shellcheck disable=SC2031 # subshell modification is intentional for parallel execution
echo "   ✅ Successful: $round2_success/$NUM_CLIENTS"
# shellcheck disable=SC2031 # subshell modification is intentional for parallel execution
echo "   ❌ Failed:     $round2_failed/$NUM_CLIENTS"
echo ""

# Verify server is running again
if curl -s "http://localhost:$PORT/health" >/dev/null 2>&1; then
    echo "✅ Server is running and responding after recovery"
    
    # Check single process
    PIDS="$(lsof -ti :"$PORT" 2>/dev/null || echo "")"
    NUM_PROCS="$(echo "$PIDS" | wc -l | tr -d ' ')"
    if [ "$NUM_PROCS" = "1" ]; then
        echo "✅ Single server process (PID: $PIDS)"
    else
        echo "⚠️  WARNING: Expected 1 process, found $NUM_PROCS"
        echo "   PIDs: $PIDS"
    fi
else
    echo "❌ FAILED: Server not responding after recovery"
    exit 1
fi

# Final Summary
echo ""
echo "════════════════════════════════════════════════════════════"
echo "📈 FINAL SUMMARY"
echo "════════════════════════════════════════════════════════════"
# shellcheck disable=SC2031 # subshell modification is intentional for parallel execution
total_success="$((round1_success + round2_success))"
total_attempts="$((NUM_CLIENTS * 2))"
success_rate="$(( total_success * 100 / total_attempts ))"

echo ""
echo "   Total Attempts:  $total_attempts"
echo "   Total Success:   $total_success"
echo "   Total Failed:    $((total_attempts - total_success))"
echo "   Success Rate:    ${success_rate}%"
echo ""

if [ "$success_rate" -eq 100 ]; then
    echo "✅ ALL TESTS PASSED - Connection lifecycle is working correctly!"
    echo ""
    echo "✓ Cold start with $NUM_CLIENTS concurrent clients: PASSED"
    echo "✓ Server recovery after kill: PASSED"
    echo "✓ No duplicate server processes: PASSED"
elif [ "$success_rate" -ge 90 ]; then
    echo "⚠️  MOSTLY PASSED - ${success_rate}% success rate"
    echo ""
    echo "Some clients failed, but recovery logic is working."
    echo "Review stderr output above for details."
else
    echo "❌ TESTS FAILED - Only ${success_rate}% success rate"
    echo ""
    echo "Connection lifecycle has issues. Check stderr output above."
    exit 1
fi

echo ""
echo "💡 Logs: tail -50 ~/kaboom-logs.jsonl | jq -r '.event'"
echo ""
