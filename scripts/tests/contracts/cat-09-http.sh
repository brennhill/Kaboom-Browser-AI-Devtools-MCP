#!/bin/bash
# cat-09-http.sh — UAT tests for HTTP endpoints (7 tests).
# NOTE: Test 9.7 (/shutdown) must be LAST as it kills the daemon.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "9" "HTTP Endpoints" "7"
ensure_daemon

# ── 9.1 — /health returns complete health object ──────────
begin_test "9.1" "/health returns complete health object" \
    "Verify /health JSON contains status, version, and uptime fields" \
    "Health shape is consumed by monitoring dashboards. Missing fields break integrations."
run_test_9_1() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    if [ -z "$body" ]; then
        fail "/health returned empty body."
        return
    fi
    # Must be valid JSON
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "/health returned invalid JSON. Body: $(truncate "$body")"
        return
    fi
    # Check required fields
    local has_status has_version
    has_status=$(echo "$body" | jq -e '.status' 2>/dev/null)
    has_version=$(echo "$body" | jq -e '.version' 2>/dev/null)
    if [ -z "$has_status" ] || [ "$has_status" = "null" ]; then
        fail "/health missing 'status' field. Body: $(truncate "$body")"
        return
    fi
    if [ -z "$has_version" ] || [ "$has_version" = "null" ]; then
        fail "/health missing 'version' field. Body: $(truncate "$body")"
        return
    fi
    pass "/health contains status=$has_status, version=$has_version. Body: $(truncate "$body" 200)"
}
run_test_9_1

# ── 9.2 — /diagnostics returns info ───────────────────────
begin_test "9.2" "/diagnostics returns info" \
    "Verify /diagnostics returns valid JSON, not 404 or 500" \
    "Diagnostics is the debugging escape hatch. Must not be broken when you need it most."
run_test_9_2() {
    local status
    status=$(get_http_status "http://localhost:${PORT}/diagnostics")
    if [ "$status" = "404" ] || [ "$status" = "500" ]; then
        fail "/diagnostics returned HTTP $status."
        return
    fi
    local body
    body=$(get_http_body "http://localhost:${PORT}/diagnostics")
    if [ -z "$body" ]; then
        fail "/diagnostics returned empty body."
        return
    fi
    # Should be valid JSON
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "/diagnostics returned invalid JSON. Body: $(truncate "$body")"
        return
    fi
    pass "/diagnostics returned HTTP $status with valid JSON. Body: $(truncate "$body" 200)"
}
run_test_9_2

# ── 9.3 — /mcp POST accepts JSON-RPC ──────────────────────
begin_test "9.3" "/mcp POST accepts JSON-RPC" \
    "Verify /mcp endpoint handles tools/list request and returns tools array" \
    "HTTP transport is the alternative to stdio. Must work for web-based MCP clients."
run_test_9_3() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
    if [ -z "$body" ]; then
        fail "/mcp POST returned empty body."
        return
    fi
    # Must be valid JSON-RPC
    if ! echo "$body" | jq -e '.jsonrpc == "2.0"' >/dev/null 2>&1; then
        fail "/mcp POST returned invalid JSON-RPC. Body: $(truncate "$body")"
        return
    fi
    # Must have tools in result
    local tool_count
    tool_count=$(echo "$body" | jq -r '.result.tools | length' 2>/dev/null)
    if [ -z "$tool_count" ] || [ "$tool_count" = "null" ] || [ "$tool_count" = "0" ]; then
        fail "/mcp POST did not return tools array. Body: $(truncate "$body")"
        return
    fi
    pass "/mcp POST returned valid JSON-RPC with $tool_count tools. Body: $(truncate "$body" 200)"
}
run_test_9_3

# ── 9.4 — /api/status returns valid JSON ───────────────────
begin_test "9.4" "/api/status returns valid flat JSON contract" \
    "GET /api/status — verify version, buffers, extension, and terminal fields" \
    "Dashboard API status endpoint. Missing fields break the dashboard UI."
run_test_9_4() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/api/status")
    if [ -z "$body" ]; then
        fail "/api/status returned empty body."
        return
    fi
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "/api/status returned invalid JSON. Body: $(truncate "$body")"
        return
    fi
    local has_version has_buffers has_extension has_terminal
    has_version=$(echo "$body" | jq -e '.version' 2>/dev/null)
    has_buffers=$(echo "$body" | jq -e '.buffers' 2>/dev/null)
    has_extension=$(echo "$body" | jq -e 'has("extension_connected")' 2>/dev/null)
    has_terminal=$(echo "$body" | jq -e '.terminal' 2>/dev/null)
    if [ -z "$has_version" ] || [ "$has_version" = "null" ]; then
        fail "/api/status missing 'version' field. Body: $(truncate "$body")"
        return
    fi
    if [ -z "$has_buffers" ] || [ "$has_buffers" = "null" ]; then
        fail "/api/status missing 'buffers' field. Body: $(truncate "$body")"
        return
    fi
    if [ "$has_extension" != "true" ]; then
        fail "/api/status missing 'extension_connected' field. Body: $(truncate "$body")"
        return
    fi
    if [ -z "$has_terminal" ] || [ "$has_terminal" = "null" ]; then
        fail "/api/status missing 'terminal' field. Body: $(truncate "$body")"
        return
    fi
    pass "/api/status returned the flat version, buffers, extension, and terminal contract. Body: $(truncate "$body" 200)"
}
run_test_9_4

# ── 9.5 — /health includes terminal_port ──────────────────
begin_test "9.5" "/health includes terminal_port field" \
    "GET /health — verify terminal_port = PORT+1" \
    "Terminal server isolation requires health to advertise the correct port."
run_test_9_5() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    local term_port
    term_port=$(echo "$body" | jq -r '.terminal_port // empty' 2>/dev/null)
    if [ -z "$term_port" ]; then
        fail "/health missing 'terminal_port' field. Body: $(truncate "$body")"
        return
    fi
    local expected_port=$((PORT + 1))
    if [ "$term_port" != "$expected_port" ]; then
        fail "/health terminal_port=$term_port, expected $expected_port. Body: $(truncate "$body")"
        return
    fi
    pass "/health includes terminal_port=$term_port (PORT+1). Body: $(truncate "$body" 200)"
}
run_test_9_5

# ── 9.6 — Terminal server on PORT+1 responds ──────────────
begin_test "9.6" "Terminal server on PORT+1 responds" \
    "GET /terminal on PORT+1 — verify HTTP 200" \
    "Terminal runs on a dedicated server (port+1). Must be reachable independently."
run_test_9_6() {
    local term_port=$((PORT + 1))
    local status
    status=$(get_http_status "http://localhost:${term_port}/terminal")
    if [ "$status" != "200" ]; then
        fail "GET /terminal on port $term_port returned HTTP $status (expected 200)."
        return
    fi
    pass "Terminal server on port $term_port responds with HTTP 200."
}
run_test_9_6

# ── 9.7 — /shutdown POST stops the server ─────────────────
# NOTE: This test MUST be last since it kills the daemon.
begin_test "9.7" "/shutdown POST stops the server" \
    "POST to /shutdown, wait, verify port is freed" \
    "Programmatic shutdown is used by CI cleanup."
run_test_9_7() {
    local shutdown_body
    shutdown_body=$(get_http_body "http://localhost:${PORT}/shutdown" -X POST -H "X-Kaboom-Client: kaboom-extension/${VERSION}")
    # Give the server time to shut down
    sleep 3
    # Verify port is freed
    if curl -s --connect-timeout 2 "http://localhost:${PORT}/health" >/dev/null 2>&1; then
        fail "Server still responding on port $PORT after /shutdown. Body: $(truncate "$shutdown_body")"
        return
    fi
    pass "/shutdown succeeded. Port $PORT is freed after 3 second wait. Shutdown response: $(truncate "$shutdown_body" 200)"
}
run_test_9_7

# finish_category calls kill_server (which no-ops if daemon is already dead)
# then writes results and exits.
finish_category
