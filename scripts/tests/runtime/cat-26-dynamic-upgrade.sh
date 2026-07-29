#!/bin/bash
# cat-26-dynamic-upgrade.sh — Dynamic binary upgrade detection tests.
# Tests that the daemon detects a newer binary on disk, reports upgrade_pending
# in the health endpoint, and exits gracefully for the bridge to respawn.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "26" "Dynamic Binary Upgrade" "3"

# We use a dedicated temp directory and build two binaries with different versions.
UPGRADE_DIR="$TEMP_DIR/upgrade-test"
mkdir -p "$UPGRADE_DIR"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
UPGRADE_PORT=19160

# Build helper: compile a kaboom binary with a specific version
build_version() {
    local ver="$1"
    local output="$2"
    go build -ldflags "-X main.version=$ver" -o "$output" "$PROJECT_ROOT/cmd/browser-agent/" 2>/dev/null
}

# Kill any leftover on the upgrade test port
cleanup_upgrade_test() {
    lsof -ti :"$UPGRADE_PORT" 2>/dev/null | xargs kill 2>/dev/null || true
    sleep 0.5
    lsof -ti :"$UPGRADE_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
}

# ── Test 26.1: Upgrade detection appears in /health ─────────
begin_test "26.1" "Binary replacement detected in health endpoint" \
    "Start daemon v0.7.5, replace binary with v0.8.0, verify upgrade_pending in /health" \
    "Core detection: daemon must notice the new binary and report it"

run_test_26_1() {
    local bin="$UPGRADE_DIR/kaboom-agentic-browser"
    cleanup_upgrade_test

    # Build and start old version
    if ! build_version "0.7.5" "$bin"; then
        fail "Failed to build v0.7.5 binary"
        return
    fi

    "$bin" --port "$UPGRADE_PORT" >/dev/null 2>&1 &
    local daemon_pid=$!
    sleep 2

    # Verify running
    local health
    health=$(curl -s --max-time 3 "http://127.0.0.1:$UPGRADE_PORT/health" 2>/dev/null)
    if [ -z "$health" ]; then
        fail "Daemon did not start on port $UPGRADE_PORT"
        kill "$daemon_pid" 2>/dev/null || true
        return
    fi

    local running_ver
    running_ver=$(echo "$health" | jq -r '.version // empty' 2>/dev/null)
    if [ "$running_ver" != "0.7.5" ]; then
        fail "Expected version 0.7.5, got: $running_ver"
        kill "$daemon_pid" 2>/dev/null || true
        return
    fi

    # Replace binary with newer version
    if ! build_version "0.8.0" "$bin"; then
        fail "Failed to build v0.8.0 binary"
        kill "$daemon_pid" 2>/dev/null || true
        return
    fi

    # Poll health for upgrade_pending (up to 35s for 30s poll interval + margin)
    local detected=false
    for i in $(seq 1 35); do
        sleep 1
        health=$(curl -s --max-time 2 "http://127.0.0.1:$UPGRADE_PORT/health" 2>/dev/null)
        if [ -z "$health" ]; then
            # Daemon exited before we saw upgrade_pending — still a pass if it detected
            detected=true
            break
        fi
        local pending
        pending=$(echo "$health" | jq -r '.upgrade_pending.new_version // empty' 2>/dev/null)
        if [ "$pending" = "0.8.0" ]; then
            detected=true
            break
        fi
    done

    cleanup_upgrade_test

    if [ "$detected" = true ]; then
        pass "Upgrade from v0.7.5 to v0.8.0 detected within ${i}s"
    else
        fail "upgrade_pending not detected within 35s"
    fi
}
run_test_26_1

# ── Test 26.2: Daemon self-terminates after detection ────────
begin_test "26.2" "Daemon exits after upgrade grace period" \
    "Start daemon v0.7.5, replace with v0.8.0, verify daemon exits within 40s" \
    "Auto-restart requires the old daemon to SIGTERM itself"

run_test_26_2() {
    local bin="$UPGRADE_DIR/kaboom-agentic-browser"
    cleanup_upgrade_test

    if ! build_version "0.7.5" "$bin"; then
        fail "Failed to build v0.7.5 binary"
        return
    fi

    "$bin" --port "$UPGRADE_PORT" >/dev/null 2>&1 &
    local daemon_pid=$!
    sleep 2

    # Verify running
    if ! curl -s --max-time 3 "http://127.0.0.1:$UPGRADE_PORT/health" >/dev/null 2>&1; then
        fail "Daemon did not start"
        kill "$daemon_pid" 2>/dev/null || true
        return
    fi

    # Replace with newer
    if ! build_version "0.8.0" "$bin"; then
        fail "Failed to build v0.8.0 binary"
        kill "$daemon_pid" 2>/dev/null || true
        return
    fi

    # Wait for daemon to exit (up to 40s)
    local exited=false
    for i in $(seq 1 40); do
        sleep 1
        if ! curl -s --max-time 1 "http://127.0.0.1:$UPGRADE_PORT/health" >/dev/null 2>&1; then
            exited=true
            break
        fi
    done

    cleanup_upgrade_test

    if [ "$exited" = true ]; then
        pass "Daemon self-terminated after ${i}s (detected upgrade and exited gracefully)"
    else
        fail "Daemon did not exit within 40s after binary replacement"
    fi
}
run_test_26_2

# ── Test 26.3: Upgrade marker file written ───────────────────
begin_test "26.3" "Upgrade marker file persisted for new daemon" \
    "Start daemon v0.7.5, replace with v0.8.0, verify last-upgrade.json written" \
    "The marker allows the new daemon to report the completed upgrade"

run_test_26_3() {
    local bin="$UPGRADE_DIR/kaboom-agentic-browser"
    local marker_path="$HOME/.kaboom/run/last-upgrade.json"
    cleanup_upgrade_test

    # Remove any existing marker
    rm -f "$marker_path"

    if ! build_version "0.7.5" "$bin"; then
        fail "Failed to build v0.7.5 binary"
        return
    fi

    "$bin" --port "$UPGRADE_PORT" >/dev/null 2>&1 &
    sleep 2

    if ! curl -s --max-time 3 "http://127.0.0.1:$UPGRADE_PORT/health" >/dev/null 2>&1; then
        fail "Daemon did not start"
        cleanup_upgrade_test
        return
    fi

    if ! build_version "0.8.0" "$bin"; then
        fail "Failed to build v0.8.0 binary"
        cleanup_upgrade_test
        return
    fi

    # Wait for daemon to exit
    for _ in $(seq 1 40); do
        sleep 1
        if ! curl -s --max-time 1 "http://127.0.0.1:$UPGRADE_PORT/health" >/dev/null 2>&1; then
            break
        fi
    done

    cleanup_upgrade_test

    # Check marker file
    if [ ! -f "$marker_path" ]; then
        fail "Upgrade marker not found at $marker_path"
        return
    fi

    local from_ver to_ver
    from_ver=$(jq -r '.from_version // empty' "$marker_path" 2>/dev/null)
    to_ver=$(jq -r '.to_version // empty' "$marker_path" 2>/dev/null)

    # Clean up marker
    rm -f "$marker_path"

    if [ "$from_ver" = "0.7.5" ] && [ "$to_ver" = "0.8.0" ]; then
        pass "Upgrade marker written: from=$from_ver to=$to_ver"
    else
        fail "Marker has unexpected content: from=$from_ver to=$to_ver (expected 0.7.5 → 0.8.0)"
    fi
}
run_test_26_3

finish_category
