#!/bin/bash
# Automated Test Generation Validation Script (stdio version)
# Prerequisites:
#   - Demo site running on http://localhost:3000
#   - Chrome with Kaboom extension open and connected
#   - Run this script, it will start the MCP server and send commands

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/../../validation-results"
mkdir -p "$RESULTS_DIR"

MCP_BIN="$SCRIPT_DIR/../../../dist/kaboom-agentic-browser"

if [ ! -f "$MCP_BIN" ]; then
    echo "Error: Kaboom binary not found at $MCP_BIN"
    echo "Run 'make dev' first"
    exit 1
fi

echo "=== Kaboom Test Generation Validation ==="
echo ""
echo "Prerequisites:"
echo "  ✓ Demo site running on http://localhost:3000"
echo "  ✓ Chrome with Kaboom extension connected"
echo ""
echo "This script will:"
echo "  1. Use interact to navigate and trigger bugs"
echo "  2. Use observe to verify capture"
echo "  3. Use generate to create tests"
echo "  4. Validate output quality"
echo ""
echo "Starting validation in 3 seconds..."
sleep 3

# Create a temporary file for sending commands
COMMANDS_FILE=$(mktemp)
trap 'rm -f $COMMANDS_FILE' EXIT

# Build the command sequence
cat > "$COMMANDS_FILE" << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"interact","arguments":{"what":"navigate","url":"http://localhost:3000"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"interact","arguments":{"what":"click","selector":"button:has-text('Chat')","description":"Open chat widget"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe","arguments":{"what":"websocket"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"generate","arguments":{"what":"test_from_context","context":"error","framework":"playwright","base_url":"http://localhost:3000"}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"generate","arguments":{"what":"test_from_context","context":"interaction","framework":"playwright","base_url":"http://localhost:3000"}}}
EOF

echo ""
echo "Sending commands to Kaboom MCP server..."
echo ""

# Function to send a single command and save output
send_command() {
    local id=$1
    local description=$2
    local command
    command=$(sed -n "${id}p" "$COMMANDS_FILE")

    echo "[$id/6] $description"
    echo "$command" | "$MCP_BIN" > "$RESULTS_DIR/$(printf "%02d" "$id")-response.json" 2>&1 &
    local pid=$!

    # Wait a bit for the command to process
    sleep 2

    # Check if still running
    if kill -0 "$pid" 2>/dev/null; then
        wait "$pid" || true
    fi

    # Check if we got a response
    if [ -s "$RESULTS_DIR/$(printf "%02d" "$id")-response.json" ]; then
        echo "  ✓ Response received"
    else
        echo "  ⚠ No response (command may still be processing)"
    fi

    echo ""
}

# Execute commands one by one
send_command 1 "Navigate to demo site"
send_command 2 "Click Chat button"
sleep 2  # Extra wait for WebSocket errors to occur
send_command 3 "Observe WebSocket frames"
send_command 4 "Observe console errors"
send_command 5 "Generate test from error context"
send_command 6 "Generate test from interaction context"

# Extract and validate generated tests
echo "Extracting generated test content..."
echo ""

# Extract test from response 5 (error context)
if [ -f "$RESULTS_DIR/05-response.json" ]; then
    python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('result',{}).get('content',{}).get('test',{}).get('content',''))" \
        < "$RESULTS_DIR/05-response.json" \
        > "$RESULTS_DIR/generated-error-test.spec.ts" 2>/dev/null || \
    grep -o '"content":".*"' < "$RESULTS_DIR/05-response.json" | \
        sed 's/"content":"//;s/"$//' | \
        sed 's/\\n/\n/g' | \
        head -100 > "$RESULTS_DIR/generated-error-test.spec.ts"

    if [ -s "$RESULTS_DIR/generated-error-test.spec.ts" ]; then
        echo "✓ Extracted test from error context"
        echo "  File: $RESULTS_DIR/generated-error-test.spec.ts"
        echo "  Size: $(wc -l < "$RESULTS_DIR/generated-error-test.spec.ts") lines"
    fi
fi

# Extract test from response 6 (interaction context)
if [ -f "$RESULTS_DIR/06-response.json" ]; then
    python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('result',{}).get('content',{}).get('test',{}).get('content',''))" \
        < "$RESULTS_DIR/06-response.json" \
        > "$RESULTS_DIR/generated-interaction-test.spec.ts" 2>/dev/null || \
    grep -o '"content":".*"' < "$RESULTS_DIR/06-response.json" | \
        sed 's/"content":"//;s/"$//' | \
        sed 's/\\n/\n/g' | \
        head -100 > "$RESULTS_DIR/generated-interaction-test.spec.ts"

    if [ -s "$RESULTS_DIR/generated-interaction-test.spec.ts" ]; then
        echo "✓ Extracted test from interaction context"
        echo "  File: $RESULTS_DIR/generated-interaction-test.spec.ts"
        echo "  Size: $(wc -l < "$RESULTS_DIR/generated-interaction-test.spec.ts") lines"
    fi
fi

echo ""
echo "=== Validation Summary ==="
echo ""

# List all output files
echo "Generated files:"
# shellcheck disable=SC2012 # ls used for human-readable display
ls -lh "$RESULTS_DIR"/*.json "$RESULTS_DIR"/*.ts 2>/dev/null | awk '{print "  "$9" ("$5")"}'

echo ""
echo "Next steps:"
echo "  1. Review responses: cat $RESULTS_DIR/*.json"
echo "  2. Review tests: cat $RESULTS_DIR/*.spec.ts"
echo "  3. Run tests: cd ~/dev/kaboom-demos && npx playwright test"
echo ""
echo "To view the generated test from error:"
echo "  cat $RESULTS_DIR/generated-error-test.spec.ts"
