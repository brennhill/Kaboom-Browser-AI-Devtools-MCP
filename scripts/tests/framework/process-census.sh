#!/bin/bash
# process-census.sh — Fail the run when kaboom processes multiply.
# Docs: docs/core/reliability/zombie-prevention.md
#
# WHY THIS ASSERTS INSTEAD OF SWEEPING:
# cleanup-test-daemons.sh already sweeps, and a sweep that silently matches
# nothing looks exactly like a clean run. That is not hypothetical here — the
# daemon rewrites its own process title, the sweep pattern stopped matching, and
# twelve daemons stayed alive for twenty hours while every suite reported green.
# Separately, a Node launcher blocked in execFileSync leaked ~6,900 orphans.
# Both were invisible because nothing ever counted.
#
# So: count before cleaning, and FAIL on growth. A leak must cost a red run.
#
# The census is baseline-relative because a developer legitimately has their own
# daemon running. It measures growth caused by the suite, not absolute presence.

# ── What counts as a suite-owned kaboom process ───────────
#
# These patterns must NEVER match the user's production daemon. Sweeping it
# would kill a developer's working session; counting it would make every run
# fail on their machine. tests/cli/uat-assertions/process-census.test.cjs pins
# both directions.
#
# The `[^ ]*` after the base name absorbs the compact version tag the daemon
# writes into its own title (procctl.Argv0ForVersion → kaboom-test-binary-090).
# A pattern assuming a space here matches nothing; that is the exact bug above.
census_patterns() {
    cat <<'PATTERNS'
kaboom-test-binary[^ ]* --daemon --port
kaboom-test-binary[^ ]* --port
PATTERNS
}

# A Node process running a kaboom bin means a launcher came back. The bin
# entries are POSIX exec shims precisely so no such process can exist; if one
# appears, the launcher regressed and every session will leak one.
# Anchored on a separator, not end-of-line: a launcher is invoked WITH arguments
# ("node .../bin/kaboom-agentic-browser --port 7890"), so a `$` anchor would miss
# every real one. Requiring ' ' or end-of-line after the name also keeps this from
# matching node running a FILE inside the package
# (".../kaboom-agentic-browser/lib/cli/cli.js" is followed by '/', not a space) —
# that is the shim's own CLI branch, which is short-lived and legitimate.
LAUNCHER_PATTERN='node .*(kaboom-agentic-browser|kaboom-hooks)( |$)'

_census_ps() {
    ps -eo pid=,ppid=,args= 2>/dev/null || true
}

# Print one line per matching process: "<pid> <ppid> <command>".
kaboom_census() {
    local snapshot
    snapshot="$(_census_ps)"
    {
        while IFS= read -r pattern; do
            [ -z "$pattern" ] && continue
            printf '%s\n' "$snapshot" | grep -E -- "$pattern" || true
        done < <(census_patterns)
        printf '%s\n' "$snapshot" | grep -E -- "$LAUNCHER_PATTERN" || true
    } | grep -v -E 'grep -E|process-census' | sort -u
}

kaboom_census_count() {
    local census
    census="$(kaboom_census)"
    [ -z "$census" ] && { echo 0; return; }
    printf '%s\n' "$census" | wc -l | tr -d ' '
}

# Record the count the suite starts from. Everything after is measured against it.
KABOOM_CENSUS_BASELINE=""
record_census_baseline() {
    KABOOM_CENSUS_BASELINE="$(kaboom_census_count)"
    export KABOOM_CENSUS_BASELINE
    if [ "$KABOOM_CENSUS_BASELINE" != "0" ]; then
        echo "Process census baseline: $KABOOM_CENSUS_BASELINE pre-existing kaboom process(es)."
    fi
}

# How long to let processes finish exiting before calling it a leak. Shutdown is
# not instantaneous, and an immediate count would flag a daemon mid-exit. Waiting
# is the deliberate cost of never missing a real leak.
KABOOM_CENSUS_SETTLE_SECONDS="${KABOOM_CENSUS_SETTLE_SECONDS:-15}"

# assert_no_process_growth <label>
# Returns 0 when the census has returned to baseline, 1 (and prints the offending
# processes) when it has not. Poll so a slow-but-correct shutdown passes and only
# a genuine leak fails.
assert_no_process_growth() {
    local label="$1"
    local baseline="${KABOOM_CENSUS_BASELINE:-0}"
    local deadline=$((SECONDS + KABOOM_CENSUS_SETTLE_SECONDS))
    local count

    while :; do
        count="$(kaboom_census_count)"
        [ "$count" -le "$baseline" ] && return 0
        [ "$SECONDS" -ge "$deadline" ] && break
        sleep 0.5
    done

    echo "" >&2
    echo "PROCESS LEAK after ${label}: ${count} kaboom process(es), expected at most ${baseline}." >&2
    echo "They outlived the work that started them and nothing would have reaped them." >&2
    echo "" >&2
    printf '%s\n' "$(kaboom_census)" | sed 's/^/    /' >&2
    echo "" >&2
    return 1
}

# A launcher process is never acceptable, at any count, even at baseline: the
# bin entries exec their binary, so the only way one exists is a regression.
assert_no_launcher_processes() {
    local label="$1"
    local launchers
    launchers="$(_census_ps | grep -E -- "$LAUNCHER_PATTERN" | grep -v -E 'grep -E|process-census' || true)"
    [ -z "$launchers" ] && return 0

    echo "" >&2
    echo "LAUNCHER REGRESSION after ${label}: a Node process is sitting in front of a kaboom binary." >&2
    echo "bin/kaboom-agentic-browser and bin/kaboom-hooks are exec shims; this process should not exist." >&2
    echo "It cannot be signalled through its parent and will be reparented to PID 1 when that parent dies." >&2
    echo "" >&2
    printf '%s\n' "$launchers" | sed 's/^/    /' >&2
    echo "" >&2
    return 1
}

# The most literal form of "never multiple copies": two kaboom processes serving
# the same port. This holds regardless of how many daemons are legitimately
# running, so it is safe where a long-lived daemon is intentional (smoke keeps
# one alive by default) and growth-from-baseline would be the wrong question.
#
# A second process on a port is never benign — one of the two lost the bind and
# is sitting there doing nothing, or they are fighting over the same state dir.
assert_no_duplicate_daemons() {
    local label="$1"
    local dupes
    dupes="$(kaboom_census \
        | grep -oE -- '--port [0-9]+' \
        | sort | uniq -c \
        | awk '$1 > 1 { print $1 " processes on " $2 " " $3 }')"
    [ -z "$dupes" ] && return 0

    echo "" >&2
    echo "DUPLICATE DAEMONS after ${label}: more than one kaboom process is serving the same port." >&2
    printf '%s\n' "$dupes" | sed 's/^/    /' >&2
    echo "" >&2
    echo "Full census:" >&2
    printf '%s\n' "$(kaboom_census)" | sed 's/^/    /' >&2
    echo "" >&2
    return 1
}
