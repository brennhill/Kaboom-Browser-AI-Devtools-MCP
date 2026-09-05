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
#   scripts/tests/transcripts/record-connected-transcripts.sh
#   scripts/tests/transcripts/record-connected-transcripts.sh --category 36
#
# Then replay them:
#   KABOOM_UAT_REPLAY=scripts/tests/transcripts \
#     scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TESTS_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
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

# shellcheck source=scripts/uat/orchestration/uat-category-script.sh
source "$REPO_ROOT/scripts/uat/orchestration/uat-category-script.sh"
# shellcheck source=scripts/uat/orchestration/uat-browser-launch.sh
source "$REPO_ROOT/scripts/uat/orchestration/uat-browser-launch.sh"

RECORD_PORT="${KABOOM_UAT_RECORD_PORT:-7893}"
WRAPPER="${KABOOM_UAT_WRAPPER:-$REPO_ROOT/dist/kaboom-agentic-browser}"

if [ ! -x "$WRAPPER" ]; then
    echo "Building the daemon for recording..." >&2
    (cd "$REPO_ROOT" && go build -o "$WRAPPER" ./cmd/browser-agent)
fi

# KABOOM_UAT_LAUNCH_BROWSER=1 records against a Chrome this script starts with
# THIS tree's extension loaded, instead of whatever the machine happens to have.
# Opt-in rather than default: a browser window appearing unasked is worse than a
# clear failure telling you to ask for one.
if [ "${KABOOM_UAT_LAUNCH_BROWSER:-0}" = "1" ]; then
    BROWSER_PROFILE="$(mktemp -d)"
    BOOTSTRAP_PID=""
    # shellcheck disable=SC2064  # expand the profile path now, at trap time it may be unset
    trap "uat_stop_extension_browser; [ -n \"\$BOOTSTRAP_PID\" ] && kill \$BOOTSTRAP_PID 2>/dev/null; rm -rf $BROWSER_PROFILE" EXIT

    # The extension needs something to check in to, and each category starts and
    # kills its own daemon on this port. This one exists only long enough to prove
    # the browser attached; the extension reconnects to each category's daemon.
    "$WRAPPER" --daemon --parallel --port "$RECORD_PORT" >/dev/null 2>&1 &
    BOOTSTRAP_PID=$!
    for _ in $(seq 1 30); do
        curl -s --connect-timeout 1 "http://127.0.0.1:${RECORD_PORT}/health" >/dev/null 2>&1 && break
        sleep 1
    done

    echo "Launching a browser with $REPO_ROOT/extension loaded..."
    uat_launch_extension_browser "$REPO_ROOT/extension" "$BROWSER_PROFILE" "$RECORD_PORT" 60 >/dev/null
    echo "  Browser attached; its extension is the one this tree just compiled."
fi

recorded=0
skipped=""
failed=""

for id in $CONNECTED_CATEGORIES; do
    if ! script="$(uat_resolve_category_script "$TESTS_ROOT" "$id")"; then
        skipped="$skipped $id(unresolved)"
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

    if [ "$status" -ne 0 ]; then
        # A recording is a claim about what a working browser answers. Keeping the
        # exchanges from a category that FAILED turns the replay job green against
        # a browser that refused the commands: on 2026-09-05 every connected
        # category exited 1 with command_contract_mismatch, and the run still
        # reported "Recorded 10 transcript(s)" because each file was non-empty.
        rm -f "$transcript"
        failed="$failed $id(exit $status)"
        echo "  category $id FAILED (exit $status); its transcript is discarded, not committed" >&2
        continue
    fi

    lines="$(wc -l < "$transcript" | tr -d ' ')"
    echo "  category $id: $lines exchange(s) -> $transcript (category exit $status)"
    recorded=$((recorded + 1))
done

echo ""
echo "Recorded $recorded transcript(s) into $OUTPUT_DIR"
[ -n "$skipped" ] && echo "Skipped:$skipped"

if [ -n "$failed" ]; then
    # Exit non-zero so a recording run that produced nothing usable cannot be
    # mistaken for one that did, by a person or by a job.
    echo "FAILED:$failed" >&2
    echo "" >&2
    echo "No transcript was kept for those categories. Fix the failures against the live" >&2
    echo "browser first — a transcript recorded from a red category records the red." >&2
    echo "A command_contract_mismatch means the extension loaded in Chrome is older than" >&2
    echo "this tree: reload it from $REPO_ROOT/extension and re-run." >&2
    exit 1
fi

echo ""
echo "Verify the replay before committing:"
echo "  KABOOM_UAT_REPLAY=$OUTPUT_DIR scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected"
