#!/bin/bash
# json.sh — Parse-failure-aware JSON access for UAT assertions.
#
# PURPOSE: `printf '%s' "$text" | jq -r '.field' 2>/dev/null` prints nothing
# when the input does not parse, and "" compares equal to an expected-empty
# result. Every "expected zero findings" control in the suite is therefore
# satisfied by a response that was never parsed at all.
#
# That is not hypothetical. cat-36's fixture check piped a whole tool response
# — a prose summary line followed by the JSON envelope — into jq. jq aborts on
# the prose and emits nothing, so a parse failure read as "the fixture rendered
# 0 cards" and sent the author looking at the fixture instead of the pipeline.
#
# CONTRACT: three outcomes, never conflated.
#   0  the field was found; its value is on stdout
#   2  the payload parsed, but the field is absent or null
#   3  the payload could not be parsed at all
#
# A caller that ignores the distinction is no worse off than before; a caller
# that checks it cannot mistake silence for an answer.

JSON_FIELD_ABSENT=2
JSON_PARSE_FAILED=3

# json_parses <text> — true when the text is one complete JSON value.
#
# Empty input is rejected explicitly: jq given nothing exits 0 and prints
# nothing, so without this guard "the peer sent no response at all" would be
# indistinguishable from "the response parsed and the field was absent" — the
# very conflation this file exists to prevent.
json_parses() {
    case "$1" in
        *[![:space:]]*) ;;
        *) return 1 ;;
    esac
    printf '%s' "$1" | jq . >/dev/null 2>&1
}

# json_payload <text>
#
# Prints the JSON value carried by a response, which may be preceded by a
# human-readable summary line. Exit 3 when no JSON value can be found.
json_payload() {
    local text="$1" candidate

    if json_parses "$text"; then
        printf '%s' "$text"
        return 0
    fi

    # Tool responses lead with a prose summary and end with the envelope, so
    # the payload is the last line. Checked BEFORE the brace scan because a
    # summary line may itself contain braces.
    candidate="$(printf '%s' "$text" | tail -n 1)"
    if json_parses "$candidate"; then
        printf '%s' "$candidate"
        return 0
    fi

    # A multi-line envelope: take everything from the first line that opens a
    # JSON value to the end.
    candidate="$(printf '%s' "$text" | sed -n '/^[[{]/,$p')"
    if [ -n "$candidate" ] && json_parses "$candidate"; then
        printf '%s' "$candidate"
        return 0
    fi

    return "$JSON_PARSE_FAILED"
}

# json_field <text> <jq_path>
#
# Prints the field's value. Exit 2 when the payload parsed but the field is
# absent or null; exit 3 when the payload did not parse.
json_field() {
    local text="$1" path="$2" payload value

    if ! payload="$(json_payload "$text")"; then
        return "$JSON_PARSE_FAILED"
    fi
    if ! value="$(printf '%s' "$payload" | jq -r "$path" 2>/dev/null)"; then
        # A filter that will not compile is a mistake in the test, not a
        # verdict about the response, and must not read as an absent field.
        return "$JSON_PARSE_FAILED"
    fi
    if [ "$value" = "null" ] || [ -z "$value" ]; then
        return "$JSON_FIELD_ABSENT"
    fi
    printf '%s' "$value"
}

# json_field_or <text> <jq_path> <fallback>
#
# For the common case where an absent field has a sensible default but an
# unparseable response must still be distinguishable. Prints the fallback on
# exit 2, and still returns 3 when nothing parsed.
json_field_or() {
    local text="$1" path="$2" fallback="$3" value status
    value="$(json_field "$text" "$path")"
    status=$?
    case "$status" in
        0) printf '%s' "$value" ;;
        "$JSON_FIELD_ABSENT") printf '%s' "$fallback" ;;
        *) return "$JSON_PARSE_FAILED" ;;
    esac
}

# require_json_field <text> <jq_path> <what>
#
# Asserts the field is present, failing the current test with a message that
# names which of the two failures occurred. Requires framework.sh's `fail`.
require_json_field() {
    local text="$1" path="$2" what="$3" value status
    value="$(json_field "$text" "$path")"
    status=$?
    case "$status" in
        0) printf '%s' "$value"; return 0 ;;
        "$JSON_FIELD_ABSENT")
            fail "$what: the response parsed but carried no $path"
            return 1 ;;
        *)
            fail "$what: the response was not parseable JSON — $(truncate "$text")"
            return 1 ;;
    esac
}
