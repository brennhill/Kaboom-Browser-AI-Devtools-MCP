#!/bin/bash
# framework-smoke.sh — Smoke test overrides for the base framework.
# Sources framework.sh and adds human-interactive helpers:
#   pause_for_human()    — wait for Enter between tests
#   interact_and_wait()  — fire interact + poll for completion
#   log_diagnostic()     — append raw responses to diagnostics file
#   Overridden pass()/fail() that call pause_for_human
set -eo pipefail

# ── Source base framework ─────────────────────────────────
SMOKE_FRAMEWORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_FRAMEWORK_DIR/../tests/framework.sh"

# ── Shared mutable state (set by modules, read by later modules) ──
EXTENSION_CONNECTED=false
PILOT_ENABLED=false
SMOKE_MARKER="KABOOM_SMOKE_$(date +%s)_$$"
SKIP_COUNT=0
CURRENT_TEST_ID=""

# ── Color support ─────────────────────────────────────────
# Use ANSI colors only when stdout is a TTY
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    _CLR_PASS='\033[0;32m'   # green
    _CLR_FAIL='\033[0;31m'   # red
    _CLR_SKIP='\033[0;33m'   # yellow
    _CLR_RESET='\033[0m'
    _INTERACTIVE=true
else
    _CLR_PASS=''
    _CLR_FAIL=''
    _CLR_SKIP=''
    _CLR_RESET=''
    _INTERACTIVE=false
fi

# ── Composable cleanup system ─────────────────────────────
# Modules register cleanup functions instead of overwriting the EXIT trap.
# _smoke_cleanup_handlers is a space-separated list of function names.
_smoke_cleanup_handlers=""

# Register a cleanup function. Called by modules instead of `trap ... EXIT`.
register_cleanup() {
    _smoke_cleanup_handlers="$_smoke_cleanup_handlers $1"
}

# Master cleanup: runs all registered handlers, then base cleanup.
_smoke_master_cleanup() {
    _smoke_kill_harness 2>/dev/null || true
    for handler in $_smoke_cleanup_handlers; do
        "$handler" 2>/dev/null || true
    done
    # Keep daemon alive by default so developers can continue working after smoke.
    # Set SMOKE_KEEP_DAEMON_ON_EXIT=0 for strict cleanup mode in automation.
    if [ "${SMOKE_KEEP_DAEMON_ON_EXIT:-1}" != "1" ]; then
        kill_server 2>/dev/null || true
        if [ -f "$SMOKE_FRAMEWORK_DIR/../cleanup-test-daemons.sh" ]; then
            bash "$SMOKE_FRAMEWORK_DIR/../cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
        fi
    fi
    pkill -f "upload-server.py" 2>/dev/null || true
    [ -n "$TEMP_DIR" ] && rm -rf "$TEMP_DIR" 2>/dev/null || true
}

# ── Persistent output files ──────────────────────────────
# Output and diagnostics go to well-known persistent locations so they
# survive crashes and are easy to find. Printed at start of every run.
SMOKE_OUTPUT_DIR="${HOME}/.kaboom/smoke-results"
mkdir -p "$SMOKE_OUTPUT_DIR"
DIAGNOSTICS_FILE="$SMOKE_OUTPUT_DIR/diagnostics.log"
SMOKE_OUTPUT_FILE="$SMOKE_OUTPUT_DIR/output.log"

_smoke_abs_dir() {
    local dir="$1"
    (cd "$dir" >/dev/null 2>&1 && pwd -P)
}

_resolve_harness_root() {
    if [ -n "${SMOKE_HARNESS_ROOT:-}" ]; then
        if [ -d "$SMOKE_HARNESS_ROOT" ]; then
            _smoke_abs_dir "$SMOKE_HARNESS_ROOT"
            return 0
        fi
        echo "FATAL: SMOKE_HARNESS_ROOT is set but invalid: $SMOKE_HARNESS_ROOT" >&2
        return 1
    fi

    # The harness pages moved under internal/testpages/pages when the fixture
    # server was extracted into its own package — go:embed requires the assets
    # to live inside the package directory. The pre-move path is kept so this
    # script still works when checked out against an older tree.
    local candidates=(
        "$SMOKE_FRAMEWORK_DIR/../../tests/pages"
        "$SMOKE_FRAMEWORK_DIR/../../cmd/browser-agent/internal/testpages/pages"
        "$SMOKE_FRAMEWORK_DIR/../../cmd/browser-agent/testpages"
    )
    local candidate
    for candidate in "${candidates[@]}"; do
        if [ -d "$candidate" ]; then
            _smoke_abs_dir "$candidate"
            return 0
        fi
    done

    echo "FATAL: unable to resolve harness root directory." >&2
    echo "  Tried:" >&2
    for candidate in "${candidates[@]}"; do
        echo "    - $candidate" >&2
    done
    echo "  Override with: SMOKE_HARNESS_ROOT=/absolute/path/to/testpages" >&2
    return 1
}

SMOKE_HARNESS_PORT="${SMOKE_HARNESS_PORT:-8787}"
SMOKE_HARNESS_ROOT="$(_resolve_harness_root)"
SMOKE_HARNESS_PID=""
SMOKE_BASE_URL="http://127.0.0.1:${SMOKE_HARNESS_PORT}"
SMOKE_EXAMPLE_URL="${SMOKE_BASE_URL}/example.com"
SMOKE_INTERACT_URL="${SMOKE_BASE_URL}/interact.html"
SMOKE_PERFORMANCE_URL="${SMOKE_BASE_URL}/performance.html"
SMOKE_TELEMETRY_URL="${SMOKE_BASE_URL}/telemetry.html"
SMOKE_A11Y_URL="${SMOKE_BASE_URL}/a11y.html"

_smoke_kill_harness() {
    if [ -n "${SMOKE_HARNESS_PID:-}" ]; then
        kill "$SMOKE_HARNESS_PID" 2>/dev/null || true
        sleep 0.1
        kill -0 "$SMOKE_HARNESS_PID" 2>/dev/null && kill -9 "$SMOKE_HARNESS_PID" 2>/dev/null || true
        SMOKE_HARNESS_PID=""
    fi
}

start_local_harness() {
    _smoke_kill_harness
    local harness_log="$SMOKE_OUTPUT_DIR/harness.log"
    python3 "$SMOKE_FRAMEWORK_DIR/harness-server.py" \
        --root "$SMOKE_HARNESS_ROOT" \
        --port "$SMOKE_HARNESS_PORT" >"$harness_log" 2>&1 &
    SMOKE_HARNESS_PID=$!

    local ok=false
    for _i in $(seq 1 30); do
        if curl -s --connect-timeout 1 --max-time 2 "${SMOKE_BASE_URL}/healthz" >/dev/null 2>&1; then
            ok=true
            break
        fi
        sleep 0.2
    done

    if [ "$ok" != "true" ]; then
        echo "FATAL: local harness failed to start on ${SMOKE_BASE_URL}" >&2
        echo "  Harness root: $SMOKE_HARNESS_ROOT" >&2
        echo "  Harness log: $harness_log" >&2
        tail -n 20 "$harness_log" >&2 || true
        return 1
    fi
    return 0
}

wait_for_extension() {
    local timeout_s="${1:-10}"
    local attempts=$((timeout_s * 2))
    for _wfe_i in $(seq 1 "$attempts"); do
        local body
        body=$(curl -s --connect-timeout 1 --max-time 2 "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
        if echo "$body" | jq -e '.capture.extension_connected == true' >/dev/null 2>&1; then
            return 0
        fi
        sleep 0.5
    done
    return 1
}

init_smoke() {
    local port="${1:-7890}"
    init_framework "$port" "/dev/null"

    # Override OUTPUT_FILE to use persistent location instead of TEMP_DIR
    OUTPUT_FILE="$SMOKE_OUTPUT_FILE"

    # Initialize output files
    echo "Smoke Test Output — $(date)" > "$OUTPUT_FILE"
    echo "Smoke Test Diagnostics — $(date)" > "$DIAGNOSTICS_FILE"
    echo "Port: $port" >> "$DIAGNOSTICS_FILE"
    echo "Harness: $SMOKE_BASE_URL" >> "$DIAGNOSTICS_FILE"
    echo "======================================" >> "$DIAGNOSTICS_FILE"

    # Print file locations and mode upfront so they're always visible
    echo ""
    echo "  Output:      $OUTPUT_FILE"
    echo "  Diagnostics: $DIAGNOSTICS_FILE"
    echo "  Harness:     $SMOKE_BASE_URL"
    if [ "$_INTERACTIVE" = "true" ]; then
        echo "  Mode:        interactive (pauses between tests)"
    else
        echo "  Mode:        non-interactive (CI/piped — no pauses)"
    fi
    echo ""

    # Override ALL cleanup traps (init_framework sets EXIT+INT+TERM to framework_cleanup
    # which deletes TEMP_DIR — we need INT/TERM to use our handler too)
    trap _smoke_master_cleanup EXIT INT TERM

    # Trap ERR so crashes under set -e are immediately visible.
    # Logs the failing command, line, and function to both stderr and diagnostics.
    trap '_smoke_on_error $LINENO "${FUNCNAME[0]:-main}" "${BASH_COMMAND}"' ERR

    start_local_harness
}

rewrite_smoke_urls() {
    local payload="$1"
    local rewritten
    rewritten=$(
        printf '%s' "$payload" | \
            SMOKE_EXAMPLE_URL="$SMOKE_EXAMPLE_URL" SMOKE_BASE_URL="$SMOKE_BASE_URL" \
            jq -c '
                def rewrite_example_domain_url:
                    if type != "string" then .
                    elif test("^https?://(www\\.)?example\\.com(/|$)") then
                        sub("^https?://(www\\.)?example\\.com"; env.SMOKE_EXAMPLE_URL)
                    elif test("^https?://example\\.org(/|$)") then
                        sub("^https?://example\\.org"; (env.SMOKE_BASE_URL + "/example.org"))
                    else .
                    end;

                def rewrite_url_fields:
                    if type == "object" then
                        with_entries(
                            if (.key == "url" or .key == "base_url" or .key == "from_url" or .key == "to_url")
                            then .value |= rewrite_example_domain_url
                            else .value |= rewrite_url_fields
                            end
                        )
                    elif type == "array" then map(rewrite_url_fields)
                    else .
                    end;

                rewrite_url_fields
            ' 2>/dev/null
    )

    if [ -n "$rewritten" ]; then
        echo "$rewritten"
    else
        # Non-JSON payloads should pass through unchanged.
        echo "$payload"
    fi
}

# Override base framework call_tool so smoke runs stay on local harness pages.
call_tool() {
    local tool_name="$1"
    local arguments="${2:-\{\}}"
    local rewritten
    rewritten="$(rewrite_smoke_urls "$arguments")"
    local request="{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/call\",\"params\":{\"name\":\"${tool_name}\",\"arguments\":${rewritten}}}"
    send_mcp "$request" "call_${tool_name}"
}

_smoke_on_error() {
    local line="$1" func="$2" cmd="$3"
    local test_ctx="${CURRENT_TEST_ID:-unknown}"
    printf "\n  ${_CLR_FAIL}!!! CRASH${_CLR_RESET} [${test_ctx}] at line $line in $func(): $cmd\n" >&2
    echo "  !!! Diagnostics: $DIAGNOSTICS_FILE" >&2
    {
        echo ""
        echo "!!! CRASH [${test_ctx}] at $(date +%H:%M:%S)"
        echo "    Test:     $test_ctx"
        echo "    Line:     $line"
        echo "    Function: $func"
        echo "    Command:  $cmd"
        echo "    Pass=$PASS_COUNT Fail=$FAIL_COUNT Skip=$SKIP_COUNT"
    } >> "$DIAGNOSTICS_FILE" 2>/dev/null
    # Count crash as a failure but don't exit — let the runner continue
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

# ── Override begin_test to track current test ID ──────────
begin_test() {
    CURRENT_TEST_ID="$1"
    local name="$2"
    local purpose="${3:-}"
    local trust="${4:-}"
    {
        echo "============================================================"
        echo "TEST ${CURRENT_TEST_ID}: ${name}"
        echo "============================================================"
        echo "Purpose: ${purpose}"
        echo "Trust:   ${trust}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    # Log test start to diagnostics immediately
    echo "--- $(date +%H:%M:%S) START ${CURRENT_TEST_ID}: ${name}" >> "$DIAGNOSTICS_FILE"
}

# ── Override pass/fail/skip to pause for human + color ────
pass() {
    local description="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    # Colorized to terminal, plain to output file
    printf "  ${_CLR_PASS}PASS${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  PASS [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
    echo "  $(date +%H:%M:%S) PASS [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    pause_for_human
}

fail() {
    local description="$1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    # Log to diagnostics FIRST (before display) so it survives crashes
    echo "  $(date +%H:%M:%S) FAIL [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    printf "  ${_CLR_FAIL}FAIL${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  FAIL [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
    pause_for_human
}

skip() {
    local description="$1"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    echo "  $(date +%H:%M:%S) SKIP [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    printf "  ${_CLR_SKIP}SKIP${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  SKIP [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
}

pause_for_human() {
    # Skip interactive prompt in CI or non-TTY environments
    if [ -n "${CI:-}" ] || [ ! -t 0 ]; then
        return
    fi
    echo "  -- Press Enter to continue, Ctrl-C to abort --"
    read -r
    echo ""
}

# ── Log diagnostic data ──────────────────────────────────
log_diagnostic() {
    local test_name="$1"
    local action="$2"
    local response="$3"
    local result="${4:-}"
    {
        echo ""
        echo "======= $test_name — $action ======="
        echo "Response:"
        echo "$response" | head -100
        if [ -n "$result" ]; then
            echo ""
            echo "Result:"
            echo "$result" | head -100
        fi
    } >> "$DIAGNOSTICS_FILE"
}

extract_embedded_json() {
    local text="$1"
    local suffix="${text#*\{}"
    if [ "$suffix" = "$text" ]; then
        return 1
    fi
    printf '{%s' "$suffix"
}

content_json_query() {
    local text="$1"
    local expr="$2"
    local fallback="${3:-}"
    local payload
    if ! payload=$(extract_embedded_json "$text"); then
        printf '%s' "$fallback"
        return 0
    fi
    local out
    out="$(printf '%s' "$payload" | jq -r "$expr" 2>/dev/null || true)"
    if [ -z "$out" ]; then
        printf '%s' "$fallback"
    else
        printf '%s' "$out"
    fi
}

content_json_cursor() {
    content_json_query "$1" '.metadata.cursor // .metadata.after_cursor // .metadata.next_cursor // empty' ""
}

content_json_entry_count() {
    content_json_query "$1" '(.entries // .logs // []) | if type=="array" then length else 0 end' "0"
}

content_json_count() {
    content_json_query "$1" '.count // ((.entries // .logs // []) | if type=="array" then length else 0 end)' "0"
}

pending_status_for_correlation() {
    local text="$1"
    local corr="$2"
    local payload
    if ! payload=$(extract_embedded_json "$text"); then
        return 0
    fi
    printf '%s' "$payload" | jq -r --arg corr "$corr" '
        def normalize_status($bucket; $raw):
            ($raw // "" | tostring | ascii_downcase | gsub("^\\s+|\\s+$"; "")) as $s |
            if ($s == "queued" or $s == "running" or $s == "still_processing") then "pending"
            elif $s != "" then $s
            elif $bucket == "completed" then "complete"
            elif $bucket == "failed" then "error"
            else "pending"
            end;

        [
            "pending", "completed", "failed"
        ] as $buckets |
        [
            $buckets[] as $bucket |
            (.[$bucket] // []) |
            if type == "array" then .[] else empty end |
            select((.correlation_id // "" | tostring) == $corr) |
            (normalize_status($bucket; .status)) as $status |
            (
                $status + "|" +
                ("fallback pending_commands bucket=" + $bucket + " status=" + $status) +
                (if .error then " error=" + (.error | tostring) else "" end)
            )
        ][0] // empty
    ' 2>/dev/null || true
}

# ── Interact helper ──────────────────────────────────────
# Fires an interact command and waits for completion via polling.
# Sets INTERACT_RESULT to the command result text (or empty on timeout).
# Always returns 0 — callers inspect $INTERACT_RESULT for pass/fail.
# (Returning non-zero under set -eo pipefail kills the entire script.)
interact_and_wait() {
    local action="$1"
    local args="$2"
    local max_polls="${3:-}"
    if [ -z "$max_polls" ]; then
        case "$action" in
            navigate|refresh|back|forward|new_tab|upload|record_start|record_stop|screen_recording_start|screen_recording_stop)
                max_polls=120 ;; # up to 60s at 0.5s interval for slower async browser actions
            *)
                max_polls=20 ;;
        esac
    fi

    local response
    response=$(call_tool "interact" "$args")
    local content_text
    content_text=$(extract_content_text "$response")
    if [ -z "$content_text" ] && [ -n "$response" ]; then
        # Fall back to raw JSON-RPC payload if content extraction failed.
        content_text="$response"
    fi

    {
        echo ""
        echo "- $(date +%H:%M:%S) interact($action) initial response:"
        if [ -n "$content_text" ]; then
            echo "$content_text" | head -50
        else
            echo "(empty response payload)"
        fi
    } >> "$DIAGNOSTICS_FILE"

    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

    if [ -z "$corr_id" ]; then
        if [ -n "$content_text" ]; then
            INTERACT_RESULT="$content_text"
        else
            INTERACT_RESULT="empty_response_payload"
        fi
        return 0
    fi

    local last_poll_text=""
    for i in $(seq 1 "$max_polls"); do
        sleep 0.5
        local poll_response
        poll_response=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_response")
        if [ -z "$poll_text" ] && [ -n "$poll_response" ]; then
            poll_text="$poll_response"
        fi
        if [ -n "$poll_text" ]; then
            last_poll_text="$poll_text"
        fi

        if echo "$poll_text" | grep -q '"status":"complete"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "  Complete after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 0
        fi
        if echo "$poll_text" | grep -q '"status":"failed"\|"status":"error"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "  Failed after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 0
        fi

        # Fallback: command_result can be temporarily missing; inspect pending_commands.
        if [ -z "$poll_text" ] || echo "$poll_text" | grep -qi "no_data\|not found"; then
            local pending_response
            pending_response=$(call_tool "observe" '{"what":"pending_commands"}')
            local pending_text
            pending_text=$(extract_content_text "$pending_response")
            if [ -z "$pending_text" ] && [ -n "$pending_response" ]; then
                pending_text="$pending_response"
            fi
            if [ -n "$pending_text" ]; then
                local pending_hit
                pending_hit=$(pending_status_for_correlation "$pending_text" "$corr_id")
                if [ -n "$pending_hit" ]; then
                    local pending_status pending_detail
                    pending_status="${pending_hit%%|*}"
                    pending_detail="${pending_hit#*|}"
                    if [ -n "$pending_detail" ]; then
                        last_poll_text="$pending_detail"
                    fi
                    if [ "$pending_status" = "complete" ]; then
                        INTERACT_RESULT="$pending_text"
                        {
                            echo "  Complete via pending_commands fallback after poll $i"
                            echo "$pending_text" | head -30
                        } >> "$DIAGNOSTICS_FILE"
                        return 0
                    fi
                    if echo "$pending_status" | grep -qiE "error|failed|expired|timeout|cancelled"; then
                        INTERACT_RESULT="$pending_text"
                        {
                            echo "  Failed via pending_commands fallback after poll $i"
                            echo "$pending_text" | head -30
                        } >> "$DIAGNOSTICS_FILE"
                        return 0
                    fi
                fi
            fi
        fi
    done

    if [ -z "$last_poll_text" ]; then
        last_poll_text="no poll payload received"
    fi
    INTERACT_RESULT="timeout waiting for $action (corr_id=$corr_id). $last_poll_text"
    {
        echo "  Timeout after $max_polls polls"
        echo "  Last poll: $(echo "$last_poll_text" | head -1)"
    } >> "$DIAGNOSTICS_FILE"
    return 0
}
