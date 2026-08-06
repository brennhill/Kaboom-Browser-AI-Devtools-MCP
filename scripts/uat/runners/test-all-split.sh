#!/bin/bash
# test-all-split.sh — Run all tests in two phases: Original + New

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

ALLOW_NEW_FAILURES="${UAT_ALLOW_NEW_FAILURES:-1}"
ENABLE_GO_UAT_COVERAGE="${UAT_GO_COVERAGE:-0}"
GO_UAT_COVERAGE_MIN="${UAT_GO_COVERAGE_MIN:-}"
GO_UAT_COVERAGE_DIR="${UAT_GO_COVERAGE_DIR:-$PROJECT_ROOT/coverage/uat-go}"
GO_UAT_COVERAGE_RAW_DIR="$GO_UAT_COVERAGE_DIR/raw"
GO_UAT_BINARY="${UAT_GO_COVERAGE_BINARY:-$PROJECT_ROOT/kaboom-agentic-browser-uat-cover}"

PHASE1_SUMMARY_FILE="$(mktemp /tmp/kaboom-uat-phase1-summary-XXXXXX)"
PHASE2_SUMMARY_FILE="$(mktemp /tmp/kaboom-uat-phase2-summary-XXXXXX)"

# shellcheck disable=SC2329 # called via trap
cleanup() {
    rm -f "$PHASE1_SUMMARY_FILE" "$PHASE2_SUMMARY_FILE"
}
trap cleanup EXIT

is_non_negative_int() {
    case "${1:-}" in
        ''|*[!0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

load_phase_summary() {
    local summary_file="$1"
    local parsed

    if [ ! -f "$summary_file" ]; then
        return 1
    fi

    parsed="$({
        set -euo pipefail
        TOTAL_PASS=""
        TOTAL_FAIL=""
        TOTAL_SKIP=""
        TOTAL_ASSERTIONS=""
        CATEGORY_TOTAL=""
        CATEGORY_REPORTED=""
        CATEGORY_MISSING=""
        CATEGORY_CORRUPT=""
        CATEGORY_INVALID=""
        INTEGRITY_ERRORS=""
        RESULTS_DIR=""
        DURATION=""
        # shellcheck disable=SC1090
        source "$summary_file" 2>/dev/null
        printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
            "${TOTAL_PASS:-0}" "${TOTAL_FAIL:-0}" "${TOTAL_SKIP:-0}" "${TOTAL_ASSERTIONS:-0}" \
            "${CATEGORY_TOTAL:-0}" "${CATEGORY_REPORTED:-0}" "${CATEGORY_MISSING:-0}" \
            "${CATEGORY_CORRUPT:-0}" "${CATEGORY_INVALID:-0}" "${INTEGRITY_ERRORS:-0}" \
            "${DURATION:-0}" "${RESULTS_DIR:-}"
    } )" || return 2

    IFS=$'\t' read -r SUMMARY_TOTAL_PASS SUMMARY_TOTAL_FAIL SUMMARY_TOTAL_SKIP SUMMARY_TOTAL_ASSERTIONS \
        SUMMARY_CATEGORY_TOTAL SUMMARY_CATEGORY_REPORTED SUMMARY_CATEGORY_MISSING SUMMARY_CATEGORY_CORRUPT \
        SUMMARY_CATEGORY_INVALID SUMMARY_INTEGRITY_ERRORS SUMMARY_DURATION SUMMARY_RESULTS_DIR <<<"$parsed"

    for value in \
        "$SUMMARY_TOTAL_PASS" "$SUMMARY_TOTAL_FAIL" "$SUMMARY_TOTAL_SKIP" "$SUMMARY_TOTAL_ASSERTIONS" \
        "$SUMMARY_CATEGORY_TOTAL" "$SUMMARY_CATEGORY_REPORTED" "$SUMMARY_CATEGORY_MISSING" \
        "$SUMMARY_CATEGORY_CORRUPT" "$SUMMARY_CATEGORY_INVALID" "$SUMMARY_INTEGRITY_ERRORS" \
        "$SUMMARY_DURATION"; do
        if ! is_non_negative_int "$value"; then
            return 3
        fi
    done

    return 0
}

print_header() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════════════════╗"
    echo "║                       KABOOM UAT TEST SUITE (SPLIT)                            ║"
    echo "║                                                                                ║"
    echo "║ Phase 1: ORIGINAL TESTS (54 tests, 20 categories) — Known Stable              ║"
    echo "║ Phase 2: NEW TESTS (98 tests, 14 categories) — Newly Built                    ║"
    echo "╚════════════════════════════════════════════════════════════════════════════════╝"
    echo ""
}

setup_go_uat_coverage() {
    if [ "$ENABLE_GO_UAT_COVERAGE" != "1" ]; then
        return 0
    fi

    mkdir -p "$GO_UAT_COVERAGE_RAW_DIR"
    find "$GO_UAT_COVERAGE_RAW_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +

    echo "Preparing coverage-instrumented UAT daemon binary..."
    (
        cd "$PROJECT_ROOT"
        go build -cover -coverpkg=./... -o "$GO_UAT_BINARY" ./cmd/browser-agent
    )

    export KABOOM_UAT_WRAPPER="$GO_UAT_BINARY"
    export KABOOM_UAT_GOCOVERDIR="$GO_UAT_COVERAGE_RAW_DIR"

    echo "  Coverage binary: $GO_UAT_BINARY"
    echo "  Coverage output: $GO_UAT_COVERAGE_RAW_DIR"
}

report_go_uat_coverage() {
    GO_UAT_COVERAGE_OK=true
    GO_UAT_TOTAL_COVERAGE=""

    if [ "$ENABLE_GO_UAT_COVERAGE" != "1" ]; then
        return 0
    fi

    echo ""
    echo "────────────────────────────────────────────────────────────────────────────────"
    echo "GO UAT RUNTIME COVERAGE"
    echo "────────────────────────────────────────────────────────────────────────────────"

    if [ ! -d "$GO_UAT_COVERAGE_RAW_DIR" ] || [ -z "$(find "$GO_UAT_COVERAGE_RAW_DIR" -type f -print -quit)" ]; then
        echo "❌ No Go runtime coverage files were produced during UAT."
        GO_UAT_COVERAGE_OK=false
        return 0
    fi

    mkdir -p "$GO_UAT_COVERAGE_DIR"
    (
        cd "$PROJECT_ROOT"
        go tool covdata percent -i="$GO_UAT_COVERAGE_RAW_DIR" | tee "$GO_UAT_COVERAGE_DIR/percent.txt"
        go tool covdata textfmt -i="$GO_UAT_COVERAGE_RAW_DIR" -o "$GO_UAT_COVERAGE_DIR/coverage.out"
    )

    GO_UAT_TOTAL_COVERAGE="$(cd "$PROJECT_ROOT" && go tool cover -func="$GO_UAT_COVERAGE_DIR/coverage.out" | awk '/^total:/{print $3}')"
    if [ -z "$GO_UAT_TOTAL_COVERAGE" ]; then
        echo "❌ Unable to compute total Go runtime coverage from UAT artifacts."
        GO_UAT_COVERAGE_OK=false
        return 0
    fi

    echo "Total Go runtime coverage from UAT: $GO_UAT_TOTAL_COVERAGE"
    echo "Coverage artifacts: $GO_UAT_COVERAGE_DIR"

    if [ -n "$GO_UAT_COVERAGE_MIN" ]; then
        local go_uat_total_numeric
        go_uat_total_numeric="${GO_UAT_TOTAL_COVERAGE%\%}"
        if awk "BEGIN { exit !($go_uat_total_numeric >= $GO_UAT_COVERAGE_MIN) }"; then
            echo "✅ Coverage gate met (${go_uat_total_numeric}% >= ${GO_UAT_COVERAGE_MIN}%)"
        else
            echo "❌ Coverage gate failed (${go_uat_total_numeric}% < ${GO_UAT_COVERAGE_MIN}%)"
            GO_UAT_COVERAGE_OK=false
        fi
    fi
}

print_header
setup_go_uat_coverage

# Phase 1: Original tests
echo "PHASE 1: Running Original UAT Tests..."
echo "────────────────────────────────────────────────────────────────────────────────"

if KABOOM_UAT_SUMMARY_FILE="$PHASE1_SUMMARY_FILE" bash "$SCRIPT_DIR/test-original-uat.sh"; then
    echo ""
    echo "✅ PHASE 1 COMPLETE: Original tests passed"
    PHASE1_PASS=true
else
    echo ""
    echo "❌ PHASE 1 FAILED: Original tests have failures"
    PHASE1_PASS=false
fi

echo ""
echo "────────────────────────────────────────────────────────────────────────────────"
echo ""

# Phase 2: New tests
echo "PHASE 2: Running New UAT Tests..."
echo "────────────────────────────────────────────────────────────────────────────────"

PHASE2_HARD_FAILED=false
if KABOOM_UAT_SUMMARY_FILE="$PHASE2_SUMMARY_FILE" bash "$SCRIPT_DIR/test-new-uat.sh"; then
    echo ""
    echo "✅ PHASE 2 COMPLETE: New tests passed"
    PHASE2_PASS=true
else
    if [ "$ALLOW_NEW_FAILURES" = "1" ]; then
        echo ""
        echo "⚠️  PHASE 2 COMPLETE: New tests have failures (allowed by UAT_ALLOW_NEW_FAILURES=1)"
        PHASE2_PASS=true
        PHASE2_HARD_FAILED=true
    else
        echo ""
        echo "❌ PHASE 2 FAILED: New tests have failures"
        PHASE2_PASS=false
    fi
fi

PHASE1_TOTAL_PASS=0
PHASE1_TOTAL_FAIL=0
PHASE1_TOTAL_SKIP=0
PHASE1_TOTAL_ASSERTIONS=0
PHASE1_CATEGORY_TOTAL=0
PHASE1_CATEGORY_REPORTED=0
PHASE1_INTEGRITY_ERRORS=0
PHASE1_RESULTS_DIR=""

if load_phase_summary "$PHASE1_SUMMARY_FILE"; then
    PHASE1_TOTAL_PASS="$SUMMARY_TOTAL_PASS"
    PHASE1_TOTAL_FAIL="$SUMMARY_TOTAL_FAIL"
    PHASE1_TOTAL_SKIP="$SUMMARY_TOTAL_SKIP"
    PHASE1_TOTAL_ASSERTIONS="$SUMMARY_TOTAL_ASSERTIONS"
    PHASE1_CATEGORY_TOTAL="$SUMMARY_CATEGORY_TOTAL"
    PHASE1_CATEGORY_REPORTED="$SUMMARY_CATEGORY_REPORTED"
    PHASE1_INTEGRITY_ERRORS="$SUMMARY_INTEGRITY_ERRORS"
    PHASE1_RESULTS_DIR="$SUMMARY_RESULTS_DIR"
else
    PHASE1_INTEGRITY_ERRORS=1
fi

PHASE2_TOTAL_PASS=0
PHASE2_TOTAL_FAIL=0
PHASE2_TOTAL_SKIP=0
PHASE2_TOTAL_ASSERTIONS=0
PHASE2_CATEGORY_TOTAL=0
PHASE2_CATEGORY_REPORTED=0
PHASE2_INTEGRITY_ERRORS=0
PHASE2_RESULTS_DIR=""

if load_phase_summary "$PHASE2_SUMMARY_FILE"; then
    PHASE2_TOTAL_PASS="$SUMMARY_TOTAL_PASS"
    PHASE2_TOTAL_FAIL="$SUMMARY_TOTAL_FAIL"
    PHASE2_TOTAL_SKIP="$SUMMARY_TOTAL_SKIP"
    PHASE2_TOTAL_ASSERTIONS="$SUMMARY_TOTAL_ASSERTIONS"
    PHASE2_CATEGORY_TOTAL="$SUMMARY_CATEGORY_TOTAL"
    PHASE2_CATEGORY_REPORTED="$SUMMARY_CATEGORY_REPORTED"
    PHASE2_INTEGRITY_ERRORS="$SUMMARY_INTEGRITY_ERRORS"
    PHASE2_RESULTS_DIR="$SUMMARY_RESULTS_DIR"
else
    PHASE2_INTEGRITY_ERRORS=1
fi

TOTAL_PASS=$((PHASE1_TOTAL_PASS + PHASE2_TOTAL_PASS))
TOTAL_FAIL=$((PHASE1_TOTAL_FAIL + PHASE2_TOTAL_FAIL))
TOTAL_SKIP=$((PHASE1_TOTAL_SKIP + PHASE2_TOTAL_SKIP))
TOTAL_ASSERTIONS=$((PHASE1_TOTAL_ASSERTIONS + PHASE2_TOTAL_ASSERTIONS))
TOTAL_CATEGORY_REPORTED=$((PHASE1_CATEGORY_REPORTED + PHASE2_CATEGORY_REPORTED))
TOTAL_CATEGORY_EXPECTED=$((PHASE1_CATEGORY_TOTAL + PHASE2_CATEGORY_TOTAL))
TOTAL_INTEGRITY_ERRORS=$((PHASE1_INTEGRITY_ERRORS + PHASE2_INTEGRITY_ERRORS))

CATEGORY_COVERAGE_PCT="$(awk "BEGIN { if ($TOTAL_CATEGORY_EXPECTED == 0) { print \"0.0\" } else { printf \"%.1f\", ($TOTAL_CATEGORY_REPORTED*100)/$TOTAL_CATEGORY_EXPECTED } }")"

report_go_uat_coverage

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════════╗"
echo "║                            FINAL RESULTS                                       ║"
echo "╚════════════════════════════════════════════════════════════════════════════════╝"
echo ""

if [ "$PHASE1_PASS" = true ]; then
    echo "✅ Phase 1 (Original Tests):  PASSED"
else
    echo "❌ Phase 1 (Original Tests):  FAILED"
fi

if [ "$PHASE2_PASS" = true ]; then
    if [ "$PHASE2_HARD_FAILED" = true ]; then
        echo "⚠️  Phase 2 (New Tests):       SOFT-FAILED (allowed)"
    else
        echo "✅ Phase 2 (New Tests):       PASSED"
    fi
else
    echo "❌ Phase 2 (New Tests):       FAILED"
fi

echo ""
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Summary:"
echo "  Total Passed:        $TOTAL_PASS"
echo "  Total Failed:        $TOTAL_FAIL"
echo "  Total Skipped:       $TOTAL_SKIP"
echo "  Total Checks:        $TOTAL_ASSERTIONS"
echo "  Category Coverage:   $TOTAL_CATEGORY_REPORTED/$TOTAL_CATEGORY_EXPECTED (${CATEGORY_COVERAGE_PCT}%)"
echo "  Integrity Errors:    $TOTAL_INTEGRITY_ERRORS"
if [ -n "$PHASE1_RESULTS_DIR" ]; then
    echo "  Phase 1 results dir: $PHASE1_RESULTS_DIR"
fi
if [ -n "$PHASE2_RESULTS_DIR" ]; then
    echo "  Phase 2 results dir: $PHASE2_RESULTS_DIR"
fi
echo ""

OVERALL_OK=true
if [ "$PHASE1_PASS" != true ]; then
    OVERALL_OK=false
fi
if [ "$PHASE2_PASS" != true ]; then
    OVERALL_OK=false
fi
if [ "$ENABLE_GO_UAT_COVERAGE" = "1" ] && [ "$GO_UAT_COVERAGE_OK" != true ]; then
    OVERALL_OK=false
fi

if [ "$OVERALL_OK" = true ]; then
    echo "🎉 UAT RUN COMPLETE"
    exit 0
fi

echo "⚠️  TEST SUITE INCOMPLETE"
if [ "$PHASE1_PASS" = false ]; then
    echo "❌ Original UAT tests failed — daemon or core features broken"
fi
if [ "$PHASE2_PASS" = false ]; then
    echo "❌ New UAT tests failed and are required (UAT_ALLOW_NEW_FAILURES=0)"
fi
if [ "$ENABLE_GO_UAT_COVERAGE" = "1" ] && [ "$GO_UAT_COVERAGE_OK" != true ]; then
    echo "❌ Go runtime UAT coverage checks failed"
fi
exit 1
