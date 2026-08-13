#!/bin/bash
# uat-replay.sh — Runs the connected categories against a recorded transcript.
# Docs: docs/features/feature/self-testing/index.md
#
# PURPOSE: the connected categories are the only tests that prove a browser
# feature still works, and they need Chrome, the extension and a person — so
# they run nowhere automated. With KABOOM_UAT_REPLAY pointing at a directory of
# recorded transcripts, the same category scripts run headless against a fake
# extension.
#
# CONTRACT: replay mode changes who answers the browser's commands, nothing
# else. The categories are not modified and are not told which mode they are in,
# so a category that passes here is asserting the same things it asserts live.
#
# Record with:
#   scripts/tests/record-connected-transcripts.sh
# Replay with:
#   KABOOM_UAT_REPLAY=scripts/tests/transcripts \
#     scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected

UAT_REPLAY_PID=""
UAT_REPLAY_TRANSCRIPT=""
UAT_REPLAY_LOG=""

# uat_replay_enabled — true when the suite should answer from a recording.
uat_replay_enabled() {
    [ -n "${KABOOM_UAT_REPLAY:-}" ]
}

# uat_replay_binary — the replay extension binary, built on first use.
uat_replay_binary() {
    local built="${KABOOM_UAT_REPLAY_BINARY:-}"
    if [ -n "$built" ]; then
        printf '%s' "$built"
        return 0
    fi
    printf '%s' "${TMPDIR:-/tmp}/kaboom-replay-extension"
}

# uat_replay_build — compiles the replay binary unless one was supplied.
uat_replay_build() {
    local binary
    binary="$(uat_replay_binary)"
    [ -n "${KABOOM_UAT_REPLAY_BINARY:-}" ] && return 0
    [ -x "$binary" ] && return 0
    ( cd "$UAT_REPLAY_REPO_ROOT" && go build -o "$binary" ./cmd/kaboom-replay-extension ) || {
        echo "FATAL: could not build kaboom-replay-extension" >&2
        return 1
    }
}

# uat_replay_transcript_for <category_id> — the recording for one category.
#
# Falls back to a shared transcript so a category with no recording of its own
# still runs, rather than being silently skipped.
uat_replay_transcript_for() {
    local category="$1"
    local specific="${KABOOM_UAT_REPLAY}/cat-${category}.jsonl"
    if [ -f "$specific" ]; then
        printf '%s' "$specific"
        return 0
    fi
    local shared="${KABOOM_UAT_REPLAY}/connected.jsonl"
    if [ -f "$shared" ]; then
        printf '%s' "$shared"
        return 0
    fi
    return 1
}

# uat_replay_start <port> <category_id> — attaches a fake extension to the daemon.
uat_replay_start() {
    local port="$1"
    local category="${2:-}"

    uat_replay_stop
    uat_replay_build || return 1

    if ! UAT_REPLAY_TRANSCRIPT="$(uat_replay_transcript_for "$category")"; then
        echo "FATAL: no transcript for category ${category} in ${KABOOM_UAT_REPLAY}" >&2
        echo "       record one with scripts/tests/record-connected-transcripts.sh" >&2
        return 1
    fi

    UAT_REPLAY_LOG="${TMPDIR:-/tmp}/kaboom-replay-cat-${category}-${port}.log"
    nohup "$(uat_replay_binary)" \
        --port "$port" \
        --transcript "$UAT_REPLAY_TRANSCRIPT" \
        --strict=false \
        >"$UAT_REPLAY_LOG" 2>&1 </dev/null &
    UAT_REPLAY_PID=$!
    echo "  Replay extension started (PID $UAT_REPLAY_PID, $(basename "$UAT_REPLAY_TRANSCRIPT"))"
}

# uat_replay_stop — detaches the fake extension and reports its coverage.
#
# The summary is printed because a category can pass while the transcript
# answered nothing it was asked: every miss is an error result, and a category
# asserting only "not an MCP error" would not notice.
uat_replay_stop() {
    [ -n "$UAT_REPLAY_PID" ] || return 0
    kill "$UAT_REPLAY_PID" 2>/dev/null
    wait "$UAT_REPLAY_PID" 2>/dev/null
    if [ -n "$UAT_REPLAY_LOG" ] && [ -f "$UAT_REPLAY_LOG" ]; then
        grep -E "answered|no recorded answer" "$UAT_REPLAY_LOG" | sed 's/^/  replay: /' || true
    fi
    UAT_REPLAY_PID=""
    UAT_REPLAY_LOG=""
}

# uat_replay_repo_root is resolved once so uat_replay_build does not depend on
# the caller's working directory.
UAT_REPLAY_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

# uat_replay_tracked_tab_id <port> <wrapper> — the tab the transcript reports.
#
# Asked through the daemon rather than read from the file: that is the same
# answer the readiness check will get, so a transcript whose tabs recording is
# missing or stale fails here with a clear message instead of failing later as
# an unexplained readiness timeout.
uat_replay_tracked_tab_id() {
    local port="$1"
    local wrapper="$2"
    local attempt=0
    local tracked=""
    while [ "$attempt" -lt "${UAT_CONNECTED_READY_ATTEMPTS:-450}" ]; do
        tracked="$(uat_connected_tracked_tab "$port" "$wrapper")"
        if [ -n "$tracked" ]; then
            printf '%s' "$tracked" | cut -f1
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 0.1
    done
    return 1
}
