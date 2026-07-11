#!/bin/bash
# validate-architecture.sh — Enforce async queue-and-poll architecture
# Run in CI to catch architecture violations before merge
set -euo pipefail

CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
CMD_DIR="${CMD_PKG#./}"

echo "🏗️  Validating Kaboom architecture..."
echo ""

ERRORS=0

# ============================================
# 1. Critical Files Existence
# ============================================

echo "1️⃣  Checking critical files..."

CRITICAL_FILES=(
    "internal/queries/dispatcher_queries.go"
    "internal/capture/query_dispatcher.go"
    "internal/capture/handlers.go"
    "internal/capture/types.go"
    "internal/queries/types.go"
    "$CMD_DIR/tools_core.go"
    "$CMD_DIR/tools_observe.go"
    "$CMD_DIR/tools_interact_dispatch.go"
    "$CMD_DIR/bridge_adapter.go"
    "$CMD_DIR/internal/bridge/bridge.go"
)

for file in "${CRITICAL_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "   ❌ MISSING: $file"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $file"
    fi
done

# ============================================
# 2. Required Methods Existence
# ============================================

echo ""
echo "2️⃣  Checking required methods in query dispatcher..."

REQUIRED_METHODS=(
    "CreatePendingQuery"
    "CreatePendingQueryWithTimeout"
    "GetPendingQueries"
    "GetPendingQueriesForClient"
    "SetQueryResult"
    "SetQueryResultWithClient"
    "GetQueryResult"
    "RegisterCommand"
    "CompleteCommand"
    "ExpireCommand"
    "GetCommandResult"
    "GetPendingCommands"
    "GetCompletedCommands"
    "GetFailedCommands"
)

# Methods exist in either capture delegation or queries implementation
for method in "${REQUIRED_METHODS[@]}"; do
    if grep -q "func.*$method" internal/capture/query_dispatcher.go || \
       grep -q "func.*$method" internal/queries/dispatcher_queries.go || \
       grep -q "func.*$method" internal/queries/dispatcher_commands.go; then
        echo "   ✅ $method"
    else
        echo "   ❌ MISSING METHOD: $method"
        ERRORS=$((ERRORS + 1))
    fi
done

# ============================================
# 3. Handler Endpoints
# ============================================

echo ""
echo "3️⃣  Checking HTTP handler endpoints..."

REQUIRED_HANDLERS=(
    "HandleSync"
)

for handler in "${REQUIRED_HANDLERS[@]}"; do
    if ! grep -q "func.*$handler" internal/capture/sync.go; then
        echo "   ❌ MISSING HANDLER: $handler"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $handler"
    fi
done

# ============================================
# 4. MCP Tool Handlers
# ============================================

echo ""
echo "4️⃣  Checking MCP tool handlers..."

# Top-level observe handlers live in $CMD_DIR/tools_*.go; the interact action
# implementations moved to $CMD_DIR/internal/interacthandler/ (HandleExecuteJSImpl,
# HandleBrowserActionNavigateImpl).
MCP_TOOL_HANDLERS=(
    "toolObserveCommandResult"
    "toolObservePendingCommands"
    "toolObserveFailedCommands"
    "HandleExecuteJSImpl"
    "HandleBrowserActionNavigateImpl"
)

for handler in "${MCP_TOOL_HANDLERS[@]}"; do
    if ! grep -rq "func.*$handler" "${CMD_DIR}"/tools_*.go "${CMD_DIR}"/internal/interacthandler/*.go; then
        echo "   ❌ MISSING TOOL HANDLER: $handler"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $handler"
    fi
done

# ============================================
# 5. No Stub Implementations
# ============================================

echo ""
echo "5️⃣  Checking for stub implementations..."

# Check handlers.go for stub returns
if grep -q 'queries.*\[\]interface{}{}' internal/capture/handlers.go; then
    echo "   ❌ STUB DETECTED: handlers.go returns empty array"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ No stub in handlers.go"
fi

# Check tools_observe.go for stub returns in command result observer
if command -v rg >/dev/null 2>&1; then
    OBSERVE_IMPL_FILES=$(rg -l 'func \(h \*ToolHandler\) toolObserveCommandResult\(' "${CMD_DIR}"/tools_*.go || true)
else
    OBSERVE_IMPL_FILES=$(grep -El 'func \(h \*ToolHandler\) toolObserveCommandResult\(' "${CMD_DIR}"/tools_*.go 2>/dev/null || true)
fi
if [ -z "${OBSERVE_IMPL_FILES:-}" ]; then
    echo "   ❌ MISSING: toolObserveCommandResult implementation"
    ERRORS=$((ERRORS + 1))
else
    OBSERVE_CALL_OK=0
    for file in $OBSERVE_IMPL_FILES; do
        if awk '
            /func \(h \*ToolHandler\) toolObserveCommandResult\(/ { in_func=1; next }
            in_func && /^func / { in_func=0 }
            in_func && /GetCommandResult\(/ { found=1 }
            END { exit(found ? 0 : 1) }
        ' "$file"; then
            OBSERVE_CALL_OK=1
            break
        fi
    done
    if [ "$OBSERVE_CALL_OK" -eq 1 ]; then
        echo "   ✅ toolObserveCommandResult calls GetCommandResult"
    else
        echo "   ❌ STUB DETECTED: toolObserveCommandResult doesn't call GetCommandResult"
        ERRORS=$((ERRORS + 1))
    fi
fi

# ============================================
# 6. Integration Test Exists
# ============================================

echo ""
echo "6️⃣  Checking integration tests..."

if [ ! -f "internal/capture/async_queue_integration_test.go" ]; then
    echo "   ❌ MISSING: async_queue_integration_test.go"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ async_queue_integration_test.go exists"
fi

# ============================================
# 7. Run Integration Tests
# ============================================

echo ""
echo "7️⃣  Running integration tests..."

if go test -v ./internal/capture -run TestAsyncQueueIntegration > /tmp/kaboom-integration-test.log 2>&1; then
    echo "   ✅ Integration tests pass"
else
    echo "   ❌ Integration tests FAILED"
    echo ""
    echo "   Test output:"
    tail -30 /tmp/kaboom-integration-test.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 8. Constants Check
# ============================================

echo ""
echo "8️⃣  Checking critical constants..."

ASYNC_TIMEOUT_SECONDS=$(grep -E 'AsyncCommandTimeout[[:space:]]*=' internal/queries/types.go | head -1 | grep -oE '[0-9]+' | head -1 || true)
if [ -z "${ASYNC_TIMEOUT_SECONDS:-}" ]; then
    echo "   ❌ AsyncCommandTimeout constant not found"
    ERRORS=$((ERRORS + 1))
elif [ "$ASYNC_TIMEOUT_SECONDS" -lt 30 ]; then
    echo "   ❌ AsyncCommandTimeout too low (${ASYNC_TIMEOUT_SECONDS}s, expected >= 30s)"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ AsyncCommandTimeout = ${ASYNC_TIMEOUT_SECONDS}s"
fi

MAX_PENDING_QUERIES_VALUE=$(grep -E 'MaxPendingQueries[[:space:]]*=' internal/queries/dispatcher_queries.go | head -1 | grep -oE '[0-9]+' | head -1 || true)
if [ -n "${MAX_PENDING_QUERIES_VALUE:-}" ]; then
    if [ "$MAX_PENDING_QUERIES_VALUE" -lt 5 ]; then
        echo "   ⚠️  WARNING: MaxPendingQueries = ${MAX_PENDING_QUERIES_VALUE} (expected >= 5 for queue durability)"
    else
        echo "   ✅ MaxPendingQueries = ${MAX_PENDING_QUERIES_VALUE}"
    fi
else
    echo "   ⚠️  WARNING: MaxPendingQueries constant not found in internal/queries/dispatcher_queries.go"
fi

# ============================================
# 9. Documentation Check
# ============================================

echo ""
echo "9️⃣  Checking documentation..."

DOC_CANDIDATES=(
    "docs/core/async-tool-pattern.md"
    "docs/architecture/ADR-002-async-queue-immutability.md"
    "docs/architecture/diagrams/async-queue-flow.md"
)
DOC_FOUND=0
for doc in "${DOC_CANDIDATES[@]}"; do
    if [ -f "$doc" ]; then
        echo "   ✅ $doc exists"
        DOC_FOUND=1
        break
    fi
done
if [ "$DOC_FOUND" -eq 0 ]; then
    echo "   ⚠️  WARNING: No async queue documentation file found in expected locations"
else
    :
fi

# ============================================
# Summary
# ============================================

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $ERRORS -eq 0 ]; then
    echo "✅ Architecture validation PASSED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
else
    echo "❌ Architecture validation FAILED with $ERRORS error(s)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "The async queue-and-poll architecture is broken."
    echo "DO NOT merge this change."
    echo ""
    echo "See: docs/core/async-tool-pattern.md"
    echo "Or ask: 'How do I restore the async queue implementation?'"
    exit 1
fi
