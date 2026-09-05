#!/usr/bin/env bash
# Shared result parsing helpers for split UAT runners.

# parse_uat_category_result reads a category result file written by
# scripts/tests/framework/framework.sh and sets these globals on success:
#   UAT_RESULT_PASS, UAT_RESULT_FAIL, UAT_RESULT_SKIP,
#   UAT_RESULT_ELAPSED, UAT_RESULT_CATEGORY_ID, UAT_RESULT_CATEGORY_NAME
# Return codes:
#   0 = ok, 1 = missing file, 2 = unreadable/corrupt file, 3 = invalid counters

is_uat_non_negative_int() {
    case "${1:-}" in
        ''|*[!0-9]*)
            return 1
            ;;
        *)
            return 0
            ;;
    esac
}

# uat_category_ids_match compares the numeric identity of category IDs while
# accepting the zero-padded runner form (for example, 01) and the historical
# framework form (1). Validation happens before arithmetic so malformed values
# cannot be coerced into a valid category.
uat_category_ids_match() {
    local expected="${1:-}"
    local actual="${2:-}"

    is_uat_non_negative_int "$expected" || return 1
    is_uat_non_negative_int "$actual" || return 1
    [ "$((10#$expected))" -eq "$((10#$actual))" ]
}

parse_uat_category_result() {
    local result_file="$1"
    local parsed=""
    local pass=""
    local fail=""
    local skip=""
    local elapsed=""
    local category_id=""
    local category_name=""

    if [ ! -f "$result_file" ]; then
        return 1
    fi

    parsed="$({
        set -euo pipefail
        PASS_COUNT=""
        FAIL_COUNT=""
        SKIP_COUNT="0"
        ELAPSED=""
        CATEGORY_ID=""
        CATEGORY_NAME=""
        # shellcheck disable=SC1090
        source "$result_file" 2>/dev/null
        printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
            "${PASS_COUNT:-}" "${FAIL_COUNT:-}" "${SKIP_COUNT:-0}" \
            "${ELAPSED:-}" "${CATEGORY_ID:-}" "${CATEGORY_NAME:-}"
    } )" || return 2

    IFS=$'\t' read -r pass fail skip elapsed category_id category_name <<<"$parsed"

    if ! is_uat_non_negative_int "$pass" \
        || ! is_uat_non_negative_int "$fail" \
        || ! is_uat_non_negative_int "$skip" \
        || ! is_uat_non_negative_int "$elapsed"; then
        return 3
    fi

    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_PASS="$pass"
    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_FAIL="$fail"
    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_SKIP="$skip"
    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_ELAPSED="$elapsed"
    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_CATEGORY_ID="$category_id"
    # shellcheck disable=SC2034 # globals are consumed by calling scripts
    UAT_RESULT_CATEGORY_NAME="$category_name"
    return 0
}

# uat_suite_passed decides whether a completed suite exits green. The rule lives
# here, in one place, because the runner stated it twice — once in the verdict it
# printed and once in the exit code it returned — and the two disagreed. The
# printed verdict required at least one passing assertion; the exit code did not.
# A suite that ran no categories therefore printed
# "FAILURES: 0 failed, 0 skipped of 0 tests" and exited 0. That is exactly the
# shape a CI job takes on the day somebody trims a category id list: green, and
# verifying nothing.
#
# Arguments: pass fail aggregation_errors leaked_categories timed_out_categories
# Returns 0 when the suite passed, 1 otherwise.
uat_suite_passed() {
    local pass="${1:-}"
    local fail="${2:-}"
    local aggregation_errors="${3:-}"
    local leaked_categories="${4:-}"
    local timed_out_categories="${5:-}"

    # A counter that is not a number means the aggregation itself broke, and a
    # broken aggregation must never be readable as a pass.
    is_uat_non_negative_int "$pass" || return 1
    is_uat_non_negative_int "$fail" || return 1
    is_uat_non_negative_int "$aggregation_errors" || return 1

    [ "$fail" -eq 0 ] || return 1
    [ "$aggregation_errors" -eq 0 ] || return 1
    [ -z "$leaked_categories" ] || return 1
    # A category killed at its deadline leaves a result file recording only the
    # assertions it reached, with no failures. Its counters are not evidence.
    [ -z "$timed_out_categories" ] || return 1
    # A suite that asserted nothing did not pass; it did not run.
    [ "$pass" -gt 0 ] || return 1
    return 0
}
