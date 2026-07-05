---
status: shipped
scope: feature/redaction-patterns/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-redaction-patterns
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Redaction Patterns

> QA plan for the Configurable Redaction Patterns feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the redaction engine effectively prevents sensitive data from reaching AI clients via MCP tool responses. This is the most security-critical feature in Kaboom -- a failure here directly exposes sensitive data.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Redaction bypass via JSON encoding | Sensitive data in JSON-encoded strings (e.g., `\"Bearer abc\"` with escaped quotes) must still be matched | critical |
| DL-2 | Redaction bypass via Unicode variants | Sensitive data using Unicode lookalikes (e.g., fullwidth digits) should not bypass patterns | high |
| DL-3 | Partial redaction reveals sensitive data | Mask strategy with `show_first: 4, show_last: 4` on an 8-char string must mask entirely, not show all chars | critical |
| DL-4 | Test action exposes matched text | `test` action response includes `matched_text` -- this is intentional for testing but must NOT contain real production data | high |
| DL-5 | Hash strategy reversibility | SHA-256 truncated to 8 chars must still be one-way; verify collision resistance is acceptable | medium |
| DL-6 | Pattern stats leak sensitive info | `RedactionStats.by_pattern` counts are safe, but pattern names or IDs must not encode sensitive data | low |
| DL-7 | Custom replacement string leaks data | Named group expansion (`$name`) must not expand to sensitive captured content from unintended groups | high |
| DL-8 | Disabled builtin allows data through | When a builtin pattern like `bearer-token` is disabled, Bearer tokens flow through to AI clients | critical |
| DL-9 | Pattern ordering allows bypass | Lower-priority pattern redacts PART of sensitive data, higher-priority pattern then fails to match the modified string | high |
| DL-10 | Fallback to string redaction on malformed JSON | When JSON parsing fails, string-level redaction must still catch patterns | critical |
| DL-11 | Field-path-only pattern misses global occurrences | Pattern targeting `$.user.ssn` must still work; but sensitive data in OTHER fields is NOT caught by this pattern | high |
| DL-12 | Cache serves stale (unredacted) results after pattern change | When patterns are updated, cache must be invalidated immediately | critical |
| DL-13 | Concurrent redaction race condition | Pattern update during active redaction must not produce partially-redacted output | critical |
| DL-14 | Redaction not applied to all MCP tool responses | Verify redaction applies to `observe`, `generate`, `configure`, and `interact` tool outputs | critical |

### Negative Tests (must NOT leak)
- [ ] `observe` tool response for logs containing `Bearer eyJhbGciOiJ...` shows `[REDACTED:bearer-token]` instead
- [ ] `observe` tool response for logs containing `AKIA0123456789ABCDEF` shows `[REDACTED:aws-key]` instead
- [ ] `observe` tool response for network bodies containing credit card `4111-1111-1111-1111` shows `4111-****-****-1111` (masked)
- [ ] `observe` tool response for logs containing `123-45-6789` (SSN) shows masked output
- [ ] After adding a custom pattern for `ACC-[0-9]{8}`, subsequent tool responses redact matching values
- [ ] After disabling a builtin pattern, re-enabling it immediately resumes redaction
- [ ] Malformed JSON input still has string-level patterns applied
- [ ] After calling `clear` action, only builtins remain active (custom patterns removed)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading redaction-related responses can understand what was redacted and how to configure patterns.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Redacted content is clearly marked | Replacement strings follow consistent format: `[REDACTED:name]`, `[HASH:name:hex]`, or partial mask | [ ] |
| CL-2 | Pattern IDs are self-documenting | IDs follow format `builtin_<name>` or `user_<name>_<random>` | [ ] |
| CL-3 | Error messages for invalid regex are clear | Message includes what part of the regex is invalid, not just "compilation failed" | [ ] |
| CL-4 | PCRE-only feature errors are specific | Error distinguishes RE2 limitation from syntax error (e.g., "lookbehind not supported in RE2") | [ ] |
| CL-5 | Strategy names are self-documenting | `mask`, `hash`, `remove` are intuitive without documentation | [ ] |
| CL-6 | Test action output clearly shows transformations | `matches` array shows `matched_text`, `position`, and `replacement` for each match | [ ] |
| CL-7 | List action distinguishes builtins from custom | `source` field values `"builtin"` vs `"user"` are unambiguous | [ ] |
| CL-8 | Priority semantics are clear | Higher number = higher priority is documented and consistent | [ ] |
| CL-9 | Field path syntax is JSON-standard | Uses JSONPath syntax starting with `$.` | [ ] |
| CL-10 | Removal vs disabling distinction is clear | Error message for removing builtin suggests using `disable` instead | [ ] |
| CL-11 | Stats distinguish total vs per-pattern counts | `total_redactions` and `by_pattern` map are clearly nested under `stats` | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Risk: LLM interprets `[REDACTED:aws-key]` as the literal string rather than a redaction placeholder -- verify consistent formatting helps distinguish
- [ ] Risk: LLM tries to remove a builtin pattern instead of disabling it -- verify error message explicitly suggests `disable` action
- [ ] Risk: LLM confuses `priority: 100` (high) with `priority: -100` (low) -- verify priority semantics are documented in tool description
- [ ] Risk: LLM assumes `test` action persists the tested patterns -- verify `test` response clearly states no patterns were added
- [ ] Risk: LLM treats `match_count: 0` as "pattern is broken" rather than "no matches yet" -- verify context in list response
- [ ] Risk: LLM sends real sensitive data in `test_input` field -- verify `test` action does not log or store the test input

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Use default redaction | 0 steps: builtins active by default | No -- already zero-config |
| Add custom pattern | 1 MCP call: `configure_redaction` with `action: "add"` | No -- already minimal |
| Test pattern before adding | 2 MCP calls: `test` then `add` | Could combine into `add` with `dry_run` flag |
| Disable a builtin | 1 MCP call: `configure_redaction` with `action: "disable"` | No -- already minimal |
| List all patterns | 1 MCP call: `configure_redaction` with `action: "list"` | No -- already minimal |
| Update pattern priority | 1 MCP call: `configure_redaction` with `action: "update"` | No -- already minimal |
| Remove all custom patterns | 1 MCP call: `configure_redaction` with `action: "clear"` | No -- already minimal |
| Configure field-targeted redaction | 1 MCP call with `field_paths` parameter | No -- already a single call |

### Default Behavior Verification
- [ ] All builtin patterns are loaded and enabled at server startup with zero configuration
- [ ] Built-in patterns have priority 100 (above default custom pattern priority 0)
- [ ] Default mask config uses `show_first: 4, show_last: 4, mask_char: "*"`
- [ ] Default strategy for custom patterns is `remove` if not specified
- [ ] Patterns are applied to ALL MCP tool responses by default (global scope)
- [ ] `test` action does not modify any state
- [ ] `clear` action preserves all builtin patterns

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Valid RE2 pattern compiles | `"ACC-[0-9]{8}"` | Compiled regex, no error | must |
| UT-2 | PCRE lookahead fails with clear error | `"(?=.*secret)"` | Error: "lookahead not supported" | must |
| UT-3 | PCRE lookbehind fails with clear error | `"(?<=prefix)data"` | Error: "lookbehind not supported" | must |
| UT-4 | Empty pattern rejected | `""` | Error: "pattern cannot be empty" | must |
| UT-5 | Pattern exceeding max length rejected | 1025-char pattern | Error: "exceeds maximum length" | must |
| UT-6 | Named groups extracted correctly | `"EMP(?P<dept>[A-Z]{2})-[0-9]{6}"` | Group `dept` captured | must |
| UT-7 | Mask strategy: normal string | `"ACC-12345678"`, show_first=4, show_last=2 | `"ACC-****78"` | must |
| UT-8 | Mask strategy: short string (length <= show_first + show_last) | `"ABC"`, show_first=4, show_last=4 | `"***"` (fully masked) | must |
| UT-9 | Mask strategy: exact boundary string | `"ABCDEFGH"`, show_first=4, show_last=4 | `"********"` (fully masked, 8 = 4+4) | must |
| UT-10 | Mask strategy: default config (nil) | `"secretvalue123"`, config=nil | Uses defaults: show_first=4, show_last=4 | must |
| UT-11 | Mask strategy: custom mask char | `"ACC-12345678"`, mask_char="#" | `"ACC-####78"` | should |
| UT-12 | Hash strategy: deterministic output | Same input twice | Same hash both times | must |
| UT-13 | Hash strategy: different outputs for different inputs | `"secret1"` vs `"secret2"` | Different hashes | must |
| UT-14 | Hash strategy: format | `"secret123"`, pattern_name="test" | `"[HASH:test:a1b2c3d4]"` (8-char hex) | must |
| UT-15 | Remove strategy | Match with pattern name "aws-key" | `"[REDACTED:aws-key]"` | must |
| UT-16 | Custom replacement overrides strategy | `replacement: "HIDDEN"`, strategy: "mask" | `"HIDDEN"` (custom wins) | must |
| UT-17 | Named group expansion in replacement | `replacement: "EMP-$dept"`, match with dept="HR" | `"EMP-HR"` | must |
| UT-18 | Named group expansion with braces | `replacement: "EMP-${dept}"` | Same as `$dept` | must |
| UT-19 | Field path `$.user.ssn` matches exact path | JSON: `{"user":{"ssn":"123-45-6789"}}` | SSN field redacted, other fields untouched | must |
| UT-20 | Field path `$.users[*].email` matches array | JSON: `{"users":[{"email":"a@b.com"},{"email":"c@d.com"}]}` | Both emails redacted | must |
| UT-21 | Field path with wildcard `$.*.secret` | JSON: `{"a":{"secret":"x"},"b":{"secret":"y"}}` | Both secrets redacted | must |
| UT-22 | Non-matching field path leaves content unchanged | JSON: `{"user":{"name":"John"}}`, path `$.user.ssn` | No changes | must |
| UT-23 | Nested field paths | JSON: `{"a":{"b":{"c":"secret"}}}`, path `$.a.b.c` | Nested field redacted | must |
| UT-24 | Higher priority pattern matches first | Priority 50 and priority 10 both match | Priority 50 redacts first | must |
| UT-25 | Already-redacted content not re-matched | After priority 50 redacts, priority 10 pattern | Skips already-redacted range | must |
| UT-26 | Equal priority processes FIFO | Two priority-0 patterns | First added pattern matches first | must |
| UT-27 | Disabled pattern skipped | Pattern with `enabled: false` | Not applied to input | must |
| UT-28 | All builtins loaded at startup | Check pattern count | >= 10 builtin patterns present | must |
| UT-29 | Builtin removal returns error | `action: "remove"`, `pattern_id: "builtin_ssn"` | Error: "Cannot remove built-in pattern" | must |
| UT-30 | Builtin disable works | `action: "disable"`, `pattern_id: "builtin_ssn"` | Pattern disabled, SSN passes through | must |
| UT-31 | Disabled builtin re-enable | `action: "enable"`, `pattern_id: "builtin_ssn"` | Pattern re-enabled, SSN redacted again | must |
| UT-32 | Luhn validation: valid card number | `4111111111111111` | Redacted (Luhn valid) | must |
| UT-33 | Luhn validation: invalid number | `4111111111111112` | NOT redacted (Luhn invalid) | must |
| UT-34 | `pathMatches` with array wildcard | `$.users[0].email` vs `$.users[*].email` | Match | must |
| UT-35 | `pathMatches` with key wildcard | `$.foo.bar` vs `$.*.bar` | Match | must |
| UT-36 | ID generation format | Auto-generated ID | Format: `user_<name>_<6chars>` | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | `add` action creates pattern | MCP tool + RedactionManager | Pattern added, returned in list | must |
| IT-2 | `update` action modifies pattern | MCP tool + RedactionManager | Pattern updated, partial update supported | must |
| IT-3 | `remove` action deletes custom pattern | MCP tool + RedactionManager | Pattern removed, no longer in list | must |
| IT-4 | `list` action returns all patterns | MCP tool + RedactionManager | Both builtins and custom patterns with stats | must |
| IT-5 | `test` action shows matches | MCP tool + RedactionManager | Matches shown with positions and replacements | must |
| IT-6 | `test` action does not persist | MCP tool + RedactionManager | Pattern count unchanged after test | must |
| IT-7 | `clear` action removes custom only | MCP tool + RedactionManager | Custom patterns gone, builtins remain | must |
| IT-8 | Pattern applied to `observe` response | Add pattern + call `observe` | Matching data redacted in response | must |
| IT-9 | Pattern applied to `generate` response | Add pattern + call `generate` | Matching data redacted in response | must |
| IT-10 | Pattern persists across tool calls | Add pattern, call tool 3 times | Pattern active in all 3 responses | must |
| IT-11 | Redaction stats updated | Add pattern, trigger matches, call `list` | `match_count` incremented | must |
| IT-12 | Redaction audit entry created | Pattern matches in tool response | Audit entry with pattern name, field path, char count | must |
| IT-13 | `RedactJSON` with malformed JSON fallback | Invalid JSON input | Falls back to `RedactString`, patterns still applied | must |
| IT-14 | Multiple patterns with different strategies | Add mask + hash + remove patterns | Each applies its own strategy correctly | must |
| IT-15 | Pattern with field_paths only targets those fields | Add field-targeted pattern | Only specified fields redacted, global strings untouched | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | String redaction: 50KB response, 20 patterns | Latency | < 5ms | must |
| PT-2 | JSON redaction with field paths: 50KB response | Latency | < 10ms | must |
| PT-3 | Pattern test: 1KB input, all patterns | Latency | < 1ms | must |
| PT-4 | Pattern compilation | Time per pattern | < 5ms | must |
| PT-5 | Adding 100 patterns in one call | Total time | < 50ms | must |
| PT-6 | Cache hit return | Latency | < 0.1ms | must |
| PT-7 | Cache invalidation after pattern change | Time to clear + re-populate | < 1ms | should |
| PT-8 | Memory usage: 100 patterns + cache | Total memory | < 500KB | must |
| PT-9 | RE2 on adversarial input (catastrophic backtracking test) | Latency with `(a+)+` equivalent | Linear time guaranteed by RE2 | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty input string | `""` | Returns empty string, no error | must |
| EC-2 | No patterns configured | Input with sensitive data, all patterns disabled | Returns input unchanged | must |
| EC-3 | Malformed JSON falls back to string | `{invalid json "Bearer token123"` | `Bearer token123` still caught by string redaction | must |
| EC-4 | Very long match (> 10KB) | 10KB Base64 JWT token | Entire token redacted, no truncation | must |
| EC-5 | Unicode in patterns and content | Pattern matching Japanese text | RE2 handles Unicode correctly | must |
| EC-6 | Concurrent pattern updates and redaction | Update patterns while redaction is in progress | Thread-safe, no panic or data race | must |
| EC-7 | Pattern with only field paths on global string input | Field-targeted pattern + string-level redaction call | Field-targeted pattern skipped for global strings | must |
| EC-8 | Pattern with no matches | Pattern that matches nothing in current session | No error, `match_count: 0` | must |
| EC-9 | Response at maximum size (1MB) | 1MB MCP tool response | Redacted within performance bounds | must |
| EC-10 | Pattern name at max length (64 chars) | 64-char kebab-case name | Accepted | must |
| EC-11 | Pattern name with invalid characters | `"My Pattern!"` (spaces, exclamation) | Error: invalid name format | must |
| EC-12 | Duplicate pattern name | Two patterns with same name | Second gets unique ID, both coexist | should |
| EC-13 | Max patterns reached (100) | Add 101st custom pattern | Error: "maximum custom patterns reached" | must |
| EC-14 | Field path at max count (20) | Pattern with 21 field paths | Error: "maximum field paths exceeded" | must |
| EC-15 | Overlapping regex matches | Two patterns that could match overlapping text | First (higher priority) takes precedence | must |
| EC-16 | Cache eviction under pressure | 1000+ unique inputs fill cache | Half evicted, new entries added correctly | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application that logs or displays known sensitive patterns (test data: fake SSN, fake credit card, fake API key)

### Built-in Pattern Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human triggers console log: `console.log("Bearer eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoxfQ.abc123")` | Browser console | Log appears in browser console | [ ] |
| UAT-2 | AI calls: `{"tool": "observe", "params": {"category": "logs"}}` | MCP response | Log entry shows `[REDACTED:bearer-token]` instead of the JWT | [ ] |
| UAT-3 | Human triggers: `console.log("Key: AKIA0123456789ABCDEF")` | Browser console | AWS key visible in browser | [ ] |
| UAT-4 | AI calls: `{"tool": "observe", "params": {"category": "logs"}}` | MCP response | Log shows `[REDACTED:aws-key]` | [ ] |
| UAT-5 | Human triggers: `console.log("SSN: 123-45-6789")` | Browser console | SSN visible in browser | [ ] |
| UAT-6 | AI calls observe | MCP response | SSN is masked (partial reveal) | [ ] |

### Custom Pattern Management

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-7 | AI lists patterns: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "list"}}` | MCP response | All builtin patterns listed with `source: "builtin"` | [ ] |
| UAT-8 | AI adds custom pattern: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "add", "patterns": [{"name": "internal-id", "pattern": "CUST-[0-9]{8}", "strategy": "mask", "mask_config": {"show_first": 5, "show_last": 2}}]}}` | MCP response | Pattern added, ID returned | [ ] |
| UAT-9 | Human triggers: `console.log("Customer: CUST-98765432")` | Browser console | Customer ID visible in browser | [ ] |
| UAT-10 | AI calls observe | MCP response | Shows `CUST-***32` (masked per config) | [ ] |
| UAT-11 | AI tests a new pattern: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "test", "test_input": "Order ORD-123456 for CUST-98765432"}}` | MCP response | Shows matches for existing `internal-id` pattern on the CUST value | [ ] |
| UAT-12 | AI disables builtin SSN pattern: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "disable", "pattern_id": "builtin_ssn"}}` | MCP response | Pattern disabled confirmation | [ ] |
| UAT-13 | Human triggers: `console.log("SSN: 123-45-6789")` | Browser console | SSN visible | [ ] |
| UAT-14 | AI calls observe | MCP response | SSN is NOT redacted (pattern disabled) | [ ] |
| UAT-15 | AI re-enables: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "enable", "pattern_id": "builtin_ssn"}}` | MCP response | Pattern re-enabled | [ ] |
| UAT-16 | AI calls observe again | MCP response | SSN is redacted again | [ ] |

### Field-Targeted Redaction

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-17 | AI adds field-targeted pattern: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "add", "patterns": [{"name": "user-email", "pattern": "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}", "strategy": "hash", "field_paths": ["$.user.email", "$.users[*].email"]}]}}` | MCP response | Pattern added with field paths | [ ] |
| UAT-18 | AI calls observe for network data containing user objects | MCP response | Email fields in targeted paths are hashed; email addresses in other contexts (e.g., log messages) are NOT caught by this pattern | [ ] |

### Error Handling

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-19 | AI tries to add pattern with PCRE lookahead: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "add", "patterns": [{"name": "bad-pattern", "pattern": "(?=.*secret)data"}]}}` | MCP response | Clear error: "lookahead not supported in RE2" | [ ] |
| UAT-20 | AI tries to remove builtin: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "remove", "pattern_id": "builtin_aws-key"}}` | MCP response | Error: "Cannot remove built-in pattern. Use 'disable' to turn it off." | [ ] |
| UAT-21 | AI clears all custom patterns: `{"tool": "configure", "params": {"action": "configure_redaction", "redaction_action": "clear"}}` | MCP response | `removed_count` shows custom patterns removed, `remaining_builtin_count` shows builtins preserved | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Bearer token not in observe response | Trigger log with Bearer token, call observe | `[REDACTED:bearer-token]` placeholder shown | [ ] |
| DL-UAT-2 | AWS key not in observe response | Trigger log with AWS key, call observe | `[REDACTED:aws-key]` placeholder shown | [ ] |
| DL-UAT-3 | Credit card masked correctly | Trigger log with card number, call observe | Card masked with first/last 4 visible | [ ] |
| DL-UAT-4 | Custom pattern works on network bodies | Network response containing custom pattern match | Redacted in `observe` network body output | [ ] |
| DL-UAT-5 | Hash strategy is deterministic | Same input produces same hash in multiple responses | Hash values match across calls | [ ] |
| DL-UAT-6 | Test action does not persist | Call `test`, then `list` | Pattern count unchanged | [ ] |
| DL-UAT-7 | Disabled builtin allows data through (intentional) | Disable `bearer-token`, trigger log | Bearer token visible in observe (by design -- verify disabling was intentional) | [ ] |

### Regression Checks
- [ ] Existing `observe` tool returns data when no custom patterns are added (builtins only)
- [ ] Adding custom patterns does not affect performance of tool responses (< 5ms overhead on typical responses)
- [ ] Removing a custom pattern immediately stops it from being applied
- [ ] Server restart clears custom patterns but reloads builtins
- [ ] Redaction cache does not serve stale results after pattern changes
- [ ] Concurrent MCP tool calls during pattern updates do not cause data races

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
