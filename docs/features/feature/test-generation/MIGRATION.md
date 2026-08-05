---
feature: test-generation
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — Migration Plan

## Overview

This document outlines the migration plan for adding test generation capabilities to Kaboom's `generate` tool. This is a non-breaking addition that extends existing functionality.

## Version Impact

- **Current Version:** 5.2.0
- **Target Version:** 5.3.0 (minor bump - new feature, backwards compatible)
- **Breaking Changes:** None

## Implementation Phases

### Phase 1: Core Infrastructure (Priority: HIGH)

#### Files to Create:
```
cmd/browser-agent/
├── testgen.go           # Test generation from context
├── testgen_heal.go      # Selector healing
├── testgen_classify.go  # Failure classification
└── testgen_test.go      # Unit tests
```

#### Files to Modify:
```
cmd/browser-agent/tools_core.go  # Add dispatch for new generate modes
```

**Estimated LOC:** ~800-1000

**Dependencies:** None (reuses existing codegen.go)

### Phase 2: test_from_context Mode (Priority: HIGH)

#### Implementation Steps:
1. Create `TestFromContextRequest` struct
2. Implement `toolGenerateTestFromContext()` handler
3. Reuse `generatePlaywrightScript()` from codegen.go
4. Add error assertion generation
5. Add network mock generation (optional)
6. Write unit tests

#### Integration Points:
- `observe` - for error context
- `capture.Telemetry().GetAllEnhancedActions()` - for user actions
- `capture.Telemetry().NetworkBodies().Snapshot().Bodies` - for network context

### Phase 3: test_heal Mode (Priority: HIGH)

#### Implementation Steps:
1. Create `TestHealRequest` struct
2. Implement `HealSelector()` function
3. Implement DOM query for candidate elements
4. Implement confidence scoring
5. Implement test file parsing (extract selectors)
6. Write unit tests

#### Extension Integration:
- Uses existing `analyze({what: "dom"})` via pilot.go

### Phase 4: test_classify Mode (Priority: MEDIUM)

#### Implementation Steps:
1. Create `TestClassifyRequest` struct
2. Implement pattern-based classification
3. Implement evidence collection
4. Implement suggested fix generation
5. Write unit tests

**No extension changes needed** - classification is server-side pattern matching.

### Phase 5: Batch Operations (Priority: LOW)

#### Implementation Steps:
1. Implement `test_heal.batch` with directory traversal
2. Implement `test_classify.batch` with test output parsing
3. Add async pattern with correlation IDs
4. Write integration tests

## Code Changes Detail

### Generate Dispatcher Registration

```go
// cmd/browser-agent/internal/toolgenerate/dispatcher.go
"test_from_context": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
    return h.tests.HandleGenerateTestFromContext(req, args)
},
"test_heal": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
    return h.tests.HandleGenerateTestHeal(req, args)
},
"test_classify": func(h *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
    return h.tests.HandleGenerateTestClassify(req, args)
},
```

### New Error Codes

```go
// Add to error code constants
const (
    ErrNoErrorContext       = "no_error_context"
    ErrNoActionsCaptured    = "no_actions_captured"
    ErrTestFileNotFound     = "test_file_not_found"
    ErrInvalidTestSyntax    = "invalid_test_syntax"
    ErrSelectorNotParseable = "selector_not_parseable"
    ErrDOMQueryFailed       = "dom_query_failed"
    ErrClassificationUncertain = "classification_uncertain"
)
```

### Tool Schema Update

Add to `getToolSchema()` for the generate tool:

```json
{
  "type": {
    "type": "string",
    "enum": ["reproduction_script", "test", "test_from_context", "test_heal", "test_classify", ...],
    "description": "Type of content to generate"
  }
}
```

## Testing Strategy

### Unit Tests (testgen_test.go)

Run with:
```bash
go test -v ./cmd/browser-agent/ -run TestGen
```

Coverage targets:
- `GenerateTestFromError` - 90%
- `HealSelector` - 90%
- `ClassifyFailure` - 85%

### Integration Tests

```bash
go test -v ./cmd/browser-agent/ -run TestGenIntegration
```

### Manual Testing

Follow uat-guide.md for human verification.

## Rollout Plan

### Stage 1: Internal Testing
- Deploy to development environment
- Run full test suite
- Manual UAT by developers

### Stage 2: Beta Release
- Release as `5.3.0-beta.1`
- Document as experimental
- Gather feedback

### Stage 3: General Availability
- Address feedback
- Update documentation
- Release as `5.3.0`

## Rollback Plan

Since this is an additive feature with no breaking changes:

1. **Immediate rollback:** Revert to 5.2.x binary
2. **Partial rollback:** Disable new modes via feature flag (if implemented)
3. **No data migration needed** - feature doesn't create persistent state

## Documentation Updates

| File | Update Needed |
|------|---------------|
| README.md | Add test generation to feature list |
| docs/core/UAT-TEST-PLAN.md | Add test generation scenarios |
| CHANGELOG.md | Document new feature |
| .claude/docs/architecture.md | Update generate tool modes table |

## Dependencies

### Required Before Implementation
- Principal engineer review approval
- QA plan completion

### Required Before Release
- All unit tests passing
- Integration tests passing
- UAT sign-off
- Documentation updates

## Risk Assessment

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Selector healing produces wrong results | Medium | Medium | Confidence thresholds, manual review for low confidence |
| Test generation creates invalid syntax | High | Low | Syntax validation before output |
| Performance impact on large codebases | Low | Medium | Batch operation timeouts, pagination |
| Secret exposure in generated tests | High | Medium | Secret detection patterns, redaction |

## Success Metrics

After release, measure:
1. **Adoption:** % of users calling test_from_context
2. **Accuracy:** Heal confidence vs actual success rate
3. **Classification accuracy:** Manual review of category assignments
4. **User satisfaction:** Feedback and support requests

## Appendix: Competitive Parity

This feature positions Kaboom to compete with TestSprite by providing:

| Capability | TestSprite | Kaboom (After) |
|------------|------------|------------------|
| Test generation | From PRD | From captured context |
| Self-healing | Cloud-based | Local |
| Failure classification | Yes | Yes |
| Cost | $29-99/month | Free |
| Privacy | Cloud | 100% local |
