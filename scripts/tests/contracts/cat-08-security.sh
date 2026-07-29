#!/bin/bash
# cat-08-security.sh — UAT tests for security middleware (4 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "8" "Security" "4"
ensure_daemon

# ── 8.1 — Extension endpoints reject without X-Kaboom-Client header ──
begin_test "8.1" "Extension endpoints reject without X-Kaboom-Client header" \
    "Verify /sync returns 403 when no X-Kaboom-Client header is present" \
    "httpguard.ExtensionOnly middleware must actually block. Without it, any local process can inject data."
run_test_8_1() {
    local status
    status=$(get_http_status "http://localhost:${PORT}/sync")
    if [ "$status" = "403" ]; then
        pass "/sync without X-Kaboom-Client header returned HTTP 403 as expected."
    else
        fail "/sync without X-Kaboom-Client header returned HTTP $status, expected 403."
    fi
}
run_test_8_1

# ── 8.2 — Extension endpoints accept valid header ─────────
begin_test "8.2" "Extension endpoints accept valid X-Kaboom-Client header" \
    "Verify /sync with valid header does NOT return 403 (400 for missing body is acceptable)" \
    "The middleware must not over-block. Valid extension requests must pass."
run_test_8_2() {
    local status
    status=$(get_http_status "http://localhost:${PORT}/sync" -H "X-Kaboom-Client: kaboom-extension/${VERSION}" -X POST)
    if [ "$status" = "200" ] || [ "$status" = "400" ]; then
        pass "/sync with valid X-Kaboom-Client header returned HTTP $status. Middleware accepted the request."
    else
        fail "/sync with valid X-Kaboom-Client header returned HTTP $status. Expected 200 or 400 (missing body), not $status."
    fi
}
run_test_8_2

# ── 8.3 — CORS rejects non-localhost origins ──────────────
begin_test "8.3" "CORS rejects non-localhost origins" \
    "Verify requests with Origin: https://evil.com are rejected with 403" \
    "DNS rebinding / CORS bypass is the primary attack vector for local servers."
run_test_8_3() {
    local status
    status=$(get_http_status "http://localhost:${PORT}/health" -H "Origin: https://evil.com")
    if [ "$status" = "403" ]; then
        pass "/health with Origin: https://evil.com returned HTTP 403 as expected."
    else
        fail "/health with Origin: https://evil.com returned HTTP $status, expected 403."
    fi
}
run_test_8_3

# ── 8.4 — Host header validation rejects non-localhost ────
begin_test "8.4" "Host header validation rejects non-localhost" \
    "Verify requests with Host: evil.com are rejected with 403" \
    "DNS rebinding requires spoofed Host header. Must reject."
run_test_8_4() {
    local status
    status=$(get_http_status "http://127.0.0.1:${PORT}/health" -H "Host: evil.com")
    if [ "$status" = "403" ]; then
        pass "/health with Host: evil.com returned HTTP 403 as expected."
    else
        fail "/health with Host: evil.com returned HTTP $status, expected 403."
    fi
}
run_test_8_4

finish_category
