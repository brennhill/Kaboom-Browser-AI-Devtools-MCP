---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# UAT Test Quality Standard

Reference card for writing and reviewing Kaboom MCP UAT tests.
Framework: `scripts/tests/framework/framework.sh`

---

## 1. Assertion Rules

### Use structural JSON assertions, not string grep

`check_contains` proves a token exists *somewhere* in a string. It does not prove the token is a JSON key, a value, or in the correct location. A response containing `"error": "unknown field: count"` passes `check_contains "$text" "count"`.

**PREFERRED** (parse JSON, validate structure):
```bash
text=$(extract_content_text "$RESPONSE")
parsed=$(echo "$text" | jq '.')

check_json_field "$text" '.count' "0"              # exact value
check_json_has "$text" '.entries'                   # key exists
check_json_type "$text" '.entries' "array"          # correct type
check_json_array_max "$text" '.entries' 5           # limit respected
```

**BANNED for JSON validation** (string grep):
```bash
# BAD: proves nothing about JSON structure
check_contains "$text" "count"
check_contains "$text" "entries"
```

**Acceptable uses of `check_contains`**: human-readable error messages, text content that is not JSON (e.g., CSP header strings), and marker strings in roundtrip tests where the marker is globally unique (e.g., `UAT_PIPELINE_11_1`).

### Validate the MCP envelope on every tool call

Every `call_tool` response must be validated as a proper MCP envelope before inspecting content. If the envelope shape changes, tests must catch it.

```bash
RESPONSE=$(call_tool "observe" '{"what":"logs"}')

# REQUIRED: validate envelope first
if ! check_mcp_envelope "$RESPONSE"; then
    fail "Invalid MCP envelope. Response: $(truncate "$RESPONSE")"
    return
fi

# REQUIRED: validate content is parseable JSON (for JSON-returning tools)
if ! check_content_is_json "$RESPONSE"; then
    fail "Content text is not valid JSON. Response: $(truncate "$RESPONSE")"
    return
fi

# THEN inspect content fields
text=$(extract_content_text "$RESPONSE")
check_json_has "$text" '.count'
```

---

## 2. Test Structure Rules

### Every test must have exactly one of these outcomes

| Outcome | When | Function |
|---------|------|----------|
| `pass`  | All assertions passed, feature works | `pass "description"` |
| `fail`  | An assertion failed, feature is broken | `fail "description"` |
| `skip`  | Precondition not met (no extension, bridge timeout) | `skip "description"` |

**Never** count a skipped precondition as `pass`.

### Bridge timeout = SKIP, not PASS

When the extension is not connected, bridge-dependent modes (network_waterfall, accessibility, screenshot) time out. This is expected but it means the feature **was not tested**.

```bash
# GOOD: bridge timeout is SKIP
if check_bridge_timeout "$RESPONSE"; then
    skip "Bridge timeout (no extension). Cannot verify network_waterfall."
    return
fi

# BAD: bridge timeout counted as PASS
if check_bridge_timeout "$RESPONSE"; then
    pass "Got bridge timeout (expected without extension). Server did not crash."
    return
fi
```

### Poll-until-ready, not sleep

Smoke tests and daemon startup must poll for readiness. Fixed `sleep` values are flaky on slow CI.

```bash
# GOOD: poll with timeout
for i in $(seq 1 50); do
    if curl -s "http://localhost:${PORT}/health" >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

# BAD: fixed sleep
sleep 3
```

Short `sleep 0.2` after a POST before an observe read is acceptable (write propagation), but startup readiness must always poll.

---

## 3. What Must Be Tested Per Feature

Every observe mode or tool action requires these test categories:

### A. Happy path with structural validation
- Call the tool with valid params
- Validate MCP envelope (`check_mcp_envelope`)
- Validate content is JSON (`check_content_is_json`)
- Validate **each expected field** exists with correct type
- Validate at least one field has an expected value

### B. Filter/limit parameter verification
- Call with `limit:N`, verify `entries | length <= N`
- Call with filter param (min_level, url, direction, method, status_min/max), verify **all returned items** match the filter
- Verify count field reflects filtered count, not total count

```bash
# GOOD: actually verify limit works
RESPONSE=$(call_tool "observe" '{"what":"logs","limit":3}')
text=$(extract_content_text "$RESPONSE")
check_json_array_max "$text" '.entries' 3

# BAD: only checks the field exists
RESPONSE=$(call_tool "observe" '{"what":"logs","limit":3}')
text=$(extract_content_text "$RESPONSE")
check_contains "$text" "entries"   # proves nothing about limit
```

```bash
# GOOD: verify min_level filter actually filters
RESPONSE=$(call_tool "observe" '{"what":"logs","min_level":"error"}')
text=$(extract_content_text "$RESPONSE")
# Check no non-error entries leaked through
non_errors=$(echo "$text" | jq '[.entries[] | select(.level != "error")] | length')
[ "$non_errors" = "0" ]

# BAD: only checks response is not error
RESPONSE=$(call_tool "observe" '{"what":"logs","min_level":"error"}')
check_not_error "$RESPONSE"   # filter may be completely ignored
```

### C. Negative/error path tests
- Missing required param -> `check_is_error`
- Invalid param value -> `check_is_error` with helpful message
- Invalid tool name -> protocol error
- Wrong types (string where number expected) -> error, not crash

### D. Pagination tests (where applicable)
- Verify `after_cursor` / `before_cursor` return different result sets
- Verify cursor from response metadata can be used in follow-up call
- Verify empty result when paginating past end

### E. Empty state tests
- Call with no data in buffers
- Must return success (not error) with `count: 0` and empty array
- Distinguishable from error responses

---

## 4. Anti-Patterns

| Anti-Pattern | Problem | Fix |
|---|---|---|
| `check_contains "$text" "count"` for JSON | Matches `"account"`, error messages, any substring | `check_json_has "$text" '.count'` |
| Bridge timeout -> `pass` | Feature never tested, CI always green | Bridge timeout -> `skip` |
| No filter verification | `limit:5` silently ignored, returns 500 entries | `check_json_array_max "$text" '.entries' 5` |
| No envelope validation | Content block structure could change silently | `check_mcp_envelope "$RESPONSE"` on every call |
| `sleep 3` for startup | Flaky on slow CI, wastes time on fast machines | `wait_for_health` / poll loop |
| Only positive tests | Invalid input not tested, crashes not caught | Add missing-param + invalid-value tests |
| No type checking | `entries` could be string, number, null | `check_json_type "$text" '.entries' "array"` |
| `check_contains "$text" "entries"` then `pass` | Only proves 7 characters exist somewhere | Parse JSON, check type, check length |

---

## 5. Quick Reference: Framework Helpers

| Helper | Strength | Use When |
|---|---|---|
| `check_json_field(json, path, expected)` | Strong | Exact value match |
| `check_json_has(json, path)` | Strong | Field existence |
| `check_json_type(json, path, type)` | Strong | Type validation (array, object, string, number, boolean) |
| `check_json_array_max(json, path, max)` | Strong | Limit verification |
| `check_mcp_envelope(response)` | Strong | Every tool call response |
| `check_content_is_json(response)` | Strong | Every JSON-returning tool |
| `check_not_error(response)` | Medium | Happy path gate |
| `check_is_error(response)` | Medium | Negative test gate |
| `check_valid_jsonrpc(response)` | Medium | Protocol validation |
| `check_contains(haystack, needle)` | Weak | Non-JSON text, unique markers only |

---

## 6. Review Checklist

Before merging any UAT test changes, verify:

- [ ] Zero uses of `check_contains` for JSON field validation
- [ ] Every `call_tool` response validated with `check_mcp_envelope`
- [ ] Every JSON-returning tool validated with `check_content_is_json`
- [ ] Every filter param has a test that verifies filtering works (not just "accepted")
- [ ] Bridge-dependent modes use `skip` (not `pass`) on timeout
- [ ] At least one negative test per tool/mode
- [ ] No fixed `sleep` for daemon/service readiness (poll instead)
- [ ] Array fields checked with `check_json_type` for correct type
