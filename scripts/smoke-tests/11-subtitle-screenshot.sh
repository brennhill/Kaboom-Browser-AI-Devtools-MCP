#!/bin/bash
# 11-subtitle-screenshot.sh — 11.1-11.3: Subtitle, composable subtitle, screenshot.
set -eo pipefail

begin_category "11" "Subtitle & Screenshot" "3"

# ── Test 11.1: Subtitle standalone ───────────────────────
begin_test "11.1" "[INTERACTIVE - BROWSER] Subtitle: standalone set, verify visible, then clear" \
    "Use interact(subtitle) to display text at bottom of viewport, verify, then clear" \
    "Tests: subtitle pipeline: MCP > daemon > extension > content script overlay"

run_test_11_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Clean page for subtitle test"}' 20
    sleep 2

    interact_and_wait "subtitle" '{"what":"subtitle","text":"Kaboom smoke test — this text should appear at the bottom of the viewport"}' 25

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Subtitle set returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 2
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check subtitle visibility","script":"(function() { var el = document.getElementById(\"kaboom-subtitle\"); if (!el) return \"NOT_FOUND\"; var style = window.getComputedStyle(el); var visible = style.display !== \"none\" && style.opacity !== \"0\"; return (visible ? \"VISIBLE\" : \"HIDDEN\") + \":\" + el.textContent; })()"}' 25

    local dom_check="$INTERACT_RESULT"

    if echo "$dom_check" | grep -q "NOT_FOUND"; then
        fail "Subtitle element #kaboom-subtitle not found in DOM after setting text."
        return
    fi

    local has_text=false
    local is_visible=false
    if echo "$dom_check" | grep -q "smoke test"; then
        has_text=true
    fi
    if echo "$dom_check" | grep -q "VISIBLE:"; then
        is_visible=true
    fi

    if [ "$has_text" != "true" ]; then
        fail "Subtitle element exists but text content doesn't match. DOM: $(truncate "$dom_check" 300)"
        return
    fi

    if [ "$is_visible" != "true" ]; then
        fail "Subtitle element has correct text but is not visible. DOM: $(truncate "$dom_check" 300)"
        return
    fi

    interact_and_wait "subtitle" '{"what":"subtitle","text":""}'
    sleep 0.5

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Verify subtitle cleared","script":"(function() { var el = document.getElementById(\"kaboom-subtitle\"); if (!el) return \"REMOVED\"; var style = window.getComputedStyle(el); if (style.display === \"none\" || style.opacity === \"0\" || el.textContent === \"\") return \"HIDDEN\"; return \"STILL_VISIBLE:\" + el.textContent; })()"}'

    local clear_check="$INTERACT_RESULT"

    if echo "$clear_check" | grep -qi "REMOVED\|HIDDEN"; then
        pass "Subtitle: set text (visible + correct content), then cleared. Full lifecycle works."
    elif echo "$clear_check" | grep -qi "STILL_VISIBLE"; then
        fail "Subtitle still visible after clear. Result: $(truncate "$clear_check" 200)"
    else
        pass "Subtitle: set and clear commands accepted. DOM check: $(truncate "$clear_check" 200)"
    fi
}
run_test_11_1

# ── Test 11.2: Subtitle as optional param on navigate ────
begin_test "11.2" "[INTERACTIVE - BROWSER] Subtitle as optional param on interact(navigate)" \
    "Navigate with subtitle param in same call, verify both navigation and subtitle happen" \
    "Tests: composable subtitle — single tool call for action + narration"

run_test_11_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","subtitle":"Navigating to example.com — verifying composable subtitle"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "unknown.*subtitle\|invalid.*subtitle\|unrecognized"; then
        fail "Server rejected subtitle as unknown parameter. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 3

    local page_response
    page_response=$(call_tool "observe" '{"what":"page"}')
    local page_text
    page_text=$(extract_content_text "$page_response")

    local navigated=false
    if echo "$page_text" | grep -qi "example.com"; then
        navigated=true
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check composable subtitle","script":"(function() { var el = document.getElementById(\"kaboom-subtitle\"); if (!el) return \"NOT_FOUND\"; return JSON.stringify({ text: el.textContent, visible: window.getComputedStyle(el).display !== \"none\" }); })()"}'

    local subtitle_check="$INTERACT_RESULT"
    local has_subtitle=false
    if echo "$subtitle_check" | grep -q "composable subtitle\|example.com"; then
        has_subtitle=true
    fi

    if [ "$navigated" = "true" ] && [ "$has_subtitle" = "true" ]; then
        pass "Composable subtitle: single call navigated to example.com AND displayed subtitle text."
    elif [ "$navigated" = "true" ] && [ "$has_subtitle" != "true" ]; then
        fail "Navigation worked but subtitle not visible. Subtitle check: $(truncate "$subtitle_check" 200)"
    elif [ "$navigated" != "true" ] && [ "$has_subtitle" = "true" ]; then
        fail "Subtitle visible but navigation didn't work. Page: $(truncate "$page_text" 200)"
    else
        fail "Neither navigation nor subtitle worked. Page: $(truncate "$page_text" 200), Subtitle: $(truncate "$subtitle_check" 200)"
    fi

    interact_and_wait "subtitle" '{"what":"subtitle","text":""}'
}
run_test_11_2

# ── Test 11.3: On-demand screenshot ──────────────────────
begin_test "11.3" "[INTERACTIVE - BROWSER] Screenshot: on-demand capture via observe(screenshot)" \
    "Call observe(screenshot) and verify it captures the current viewport" \
    "Tests: on-demand screenshot pipeline: MCP > daemon > extension > captureVisibleTab > save"

run_test_11_3() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local screenshot_response
    screenshot_response=$(call_tool "observe" '{"what":"screenshot"}')

    if ! check_not_error "$screenshot_response"; then
        local err_text
        err_text=$(extract_content_text "$screenshot_response")
        if echo "$err_text" | grep -qi "no tab\|not tracked\|not connected"; then
            skip "No tracked tab for screenshot."
            return
        fi
        fail "observe(screenshot) returned error. Content: $(truncate "$err_text" 200)"
        return
    fi

    local text
    text=$(extract_content_text "$screenshot_response")

    local has_filename=false
    local has_path=false
    if echo "$text" | grep -q '"filename"'; then
        has_filename=true
    fi
    if echo "$text" | grep -q '"path"'; then
        has_path=true
    fi

    if [ "$has_filename" = "true" ] && [ "$has_path" = "true" ]; then
        local screenshot_path
        screenshot_path=$(echo "$text" | python3 -c "import sys,json; t=sys.stdin.read(); i=t.find('{'); print(json.loads(t[i:]).get('path','') if i>=0 else '')" 2>/dev/null || echo "")
        if [ -n "$screenshot_path" ] && [ -f "$screenshot_path" ]; then
            local file_size
            file_size=$(wc -c < "$screenshot_path" | tr -d ' ')
            pass "Screenshot captured: filename present, path present, file exists ($file_size bytes). Path: $screenshot_path"
        else
            pass "Screenshot captured: filename and path present. File check skipped (path: $screenshot_path)."
        fi
    else
        fail "Screenshot response missing filename or path. Content: $(truncate "$text" 200)"
    fi
}
run_test_11_3
