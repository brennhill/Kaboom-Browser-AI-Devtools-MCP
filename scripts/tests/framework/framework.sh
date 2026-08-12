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
# shellcheck source=uat-fixture-state.sh
source "$FRAMEWORK_DIR/uat-fixture-state.sh"

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
        # Unique per call, not per MCP_ID: a background subshell inherits the
        # parent's MCP_ID unchanged (bash 3.2 has no BASHPID and $$ is shared),
        # so concurrent callers would otherwise interleave into one file and
        # read back a corrupted response.
        local capture_base
        capture_base="$(mktemp "$TEMP_DIR/${prefix}_XXXXXXXX")"
        local stdout_file="${capture_base}.out"
        local stderr_file="${capture_base}.err"
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
    local capture_base
    capture_base="$(mktemp "$TEMP_DIR/${prefix}_XXXXXXXX")"
    local stdout_file="${capture_base}.out"
    local stderr_file="${capture_base}.err"

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

# Extracts the stable nested command error from an MCP content response.
# Falls back to the bounded human-readable envelope when older responses do not
# include a structured command result.
command_failure_message() {
    local response="$1"
    local text=""
    local structured=""
    text="$(extract_content_text "$response")"
    structured="$(printf '%s\n' "$text" |
        sed -n '/^{/,$p' |
        jq -r '
            (.result.error // .error // empty) as $error |
            (.result.message // .message // empty) as $message |
            (.result.resolved_tab_id // .resolved_tab_id // empty) as $tab_id |
            (.result.resolved_url // .resolved_url // empty) as $url |
            (.result.target_context.source // .target_context.source // empty) as $source |
            (if $tab_id != "" then " [tab_id=\($tab_id) source=\($source) url=\($url)]" else "" end) as $target |
            if $error != "" and $message != "" then "\($error): \($message)\($target)"
            elif $message != "" then $message
            elif $error != "" then $error
            else empty end
        ' 2>/dev/null || true)"
    if [ -n "$structured" ]; then
        printf '%s\n' "$structured"
        return 0
    fi
    printf '%s\n' "$text"
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

# Returns 0 when the harness never received a usable answer — a malformed
# envelope, or the synthesized transport_no_response placeholder send_mcp emits
# on timeout. This is the difference between "the tool reported an error" (a
# legitimate outcome worth asserting on) and "the call hung or the protocol
# broke" (always a defect, whatever the feature's implementation status).
check_transport_failure() {
    local response="$1"
    if ! check_valid_jsonrpc "$response"; then
        return 0
    fi
    extract_content_text "$response" | grep -q "transport_no_response"
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

# Quotes an arbitrary string as a JSON scalar, for safe interpolation into
# tool arguments. Raw interpolation emits malformed JSON, which silently stops
# the action under test from being exercised at all.
json_string() {
    printf '%s' "$1" | jq -Rs .
}

# ── Fixture Routing ────────────────────────────────────────
#
# The daemon embeds a corpus of deterministic pages at /tests/ (see
# cmd/browser-agent/internal/testpages/pages). Until now every connected test
# drove interact.html, so modes that read console errors, network traffic,
# websocket frames, long tasks or layout shift were invoked against a page that
# contains none of those. They returned empty, and empty is not an error, so
# they passed while verifying nothing.
#
# fixture_url names a page; arm_fixture puts known state into it. Assert against
# uat-fixture-state.sh, which declares what each armed fixture contains.

# Resolves a fixture name to its daemon-served URL. Unknown names are a hard
# error rather than a silent fallback: falling back to interact.html is exactly
# how a mode ends up asserting against a page that cannot exercise it.
fixture_url() {
    local name="${1:-interact}"
    case "$name" in
        interact | telemetry | performance | a11y | recording | interaction-test | cdp-smoke-test | design-drift)
            echo "http://127.0.0.1:${PORT}/tests/${name}.html"
            ;;
        *)
            echo "FATAL: unknown fixture '$name'" >&2
            return 1
            ;;
    esac
}

# The default fixture for connected categories that only need a known DOM.
connected_fixture_url() {
    fixture_url interact
}

# Navigates the tracked tab to a fixture and waits for its readiness anchor.
ensure_fixture() {
    local name="${1:-interact}"
    local anchor=""
    case "$name" in
        interact) anchor="#sf-btn" ;;
        telemetry) anchor="#console-count" ;;
        performance) anchor="#cls-box" ;;
        a11y) anchor="#bad-input-name" ;;
        recording) anchor="#rec-indicator" ;;
        design-drift) anchor="#design-drift-ready" ;;
        *) anchor="body" ;;
    esac

    local url
    url="$(fixture_url "$name")" || return 1

    local response
    response="$(call_tool "interact" \
        '{"what":"navigate_and_wait_for","url":'"$(json_string "$url")"',"wait_for":'"$(json_string "$anchor")"'}')"
    if ! check_valid_jsonrpc "$response" || check_is_error "$response"; then
        return 1
    fi
    return 0
}

# Clears the capture buffers an assertion is about to read.
#
# Without this, counts are cumulative across every navigation the category has
# already performed, so "exactly three errors" is unassertable and tests degrade
# to "at least one of something". Clearing first is what makes exact counts
# meaningful, and it is why arming is clear -> navigate -> trigger rather than
# just navigate.
clear_capture_buffer() {
    local buffer="${1:-all}"
    call_tool "configure" '{"what":"clear","buffer":"'"$buffer"'"}' >/dev/null 2>&1 || true
}

# Runs a fixture's in-page generator and gives the capture pipeline time to
# deliver. Returns non-zero if the generator itself failed, so a caller never
# asserts against state that was never produced.
trigger_fixture() {
    local script="$1"
    local settle_seconds="${2:-2}"
    local response
    response="$(call_tool "interact" '{"what":"execute_js","script":'"$(json_string "$script")"'}')"
    if ! check_valid_jsonrpc "$response" || check_is_error "$response"; then
        return 1
    fi
    # The generator runs in the page; capture has to travel page -> extension ->
    # daemon before a read can see it. Measured delivery for the telemetry arm
    # was under 2s; asserting immediately reads an empty buffer and looks like a
    # capture failure.
    sleep "$settle_seconds"
    return 0
}

# Establishes a fixture in a known, asserted-against state.
#
# Idempotent by construction: the buffer clear means a retry re-establishes the
# same state rather than doubling the counts a caller is about to assert on.
arm_fixture() {
    local name="$1"
    local buffer="${2:-all}"

    clear_capture_buffer "$buffer"
    ensure_fixture "$name" || return 1
    # Clear again after navigation: the page load itself emits requests (and
    # telemetry.html auto-fires 404/500/CORS fetches 300ms in), which would
    # otherwise land in the buffer the caller is about to read.
    clear_capture_buffer "$buffer"

    case "$name" in
        telemetry)
            trigger_fixture "$UAT_ARM_TELEMETRY" || return 1
            ;;
        performance)
            trigger_fixture "$UAT_ARM_PERFORMANCE" 3 || return 1
            ;;
        a11y | interact | recording | *)
            # These fixtures carry their state statically; nothing to trigger.
            ;;
    esac
    return 0
}

# Extracts the recording_id from an event_recording_start response.
recording_id_from_response() {
    extract_content_text "$1" |
        sed -n 's/.*"recording_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
        head -n 1
}

# Starts DOM event recording and sets LAST_RECORDING_ID.
#
# Use this rather than interact/screen_recording_start when a test needs to
# record user actions. Screen recording captures video through getDisplayMedia
# and cannot start without an explicit browser user gesture — the extension
# replies "awaiting user gesture" until someone clicks Approve in the popup, so
# any unattended test that requires it to succeed will always fail. Event
# recording captures the action stream and needs no gesture.
start_event_recording() {
    local name="$1"
    local response
    response="$(call_tool "configure" '{"what":"event_recording_start","name":'"$(json_string "$name")"'}')"
    LAST_RECORDING_ID="$(recording_id_from_response "$response")"

    # Only one recording can be active at a time, so a test that fails before
    # stopping its own leaves every later start failing with already_recording.
    # That turned one real failure in cat-30 into three. The invariant belongs
    # here rather than in each caller's error paths: adopt and close the stale
    # recording, then start the one that was asked for.
    if [ -z "$LAST_RECORDING_ID" ]; then
        local stale
        stale="$(command_failure_message "$response" |
            sed -n 's/.*already active (id: \([^)]*\)).*/\1/p' | head -n 1)"
        if [ -n "$stale" ]; then
            call_tool "configure" '{"what":"event_recording_stop","recording_id":'"$(json_string "$stale")"'}' >/dev/null 2>&1
            response="$(call_tool "configure" '{"what":"event_recording_start","name":'"$(json_string "$name")"'}')"
            LAST_RECORDING_ID="$(recording_id_from_response "$response")"
        fi
    fi

    if [ -z "$LAST_RECORDING_ID" ]; then
        fail "event_recording_start returned no recording_id: $(truncate "$(command_failure_message "$response")")"
        return 1
    fi
    return 0
}

# Stops the recording started by start_event_recording. The stop action requires
# the recording_id the start returned.
stop_event_recording() {
    local recording_id="${1:-$LAST_RECORDING_ID}"
    local response
    response="$(call_tool "configure" '{"what":"event_recording_stop","recording_id":'"$(json_string "$recording_id")"'}')"
    if ! check_not_error "$response"; then
        fail "event_recording_stop failed for '$recording_id': $(truncate "$(command_failure_message "$response")")"
        return 1
    fi
    LAST_RECORDING_RESPONSE="$response"
    return 0
}

# ── Extension-Facing HTTP Helpers ──────────────────────────
# Every helper records LAST_HTTP_STATUS and LAST_HTTP_BODY so callers assert on
# the status code. Parsing the reply is not an assertion: the daemon answers
# 400 (invalid JSON), 403 (bad client header) and 409 (stale generation) with
# well-formed JSON bodies, so `jq .` succeeds on all of them.

# POST to an extension-protected endpoint under a chosen client identity.
# Client classes are defined in internal/extclient:
#   kaboom-extension — owns the extension session (adopts settings, bumps generation)
#   kaboom-probe     — contract prober; the daemon answers with an empty envelope
#                      and adopts nothing, so it cannot disturb a live extension.
post_as_client() {
    local client="$1"
    local endpoint="$2"
    local payload="$3"
    local response_file="$TEMP_DIR/http_post_${MCP_ID:-0}.txt"
    LAST_HTTP_STATUS=$(curl -s -o "$response_file" -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: ${client}" \
        "http://localhost:${PORT}${endpoint}" \
        -d "$payload" 2>/dev/null)
    LAST_HTTP_BODY=$(cat "$response_file" 2>/dev/null)
}

# POST to an extension-protected endpoint as the extension itself.
post_extension() {
    post_as_client "kaboom-extension/${VERSION}" "$1" "$2"
}

# POST to /logs as the extension.
post_logs() {
    post_extension "/logs" "$1"
}

# POST to an absolute URL, optionally with a single extra header. Used for
# negative tests that omit or corrupt the client identity header.
post_raw() {
    local url="$1"
    local payload="$2"
    local extra_headers="${3:-}"
    local response_file="$TEMP_DIR/http_post_raw_${MCP_ID:-0}.txt"
    if [ -n "$extra_headers" ]; then
        LAST_HTTP_STATUS=$(curl -s -o "$response_file" -w "%{http_code}" \
            -X POST -H "Content-Type: application/json" \
            -H "$extra_headers" \
            "$url" -d "$payload" 2>/dev/null)
    else
        LAST_HTTP_STATUS=$(curl -s -o "$response_file" -w "%{http_code}" \
            -X POST -H "Content-Type: application/json" \
            "$url" -d "$payload" 2>/dev/null)
    fi
    LAST_HTTP_BODY=$(cat "$response_file" 2>/dev/null)
}

# Asserts the last POST returned an expected status; reports the body on mismatch.
# Returns 0 on match so callers can early-return.
expect_http_status() {
    local expected="$1"
    local context="$2"
    if [ "$LAST_HTTP_STATUS" = "$expected" ]; then
        return 0
    fi
    fail "$context: expected HTTP $expected, got $LAST_HTTP_STATUS. Body: $(truncate "$LAST_HTTP_BODY")"
    return 1
}

# Validates the SyncResponse envelope the extension depends on. A reply missing
# any of these fields breaks the extension's poll loop even with a 200 status.
check_sync_envelope() {
    local body="$1"
    echo "$body" | jq -e '
        (.ack | type == "boolean")
        and (.commands | type == "array")
        and (.next_poll_ms | type == "number" and . > 0)
        and (.server_time | type == "string" and length > 0)
        and (.server_version | type == "string" and length > 0)
    ' >/dev/null 2>&1
}

# Reads a capture-state field the extension reported, so tests can prove the
# daemon applied a payload instead of merely returning 200 for it.
capture_state_field() {
    curl -s --max-time 5 "http://127.0.0.1:${PORT}/health" 2>/dev/null |
        jq -r --arg field "$1" '.capture[$field] // empty' 2>/dev/null
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
    if uat_wait_for_connected_browser "$PORT" "$WRAPPER"; then
        echo "  Connected browser ready: extension heartbeat and tracked tab available"
        return 0
    fi
    # Every category restarts the daemon and the suite toggles pilot state, so the
    # preflight's tracked tab cannot be assumed to survive to the last category.
    # A live extension with nothing tracked is recoverable: open a fresh disposable
    # fixture rather than failing every remaining test that needs a target.
    if ! uat_check_extension_readiness "$PORT"; then
        echo "WARNING: connected browser readiness failed: $UAT_CONNECTED_READINESS_REASON" >&2
        return 1
    fi
    echo "  Tracked tab lost; re-establishing the disposable connected fixture"
    if ! uat_create_disposable_tab "$PORT" "$WRAPPER" ||
        ! uat_wait_for_connected_browser "$PORT" "$WRAPPER"; then
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
