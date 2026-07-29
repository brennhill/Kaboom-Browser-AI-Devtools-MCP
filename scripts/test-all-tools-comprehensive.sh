#!/bin/bash
# test-all-tools-comprehensive.sh — Deterministic offline and connected-browser UAT runner.
# Usage: test-all-tools-comprehensive.sh [--suite offline|connected|all]
# Compatible with bash 3.2+ (macOS default).
# NO set -e: we need to collect all results even if some groups fail.
#
# NOTE: cat-27-interactive-overlays.sh is NOT included here — it requires
# a human operator to visually verify browser overlays (subtitle, draw mode,
# recording watermark, action toast, tracked hover launcher island).
# Run it standalone: bash scripts/tests/capture/cat-27-interactive-overlays.sh <port>

# ── Suite Selection ───────────────────────────────────────
SUITE="all"
if [ "$#" -gt 0 ]; then
    if [ "$#" -ne 2 ] || [ "$1" != "--suite" ]; then
        echo "Usage: $0 [--suite offline|connected|all]" >&2
        exit 2
    fi
    SUITE="$2"
fi

case "$SUITE" in
    offline|connected|all) ;;
    *)
        echo "Usage: $0 [--suite offline|connected|all]" >&2
        exit 2
        ;;
esac

# ── Dependency Checks ─────────────────────────────────────
check_deps() {
    local missing=""

    for cmd in jq curl lsof python3; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing="$missing $cmd"
        fi
    done

    # timeout may be gtimeout on macOS
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
        missing="$missing timeout(brew install coreutils)"
    fi

    if [ -n "$missing" ]; then
        echo "FATAL: Missing dependencies:$missing" >&2
        exit 1
    fi
}

check_deps

# Categories share the extension's configured port and clean up their own daemon.
export KABOOM_TEST_DISABLE_GLOBAL_CLEANER=1
# Comprehensive run should collect all test outcomes, not abort category scripts
# on first non-zero helper command.
export KABOOM_TEST_FAIL_FAST=0

# ── Timeout Compatibility ─────────────────────────────────
if command -v timeout >/dev/null 2>&1; then
    TIMEOUT_CMD="timeout"
elif command -v gtimeout >/dev/null 2>&1; then
    TIMEOUT_CMD="gtimeout"
else
    echo "FATAL: 'timeout' not found. Install with: brew install coreutils" >&2
    exit 1
fi

# ── Resolve Paths ─────────────────────────────────────────
SCRIPT_PATH="${BASH_SOURCE[0]:-$0}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd -P)"

resolve_project_root() {
    if [ -n "${KABOOM_PROJECT_ROOT:-}" ] && [ -d "${KABOOM_PROJECT_ROOT:-}" ]; then
        (cd "$KABOOM_PROJECT_ROOT" && pwd -P)
        return 0
    fi

    # Standard in-repo layout: scripts/test-all-tools-comprehensive.sh
    if [ -d "$SCRIPT_DIR/tests" ] && [ -f "$SCRIPT_DIR/../go.mod" ]; then
        (cd "$SCRIPT_DIR/.." && pwd -P)
        return 0
    fi

    # Fallback for copied runners: infer from current git worktree.
    if command -v git >/dev/null 2>&1; then
        local repo_root=""
        repo_root="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || true)"
        if [ -z "$repo_root" ]; then
            repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
        fi
        if [ -n "$repo_root" ] && [ -d "$repo_root/scripts/tests" ]; then
            (cd "$repo_root" && pwd -P)
            return 0
        fi
    fi

    return 1
}

PROJECT_ROOT="$(resolve_project_root)" || {
    echo "FATAL: Could not resolve project root. Set KABOOM_PROJECT_ROOT=/path/to/repo" >&2
    exit 1
}
TESTS_DIR="$PROJECT_ROOT/scripts/tests"

if [ -n "${KABOOM_UAT_WRAPPER:-}" ] && [ -x "${KABOOM_UAT_WRAPPER:-}" ]; then
    WRAPPER="$KABOOM_UAT_WRAPPER"
elif command -v kaboom-agentic-browser >/dev/null 2>&1; then
    WRAPPER="$(command -v kaboom-agentic-browser)"
elif [ -x "$PROJECT_ROOT/dist/kaboom-agentic-browser" ]; then
    WRAPPER="$PROJECT_ROOT/dist/kaboom-agentic-browser"
elif [ -x "$PROJECT_ROOT/npm/kaboom-agentic-browser/bin/kaboom-agentic-browser" ]; then
    WRAPPER="$PROJECT_ROOT/npm/kaboom-agentic-browser/bin/kaboom-agentic-browser"
else
    echo "FATAL: kaboom-agentic-browser not found in $PROJECT_ROOT or PATH" >&2
    exit 1
fi

# ── Temp Dir for Results ──────────────────────────────────
RESULTS_DIR="$(mktemp -d)"
OVERALL_START="$(date +%s)"

echo ""
echo "############################################################"
echo "# Kaboom MCP — COMPREHENSIVE UAT"
echo "############################################################"
echo ""
echo "Binary:     $WRAPPER"
echo "Tests dir:  $TESTS_DIR"
echo "Results:    $RESULTS_DIR"
echo "Suite:      $SUITE"
echo ""

# Offline contracts run away from the extension's configured endpoint so their
# Pilot-unavailable assertions cannot be invalidated by a live browser.
OFFLINE_UAT_PORT="${KABOOM_UAT_OFFLINE_PORT:-17890}"
CONNECTED_UAT_PORT="${KABOOM_UAT_CONNECTED_PORT:-7890}"
OFFLINE_CAT_IDS="01 02 03 04 05 06 07 08 09 10 11 12 13 20 25 26 28"
CONNECTED_CAT_IDS="14 15 16 18 19 23 24"

# shellcheck source=tests/framework/uat-user-state.sh
source "$TESTS_DIR/framework/uat-user-state.sh"
# shellcheck source=uat-result-lib.sh
source "$SCRIPT_DIR/uat-result-lib.sh"
if [ "$SUITE" = "connected" ] || [ "$SUITE" = "all" ]; then
    uat_snapshot_user_state "$CONNECTED_UAT_PORT" "$WRAPPER"
fi

case "$SUITE" in
    offline) CAT_IDS="$OFFLINE_CAT_IDS" ;;
    connected) CAT_IDS="$CONNECTED_CAT_IDS" ;;
    all) CAT_IDS="$OFFLINE_CAT_IDS $CONNECTED_CAT_IDS" ;;
esac

stop_preflight_daemon() {
    kill "$1" 2>/dev/null || true
    lsof -tiTCP:"$CONNECTED_UAT_PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    wait "$1" 2>/dev/null || true
}

preflight_connected_extension() {
    local preflight_pid=""

    lsof -tiTCP:"$CONNECTED_UAT_PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.3
    echo "Pre-flight: waiting for daemon, extension, and tracked tab..."
    (cd "$PROJECT_ROOT" && "$WRAPPER" --daemon --port "$CONNECTED_UAT_PORT" >/dev/null 2>&1) &
    preflight_pid="$!"

    uat_wait_for_connected_browser "$CONNECTED_UAT_PORT" "$WRAPPER"
    local readiness_status="$?"

    if [ "$readiness_status" -ne 0 ]; then
        echo "FATAL: connected suite prerequisite failed: $UAT_CONNECTED_READINESS_REASON" >&2
        stop_preflight_daemon "$preflight_pid"
        return 1
    fi
    if ! uat_create_disposable_tab "$CONNECTED_UAT_PORT" "$WRAPPER"; then
        echo "FATAL: connected suite could not create its disposable browser tab." >&2
        stop_preflight_daemon "$preflight_pid"
        return 1
    fi
    stop_preflight_daemon "$preflight_pid"
    echo "Pre-flight: connected browser ready on disposable tab $UAT_DISPOSABLE_TAB_ID."
}

# Safety-net trap: clean only suite-owned ports, then restore user state.
_uat_cleanup() {
    local _cleanup_ports=""
    case "$SUITE" in
        offline) _cleanup_ports="$OFFLINE_UAT_PORT" ;;
        connected) _cleanup_ports="$CONNECTED_UAT_PORT" ;;
        all) _cleanup_ports="$OFFLINE_UAT_PORT $CONNECTED_UAT_PORT" ;;
    esac
    for _base_port in $_cleanup_ports; do
        lsof -tiTCP:"$_base_port" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
        for _p in $((_base_port + 100)) $((_base_port + 101)) $((_base_port + 102)); do
            lsof -tiTCP:"$_p" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
        done
    done
    if [ "${KABOOM_TEST_DISABLE_GLOBAL_CLEANER:-0}" != "1" ] &&
        [ -f "$SCRIPT_DIR/cleanup-test-daemons.sh" ]; then
        bash "$SCRIPT_DIR/cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
    fi
    uat_restore_user_state
}
trap _uat_cleanup EXIT
trap 'uat_exit_for_signal INT' INT
trap 'uat_exit_for_signal TERM' TERM
trap 'uat_exit_for_signal HUP' HUP

# ── Run Categories ────────────────────────────────────────
category_timeout() {
    case "$1" in
        19) echo 600 ;;
        26) echo 180 ;;
        *) echo 120 ;;
    esac
}

run_category() {
    local cat_id="$1"
    local uat_port="$2"
    local category_script
    local timeout_seconds
    category_script="$(find "$TESTS_DIR" -type f -name "cat-${cat_id}-*.sh" -print | head -n 1)"
    if [ -z "$category_script" ]; then
        echo "Missing category script: cat-${cat_id}-*.sh" > "$RESULTS_DIR/output-${cat_id}.txt"
        return
    fi
    timeout_seconds="$(category_timeout "$cat_id")"
    (
        cd "$PROJECT_ROOT" || exit
        "$TIMEOUT_CMD" "$timeout_seconds" bash "$category_script" \
            "$uat_port" "$RESULTS_DIR/results-${cat_id}.txt" \
            > "$RESULTS_DIR/output-${cat_id}.txt" 2>&1
    ) || true
}

run_suite() {
    local suite_name="$1"
    local uat_port="$2"
    local suite_cat_ids="$3"
    local category_count
    category_count="$(echo "$suite_cat_ids" | wc -w | tr -d ' ')"

    lsof -tiTCP:"$uat_port" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.5
    echo "Running $category_count $suite_name categories sequentially on port $uat_port..."
    echo ""
    for cat_id in $suite_cat_ids; do
        run_category "$cat_id" "$uat_port"
    done
}

if [ "$SUITE" = "offline" ] || [ "$SUITE" = "all" ]; then
    run_suite "offline-contract" "$OFFLINE_UAT_PORT" "$OFFLINE_CAT_IDS"
fi
if [ "$SUITE" = "connected" ] || [ "$SUITE" = "all" ]; then
    export KABOOM_UAT_REQUIRE_CONNECTED=1
    preflight_connected_extension || exit 1
    run_suite "connected-browser" "$CONNECTED_UAT_PORT" "$CONNECTED_CAT_IDS"
fi

# ── Collect and Display Results ───────────────────────────

# Category display order and default names
get_default_name() {
    case "$1" in
        01) echo "Protocol Compliance" ;;
        02) echo "Observe Tool" ;;
        03) echo "Generate Tool" ;;
        04) echo "Configure Tool" ;;
        05) echo "Interact Tool" ;;
        06) echo "Server Lifecycle" ;;
        07) echo "Concurrency" ;;
        08) echo "Security" ;;
        09) echo "HTTP Endpoints" ;;
        10) echo "Regression Guards" ;;
        11) echo "Data Pipeline" ;;
        12) echo "Rich Action Results" ;;
        13) echo "Pilot State Contract" ;;
        14) echo "Extension Startup" ;;
        15) echo "Pilot Success Path" ;;
        16) echo "API Contract" ;;
        18) echo "Recording & Audio" ;;
        19) echo "Link Health Analyzer" ;;
        20) echo "Noise Persistence" ;;
        23) echo "Draw Mode" ;;
        24) echo "File Upload" ;;
        25) echo "Annotation Integration" ;;
        26) echo "Dynamic Binary Upgrade" ;;
        28) echo "Terminal HTTP Endpoints" ;;
        *)  echo "Unknown" ;;
    esac
}

TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0
AGGREGATION_ERRORS=0

# Print category outputs in order
for cat_id in $CAT_IDS; do
    output_file="$RESULTS_DIR/output-${cat_id}.txt"
    if [ -f "$output_file" ]; then
        cat "$output_file"
    fi
done

echo ""
echo ""

# ── Summary Table ─────────────────────────────────────────
echo "############################################################"
echo "# COMPREHENSIVE UAT RESULTS"
echo "############################################################"
echo ""
printf "%-28s | %4s | %4s | %4s | %5s | %5s\n" \
    "Category" "Pass" "Fail" "Skip" "Total" "Time"
echo "-------------------------------------------------------------------"

for cat_id in $CAT_IDS; do
    results_file="$RESULTS_DIR/results-${cat_id}.txt"
    cat_pass=0
    cat_fail=0
    cat_skip=0
    cat_elapsed="?"
    cat_name="$(get_default_name "$cat_id")"

    if parse_uat_category_result "$results_file"; then
        if [ "$UAT_RESULT_CATEGORY_ID" != "$cat_id" ]; then
            echo "AGGREGATION ERROR: malformed result file for category $cat_id (CATEGORY_ID=$UAT_RESULT_CATEGORY_ID)" >&2
            cat_fail=1
            AGGREGATION_ERRORS="$((AGGREGATION_ERRORS + 1))"
        else
            cat_pass="$UAT_RESULT_PASS"
            cat_fail="$UAT_RESULT_FAIL"
            cat_skip="$UAT_RESULT_SKIP"
            cat_elapsed="$UAT_RESULT_ELAPSED"
            if [ -n "$UAT_RESULT_CATEGORY_NAME" ]; then
                cat_name="$UAT_RESULT_CATEGORY_NAME"
            fi
        fi
    else
        result_status="$?"
        if [ "$result_status" -eq 1 ]; then
            echo "AGGREGATION ERROR: missing result file for category $cat_id: $results_file" >&2
        else
            echo "AGGREGATION ERROR: malformed result file for category $cat_id: $results_file" >&2
        fi
        cat_fail=1
        AGGREGATION_ERRORS="$((AGGREGATION_ERRORS + 1))"
    fi

    cat_total="$((cat_pass + cat_fail + cat_skip))"
    TOTAL_PASS="$((TOTAL_PASS + cat_pass))"
    TOTAL_FAIL="$((TOTAL_FAIL + cat_fail))"
    TOTAL_SKIP="$((TOTAL_SKIP + cat_skip))"

    printf "%2s. %-24s | %4d | %4d | %4d | %5d | %3ss\n" \
        "$cat_id" "$cat_name" "$cat_pass" "$cat_fail" "$cat_skip" "$cat_total" "$cat_elapsed"
done

TOTAL_ALL="$((TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP))"
OVERALL_ELAPSED="$(( $(date +%s) - OVERALL_START ))"

echo "-------------------------------------------------------------------"
printf "%-28s | %4d | %4d | %4d | %5d | %3ss\n" \
    "TOTAL" "$TOTAL_PASS" "$TOTAL_FAIL" "$TOTAL_SKIP" "$TOTAL_ALL" "$OVERALL_ELAPSED"

echo ""

# ── Final Verdict ─────────────────────────────────────────
if [ "$TOTAL_FAIL" -eq 0 ] && [ "$TOTAL_PASS" -gt 0 ] && [ "$AGGREGATION_ERRORS" -eq 0 ]; then
    echo "ALL $TOTAL_PASS TESTS PASSED ($TOTAL_SKIP skipped)"
else
    echo "FAILURES: $TOTAL_FAIL failed, $TOTAL_SKIP skipped of $TOTAL_ALL tests ($AGGREGATION_ERRORS aggregation errors)"
fi

echo ""

if [ "${KABOOM_KEEP_RESULTS:-0}" = "1" ]; then
    echo "Results kept at: $RESULTS_DIR"
else
    rm -rf "$RESULTS_DIR"
fi

# Exit code
if [ "$TOTAL_FAIL" -gt 0 ] || [ "$AGGREGATION_ERRORS" -gt 0 ]; then
    exit 1
fi
exit 0
