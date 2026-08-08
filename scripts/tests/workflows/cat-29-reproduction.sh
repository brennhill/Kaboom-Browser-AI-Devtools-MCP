#!/bin/bash
# cat-17-reproduction.sh — Category 17: Reproduction Export & Replay (6 tests).
# Tests the full reproduction script lifecycle:
# seed actions → export (kaboom + playwright) → write to file → parse → replay.
# Proves both export formats and recreation pipeline work end-to-end.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "29" "Reproduction Replay" "6"

ensure_daemon

KABOOM_FILE="$TEMP_DIR/kaboom-script.txt"
PLAYWRIGHT_FILE="$TEMP_DIR/playwright-script.js"

# Helper: seed 5 realistic actions via HTTP POST to /enhanced-actions.
# Simulates the extension recording a user flow: navigate → click → type → select → keypress.
seed_actions() {
    curl -s -X POST "http://localhost:${PORT}/enhanced-actions" \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension" \
        -d @- <<'SEED_EOF'
{
    "actions": [
        {
            "type": "navigate",
            "timestamp": 1707000000000,
            "url": "about:blank",
            "to_url": "https://app.example.com/dashboard",
            "source": "human"
        },
        {
            "type": "click",
            "timestamp": 1707000001000,
            "url": "https://app.example.com/dashboard",
            "selectors": {
                "text": "Export Data",
                "role": {"role": "button", "name": "Export Data"},
                "testId": "export-btn"
            },
            "source": "human"
        },
        {
            "type": "input",
            "timestamp": 1707000002000,
            "url": "https://app.example.com/dashboard",
            "selectors": {
                "ariaLabel": "File name",
                "role": {"role": "textbox", "name": "File name"},
                "id": "filename"
            },
            "value": "report-2026",
            "source": "human"
        },
        {
            "type": "select",
            "timestamp": 1707000005000,
            "url": "https://app.example.com/dashboard",
            "selectors": {
                "ariaLabel": "Format",
                "role": {"role": "combobox", "name": "Format"},
                "id": "format-select"
            },
            "selected_value": "csv",
            "selected_text": "CSV",
            "source": "human"
        },
        {
            "type": "keypress",
            "timestamp": 1707000006000,
            "url": "https://app.example.com/dashboard",
            "key": "Enter",
            "source": "human"
        }
    ]
}
SEED_EOF
}

# Helper: extract JSON payload from mcpJSONResponse content text.
# Content format is: "Summary line\n{json...}" — we want the JSON part.
extract_json_payload() {
    local text="$1"
    echo "$text" | tail -1
}

# ── 17.1 — Seed realistic actions ────────────────────────
begin_test "29.1" "seed actions via HTTP POST" \
    "POST 5 actions to /enhanced-actions. Verify accepted with status ok and count 5." \
    "Action ingestion is the foundation for reproduction scripts."
run_test_17_1() {
    local response
    response=$(seed_actions)
    local status count
    status=$(echo "$response" | jq -r '.status // empty' 2>/dev/null)
    count=$(echo "$response" | jq -r '.count // 0' 2>/dev/null)

    if [ "$status" != "ok" ]; then
        fail "POST /enhanced-actions failed. Response: $(truncate "$response")"
        return
    fi
    if [ "$count" -ne 5 ]; then
        fail "Expected 5 actions seeded, got $count. Response: $(truncate "$response")"
        return
    fi
    pass "Seeded 5 actions (navigate, click, input, select, keypress). Status: ok, count: 5."
}
run_test_17_1

# ── 17.2 — Export kaboom format ────────────────────────
begin_test "29.2" "export kaboom format with all action types" \
    "Call generate(reproduction, output_format=kaboom). Verify all 5 action types in numbered steps." \
    "Kaboom natural language format must be complete and human-readable."
run_test_17_2() {
    RESPONSE=$(call_tool "generate" '{"what":"reproduction","output_format":"kaboom-agentic-browser"}')
    if ! check_not_error "$RESPONSE"; then
        fail "generate(reproduction, kaboom) returned error: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text json_payload script
    text=$(extract_content_text "$RESPONSE")
    json_payload=$(extract_json_payload "$text")
    script=$(echo "$json_payload" | jq -r '.script // empty' 2>/dev/null)
    if [ -z "$script" ]; then
        fail "No 'script' field in response. Content: $(truncate "$text" 400)"
        return
    fi

    # Write to file for later tests
    echo "$script" > "$KABOOM_FILE"

    # Verify header
    if ! echo "$script" | grep -q "^# Reproduction:"; then
        fail "Missing '# Reproduction:' header. Script: $(truncate "$script" 400)"
        return
    fi

    # Verify all action types present
    local missing=""
    echo "$script" | grep -q "Navigate to:" || missing="$missing navigate"
    echo "$script" | grep -q "Click:" || missing="$missing click"
    echo "$script" | grep -q "Type " || missing="$missing input"
    echo "$script" | grep -q "Select " || missing="$missing select"
    echo "$script" | grep -q "Press:" || missing="$missing keypress"

    if [ -n "$missing" ]; then
        fail "Missing action types in kaboom script:$missing. Script: $(truncate "$script" 500)"
        return
    fi

    # Verify numbered steps
    local step_count
    step_count=$(echo "$script" | grep -cE '^[0-9]+\.' || true)
    if [ "$step_count" -ne 5 ]; then
        fail "Expected 5 numbered steps, got $step_count. Script: $(truncate "$script" 500)"
        return
    fi

    # Verify timing pause (3s gap between input@t+2s and select@t+5s)
    if ! echo "$script" | grep -q "\[3s pause\]"; then
        fail "Expected [3s pause] between input and select. Script: $(truncate "$script" 500)"
        return
    fi

    pass "Kaboom script: 5 numbered steps, all action types, timing pause. Written to file."
}
run_test_17_2

# ── 17.3 — Export playwright format ──────────────────────
begin_test "29.3" "export playwright format with valid code" \
    "Call generate(reproduction, output_format=playwright). Verify Playwright imports, locators, actions." \
    "Playwright format enables automated CI replay of recorded flows."
run_test_17_3() {
    RESPONSE=$(call_tool "generate" '{"what":"reproduction","output_format":"playwright"}')
    if ! check_not_error "$RESPONSE"; then
        fail "generate(reproduction, playwright) returned error: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text json_payload script
    text=$(extract_content_text "$RESPONSE")
    json_payload=$(extract_json_payload "$text")
    script=$(echo "$json_payload" | jq -r '.script // empty' 2>/dev/null)
    if [ -z "$script" ]; then
        fail "No 'script' field in response. Content: $(truncate "$text" 400)"
        return
    fi

    # Write to file
    echo "$script" > "$PLAYWRIGHT_FILE"

    # Verify Playwright structure
    local missing=""
    echo "$script" | grep -q "import.*@playwright/test" || missing="$missing import"
    echo "$script" | grep -q "page.goto" || missing="$missing goto"
    echo "$script" | grep -q "\.click()" || missing="$missing click"
    echo "$script" | grep -q "\.fill(" || missing="$missing fill"
    echo "$script" | grep -q "\.selectOption(" || missing="$missing selectOption"
    echo "$script" | grep -q "keyboard.press" || missing="$missing keypress"

    if [ -n "$missing" ]; then
        fail "Missing Playwright elements:$missing. Script: $(truncate "$script" 500)"
        return
    fi

    # Verify best-practice locators (testId > role > label > text)
    echo "$script" | grep -q "getByTestId" || { fail "Expected getByTestId locator for click. Script: $(truncate "$script" 500)"; return; }
    echo "$script" | grep -q "getByRole" || { fail "Expected getByRole locator. Script: $(truncate "$script" 500)"; return; }

    pass "Playwright script: imports, goto, click, fill, selectOption, keyboard.press. Best-practice locators."
}
run_test_17_3

# ── 17.4 — Verify exported files ─────────────────────────
begin_test "29.4" "verify exported files are parseable" \
    "Read kaboom and playwright files. Verify step counts and structural integrity." \
    "Export must produce deterministic, parseable output files."
run_test_17_4() {
    if [ ! -f "$KABOOM_FILE" ]; then
        fail "Kaboom file not found at $KABOOM_FILE (depends on test 17.2)"
        return
    fi
    if [ ! -f "$PLAYWRIGHT_FILE" ]; then
        fail "Playwright file not found at $PLAYWRIGHT_FILE (depends on test 17.3)"
        return
    fi

    # Verify kaboom step count
    local gas_steps
    gas_steps=$(grep -cE '^[0-9]+\.' "$KABOOM_FILE" || true)
    if [ "$gas_steps" -ne 5 ]; then
        fail "Expected 5 numbered steps in kaboom file, got $gas_steps"
        return
    fi

    # Verify playwright has test block
    local pw_lines
    pw_lines=$(wc -l < "$PLAYWRIGHT_FILE" | tr -d ' ')
    if [ "$pw_lines" -lt 5 ]; then
        fail "Playwright file too short: $pw_lines lines"
        return
    fi

    # Verify playwright has closing brace (complete test)
    if ! grep -q "});" "$PLAYWRIGHT_FILE"; then
        fail "Playwright file missing closing '});' — script may be truncated"
        return
    fi

    pass "Kaboom: $gas_steps steps. Playwright: $pw_lines lines with complete test block."
}
run_test_17_4

# ── 17.5 — Replay kaboom steps via interact ────────────
begin_test "29.5" "replay kaboom steps via interact tool" \
    "Parse kaboom script, convert each step to interact() call, verify dispatch." \
    "Proves the kaboom format is machine-parseable and the interact tool accepts the commands."
run_test_17_5() {
    if [ ! -f "$KABOOM_FILE" ]; then
        fail "Kaboom file not found (depends on test 17.2)"
        return
    fi

    local dispatched=0
    local total=0
    local rejected_example=""

    # Parse each numbered step and attempt replay via interact
    while IFS= read -r line; do
        # Only process numbered step lines (e.g., "1. Navigate to: ...")
        echo "$line" | grep -qE '^[0-9]+\.' || continue

        total=$((total + 1))
        local resp=""

        if echo "$line" | grep -q "Navigate to:"; then
            local url
            url="${line##*Navigate to: }"
            resp=$(call_tool "interact" "{\"what\":\"navigate\",\"url\":\"${url}\"}")

        elif echo "$line" | grep -q "Click:"; then
            # Extract text between first pair of quotes (capture group requires sed)
            local sel_text
            # shellcheck disable=SC2001 # capture group extraction requires sed
            sel_text=$(echo "$line" | sed 's/.*Click: "\([^"]*\)".*/\1/')
            resp=$(call_tool "interact" "{\"what\":\"click\",\"selector\":\"text=${sel_text}\"}")

        elif echo "$line" | grep -q "^[0-9]*\. Type"; then
            local val
            # shellcheck disable=SC2001 # capture group extraction requires sed
            val=$(echo "$line" | sed 's/.*Type "\([^"]*\)".*/\1/')
            resp=$(call_tool "interact" "{\"what\":\"type\",\"selector\":\"body\",\"text\":\"${val}\"}")

        elif echo "$line" | grep -q "Select "; then
            local val
            # shellcheck disable=SC2001 # capture group extraction requires sed
            val=$(echo "$line" | sed 's/.*Select "\([^"]*\)".*/\1/')
            resp=$(call_tool "interact" "{\"what\":\"select\",\"selector\":\"body\",\"value\":\"${val}\"}")

        elif echo "$line" | grep -q "Press:"; then
            local key
            key="${line##*Press: }"
            resp=$(call_tool "interact" "{\"what\":\"key_press\",\"text\":\"${key}\"}")
        fi

        # interact calls fail without pilot/extension, and that is expected — the
        # claim here is only that the step parsed and the tool ACCEPTED the
        # parameters. A valid JSON-RPC envelope alone does not show that:
        # missing_param and unknown_mode are well-formed rejections, and this
        # test passed for years while sending "action" instead of "what", so
        # every step was rejected before reaching a handler.
        local code=""
        if [ -n "$resp" ] && check_valid_jsonrpc "$resp"; then
            code=$(extract_content_text "$resp" | sed -n '/^{/,$p' | jq -r '.error_code // empty' 2>/dev/null)
            case "$code" in
                missing_param | unknown_mode | unknown_action | invalid_param) ;;
                *) dispatched=$((dispatched + 1)) ;;
            esac
        fi
        if [ -n "$code" ] && [ -z "$rejected_example" ]; then
            case "$code" in
                missing_param | unknown_mode | unknown_action | invalid_param)
                    rejected_example="$line -> $code"
                    ;;
            esac
        fi
    done < "$KABOOM_FILE"

    if [ "$total" -eq 0 ]; then
        fail "No steps parsed from kaboom file"
        return
    fi

    if [ "$dispatched" -ne "$total" ]; then
        fail "Only $dispatched of $total steps were accepted by the interact tool. First rejection: ${rejected_example:-unknown}"
        return
    fi

    pass "All $dispatched/$total steps parsed from kaboom file and accepted by the interact tool."
}
run_test_17_5

# ── 17.6 — Verify last_n filtering ───────────────────────
begin_test "29.6" "last_n filter returns correct subset" \
    "Call generate(reproduction, last_n=2). Verify only the last 2 actions (select + keypress)." \
    "last_n is essential for focusing reproduction on recent actions."
run_test_17_6() {
    RESPONSE=$(call_tool "generate" '{"what":"reproduction","output_format":"kaboom-agentic-browser","last_n":2}')
    if ! check_not_error "$RESPONSE"; then
        fail "generate with last_n returned error: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text json_payload script
    text=$(extract_content_text "$RESPONSE")
    json_payload=$(extract_json_payload "$text")
    script=$(echo "$json_payload" | jq -r '.script // empty' 2>/dev/null)

    # Count steps — should be 2 (last 2 of 5)
    local step_count
    step_count=$(echo "$script" | grep -cE '^[0-9]+\.' || true)
    if [ "$step_count" -ne 2 ]; then
        fail "Expected 2 steps with last_n=2, got $step_count. Script: $(truncate "$script" 400)"
        return
    fi

    # Verify it's the LAST 2 actions (select + keypress)
    if ! echo "$script" | grep -q "Select "; then
        fail "Expected Select in last 2 actions. Script: $(truncate "$script" 400)"
        return
    fi
    if ! echo "$script" | grep -q "Press:"; then
        fail "Expected Press in last 2 actions. Script: $(truncate "$script" 400)"
        return
    fi

    # Verify action_count in metadata
    local action_count
    action_count=$(echo "$json_payload" | jq -r '.action_count // 0' 2>/dev/null)
    if [ "$action_count" -ne 2 ]; then
        fail "Expected action_count=2 in metadata, got $action_count"
        return
    fi

    pass "last_n=2: exactly 2 steps (Select + Press), action_count=2."
}
run_test_17_6

finish_category
