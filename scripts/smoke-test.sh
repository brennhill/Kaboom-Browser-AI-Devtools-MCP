#!/bin/bash
# smoke-test.sh — Human smoke test for Kaboom MCP.
# Sources modular test modules sequentially. Shared mutable state
# (EXTENSION_CONNECTED, PILOT_ENABLED, SMOKE_MARKER) flows across modules.
#
# Usage:
#   bash scripts/smoke-test.sh                    # default port 7890
#   bash scripts/smoke-test.sh 7890               # explicit port
#   bash scripts/smoke-test.sh --start-from 12    # resume from module 12
#   bash scripts/smoke-test.sh 7890 --start-from 07
set -euo pipefail

RUNNER_DIR="$(cd "$(dirname "$0")" && pwd)"
SMOKE_DIR="$RUNNER_DIR/smoke-tests"
PORT="7890"
START_FROM=""
ONLY_MODULE=""

normalize_semver() {
    local raw="${1:-}"
    raw="$(printf '%s' "$raw" | tr -d '[:space:]')"
    raw="${raw#v}"

    if [ -z "$raw" ] || [ "$raw" = "unknown" ] || [ "$raw" = "?" ]; then
        echo ""
        return 0
    fi

    local semver
    semver="$(printf '%s' "$raw" | sed -nE 's/.*([0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?).*/\1/p')"
    if [ -n "$semver" ]; then
        echo "$semver"
    else
        echo "$raw"
    fi
}

versions_match() {
    local left right
    left="$(normalize_semver "${1:-}")"
    right="$(normalize_semver "${2:-}")"
    [ -n "$left" ] && [ -n "$right" ] && [ "$left" = "$right" ]
}

# Parse args: positional port + optional --start-from
while [ $# -gt 0 ]; do
    case "$1" in
        --help|-h)
            echo "Usage: bash scripts/smoke-test.sh [PORT] [OPTIONS]"
            echo ""
            echo "  PORT           Daemon port (default: 7890)"
            echo "  Env overrides:"
            echo "    SMOKE_HARNESS_PORT   Local page harness port (default: 8787)"
            echo "    SMOKE_HARNESS_ROOT   Local page harness root directory"
            echo "    FRAMEWORK_RESILIENCE_FULL_REPEATS  Full run repeats for module 29 (default: 1)"
            echo "    FRAMEWORK_SELECTOR_REFRESH_CYCLES  Refresh cycles per run for module 29 (default: 3)"
            echo "  --start-from   Skip modules until MODULE matches (substring)"
            echo "  --only         Run only the matching module, then stop"
            echo "                 Examples: --only 15, --only upload"
            echo ""
            echo "Modules: 01-bootstrap, 02-core-telemetry, 03-observe-modes,"
            echo "  04-network-websocket, 05-interact-dom, 06-interact-state,"
            echo "  07-generate-formats, 08-configure-features, 09-perf-analysis,"
            echo "  10-recording, 11-subtitle-screenshot, 12-cross-cutting,"
            echo "  13-draw-mode, 14-browser-push, 15-file-upload, 20-inspect-visual,"
            echo "  21-macro-recording, 22-log-aggregation, 23-doctor-preflight,"
            echo "  24-retryable-errors, 25-action-enrichment, 26-default-upload-dir,"
            echo "  27-extension-refactor, 28-proof-first, 29-framework-selector-resilience,"
            echo "  31-annotation-parity, 30-stability-shutdown"
            exit 0
            ;;
        --start-from)
            if [ -z "${2:-}" ]; then
                echo "Error: --start-from requires a module name or number." >&2
                echo "Example: --start-from 05" >&2
                exit 1
            fi
            START_FROM="$2"
            shift 2
            ;;
        --only)
            if [ -z "${2:-}" ]; then
                echo "Error: --only requires a module name or number." >&2
                echo "Example: --only 15" >&2
                exit 1
            fi
            # --only is implemented as --start-from + stop after match
            START_FROM="$2"
            ONLY_MODULE="$2"
            shift 2
            ;;
        *)
            PORT="$1"
            shift
            ;;
    esac
done

# ── Dependency checks ─────────────────────────────────────
_smoke_check_deps() {
    local missing=""
    for cmd in jq curl python3 lsof; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing="$missing $cmd"
        fi
    done
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
        missing="$missing timeout(brew install coreutils)"
    fi
    if [ -n "$missing" ]; then
        echo "FATAL: Missing dependencies:$missing" >&2
        exit 1
    fi
}
_smoke_check_deps

# ── Source framework (initializes globals) ────────────────
# shellcheck source=/dev/null
export SMOKE_KEEP_DAEMON_ON_EXIT="${SMOKE_KEEP_DAEMON_ON_EXIT:-1}"
source "$SMOKE_DIR/framework-smoke.sh"
init_smoke "$PORT"
# Note: init_smoke sets the EXIT trap (_smoke_master_cleanup).
# Do NOT set another EXIT trap here — use register_cleanup instead.

EXPECTED_VERSION="${VERSION:-unknown}"
EXPECTED_VERSION_NORM="$(normalize_semver "$EXPECTED_VERSION")"

# ── Module list ──────────────────────────────────────────
MODULES=(
    "01-bootstrap.sh"
    "02-core-telemetry.sh"
    "03-observe-modes.sh"
    "04-network-websocket.sh"
    "05-interact-dom.sh"
    "06-interact-state.sh"
    "07-generate-formats.sh"
    "08-configure-features.sh"
    "09-perf-analysis.sh"
    "10-recording.sh"
    "11-subtitle-screenshot.sh"
    "12-cross-cutting.sh"
    "13-draw-mode.sh"
    "14-browser-push.sh"
    "15-file-upload.sh"       # upload requires a live daemon before shutdown/stability modules
    "20-inspect-visual.sh"
    "21-macro-recording.sh"
    "22-log-aggregation.sh"
    "23-doctor-preflight.sh"
    "24-retryable-errors.sh"
    "25-action-enrichment.sh"
    "26-default-upload-dir.sh"
    "27-extension-refactor.sh"
    "28-proof-first.sh"
    "29-framework-selector-resilience.sh"
    "31-annotation-parity.sh"
    "32-metrics-collection.sh"
    "30-stability-shutdown.sh" # 30 must be last: it kills the daemon
)

# ── Port conflict check ───────────────────────────────────
# If a healthy daemon is already on the port, reuse it (preserves extension connection).
if lsof -ti :"$PORT" >/dev/null 2>&1; then
    existing_ver=$(curl -s --connect-timeout 2 --max-time 3 "http://localhost:${PORT}/health" 2>/dev/null | jq -r '.version // empty' 2>/dev/null)
    if [ -n "$existing_ver" ]; then
        if [ -n "$EXPECTED_VERSION_NORM" ] && ! versions_match "$existing_ver" "$EXPECTED_VERSION"; then
            echo "  Existing daemon version mismatch on port $PORT (have v${existing_ver}, expected v${EXPECTED_VERSION}). Restarting..."
            kill_server 2>/dev/null || true
        else
            echo "  Reusing existing daemon on port $PORT (v${existing_ver})"
            DAEMON_PID=$(lsof -ti :"$PORT" 2>/dev/null | head -1)
        fi
    else
        echo "  Port $PORT occupied by non-Kaboom process. Killing..." >&2
        lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
        sleep 0.5
    fi
fi

BINARY_VERSION=$("$WRAPPER" --version 2>/dev/null || echo "unknown")
BINARY_VERSION_NORM="$(normalize_semver "$BINARY_VERSION")"
echo ""
echo "============================================================"
echo "  Kaboom SMOKE TEST SUITE"
echo "  Port: $PORT | $(date)"
echo "  Binary: $BINARY_VERSION"
echo "  Expected: $EXPECTED_VERSION"
if [ -n "$BINARY_VERSION_NORM" ] && [ -n "$EXPECTED_VERSION_NORM" ] && ! versions_match "$BINARY_VERSION" "$EXPECTED_VERSION"; then
    echo "  WARNING: Wrapper version (v${BINARY_VERSION_NORM}) differs from VERSION (v${EXPECTED_VERSION_NORM})."
fi
echo "  ${#MODULES[@]} modules"
echo "============================================================"
echo ""

# ── Resume support ───────────────────────────────────────
# --start-from skips all modules before the target.
# Instead of running full bootstrap, does a quick health probe
# to set EXTENSION_CONNECTED and PILOT_ENABLED.
SKIP_UNTIL_FOUND="${START_FROM:+true}"

if [ -n "$START_FROM" ]; then
    echo "  Resuming from module matching: $START_FROM"
    echo ""

    # Quick state init — replaces full bootstrap when resuming.
    # Assumes daemon is already running from the previous run.
    if ! wait_for_health 30; then
        echo "  Daemon not healthy. Starting fresh with --enable-os-upload-automation..."
        start_daemon_with_flags --enable-os-upload-automation || true
        # Give daemon extra time to fully initialize (MCP readiness lags HTTP readiness)
        sleep 2
    fi

    # Verify daemon is actually responding to MCP calls
    probe_resp=$(call_tool "observe" '{"what":"page"}' 2>/dev/null)
    if echo "$probe_resp" | grep -q "starting up" 2>/dev/null; then
        echo "  Daemon still starting, waiting 3s..."
        sleep 3
    fi

    health_body=$(get_http_body "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
    daemon_ver=$(echo "$health_body" | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")
    if [ -n "$EXPECTED_VERSION_NORM" ] && ! versions_match "$daemon_ver" "$EXPECTED_VERSION"; then
        echo "  Resume daemon version mismatch (have v${daemon_ver}, expected v${EXPECTED_VERSION}). Restarting..."
        start_daemon_with_flags --enable-os-upload-automation || true
        sleep 2
        health_body=$(get_http_body "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
        daemon_ver=$(echo "$health_body" | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")
    fi
    echo "  Daemon version: v${daemon_ver}"
    if echo "$health_body" | jq -e '.capture.extension_connected == true' >/dev/null 2>&1; then
        EXTENSION_CONNECTED=true
        echo "  Extension: connected"
    else
        echo "  Extension: NOT connected (some tests will skip)"
    fi

    # Probe pilot explicitly (instead of assuming enabled from prior run).
    PILOT_ENABLED=false
    if [ "$EXTENSION_CONNECTED" = "true" ]; then
        pilot_probe=$(call_tool "interact" '{"what":"execute_js","script":"1","reason":"resume-pilot-probe"}' 2>/dev/null || true)
        pilot_text=$(extract_content_text "$pilot_probe")
        if [ -z "$pilot_text" ] && [ -n "$pilot_probe" ]; then
            pilot_text="$pilot_probe"
        fi
        if echo "$pilot_text" | grep -qi "pilot_disabled\|ai_web_pilot_disabled"; then
            PILOT_ENABLED=false
            echo "  Pilot: disabled"
        elif echo "$pilot_text" | grep -qi "error\|failed\|no_data\|starting up"; then
            PILOT_ENABLED=false
            echo "  Pilot: unavailable (probe error)"
        else
            PILOT_ENABLED=true
            echo "  Pilot: enabled"
        fi
    else
        echo "  Pilot: unavailable (extension not connected)"
    fi
    echo ""
fi

# ── Run modules in order ─────────────────────────────────
MODULE_NUM=0
for module in "${MODULES[@]}"; do
    # Skip modules until START_FROM match (substring: "05" matches "05-interact-dom.sh")
    if [ "$SKIP_UNTIL_FOUND" = "true" ]; then
        if [[ "$module" == *"$START_FROM"* ]]; then
            SKIP_UNTIL_FOUND=""
        else
            continue
        fi
    fi

    MODULE_NUM=$((MODULE_NUM + 1))

    module_path="$SMOKE_DIR/$module"
    if [ ! -f "$module_path" ]; then
        echo "WARNING: Module $module not found, skipping."
        continue
    fi
    echo "── Module $MODULE_NUM/${#MODULES[@]}: $module ──"
    # Run the module in a subshell so its `set -eo pipefail` can't kill the runner.
    # Shared state (EXTENSION_CONNECTED, PILOT_ENABLED, etc.) is exported via a
    # temp file since subshell variables don't propagate to the parent.
    _state_file="$TEMP_DIR/smoke_state_$$"
    (
        # shellcheck source=/dev/null
        source "$module_path"
        # Export mutable state back to parent
        cat > "$_state_file" <<STATE_EOF
EXTENSION_CONNECTED=$EXTENSION_CONNECTED
PILOT_ENABLED=$PILOT_ENABLED
PASS_COUNT=$PASS_COUNT
FAIL_COUNT=$FAIL_COUNT
SKIP_COUNT=$SKIP_COUNT
DAEMON_PID=$DAEMON_PID
STATE_EOF
    ) || true
    # Import state from subshell
    if [ -f "$_state_file" ]; then
        # shellcheck source=/dev/null
        source "$_state_file"
        rm -f "$_state_file"
    fi

    # --only: stop after running the matched module
    if [ -n "$ONLY_MODULE" ] && [[ "$module" == *"$ONLY_MODULE"* ]]; then
        echo ""
        echo "  (--only $ONLY_MODULE: stopping after this module)"
        break
    fi
done

# ── Detect --start-from / --only with no match ────────────
if [ "$SKIP_UNTIL_FOUND" = "true" ]; then
    echo "ERROR: No module matched '$START_FROM'." >&2
    echo "  Available modules: ${MODULES[*]}" >&2
    exit 1
fi

if [ "$MODULE_NUM" -eq 0 ] && [ -z "$START_FROM" ]; then
    echo "ERROR: No modules were run." >&2
    exit 1
fi

# ── Cleanup: kill test helpers, then ensure daemon remains available ──
pkill -f "upload-server.py" 2>/dev/null || true

if [ "${SMOKE_KEEP_DAEMON_ON_EXIT:-1}" = "1" ]; then
    if ! wait_for_health 10; then
        echo "  Daemon is down after smoke run. Restarting on port $PORT..."
        start_daemon_with_flags --enable-os-upload-automation >/dev/null 2>&1 || true
        sleep 1
    fi

    if wait_for_health 10; then
        post_body=$(get_http_body "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
        post_ver=$(echo "$post_body" | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")
        echo "  Post-run daemon status: up on port $PORT (v${post_ver})."
    else
        echo "  WARNING: post-run daemon is still unavailable on port $PORT."
    fi
else
    echo "  Post-run daemon cleanup mode enabled (SMOKE_KEEP_DAEMON_ON_EXIT=0)."
fi

# ── Summary ──────────────────────────────────────────────
ELAPSED=$(( $(date +%s) - START_TIME ))
{
    echo ""
    echo "============================================================"
    echo "SMOKE TEST SUMMARY"
    echo "============================================================"
    echo "  Passed:  $PASS_COUNT"
    echo "  Failed:  $FAIL_COUNT"
    echo "  Skipped: $SKIP_COUNT"
    echo "  Time:    ${ELAPSED}s"
    echo ""
    if [ "$FAIL_COUNT" -eq 0 ]; then
        if [ "$SKIP_COUNT" -gt 0 ]; then
            echo "  Result: PASSED (with $SKIP_COUNT skipped)"
        else
            echo "  Result: ALL PASSED"
        fi
    else
        echo "  Result: FAILED"
    fi
    echo ""
    echo "  Output:      $OUTPUT_FILE"
    echo "  Diagnostics: $DIAGNOSTICS_FILE"
    echo ""
} | tee -a "$OUTPUT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
