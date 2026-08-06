#!/bin/bash
# verify-no-regressions.sh — Comprehensive regression testing for Kaboom
# Tests: Go binary + Browser extension + MCP bridge + End-to-end flow
set -euo pipefail

CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
CMD_DIR="${CMD_PKG#./}"

echo "🔬 Kaboom Regression Testing Suite"
echo "======================================"
echo ""

ERRORS=0
WARNINGS=0

# ============================================
# 1. Go Binary Compilation
# ============================================

echo "1️⃣  Testing Go binary compilation..."

if go build -o /tmp/kaboom-test "$CMD_PKG"; then
    echo "   ✅ Binary compiles successfully"
    BINARY_PATH="/tmp/kaboom-test"
else
    echo "   ❌ Binary compilation FAILED"
    ERRORS=$((ERRORS + 1))
    exit 1
fi

# Check binary size (should be reasonable, not empty)
BINARY_SIZE=$(stat -f%z "$BINARY_PATH" 2>/dev/null || stat -c%s "$BINARY_PATH" 2>/dev/null)
if [ "$BINARY_SIZE" -lt 1000000 ]; then
    echo "   ⚠️  WARNING: Binary is suspiciously small ($BINARY_SIZE bytes)"
    WARNINGS=$((WARNINGS + 1))
else
    echo "   ✅ Binary size: $BINARY_SIZE bytes"
fi

# ============================================
# 2. Binary Smoke Test (Without Extension)
# ============================================

echo ""
echo "2️⃣  Testing binary startup (smoke test)..."

# Start binary in background with timeout
timeout 5s "$BINARY_PATH" --port 17890 > /tmp/kaboom-startup.log 2>&1 &
KABOOM_PID=$!

sleep 2

# Check if process is still running
if kill -0 "$KABOOM_PID" 2>/dev/null; then
    echo "   ✅ Binary started successfully (PID: $KABOOM_PID)"

    # Try to hit health endpoint
    if curl -s http://localhost:17890/health > /dev/null 2>&1; then
        echo "   ✅ Health endpoint responding"
    else
        echo "   ⚠️  WARNING: Health endpoint not responding"
        WARNINGS=$((WARNINGS + 1))
    fi

    # Kill test process
    kill "$KABOOM_PID" 2>/dev/null || true
    wait "$KABOOM_PID" 2>/dev/null || true
else
    echo "   ❌ Binary crashed on startup"
    echo "   Startup log:"
    cat /tmp/kaboom-startup.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 3. Go Unit Tests
# ============================================

echo ""
echo "3️⃣  Running Go unit tests..."

if go test ./internal/capture -v -short > /tmp/kaboom-unit-tests.log 2>&1; then
    TEST_COUNT=$(grep -c "^=== RUN" /tmp/kaboom-unit-tests.log || echo "0")
    PASS_COUNT=$(grep -c "^--- PASS" /tmp/kaboom-unit-tests.log || echo "0")
    echo "   ✅ Unit tests passed ($PASS_COUNT/$TEST_COUNT tests)"
else
    echo "   ❌ Unit tests FAILED"
    echo "   Last 20 lines:"
    tail -20 /tmp/kaboom-unit-tests.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 4. Integration Tests (Critical Path)
# ============================================

echo ""
echo "4️⃣  Running integration tests (async queue)..."

if go test -v ./internal/capture -run TestAsyncQueueIntegration > /tmp/kaboom-integration.log 2>&1; then
    echo "   ✅ Async queue integration test passed"
else
    echo "   ❌ Integration test FAILED"
    cat /tmp/kaboom-integration.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 5. Architecture Validation
# ============================================

echo ""
echo "5️⃣  Validating architecture integrity..."

if ./scripts/quality/verification/validate-architecture.sh > /tmp/kaboom-arch-validation.log 2>&1; then
    echo "   ✅ Architecture validation passed"
else
    echo "   ❌ Architecture validation FAILED"
    tail -30 /tmp/kaboom-arch-validation.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 6. Critical Endpoint Availability
# ============================================

echo ""
echo "6️⃣  Testing critical endpoints (mock server)..."

# Start server on test port
"$BINARY_PATH" --port 17891 > /tmp/kaboom-endpoints.log 2>&1 &
SERVER_PID=$!
sleep 2

# Test critical endpoints
ENDPOINTS=(
    "/health"
    "/pending-queries"
    "/pilot-status"
)

for endpoint in "${ENDPOINTS[@]}"; do
    if curl -s -o /dev/null -w "%{http_code}" "http://localhost:17891$endpoint" | grep -q "200\|404\|405"; then
        echo "   ✅ $endpoint reachable"
    else
        echo "   ❌ $endpoint NOT reachable"
        ERRORS=$((ERRORS + 1))
    fi
done

# Kill server
kill "$SERVER_PID" 2>/dev/null || true
wait "$SERVER_PID" 2>/dev/null || true

# ============================================
# 7. Bridge Binary Test
# ============================================

echo ""
echo "7️⃣  Testing MCP bridge..."

# Check bridge functionality exists
if grep -q "bridgeStdioToHTTP" "$CMD_DIR/bridge.go"; then
    echo "   ✅ Bridge code present"
else
    echo "   ❌ Bridge code MISSING"
    ERRORS=$((ERRORS + 1))
fi

# Test JSON-RPC parsing
echo '{"jsonrpc":"2.0","method":"test","id":1}' | timeout 2s "$BINARY_PATH" bridge http://localhost:17892 > /tmp/bridge-test.log 2>&1 || true

if grep -q "Server connection error" /tmp/bridge-test.log; then
    echo "   ✅ Bridge handles connection errors correctly"
elif grep -q "error" /tmp/bridge-test.log; then
    echo "   ⚠️  WARNING: Unexpected bridge error"
    WARNINGS=$((WARNINGS + 1))
else
    echo "   ✅ Bridge processes input (no server = expected error)"
fi

# ============================================
# 8. Extension Compatibility Check
# ============================================

echo ""
echo "8️⃣  Checking extension compatibility..."

# Check if extension directory exists
if [ -d "extension" ] || [ -d "../kaboom-extension" ]; then
    echo "   ✅ Extension directory found"

    # Check for manifest.json
    if [ -f "extension/manifest.json" ] || [ -f "../kaboom-extension/manifest.json" ]; then
        echo "   ✅ Extension manifest exists"
    else
        echo "   ⚠️  WARNING: Extension manifest not found"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    echo "   ⚠️  WARNING: Extension directory not found (may be in separate repo)"
    WARNINGS=$((WARNINGS + 1))
fi

# ============================================
# 9. TypeScript Compilation (if applicable)
# ============================================

echo ""
echo "9️⃣  Testing TypeScript compilation (if present)..."

if [ -f "tsconfig.json" ] || [ -f "extension/tsconfig.json" ]; then
    if command -v npm &> /dev/null; then
        if npm run typecheck > /tmp/kaboom-ts.log 2>&1; then
            echo "   ✅ TypeScript compiles successfully"
        else
            echo "   ❌ TypeScript compilation FAILED"
            tail -20 /tmp/kaboom-ts.log
            ERRORS=$((ERRORS + 1))
        fi
    else
        echo "   ⚠️  WARNING: npm not found, skipping TypeScript check"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    echo "   ℹ️  No TypeScript config found (Go-only project)"
fi

# ============================================
# 10. API Contract Test
# ============================================

echo ""
echo "🔟 Testing API contracts..."

# Start server
"$BINARY_PATH" --port 17893 > /tmp/kaboom-api-test.log 2>&1 &
API_SERVER_PID=$!
sleep 2

# Test observe tool structure
OBSERVE_RESPONSE=$(curl -s -X POST http://localhost:17893/api/tools \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{"what":"pilot"}},"id":1}' 2>/dev/null || echo "{}")

if echo "$OBSERVE_RESPONSE" | grep -q '"enabled"'; then
    echo "   ✅ Observe tool returns expected structure"
else
    echo "   ⚠️  WARNING: Observe tool response unexpected"
    echo "   Response: $OBSERVE_RESPONSE"
    WARNINGS=$((WARNINGS + 1))
fi

# Kill server
kill "$API_SERVER_PID" 2>/dev/null || true
wait "$API_SERVER_PID" 2>/dev/null || true

# ============================================
# 11. Memory Leak Check (Basic)
# ============================================

echo ""
echo "1️⃣1️⃣ Testing for obvious memory leaks..."

# Start server and let it run for 3 seconds
"$BINARY_PATH" --port 17894 > /dev/null 2>&1 &
LEAK_TEST_PID=$!

sleep 1
INITIAL_MEM=$(ps -o rss= -p "$LEAK_TEST_PID" 2>/dev/null || echo "0")

sleep 2
FINAL_MEM=$(ps -o rss= -p "$LEAK_TEST_PID" 2>/dev/null || echo "0")

kill "$LEAK_TEST_PID" 2>/dev/null || true
wait "$LEAK_TEST_PID" 2>/dev/null || true

MEM_GROWTH=$((FINAL_MEM - INITIAL_MEM))
if [ "$MEM_GROWTH" -gt 50000 ]; then
    echo "   ⚠️  WARNING: High memory growth during idle ($MEM_GROWTH KB)"
    WARNINGS=$((WARNINGS + 1))
else
    echo "   ✅ Memory usage stable ($MEM_GROWTH KB growth)"
fi

# ============================================
# 12. Pre-Commit Hook Verification
# ============================================

echo ""
echo "1️⃣2️⃣ Verifying pre-commit hook..."

if [ -f ".git/hooks/pre-commit" ] && [ -x ".git/hooks/pre-commit" ]; then
    echo "   ✅ Pre-commit hook installed and executable"
else
    echo "   ⚠️  WARNING: Pre-commit hook missing or not executable"
    WARNINGS=$((WARNINGS + 1))
fi

# ============================================
# Summary
# ============================================

echo ""
echo "======================================"
echo "📊 Regression Test Summary"
echo "======================================"
echo ""

if [ "$ERRORS" -eq 0 ] && [ "$WARNINGS" -eq 0 ]; then
    echo "✅ ALL TESTS PASSED"
    echo ""
    echo "No regressions detected in:"
    echo "  • Go binary compilation"
    echo "  • Runtime stability"
    echo "  • Unit tests"
    echo "  • Integration tests"
    echo "  • Architecture integrity"
    echo "  • HTTP endpoints"
    echo "  • MCP bridge"
    echo "  • API contracts"
    echo ""
    echo "Safe to deploy! 🚀"
    exit 0
elif [ "$ERRORS" -eq 0 ]; then
    echo "⚠️  PASSED WITH WARNINGS"
    echo ""
    echo "Warnings: $WARNINGS"
    echo ""
    echo "Review warnings above. May be safe to deploy."
    exit 0
else
    echo "❌ REGRESSION DETECTED"
    echo ""
    echo "Errors: $ERRORS"
    echo "Warnings: $WARNINGS"
    echo ""
    echo "DO NOT DEPLOY until errors are fixed."
    echo ""
    echo "Check logs in /tmp/kaboom-*.log for details."
    exit 1
fi
