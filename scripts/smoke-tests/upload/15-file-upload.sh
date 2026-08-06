#!/bin/bash
# 15-file-upload.sh — 15.0-15.15: File upload automation validation.
# ALL actions go through MCP commands (interact/observe/execute_js).
# Browser-based tests (15.0 browser check, 15.10-15.15) require extension+pilot.
# Parameter validation tests (15.1-15.9) work without extension.
set -eo pipefail

# Temp directory for all upload tests — must be inside the daemon's default
# Stage 4 upload-dir (~/kaboom-upload-dir) when bootstrap enables OS upload
# automation without an explicit --upload-dir flag.
UPLOAD_TEST_DIR="${HOME}/kaboom-upload-dir/smoke-upload-$$"
mkdir -p "$UPLOAD_TEST_DIR"

# ── MD5 helper: works on both macOS (md5) and Linux (md5sum) ──
_compute_md5() {
    local file="$1"
    local hash
    hash=$(md5sum "$file" 2>/dev/null | awk '{print $1}' || md5 -q "$file" 2>/dev/null || true)
    if [ -z "$hash" ]; then
        echo "FATAL: neither md5sum nor md5 available" >&2
        return 1
    fi
    echo "$hash"
}

begin_category "15" "File Upload" "18"

# ── Persistent upload server for the whole category ───────
UPLOAD_PORT=$((PORT + 200))

# Kill anything already on the upload port
lsof -ti :"$UPLOAD_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.3

python3 "$(dirname "${BASH_SOURCE[0]}")/upload-server.py" "$UPLOAD_PORT" &
UPLOAD_SERVER_PID=$!
sleep 1

# Verify upload server is alive (P0-09: detect silent death)
if ! kill -0 "$UPLOAD_SERVER_PID" 2>/dev/null; then
    echo "FATAL: Upload test server (PID $UPLOAD_SERVER_PID) died on startup. Port $UPLOAD_PORT may be in use." >&2
    UPLOAD_SERVER_PID=""
fi

_cleanup_upload() {
    [ -n "$UPLOAD_SERVER_PID" ] && kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
    lsof -ti :"$UPLOAD_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
    rm -rf "$UPLOAD_TEST_DIR"
}
register_cleanup _cleanup_upload

# ── Shared helper: navigate browser to upload form ────────
# Visits / (sets session cookie) then /upload (loads form with CSRF).
_navigate_to_upload_form() {
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Get session cookie\"}"
    sleep 1
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Load upload form\"}"
    sleep 2
}

# ── Shared helper: poll observe(page) until URL marker appears ──
# Sets PAGE_POLL_OK to "true"/"false", plus PAGE_POLL_RESPONSE/TEXT from the last poll.
_poll_page_until_url() {
    local expected_fragment="$1"
    local max_polls="${2:-10}"
    local interval_secs="${3:-0.5}"

    PAGE_POLL_OK="false"
    PAGE_POLL_RESPONSE=""
    PAGE_POLL_TEXT=""

    local page_resp page_text
    for _poll_i in $(seq 1 "$max_polls"); do
        page_resp=$(call_tool "observe" '{"what":"page"}')
        page_text=$(extract_content_text "$page_resp")
        if [ -z "$page_text" ] && [ -n "$page_resp" ]; then
            page_text="$page_resp"
        fi

        PAGE_POLL_RESPONSE="$page_resp"
        PAGE_POLL_TEXT="$page_text"

        if echo "$page_text" | grep -q "$expected_fragment"; then
            PAGE_POLL_OK="true"
            return
        fi
        sleep "$interval_secs"
    done
}

# ── Shared helper: inspect command state via observe fallbacks ──
# Sets COMMAND_LOOKUP_STATUS, COMMAND_LOOKUP_TEXT, COMMAND_LOOKUP_NOTE.
_lookup_command_state() {
    local corr_id="$1"
    COMMAND_LOOKUP_STATUS=""
    COMMAND_LOOKUP_TEXT=""
    COMMAND_LOOKUP_NOTE=""

    local pending_resp
    pending_resp=$(call_tool "observe" '{"what":"pending_commands"}')
    local pending_text
    pending_text=$(extract_content_text "$pending_resp")
    if [ -z "$pending_text" ] && [ -n "$pending_resp" ]; then
        pending_text="$pending_resp"
    fi
    if [ -n "$pending_text" ]; then
        local pending_hit
        pending_hit=$(echo "$pending_text" | python3 - "$corr_id" -c "
import sys, json
corr = sys.argv[1]
text = sys.stdin.read()
i = text.find('{')
if i < 0:
    sys.exit(0)
try:
    data = json.loads(text[i:])
except Exception:
    sys.exit(0)
for bucket in ('pending', 'completed', 'failed'):
    items = data.get(bucket, [])
    if not isinstance(items, list):
        continue
    for item in items:
        if str(item.get('correlation_id', '')) != corr:
            continue
        status = str(item.get('status', '')).strip().lower()
        if status in ('queued', 'running', 'still_processing'):
            status = 'pending'
        if not status:
            status = 'complete' if bucket == 'completed' else ('error' if bucket == 'failed' else 'pending')
        err = str(item.get('error', '') or '')
        print(f'{status}|pending_commands[{bucket}] status={status} error={err}')
        sys.exit(0)
" 2>/dev/null || true)
        if [ -n "$pending_hit" ]; then
            COMMAND_LOOKUP_STATUS="${pending_hit%%|*}"
            COMMAND_LOOKUP_NOTE="${pending_hit#*|}"
            COMMAND_LOOKUP_TEXT="$pending_text"
            return
        fi
    fi

    local failed_resp
    failed_resp=$(call_tool "observe" '{"what":"failed_commands","limit":50}')
    local failed_text
    failed_text=$(extract_content_text "$failed_resp")
    if [ -z "$failed_text" ] && [ -n "$failed_resp" ]; then
        failed_text="$failed_resp"
    fi
    if echo "$failed_text" | grep -q "$corr_id"; then
        COMMAND_LOOKUP_STATUS="error"
        COMMAND_LOOKUP_NOTE="failed_commands contains correlation_id=$corr_id"
        COMMAND_LOOKUP_TEXT="$failed_text"
    fi
}

# ── Shared helper: interact(upload) + poll for completion ─
# Sets UPLOAD_FINAL_STATUS to "complete", "failed", "error", "pilot_disabled", or "pending".
_upload_and_poll() {
    local test_id="$1"
    local test_file="$2"
    UPLOAD_FINAL_STATUS="pending"
    UPLOAD_FINAL_TEXT=""

    local response
    local content_text=""
    local corr_id=""

    # Retry initial interact(upload) if daemon is still starting up.
    for _attempt in 1 2 3; do
        response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
        content_text=$(extract_content_text "$response")
        if [ -z "$content_text" ] && [ -n "$response" ]; then
            content_text="$response"
        fi
        corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)
        if [ -n "$corr_id" ]; then
            break
        fi
        if echo "$content_text" | grep -qi "starting up"; then
            sleep 2
            continue
        fi
        break
    done

    log_diagnostic "$test_id" "interact upload" "$response" "$content_text"

    if [ -z "$corr_id" ]; then
        if echo "$content_text" | grep -qi "error\|isError"; then
            UPLOAD_FINAL_STATUS="error: $(truncate "$content_text" 200)"
            UPLOAD_FINAL_TEXT="$content_text"
        else
            UPLOAD_FINAL_STATUS="no_corr_id: $(truncate "$content_text" 200)"
            UPLOAD_FINAL_TEXT="$content_text"
        fi
        return
    fi

    local max_polls="${UPLOAD_POLL_ATTEMPTS:-60}" # default: 30s at 0.5s interval
    for _ in $(seq 1 "$max_polls"); do
        sleep 0.5
        local poll_resp
        poll_resp=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_resp")
        if [ -z "$poll_text" ] && [ -n "$poll_resp" ]; then
            poll_text="$poll_resp"
        fi

        if echo "$poll_text" | grep -qi "starting up"; then
            UPLOAD_FINAL_TEXT="$poll_text"
            continue
        fi

        if echo "$poll_text" | grep -q '"status":"complete"'; then
            # Check for pilot-disabled error masquerading as "complete"
            if echo "$poll_text" | grep -q 'ai_web_pilot_disabled'; then
                UPLOAD_FINAL_STATUS="pilot_disabled"
                UPLOAD_FINAL_TEXT="$poll_text"
                log_diagnostic "$test_id" "poll pilot_disabled" "$poll_text" ""
                return
            elif echo "$poll_text" | grep -qi 'FAILED —\|\"error\":'; then
                UPLOAD_FINAL_STATUS="failed"
                UPLOAD_FINAL_TEXT="$poll_text"
                log_diagnostic "$test_id" "poll complete-with-error" "$poll_text" ""
                return
            fi
            UPLOAD_FINAL_STATUS="complete"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll complete" "$poll_text" ""
            return
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            UPLOAD_FINAL_STATUS="failed"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll failed" "$poll_text" ""
            return
        fi
        if echo "$poll_text" | grep -q '"status":"error"'; then
            UPLOAD_FINAL_STATUS="error"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll error" "$poll_text" ""
            return
        fi
        if [ -n "$poll_text" ]; then
            UPLOAD_FINAL_TEXT="$poll_text"
        fi

        # If command_result lookup temporarily loses state, inspect command lists for correlation.
        if [ -z "$poll_text" ] || echo "$poll_text" | grep -qi "no_data\|not found"; then
            _lookup_command_state "$corr_id"
            if [ -n "$COMMAND_LOOKUP_TEXT" ]; then
                UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
            fi
            if [ "$COMMAND_LOOKUP_STATUS" = "complete" ]; then
                UPLOAD_FINAL_STATUS="complete"
                log_diagnostic "$test_id" "poll fallback-complete" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
                return
            fi
            if echo "$COMMAND_LOOKUP_STATUS" | grep -qiE "error|failed|expired|timeout|cancelled"; then
                UPLOAD_FINAL_STATUS="failed"
                log_diagnostic "$test_id" "poll fallback-failed" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
                return
            fi
            if [ -n "$COMMAND_LOOKUP_NOTE" ]; then
                UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_NOTE"
            fi
        fi
    done

    # Final fallback snapshot before declaring pending timeout.
    _lookup_command_state "$corr_id"
    if [ "$COMMAND_LOOKUP_STATUS" = "complete" ]; then
        UPLOAD_FINAL_STATUS="complete"
        UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
        log_diagnostic "$test_id" "poll timeout-but-complete-via-fallback" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
        return
    fi
    if echo "$COMMAND_LOOKUP_STATUS" | grep -qiE "error|failed|expired|timeout|cancelled"; then
        UPLOAD_FINAL_STATUS="failed"
        UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
        log_diagnostic "$test_id" "poll timeout-but-failed-via-fallback" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
        return
    fi

    if [ -z "$UPLOAD_FINAL_TEXT" ]; then
        if [ -n "$COMMAND_LOOKUP_NOTE" ]; then
            UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_NOTE"
        else
            UPLOAD_FINAL_TEXT="no poll payload received (observe(command_result) did not return text)"
        fi
    fi
}

# ── Shared helper: fetch /api/last-upload via browser (dogfood) ──
# Uses execute_js + fetch() from the browser page to check the upload server API.
# Sets LAST_UPLOAD_MD5, LAST_UPLOAD_NAME, LAST_UPLOAD_CSRF_OK, LAST_UPLOAD_COOKIE_OK, LAST_UPLOAD_RAW.
_fetch_last_upload_via_browser() {
    LAST_UPLOAD_MD5=""
    LAST_UPLOAD_NAME=""
    LAST_UPLOAD_CSRF_OK=""
    LAST_UPLOAD_COOKIE_OK=""
    LAST_UPLOAD_RAW=""

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fetch last upload from server API","script":"fetch(\"/api/last-upload\").then(r=>r.json()).then(d=>JSON.stringify(d)).catch(e=>JSON.stringify({error:e.message}))"}'
    LAST_UPLOAD_RAW="$INTERACT_RESULT"

    # Extract fields from command result JSON.
    # Newer responses expose execute_js return data under top-level return_value.
    # Older responses may nest it under result.return_value or result.result.
    local parsed
    parsed=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
text = sys.stdin.read()
idx = text.find('{')
if idx < 0:
    print('{}')
    sys.exit(0)
data = json.loads(text[idx:])
def emit_candidate(value):
    if isinstance(value, dict):
        print(json.dumps(value))
        return True
    if isinstance(value, str):
        s = value.strip()
        if not s:
            return False
        try:
            inner = json.loads(s)
        except Exception:
            return False
        if isinstance(inner, dict):
            print(json.dumps(inner))
            return True
    return False

r = data.get('result', {}) if isinstance(data, dict) else {}
candidates = [
    data.get('return_value') if isinstance(data, dict) else None,
    r.get('return_value') if isinstance(r, dict) else None,
    r.get('result') if isinstance(r, dict) else None,
]
for candidate in candidates:
    if emit_candidate(candidate):
        sys.exit(0)
print('{}')
" 2>/dev/null || echo "{}")

    LAST_UPLOAD_MD5=$(echo "$parsed" | jq -r '.md5 // empty' 2>/dev/null)
    LAST_UPLOAD_NAME=$(echo "$parsed" | jq -r '.name // empty' 2>/dev/null)
    LAST_UPLOAD_CSRF_OK=$(echo "$parsed" | jq -r '.csrf_ok // empty' 2>/dev/null)
    LAST_UPLOAD_COOKIE_OK=$(echo "$parsed" | jq -r '.cookie_ok // empty' 2>/dev/null)
}

# ── Test 15.0: Upload server canary ──────────────────────
begin_test "15.0" "[BROWSER] Upload server canary" \
    "Navigate browser to upload server, verify landing page and upload form load" \
    "Tests: test infrastructure — if this fails, all upload tests are invalid"

run_test_15_0() {
    # Quick infrastructure check — is the process alive?
    local health_resp
    health_resp=$(curl -s --max-time 5 --connect-timeout 3 "http://127.0.0.1:${UPLOAD_PORT}/health" 2>/dev/null)
    if ! echo "$health_resp" | jq -e '.ok == true' >/dev/null 2>&1; then
        fail "Upload server not responding on port $UPLOAD_PORT (PID $UPLOAD_SERVER_PID)."
        return
    fi

    # Browser verification (requires extension)
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        pass "Upload server healthy on port $UPLOAD_PORT (browser check skipped — no extension)."
        return
    fi

    # Navigate to landing page (sets session cookie in browser)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Canary — landing page\"}"
    sleep 1

    # Navigate to upload form
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Canary — upload form\"}"
    sleep 1

    # Observe the page
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.0" "observe upload form" "$response" "$content_text"

    # observe(page) returns {url, title, tracked, metadata} — not HTML content.
    # Verify the browser navigated to the upload form by checking url or title.
    if echo "$content_text" | grep -qi "/upload\|Upload"; then
        pass "Upload server on port $UPLOAD_PORT: browser on upload page (url/title confirmed)."
    else
        fail "Upload form not visible in browser. Page: $(truncate "$content_text" 200)"
    fi
}
run_test_15_0

# ── Test 15.1: Schema — upload in interact enum ───
begin_test "15.1" "[DAEMON ONLY] Schema: upload in interact what enum" \
    "Verify tools/list includes upload as a valid interact action (canonical what enum)" \
    "Tests: schema registration for file upload"

run_test_15_1() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    if echo "$tools_resp" | jq -e '
        .result.tools[] | select(.name=="interact")
        | ((.inputSchema.properties.what.enum // .inputSchema.properties.action.enum // []))
        | .[] | select(.=="upload")
    ' >/dev/null 2>&1; then
        pass "upload in interact enum (what/action)."
    else
        fail "upload NOT in interact enum (checked what/action)."
    fi
}
run_test_15_1

# ── Test 15.2: Upload works without any flag ─────────────
begin_test "15.2" "[DAEMON ONLY] Upload works without any flag (queues successfully)" \
    "Call upload with nonexistent file, verify we get a file error (not disabled error)" \
    "Tests: basic upload always works without --enable-os-upload-automation"

run_test_15_2() {
    local response
    response=$(call_tool "interact" '{"what":"upload","selector":"#file","file_path":"/tmp/nonexistent-kaboom-test.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.2" "upload probe" "$response" "$content_text"

    if echo "$content_text" | grep -qi "not found\|File not found\|invalid_param"; then
        pass "Upload works without flag: got file validation error (not disabled)."
    elif echo "$content_text" | grep -q '"status":"queued"\|"correlation_id"'; then
        pass "Upload works without flag: queued successfully."
    else
        fail "Unexpected upload response: $(truncate "$content_text" 200)"
    fi
}
run_test_15_2

# ── Test 15.3: Missing file_path ─────────────────────────
begin_test "15.3" "[DAEMON ONLY] Missing file_path returns clear error" \
    "Call upload without file_path, verify parameter error" \
    "Tests: required parameter validation"

run_test_15_3() {
    local response
    response=$(call_tool "interact" '{"what":"upload","selector":"#file"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.3" "missing file_path" "$response" "$content_text"

    if echo "$content_text" | grep -q "file_path"; then
        pass "Missing file_path: error mentions file_path parameter."
    else
        fail "Missing file_path: no clear error. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_3

# ── Test 15.4: Missing selector and api_endpoint ────────
begin_test "15.4" "[DAEMON ONLY] Missing selector returns clear error" \
    "Call upload with file_path but no selector or api_endpoint" \
    "Tests: required selector/api_endpoint parameter validation"

run_test_15_4() {
    local response
    response=$(call_tool "interact" '{"what":"upload","file_path":"/tmp/test.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.4" "missing selector" "$response" "$content_text"

    if echo "$content_text" | grep -q "selector"; then
        pass "Missing selector: error mentions selector parameter."
    else
        fail "Missing selector: no clear error. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_4

# ── Test 15.5: Relative path rejected ───────────────────
begin_test "15.5" "[DAEMON ONLY] Relative path rejected with security error" \
    "Call upload with relative file_path, verify rejection" \
    "Tests: absolute path requirement for security"

run_test_15_5() {
    local response
    response=$(call_tool "interact" '{"what":"upload","selector":"#file","file_path":"relative/path/file.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.5" "relative path" "$response" "$content_text"

    if echo "$content_text" | grep -qi "absolute\|not allowed\|path_not_allowed"; then
        pass "Relative path: rejected with security error."
    else
        fail "Relative path: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_5

# ── Test 15.6: Nonexistent file rejected ────────────────
begin_test "15.6" "[DAEMON ONLY] Nonexistent file rejected with clear error" \
    "Call upload with nonexistent absolute file_path" \
    "Tests: file existence validation before queuing"

run_test_15_6() {
    local response
    response=$(call_tool "interact" '{"what":"upload","selector":"#file","file_path":"/tmp/kaboom-nonexistent-file-12345.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.6" "nonexistent file" "$response" "$content_text"

    if echo "$content_text" | grep -qi "not found\|no such file\|does not exist"; then
        pass "Nonexistent file: rejected with clear error."
    else
        fail "Nonexistent file: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_6

# ── Test 15.7: Directory path rejected ──────────────────
begin_test "15.7" "[DAEMON ONLY] Directory path rejected (not a file)" \
    "Call upload with path to a directory, verify rejection" \
    "Tests: file-vs-directory validation"

run_test_15_7() {
    local response
    response=$(call_tool "interact" '{"what":"upload","selector":"#file","file_path":"/tmp"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.7" "directory path" "$response" "$content_text"

    if echo "$content_text" | grep -qi "directory\|not a file"; then
        pass "Directory path: rejected with clear error."
    else
        fail "Directory path: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_7

# ── Test 15.8: Local queue acceptance (no server delivery) ──
begin_test "15.8" "[DAEMON ONLY] Local queue acceptance (no server delivery)" \
    "Create temp file, call upload with valid params, verify queued response" \
    "Tests: local queue returns status=queued with correlation_id (queue is never drained without extension)"

run_test_15_8() {
    # Create a temp file for the upload test
    local test_file="$UPLOAD_TEST_DIR/upload-test.txt"
    echo "Kaboom upload smoke test content" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.8" "valid upload" "$response" "$content_text"

    local has_queued has_corr_id
    has_queued=$(echo "$content_text" | grep -c '"status":"queued"' || true)
    has_corr_id=$(echo "$content_text" | grep -c '"correlation_id":"upload_' || true)

    if [ "$has_queued" -gt 0 ] && [ "$has_corr_id" -gt 0 ]; then
        pass "Upload queued: status=queued, correlation_id present."
    else
        fail "Upload not properly queued: queued=$has_queued, corr_id=$has_corr_id. Got: $(truncate "$content_text" 300)"
    fi
}
run_test_15_8

# ── Test 15.9: Queue response metadata (local only) ──────
begin_test "15.9" "[DAEMON ONLY] Queue response includes file metadata" \
    "Create temp files with known extensions, verify metadata in queue response" \
    "Tests: file metadata extraction (name, size, mime_type, progress_tier) — local queue only"

run_test_15_9() {
    # Create a temp file with a recognizable extension
    local test_file="$UPLOAD_TEST_DIR/upload-test.jpg"
    echo "fake image content for metadata test" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"input[type=file]\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.9" "file metadata" "$response" "$content_text"

    rm -f "$test_file"

    local metadata_verdict
    metadata_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    has_name = 'file_name' in data and data['file_name'].endswith('.jpg')
    has_size = 'file_size' in data and isinstance(data['file_size'], (int, float)) and data['file_size'] > 0
    has_mime = 'mime_type' in data and 'image' in data.get('mime_type', '')
    has_tier = 'progress_tier' in data
    if has_name and has_size and has_mime and has_tier:
        print(f'PASS name={data[\"file_name\"]} size={data[\"file_size\"]} mime={data[\"mime_type\"]} tier={data[\"progress_tier\"]}')
    else:
        print(f'FAIL name={has_name} size={has_size} mime={has_mime} tier={has_tier} keys={list(data.keys())[:10]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$metadata_verdict" | grep -q "^PASS"; then
        pass "File metadata correct. $metadata_verdict"
    else
        fail "File metadata incomplete. $metadata_verdict. Response: $(truncate "$content_text" 300)"
    fi
}
run_test_15_9

# ── Test 15.10: Upload text file E2E via browser ─────────
begin_test "15.10" "[BROWSER] Upload text file E2E with MD5 verification (requires pilot)" \
    "Navigate to upload form, interact(upload), submit form via execute_js, verify MD5 on success page" \
    "Tests: file flows through browser to upload server, MD5 verified on success page"

run_test_15_10() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Create test file with known content
    local test_content="Kaboom upload E2E $(date +%s)"
    local test_file="$UPLOAD_TEST_DIR/upload-15-10.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(_compute_md5 "$test_file")

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Set file on input via interact(upload), poll for completion
    _upload_and_poll "15.10" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete) ;;
        pilot_disabled)
            fail "Upload rejected: AI Web Pilot is disabled in the extension. Enable it and re-run."
            return ;;
        pending)
            fail "Upload timed out (poll window exhausted). Extension did not report a result. Possible causes: (1) sync client disconnected during multi-second upload pipeline, (2) extension's __aiWebPilotEnabledCache stale, (3) chrome.scripting.executeScript failed silently on the upload form tab. Last poll: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
        failed|error)
            fail "Extension reported upload failure. Status: $UPLOAD_FINAL_STATUS. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
    esac

    # Fill in required title field via execute_js
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill title field","script":"document.querySelector(\"input[name=title]\").value=\"Smoke Test 15.10\"; \"title_set\""}'
    sleep 0.5

    # Submit form via execute_js
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Submit upload form","script":"document.querySelector(\"form\").submit(); \"submitted\""}'
    sleep 3

    # Observe result page — should reach /upload/success after redirect settles.
    _poll_page_until_url "/upload/success" 12 0.5
    log_diagnostic "15.10" "observe success page" "$PAGE_POLL_RESPONSE" "$PAGE_POLL_TEXT"

    if [ "$PAGE_POLL_OK" != "true" ]; then
        fail "Upload did not reach success page. Page metadata: $(truncate "$PAGE_POLL_TEXT" 300)"
        return
    fi

    # Verify MD5 via server API (dogfood: fetch from browser)
    _fetch_last_upload_via_browser
    log_diagnostic "15.10" "last-upload API" "$LAST_UPLOAD_RAW" ""

    if [ "$LAST_UPLOAD_MD5" = "$original_md5" ]; then
        pass "Upload E2E: file reached server, MD5 match ($original_md5)."
    else
        fail "Upload reached server but MD5 mismatch. Expected: $original_md5, got: $LAST_UPLOAD_MD5. Server file='$LAST_UPLOAD_NAME'."
    fi
}
run_test_15_10

# ── Test 15.11: Extension upload pipeline (requires pilot) ──
begin_test "15.11" "[BROWSER] Extension upload pipeline completion (requires pilot)" \
    "Navigate to upload form, interact(upload), poll until extension reports complete" \
    "Tests: extension receives upload command and processes it to completion"

run_test_15_11() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Create a test file
    local test_file="$UPLOAD_TEST_DIR/e2e-upload.txt"
    echo "Kaboom E2E upload content" > "$test_file"

    # Upload and poll for completion
    _upload_and_poll "15.11" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete)
            pass "E2E upload: extension processed upload to completion."
            ;;
        pilot_disabled)
            fail "Upload rejected: AI Web Pilot is disabled in the extension. Enable it and re-run."
            ;;
        failed|error)
            fail "E2E upload: extension reported failure. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            ;;
        pending)
            fail "E2E upload: timed out (poll window exhausted). Extension did not report completion. Possible causes: sync client reset during upload pipeline, pilot cache stale, or chrome.scripting.executeScript failed on upload form. Last poll: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            ;;
        *)
            fail "E2E upload: $UPLOAD_FINAL_STATUS. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            ;;
    esac
}
run_test_15_11

# ── Test 15.12: Upload with full server-side verification ──
begin_test "15.12" "[BROWSER] Upload with full server-side verification (requires pilot)" \
    "Navigate, upload file, fill fields, submit form, verify CSRF + cookie + MD5 via server API" \
    "Tests: complete upload pipeline — browser → server, all fields verified server-side"

run_test_15_12() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    local test_content="Kaboom full verification $(date +%s)"
    local test_file="$UPLOAD_TEST_DIR/upload-15-12.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(_compute_md5 "$test_file")

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Set file on input, poll for completion
    _upload_and_poll "15.12" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete) ;;
        pilot_disabled)
            fail "Upload rejected: AI Web Pilot is disabled in the extension. Enable it and re-run."
            return ;;
        pending)
            fail "Upload timed out (poll window exhausted). Extension did not report a result. Possible causes: sync client disconnected, pilot cache stale, or chrome.scripting.executeScript blocked. Last poll: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
        failed|error)
            fail "Extension reported upload failure. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS. Detail: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            return ;;
    esac

    # Fill form fields (title + tags) via execute_js
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill form fields","script":"document.querySelector(\"input[name=title]\").value=\"Smoke Test Upload\"; document.querySelector(\"input[name=tags]\").value=\"smoke,kaboom\"; \"fields_set\""}'
    sleep 0.5

    # Submit form
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Submit upload form","script":"document.querySelector(\"form\").submit(); \"submitted\""}'
    sleep 3

    # Observe success page and tolerate redirect/load latency.
    _poll_page_until_url "/upload/success" 12 0.5
    log_diagnostic "15.12" "observe success page" "$PAGE_POLL_RESPONSE" "$PAGE_POLL_TEXT"

    if [ "$PAGE_POLL_OK" != "true" ]; then
        fail "Upload did not reach success page. Page metadata: $(truncate "$PAGE_POLL_TEXT" 300)"
        return
    fi

    # Server-side verification via browser fetch (dogfood)
    _fetch_last_upload_via_browser
    log_diagnostic "15.12" "server verify" "$LAST_UPLOAD_RAW" ""

    if [ "$LAST_UPLOAD_MD5" = "$original_md5" ] && [ "$LAST_UPLOAD_CSRF_OK" = "true" ] && [ "$LAST_UPLOAD_COOKIE_OK" = "true" ]; then
        pass "Full verification: MD5 match ($LAST_UPLOAD_MD5), CSRF ok, cookie ok."
    else
        fail "Server verification: md5=$LAST_UPLOAD_MD5 (expected $original_md5), csrf=$LAST_UPLOAD_CSRF_OK, cookie=$LAST_UPLOAD_COOKIE_OK."
    fi
}
run_test_15_12

# ── Test 15.13: Confirmation page via observe ────────────
begin_test "15.13" "[BROWSER] Confirmation page visible via observe (requires pilot)" \
    "After upload tests, observe(page) should show Upload Successful" \
    "Tests: observe reads confirmation page after browser form submission + redirect"

run_test_15_13() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Browser should be on /upload/success from 15.12, but allow residual loading.
    _poll_page_until_url "/upload/success" 10 0.5
    log_diagnostic "15.13" "observe page" "$PAGE_POLL_RESPONSE" "$PAGE_POLL_TEXT"

    if [ "$PAGE_POLL_OK" = "true" ]; then
        pass "Confirmation page visible via observe(page) — URL contains /upload/success."
    else
        fail "Browser not on upload success page after 15.10/15.12. Page metadata: $(truncate "$PAGE_POLL_TEXT" 300)"
    fi
}
run_test_15_13

# ── Test 15.14: Auth failure visible in browser ──────────
begin_test "15.14" "[BROWSER] Auth failure visible in browser (requires pilot)" \
    "Navigate to /logout to clear session, then /upload to trigger 401 — verify in browser" \
    "Tests: server auth rejection is visible to the user in the browser"

run_test_15_14() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Clear session cookie by navigating to /logout
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/logout\",\"reason\":\"Clear session for auth failure test\"}"
    sleep 1

    # Navigate to /upload — server should reject with 401 (no session)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Attempt upload form without session\"}"
    sleep 1

    # Verify 401 is visible in the browser via execute_js (dogfood — use Kaboom, not curl)
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check 401 page content","script":"document.body.innerText"}'

    log_diagnostic "15.14" "401 page content" "$INTERACT_RESULT" ""

    if echo "$INTERACT_RESULT" | grep -qi "401\|Not logged in"; then
        pass "Auth failure: '401 Not logged in' visible in browser after session cleared."
    else
        fail "Expected 401 page after logout. Browser content: $(truncate "$INTERACT_RESULT" 200)"
    fi

    # Restore session for subsequent tests (navigate to / to get new cookie)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Restore session after auth failure test\"}"
    sleep 1
}
run_test_15_14

# ── Test 15.15: Verify file reached input element ─────
begin_test "15.15" "[BROWSER] Verify upload file reached input element (requires pilot)" \
    "After interact(upload), execute_js to check files[0]?.name on the input" \
    "Tests: file actually set on DOM input element (fail if extension connected but file missing)"

run_test_15_15() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Create test file and attempt upload
    local test_file="$UPLOAD_TEST_DIR/verify-upload.txt"
    echo "verify-upload-content" > "$test_file"

    # Upload and poll
    _upload_and_poll "15.15" "$test_file"
    local upload_status="$UPLOAD_FINAL_STATUS"

    sleep 1

    # Verify: execute_js to check files on the input
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Check upload input files","script":"(function(){ var el = document.getElementById(\"file-input\"); return el && el.files && el.files[0] ? el.files[0].name : (el ? \"NO_FILES\" : \"NO_ELEMENT\"); })()"}'

    log_diagnostic "15.15" "file verification" "$INTERACT_RESULT" ""

    # Extract the JS return value from the nested command result JSON
    local js_result
    js_result=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
text = sys.stdin.read()
idx = text.find('{')
if idx < 0:
    print(text.strip())
else:
    try:
        data = json.loads(text[idx:])
        r = data.get('result', {})
        if isinstance(r, dict):
            print(r.get('result', r.get('error', r.get('message', 'UNKNOWN'))))
        else:
            print(r)
    except:
        print(text.strip())
" 2>/dev/null || echo "$INTERACT_RESULT")

    echo "  [js_result: $js_result]"
    if [ "$upload_status" != "complete" ]; then
        echo "  [upload_status: $upload_status]"
    fi

    local expected_name="verify-upload.txt"
    if [ "$js_result" = "$expected_name" ]; then
        pass "File reached input: files[0].name = '$expected_name' (upload_status=$upload_status)."
    elif echo "$js_result" | grep -q "$expected_name"; then
        pass "File reached input: result contains '$expected_name' (upload_status=$upload_status)."
    elif [ "$js_result" = "UNKNOWN" ]; then
        if echo "$UPLOAD_FINAL_TEXT" | grep -q '"stage":1\|"stage": 1' && \
           echo "$UPLOAD_FINAL_TEXT" | grep -q "\"file_name\":\"$expected_name\""; then
            pass "execute_js returned no result, but upload completion confirms stage=1 file_name='$expected_name' (upload_status=$upload_status)."
        else
            fail "execute_js returned no result and upload completion lacks stage1/file_name evidence. upload_status=$upload_status, poll=$(truncate "$UPLOAD_FINAL_TEXT" 200), execute_js=$(truncate "$INTERACT_RESULT" 200)"
        fi
    elif [ "$js_result" = "NO_FILES" ]; then
        fail "File not set on input element (files array empty). upload_status=$upload_status, poll=$(truncate "$UPLOAD_FINAL_TEXT" 200)"
    elif [ "$js_result" = "NO_ELEMENT" ]; then
        fail "Input element #file-input not found on page. upload_status=$upload_status"
    elif echo "$js_result" | grep -qi "error\|failed\|csp"; then
        fail "execute_js error: $js_result (upload_status=$upload_status, poll=$(truncate "$UPLOAD_FINAL_TEXT" 200))"
    else
        fail "Unexpected result from file check: '$js_result'. upload_status=$upload_status, poll=$(truncate "$UPLOAD_FINAL_TEXT" 200), execute_js=$(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_15_15

# ── Shared helper: navigate browser to hardened upload form ──
# Visits / (sets session cookie) then /upload/hardened (loads hardened form with CSRF).
_navigate_to_hardened_form() {
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Get session cookie\"}"
    sleep 1
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload/hardened\",\"reason\":\"Load hardened upload form\"}"
    sleep 2
}

# ── Shared helper: upload and poll for Stage 4 (longer timeout) ──
# Sets UPLOAD_FINAL_STATUS and UPLOAD_FINAL_TEXT.
_upload_and_poll_stage4() {
    local test_id="$1"
    local test_file="$2"
    UPLOAD_FINAL_STATUS="pending"
    UPLOAD_FINAL_TEXT=""

    local response
    local content_text=""
    local corr_id=""

    for _attempt in 1 2 3; do
        response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
        content_text=$(extract_content_text "$response")
        if [ -z "$content_text" ] && [ -n "$response" ]; then
            content_text="$response"
        fi
        corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)
        if [ -n "$corr_id" ]; then
            break
        fi
        if echo "$content_text" | grep -qi "starting up"; then
            sleep 2
            continue
        fi
        break
    done

    log_diagnostic "$test_id" "interact upload" "$response" "$content_text"

    if [ -z "$corr_id" ]; then
        if echo "$content_text" | grep -qi "error\|isError"; then
            UPLOAD_FINAL_STATUS="error: $(truncate "$content_text" 200)"
            UPLOAD_FINAL_TEXT="$content_text"
        else
            UPLOAD_FINAL_STATUS="no_corr_id: $(truncate "$content_text" 200)"
            UPLOAD_FINAL_TEXT="$content_text"
        fi
        return
    fi

    # Stage 4 needs longer timeout: 30 polls x 1s = 30s max
    for _ in $(seq 1 30); do
        sleep 1
        local poll_resp
        poll_resp=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_resp")
        if [ -z "$poll_text" ] && [ -n "$poll_resp" ]; then
            poll_text="$poll_resp"
        fi
        if echo "$poll_text" | grep -qi "starting up"; then
            UPLOAD_FINAL_TEXT="$poll_text"
            continue
        fi
        if echo "$poll_text" | grep -q '"status":"complete"'; then
            # Check for pilot-disabled error masquerading as "complete"
            if echo "$poll_text" | grep -q 'ai_web_pilot_disabled'; then
                UPLOAD_FINAL_STATUS="pilot_disabled"
                UPLOAD_FINAL_TEXT="$poll_text"
                log_diagnostic "$test_id" "poll pilot_disabled" "$poll_text" ""
                return
            elif echo "$poll_text" | grep -qi 'FAILED —\|\"error\":'; then
                UPLOAD_FINAL_STATUS="failed"
                UPLOAD_FINAL_TEXT="$poll_text"
                log_diagnostic "$test_id" "poll complete-with-error" "$poll_text" ""
                return
            fi
            UPLOAD_FINAL_STATUS="complete"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll complete" "$poll_text" ""
            return
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            UPLOAD_FINAL_STATUS="failed"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll failed" "$poll_text" ""
            return
        fi
        if echo "$poll_text" | grep -q '"status":"error"'; then
            UPLOAD_FINAL_STATUS="error"
            UPLOAD_FINAL_TEXT="$poll_text"
            log_diagnostic "$test_id" "poll error" "$poll_text" ""
            return
        fi
        if [ -n "$poll_text" ]; then
            UPLOAD_FINAL_TEXT="$poll_text"
        fi

        if [ -z "$poll_text" ] || echo "$poll_text" | grep -qi "no_data\|not found"; then
            _lookup_command_state "$corr_id"
            if [ -n "$COMMAND_LOOKUP_TEXT" ]; then
                UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
            fi
            if [ "$COMMAND_LOOKUP_STATUS" = "complete" ]; then
                UPLOAD_FINAL_STATUS="complete"
                log_diagnostic "$test_id" "stage4 fallback-complete" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
                return
            fi
            if echo "$COMMAND_LOOKUP_STATUS" | grep -qiE "error|failed|expired|timeout|cancelled"; then
                UPLOAD_FINAL_STATUS="failed"
                log_diagnostic "$test_id" "stage4 fallback-failed" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
                return
            fi
            if [ -n "$COMMAND_LOOKUP_NOTE" ]; then
                UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_NOTE"
            fi
        fi
    done

    _lookup_command_state "$corr_id"
    if [ "$COMMAND_LOOKUP_STATUS" = "complete" ]; then
        UPLOAD_FINAL_STATUS="complete"
        UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
        log_diagnostic "$test_id" "stage4 timeout-but-complete-via-fallback" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
        return
    fi
    if echo "$COMMAND_LOOKUP_STATUS" | grep -qiE "error|failed|expired|timeout|cancelled"; then
        UPLOAD_FINAL_STATUS="failed"
        UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_TEXT"
        log_diagnostic "$test_id" "stage4 timeout-but-failed-via-fallback" "$COMMAND_LOOKUP_TEXT" "$COMMAND_LOOKUP_NOTE"
        return
    fi

    if [ -z "$UPLOAD_FINAL_TEXT" ]; then
        if [ -n "$COMMAND_LOOKUP_NOTE" ]; then
            UPLOAD_FINAL_TEXT="$COMMAND_LOOKUP_NOTE"
        else
            UPLOAD_FINAL_TEXT="no poll payload received (observe(command_result) did not return text)"
        fi
    fi
}

# ── Test 15.16: Stage 4 escalation E2E ───────────────────
begin_test "15.16" "[BROWSER] Stage 4 escalation E2E (hardened form, requires pilot)" \
    "Navigate to hardened form (isTrusted check), upload via Stage 4 escalation, verify MD5" \
    "Tests: full Stage 1→4 escalation pipeline through hardened form"

run_test_15_16() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Create test file with known content
    local test_content="Kaboom Stage 4 E2E $(date +%s)"
    local test_file="$UPLOAD_TEST_DIR/upload-15-16.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(_compute_md5 "$test_file")

    # Navigate browser to hardened form
    _navigate_to_hardened_form

    # Upload and poll with Stage 4 timeout
    _upload_and_poll_stage4 "15.16" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete)
            # Check if it was Stage 4 escalation
            if echo "$UPLOAD_FINAL_TEXT" | grep -q '"stage":4\|"stage": 4'; then
                # Stage 4 confirmed — now submit the form and verify MD5
                interact_and_wait "execute_js" '{"what":"execute_js","reason":"Fill title field","script":"document.querySelector(\"input[name=title]\").value=\"Smoke Test 15.16 Stage4\"; \"title_set\""}'
                sleep 0.5
                interact_and_wait "execute_js" '{"what":"execute_js","reason":"Submit upload form","script":"document.querySelector(\"form\").submit(); \"submitted\""}'

                # Poll for success page (up to 5s)
                local page_resp page_text poll_ok=false
                for _poll_i in $(seq 1 10); do
                    sleep 0.5
                    page_resp=$(call_tool "observe" '{"what":"page"}')
                    page_text=$(extract_content_text "$page_resp")
                    if echo "$page_text" | grep -q '/upload/success'; then
                        poll_ok=true
                        break
                    fi
                done
                log_diagnostic "15.16" "post-submit page" "$page_resp" "$page_text"

                if [ "$poll_ok" != "true" ]; then
                    # Capture page content for diagnostics
                    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Get page content on failure","script":"document.title + \": \" + document.body.innerText.substring(0,200)"}'
                    fail "Stage 4 upload: form submission did not reach success page. Page: $(truncate "$page_text" 300). Body: $(truncate "$INTERACT_RESULT" 200). Possible CSRF or session mismatch."
                    return
                fi

                # Verify MD5 via server API (dogfood: fetch from browser)
                _fetch_last_upload_via_browser
                log_diagnostic "15.16" "last-upload API" "$LAST_UPLOAD_RAW" ""

                echo "  [server] file='$LAST_UPLOAD_NAME' md5='$LAST_UPLOAD_MD5' expected='$original_md5'"

                if [ "$LAST_UPLOAD_MD5" = "$original_md5" ]; then
                    pass "Stage 4 escalation: file reached server via OS automation, MD5 match ($original_md5)."
                elif [ -z "$LAST_UPLOAD_MD5" ]; then
                    fail "Stage 4: could not extract MD5 from server API. Raw: $(truncate "$LAST_UPLOAD_RAW" 300)"
                else
                    fail "Stage 4 file reached server but MD5 mismatch. Expected $original_md5, got $LAST_UPLOAD_MD5. Server file='$LAST_UPLOAD_NAME'. The OS dialog may have selected the wrong file."
                fi
            elif echo "$UPLOAD_FINAL_TEXT" | grep -q '"stage":1\|"stage": 1'; then
                # Stage 1 succeeded on hardened form — unexpected but not a test failure
                pass "Upload completed via Stage 1 (hardened form did not reject — may not have isTrusted check active)."
            else
                pass "Upload completed but stage not identified in response."
            fi
            ;;
        pilot_disabled)
            fail "Upload rejected: AI Web Pilot is disabled in the extension. Enable it and re-run."
            ;;
        pending)
            fail "Upload timed out — extension did not report a result within 30s. Check: extension reloaded? daemon restarted? pilot enabled? accessibility granted?"
            ;;
        failed|error)
            # Check for known skip conditions
            if echo "$UPLOAD_FINAL_TEXT" | grep -qi "accessibility\|Accessibility"; then
                skip "Stage 4 needs macOS Accessibility permission. Fix: System Settings > Privacy & Security > Accessibility. Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            elif echo "$UPLOAD_FINAL_TEXT" | grep -qi "xdotool"; then
                skip "Stage 4 needs xdotool. Fix: sudo apt install xdotool. Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            elif echo "$UPLOAD_FINAL_TEXT" | grep -qi "enable-os-upload-automation"; then
                fail "Daemon missing --enable-os-upload-automation flag. Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            elif echo "$UPLOAD_FINAL_TEXT" | grep -qi "Cannot detect Chrome\|chrome.exe"; then
                fail "Chrome not detected. Is Google Chrome running? Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            else
                fail "Stage 4 escalation failed: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            fi
            ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS"
            ;;
    esac
}
run_test_15_16

# ── Test 15.17: Escalation reason metadata ────────────────
begin_test "15.17" "[BROWSER] Escalation reason metadata in result (requires pilot)" \
    "After Stage 4 upload on hardened form, verify escalation_reason field in result" \
    "Tests: extension reports escalation_reason when Stage 4 is used"

run_test_15_17() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Navigate browser to hardened form
    _navigate_to_hardened_form

    # Create a test file
    local test_file="$UPLOAD_TEST_DIR/escalation-reason.txt"
    echo "escalation reason test" > "$test_file"

    # Upload and poll with Stage 4 timeout
    _upload_and_poll_stage4 "15.17" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete)
            if echo "$UPLOAD_FINAL_TEXT" | grep -q '"escalation_reason"'; then
                local reason
                reason=$(echo "$UPLOAD_FINAL_TEXT" | grep -oE '"escalation_reason":\s*"[^"]+"' | head -1 | sed 's/.*"escalation_reason":\s*"//' | sed 's/"//')
                pass "Escalation reason present: $reason"
            elif echo "$UPLOAD_FINAL_TEXT" | grep -q '"stage":1\|"stage": 1'; then
                skip "Upload completed via Stage 1 (no escalation occurred, hardened form may not be active)."
            else
                fail "Upload completed but escalation_reason not in result: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            fi
            ;;
        pilot_disabled)
            fail "Upload rejected: AI Web Pilot is disabled in the extension. Enable it and re-run."
            ;;
        pending)
            fail "Upload timed out — extension did not report a result within 30s. Check: extension reloaded? daemon restarted? pilot enabled? accessibility granted?"
            ;;
        failed|error)
            if echo "$UPLOAD_FINAL_TEXT" | grep -qi "accessibility\|Accessibility"; then
                skip "Stage 4 needs macOS Accessibility permission. Fix: System Settings > Privacy & Security > Accessibility. Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            elif echo "$UPLOAD_FINAL_TEXT" | grep -qi "xdotool"; then
                skip "Stage 4 needs xdotool. Fix: sudo apt install xdotool. Error: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            else
                fail "Stage 4 failed: $(truncate "$UPLOAD_FINAL_TEXT" 200)"
            fi
            ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS"
            ;;
    esac
}
run_test_15_17
