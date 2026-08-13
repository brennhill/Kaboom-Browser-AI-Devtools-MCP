#!/bin/bash
# json.test.sh — Pins the three-way outcome that raw jq collapses into one.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/json.sh"

FAILURES=0

expect() {
    local what="$1" want="$2" got="$3"
    if [ "$want" != "$got" ]; then
        echo "FAIL: $what — want [$want], got [$got]"
        FAILURES=$((FAILURES + 1))
    fi
}

run_field() {
    local text="$1" path="$2" value status
    value="$(json_field "$text" "$path")"
    status=$?
    printf '%s|%s' "$status" "$value"
}

# ── The defect: a parse failure and an absent field are different answers ──
expect "unparseable input reports parse failure, not an empty field" \
    "3|" "$(run_field 'Design audit found no drift' '.count')"

expect "parsed payload with a missing field reports absence" \
    "2|" "$(run_field '{"other":1}' '.count')"

expect "present field returns its value" \
    "0|5" "$(run_field '{"count":5}' '.count')"

expect "explicit null reads as absent, not as the string null" \
    "2|" "$(run_field '{"count":null}' '.count')"

# ── The exact cat-36 shape: prose line, then the envelope ──
PROSE_THEN_JSON='Design audit found 3 finding(s) across 5 element(s)
{"match_count":5,"elements_audited":5}'

expect "the envelope after a prose summary is found" \
    "0|5" "$(run_field "$PROSE_THEN_JSON" '.match_count')"

expect "a field absent from an envelope behind prose still reads as absent" \
    "2|" "$(run_field "$PROSE_THEN_JSON" '.missing')"

# A summary line containing braces must not be mistaken for the payload.
BRACED_PROSE='Matched {"a"} in the page
{"match_count":7}'
expect "a braced prose line does not shadow the real payload" \
    "0|7" "$(run_field "$BRACED_PROSE" '.match_count')"

# ── A filter that will not compile is a test bug, not a verdict ──
expect "an invalid jq filter reports parse failure rather than absence" \
    "3|" "$(run_field '{"count":5}' '.[[[')"

# ── Empty and whitespace input ──
expect "empty input reports parse failure" "3|" "$(run_field '' '.count')"
expect "whitespace-only input reports parse failure" "3|" "$(run_field '   ' '.count')"

# ── json_payload isolates the value ──
payload="$(json_payload "$PROSE_THEN_JSON")"
expect "json_payload returns just the JSON value" \
    '{"match_count":5,"elements_audited":5}' "$payload"

if json_payload 'not json at all' >/dev/null 2>&1; then
    echo "FAIL: json_payload accepted a non-JSON body"
    FAILURES=$((FAILURES + 1))
fi

# ── json_field_or keeps the fallback distinct from a parse failure ──
expect "json_field_or substitutes the fallback for an absent field" \
    "0" "$(json_field_or '{"other":1}' '.count' '0' >/dev/null 2>&1; echo $?)"
expect "json_field_or prints the fallback value" \
    "0" "$(json_field_or '{"other":1}' '.count' '0')"
expect "json_field_or still reports a parse failure" \
    "3" "$(json_field_or 'prose only' '.count' '0' >/dev/null 2>&1; echo $?)"

# ── Multi-line envelope with no prose ──
MULTILINE='{
  "count": 2
}'
expect "a pretty-printed envelope parses" "0|2" "$(run_field "$MULTILINE" '.count')"

if [ "$FAILURES" -eq 0 ]; then
    echo "OK: json.sh distinguishes parse failure, absent field, and value"
    exit 0
fi
echo "FAILED: $FAILURES assertion(s)"
exit 1
