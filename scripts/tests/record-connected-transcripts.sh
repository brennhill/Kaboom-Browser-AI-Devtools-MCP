#!/bin/bash
# record-connected-transcripts.sh — Capture one transcript per connected category.
# Docs: docs/features/feature/self-testing/index.md
#
# PURPOSE: the connected categories prove browser features still work, and they
# need Chrome, the extension and a person. Recording one live run turns them
# into a headless CI job. This is the only step that needs the browser, and it
# needs it once per meaningful change to what the extension returns.
#
# Usage (with Chrome open and the Kaboom extension connected):
#   scripts/tests/record-connected-transcripts.sh
#   scripts/tests/record-connected-transcripts.sh --category 36
#
# Then replay them:
#   KABOOM_UAT_REPLAY=scripts/tests/transcripts \
#     scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="${KABOOM_UAT_REPLAY:-$REPO_ROOT/scripts/tests/transcripts}"

# Mirrors CONNECTED_CAT_IDS in scripts/uat/runners/test-all-tools-comprehensive.sh.
# Kept as one list here rather than sourced, because recording deliberately runs
# one category at a time against a daemon it owns.
CONNECTED_CATEGORIES="15 33 35 36 18 19 22 23 24 30 31"

ONLY_CATEGORY=""
if [ "${1:-}" = "--category" ]; then
    ONLY_CATEGORY="${2:?--category needs an id}"
    CONNECTED_CATEGORIES="$ONLY_CATEGORY"
fi

# Recording writes into the repo. Refuse on a dirty transcripts directory so a
# partial run cannot leave half-updated fixtures that look deliberate.
if [ -d "$OUTPUT_DIR" ] && [ -n "$(git -C "$REPO_ROOT" status --porcelain -- "$OUTPUT_DIR")" ]; then
    echo "REFUSING: $OUTPUT_DIR has uncommitted changes." >&2
    echo "Commit or discard them first — a partial re-record is hard to tell from a real diff." >&2
    exit 1
fi

mkdir -p "$OUTPUT_DIR"

category_script() {
    local id="$1"
    find "$SCRIPT_DIR" -name "cat-${id}-*.sh" -type f | head -n 1
}

RECORD_PORT="${KABOOM_UAT_RECORD_PORT:-7893}"
WRAPPER="${KABOOM_UAT_WRAPPER:-$REPO_ROOT/dist/kaboom-agentic-browser}"

if [ ! -x "$WRAPPER" ]; then
    echo "Building the daemon for recording..." >&2
    (cd "$REPO_ROOT" && go build -o "$WRAPPER" ./cmd/browser-agent)
fi

recorded=0
skipped=""

for id in $CONNECTED_CATEGORIES; do
    script="$(category_script "$id")"
    if [ -z "$script" ]; then
        skipped="$skipped $id(no-script)"
        continue
    fi
    transcript="$OUTPUT_DIR/cat-${id}.jsonl"
    rm -f "$transcript"

    echo ""
    echo "=== Recording category $id ($(basename "$script")) ==="

    # KABOOM_SYNC_TRANSCRIPT makes the daemon append every command exchange it
    # brokers. The category starts its own daemon through the framework, so the
    # variable is exported for the whole category run rather than set on a
    # daemon started here.
    set +e
    KABOOM_SYNC_TRANSCRIPT="$transcript" \
    KABOOM_UAT_REQUIRE_CONNECTED=1 \
    KABOOM_UAT_WRAPPER="$WRAPPER" \
        bash "$script" "$RECORD_PORT" /dev/stdout
    status=$?
    set -e

    if [ ! -s "$transcript" ]; then
        # An empty transcript replays as a browser that answers nothing, which
        # is indistinguishable from one that never attached. Do not keep it.
        rm -f "$transcript"
        skipped="$skipped $id(no-exchanges)"
        echo "  category $id recorded no command exchanges (exit $status)" >&2
        continue
    fi

    lines="$(wc -l < "$transcript" | tr -d ' ')"
    echo "  category $id: $lines exchange(s) -> $transcript (category exit $status)"
    recorded=$((recorded + 1))
done

echo ""
echo "Recorded $recorded transcript(s) into $OUTPUT_DIR"
[ -n "$skipped" ] && echo "Skipped:$skipped"
echo ""
echo "Verify the replay before committing:"
echo "  KABOOM_UAT_REPLAY=$OUTPUT_DIR scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected"
