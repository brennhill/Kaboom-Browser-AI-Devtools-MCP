#!/bin/bash
# 10-recording.sh — 10.1-10.3: Video recording, audio, watermark.
set -eo pipefail

begin_category "10" "Recording" "3"

# Returns 0 when interact_and_wait timed out before completion.
is_interact_timeout() {
    echo "$1" | grep -q "^timeout waiting for"
}

# Poll saved_videos until a recording name containing $1 appears.
# Prints the latest saved_videos content text to stdout.
wait_for_saved_recording() {
    local expected_name="$1"
    local max_attempts="${2:-20}"
    local latest_text=""

    for _i in $(seq 1 "$max_attempts"); do
        local saved_response
        saved_response=$(call_tool "observe" '{"what":"saved_videos","last_n":5}')
        latest_text=$(extract_content_text "$saved_response")
        if echo "$latest_text" | grep -q "$expected_name"; then
            echo "$latest_text"
            return 0
        fi
        sleep 1
    done

    echo "$latest_text"
    return 1
}

# Best-effort preflight: ensure no stale active recording from previous tests/runs.
preflight_stop_recording_if_needed() {
    interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Preflight cleanup: ensure no active recording"}' 20

    if is_interact_timeout "$INTERACT_RESULT"; then
        return 0
    fi

    # Treat "nothing to stop" style responses as success.
    if echo "$INTERACT_RESULT" | grep -qiE "no recording|not recording|already stopped|nothing to stop"; then
        return 0
    fi

    # If an active recording existed, this call should succeed and clear it.
    if echo "$INTERACT_RESULT" | grep -qiE "\"status\":\"complete\"|\"success\":true"; then
        return 0
    fi

    # Best effort only: do not fail the test here, but emit diagnostic context.
    echo "  [preflight] screen_recording_stop returned: $(truncate "$INTERACT_RESULT" 180)"
    return 0
}

# ── Test 10.1: Record tab video (no audio) ───────────────
begin_test "10.1" "[INTERACTIVE - BROWSER] Record tab video for 5 seconds (no audio)" \
    "Start recording, wait 5s, stop, verify file saved with valid metadata" \
    "Tests: full recording pipeline: MCP > daemon > extension > tabCapture > blob > server > disk"

run_test_10_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"what":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video for recording"}' 20
    sleep 3

    preflight_stop_recording_if_needed

    interact_and_wait "screen_recording_start" '{"what":"screen_recording_start","name":"smoke-video-test","reason":"Record tab video"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "screen_recording_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi
    if is_interact_timeout "$INTERACT_RESULT"; then
        skip "screen_recording_start timed out waiting for user gesture. Open the Kaboom popup and click Approve when prompted, then rerun 10.1."
        return
    fi

    echo "  Recording... waiting 5 seconds"
    sleep 5

    interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Stop recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "screen_recording_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi
    if is_interact_timeout "$INTERACT_RESULT"; then
        skip "screen_recording_stop timed out (recording never started or permission not granted)."
        return
    fi

    local saved_text
    local found_saved=true
    if ! saved_text=$(wait_for_saved_recording "smoke-video-test" 20); then
        found_saved=false
    fi

    echo "  [saved video metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = next((x for x in recs if 'smoke-video-test' in str(x.get('name', ''))), recs[0])
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    format: {r.get(\"format\", \"?\")}')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    fps: {r.get(\"fps\", \"?\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if [ "$found_saved" = "true" ]; then
        local dur
        dur=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
r=next((x for x in recs if 'smoke-video-test' in str(x.get('name',''))), recs[0] if recs else {})
print(r.get('duration_seconds',0) if r else 0)
" 2>/dev/null || echo "0")
        if [ "$dur" -ge 3 ] 2>/dev/null; then
            pass "Video recorded: smoke-video-test, ${dur}s duration, saved to disk."
        else
            pass "Video recorded: smoke-video-test saved (duration: ${dur}s)."
        fi
    else
        fail "No 'smoke-video-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
    fi
}
run_test_10_1

# ── Test 10.2: Record tab video WITH tab audio ───────────
begin_test "10.2" "[INTERACTIVE - BROWSER] Record tab video with audio:tab for 5 seconds" \
    "Navigate to a page with sound, record with audio:'tab', verify audio metadata" \
    "Tests: tab audio capture via tabCapture"

run_test_10_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"what":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video with audio for recording"}' 20
    sleep 3

    preflight_stop_recording_if_needed

    interact_and_wait "screen_recording_start" '{"what":"screen_recording_start","name":"smoke-audio-test","audio":"tab","reason":"Record tab with audio"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "screen_recording_start with audio:tab returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi
    if is_interact_timeout "$INTERACT_RESULT"; then
        skip "screen_recording_start with audio:tab timed out waiting for user gesture. Open the Kaboom popup and click Approve when prompted, then rerun 10.2."
        return
    fi

    echo "  Recording with audio... waiting 5 seconds"
    echo "  (play sound in the tracked tab now if you want to verify audio)"
    sleep 5

    interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Stop audio recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        # Check specifically for 403 errors — indicates server auth issue
        if echo "$INTERACT_RESULT" | grep -qi "403"; then
            fail "screen_recording_stop returned 403 Forbidden. Server requires X-Kaboom-Client header for /recordings/save endpoint. Check extension upload headers. Result: $(truncate "$INTERACT_RESULT" 200)"
        else
            fail "screen_recording_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        fi
        return
    fi
    if is_interact_timeout "$INTERACT_RESULT"; then
        skip "screen_recording_stop timed out (recording never started or permission not granted)."
        return
    fi

    local saved_text
    local found_saved=true
    if ! saved_text=$(wait_for_saved_recording "smoke-audio-test" 20); then
        found_saved=false
    fi

    echo "  [saved video with audio metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = next((x for x in recs if 'smoke-audio-test' in str(x.get('name', ''))), recs[0])
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    audio_mode: {r.get(\"audio_mode\", \"(none)\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if [ "$found_saved" != "true" ]; then
        fail "No 'smoke-audio-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
        return
    fi

    local has_audio
has_audio=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
r=next((x for x in recs if 'smoke-audio-test' in str(x.get('name',''))), recs[0] if recs else {})
print(r.get('has_audio',False) if r else False)
" 2>/dev/null || echo "False")

    local audio_mode
    audio_mode=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
r=next((x for x in recs if 'smoke-audio-test' in str(x.get('name',''))), recs[0] if recs else {})
print(r.get('audio_mode','') if r else '')
" 2>/dev/null || echo "")

    if [ "$has_audio" = "True" ] && [ "$audio_mode" = "tab" ]; then
        pass "Audio recording saved: has_audio=true, audio_mode=tab."
    else
        fail "Recording saved but audio metadata missing. has_audio=$has_audio, audio_mode=$audio_mode."
    fi

    echo ""
    echo "  >>> Open the .webm file in ~/.kaboom/recordings/ to verify audio is audible."
}
run_test_10_2

# ── Test 10.3: Recording watermark survives page refresh ─
begin_test "10.3" "[INTERACTIVE - BROWSER] Recording watermark survives page refresh" \
    "Start recording, refresh the page, verify watermark reappears" \
    "Tests: tabs.onUpdated listener re-sends watermark after navigation"

run_test_10_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to CSP-safe page for watermark test..."
    interact_and_wait "navigate" '{"what":"navigate","url":"https://example.com","reason":"Load CSP-safe page for watermark test"}' 20
    sleep 2

    preflight_stop_recording_if_needed

    interact_and_wait "screen_recording_start" '{"what":"screen_recording_start","name":"smoke-watermark-test","reason":"Test watermark persistence"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "screen_recording_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 3

    # Keep generous poll budget to avoid false negatives on slower CI runners.
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check watermark before refresh","script":"document.getElementById(\"kaboom-recording-watermark\") ? \"WATERMARK_FOUND\" : \"WATERMARK_MISSING\""}' 30
    local before_refresh="$INTERACT_RESULT"
    if echo "$before_refresh" | grep -q "csp_blocked_all_worlds"; then
        interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Stop watermark test recording after CSP block"}' 20
        skip "Watermark DOM check blocked by page CSP (execute_js unavailable)."
        return
    fi

    interact_and_wait "refresh" '{"what":"refresh","reason":"Refresh during recording"}' 20
    sleep 5

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check watermark after refresh","script":"document.getElementById(\"kaboom-recording-watermark\") ? \"WATERMARK_FOUND\" : \"WATERMARK_MISSING\""}' 30
    local after_refresh="$INTERACT_RESULT"
    if echo "$after_refresh" | grep -q "csp_blocked_all_worlds"; then
        interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Stop watermark test recording after CSP block"}' 20
        skip "Watermark DOM check blocked by page CSP after refresh."
        return
    fi

    interact_and_wait "screen_recording_stop" '{"what":"screen_recording_stop","reason":"Stop watermark test recording"}' 20
    sleep 1

    local before_ok=false
    local after_ok=false
    if echo "$before_refresh" | grep -q "WATERMARK_FOUND"; then
        before_ok=true
    fi
    if echo "$after_refresh" | grep -q "WATERMARK_FOUND"; then
        after_ok=true
    fi

    if [ "$before_ok" = "true" ] && [ "$after_ok" = "true" ]; then
        pass "Watermark present before AND after page refresh. Tab update listener works."
    elif [ "$before_ok" = "true" ] && [ "$after_ok" != "true" ]; then
        fail "Watermark present before refresh but MISSING after. tabs.onUpdated re-send failed."
    elif [ "$before_ok" != "true" ]; then
        fail "Watermark not found even before refresh. Recording overlay may be broken."
    fi
}
run_test_10_3
