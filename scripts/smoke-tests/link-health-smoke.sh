#!/bin/bash
# link-health-smoke.sh — Quick smoke test for link health feature
# Verifies analyze({what:'link_health'}) works end-to-end

set -euo pipefail

echo "============================================================"
echo "Link Health Analyzer — Smoke Test"
echo "============================================================"
echo ""

# Get the binary
BINARY="${1:-kaboom-agentic-browser}"
PORT="${2:-7890}"

if ! command -v "$BINARY" &> /dev/null; then
    echo "ERROR: $BINARY not found in PATH"
    echo "Install with: npm install -g kaboom-agentic-browser"
    exit 1
fi

echo "Binary:  $BINARY"
echo "Port:    $PORT"
echo ""

# Kill any existing process on the port
lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

# Start daemon
echo "Starting daemon on port $PORT..."
"$BINARY" --port "$PORT" > /tmp/kaboom-smoke.log 2>&1 &
DAEMON_PID=$!
trap "kill $DAEMON_PID 2>/dev/null || true" EXIT

# Wait for daemon to start
sleep 2

# Health check
echo "Checking daemon health..."
if ! nc -z localhost "$PORT" 2>/dev/null; then
    echo "ERROR: Daemon failed to start"
    cat /tmp/kaboom-smoke.log
    exit 1
fi
echo "✓ Daemon is running"
echo ""

# Test 1: analyze({what:'link_health'}) returns correlation_id
echo "TEST 1: analyze({what:'link_health'}) returns correlation_id"
RESPONSE=$(cat <<'EOF' | nc localhost "$PORT"
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"link_health"}}}
EOF
)

if echo "$RESPONSE" | grep -q "correlation_id"; then
    echo "✓ PASS: correlation_id in response"
else
    echo "✗ FAIL: correlation_id not found in response"
    echo "Response: $RESPONSE"
    exit 1
fi

if echo "$RESPONSE" | grep -q "link_health_"; then
    echo "✓ PASS: correlation_id has 'link_health_' prefix"
else
    echo "✗ FAIL: correlation_id missing 'link_health_' prefix"
    exit 1
fi
echo ""

# Test 2: analyze tool is in tools/list
echo "TEST 2: analyze tool registered in tools/list"
TOOLS=$(cat <<'EOF' | nc localhost "$PORT"
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF
)

if echo "$TOOLS" | grep -q '"name":"analyze"'; then
    echo "✓ PASS: analyze tool is in tools/list"
else
    echo "✗ FAIL: analyze tool not found in tools/list"
    exit 1
fi
echo ""

# Test 3: Invalid mode returns error
echo "TEST 3: analyze({what:'invalid_mode'}) returns error"
ERROR=$(cat <<'EOF' | nc localhost "$PORT"
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"invalid_mode"}}}
EOF
)

if echo "$ERROR" | grep -q "error"; then
    echo "✓ PASS: Invalid mode returns error"
else
    echo "✗ FAIL: Should error on invalid mode"
    exit 1
fi
echo ""

echo "============================================================"
echo "All smoke tests passed! ✓"
echo "============================================================"
echo ""
echo "Next steps:"
echo "1. Build and install: npm run build && npm install -g ."
echo "2. Run UAT suite: ./scripts/uat/runners/test-all-tools-comprehensive.sh"
echo "3. Check link health in browser (with extension connected)"
