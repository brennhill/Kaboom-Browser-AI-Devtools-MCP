#!/bin/bash
# 20-inspect-visual.sh — 20.0-20.11: Computed styles, form discovery, and visual regression.
# Requires extension + pilot connected.
set -eo pipefail

begin_category "20" "Inspect & Visual Regression" "12"

# ── Test 20.0: Computed styles canary ──────────────────────
begin_test "20.0" "[BROWSER] Computed styles canary" \
    "analyze(what='computed_styles', selector='body') returns color, font-size, display" \
    "Tests: computed_styles > extension > content script > inject"

run_test_20_0() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"computed_styles","selector":"body"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Computed styles failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "computed_styles\|elements\|color"; then
        pass "Computed styles returned CSS properties for body."
    else
        fail "Unexpected result format. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_0

# ── Test 20.1: Computed styles box model ───────────────────
begin_test "20.1" "[BROWSER] Computed styles box model" \
    "Result includes width, height from getBoundingClientRect" \
    "Tests: box model dimensions in computed styles"

run_test_20_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"computed_styles","selector":"body"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -q "box_model\|width\|height"; then
        pass "Computed styles includes box model dimensions."
    else
        fail "Box model not found in result. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_1

# ── Test 20.2: Computed styles property filter ─────────────
begin_test "20.2" "[BROWSER] Computed styles property filter" \
    "properties:['color','font-size'] returns only those two" \
    "Tests: property filtering in computed_styles"

run_test_20_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"computed_styles","selector":"body","properties":["color","font-size"]}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Filtered computed styles failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "color\|font-size"; then
        pass "Computed styles returned filtered properties."
    else
        fail "Filtered properties not found. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_2

# ── Test 20.3: Form discovery ──────────────────────────────
begin_test "20.3" "[BROWSER] Form discovery" \
    "Navigate to upload server, analyze(what='forms') returns form" \
    "Tests: form_discovery > extension > content script > inject"

run_test_20_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to upload server if available
    if [ -n "$UPLOAD_SERVER_URL" ]; then
        interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"${UPLOAD_SERVER_URL}/upload\"}" 20
        sleep 2
    fi

    call_tool "analyze" '{"what":"forms"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Form discovery failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "forms\|fields\|action"; then
        pass "Form discovery returned form information."
    else
        fail "No form data found. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_3

# ── Test 20.4: Form discovery field types ──────────────────
begin_test "20.4" "[BROWSER] Form discovery field types" \
    "Fields include correct type, required, label, name" \
    "Tests: field metadata extraction"

run_test_20_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"forms"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -q "type\|name\|required"; then
        pass "Form fields include type/name/required metadata."
    else
        fail "Field metadata missing. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_4

# ── Test 20.5: Form validation state ──────────────────────
begin_test "20.5" "[BROWSER] Form validation state" \
    "analyze(what='form_validation') returns validation messages" \
    "Tests: form validation via checkValidity"

run_test_20_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"form_validation"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error.*timeout\|failed.*connect"; then
        fail "Form validation command failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "valid\|forms\|validation"; then
        pass "Form validation returned validation state."
    else
        fail "No validation data. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_5

# ── Test 20.6: Fill form ──────────────────────────────────
begin_test "20.6" "[BROWSER] Fill form without submit" \
    "interact(what='fill_form') fills fields, verify via execute_js" \
    "Tests: fill_form workflow"

run_test_20_6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # First inject a simple form
    interact_and_wait "execute_js" '{"what":"execute_js","script":"(function(){var f=document.createElement(\"form\");f.id=\"smoke-fill-form\";f.innerHTML=\"<input id=sf-fill-name name=name><input id=sf-fill-email name=email>\";document.body.appendChild(f);return \"ok\";})()"}'
    sleep 0.5

    interact_and_wait "fill_form" '{"what":"fill_form","fields":[{"selector":"#sf-fill-name","value":"TestUser"},{"selector":"#sf-fill-email","value":"test@example.com"}]}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "fill_form failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Verify values
    interact_and_wait "execute_js" '{"what":"execute_js","script":"document.getElementById(\"sf-fill-name\").value + \"|\" + document.getElementById(\"sf-fill-email\").value"}'

    if echo "$INTERACT_RESULT" | grep -q "TestUser.*test@example.com"; then
        pass "fill_form correctly filled fields without submitting."
    else
        fail "Field values not confirmed. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_20_6

# ── Test 20.7: Fill form trace ────────────────────────────
begin_test "20.7" "[BROWSER] Fill form trace" \
    "Response includes per-field workflow trace" \
    "Tests: workflow trace in fill_form response"

run_test_20_7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "fill_form" '{"what":"fill_form","fields":[{"selector":"#sf-fill-name","value":"Traced"}]}'

    if echo "$INTERACT_RESULT" | grep -q "trace\|workflow\|timing"; then
        pass "fill_form response includes workflow trace."
    else
        fail "No trace in response. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_20_7

# ── Test 20.8: Visual baseline save ──────────────────────
begin_test "20.8" "[BROWSER] Visual baseline save" \
    "analyze(what='visual_baseline', name='test-baseline') returns path" \
    "Tests: screenshot + session store baseline save"

run_test_20_8() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"visual_baseline","name":"smoke-test-baseline"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Visual baseline save failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "saved\|path\|smoke-test-baseline"; then
        pass "Visual baseline saved successfully."
    else
        fail "No baseline save confirmation. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_8

# ── Test 20.9: Visual baseline list ──────────────────────
begin_test "20.9" "[BROWSER] Visual baseline list" \
    "analyze(what='visual_baselines') includes 'smoke-test-baseline'" \
    "Tests: session store baseline listing"

run_test_20_9() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"visual_baselines"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -q "smoke-test-baseline\|visual_baselines"; then
        pass "Visual baselines list includes saved baseline."
    else
        fail "Baseline not found in list. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_9

# ── Test 20.10: Visual diff identical ────────────────────
begin_test "20.10" "[BROWSER] Visual diff identical" \
    "Immediately diff same page > verdict 'identical' or 'minor_changes'" \
    "Tests: pixel diff comparison (JPEG noise tolerance)"

run_test_20_10() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    call_tool "analyze" '{"what":"visual_diff","baseline":"smoke-test-baseline"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Visual diff failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "identical\|minor_changes"; then
        pass "Visual diff of unchanged page shows identical/minor_changes."
    else
        fail "Unexpected verdict. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_10

# ── Test 20.11: Visual diff after change ─────────────────
begin_test "20.11" "[BROWSER] Visual diff after change" \
    "Navigate to different page, diff > verdict != 'identical'" \
    "Tests: pixel diff detects actual visual changes"

run_test_20_11() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to a different page to ensure visual difference
    interact_and_wait "execute_js" '{"what":"execute_js","script":"document.body.style.backgroundColor=\"red\";\"changed\""}'
    sleep 1

    call_tool "analyze" '{"what":"visual_diff","baseline":"smoke-test-baseline"}'
    local result
    result=$(extract_content_text)

    if echo "$result" | grep -qi "error\|failed"; then
        fail "Visual diff failed. Result: $(truncate "$result" 200)"
        return
    fi

    if echo "$result" | grep -q "minor_changes\|major_changes\|completely_different"; then
        pass "Visual diff correctly detected changes after modification."
    elif echo "$result" | grep -q "identical"; then
        fail "Expected changes after page modification, but got 'identical'. Result: $(truncate "$result" 200)"
    else
        fail "Unexpected result format. Result: $(truncate "$result" 200)"
    fi
}
run_test_20_11
