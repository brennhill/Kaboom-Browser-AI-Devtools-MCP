#!/bin/bash
# 23-doctor-preflight.sh — 23.1-23.3: Doctor mode preflight tests.
# Verifies --doctor output, the removed alias, exit codes, and diagnostic checks.
set -eo pipefail

begin_category "23" "Doctor Preflight" "3"

# ── Test 23.1: --doctor exits with structured output ─────
begin_test "23.1" "[DAEMON ONLY] Doctor mode produces structured output" \
    "kaboom-agentic-browser --doctor should run diagnostics and exit" \
    "Tests: doctor preflight runs and produces readable output"

run_test_23_1() {
    local output exit_code
    output=$("$WRAPPER" --doctor --port "$PORT" 2>&1) || exit_code=$?

    if [ -z "$output" ]; then
        fail "Doctor mode produced no output."
        return
    fi

    log_diagnostic "23.1" "doctor" "$output"

    # Doctor output should mention version and port
    if echo "$output" | grep -qi "version\|port\|kaboom"; then
        pass "Doctor mode produced structured output (${#output} chars). Exit code: ${exit_code:-0}"
    else
        fail "Doctor output missing expected fields. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_1

# ── Test 23.2: removed alias is rejected ────────────────
begin_test "23.2" "[DAEMON ONLY] Removed check alias is rejected" \
    "kaboom-agentic-browser --check should fail as an unknown flag" \
    "Tests: compatibility facade stays deleted"

run_test_23_2() {
    local output exit_code=0
    output=$("$WRAPPER" --check --port "$PORT" 2>&1) || exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        fail "Removed --check alias was accepted."
        return
    fi

    if echo "$output" | grep -qi "unknown\\|not defined\\|invalid"; then
        pass "Removed --check alias is rejected."
    else
        fail "Removed --check alias failed without an unknown-flag diagnostic. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_2

# ── Test 23.3: Doctor checks port availability ──────────
begin_test "23.3" "[DAEMON ONLY] Doctor reports port status" \
    "Doctor should indicate whether the configured port is available or in use" \
    "Tests: port availability preflight check"

run_test_23_3() {
    # Start daemon on PORT first so doctor detects it
    ensure_server_running

    local output
    output=$("$WRAPPER" --doctor --port "$PORT" 2>&1) || true

    log_diagnostic "23.3" "doctor-port" "$output"

    # Should mention port status (available, in use, running, etc.)
    if echo "$output" | grep -qi "port\|running\|pid\|listening\|available\|in.use"; then
        pass "Doctor reports port status information."
    else
        fail "Doctor output missing port status. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_3
