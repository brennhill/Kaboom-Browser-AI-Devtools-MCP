#!/bin/bash
# validate-architecture.sh — Enforce the canonical async queue-and-poll architecture.
# Docs: docs/features/feature/query-service/index.md
set -euo pipefail

CMD_PKG="${KABOOM_CMD_PKG:-./cmd/browser-agent}"
CMD_DIR="${CMD_PKG#./}"
ERRORS=0

pass() {
    echo "   ✅ $1"
}

fail() {
    echo "   ❌ $1"
    ERRORS=$((ERRORS + 1))
}

require_file() {
    if [ -f "$1" ]; then
        pass "$1"
    else
        fail "MISSING: $1"
    fi
}

require_method() {
    local method="$1"
    shift
    if grep -Eq "func .*${method}\\(" "$@"; then
        pass "$method"
    else
        fail "MISSING METHOD: $method"
    fi
}

echo "🏗️  Validating Kaboom architecture..."
echo ""
echo "1️⃣  Checking canonical owners..."

CRITICAL_FILES=(
    "internal/queries/dispatcher.go"
    "internal/queries/dispatcher_queries.go"
    "internal/queries/dispatcher_commands.go"
    "internal/queries/dispatcher_results.go"
    "internal/queries/types.go"
    "internal/capture/capture.go"
    "internal/capture/httpingest/handlers.go"
    "internal/capture/syncruntime/handler.go"
    "$CMD_DIR/tools_core.go"
    "$CMD_DIR/tools_interact_dispatch.go"
    "$CMD_DIR/internal/toolobserve/dispatcher.go"
    "$CMD_DIR/internal/toolinteract/interact_browser.go"
    "$CMD_DIR/internal/bridge/bridge.go"
)

for file in "${CRITICAL_FILES[@]}"; do
    require_file "$file"
done

echo ""
echo "2️⃣  Checking query and command lifecycle methods..."

QUERY_SOURCES=(
    "internal/queries/dispatcher_queries.go"
    "internal/queries/dispatcher_commands.go"
    "internal/queries/dispatcher_results.go"
)
for method in \
    CreatePendingQuery CreatePendingQueryWithTimeout GetPendingQueries GetPendingQueriesForClient \
    SetQueryResult SetQueryResultWithClient TakeQueryResult RegisterCommand ApplyCommandResult \
    ExpireCommand GetCommandResult GetPendingCommands GetCompletedCommands GetFailedCommands; do
    require_method "$method" "${QUERY_SOURCES[@]}"
done

echo ""
echo "3️⃣  Checking transport and MCP boundaries..."

require_method "HandleSync" "internal/capture/syncruntime/handler.go"
require_method "CommandResult" "$CMD_DIR/internal/toolobserve/dispatcher.go"
require_method "pendingCommands" "$CMD_DIR/internal/toolobserve/dispatcher.go"
require_method "FailedCommands" "$CMD_DIR/internal/toolobserve/dispatcher.go"
require_method "handleExecuteJS" "$CMD_DIR/internal/toolinteract/interact_browser.go"
require_method "handleNavigate" "$CMD_DIR/internal/toolinteract/interact_browser.go"

if grep -q 'queries.*\\[\\]interface{}{}' internal/capture/httpingest/handlers.go; then
    fail "STUB DETECTED: handlers.go returns an empty query array"
else
    pass "capture handlers contain no empty query stub"
fi

if awk '
    /func \(d \*Dispatcher\) CommandResult\(/ { in_func=1; next }
    in_func && /^func / { in_func=0 }
    in_func && /GetCommandResult\(/ { found=1 }
    END { exit(found ? 0 : 1) }
' "$CMD_DIR/internal/toolobserve/dispatcher.go"; then
    pass "observe command_result reads the canonical command store"
else
    fail "STUB DETECTED: observe CommandResult does not read GetCommandResult"
fi

echo ""
echo "4️⃣  Running executable architecture contracts..."

if go test ./internal/queries ./internal/capture "$CMD_PKG/internal/toolobserve" \
    -run 'Test(QueryDispatcherHasNoCompatibilityFacades|CaptureHasNoCompatibilityAliases|AsyncQueueIntegration|DispatcherCommandResult)' \
    > /tmp/kaboom-architecture-tests.log 2>&1; then
    pass "query, capture, and observe architecture contracts pass"
else
    fail "architecture contract tests failed"
    tail -30 /tmp/kaboom-architecture-tests.log
fi

echo ""
echo "5️⃣  Checking bounded queue constants..."

ASYNC_TIMEOUT_SECONDS=$(grep -E 'AsyncCommandTimeout[[:space:]]*=' internal/queries/types.go | head -1 | grep -oE '[0-9]+' | head -1 || true)
if [ -z "${ASYNC_TIMEOUT_SECONDS:-}" ]; then
    fail "AsyncCommandTimeout constant not found"
elif [ "$ASYNC_TIMEOUT_SECONDS" -lt 30 ]; then
    fail "AsyncCommandTimeout too low (${ASYNC_TIMEOUT_SECONDS}s, expected >= 30s)"
else
    pass "AsyncCommandTimeout = ${ASYNC_TIMEOUT_SECONDS}s"
fi

MAX_PENDING_QUERIES_VALUE=$(grep -E 'MaxPendingQueries[[:space:]]*=' internal/queries/dispatcher_queries.go | head -1 | grep -oE '[0-9]+' | head -1 || true)
if [ -z "${MAX_PENDING_QUERIES_VALUE:-}" ]; then
    fail "MaxPendingQueries constant not found"
elif [ "$MAX_PENDING_QUERIES_VALUE" -lt 5 ]; then
    fail "MaxPendingQueries too low (${MAX_PENDING_QUERIES_VALUE}, expected >= 5)"
else
    pass "MaxPendingQueries = ${MAX_PENDING_QUERIES_VALUE}"
fi

echo ""
echo "6️⃣  Checking architecture documentation..."
require_file "docs/core/protocol/async-tool-pattern.md"
require_file "docs/architecture/decisions/ADR-002-async-queue-immutability.md"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$ERRORS" -eq 0 ]; then
    echo "✅ Architecture validation PASSED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
fi

echo "❌ Architecture validation FAILED with $ERRORS error(s)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "The canonical async queue-and-poll architecture is broken."
echo "DO NOT merge this change."
echo "See: docs/core/protocol/async-tool-pattern.md"
exit 1
