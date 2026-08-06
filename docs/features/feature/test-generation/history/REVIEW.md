---
feature: test-generation
type: review
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Spec Review: Test Generation Feature

## Review Metadata

| Field | Value |
|-------|-------|
| **Review Date** | 2026-01-29 |
| **Reviewer** | Principal Engineer Agent (Claude Opus 4.5) |
| **Spec Version** | v1.0 (proposed) |
| **Files Reviewed** | product-spec.md, tech-spec.md, architecture.md |

---

## 1. Performance

### Critical Issues

#### P1-1: 30-second batch timeout is too long for MCP tool calls
- TECH_SPEC specifies 30-second timeout for `test_heal.batch` and `test_classify.batch`
- MCP clients (Claude Code, Cursor) typically expect responses within 10-15 seconds
- Long-running operations will timeout the HTTP response layer
- **Resolution required**: Either reduce timeout to align with existing 10-second async command pattern, or explicitly implement correlation_id async pattern

#### P1-2: Test file parsing memory usage unspecified
- `test_heal.batch` can process "all broken tests in directory"
- No maximum file count or total file size limit specified
- Large test suites (100+ files, 10MB+ total) could cause memory pressure
- **Resolution required**: Add explicit limits: max files per batch, max file size, total batch size cap

### Medium Issues

#### P2-1: DOM snapshot caching TTL (10 seconds) may cause stale selector matches
- If user is actively modifying DOM during healing session, cached snapshot won't reflect changes
- **Suggestion**: Add `force_refresh` parameter to bypass cache

#### P2-2: "< 1 second per selector" timing claim is unsubstantiated
- Complex DOM trees (10,000+ nodes) may exceed 1 second for structural matching
- **Suggestion**: Add caveat about page complexity or implement early-exit on timeout

### Minor Issues

#### P3-1: Screenshot base64 in test_classify.failure could be very large
- A 4K screenshot could be 5-10MB base64
- **Suggestion**: Set explicit size limit (100KB) or recommend thumbnails

---

## 2. Concurrency

### Critical Issues

#### C1-1: Async pattern for batch operations incompletely specified
- TECH_SPEC table shows batch operations use "Async (correlation_id)" pattern
- But the spec doesn't detail how correlation_id is returned or polled
- **Resolution required**: Either use existing pending query infrastructure or design new async pattern

#### C1-2: No concurrency limit for simultaneous test generation requests
- Multiple AI clients could simultaneously request test generation
- **Resolution required**: Add per-client or global concurrency limit

### Medium Issues

#### C2-1: Race condition possible during test file reading/healing
- File could change between read and write if user is editing in IDE
- **Suggestion**: Use file locking or implement optimistic concurrency

---

## 3. Data Contracts

### Critical Issues

#### D1-1: error_id reference scheme undefined
- `test_from_context.error` uses `error_id` parameter but no error ID system exists
- **Resolution required**: Define error ID generation scheme or use index-based reference

#### D1-2: Response schemas don't align with existing mcpJSONResponse pattern
- Existing tools use `mcpJSONResponse(summary, data)` format
- **Resolution required**: Adapt response format to match existing pattern

### Medium Issues

#### D2-1: TestFromContextRequest.Context enum values inconsistent with implementation
- Need clarity: Is it `generate({what:"test_from_context", context:"error"})` or `generate({what:"test_from_context.error"})`?

#### D2-2: Generated test output format inconsistent with existing generate.test
- Existing returns plain script; proposed returns JSON wrapper
- **Suggestion**: Maintain consistency or explicitly deprecate old format

---

## 4. Error Handling

### Critical Issues

#### E1-1: Test file path validation missing security specification
- Existing pattern uses `validatePathInDir()` but TECH_SPEC doesn't specify project directory definition
- **Resolution required**: Specify path validation rules and reuse existing function

### Medium Issues

#### E2-1: Error codes incomplete
- Missing: `test_file_write_failed`, `framework_not_supported`, `context_expired`
- **Suggestion**: Add these error codes

#### E2-2: Graceful degradation for missing network data underspecified
- **Suggestion**: Specify TODO comment format

---

## 5. Security

### Critical Issues

#### S1-1: Test file write capability is high-risk without AI Web Pilot toggle
- `test_heal.repair` with `auto_apply: true` writes to test files
- File write operations bypass AI Web Pilot protection
- **Resolution required**: Clarify security model for file writes

#### S1-2: Selector injection via broken_selectors parameter
- Malicious selector could potentially cause issues if echoed unsanitized
- **Resolution required**: Validate/sanitize selector input before DOM query

### Medium Issues

#### S2-1: Secret detection regex patterns may have false negatives
- Missing: AWS keys, GitHub tokens, private keys
- **Suggestion**: Expand pattern list

---

## 6. Maintainability

### Critical Issues

#### M1-1: File organization conflicts with existing patterns
- Existing pattern uses single file per feature (e.g., `pilot.go`, `codegen.go`)
- **Resolution required**: Either merge into single `testgen.go` or justify split

### Medium Issues

#### M2-1: Existing generate.test functionality overlap
- Creates two ways to generate tests with unclear differentiation
- **Suggestion**: Deprecate old or clearly document when to use which

---

## 7. Architecture

### Tool Constraint Compliance

**APPROVED**: Feature correctly extends existing `generate` tool rather than creating new tool.

### Integration with Existing Infrastructure

#### Positive:
1. Correctly identifies reuse of `codegen.go` for Playwright generation
2. DOM query infrastructure is appropriate for selector healing
3. Error context from `observe` provides needed data

#### Concerns:
1. `test_heal` requires reading external test files - new capability
2. `error_id` concept requires new infrastructure

---

## Sign-Off

| Status | Details |
|--------|---------|
| **NEEDS REVISION** | Resolve 10 Critical issues before implementation |

### Critical Issues Requiring Resolution

| ID | Issue | Resolution Path |
|----|-------|-----------------|
| P1-1 | 30-second batch timeout too long | Reduce to 10s or implement proper async pattern |
| P1-2 | No batch size limits | Add max files, max file size limits |
| C1-1 | Async pattern incomplete | Detail correlation_id flow |
| C1-2 | No concurrency limit | Add limit for generate ops |
| D1-1 | error_id scheme undefined | Define ID generation |
| D1-2 | Response format inconsistent | Adapt to mcpJSONResponse pattern |
| E1-1 | Path validation unspecified | Reuse validatePathInDir |
| S1-1 | File write without toggle | Decide on security model |
| S1-2 | Selector injection risk | Validate selector input |
| M1-1 | File organization unclear | Decide single file vs split |

### Recommended Next Steps

1. Address critical issues in updated TECH_SPEC
2. Define error_id scheme (recommend: timestamp + message hash)
3. Use existing concurrency patterns from pilot.go
4. Re-submit for final approval

---

## Key Implementation Files

- `cmd/browser-agent/testgen.go` — Existing Playwright generation to extend
- `cmd/browser-agent/tools_core.go` — Tool dispatch and response helpers
- `cmd/browser-agent/queries.go` — Pending query infrastructure
- `extension/lib/analysis/dom-queries.js` — DOM query execution
- `cmd/browser-agent/ai_persistence.go` — Path validation pattern
