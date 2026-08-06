#!/bin/bash
# 30-stability-shutdown.sh — 30.1-30.3: Post-barrage stability and graceful shutdown.
# MUST BE LAST — 30.3 kills the daemon.
set -eo pipefail

begin_category "30" "Stability & Shutdown" "3"

# ── Test 30.1: observe(page) still works after all actions ─
begin_test "30.1" "[BROWSER] Page state survives action barrage" \
    "After navigate + JS execution + clicks + forms + WS, observe(page) still returns valid data" \
    "Verifies no corruption from heavy interaction"

run_test_30_1() {
    # EXTENSION_CONNECTED is captured at suite start; refresh runtime state here.
    local health_body
    health_body=$(get_http_body "http://localhost:${PORT}/health")
    if ! echo "$health_body" | jq -e '.capture.extension_connected == true' >/dev/null 2>&1; then
        fail "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "Extension is not connected"; then
        fail "Extension disconnected during observe(page) check."
    elif echo "$content_text" | grep -qE 'https?://'; then
        pass "observe(page) still returns a valid URL after all actions."
    else
        fail "observe(page) broken after actions. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_30_1

# ── Test 30.2: Health still OK after everything ──────────
begin_test "30.2" "[DAEMON ONLY] Health still OK after everything" \
    "Verify daemon is healthy after all the interaction and observation" \
    "Detects memory leaks, crashes, or degraded state"

run_test_30_2() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ "$status_val" != "ok" ]; then
        fail "Health status='$status_val' after test barrage. Body: $(truncate "$body")"
        return
    fi

    pass "Daemon still healthy after full smoke test. status='ok'."
}
run_test_30_2

# ── Test 30.3: Graceful shutdown ─────────────────────────
begin_test "30.3" "[DAEMON ONLY] Graceful shutdown via --stop" \
    "Run --stop, verify port is freed and PID file is cleaned up" \
    "Ungraceful shutdown leaves orphan processes and stale PID files"

run_test_30_3() {
    local stop_output
    stop_output=$("$WRAPPER" --stop --port "$PORT" 2>&1)
    local stop_exit=$?

    if [ $stop_exit -ne 0 ]; then
        fail "--stop exited with code $stop_exit. Output: $(truncate "$stop_output")"
        return
    fi

    # Poll for port release — srv.Shutdown() may drain active connections (WebSocket, /sync)
    # for up to 3s after the process signals exit. Check every 0.5s for up to 5s.
    local port_freed=false
    for i in $(seq 1 10); do
        if ! lsof -ti :"$PORT" >/dev/null 2>&1; then
            port_freed=true
            break
        fi
        sleep 0.5
    done

    if [ "$port_freed" != "true" ]; then
        local occupants
        occupants=$(lsof -ti :"$PORT" 2>/dev/null | head -3 || true)
        fail "Port $PORT still occupied 5s after --stop. PIDs on port: $occupants. srv.Shutdown() drain may be stuck on active connections."
        return
    fi

    # Check both new and legacy PID file paths
    local pid_file_legacy="$HOME/.kaboom-${PORT}.pid"
    # New path: ~/.local/share/kaboom/run/kaboom-PORT.pid (macOS/Linux)
    local state_dir="${XDG_DATA_HOME:-$HOME/.local/share}/kaboom/run"
    local pid_file_new="$state_dir/kaboom-${PORT}.pid"

    local leaked_pid=""
    if [ -f "$pid_file_legacy" ]; then
        leaked_pid="$pid_file_legacy"
        rm -f "$pid_file_legacy"
    fi
    if [ -f "$pid_file_new" ]; then
        leaked_pid="$pid_file_new"
        rm -f "$pid_file_new"
    fi

    if [ -n "$leaked_pid" ]; then
        fail "PID file '$leaked_pid' still exists after --stop. removePIDFile() may not have run."
        return
    fi

    pass "Graceful shutdown: --stop exited 0, port freed in ${i}x0.5s, PID file cleaned."
}
run_test_30_3
