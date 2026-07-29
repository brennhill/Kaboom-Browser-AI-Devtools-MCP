#!/bin/bash
# cat-06-lifecycle.sh — Server lifecycle tests.
# Tests cold start, persistence across disconnects, graceful shutdown,
# health endpoint, and version consistency.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "6" "Server Lifecycle" "5"

# ── Test 6.1: Cold start: first tool call works ───────────
begin_test "6.1" "Cold start: first tool call works" \
    "Kill daemon, send tools/call (observe page), verify daemon auto-spawns and responds" \
    "Cold start is the most fragile path — daemon must auto-spawn and respond"

run_test_6_1() {
    # Ensure clean slate: no daemon running
    kill_server
    sleep 0.5

    # Verify port is actually free
    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after kill_server. Cannot test cold start."
        return
    fi

    # Send a tool call — this should trigger daemon auto-spawn via fast-start bridge
    local response
    response=$(call_tool "observe" '{"what":"page"}')

    if [ -z "$response" ]; then
        fail "No response received within timeout. Daemon failed to auto-spawn on cold start."
        return
    fi

    # Verify it's valid JSON-RPC (not a timeout error or garbage)
    if ! check_valid_jsonrpc "$response"; then
        fail "Response is not valid JSON-RPC. Response: $(truncate "$response")"
        return
    fi

    # Check it's not a protocol-level error (tool errors with isError are acceptable)
    local has_result
    has_result=$(echo "$response" | jq -e '.result' 2>/dev/null || true)

    if echo "$response" | jq -e '.error' >/dev/null 2>&1 && [ -z "$has_result" ]; then
        local err_code
        err_code=$(echo "$response" | jq -r '.error.code // empty' 2>/dev/null)
        fail "Got protocol error on cold start: code=$err_code. Response: $(truncate "$response")"
        return
    fi

    pass "Cold start successful. Sent observe(page), daemon auto-spawned, valid JSON-RPC response received within timeout."
}
run_test_6_1

# ── Test 6.2: Server persists across client disconnects ───
begin_test "6.2" "Server persists across client disconnects" \
    "After 6.1, daemon should still be running. Send another tool call from a new client." \
    "Daemon mode is persistent — client disconnect must not kill the server"

run_test_6_2() {
    # The daemon should still be running from test 6.1
    # Give it a moment to stabilize
    sleep 0.3

    # Send a new tool call (this is a new client connection)
    local response
    response=$(call_tool "observe" '{"what":"page"}')

    if [ -z "$response" ]; then
        fail "No response from second client. Daemon may have died after first client disconnected."
        return
    fi

    if ! check_valid_jsonrpc "$response"; then
        fail "Second client got invalid JSON-RPC. Response: $(truncate "$response")"
        return
    fi

    # Also verify health endpoint responds (proves daemon is alive)
    local health_status
    health_status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health" 2>/dev/null)

    if [ "$health_status" != "200" ]; then
        fail "Health endpoint returned status $health_status (expected 200). Daemon may be degraded."
        return
    fi

    pass "Server persisted across client disconnect. Second client got valid response, health endpoint returned 200."
}
run_test_6_2

# ── Test 6.3: Graceful shutdown via --stop ────────────────
begin_test "6.3" "Graceful shutdown via --stop" \
    "Start daemon, verify health, run --stop, verify port freed and PID file cleaned" \
    "Ungraceful shutdown leaves orphan processes. PID file check prevents stale state."

run_test_6_3() {
    # Kill any existing daemon and start fresh
    kill_server
    sleep 0.3

    # Start daemon explicitly
    start_daemon
    if ! wait_for_health 50; then
        fail "Daemon failed to start (health not responding after 5s)."
        return
    fi

    # Verify health before stopping
    local health_body
    health_body=$(get_http_body "http://localhost:${PORT}/health")
    local health_status
    health_status=$(echo "$health_body" | jq -r '.status // empty' 2>/dev/null)

    if [ -z "$health_status" ]; then
        fail "Health endpoint did not return valid JSON before --stop. Body: $(truncate "$health_body")"
        return
    fi

    # Run --stop
    local stop_output
    stop_output=$("$WRAPPER" --stop --port "$PORT" 2>&1)
    local stop_exit=$?

    if [ $stop_exit -ne 0 ]; then
        fail "--stop exited with code $stop_exit. Output: $(truncate "$stop_output")"
        return
    fi

    # Wait a moment for port to be freed
    sleep 1

    # Verify port is freed
    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after --stop. Server did not shut down."
        return
    fi

    # Verify PID file is cleaned up
    local pid_file="$HOME/.kaboom-${PORT}.pid"
    if [ -f "$pid_file" ]; then
        fail "PID file $pid_file still exists after --stop (stale state)."
        rm -f "$pid_file"
        return
    fi

    pass "Graceful shutdown: --stop exited 0, port $PORT freed, PID file cleaned up."
}
run_test_6_3

# ── Test 6.4: Daemon health endpoint responds ────────────
begin_test "6.4" "Daemon health endpoint responds" \
    "Start daemon, curl /health, verify JSON with status/version/daemon_uptime" \
    "Health endpoint is the liveness probe for monitoring"

run_test_6_4() {
    # Start a fresh daemon
    kill_server
    sleep 0.3

    # One bounded retry for occasional port-release/startup races.
    local started=0
    for attempt in 1 2; do
        if start_daemon; then
            started=1
            break
        fi
        if [ "$attempt" -lt 2 ]; then
            echo "  [retry] Daemon startup race detected, retrying once..." | tee -a "$OUTPUT_FILE"
            kill_server
            sleep 0.5
        fi
    done

    if [ "$started" -ne 1 ]; then
        fail "Daemon failed to start for health endpoint test."
        return
    fi

    # Curl the health endpoint
    local http_status
    http_status=$(get_http_status "http://localhost:${PORT}/health")
    if [ "$http_status" != "200" ]; then
        fail "Health endpoint returned HTTP $http_status, expected 200."
        return
    fi

    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    # Check required fields
    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ -z "$status_val" ]; then
        fail "'status' field missing from health response. Body: $(truncate "$body")"
        return
    fi

    local version_val
    version_val=$(echo "$body" | jq -r '.version // empty' 2>/dev/null)
    if [ -z "$version_val" ]; then
        fail "'version' field missing from health response. Body: $(truncate "$body")"
        return
    fi

    pass "Health endpoint returned HTTP 200 with status='$status_val', version='$version_val'."
}
run_test_6_4

# ── Test 6.5: Version matches VERSION file ────────────────
begin_test "6.5" "Version matches VERSION file" \
    "Compare version from health endpoint, initialize response, and --version flag against VERSION file" \
    "Version mismatch means the wrong binary is running"

run_test_6_5() {
    # Ensure daemon is running (from test 6.4)
    ensure_daemon

    # Source 1: Health endpoint
    local health_body
    health_body=$(get_http_body "http://localhost:${PORT}/health")
    local health_version
    health_version=$(echo "$health_body" | jq -r '.version // empty' 2>/dev/null)

    if [ -z "$health_version" ]; then
        fail "Could not get version from health endpoint. Body: $(truncate "$health_body")"
        return
    fi

    # Source 2: Initialize response
    local init_request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"uat-version","version":"1.0"}}}'
    local init_response
    init_response=$(send_mcp "$init_request" "version_init")
    local init_version
    init_version=$(echo "$init_response" | jq -r '.result.serverInfo.version // empty' 2>/dev/null)

    if [ -z "$init_version" ]; then
        fail "Could not get version from initialize response. Response: $(truncate "$init_response")"
        return
    fi

    # Source 3: --version flag
    local cli_version
    cli_version=$("$WRAPPER" --version 2>&1 | awk '{print $NF}' | sed 's/^v//' | tr -d '[:space:]')

    if [ -z "$cli_version" ]; then
        fail "Could not get version from --version flag."
        return
    fi

    # Compare all three against VERSION file
    local mismatches=""
    if [ "$health_version" != "$VERSION" ]; then
        mismatches="${mismatches}health='$health_version' "
    fi
    if [ "$init_version" != "$VERSION" ]; then
        mismatches="${mismatches}initialize='$init_version' "
    fi
    if [ "$cli_version" != "$VERSION" ]; then
        mismatches="${mismatches}--version='$cli_version' "
    fi

    if [ -n "$mismatches" ]; then
        fail "Version mismatch against VERSION file ('$VERSION'): $mismatches"
        return
    fi

    pass "All 3 version sources match VERSION file: health='$health_version', initialize='$init_version', --version='$cli_version' (all == '$VERSION')."
}
run_test_6_5

# ── Done ──────────────────────────────────────────────────
finish_category
