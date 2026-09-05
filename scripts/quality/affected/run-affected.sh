#!/bin/bash
# run-affected.sh — Runs only the tests a change reaches, or everything when it cannot tell.
#
# Usage: run-affected.sh [base-ref]   (default: UNSTABLE)
#
# The fallback is the point. A branch gate that silently skips the test a change
# broke is worse than a slow one: two branches reported green and paid for it at
# integration (kaboom-2n0x).

set -uo pipefail

BASE="${1:-UNSTABLE}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT" || exit 1

SELECTION="$(node scripts/quality/affected/affected-tests.mjs --base "$BASE" --format json)" || {
    echo "affected-tests failed; falling back to the full suite so nothing is skipped silently." >&2
    exec npm run test:ext
}

FULL="$(printf '%s' "$SELECTION" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(String(JSON.parse(s).full_suite)))')"
if [ "$FULL" = "true" ]; then
    REASON="$(printf '%s' "$SELECTION" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(JSON.parse(s).full_suite_reason))')"
    echo "Full suite: $REASON"
    exec npm run test:ext
fi

JS_TESTS="$(printf '%s' "$SELECTION" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(JSON.parse(s).js_tests.join("\n")))')"
GO_PKGS="$(printf '%s' "$SELECTION" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(JSON.parse(s).go_packages.join("\n")))')"

STATUS=0
if [ -n "$JS_TESTS" ]; then
    COUNT="$(printf '%s\n' "$JS_TESTS" | wc -l | tr -d ' ')"
    echo "=== $COUNT JavaScript test file(s) reached by this change ==="
    # shellcheck disable=SC2086
    printf '%s\n' "$JS_TESTS" | xargs node --test || STATUS=1
fi

if [ -n "$GO_PKGS" ]; then
    COUNT="$(printf '%s\n' "$GO_PKGS" | wc -l | tr -d ' ')"
    echo "=== $COUNT Go package(s) reached by this change ==="
    # shellcheck disable=SC2086
    printf '%s\n' "$GO_PKGS" | xargs go test || STATUS=1
fi

if [ -z "$JS_TESTS" ] && [ -z "$GO_PKGS" ]; then
    echo "Nothing changed that any test reaches. If that is a surprise, it is a bug in the selector — run the full suite."
fi

exit "$STATUS"
