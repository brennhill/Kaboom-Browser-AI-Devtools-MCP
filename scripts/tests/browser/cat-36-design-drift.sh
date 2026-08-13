#!/bin/bash
# cat-36-design-drift.sh — Connected analyze/design_audit verdict verification (GitHub #693, #694, #695).
# Docs: docs/features/feature/design-drift-audit/index.md
#
# This category asserts VERDICTS, not reachability. cat-33 already invokes
# design_audit and reports a pass whenever the response is not an error, so a
# shallow category here would add nothing: the mode would look covered while
# proving only that it returns JSON.
#
# Every expectation comes from the shared table at
# cmd/browser-agent/internal/toolanalyze/designdrift/testdata/expected-findings.json,
# which the Go analyzer tests assert against too. One source, so the two cannot
# drift apart. Both directions are checked — each planted positive must be
# found, and each negative control must yield nothing.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-7890}"
OUTPUT_FILE="${2:-/dev/null}"
init_framework "$PORT" "$OUTPUT_FILE"

REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
EXPECTED_TABLE="$REPO_ROOT/cmd/browser-agent/internal/toolanalyze/designdrift/testdata/expected-findings.json"

if [ ! -f "$EXPECTED_TABLE" ]; then
    begin_category "36" "Design Drift Audit" "1"
    fail "Expected-findings table missing at $EXPECTED_TABLE"
    finish_category
fi

CASE_COUNT="$(jq '.cases | length' "$EXPECTED_TABLE")"
begin_category "36" "Design Drift Audit" "$((CASE_COUNT + 1))"

start_daemon || {
    fail "Design drift UAT could not start with an attached browser"
    finish_category
}

# ── Arm the fixture ────────────────────────────────────────
begin_test "36.1" "Design drift fixture loads" \
    "Navigate the tracked tab to the planted-drift fixture" \
    "Every later assertion is meaningless if the page did not render"
run_test_36_1() {
    if ! ensure_fixture "design-drift"; then
        fail "Could not navigate to the design-drift fixture"
        return
    fi
    local probe
    probe="$(call_tool "analyze" '{"what":"computed_styles","selector":".rhythm-card"}')"
    if ! check_valid_jsonrpc "$probe" || check_is_error "$probe"; then
        fail "Style probe failed against the fixture: $(truncate "$(extract_content_text "$probe")")"
        return
    fi
    # Five rhythm cards is the fixture's own signature; a different count means
    # the page changed and the expected-findings table is stale.
    #
    # tail BEFORE jq, not after: the response text is a prose summary line
    # followed by the JSON envelope, and jq given both aborts on the prose and
    # emits nothing at all, which reads as a count of zero rather than a parse
    # failure. match_count is hoisted onto the async envelope; the result object
    # carries count.
    local count
    count="$(extract_content_text "$probe" | tail -n 1 |
        jq -r '.match_count // .result.match_count // .result.count // .count // 0' 2>/dev/null)"
    if [ "$count" != "5" ]; then
        fail "Fixture rendered $count rhythm cards, expected 5 — the fixture and the expected-findings table disagree"
        return
    fi
    pass "Design drift fixture rendered with its planted geometry"
}
run_test_36_1

# audit_findings runs one case and echoes the findings array as compact JSON.
audit_findings() {
    local args="$1"
    local response
    response="$(call_tool "analyze" "$args")"
    if ! check_valid_jsonrpc "$response" || check_is_error "$response"; then
        printf 'ERROR:%s' "$(truncate "$(extract_content_text "$response")")"
        return
    fi
    extract_content_text "$response" | tail -n 1 | jq -c '
        (.result // .) as $r
        | {
            findings: [ ($r.sections // {}) | to_entries[] | .value.findings[]?
                        | {category, property, element_index, severity, expected_from} ],
            skipped: [ ($r.checks_skipped // [])[] | {category, reason} ]
          }' 2>/dev/null
}

# ── One test per case in the shared table ──────────────────
case_index=1
while [ "$case_index" -le "$CASE_COUNT" ]; do
    idx=$((case_index - 1))
    name="$(jq -r ".cases[$idx].name" "$EXPECTED_TABLE")"
    kind="$(jq -r ".cases[$idx].kind" "$EXPECTED_TABLE")"
    why="$(jq -r ".cases[$idx].why" "$EXPECTED_TABLE")"
    selector="$(jq -r ".cases[$idx].selector" "$EXPECTED_TABLE")"
    args="$(jq -c ".cases[$idx] | {what:\"design_audit\", selector} + (if .categories then {categories} else {} end) + (if .spec then {spec} else {} end)" "$EXPECTED_TABLE")"
    expected_findings="$(jq -c ".cases[$idx].expect_findings | sort_by(.category, .element_index, .property)" "$EXPECTED_TABLE")"
    expected_skipped="$(jq -c ".cases[$idx].expect_skipped | sort_by(.category, .reason)" "$EXPECTED_TABLE")"

    begin_test "36.$((case_index + 1))" "$name ($kind)" \
        "analyze design_audit on $selector" \
        "$why"

    actual="$(audit_findings "$args")"
    if [ -z "$actual" ]; then
        fail "$name: could not parse the design_audit response"
    elif [ "${actual:0:6}" = "ERROR:" ]; then
        fail "$name: design_audit returned an error — ${actual:6}"
    else
        actual_findings="$(printf '%s' "$actual" | jq -c '[.findings[] | {category, property, element_index, severity, expected_from}] | sort_by(.category, .element_index, .property)')"
        actual_skipped="$(printf '%s' "$actual" | jq -c '[.skipped[] | {category, reason}] | sort_by(.category, .reason)')"

        # Compare only the fields the table declares, so adding a field to the
        # finding contract does not break every UAT case.
        expected_trimmed="$(printf '%s' "$expected_findings" | jq -c '[.[] | {category, property, element_index, severity, expected_from}]')"

        if [ "$actual_findings" != "$expected_trimmed" ]; then
            fail "$name: findings did not match the expected table
  expected: $expected_trimmed
  actual:   $actual_findings"
        elif [ "$actual_skipped" != "$expected_skipped" ]; then
            fail "$name: skipped categories did not match
  expected: $expected_skipped
  actual:   $actual_skipped"
        elif [ "$kind" = "control" ]; then
            pass "$name: legitimate variation produced no findings, as required"
        else
            pass "$name: every planted finding was detected and nothing extra"
        fi
    fi

    case_index=$((case_index + 1))
done

finish_category
