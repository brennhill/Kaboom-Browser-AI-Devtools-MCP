#!/bin/bash
# quick-regression-check.sh — Fast regression check (no server startup)
# Run this before every commit or deployment
set -euo pipefail

CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
CMD_DIR="${CMD_PKG#./}"

echo "⚡ Quick Regression Check"
echo "========================"
echo ""

START_TIME="$(date +%s)"
ERRORS=0

# 1. Binary compilation
echo "1️⃣  Compiling binary..."
if go build -o /tmp/kaboom-quick-test "$CMD_PKG" 2>/dev/null; then
    echo "   ✅ Binary compiles"
    rm /tmp/kaboom-quick-test
else
    echo "   ❌ COMPILATION FAILED"
    ERRORS="$((ERRORS + 1))"
fi

# 2. Integration tests
echo "2️⃣  Running integration tests..."
if go test ./internal/capture -run TestAsyncQueueIntegration -timeout 10s > /dev/null 2>&1; then
    echo "   ✅ Integration tests pass"
else
    echo "   ❌ INTEGRATION TESTS FAILED"
    ERRORS="$((ERRORS + 1))"
fi

# 3. Architecture validation
echo "3️⃣  Validating architecture..."
if ./scripts/validate-architecture.sh > /dev/null 2>&1; then
    echo "   ✅ Architecture intact"
else
    echo "   ❌ ARCHITECTURE BROKEN"
    ERRORS="$((ERRORS + 1))"
fi

# 4. Critical files exist
echo "4️⃣  Checking critical files..."
CRITICAL_FILES=(
    "internal/queries/dispatcher_queries.go"
    "internal/capture/query_dispatcher.go"
    "internal/capture/httpingest/handlers.go"
    "$CMD_DIR/tools_observe.go"
    "$CMD_DIR/tools_core.go"
)

for file in "${CRITICAL_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "   ❌ MISSING: $file"
        ERRORS="$((ERRORS + 1))"
    fi
done

if [ "$ERRORS" -eq 0 ]; then
    echo "   ✅ All critical files present"
fi

# 5. No stub implementations
echo "5️⃣  Checking for stubs..."
if grep -q 'queries.*\[\]interface{}{}' internal/capture/httpingest/handlers.go 2>/dev/null; then
    echo "   ❌ STUB in handlers.go"
    ERRORS="$((ERRORS + 1))"
elif ! grep -q 'GetCommandResult' "$CMD_DIR/tools_observe_analysis.go" 2>/dev/null; then
    echo "   ❌ STUB in toolObserveCommandResult"
    ERRORS="$((ERRORS + 1))"
else
    echo "   ✅ No stubs detected"
fi

echo ""
echo "========================"
if [ "$ERRORS" -eq 0 ]; then
    echo "✅ PASSED ($(( $(date +%s) - START_TIME ))s)"
    echo "No regressions detected"
    exit 0
else
    echo "❌ FAILED with $ERRORS errors"
    echo "Run: ./scripts/verify-no-regressions.sh for detailed diagnostics"
    exit 1
fi
