#!/bin/bash
# framework.sh — Shared test harness for Kaboom MCP UAT.
# Sourced by each category file. Provides assertion helpers,
# MCP request sending, daemon lifecycle, and structured output.
set -eo pipefail

# Category traffic is synthetic, including when a category is run standalone.
export KABOOM_TELEMETRY=off

FRAMEWORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DAEMON_CLEANER="$FRAMEWORK_DIR/../../cleanup-test-daemons.sh"
# shellcheck source=uat-user-state.sh
source "$FRAMEWORK_DIR/uat-user-state.sh"

# ── Timeout Compatibility ──────────────────────────────────
# macOS doesn't ship with `timeout`. Use gtimeout from coreutils if available.
if command -v timeout >/dev/null 2>&1; then
    TIMEOUT_CMD="timeout"
elif command -v gtimeout >/dev/null 2>&1; then
    TIMEOUT_CMD="gtimeout"
else
    echo "FATAL: 'timeout' not found. Install with: brew install coreutils" >&2
    exit 1
fi

# ── Globals ────────────────────────────────────────────────
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
MCP_ID=100
TEMP_DIR=""
WRAPPER=""
VERSION=""
CATEGORY_NAME=""
CATEGORY_ID=""
RESULTS_FILE=""
OUTPUT_FILE=""
START_TIME=""
DAEMON_PID=""
MCP_TIMEOUT_SECONDS="${MCP_TIMEOUT_SECONDS:-35}"
MCP_MULTI_TIMEOUT_SECONDS="${MCP_MULTI_TIMEOUT_SECONDS:-40}"
MCP_STARTUP_RETRIES="${MCP_STARTUP_RETRIES:-5}"
MCP_STARTUP_RETRY_SLEEP_SECONDS="${MCP_STARTUP_RETRY_SLEEP_SECONDS:-2}"

# ── Exit Cleanup ───────────────────────────────────────────
# Always run daemon cleanup on script exit so failed/interrupted tests do not
# leak persistent daemons.
framework_cleanup() {
    # Best effort: kill daemon tracked by this framework and cleanup by port.
    if [ -n "${PORT:-}" ] && [ -n "${WRAPPER:-}" ]; then
        kill_server || true
    elif [ -n "${PORT:-}" ]; then
        lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
        lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    fi

    # Always clean temporary artifacts.
    [ -n "${TEMP_DIR:-}" ] && rm -rf "$TEMP_DIR"

    # Global safety net for stale test binaries/daemons.
    # In parallel UAT, this must be disabled to avoid one category killing
    # daemons owned by other concurrently running categories.
    if [ "${KABOOM_TEST_DISABLE_GLOBAL_CLEANER:-0}" != "1" ] && [ -f "$TEST_DAEMON_CLEANER" ]; then
        bash "$TEST_DAEMON_CLEANER" --quiet >/dev/null 2>&1 || true
    fi
}

# ── Init ───────────────────────────────────────────────────
init_framework() {
    PORT="${1:-7890}"
    RESULTS_FILE="${2:-/dev/null}"
    TEMP_DIR="$(mktemp -d)"
    trap framework_cleanup EXIT INT TERM
    START_TIME="$(date +%s)"

    # Resolve binary: explicit override > local build > PATH
    if [ -n "${KABOOM_UAT_WRAPPER:-}" ]; then
        if [ -x "${KABOOM_UAT_WRAPPER}" ]; then
            WRAPPER="${KABOOM_UAT_WRAPPER}"
        else
            echo "FATAL: KABOOM_UAT_WRAPPER is not executable: ${KABOOM_UAT_WRAPPER}" >&2
            exit 1
        fi
    elif [ -x "./kaboom-agentic-browser" ]; then
        WRAPPER="./kaboom-agentic-browser"
    elif command -v kaboom-agentic-browser >/dev/null 2>&1; then
        WRAPPER="kaboom-agentic-browser"
    else
        echo "FATAL: kaboom-agentic-browser not found in PATH or current directory" >&2
        exit 1
    fi

    # Read VERSION file
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local project_root="$script_dir/../../.."
    if [ -f "$project_root/VERSION" ]; then
        # shellcheck disable=SC2034 # VERSION used by sourcing scripts
        VERSION="$(tr -d '[:space:]' < "$project_root/VERSION")"
    else
        # shellcheck disable=SC2034 # VERSION used by sourcing scripts
        VERSION="unknown"
    fi

    # Output file for the runner to read
    OUTPUT_FILE="$TEMP_DIR/output.txt"
    touch "$OUTPUT_FILE"

    # Default to fail-fast for strictness. Comprehensive parallel runner can opt out
    # to collect full pass/fail evidence across all categories.
    if [ "${KABOOM_TEST_FAIL_FAST:-1}" = "0" ]; then
        set +e
    else
        set -e
    fi
}

# ── Category/Test Headers ──────────────────────────────────
begin_category() {
    CATEGORY_ID="$1"
    CATEGORY_NAME="$2"
    local count="$3"
    {
        echo ""
        echo "############################################################"
        echo "# CATEGORY ${CATEGORY_ID}: ${CATEGORY_NAME} (${count} tests)"
        echo "############################################################"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

begin_test() {
    local id="$1"
    local name="$2"
    local purpose="$3"
    local trust="$4"
    {
        echo "============================================================"
        echo "TEST ${id}: ${name}"
        echo "============================================================"
        echo "Purpose: ${purpose}"
        echo "Trust:   ${trust}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

# ── Pass/Fail ──────────────────────────────────────────────
pass() {
    local description="$1"
    PASS_COUNT="$((PASS_COUNT + 1))"
    {
        echo "  PASS: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

fail() {
    local description="$1"
    FAIL_COUNT="$((FAIL_COUNT + 1))"
    {
        echo "  FAIL: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

skip() {
    local description="$1"
    SKIP_COUNT="$((SKIP_COUNT + 1))"
    {
        echo "  SKIP: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

# ── MCP Request Sending ───────────────────────────────────
# Sends raw JSON-RPC via stdio to the wrapper binary.
# Sets globals: LAST_RESPONSE, LAST_EXIT_CODE
# Returns: the response text on stdout
send_mcp() {
    local request="$1"
    local prefix="${2:-mcp}"
    local max_retries="$MCP_STARTUP_RETRIES"

    # Self-heal if TEMP_DIR was deleted (e.g., by a cleanup trap in a prior subshell)
    if [ ! -d "$TEMP_DIR" ]; then
        mkdir -p "$TEMP_DIR" 2>/dev/null || TEMP_DIR="$(mktemp -d)"
    fi

    for attempt in $(seq 0 "$max_retries"); do
        local stdout_file="$TEMP_DIR/${prefix}_${MCP_ID}_stdout.txt"
        local stderr_file="$TEMP_DIR/${prefix}_${MCP_ID}_stderr.txt"
        local stderr_text=""

        # Use || true to prevent set -eo pipefail from killing the script on timeout (exit 124)
        echo "$request" | "$TIMEOUT_CMD" "$MCP_TIMEOUT_SECONDS" "$WRAPPER" --port "$PORT" > "$stdout_file" 2>"$stderr_file" || true
        # shellcheck disable=SC2034 # LAST_EXIT_CODE used by sourcing scripts
        LAST_EXIT_CODE="${PIPESTATUS[1]:-$?}"

        # Get last non-empty line (the JSON-RPC response)
        LAST_RESPONSE="$(grep -v '^$' "$stdout_file" 2>/dev/null | tail -1 || true)"
        stderr_text="$(cat "$stderr_file" 2>/dev/null || true)"
        # shellcheck disable=SC2034 # LAST_STDOUT_FILE used by sourcing scripts
        LAST_STDOUT_FILE="$stdout_file"
        # shellcheck disable=SC2034 # LAST_STDERR_FILE used by sourcing scripts
        LAST_STDERR_FILE="$stderr_file"

        # Retry on "starting up" — daemon needs more time to initialize
        if { echo "$LAST_RESPONSE" | grep -q "starting up" 2>/dev/null || echo "$stderr_text" | grep -qi "starting up" 2>/dev/null; } && [ "$attempt" -lt "$max_retries" ]; then
            echo "  [retry] Daemon starting up, waiting ${MCP_STARTUP_RETRY_SLEEP_SECONDS}s... (attempt $((attempt + 1))/$max_retries)" >&2
            sleep "$MCP_STARTUP_RETRY_SLEEP_SECONDS"
            continue
        fi

        # Never allow a silent empty response: synthesize a structured transport error.
        if [ -z "$LAST_RESPONSE" ]; then
            local stderr_tail
            local reason
            stderr_tail="$(tail -n 5 "$stderr_file" 2>/dev/null | tr '\n' ' ' | sed 's/"/\\"/g' | sed 's/[[:space:]]\+/ /g')"
            if [ "$LAST_EXIT_CODE" = "124" ]; then
                reason="timeout after ${MCP_TIMEOUT_SECONDS}s waiting for wrapper response"
            else
                reason="wrapper returned no stdout payload"
            fi
            LAST_RESPONSE="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Error: transport_no_response — ${reason}. exit_code=${LAST_EXIT_CODE}. stderr=${stderr_tail}\"}],\"isError\":true}}"
        fi
        break
    done

    MCP_ID="$((MCP_ID + 1))"
    echo "$LAST_RESPONSE"
}

# Sends multiple requests in a single pipe. Returns all non-empty stdout lines.
send_mcp_multi() {
    local requests="$1"
    local prefix="${2:-multi}"
    local stdout_file="$TEMP_DIR/${prefix}_${MCP_ID}_stdout.txt"
    local stderr_file="$TEMP_DIR/${prefix}_${MCP_ID}_stderr.txt"

    echo "$requests" | "$TIMEOUT_CMD" "$MCP_MULTI_TIMEOUT_SECONDS" "$WRAPPER" --port "$PORT" > "$stdout_file" 2>"$stderr_file" || true
    # shellcheck disable=SC2034 # LAST_EXIT_CODE used by sourcing scripts
    LAST_EXIT_CODE="${PIPESTATUS[1]:-$?}"
    # shellcheck disable=SC2034 # LAST_STDOUT_FILE used by sourcing scripts
    LAST_STDOUT_FILE="$stdout_file"
    # shellcheck disable=SC2034 # LAST_STDERR_FILE used by sourcing scripts
    LAST_STDERR_FILE="$stderr_file"

    MCP_ID="$((MCP_ID + 1))"
    grep -v '^$' "$stdout_file" 2>/dev/null || true
}

# Builds a tools/call JSON-RPC request and sends it.
# Usage: call_tool "observe" '{"what":"page"}'
call_tool() {
    local tool_name="$1"
    local arguments="${2:-\{\}}"
    local request="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/call\",\"params\":{\"name\":\"${tool_name}\",\"arguments\":${arguments}}}"
    send_mcp "$request" "call_${tool_name}"
}

# ── Response Extraction ────────────────────────────────────
# Extracts result.content[0].text from a JSON-RPC tool response
extract_content_text() {
    local response="$1"
    echo "$response" | jq -r '.result.content[0].text // empty' 2>/dev/null || true
}

# Truncates a string for display in pass/fail messages
truncate() {
    local text="$1"
    local max="${2:-300}"
    if [ ${#text} -gt "$max" ]; then
        echo "${text:0:$max}..."
    else
        echo "$text"
    fi
}

# ── Assertion Helpers ──────────────────────────────────────
# Each returns 0 on success, 1 on failure.
# They do NOT call pass/fail — the caller decides how to report.
# This allows multi-assertion tests to use early-return-on-failure.

check_not_error() {
    local response="$1"
    local is_error
    is_error="$(echo "$response" | jq -r '.result.isError // false' 2>/dev/null)"
    [ "$is_error" != "true" ]
}

check_is_error() {
    local response="$1"
    local is_error
    is_error="$(echo "$response" | jq -r '.result.isError // false' 2>/dev/null)"
    [ "$is_error" = "true" ]
}

check_json_field() {
    local json="$1"
    local jq_path="$2"
    local expected="$3"
    local actual
    actual="$(echo "$json" | jq -r "$jq_path" 2>/dev/null)"
    [ "$actual" = "$expected" ]
}

check_json_has() {
    local json="$1"
    local jq_path="$2"
    local value
    if value="$(echo "$json" | jq -e "$jq_path" 2>/dev/null)"; then
        [ "$value" != "null" ]
    else
        return 1
    fi
}

check_contains() {
    local haystack="$1"
    local needle="$2"
    echo "$haystack" | grep -qF "$needle"
}

# Like check_contains but uses extended regex (supports alternation with |)
check_matches() {
    local haystack="$1"
    local pattern="$2"
    echo "$haystack" | grep -qiE "$pattern"
}

check_protocol_error() {
    local response="$1"
    local expected_code="$2"
    local code
    code="$(echo "$response" | jq -r '.error.code // empty' 2>/dev/null)"
    [ "$code" = "$expected_code" ]
}

check_valid_jsonrpc() {
    local line="$1"
    echo "$line" | jq -e '.jsonrpc == "2.0"' >/dev/null 2>&1
}

# Returns 0 if response is a bridge→daemon connection timeout (expected without extension)
check_bridge_timeout() {
    local response="$1"
    if echo "$response" | jq -e '.error.message | test("deadline exceeded|connection refused|EOF|transport_no_response")' >/dev/null 2>&1; then
        return 0
    fi
    local text
    text="$(extract_content_text "$response")"
    echo "$text" | grep -qiE "transport_no_response|deadline exceeded|connection refused|Server connection error|EOF"
}

check_http_status() {
    local url="$1"
    local expected="$2"
    local extra_headers="${3:-}"
    local actual
    if [ -n "$extra_headers" ]; then
        actual="$(curl -s --max-time 10 --connect-timeout 3 -o /dev/null -w "%{http_code}" "$extra_headers" "$url" 2>/dev/null)"
    else
        actual="$(curl -s --max-time 10 --connect-timeout 3 -o /dev/null -w "%{http_code}" "$url" 2>/dev/null)"
    fi
    [ "$actual" = "$expected" ]
}

elapsed_seconds() {
    local started_at="$1"
    local finished_at="${2:-$(date +%s)}"
    echo "$((finished_at - started_at))"
}

json_boolean() {
    local payload="$1"
    local field="$2"
    echo "$payload" | jq -r --arg field "$field" \
        'if has($field) and (.[$field] | type == "boolean") then .[$field] else empty end' 2>/dev/null
}

get_http_status() {
    local url="$1"
    shift
    curl -s --max-time 10 --connect-timeout 3 -o /dev/null -w "%{http_code}" "$@" "$url" 2>/dev/null
}

get_http_body() {
    local url="$1"
    shift
    curl -s --max-time 10 --connect-timeout 3 "$@" "$url" 2>/dev/null
}

# ── Daemon Lifecycle ───────────────────────────────────────
kill_server() {
    # Prefer killing the tracked daemon PID over indiscriminate lsof
    if [ -n "$DAEMON_PID" ]; then
        # SIGTERM first for clean shutdown, then SIGKILL if still alive
        kill "$DAEMON_PID" 2>/dev/null || true
        sleep 0.2
        kill -0 "$DAEMON_PID" 2>/dev/null && kill -9 "$DAEMON_PID" 2>/dev/null || true
        DAEMON_PID=""
        sleep 0.1
    fi
    # Kill by port (e.g., if daemon was pre-existing)
    lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
    sleep 0.2
    lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    # Also clean up via PID file — catches zombie daemons that are alive
    # but no longer listening on the port
    "$WRAPPER" --stop --port "$PORT" >/dev/null 2>&1 || true
    sleep 0.1
}

wait_for_health() {
    local max_attempts="${1:-50}"
    for i in $(seq 1 "$max_attempts"); do
        # Use 127.0.0.1 (not localhost) to skip IPv6 ::1 fallback delay — daemon binds IPv4 only
        if curl -s --connect-timeout 3 "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
            return 0
        fi
        # Exponential backoff: 10ms → 50ms → 100ms
        # Typical startup is <100ms, so this is much faster than fixed 0.1s
        if [ "$i" -lt 3 ]; then
            sleep 0.01
        elif [ "$i" -lt 10 ]; then
            sleep 0.05
        else
            sleep 0.1
        fi
    done
    return 1
}

wait_for_required_connected_browser() {
    [ "${KABOOM_UAT_REQUIRE_CONNECTED:-0}" = "1" ] || return 0
    if ! uat_wait_for_connected_browser "$PORT" "$WRAPPER"; then
        echo "WARNING: connected browser readiness failed: $UAT_CONNECTED_READINESS_REASON" >&2
        return 1
    fi
    echo "  Connected browser ready: extension heartbeat and tracked tab available"
}

start_daemon() {
    # Kill any existing daemon first to prevent PID leaks
    kill_server
    # --parallel: isolate state dir so parallel test daemons don't trigger takeover logic
    # KABOOM_DEBUG=1: enable debug endpoints (/debug/usage, /debug/beacon-flush) for metrics tests
    if [ -n "${KABOOM_UAT_GOCOVERDIR:-}" ]; then
        mkdir -p "${KABOOM_UAT_GOCOVERDIR}"
        nohup env KABOOM_DEBUG=1 GOCOVERDIR="${KABOOM_UAT_GOCOVERDIR}" "$WRAPPER" --daemon --parallel --port "$PORT" >/dev/null 2>&1 < /dev/null &
    else
        nohup env KABOOM_DEBUG=1 "$WRAPPER" --daemon --parallel --port "$PORT" >/dev/null 2>&1 < /dev/null &
    fi
    DAEMON_PID=$!
    if ! wait_for_health 50; then
        echo "WARNING: daemon on port $PORT not healthy after startup (PID $DAEMON_PID)" >&2
        return 1
    fi
    wait_for_required_connected_browser || return 1
    # Print daemon version to catch stale binary issues
    local daemon_ver
    daemon_ver="$(curl -s --connect-timeout 3 "http://127.0.0.1:${PORT}/health" 2>/dev/null | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")"
    echo "  Daemon started: v${daemon_ver} (PID $DAEMON_PID, port $PORT)"
}

start_daemon_with_flags() {
    # Kill any existing daemon first to prevent PID leaks
    kill_server
    # --parallel: isolate state dir so parallel test daemons don't trigger takeover logic
    # KABOOM_DEBUG=1: enable debug endpoints for metrics tests
    if [ -n "${KABOOM_UAT_GOCOVERDIR:-}" ]; then
        mkdir -p "${KABOOM_UAT_GOCOVERDIR}"
        nohup env KABOOM_DEBUG=1 GOCOVERDIR="${KABOOM_UAT_GOCOVERDIR}" "$WRAPPER" --daemon --parallel --port "$PORT" "$@" >/dev/null 2>&1 < /dev/null &
    else
        nohup env KABOOM_DEBUG=1 "$WRAPPER" --daemon --parallel --port "$PORT" "$@" >/dev/null 2>&1 < /dev/null &
    fi
    DAEMON_PID=$!
    if ! wait_for_health 50; then
        echo "WARNING: daemon on port $PORT not healthy after startup (PID $DAEMON_PID)" >&2
        return 1
    fi
    wait_for_required_connected_browser || return 1
    # Print daemon version to catch stale binary issues
    local daemon_ver
    daemon_ver="$(curl -s --connect-timeout 3 "http://127.0.0.1:${PORT}/health" 2>/dev/null | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")"
    echo "  Daemon started: v${daemon_ver} (PID $DAEMON_PID, port $PORT)"
}

ensure_daemon() {
    if ! curl -s --connect-timeout 3 "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
        start_daemon
    fi
}

# ── Category Finish ────────────────────────────────────────
finish_category() {
    # Kill our daemon (tracked PID + port fallback)
    kill_server
    # Safety net: also kill by port in case DAEMON_PID was stale
    lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true

    # Clean up temp
    local elapsed
    elapsed="$(elapsed_seconds "$START_TIME")"

    # Write structured results for the runner
    if [ "$RESULTS_FILE" != "/dev/null" ]; then
        cat > "$RESULTS_FILE" <<RESULTS_EOF
PASS_COUNT=$PASS_COUNT
FAIL_COUNT=$FAIL_COUNT
SKIP_COUNT=$SKIP_COUNT
ELAPSED=${elapsed}
CATEGORY_ID=$CATEGORY_ID
CATEGORY_NAME="$CATEGORY_NAME"
RESULTS_EOF
    fi

    # Exit with appropriate code
    if [ "$FAIL_COUNT" -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}
