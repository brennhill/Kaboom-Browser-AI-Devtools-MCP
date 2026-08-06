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
