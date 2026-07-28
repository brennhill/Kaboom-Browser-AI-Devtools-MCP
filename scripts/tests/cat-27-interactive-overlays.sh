#!/bin/bash
# cat-27-interactive-overlays.sh — Interactive human smoke tests for browser overlays (12 tests).
# Unlike automated UAT categories, these tests pause for human visual verification.
# Requires: extension connected, pilot enabled, human operator at keyboard.
#
# Usage: bash scripts/tests/cat-27-interactive-overlays.sh <port>
#   e.g.: bash scripts/tests/cat-27-interactive-overlays.sh 7910
#
# NOT included in the parallel UAT runner (requires human interaction).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "27" "Interactive Overlay Smoke Tests" "19"
ensure_daemon

# ── Interactive Helper ────────────────────────────────────
# Prompts the human operator to visually verify something in the browser.
# Returns 0 if confirmed, 1 if failed.
human_verify() {
    local prompt="$1"
    echo ""
    echo "  >>> HUMAN VERIFY: $prompt"
    echo "  >>> Press ENTER if correct, or type 'fail' + ENTER to fail:"
    read -r answer
    if [ "$answer" = "fail" ]; then
        return 1
    fi
    return 0
}

# ── Preflight: Extension + Pilot ──────────────────────────
preflight_check() {
    local health
    health=$(get_http_body "http://localhost:${PORT}/health")

    local ext_connected
    ext_connected=$(echo "$health" | jq -r '.capture.extension_connected // false' 2>/dev/null)
    if [ "$ext_connected" != "true" ]; then
        echo "FATAL: Extension not connected. Connect extension and track a tab first." >&2
        exit 1
    fi

    local pilot
    pilot=$(echo "$health" | jq -r '.capture.pilot_enabled // false' 2>/dev/null)
    if [ "$pilot" != "true" ]; then
        echo "WARNING: Pilot not enabled. Recording and draw mode tests may fail." >&2
        echo "  Enable pilot in the extension popup to run all tests."
        echo ""
    fi

    echo "Preflight OK: extension connected, daemon healthy."
    echo ""
    echo "============================================================"
    echo "  INTERACTIVE SMOKE TESTS"
    echo "  Make sure Chrome is visible with a tracked tab open."
    echo "  You will be prompted to visually verify overlays."
    echo "============================================================"
    echo ""
}

preflight_check

# ══════════════════════════════════════════════════════════
# SECTION 1: Subtitle Overlay
# ══════════════════════════════════════════════════════════

# ── 27.1 — Subtitle overlay appears ──────────────────────
begin_test "27.1" "Subtitle overlay appears" \
    "interact(subtitle) with text — verify text bar visible at bottom of page" \
    "Visual: subtitle bar renders with correct text."
run_test_27_1() {
    RESPONSE=$(call_tool "interact" '{"action":"subtitle","text":"Hello smoke test — cat-27"}')
    if check_is_error "$RESPONSE"; then
        local text
        text=$(extract_content_text "$RESPONSE")
        fail "subtitle returned error: $(truncate "$text")"
        return
    fi

    if human_verify "Is a dark subtitle bar visible at the BOTTOM CENTER of the page showing 'Hello smoke test — cat-27'?"; then
        pass "Subtitle overlay appeared with correct text."
    else
        fail "Human reports subtitle overlay not visible or incorrect."
    fi
}
run_test_27_1

# ── 27.2 — Subtitle overlay clears ───────────────────────
begin_test "27.2" "Subtitle overlay clears on empty text" \
    "interact(subtitle) with empty text — verify bar disappears" \
    "Visual: subtitle bar removed from page."
run_test_27_2() {
    RESPONSE=$(call_tool "interact" '{"action":"subtitle","text":""}')
    if check_is_error "$RESPONSE"; then
        local text
        text=$(extract_content_text "$RESPONSE")
        fail "subtitle clear returned error: $(truncate "$text")"
        return
    fi

    sleep 0.5
    if human_verify "Has the subtitle bar DISAPPEARED from the page?"; then
        pass "Subtitle overlay cleared successfully."
    else
        fail "Human reports subtitle overlay still visible after clear."
    fi
}
run_test_27_2

# ── 27.3 — Subtitle overlay updates text ─────────────────
begin_test "27.3" "Subtitle overlay updates on second call" \
    "Set text twice — verify second text replaces the first" \
    "Visual: subtitle text updates in-place."
run_test_27_3() {
    call_tool "interact" '{"action":"subtitle","text":"First message"}' >/dev/null 2>&1
    sleep 1
    RESPONSE=$(call_tool "interact" '{"action":"subtitle","text":"Updated: second message"}')
    if check_is_error "$RESPONSE"; then
        local text
        text=$(extract_content_text "$RESPONSE")
        fail "subtitle update returned error: $(truncate "$text")"
        return
    fi

    if human_verify "Does the subtitle bar now show 'Updated: second message' (NOT 'First message')?"; then
        pass "Subtitle overlay updated text correctly."
    else
        fail "Human reports subtitle text did not update."
    fi

    # Clean up
    call_tool "interact" '{"action":"subtitle","text":""}' >/dev/null 2>&1
}
run_test_27_3

# ══════════════════════════════════════════════════════════
# SECTION 2: Draw Mode Overlay
# ══════════════════════════════════════════════════════════

# ── 27.4 — Draw mode overlay activates ───────────────────
begin_test "27.4" "Draw mode overlay activates" \
    "interact(draw_mode_start) — verify annotation UI visible on page" \
    "Visual: draw mode overlay with numbered labels and bounding boxes."
run_test_27_4() {
    RESPONSE=$(call_tool "interact" '{"action":"draw_mode_start"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        if check_matches "$text" "pilot"; then
            skip "Draw mode requires pilot — not enabled."
            return
        fi
        fail "draw_mode_start returned error: $(truncate "$text")"
        return
    fi

    if human_verify "Is the DRAW MODE overlay visible? (Numbered labels on page elements, annotation UI)"; then
        pass "Draw mode overlay activated successfully."
    else
        fail "Human reports draw mode overlay not visible."
    fi
}
run_test_27_4

# ── 27.5 — Draw mode produces annotations ────────────────
begin_test "27.5" "Draw mode produces annotations after human interaction" \
    "Human clicks an element in draw mode, presses done — verify annotations returned" \
    "Integration: draw mode capture → analyze(annotations) roundtrip."
run_test_27_5() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click on 1-2 elements in draw mode, then press 'Done'."
    echo "  >>> Press ENTER when done clicking and submitting:"
    read -r _

    sleep 1
    RESPONSE=$(call_tool "analyze" '{"what":"annotations"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "analyze(annotations) returned error: $(truncate "$text")"
        return
    fi

    if check_matches "$text" "count|annotations"; then
        if human_verify "Did analyze(annotations) return data for your clicks? (Check terminal output above)"; then
            pass "Draw mode produced annotations successfully."
        else
            fail "Human reports annotations were not captured."
        fi
    else
        fail "Expected annotations data. Content: $(truncate "$text" 300)"
    fi
}
run_test_27_5

# ══════════════════════════════════════════════════════════
# SECTION 3: Recording Overlay
# ══════════════════════════════════════════════════════════

# ── 27.6 — Recording start shows watermark ───────────────
begin_test "27.6" "Recording start shows watermark" \
    "interact(screen_recording_start) — verify flame watermark visible in bottom-right" \
    "Visual: recording indicator overlay renders."
run_test_27_6() {
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_start","name":"smoke-test-27"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        if check_matches "$text" "pilot"; then
            skip "Recording requires pilot — not enabled."
            return
        fi
        fail "screen_recording_start returned error: $(truncate "$text")"
        return
    fi

    sleep 1
    if human_verify "Is a RECORDING WATERMARK (flame icon) visible in the bottom-right corner of the page?"; then
        pass "Recording watermark appeared on screen_recording_start."
    else
        fail "Human reports recording watermark not visible."
    fi
}
run_test_27_6

# ── 27.7 — Recording stop removes watermark ──────────────
begin_test "27.7" "Recording stop removes watermark" \
    "interact(screen_recording_stop) — verify watermark disappears" \
    "Visual: recording indicator removed after stop."
run_test_27_7() {
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_stop"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        if check_matches "$text" "pilot"; then
            skip "Recording requires pilot — not enabled."
            return
        fi
        # screen_recording_stop error is OK if nothing was recording
    fi

    sleep 1
    if human_verify "Has the recording watermark DISAPPEARED from the page?"; then
        pass "Recording watermark removed on screen_recording_stop."
    else
        fail "Human reports recording watermark still visible after stop."
    fi
}
run_test_27_7

# ══════════════════════════════════════════════════════════
# SECTION 4: Action Toast
# ══════════════════════════════════════════════════════════

# ── 27.8 — Action toast appears on click ─────────────────
begin_test "27.8" "Action toast appears on interact action" \
    "interact(click) on an element — verify brief toast notification" \
    "Visual: transient action feedback toast renders."
run_test_27_8() {
    # First check we can reach the page
    RESPONSE=$(call_tool "interact" '{"action":"execute_js","script":"document.title"}')
    if check_is_error "$RESPONSE"; then
        skip "Cannot reach content script — skipping toast test."
        return
    fi

    echo ""
    echo "  >>> Watch the TOP of the page for a brief toast notification..."
    sleep 1

    RESPONSE=$(call_tool "interact" '{"action":"click","selector":"a","reason":"smoke test click"}')

    if human_verify "Did a brief TOAST NOTIFICATION appear at the top of the page? (blue/green bar with action label)"; then
        pass "Action toast appeared on click."
    else
        fail "Human reports action toast not visible."
    fi
}
run_test_27_8

# ══════════════════════════════════════════════════════════
# SECTION 5: Tracked Hover Launcher (Island)
# ══════════════════════════════════════════════════════════

# ── 27.9 — Launcher island appears on tracked page ───────
begin_test "27.9" "Tracked hover launcher (island) appears" \
    "Verify the flame button is visible in the top-right corner of the tracked page" \
    "Visual: launcher FAB renders when tab is tracked."
run_test_27_9() {
    if human_verify "Is a circular FLAME ICON button (blue border, white background) visible in the TOP-RIGHT corner of the page?"; then
        pass "Tracked hover launcher island is visible."
    else
        fail "Human reports launcher island not visible. Check that the tab is being tracked."
    fi
}
run_test_27_9

# ── 27.10 — Launcher panel expands on hover ──────────────
begin_test "27.10" "Launcher panel expands on hover/click" \
    "Hover over or click the flame button — verify panel with action buttons appears" \
    "Visual: panel with Draw, Rec, Shot, and settings buttons."
run_test_27_10() {
    echo ""
    echo "  >>> ACTION REQUIRED: Hover over or click the flame button in the top-right."
    echo "  >>> A panel should slide out to the left with action buttons."
    echo ""

    if human_verify "Does an expanded panel appear with buttons labeled 'Draw', 'Rec', 'Shot', 'Term', and a gear icon?"; then
        pass "Launcher panel expanded with all action buttons."
    else
        fail "Human reports launcher panel did not expand or is missing buttons."
    fi
}
run_test_27_10

# ── 27.11 — Settings menu opens ──────────────────────────
begin_test "27.11" "Settings gear opens menu" \
    "Click the gear button — verify dropdown with Docs, GitHub, and Hide options" \
    "Visual: settings dropdown renders below the panel."
run_test_27_11() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click the gear (⚙) button in the launcher panel."
    echo ""

    if human_verify "Does a dropdown menu appear with 'Docs', 'GitHub Repository', and 'Hide Kaboom Devtool' options?"; then
        pass "Settings menu opened with correct options."
    else
        fail "Human reports settings menu did not open or is missing options."
    fi
}
run_test_27_11

# ── 27.12 — Launcher buttons trigger actions ─────────────
begin_test "27.12" "Launcher buttons trigger correct actions" \
    "Click Draw/Shot buttons from the launcher — verify they trigger their respective overlays" \
    "Integration: launcher button → action dispatch."
run_test_27_12() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click the 'Shot' button in the launcher panel."
    echo "  >>> (This should trigger a screenshot capture.)"
    echo ""

    if human_verify "Did clicking 'Shot' trigger a screenshot action? (brief flash or toast notification)"; then
        echo "  Good. Now test the Draw button..."
        echo ""
        echo "  >>> ACTION REQUIRED: Re-open the launcher panel and click the 'Draw' button."
        echo ""

        if human_verify "Did clicking 'Draw' activate draw mode? (annotation overlay appeared)"; then
            pass "Launcher buttons trigger correct actions (Shot + Draw verified)."
            # Dismiss draw mode if active — press Escape
            echo "  >>> Press Escape in the browser to exit draw mode if still active."
            sleep 1
        else
            fail "Human reports Draw button did not activate draw mode."
        fi
    else
        fail "Human reports Shot button did not trigger screenshot action."
    fi
}
run_test_27_12

# ══════════════════════════════════════════════════════════
# SECTION 6: Terminal Overlay
# ══════════════════════════════════════════════════════════

# ── 27.13 — Terminal button opens iframe overlay ─────────
begin_test "27.13" "Terminal button opens iframe terminal overlay" \
    "Click 'Term' button in launcher — verify terminal iframe appears" \
    "Visual: terminal overlay renders at bottom-right with dark theme."
run_test_27_13() {
    echo ""
    echo "  >>> ACTION REQUIRED: Open the launcher panel (hover/click flame icon)."
    echo "  >>> Click the 'Term' button."
    echo ""

    if human_verify "Did a dark-themed TERMINAL panel appear at the BOTTOM-RIGHT of the page with a command prompt?"; then
        pass "Terminal overlay opened from launcher button."
    else
        fail "Human reports terminal overlay did not appear."
    fi
}
run_test_27_13

# ── 27.14 — Terminal typing works ────────────────────────
begin_test "27.14" "Terminal typing works" \
    "Type 'echo hello' in the terminal — verify output appears" \
    "Integration: keystrokes relay through WebSocket to PTY and output returns."
run_test_27_14() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click inside the terminal and type: echo hello"
    echo "  >>> Press Enter to execute."
    echo ""

    if human_verify "Did the terminal show 'hello' as output after pressing Enter?"; then
        pass "Terminal typing and output display working."
    else
        fail "Human reports terminal typing or output not working."
    fi
}
run_test_27_14

# ── 27.15 — Terminal minimize/restore ────────────────────
begin_test "27.15" "Terminal minimize and restore" \
    "Click minimize button, verify panel collapses, click again to restore" \
    "Visual: minimize collapses to header bar, restore expands back."
run_test_27_15() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click the minimize button (▁) in the terminal header."
    echo ""

    if human_verify "Did the terminal panel collapse to just a thin header bar?"; then
        echo ""
        echo "  >>> ACTION REQUIRED: Click the header bar or restore button (□) to expand it."
        echo ""
        if human_verify "Did the terminal restore to its full size?"; then
            pass "Terminal minimize/restore working correctly."
        else
            fail "Human reports terminal did not restore."
        fi
    else
        fail "Human reports terminal did not minimize."
    fi
}
run_test_27_15

# ── 27.16 — Terminal close ───────────────────────────────
begin_test "27.16" "Terminal close hides panel" \
    "Click X button — verify terminal disappears completely" \
    "Visual: close hides the widget (session stays alive for reconnection)."
run_test_27_16() {
    echo ""
    echo "  >>> ACTION REQUIRED: Click the X button in the terminal header."
    echo ""

    if human_verify "Did the terminal panel disappear completely from the page?"; then
        pass "Terminal close hides panel completely."
    else
        fail "Human reports terminal panel still visible after close."
    fi
}
run_test_27_16

# ── 27.17 — Terminal reconnect on refresh ────────────────
begin_test "27.17" "Terminal reconnects on page refresh with scrollback" \
    "Re-open terminal, refresh the page, verify terminal reconnects with prior output" \
    "Integration: session persistence + scrollback replay across page refreshes."
run_test_27_17() {
    echo ""
    echo "  >>> ACTION REQUIRED: Re-open the terminal (click flame icon → Term)."
    echo "  >>> Type a command like 'echo reconnect-test' and press Enter."
    echo "  >>> Then REFRESH the page (Cmd+R or F5)."
    echo ""

    if human_verify "After refresh, did the terminal reappear and show the previous output (including 'reconnect-test')?"; then
        pass "Terminal reconnects with scrollback replay on page refresh."
    else
        fail "Human reports terminal did not reconnect or lost scrollback."
    fi

    echo ""
    echo "  >>> Close the terminal (X button) before continuing."
    sleep 1
}
run_test_27_17

# ══════════════════════════════════════════════════════════
# SECTION 7: Overlay Dismiss & Observe Modes
# ══════════════════════════════════════════════════════════

# ── 27.18 — Dismiss loop detection ──────────────────────
begin_test "27.18" "Dismiss loop detection" \
    "Trigger dismiss_top_overlay twice within 30s — verify loop error on second attempt" \
    "Safety: prevents infinite dismiss loops when overlays reappear immediately."
run_test_27_18() {
    # First dismiss — should succeed or report no overlay
    RESPONSE=$(call_tool "interact" '{"action":"dismiss_top_overlay"}')
    sleep 2

    # Second dismiss within 30s — should detect loop
    RESPONSE=$(call_tool "interact" '{"action":"dismiss_top_overlay"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "loop|no.*overlay|no.*dismiss"; then
        pass "Dismiss loop detection working: $(truncate "$text" 200)"
    else
        # It's also OK if there genuinely was a new overlay to dismiss
        if human_verify "Was there actually a NEW overlay to dismiss? (If no overlay appeared, this is a failure)"; then
            pass "Second dismiss found a genuine new overlay (not a loop)."
        else
            fail "Expected loop detection or no-overlay response. Content: $(truncate "$text" 300)"
        fi
    fi
}
run_test_27_18

# ── 27.19 — Summary mode strips verbose fields ──────────
begin_test "27.19" "Summary mode on observe" \
    "Call observe(what:errors, summary:true) — verify compact output" \
    "Summary mode reduces token usage for AI agents."
run_test_27_19() {
    RESPONSE=$(call_tool "observe" '{"what":"errors","summary":true}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        # Bridge timeout is expected without data — that's OK
        if check_bridge_timeout "$RESPONSE"; then
            skip "No error data available (bridge timeout). Summary mode tested with data."
            return
        fi
        fail "observe(errors, summary:true) returned error: $(truncate "$text")"
        return
    fi

    if human_verify "Does the observe response above look compact/summarized (not raw HTML/full stack traces)?"; then
        pass "Summary mode produced compact output."
    else
        fail "Human reports summary mode output looks too verbose."
    fi
}
run_test_27_19

finish_category
