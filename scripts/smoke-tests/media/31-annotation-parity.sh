#!/bin/bash
# 31-annotation-parity.sh — deterministic annotation parity gate.
# Verifies annotation ingest/retrieval/generation paths without manual draw input.
set -eo pipefail

begin_category "31" "Annotation Parity Gate" "8"

# Ensure daemon is running — a prior test (e.g. 24.1) may have restarted it.
ensure_daemon

PARITY_SESSION_NAME="parity-${SMOKE_MARKER:-$(date +%s)}"
PARITY_TAB_A=31001
PARITY_TAB_B=31002
PARITY_ANN_A_ID="ann_parity_a_${SMOKE_MARKER:-seed}"
PARITY_ANN_B_ID="ann_parity_b_${SMOKE_MARKER:-seed}"
PARITY_CORR_A="corr_parity_a_${SMOKE_MARKER:-seed}"
PARITY_CORR_B="corr_parity_b_${SMOKE_MARKER:-seed}"
PARITY_URL_A="http://localhost:3000/dashboard"
PARITY_URL_B="http://localhost:5173/settings"
PARITY_CLIENT_HEADER="kaboom-extension/smoke-parity"
PARITY_SCREENSHOT_DATA_URL="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg=="

post_parity_annotation() {
    local tab_id="$1"
    local ann_id="$2"
    local ann_text="$3"
    local corr_id="$4"
    local page_url="$5"
    local framework_name="$6"
    local timestamp_ms="$7"

    local payload
    payload=$(
        jq -n \
            --arg screenshot "$PARITY_SCREENSHOT_DATA_URL" \
            --arg ann_id "$ann_id" \
            --arg ann_text "$ann_text" \
            --arg corr_id "$corr_id" \
            --arg page_url "$page_url" \
            --arg session_name "$PARITY_SESSION_NAME" \
            --arg framework_name "$framework_name" \
            --argjson tab_id "$tab_id" \
            --argjson ts "$timestamp_ms" \
            '{
                screenshot_data_url: $screenshot,
                annotations: [
                  {
                    id: $ann_id,
                    text: $ann_text,
                    element_summary: "button.submit",
                    correlation_id: $corr_id,
                    rect: {x: 24, y: 48, width: 220, height: 60},
                    page_url: $page_url,
                    timestamp: $ts
                  }
                ],
                element_details: {
                  ($corr_id): {
                    selector: "button.submit",
                    tag: "button",
                    text_content: $ann_text,
                    classes: ["submit", "primary"],
                    computed_styles: {
                      display: "inline-flex",
                      color: "rgb(17, 24, 39)"
                    },
                    parent_context: [
                      {
                        tag: "form",
                        selector: "form.profile"
                      }
                    ],
                    siblings: [
                      {
                        tag: "span",
                        text: "helper"
                      }
                    ],
                    css_framework: $framework_name
                  }
                },
                page_url: $page_url,
                tab_id: $tab_id,
                annot_session_name: $session_name
              }'
    )

    curl -s --max-time 12 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: ${PARITY_CLIENT_HEADER}" \
        -w "\n%{http_code}" \
        -d "$payload" \
        "http://localhost:${PORT}/draw-mode/complete" 2>/dev/null
}

should_retry_annotation_response() {
    local response="$1"
    local text="$2"

    if echo "$text" | grep -qiE "Server is starting up|retry this tool call in [0-9]+ seconds"; then
        return 0
    fi

    if check_is_error "$response" && echo "$text" | grep -qiE "no_data|annotation.*not found|annot_session.*not found|no annotations"; then
        return 0
    fi

    return 1
}

call_tool_with_annotation_retry() {
    local tool_name="$1"
    local args="$2"
    local max_attempts="${3:-6}"
    local sleep_seconds="${4:-1}"
    local response text

    for attempt in $(seq 1 "$max_attempts"); do
        response=$(call_tool "$tool_name" "$args")
        text=$(extract_content_text "$response")
        if should_retry_annotation_response "$response" "$text" && [ "$attempt" -lt "$max_attempts" ]; then
            sleep "$sleep_seconds"
            continue
        fi
        echo "$response"
        return 0
    done

    echo "$response"
    return 0
}

# ── Test 31.1: POST annotation A accepted ─────────────────
begin_test "31.1" "[DAEMON ONLY] POST draw-mode complete stores annotation A" \
    "POST deterministic annotation payload for project A" \
    "Tests: ingest path /draw-mode/complete and session persistence"

run_test_31_1() {
    local now_ms result status body ann_count
    now_ms=$(( $(date +%s) * 1000 ))
    result=$(post_parity_annotation "$PARITY_TAB_A" "$PARITY_ANN_A_ID" "react-annotation-parity" "$PARITY_CORR_A" "$PARITY_URL_A" "React" "$now_ms")
    status=$(echo "$result" | tail -1)
    body=$(echo "$result" | sed '$d')
    ann_count=$(echo "$body" | jq -r '.annotation_count // empty' 2>/dev/null)

    if [ "$status" = "200" ] && [ "$ann_count" = "1" ]; then
        pass "Annotation A stored (HTTP 200, annotation_count=1)."
    else
        fail "Annotation A POST failed. status=$status annotation_count=$ann_count body=$(truncate "$body" 220)"
    fi
}
run_test_31_1

# ── Test 31.2: POST annotation B accepted ─────────────────
begin_test "31.2" "[DAEMON ONLY] POST draw-mode complete stores annotation B" \
    "POST deterministic annotation payload for project B in same named session" \
    "Tests: multi-project named-session accumulation path"

run_test_31_2() {
    local now_ms result status body ann_count
    now_ms=$(( $(date +%s) * 1000 + 1 ))
    result=$(post_parity_annotation "$PARITY_TAB_B" "$PARITY_ANN_B_ID" "vue-annotation-parity" "$PARITY_CORR_B" "$PARITY_URL_B" "Vue" "$now_ms")
    status=$(echo "$result" | tail -1)
    body=$(echo "$result" | sed '$d')
    ann_count=$(echo "$body" | jq -r '.annotation_count // empty' 2>/dev/null)

    if [ "$status" = "200" ] && [ "$ann_count" = "1" ]; then
        pass "Annotation B stored (HTTP 200, annotation_count=1)."
    else
        fail "Annotation B POST failed. status=$status annotation_count=$ann_count body=$(truncate "$body" 220)"
    fi
}
run_test_31_2

# ── Test 31.3: analyze named session includes both projects ──
begin_test "31.3" "[DAEMON ONLY] analyze(annotations) named session returns multi-project data" \
    "Retrieve named session and verify both annotations + scope ambiguity metadata" \
    "Tests: named-session retrieval, project grouping, scope ambiguity hints"

run_test_31_3() {
    local response text verdict
    response=$(call_tool "analyze" "{\"what\":\"annotations\",\"annot_session\":\"${PARITY_SESSION_NAME}\"}")
    text=$(extract_content_text "$response")

    if check_is_error "$response"; then
        fail "analyze(annotations, annot_session) returned error. Content: $(truncate "$text" 220)"
        return
    fi

    verdict=$(
        PARITY_JSON_TEXT="$text" python3 - "$PARITY_ANN_A_ID" "$PARITY_ANN_B_ID" <<'PY'
import json, os, sys
ann_a, ann_b = sys.argv[1], sys.argv[2]
raw = os.environ.get("PARITY_JSON_TEXT", "")
start = raw.find("{")
if start < 0:
    print("FAIL no_json")
    raise SystemExit(0)
try:
    data = json.loads(raw[start:])
except Exception as exc:
    print(f"FAIL parse_error={exc}")
    raise SystemExit(0)
ids = set()
for page in data.get("pages", []):
    for ann in page.get("annotations", []):
        ann_id = ann.get("id")
        if ann_id:
            ids.add(ann_id)
total_count = int(data.get("total_count", -1))
page_count = int(data.get("page_count", -1))
projects = data.get("projects", [])
scope_ambiguous = bool(data.get("scope_ambiguous"))
if total_count >= 2 and page_count >= 2 and ann_a in ids and ann_b in ids and len(projects) >= 2 and scope_ambiguous:
    print(f"PASS total_count={total_count} page_count={page_count} projects={len(projects)}")
else:
    print(f"FAIL total_count={total_count} page_count={page_count} ids={sorted(ids)} projects={len(projects)} scope_ambiguous={scope_ambiguous}")
PY
    )

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Named-session retrieval verified. $verdict"
    else
        fail "Named-session retrieval invalid. $verdict. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_3

# ── Test 31.4: URL scope filter narrows project correctly ──
begin_test "31.4" "[DAEMON ONLY] URL scope filter narrows to project A" \
    "Use analyze(annotations, annot_session, url) and verify only project A annotation remains" \
    "Tests: cross-project scope safety filtering"

run_test_31_4() {
    local response text verdict
    response=$(call_tool "analyze" "{\"what\":\"annotations\",\"annot_session\":\"${PARITY_SESSION_NAME}\",\"url\":\"http://localhost:3000/*\"}")
    text=$(extract_content_text "$response")

    if check_is_error "$response"; then
        fail "Scoped analyze returned error. Content: $(truncate "$text" 220)"
        return
    fi

    verdict=$(
        PARITY_JSON_TEXT="$text" python3 - "$PARITY_ANN_A_ID" "$PARITY_ANN_B_ID" <<'PY'
import json, os, sys
ann_a, ann_b = sys.argv[1], sys.argv[2]
raw = os.environ.get("PARITY_JSON_TEXT", "")
start = raw.find("{")
if start < 0:
    print("FAIL no_json")
    raise SystemExit(0)
try:
    data = json.loads(raw[start:])
except Exception as exc:
    print(f"FAIL parse_error={exc}")
    raise SystemExit(0)
ids = []
for page in data.get("pages", []):
    for ann in page.get("annotations", []):
        ann_id = ann.get("id")
        if ann_id:
            ids.append(ann_id)
total_count = int(data.get("total_count", -1))
if total_count == 1 and ann_a in ids and ann_b not in ids:
    print(f"PASS total_count={total_count} ids={ids}")
else:
    print(f"FAIL total_count={total_count} ids={ids}")
PY
    )

    if echo "$verdict" | grep -q "^PASS"; then
        pass "URL filter enforced correctly. $verdict"
    else
        fail "URL filter mismatch. $verdict. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_4

# ── Test 31.5: annotation_detail returns enriched fields ──
begin_test "31.5" "[DAEMON ONLY] annotation_detail returns selector/tag/framework context" \
    "Fetch annotation_detail for known correlation_id and verify key fields" \
    "Tests: detail store + retrieval enrichment path"

run_test_31_5() {
    local response text verdict
    response=$(call_tool_with_annotation_retry "analyze" "{\"what\":\"annotation_detail\",\"correlation_id\":\"${PARITY_CORR_A}\"}" 6 1)
    text=$(extract_content_text "$response")

    if check_is_error "$response"; then
        fail "annotation_detail returned error. Content: $(truncate "$text" 220)"
        return
    fi

    verdict=$(
        PARITY_JSON_TEXT="$text" python3 - <<'PY'
import json, os, sys
raw = os.environ.get("PARITY_JSON_TEXT", "")
start = raw.find("{")
if start < 0:
    print("FAIL no_json")
    raise SystemExit(0)
try:
    data = json.loads(raw[start:])
except Exception as exc:
    print(f"FAIL parse_error={exc}")
    raise SystemExit(0)
selector = data.get("selector")
tag = data.get("tag")
framework = data.get("css_framework")
has_parent = bool(data.get("parent_context"))
has_styles = bool(data.get("computed_styles"))
if selector and tag and framework == "React" and has_parent and has_styles:
    print(f"PASS selector={selector} tag={tag} framework={framework}")
else:
    print(f"FAIL selector={bool(selector)} tag={bool(tag)} framework={framework} parent={has_parent} styles={has_styles}")
PY
    )

    if echo "$verdict" | grep -q "^PASS"; then
        pass "annotation_detail verified. $verdict"
    else
        fail "annotation_detail missing expected fields. $verdict. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_5

# ── Test 31.6: generate visual_test from named session ─────
begin_test "31.6" "[DAEMON ONLY] generate(visual_test) works from named session" \
    "Generate Playwright visual test from deterministic named session data" \
    "Tests: annotation-to-test generation path"

run_test_31_6() {
    local response text
    response=$(call_tool_with_annotation_retry "generate" "{\"format\":\"visual_test\",\"annot_session\":\"${PARITY_SESSION_NAME}\",\"test_name\":\"annotation parity smoke\"}" 6 1)
    text=$(extract_content_text "$response")

    if check_matches "$text" "test\\(|page\\.goto\\(" ; then
        pass "visual_test generation contains test() + page.goto()."
    else
        fail "visual_test generation missing expected code. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_6

# ── Test 31.7: generate annotation_report markdown ─────────
begin_test "31.7" "[DAEMON ONLY] generate(annotation_report) returns markdown report" \
    "Generate annotation report from deterministic named session and verify header" \
    "Tests: annotation report generation path"

run_test_31_7() {
    local response text
    response=$(call_tool_with_annotation_retry "generate" "{\"format\":\"annotation_report\",\"annot_session\":\"${PARITY_SESSION_NAME}\"}" 6 1)
    text=$(extract_content_text "$response")

    if check_matches "$text" "^# Annotation Report|## Page"; then
        pass "annotation_report includes expected markdown structure."
    else
        fail "annotation_report missing expected markdown header/sections. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_7

# ── Test 31.8: generate annotation_issues structure ────────
begin_test "31.8" "[DAEMON ONLY] generate(annotation_issues) returns structured issues" \
    "Generate structured issues and verify count + ids" \
    "Tests: issue artifact generation path"

run_test_31_8() {
    local response text verdict
    response=$(call_tool_with_annotation_retry "generate" "{\"format\":\"annotation_issues\",\"annot_session\":\"${PARITY_SESSION_NAME}\"}" 6 1)
    text=$(extract_content_text "$response")

    verdict=$(
        PARITY_JSON_TEXT="$text" python3 - "$PARITY_ANN_A_ID" "$PARITY_ANN_B_ID" <<'PY'
import json, os, sys
ann_a, ann_b = sys.argv[1], sys.argv[2]
raw = os.environ.get("PARITY_JSON_TEXT", "")
start = raw.find("{")
if start < 0:
    print("FAIL no_json")
    raise SystemExit(0)
try:
    data = json.loads(raw[start:])
except Exception as exc:
    print(f"FAIL parse_error={exc}")
    raise SystemExit(0)
issues = data.get("issues", [])
total_count = data.get("total_count")
ids = {issue.get("annotation_id") for issue in issues if isinstance(issue, dict)}
if isinstance(total_count, int) and total_count >= 2 and ann_a in ids and ann_b in ids:
    print(f"PASS total_count={total_count} ids={sorted(ids)}")
else:
    print(f"FAIL total_count={total_count} ids={sorted(ids)}")
PY
    )

    if echo "$verdict" | grep -q "^PASS"; then
        pass "annotation_issues structure validated. $verdict"
    else
        fail "annotation_issues validation failed. $verdict. Content: $(truncate "$text" 260)"
    fi
}
run_test_31_8
