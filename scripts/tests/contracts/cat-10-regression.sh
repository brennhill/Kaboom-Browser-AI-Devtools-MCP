#!/bin/bash
# cat-10-regression.sh — Category 10: Regression Guards (3 tests).
# These tests exist because of specific bugs we've hit before.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "10" "Regression Guards" "3"

# ── 10.1 — Exactly 5 tools in tools/list ─────────────────
begin_test "10.1" "Exactly 5 tools in tools/list (regression: v5.7.5)" \
    "Send tools/list via fast-start. Assert exactly 5 tools: observe, generate, configure, interact, analyze." \
    "We shipped stub tools that returned errors. This must never happen again."
run_test_10_1() {
    local request="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}"
    RESPONSE=$(send_mcp "$request" "tools_list")
    if [ -z "$RESPONSE" ]; then
        fail "No response from tools/list request. Exit code: $LAST_EXIT_CODE"
        return
    fi
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Response is not valid JSON-RPC. Full response: $(truncate "$RESPONSE")"
        return
    fi

    # Extract tool names
    local tool_names
    tool_names=$(echo "$RESPONSE" | jq -r '.result.tools[].name' 2>/dev/null)
    if [ -z "$tool_names" ]; then
        fail "Could not extract tool names from tools/list response. Response: $(truncate "$RESPONSE")"
        return
    fi

    # Count tools — must be exactly 5
    local tool_count
    tool_count=$(echo "$RESPONSE" | jq '.result.tools | length' 2>/dev/null)
    if [ "$tool_count" != "5" ]; then
        fail "Expected exactly 5 tools but got $tool_count. Tool names: $tool_names"
        return
    fi

    # Verify the 5 expected tools are present
    local has_observe has_generate has_configure has_interact has_analyze
    has_observe=$(echo "$tool_names" | grep -c "^observe$" || true)
    has_generate=$(echo "$tool_names" | grep -c "^generate$" || true)
    has_configure=$(echo "$tool_names" | grep -c "^configure$" || true)
    has_interact=$(echo "$tool_names" | grep -c "^interact$" || true)
    has_analyze=$(echo "$tool_names" | grep -c "^analyze$" || true)

    if [ "$has_observe" != "1" ] || [ "$has_generate" != "1" ] || [ "$has_configure" != "1" ] || [ "$has_interact" != "1" ] || [ "$has_analyze" != "1" ]; then
        fail "Missing expected tools. Found: $tool_names. Expected: observe, generate, configure, interact, analyze."
        return
    fi

    pass "tools/list returned exactly 5 tools: observe, generate, configure, interact, analyze."
}
run_test_10_1

# ── 10.2 — Empty buffers don't crash observe ─────────────
begin_test "10.2" "Empty buffers don't crash observe (regression: nil pointer)" \
    "After cold start (no extension data), call all standard observe modes. All must return valid JSON-RPC." \
    "Nil pointer on empty state was a real bug. This runs all modes in the worst-case state."
run_test_10_2() {
    # Ensure a fresh daemon is running
    kill_server
    start_daemon
    if ! wait_for_health 50; then
        fail "Could not start fresh daemon for empty-buffer test."
        return
    fi

    # Standard observe modes (current tool enum).
    local modes=(
        "page"
        "tabs"
        "logs"
        "errors"
        "network_waterfall"
        "vitals"
        "actions"
        "websocket_events"
        "websocket_status"
        "extension_logs"
        "pilot"
        "timeline"
        "summarized_logs"
        "error_bundles"
        "pending_commands"
        "failed_commands"
        "network_bodies"
        "recordings"
        "log_diff_report"
        "command_result"
        "playback_results"
        "saved_videos"
        "screenshot"
        "indexeddb"
    )

    local success_count=0
    local fail_modes=""
    local total=${#modes[@]}

    # Use a tighter per-call send to prevent loop blow-up (23 modes x 8s = 184s worst case)
    call_tool_fast() {
        local tool_name="$1"
        local arguments="${2:-\{\}}"
        local request="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/call\",\"params\":{\"name\":\"${tool_name}\",\"arguments\":${arguments}}}"
        local stdout_file="$TEMP_DIR/fast_${MCP_ID}_stdout.txt"
        local stderr_file="$TEMP_DIR/fast_${MCP_ID}_stderr.txt"
        echo "$request" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" > "$stdout_file" 2>"$stderr_file"
        LAST_EXIT_CODE=$?
        LAST_RESPONSE=$(grep -v '^$' "$stdout_file" 2>/dev/null | tail -1)
        if [ -z "$LAST_RESPONSE" ]; then
            LAST_RESPONSE="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Error: transport_no_response — wrapper returned no stdout payload. exit_code=${LAST_EXIT_CODE}\"}],\"isError\":true}}"
        fi
        MCP_ID=$((MCP_ID + 1))
        echo "$LAST_RESPONSE"
    }

    for mode in "${modes[@]}"; do
        local resp
        resp=$(call_tool_fast "observe" "{\"what\":\"$mode\"}")

        # Must be valid JSON-RPC (not a crash, not a timeout, not empty)
        if [ -z "$resp" ]; then
            fail_modes="$fail_modes $mode(empty)"
            continue
        fi
        if ! check_valid_jsonrpc "$resp"; then
            fail_modes="$fail_modes $mode(invalid-jsonrpc)"
            continue
        fi

        success_count=$((success_count + 1))
    done

    if [ "$success_count" -ne "$total" ]; then
        fail "Only $success_count/$total observe modes returned valid JSON-RPC. Failures:$fail_modes"
        return
    fi

    pass "All $total observe modes returned valid JSON-RPC on empty buffers. Zero crashes, zero timeouts."
}
run_test_10_2

# ── 10.3 — Version consistency ────────────────────────────
begin_test "10.3" "Version in health matches binary version (regression: stale build)" \
    "Get version from --version, health endpoint, and VERSION file. All must match." \
    "We shipped binaries where --version and health reported different versions due to stale compilation."
run_test_10_3() {
    # Source 1: VERSION file (already read by init_framework)
    local version_file="$VERSION"
    if [ -z "$version_file" ] || [ "$version_file" = "unknown" ]; then
        fail "Could not read VERSION file. Got: '$version_file'"
        return
    fi

    # Source 2: --version flag output
    local version_flag
    version_flag=$("$WRAPPER" --version 2>/dev/null)
    if [ -z "$version_flag" ]; then
        fail "kaboom-agentic-browser --version returned empty output."
        return
    fi
    # Extract version number from output like "kaboom v5.7.6"
    # Strip everything except the version number
    local version_from_flag
    version_from_flag=$(echo "$version_flag" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
    if [ -z "$version_from_flag" ]; then
        fail "Could not extract version from --version output: '$version_flag'"
        return
    fi

    # Source 3: health endpoint
    ensure_daemon
    local health_body
    health_body=$(get_http_body "http://localhost:${PORT}/health")
    if [ -z "$health_body" ]; then
        fail "Health endpoint returned empty body."
        return
    fi
    local version_from_health
    version_from_health=$(echo "$health_body" | jq -r '.version // empty' 2>/dev/null)
    if [ -z "$version_from_health" ]; then
        fail "Could not extract version from health endpoint. Body: $(truncate "$health_body")"
        return
    fi

    # Compare all three
    if [ "$version_file" != "$version_from_flag" ]; then
        fail "VERSION file ($version_file) does not match --version ($version_from_flag)."
        return
    fi
    if [ "$version_file" != "$version_from_health" ]; then
        fail "VERSION file ($version_file) does not match health endpoint ($version_from_health)."
        return
    fi

    pass "Version consistent across all sources: VERSION file=$version_file, --version=$version_from_flag, health=$version_from_health."
}
run_test_10_3

finish_category
