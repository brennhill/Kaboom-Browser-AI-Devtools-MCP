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
# UAT launches many isolated daemon states. Never let those synthetic sessions
# create production analytics rows or inflate distinct active-install counts.
export KABOOM_TELEMETRY=off
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

    # Standard in-repo layout: scripts/uat/runners/test-all-tools-comprehensive.sh
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
ARTIFACT_DIR="${KABOOM_UAT_ARTIFACT_DIR:-$PROJECT_ROOT/artifacts/uat}"
JSON_ARTIFACT="$ARTIFACT_DIR/uat-results.json"
JUNIT_ARTIFACT="$ARTIFACT_DIR/uat-results.xml"
CATEGORY_RECORDS="$RESULTS_DIR/categories.ndjson"
: > "$CATEGORY_RECORDS"

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
# 14 and 16 verify the server's half of the /sync contract, so they speak as the
# extension to prove settings were applied. That identity is only safe where no
# real extension is attached, which is why they run offline: on the connected
# port they would have to pose as a probe, and the daemon answers probes with a
# canned envelope that adopts nothing — leaving them unable to assert anything.
#
# Category 27 is deliberately absent: it pauses on `read -r` for human visual
# verification of browser overlays, so scheduling it here would hang the suite.
# Every other category on disk is scheduled — an unscheduled script looks like
# coverage while never running, which is how category 32 sat at 8/8 green with
# every one of its calls failing to parse.
OFFLINE_CAT_IDS="01 02 03 04 05 06 07 08 09 10 11 12 13 14 16 17 20 21 25 26 28 29 34"
CONNECTED_CAT_IDS="15 33 35 36 18 19 22 23 24 30 31"

# shellcheck source=tests/framework/uat-user-state.sh
source "$TESTS_DIR/framework/uat-user-state.sh"
# shellcheck source=tests/framework/uat-artifacts.sh
source "$TESTS_DIR/framework/uat-artifacts.sh"
# shellcheck source=tests/framework/uat-replay.sh
source "$TESTS_DIR/framework/uat-replay.sh"
# shellcheck source=tests/framework/process-census.sh
source "$TESTS_DIR/framework/process-census.sh"
# shellcheck source=uat-result-lib.sh
source "$SCRIPT_DIR/../orchestration/uat-result-lib.sh"
# shellcheck source=scripts/uat/orchestration/uat-category-script.sh
source "$SCRIPT_DIR/../orchestration/uat-category-script.sh"

# Categories that leak processes must cost a red run. Each category registers
# framework_cleanup on EXIT, so the census returns to baseline after every one;
# anything still standing outlived the work that started it.
PROCESS_LEAK_CATEGORIES=""
# Categories killed at their deadline. Their result files under-report, so they
# are tracked separately rather than trusted.
TIMED_OUT_CATEGORIES=""
if [ "$SUITE" = "connected" ] || [ "$SUITE" = "all" ]; then
    uat_snapshot_user_state "$CONNECTED_UAT_PORT" "$WRAPPER"
fi

record_census_baseline

case "$SUITE" in
    offline) CAT_IDS="$OFFLINE_CAT_IDS" ;;
    connected) CAT_IDS="$CONNECTED_CAT_IDS" ;;
    all) CAT_IDS="$OFFLINE_CAT_IDS $CONNECTED_CAT_IDS" ;;
esac

if [ -n "${KABOOM_UAT_CATEGORY:-}" ]; then
    case " $CAT_IDS " in
        *" $KABOOM_UAT_CATEGORY "*) CAT_IDS="$KABOOM_UAT_CATEGORY" ;;
        *)
            echo "FATAL: category $KABOOM_UAT_CATEGORY is not part of the selected $SUITE suite" >&2
            exit 2
            ;;
    esac
fi

stop_preflight_daemon() {
    kill "$1" 2>/dev/null || true
    lsof -tiTCP:"$CONNECTED_UAT_PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    wait "$1" 2>/dev/null || true
}

preflight_connected_extension() {
    local preflight_pid=""

    # The preflight exists to fail fast when no browser is attached. In replay
    # mode there is no browser to check, and each category attaches its own fake
    # extension in start_daemon, so a preflight here would only start a daemon
    # nothing is polling and time out.
    if uat_replay_enabled; then
        echo "Pre-flight: replay mode — answering from ${KABOOM_UAT_REPLAY}, no browser required."
        return 0
    fi

    lsof -tiTCP:"$CONNECTED_UAT_PORT" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.3
    echo "Pre-flight: waiting for daemon, extension, and tracked tab..."
    (cd "$PROJECT_ROOT" && "$WRAPPER" --daemon --parallel --port "$CONNECTED_UAT_PORT" >/dev/null 2>&1) &
    preflight_pid="$!"

    uat_wait_for_extension "$CONNECTED_UAT_PORT"
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
    # Normal completion restores the user's daemon before emitting artifacts.
    # The EXIT trap runs afterward; killing the shared connected port again
    # would terminate the daemon we just restored.
    [ "$UAT_USER_STATE_RESTORED" = "1" ] && return 0

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
        [ -f "$SCRIPT_DIR/../../maintenance/cleanup-test-daemons.sh" ]; then
        bash "$SCRIPT_DIR/../../maintenance/cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
    fi
    uat_restore_user_state
}
trap _uat_cleanup EXIT
trap 'uat_exit_for_signal INT' INT
trap 'uat_exit_for_signal TERM' TERM
trap 'uat_exit_for_signal HUP' HUP

# ── Run Categories ────────────────────────────────────────
# These are hang budgets, not performance assertions. Exceeding one now fails the
# suite, so a budget set near a category's measured time turns ordinary variance
# into a red build — and a gate that cries wolf is the one people switch off.
#
# Three offline runs on an M-series Mac, 2026-09-05: category 5 at 120/120/116s
# against the old 120s budget (it was killed during cleanup on the first),
# category 26 at 116s, category 21 at 76-84s, and category 12 at 7s, 12s and then
# 72s — a sixfold swing on an idle machine. A shared CI runner is slower and
# noisier than that, so the default is 300s: still short enough that a genuine
# hang is caught rather than run to the job's own limit.
#
# Category 5 is the slow one by construction: each pilot-gated call waits out the
# no-extension path at roughly 5s, and test 5.9 alone spends 69s of that.
category_timeout() {
    case "$1" in
        19) echo 600 ;;
        33) echo 900 ;;
        *) echo 300 ;;
    esac
}

run_category() {
    local cat_id="$1"
    local uat_port="$2"
    local category_script
    local timeout_seconds
    # Resolution failures land in the category's own output file so the summary
    # reports them against the category rather than losing them to stderr, and
    # the missing result file makes the aggregator count the category as failed.
    if ! category_script="$(uat_resolve_category_script "$TESTS_DIR" "$cat_id" 2>"$RESULTS_DIR/output-${cat_id}.txt")"; then
        return
    fi
    timeout_seconds="$(category_timeout "$cat_id")"
    local category_status=0
    (
        cd "$PROJECT_ROOT" || exit
        "$TIMEOUT_CMD" "$timeout_seconds" bash "$category_script" \
            "$uat_port" "$RESULTS_DIR/results-${cat_id}.txt" \
            > "$RESULTS_DIR/output-${cat_id}.txt" 2>&1
    ) || category_status="$?"

    # timeout(1) reports 124 when it kills the command at the deadline, 137 when
    # the kernel had to SIGKILL it. Either way the category's EXIT trap still
    # writes a result file, so it reports the assertions it happened to reach and
    # FAIL_COUNT=0 — a truncated run that the aggregator reads as green. Category
    # 5 was one slow machine away from that: it finished its 19th assertion at
    # the 120s mark and was terminated during cleanup. Record the kill instead.
    if [ "$category_status" -eq 124 ] || [ "$category_status" -eq 137 ]; then
        TIMED_OUT_CATEGORIES="$TIMED_OUT_CATEGORIES ${cat_id}(${timeout_seconds}s)"
        printf '\n  FAIL: category %s exceeded its %ss budget and was killed. Every assertion after that point never ran, and the result file it left behind reports no failures.\n' \
            "$cat_id" "$timeout_seconds" >> "$RESULTS_DIR/output-${cat_id}.txt"
    fi

    # Counted before any sweep, so the census reports what the category actually
    # left behind rather than what cleanup managed to hide.
    if ! assert_no_process_growth "category ${cat_id}"; then
        PROCESS_LEAK_CATEGORIES="$PROCESS_LEAK_CATEGORIES $cat_id"
        # Re-baseline so one leak reports once instead of failing every category
        # after it, then clear it so the suite does not compound.
        KABOOM_CENSUS_BASELINE="$(kaboom_census_count)"
        bash "$SCRIPT_DIR/../../maintenance/cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
        KABOOM_CENSUS_BASELINE="$(kaboom_census_count)"
    fi
    if ! assert_no_duplicate_daemons "category ${cat_id}"; then
        PROCESS_LEAK_CATEGORIES="$PROCESS_LEAK_CATEGORIES ${cat_id}(duplicate)"
    fi
    if ! assert_no_launcher_processes "category ${cat_id}"; then
        PROCESS_LEAK_CATEGORIES="$PROCESS_LEAK_CATEGORIES ${cat_id}(launcher)"
    fi
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
    offline_run_ids="$OFFLINE_CAT_IDS"
    if [ -n "${KABOOM_UAT_CATEGORY:-}" ]; then
        case " $OFFLINE_CAT_IDS " in
            *" $KABOOM_UAT_CATEGORY "*) offline_run_ids="$KABOOM_UAT_CATEGORY" ;;
            *) offline_run_ids="" ;;
        esac
    fi
    [ -z "$offline_run_ids" ] || run_suite "offline-contract" "$OFFLINE_UAT_PORT" "$offline_run_ids"
fi
if [ "$SUITE" = "connected" ] || [ "$SUITE" = "all" ]; then
    export KABOOM_UAT_REQUIRE_CONNECTED=1
    preflight_connected_extension || exit 1
    connected_run_ids="$CONNECTED_CAT_IDS"
    if [ -n "${KABOOM_UAT_CATEGORY:-}" ]; then
        case " $CONNECTED_CAT_IDS " in
            *" $KABOOM_UAT_CATEGORY "*) connected_run_ids="$KABOOM_UAT_CATEGORY" ;;
            *) connected_run_ids="" ;;
        esac
    fi
    [ -z "$connected_run_ids" ] || run_suite "connected-browser" "$CONNECTED_UAT_PORT" "$connected_run_ids"
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
        33) echo "Connected Action Coverage" ;;
        34) echo "Packaged Corruption Recovery" ;;
        35) echo "QA Fixture Transactions" ;;
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
    result_status="complete"

    if parse_uat_category_result "$results_file"; then
        if ! uat_category_ids_match "$cat_id" "$UAT_RESULT_CATEGORY_ID"; then
            echo "AGGREGATION ERROR: malformed result file for category $cat_id (CATEGORY_ID=$UAT_RESULT_CATEGORY_ID)" >&2
            cat_fail=1
            result_status="category_id_mismatch"
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
            result_status="missing_result"
        else
            echo "AGGREGATION ERROR: malformed result file for category $cat_id: $results_file" >&2
            result_status="malformed_result"
        fi
        cat_fail=1
        AGGREGATION_ERRORS="$((AGGREGATION_ERRORS + 1))"
    fi

    cat_total="$((cat_pass + cat_fail + cat_skip))"
    TOTAL_PASS="$((TOTAL_PASS + cat_pass))"
    TOTAL_FAIL="$((TOTAL_FAIL + cat_fail))"
    TOTAL_SKIP="$((TOTAL_SKIP + cat_skip))"

    skip_reasons='[]'
    if [ -f "$RESULTS_DIR/output-${cat_id}.txt" ]; then
        skip_reasons="$(sed -n 's/^[[:space:]]*SKIP: //p' "$RESULTS_DIR/output-${cat_id}.txt" |
            jq -Rsc 'split("\n") | map(select(length > 0))')"
    fi
    jq -cn \
        --arg id "$cat_id" \
        --arg name "$cat_name" \
        --arg result_status "$result_status" \
        --argjson pass "$cat_pass" \
        --argjson fail "$cat_fail" \
        --argjson skip "$cat_skip" \
        --argjson total "$cat_total" \
        --argjson elapsed_seconds "${cat_elapsed//[^0-9]/0}" \
        --argjson skip_reasons "$skip_reasons" \
        '{id:$id,name:$name,pass:$pass,fail:$fail,skip:$skip,total:$total,
          elapsed_seconds:$elapsed_seconds,result_status:$result_status,
          skip_reasons:$skip_reasons}' >> "$CATEGORY_RECORDS"

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
if [ -n "$PROCESS_LEAK_CATEGORIES" ]; then
    echo "PROCESS LEAK:$PROCESS_LEAK_CATEGORIES"
    echo "A category left kaboom processes running. Nothing would have reaped them."
fi

if [ -n "$TIMED_OUT_CATEGORIES" ]; then
    echo "TIMED OUT:$TIMED_OUT_CATEGORIES"
    echo "A killed category reports only the assertions it reached, with no failures. Its counters above under-report."
fi

# One rule, consulted once, for both the printed verdict and the exit status.
# They used to be written separately and drifted: the verdict demanded a passing
# assertion, the exit code did not.
if uat_suite_passed "$TOTAL_PASS" "$TOTAL_FAIL" "$AGGREGATION_ERRORS" \
    "$PROCESS_LEAK_CATEGORIES" "$TIMED_OUT_CATEGORIES"; then
    SUITE_EXIT=0
    echo "ALL $TOTAL_PASS TESTS PASSED ($TOTAL_SKIP skipped)"
else
    SUITE_EXIT=1
    if [ "$TOTAL_ALL" -eq 0 ]; then
        echo "NO TESTS RAN: the $SUITE suite scheduled $(echo "$CAT_IDS" | wc -w | tr -d ' ') categories and collected no assertions."
        echo "A suite that runs nothing cannot pass. Check the category id lists in $(basename "$SCRIPT_PATH")."
    fi
    echo "FAILURES: $TOTAL_FAIL failed, $TOTAL_SKIP skipped of $TOTAL_ALL tests ($AGGREGATION_ERRORS aggregation errors)"
fi

echo ""

uat_restore_user_state
uat_emit_artifacts \
    "$CATEGORY_RECORDS" "$JSON_ARTIFACT" "$JUNIT_ARTIFACT" "$SUITE" \
    "$OVERALL_ELAPSED" "$UAT_USER_STATE_RESTORE_STATUS" \
    "${UAT_CONNECTED_READINESS_REASON:-not_applicable}"
echo "JSON:  $JSON_ARTIFACT"
echo "JUnit: $JUNIT_ARTIFACT"
echo ""

if [ "${KABOOM_KEEP_RESULTS:-0}" = "1" ]; then
    echo "Results kept at: $RESULTS_DIR"
else
    rm -rf "$RESULTS_DIR"
fi

exit "$SUITE_EXIT"
