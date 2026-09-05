#!/bin/bash
# uat-browser-launch.sh — Bring up a Chrome that has THIS tree's extension loaded.
# Docs: docs/features/feature/self-testing/index.md
#
# PURPOSE: the connected UAT categories are the only tests that prove a browser
# feature still works, and they need an extension whose command contract matches
# the daemon. Relying on whatever extension a person happens to have loaded makes
# that a coin flip: recording on 2026-09-05 failed all eleven connected
# categories with command_contract_mismatch because the browser held an older
# build than the tree.
#
# Launching Chrome with --load-extension from extension/ removes the guess. The
# browser is disposable, its profile is a temp directory, and the extension is by
# construction the one that was just compiled — so a contract mismatch here means
# a real drift between src/ and internal/commandcontract, not a stale install.
#
# It also removes the human from recording and from the canary. Chrome is
# preinstalled on GitHub's hosted runners, so a job that sources this can run the
# connected categories without a self-hosted machine.

# uat_find_chrome — print the Chrome binary to use, or fail.
uat_find_chrome() {
    if [ -n "${KABOOM_UAT_CHROME:-}" ]; then
        if [ ! -x "$KABOOM_UAT_CHROME" ]; then
            echo "KABOOM_UAT_CHROME=$KABOOM_UAT_CHROME is not executable" >&2
            return 1
        fi
        printf '%s\n' "$KABOOM_UAT_CHROME"
        return 0
    fi

    local candidate
    for candidate in \
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
        "/Applications/Chromium.app/Contents/MacOS/Chromium" \
        "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"; do
        [ -x "$candidate" ] && { printf '%s\n' "$candidate"; return 0; }
    done
    for candidate in google-chrome google-chrome-stable chromium chromium-browser; do
        if command -v "$candidate" >/dev/null 2>&1; then
            command -v "$candidate"
            return 0
        fi
    done

    echo "No Chrome or Chromium found. Set KABOOM_UAT_CHROME to the binary." >&2
    return 1
}

# uat_assert_sole_extension <port>
#
# Refuse to launch while another browser already owns the daemon's extension slot.
#
# The daemon keeps ONE slot: extension_connected and command_contract_id belong to
# whichever browser checked in last. A developer's own Chrome polls 7890 too, so a
# UAT browser sharing that port is indistinguishable from it — which is how a run
# with a freshly compiled extension loaded still failed every category with
# command_contract_mismatch, looking like a flake rather than two browsers.
uat_assert_sole_extension() {
    local port="$1"
    curl -s --connect-timeout 2 "http://127.0.0.1:${port}/health" 2>/dev/null |
        grep -q '"extension_connected":true' || return 0

    {
        echo "An extension is already attached to the daemon on port ${port}."
        echo "The daemon has one extension slot, so a second browser would race the first"
        echo "for it and the run would fail with command_contract_mismatch at random."
        echo "Close the other browser, or point it at a different server URL, then re-run."
    } >&2
    return 1
}

# uat_launch_extension_browser <extension_dir> <profile_dir> <port> [timeout_seconds]
#
# Starts Chrome, waits for the extension to report in to the daemon on <port>,
# and prints the browser's PID. Exports UAT_BROWSER_PID for the caller's trap.
uat_launch_extension_browser() {
    local extension_dir="$1" profile_dir="$2" port="$3" timeout_seconds="${4:-45}"
    local chrome headless=()

    [ -f "$extension_dir/manifest.json" ] ||
        { echo "No manifest.json under $extension_dir; run 'make compile-ts' first" >&2; return 1; }
    chrome="$(uat_find_chrome)" || return 1
    uat_assert_sole_extension "$port" || return 1
    mkdir -p "$profile_dir"

    # Headless is opt-in rather than inferred: a CI job knows it has no display,
    # and silently going headless on a developer's machine hides the window they
    # were about to watch the run in.
    [ "${KABOOM_UAT_CHROME_HEADLESS:-0}" = "1" ] && headless=(--headless=new)

    "$chrome" \
        --user-data-dir="$profile_dir" \
        --load-extension="$extension_dir" \
        --disable-extensions-except="$extension_dir" \
        --no-first-run --no-default-browser-check \
        --disable-background-timer-throttling \
        --disable-backgrounding-occluded-windows \
        --disable-renderer-backgrounding \
        "${headless[@]}" \
        about:blank >"$profile_dir/chrome.log" 2>&1 </dev/null &
    UAT_BROWSER_PID=$!
    export UAT_BROWSER_PID

    if ! uat_wait_for_extension_checkin "$port" "$timeout_seconds"; then
        uat_stop_extension_browser
        return 1
    fi
    printf '%s\n' "$UAT_BROWSER_PID"
}

# uat_wait_for_extension_checkin <port> <timeout_seconds>
uat_wait_for_extension_checkin() {
    local port="$1" timeout_seconds="$2" waited=0
    while [ "$waited" -lt "$timeout_seconds" ]; do
        if curl -s --connect-timeout 2 "http://127.0.0.1:${port}/health" 2>/dev/null |
            grep -q '"extension_connected":true'; then
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    echo "The launched browser never reported in on port ${port} within ${timeout_seconds}s." >&2
    echo "An MV3 service worker that cannot start leaves no error in the page; check the daemon log." >&2
    return 1
}

# uat_stop_extension_browser — stop the browser this library launched.
uat_stop_extension_browser() {
    [ -n "${UAT_BROWSER_PID:-}" ] || return 0
    kill "$UAT_BROWSER_PID" 2>/dev/null || true
    wait "$UAT_BROWSER_PID" 2>/dev/null || true
    unset UAT_BROWSER_PID
}
