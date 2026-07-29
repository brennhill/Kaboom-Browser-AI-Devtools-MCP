#!/bin/bash
# cat-28-terminal.sh — Automated terminal HTTP endpoint tests (4 tests).
# Pure HTTP — no extension or human required.
# Terminal endpoints live on PORT+1 (dedicated terminal server).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "28" "Terminal HTTP Endpoints" "4"
ensure_daemon

TERM_PORT=$((PORT + 1))

# ── 28.1 — Terminal page served ────────────────────────────
begin_test "28.1" "Terminal page served by daemon" \
    "GET /terminal on PORT+1 — verify HTML page with xterm.js" \
    "HTTP: daemon serves the embedded terminal page on the isolated terminal server."
run_test_28_1() {
    local status
    status=$(get_http_status "http://localhost:${TERM_PORT}/terminal")
    if [ "$status" != "200" ]; then
        fail "GET /terminal on port $TERM_PORT returned HTTP $status (expected 200)."
        return
    fi

    local body
    body=$(get_http_body "http://localhost:${TERM_PORT}/terminal")
    if echo "$body" | grep -q "xterm"; then
        pass "Terminal page served on port $TERM_PORT with xterm.js."
    else
        fail "Terminal page on port $TERM_PORT does not contain xterm.js references."
    fi
}
run_test_28_1

# ── 28.2 — Session start/stop lifecycle ───────────────────
begin_test "28.2" "Terminal session start/stop lifecycle" \
    "POST /terminal/start then /terminal/stop — verify session lifecycle" \
    "HTTP: session creation returns token and PID, stop cleans up."
run_test_28_2() {
    local start_resp
    start_resp=$(curl -s --max-time 10 -X POST "http://localhost:${TERM_PORT}/terminal/start" \
        -H "Content-Type: application/json" \
        -d '{"cmd":"/bin/sh","args":["-c","exec cat"]}')

    local token session_id
    token=$(echo "$start_resp" | jq -r '.token // empty' 2>/dev/null)
    session_id=$(echo "$start_resp" | jq -r '.session_id // empty' 2>/dev/null)

    if [ -z "$token" ] || [ -z "$session_id" ]; then
        fail "Start response missing token or session_id: $start_resp"
        return
    fi

    # Stop the session
    local stop_resp
    stop_resp=$(curl -s --max-time 10 -X POST "http://localhost:${TERM_PORT}/terminal/stop" \
        -H "Content-Type: application/json" \
        -d "{\"id\":\"$session_id\"}")
    local stop_status
    stop_status=$(echo "$stop_resp" | jq -r '.status // empty' 2>/dev/null)

    if [ "$stop_status" = "stopped" ]; then
        pass "Terminal session started (token=$token) and stopped cleanly."
    else
        fail "Stop response unexpected: $stop_resp"
    fi
}
run_test_28_2

# ── 28.3 — Session validate endpoint ──────────────────────
begin_test "28.3" "Session validate endpoint" \
    "Start session, validate token (true), stop, validate again (false)" \
    "Token validation is used by the extension to detect stale sessions on reconnect."
run_test_28_3() {
    # Start a session
    local start_resp
    start_resp=$(curl -s --max-time 10 -X POST "http://localhost:${TERM_PORT}/terminal/start" \
        -H "Content-Type: application/json" \
        -d '{"id":"validate-test","cmd":"/bin/sh","args":["-c","exec cat"]}')

    local token session_id
    token=$(echo "$start_resp" | jq -r '.token // empty' 2>/dev/null)
    session_id=$(echo "$start_resp" | jq -r '.session_id // empty' 2>/dev/null)

    if [ -z "$token" ]; then
        fail "Start response missing token: $start_resp"
        return
    fi

    # Validate — should be true
    local valid_resp
    valid_resp=$(get_http_body "http://localhost:${TERM_PORT}/terminal/validate?token=${token}")
    local is_valid
    is_valid=$(json_boolean "$valid_resp" valid)

    if [ "$is_valid" != "true" ]; then
        fail "Validate returned $is_valid for live session (expected true). Response: $valid_resp"
        curl -s --max-time 5 -X POST "http://localhost:${TERM_PORT}/terminal/stop" \
            -H "Content-Type: application/json" -d "{\"id\":\"$session_id\"}" >/dev/null 2>&1
        return
    fi

    # Stop the session
    curl -s --max-time 5 -X POST "http://localhost:${TERM_PORT}/terminal/stop" \
        -H "Content-Type: application/json" -d "{\"id\":\"$session_id\"}" >/dev/null 2>&1

    # Validate again — should be false
    valid_resp=$(get_http_body "http://localhost:${TERM_PORT}/terminal/validate?token=${token}")
    is_valid=$(json_boolean "$valid_resp" valid)

    if [ "$is_valid" = "false" ]; then
        pass "Validate returns true for live session, false after stop."
    else
        fail "Validate returned $is_valid after stop (expected false). Response: $valid_resp"
    fi
}
run_test_28_3

# ── 28.4 — Terminal config endpoint ───────────────────────
begin_test "28.4" "Terminal config endpoint shows session count" \
    "Start session, GET /terminal/config, verify count=1, then stop" \
    "Config endpoint is used by the dashboard to show active sessions."
run_test_28_4() {
    # Start a session
    local start_resp
    start_resp=$(curl -s --max-time 10 -X POST "http://localhost:${TERM_PORT}/terminal/start" \
        -H "Content-Type: application/json" \
        -d '{"id":"config-test","cmd":"/bin/sh","args":["-c","exec cat"]}')

    local session_id
    session_id=$(echo "$start_resp" | jq -r '.session_id // empty' 2>/dev/null)

    if [ -z "$session_id" ]; then
        fail "Start response missing session_id: $start_resp"
        return
    fi

    # Check config
    local config_resp
    config_resp=$(get_http_body "http://localhost:${TERM_PORT}/terminal/config")
    local count
    count=$(echo "$config_resp" | jq -r '.count // 0' 2>/dev/null)

    # Clean up
    curl -s --max-time 5 -X POST "http://localhost:${TERM_PORT}/terminal/stop" \
        -H "Content-Type: application/json" -d "{\"id\":\"$session_id\"}" >/dev/null 2>&1

    if [ "$count" -ge 1 ]; then
        pass "Config endpoint shows count=$count with active session."
    else
        fail "Config shows count=$count after start (expected >= 1). Response: $config_resp"
    fi
}
run_test_28_4

finish_category
