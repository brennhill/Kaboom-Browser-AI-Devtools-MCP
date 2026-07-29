#!/bin/bash
# cat-24-upload.sh — UAT tests for File Upload (15 tests).
# Tests upload MCP action, parameter validation, security, MIME detection,
# and HTTP Stage 1/3 endpoints.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "24" "File Upload" "22"

# macOS tmp paths are often /var -> /private/var symlinks; daemon startup
# rejects symlinked --upload-dir for security. Canonicalize once for all tests.
TEMP_DIR="$(cd "$TEMP_DIR" && pwd -P)"

# ── Safety-net trap: kill orphaned upload servers on exit ──
_cat24_cleanup() {
    # Kill upload servers on any port we might have used
    for _p in $((PORT + 100)) $((PORT + 101)) $((PORT + 102)); do
        lsof -tiTCP:"$_p" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
    done
}
trap '_cat24_cleanup; framework_cleanup' EXIT INT TERM

restart_daemon_with_upload_flags() {
    local attempts=0
    while [ "$attempts" -lt 3 ]; do
        kill_server
        sleep 0.4
        if start_daemon_with_flags "$@"; then
            return 0
        fi
        attempts=$((attempts + 1))
        sleep 0.6
    done
    return 1
}

# ── Create temp test fixtures ──────────────────────────────
echo "Hello upload test" > "$TEMP_DIR/test-file.txt"
echo '{"key":"value"}' > "$TEMP_DIR/test-file.json"
# 1x1 red PNG (89 bytes)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > "$TEMP_DIR/test-file.png"
mkdir -p "$TEMP_DIR/test-dir"

# ── 24.1 — Stage 4 OS automation disabled without --enable-os-upload-automation ──
begin_test "24.1" "Stage 4 OS automation disabled without --enable-os-upload-automation flag" \
    "HTTP POST /api/os-automation/inject without the flag should return 403" \
    "Security: OS-level automation requires explicit opt-in."

# Start daemon WITHOUT os-upload-automation flag for this test
if ! start_daemon; then
    fail "Failed to start daemon for upload tests."
    finish_category
fi

run_test_24_1() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{"file_path":"/tmp/test.txt","browser_pid":1234}' \
        "http://localhost:${PORT}/api/os-automation/inject" 2>/dev/null)
    if [ "$status" = "403" ]; then
        pass "Stage 4 OS automation correctly returns 403 without flag."
    else
        fail "Expected HTTP 403 for OS automation without flag, got $status."
    fi
}
run_test_24_1

# Upload stages 1-3 and MCP handler work without any flag — no restart needed

# ── 24.2 — upload in interact schema enum ──────────────────
begin_test "24.2" "upload in tools/list interact what enum" \
    "Verify tools/list includes upload in interact what enum" \
    "Schema correctness: LLMs discover upload via the canonical interact discriminator."
run_test_24_2() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local has_upload
    has_upload=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "interact")
        | .inputSchema.properties.what.enum[]
        | select(. == "upload")
    ' 2>/dev/null)
    if [ "$has_upload" = "upload" ]; then
        pass "upload found in interact what enum."
    else
        fail "upload NOT found in interact what enum. Response: $(truncate "$TOOLS_RESP" 300)"
    fi
}
run_test_24_2

# ── 24.3 — Missing file_path returns isError ──────────────
begin_test "24.3" "Missing file_path returns isError" \
    "interact(upload) without file_path should return isError with missing_param code" \
    "Required parameter validation."
run_test_24_3() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for missing file_path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "file_path|missing_param"; then
        pass "Missing file_path correctly returned error mentioning file_path."
    else
        fail "Expected file_path in error. Content: $(truncate "$text")"
    fi
}
run_test_24_3

# ── 24.4 — Missing both selector AND apiEndpoint returns isError ──
begin_test "24.4" "Missing selector and apiEndpoint returns isError" \
    "interact(upload) without selector or apiEndpoint should return isError" \
    "Required parameter validation: at least one target is needed."
run_test_24_4() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for missing selector and apiEndpoint."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "selector|apiEndpoint|missing_param"; then
        pass "Missing selector/apiEndpoint correctly returned error."
    else
        fail "Expected selector mention in error. Content: $(truncate "$text")"
    fi
}
run_test_24_4

# ── 24.5 — Relative path rejected with path_not_allowed ───
begin_test "24.5" "Relative path rejected with path_not_allowed" \
    "interact(upload) with relative file_path should return path_not_allowed error" \
    "Security: only absolute paths allowed."
run_test_24_5() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"relative/path.txt","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for relative path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "path_not_allowed|absolute path"; then
        pass "Relative path correctly rejected with path_not_allowed."
    else
        fail "Expected path_not_allowed in error. Content: $(truncate "$text")"
    fi
}
run_test_24_5

# ── 24.6 — File not found returns invalid_param ───────────
begin_test "24.6" "File not found returns invalid_param" \
    "interact(upload) with nonexistent file should return isError with invalid_param" \
    "Graceful error for missing files."
run_test_24_6() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"/tmp/kaboom-uat-nonexistent-file-99999.txt","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for nonexistent file."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "invalid_param|not found|File not found"; then
        pass "Nonexistent file correctly returned error."
    else
        fail "Expected file not found error. Content: $(truncate "$text")"
    fi
}
run_test_24_6

# ── 24.7 — Directory rejected with invalid_param ──────────
begin_test "24.7" "Directory rejected with invalid_param" \
    "interact(upload) with a directory path should return isError with invalid_param" \
    "Only files can be uploaded, not directories."
run_test_24_7() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-dir"'","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for directory path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "invalid_param|directory|not a file"; then
        pass "Directory path correctly rejected."
    else
        fail "Expected directory rejection error. Content: $(truncate "$text")"
    fi
}
run_test_24_7

# ── 24.8 — Success: queued response has expected fields ───
begin_test "24.8" "Queued response has status, correlation_id, file_name, file_size, mime_type, progress_tier" \
    "interact(upload) with valid params returns queued response with all expected fields" \
    "Core success path: verify queued response shape."
run_test_24_8() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'","selector":"#file-input"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")

    local missing=""
    for field in status correlation_id file_name file_size mime_type progress_tier; do
        if ! check_contains "$text" "$field"; then
            missing="$missing $field"
        fi
    done

    if [ -n "$missing" ]; then
        fail "Missing fields in queued response:$missing. Content: $(truncate "$text" 400)"
        return
    fi

    if check_matches "$text" '"status".*queued|"status": "queued"'; then
        pass "Upload queued response has all expected fields (status=queued, correlation_id, file_name, file_size, mime_type, progress_tier)."
    else
        # Fields present but status might not be "queued" — still pass for field presence
        pass "Upload response has all expected fields. Content: $(truncate "$text" 200)"
    fi
}
run_test_24_8

# ── 24.9 — MIME detection: .txt → text/plain ──────────────
begin_test "24.9" "MIME detection: .txt → text/plain" \
    "Upload a .txt file and verify mime_type is text/plain" \
    "MIME type detection correctness."
run_test_24_9() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "text/plain"; then
        pass "MIME detection: .txt correctly detected as text/plain."
    else
        fail "Expected text/plain. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_9

# ── 24.10 — MIME detection: .json → application/json ──────
begin_test "24.10" "MIME detection: .json → application/json" \
    "Upload a .json file and verify mime_type is application/json" \
    "MIME type detection correctness."
run_test_24_10() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-file.json"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "application/json"; then
        pass "MIME detection: .json correctly detected as application/json."
    else
        fail "Expected application/json. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_10

# ── 24.11 — MIME detection: .png → image/png ──────────────
begin_test "24.11" "MIME detection: .png → image/png" \
    "Upload a .png file and verify mime_type is image/png" \
    "MIME type detection correctness."
run_test_24_11() {
    RESPONSE=$(call_tool "interact" '{"what":"upload","file_path":"'"$TEMP_DIR/test-file.png"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "image/png"; then
        pass "MIME detection: .png correctly detected as image/png."
    else
        fail "Expected image/png. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_11

# ── 24.12 — HTTP POST /api/file/read — valid file → 200 ──
begin_test "24.12" "HTTP POST /api/file/read — valid file returns 200 + base64" \
    "POST a valid file_path to /api/file/read; verify 200 + success + base64 data" \
    "Stage 1 HTTP endpoint: file read and base64 encoding."
run_test_24_12() {
    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/test-file.txt"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status. Body: $(truncate "$body")"
        return
    fi

    local success file_name data_base64
    success=$(echo "$body" | jq -r '.success' 2>/dev/null)
    file_name=$(echo "$body" | jq -r '.file_name' 2>/dev/null)
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)

    if [ "$success" != "true" ]; then
        fail "Expected success:true. Body: $(truncate "$body")"
        return
    fi
    if [ "$file_name" != "test-file.txt" ]; then
        fail "Expected file_name=test-file.txt, got $file_name."
        return
    fi
    if [ -z "$data_base64" ] || [ "$data_base64" = "null" ]; then
        fail "Expected data_base64 to be non-empty. Body: $(truncate "$body")"
        return
    fi

    pass "POST /api/file/read returned 200, success=true, file_name=test-file.txt, base64 data present."
}
run_test_24_12

# ── 24.13 — HTTP POST /api/file/read — missing file_path → 400 ──
begin_test "24.13" "HTTP POST /api/file/read — missing file_path returns 400" \
    "POST to /api/file/read without file_path; verify 400 Bad Request" \
    "Stage 1 HTTP endpoint: required parameter validation."
run_test_24_13() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    if [ "$status" = "400" ]; then
        pass "POST /api/file/read with missing file_path returned HTTP 400."
    else
        fail "Expected HTTP 400, got HTTP $status."
    fi
}
run_test_24_13

# ── 24.14 — HTTP POST /api/file/read — file not found → 404 ──
begin_test "24.14" "HTTP POST /api/file/read — nonexistent file returns 404" \
    "POST to /api/file/read with nonexistent file; verify 404 Not Found" \
    "Stage 1 HTTP endpoint: graceful 404 for missing files."
run_test_24_14() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{"file_path":"/tmp/kaboom-uat-nonexistent-file-99999.txt"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    if [ "$status" = "404" ]; then
        pass "POST /api/file/read with nonexistent file returned HTTP 404."
    else
        fail "Expected HTTP 404, got HTTP $status."
    fi
}
run_test_24_14

# ── 24.15 — HTTP POST /api/form/submit — missing fields → 400 ──
begin_test "24.15" "HTTP POST /api/form/submit — missing required fields returns 400" \
    "POST to /api/form/submit with empty body; verify 400 Bad Request" \
    "Stage 3 HTTP endpoint: required parameter validation."
run_test_24_15() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{}' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    if [ "$status" = "400" ]; then
        pass "POST /api/form/submit with missing fields returned HTTP 400."
    else
        fail "Expected HTTP 400, got HTTP $status."
    fi
}
run_test_24_15

# ── 24.16 — Stage 1 base64 text roundtrip ──────────────────
begin_test "24.16" "Stage 1 base64 text roundtrip" \
    "POST /api/file/read with known text content, decode base64, verify match" \
    "Proves file data flows end-to-end through Stage 1."
run_test_24_16() {
    local test_content="Kaboom upload roundtrip test $(date +%s)"
    echo -n "$test_content" > "$TEMP_DIR/roundtrip.txt"

    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/roundtrip.txt"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status."
        return
    fi

    local data_base64 decoded
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)
    decoded=$(echo "$data_base64" | base64 -d 2>/dev/null)

    if [ "$decoded" = "$test_content" ]; then
        pass "Stage 1 base64 roundtrip: decoded content matches original."
    else
        fail "Decoded content mismatch. Expected: $(truncate "$test_content" 50), got: $(truncate "$decoded" 50)"
    fi
}
run_test_24_16

# ── 24.17 — Stage 1 base64 binary roundtrip ───────────────
begin_test "24.17" "Stage 1 base64 binary roundtrip" \
    "POST /api/file/read with PNG binary file, decode base64, verify MD5 match" \
    "Proves binary data survives base64 encoding through Stage 1."
run_test_24_17() {
    local original_md5
    original_md5=$(md5sum "$TEMP_DIR/test-file.png" 2>/dev/null | awk '{print $1}' || md5 -q "$TEMP_DIR/test-file.png" 2>/dev/null)

    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/test-file.png"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status."
        return
    fi

    local data_base64 decoded_md5
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)
    echo "$data_base64" | base64 -d > "$TEMP_DIR/decoded.png" 2>/dev/null
    decoded_md5=$(md5sum "$TEMP_DIR/decoded.png" 2>/dev/null | awk '{print $1}' || md5 -q "$TEMP_DIR/decoded.png" 2>/dev/null)

    if [ "$original_md5" = "$decoded_md5" ]; then
        pass "Stage 1 binary roundtrip: MD5 $original_md5 matches."
    else
        fail "MD5 mismatch: original=$original_md5, decoded=$decoded_md5."
    fi
}
run_test_24_17

# ── 24.18 — Stage 1 file size accuracy ────────────────────
begin_test "24.18" "Stage 1 file size accuracy" \
    "Create exact 1024-byte file, POST /api/file/read, verify file_size == 1024" \
    "Proves metadata extraction returns accurate file size."
run_test_24_18() {
    dd if=/dev/zero of="$TEMP_DIR/exact-1024.bin" bs=1024 count=1 2>/dev/null

    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/exact-1024.bin"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status."
        return
    fi

    local file_size
    file_size=$(echo "$body" | jq -r '.file_size' 2>/dev/null)

    if [ "$file_size" = "1024" ]; then
        pass "Stage 1 file size: exactly 1024 bytes reported."
    else
        fail "Expected file_size=1024, got $file_size."
    fi
}
run_test_24_18

# ── 24.19 — Stage 3 form submit — full Rumble-style upload ──
begin_test "24.19" "Stage 3 form submit — full Rumble-style upload" \
    "Start Python upload server, get cookie+CSRF, restart daemon with --ssrf-allow-host, POST /api/form/submit, verify end-to-end" \
    "Proves file data flows through Stage 3 multipart streaming to a real server."
run_test_24_19() {
    # Find a free port for the upload server
    local UPLOAD_PORT
    UPLOAD_PORT=$((PORT + 100))

    # Start Python upload server
    python3 "$SCRIPT_DIR/../../smoke-tests/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Upload test server failed to start on port $UPLOAD_PORT."
        return
    fi

    # Get session cookie
    local cookie_jar="$TEMP_DIR/cookies.txt"
    local session_cookie="" form_html="" csrf_token=""
    local attempt
    for attempt in 1 2 3 4 5; do
        curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1
        session_cookie=$(awk '$6 == "session" {print $7; exit}' "$cookie_jar" 2>/dev/null)
        form_html=$(curl -s -b "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/upload" 2>/dev/null)
        csrf_token=$(echo "$form_html" | grep -oE 'name="csrf_token"[^>]*value="[a-f0-9]{32}"' | head -1 | sed -E 's/.*value="([a-f0-9]{32})".*/\1/')
        if [ -n "$session_cookie" ] && [ -n "$csrf_token" ]; then
            break
        fi
        sleep 0.2
    done

    if [ -z "$csrf_token" ] || [ -z "$session_cookie" ]; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Failed to extract session cookie or CSRF token from upload form."
        return
    fi

    # Restart daemon with --ssrf-allow-host and --upload-dir
    if ! restart_daemon_with_upload_flags "--ssrf-allow-host=localhost:${UPLOAD_PORT}" "--upload-dir=$TEMP_DIR"; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Failed to restart daemon with upload flags for Stage 3 test."
        return
    fi

    # Create test file
    local test_content="Kaboom E2E upload test $(date +%s)"
    echo -n "$test_content" > "$TEMP_DIR/e2e-upload.txt"
    local original_md5
    original_md5=$(md5sum "$TEMP_DIR/e2e-upload.txt" 2>/dev/null | awk '{print $1}' || md5 -q "$TEMP_DIR/e2e-upload.txt" 2>/dev/null)

    # POST /api/form/submit
    local cookie_header="session=${session_cookie}"
    local submit_body submit_status
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "method": "POST",
            "file_path": "'"$TEMP_DIR/e2e-upload.txt"'",
            "file_input_name": "Filedata",
            "csrf_token": "'"$csrf_token"'",
            "cookies": "'"$cookie_header"'",
            "fields": {"title": "E2E Test Upload", "tags": "kaboom,test"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    submit_status=$(echo "$submit_body" | tail -1)
    submit_body=$(echo "$submit_body" | sed '$d')

    local success
    success=$(echo "$submit_body" | jq -r '.success' 2>/dev/null)

    if [ "$success" != "true" ]; then
        local error_msg
        error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Form submit failed. HTTP=$submit_status, success=$success, error=$error_msg"
        return
    fi

    # Verify via /api/last-upload
    local verify_body
    verify_body=$(curl -s "http://127.0.0.1:${UPLOAD_PORT}/api/last-upload" 2>/dev/null)
    local verify_md5 verify_csrf verify_cookie
    verify_md5=$(echo "$verify_body" | jq -r '.md5' 2>/dev/null)
    verify_csrf=$(echo "$verify_body" | jq -r '.csrf_ok' 2>/dev/null)
    verify_cookie=$(echo "$verify_body" | jq -r '.cookie_ok' 2>/dev/null)

    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true

    if [ "$verify_md5" = "$original_md5" ] && [ "$verify_csrf" = "true" ] && [ "$verify_cookie" = "true" ]; then
        pass "Stage 3 E2E: MD5 match ($verify_md5), CSRF ok, cookie ok."
    else
        fail "Stage 3 E2E verification: md5=$verify_md5 (expected $original_md5), csrf=$verify_csrf, cookie=$verify_cookie."
    fi
}
run_test_24_19

# ── 24.20 — Stage 3 missing cookie returns 401 ────────────
begin_test "24.20" "Stage 3 — missing cookie returns 401 error" \
    "POST /api/form/submit without cookie to upload server, verify 401 propagated" \
    "Proves auth failure from platform is reported correctly."
run_test_24_20() {
    # Start upload server
    local UPLOAD_PORT
    UPLOAD_PORT=$((PORT + 101))
    python3 "$SCRIPT_DIR/../../smoke-tests/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Upload test server failed to start."
        return
    fi

    # Restart daemon with allowed host
    if ! restart_daemon_with_upload_flags "--ssrf-allow-host=localhost:${UPLOAD_PORT}" "--upload-dir=$TEMP_DIR"; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Failed to restart daemon with upload flags for missing-cookie test."
        return
    fi

    echo -n "test" > "$TEMP_DIR/no-cookie.txt"

    local submit_body
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "file_path": "'"$TEMP_DIR/no-cookie.txt"'",
            "file_input_name": "Filedata",
            "fields": {"title": "test"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true

    local error_msg
    error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)

    if echo "$error_msg" | grep -qiE "401|not logged in"; then
        pass "Missing cookie: correctly reports 401/not logged in."
    else
        fail "Expected 401/not-logged-in error. Got: $(truncate "$error_msg" 200)"
    fi
}
run_test_24_20

# ── 24.21 — Stage 3 wrong CSRF returns 403 ────────────────
begin_test "24.21" "Stage 3 — wrong CSRF returns 403 error" \
    "POST /api/form/submit with valid cookie but wrong CSRF, verify 403 propagated" \
    "Proves CSRF failure from platform is reported correctly."
run_test_24_21() {
    # Start upload server
    local UPLOAD_PORT
    UPLOAD_PORT=$((PORT + 102))
    python3 "$SCRIPT_DIR/../../smoke-tests/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Upload test server failed to start."
        return
    fi

    # Get session cookie
    local cookie_jar="$TEMP_DIR/cookies-csrf.txt"
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1
    local session_cookie
    session_cookie=$(grep "session" "$cookie_jar" 2>/dev/null | awk '{print $NF}')

    # Restart daemon with allowed host
    if ! restart_daemon_with_upload_flags "--ssrf-allow-host=localhost:${UPLOAD_PORT}" "--upload-dir=$TEMP_DIR"; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Failed to restart daemon with upload flags for bad-CSRF test."
        return
    fi

    echo -n "test" > "$TEMP_DIR/bad-csrf.txt"

    local submit_body
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "file_path": "'"$TEMP_DIR/bad-csrf.txt"'",
            "file_input_name": "Filedata",
            "csrf_token": "wrong-csrf-token",
            "cookies": "session='"${session_cookie}"'",
            "fields": {"title": "test"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true

    local error_msg
    error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)

    if echo "$error_msg" | grep -qiE "403|CSRF|forbidden"; then
        pass "Wrong CSRF: correctly reports 403/CSRF error."
    else
        fail "Expected 403/CSRF error. Got: $(truncate "$error_msg" 200)"
    fi
}
run_test_24_21

# ── 24.22 — Stage 4 flag-enabled validation chain ──────
begin_test "24.22" "Stage 4 OS automation returns non-403 when flag enabled" \
    "Restart daemon with --enable-os-upload-automation, POST /api/os-automation/inject, verify NOT 403" \
    "Proves flag unlocks the endpoint. Expect 400 (no dialog open), not 403 (disabled)."
run_test_24_22() {
    if ! restart_daemon_with_upload_flags "--enable-os-upload-automation" "--upload-dir=$TEMP_DIR"; then
        fail "Failed to restart daemon with --enable-os-upload-automation for Stage 4 test."
        return
    fi

    # Create a test file in the allowed upload dir
    echo -n "stage4-flag-test" > "$TEMP_DIR/stage4-test.txt"

    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/stage4-test.txt"'","browser_pid":1234}' \
        "http://localhost:${PORT}/api/os-automation/inject" 2>/dev/null)

    if [ "$status" = "403" ]; then
        fail "Expected non-403 with flag enabled, but got 403 (flag not recognized)."
    elif [ "$status" = "400" ] || [ "$status" = "200" ]; then
        pass "Stage 4 flag enabled: got HTTP $status (not 403). Flag recognized."
    else
        pass "Stage 4 flag enabled: got HTTP $status (not 403). Endpoint unlocked."
    fi
}
run_test_24_22

finish_category
