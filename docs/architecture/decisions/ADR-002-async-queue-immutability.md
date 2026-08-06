---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# ADR-002: Async Queue Architecture is Immutable

**Status**: ACCEPTED (2026-02-02)
**Deciders**: Engineering Team
**Supersedes**: None
**Related**: [ADR-001: Async Queue Pattern](ADR-001-async-queue-pattern.md)

---

## Context

On 2026-02-02, we experienced a critical production incident where the async queue-and-poll implementation was **deleted** during Phase 4b refactoring (commit 12c6a02). This caused:

- **100% failure rate** on the "UNSTABLE" branch
- All `interact()` commands timing out
- No mechanism for AI to control the browser
- Complete loss of correlation ID tracking

The deletion went undetected until production deployment, revealing gaps in our architectural safeguards.

## Problem Statement

**How do we prevent critical architecture from being deleted in future refactors?**

The async queue-and-poll pattern is **FOUNDATIONAL**:
- Without it, ALL browser automation fails
- It's not "just another feature" - it's the **core communication primitive**
- Deletion is catastrophic, not degrading

## Decision

We declare the async queue-and-poll architecture **IMMUTABLE** and enforce it with **multiple defense layers**:

### Layer 1: Pre-Commit Hook (Immediate Feedback)

`.git/hooks/pre-commit` blocks commits that:
- Delete canonical queue, sync, observe, interact, or bridge owners
- Introduce stub implementations
- Remove required methods

**Strength**: Catches issues BEFORE commit
**Weakness**: Local only, can be bypassed with `--no-verify`

### Layer 2: Integration Tests (Compilation Guarantee)

`internal/capture/async_queue_integration_test.go` exercises the **full flow**:
```
MCP → CreatePendingQuery → Extension polls → Extension executes → SetQueryResult → GetCommandResult
```

If ANY component is deleted, the test **fails to compile or run**.

**Strength**: Cannot be bypassed, runs in CI
**Weakness**: Developers might disable failing tests

### Layer 3: Architecture Validation Script (CI Enforcement)

`scripts/validate-architecture.sh` checks:
- Canonical post-refactor owner existence across `internal/queries`,
  `internal/capture`, `internal/toolobserve`, `internal/toolinteract`, and
  `internal/bridge`
- Query and command lifecycle method existence
- Real observe command-store use and absence of empty query stubs
- Executable integration and no-facade contracts
- Bounded timeout and queue-capacity constants

**Runs in CI** - PRs cannot merge if validation fails.

**Strength**: Automated, comprehensive, clear error messages
**Weakness**: Adds ~5 seconds to CI runtime

### Layer 4: Documentation (Human Context)

- [ADR-002](ADR-002-async-queue-immutability.md) - This document (WHY it's immutable)
- [async-queue-correlation-tracking.md](../../core/async-queue-correlation-tracking.md) - Implementation details
- Inline comments in critical files referencing ADR-002

**Strength**: Provides context for future developers
**Weakness**: Easily ignored

### Layer 5: Type System Enforcement (Future)

Define required interfaces that force implementation:

```go
// AsyncQueue defines the IMMUTABLE async queue contract.
// DO NOT modify this interface without ADR approval.
// See: docs/architecture/decisions/ADR-002-async-queue-immutability.md
type AsyncQueue interface {
    CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration, clientID string) string
    GetPendingQueries() []PendingQueryResponse
    SetQueryResult(id string, result json.RawMessage)
    // ... all required methods
}
```

If `Capture` doesn't implement `AsyncQueue`, compilation fails.

**Strength**: Compile-time enforcement
**Weakness**: Requires design changes, adds complexity

## Consequences

### Positive

✅ **Multiple failure points** - To break the architecture, you'd need to:
1. Bypass pre-commit hook (`--no-verify`)
2. Disable or delete integration tests
3. Modify CI to skip validation
4. Ignore failing CI checks
5. Merge without review

✅ **Clear error messages** - Each layer provides actionable guidance:
```
❌ COMMIT BLOCKED: Critical file 'internal/capture/queries.go' is missing!
   See docs/architecture/decisions/ADR-002-async-queue-immutability.md
   Or ask: 'How do I restore the async queue implementation?'
```

✅ **Self-documenting** - Scripts and tests explain WHY things exist

✅ **Low maintenance** - Once set up, runs automatically

### Negative

❌ **Adds friction** - Legitimate refactors must update all layers
❌ **CI overhead** - +5 seconds per build
❌ **False sense of security** - Determined developers can still bypass

### Neutral

⚡ **Cultural shift** - Teaches that some code is "special"
⚡ **Documentation burden** - Must keep ADRs up to date

## Enforcement Policy

### Required Before Merge

ALL PRs must pass:
1. ✅ Pre-commit hook (local)
2. ✅ Integration tests (CI)
3. ✅ Architecture validation (CI)
4. ✅ Code review with architecture checklist

### When to Update

Update enforcement layers when:
- Adding new critical files (update `validate-architecture.sh`, pre-commit hook)
- Adding new required methods (update script, integration test)
- Changing async queue behavior (update ADR-002, tests)

### Bypass Procedure

To modify the async queue architecture:
1. Create ADR explaining WHY change is needed
2. Get approval from 2+ senior engineers
3. Update ALL enforcement layers BEFORE making changes
4. Add compensating tests for new behavior
5. Document migration path in ADR

**DO NOT** bypass hooks without ADR approval.

## Implementation Checklist

- [x] Pre-commit hook created (`.git/hooks/pre-commit`)
- [x] Integration test created (`async_queue_integration_test.go`)
- [x] Architecture validation script (`scripts/validate-architecture.sh`)
- [x] ADR-002 written (this document)
- [x] Documentation updated ([async-queue-correlation-tracking.md](../../core/async-queue-correlation-tracking.md))
- [ ] CI workflow added (`.github/workflows/architecture-validation.yml`) - TODO
- [ ] Interface-based enforcement added (future enhancement)
- [ ] Team training on bypass procedure

## Rationale

### Why NOT rely on code review alone?

- Reviewers are human, miss things
- Large refactors touch many files
- Context is lost over time
- New team members don't know what's critical

### Why NOT just use CI tests?

- Tests can be disabled "temporarily"
- Tests don't explain WHY code exists
- Tests are often ignored when failing

### Why NOT just use pre-commit hooks?

- Can be bypassed with `--no-verify`
- Only run locally, not in CI
- Don't provide context or documentation

### Why ALL layers?

**Defense in depth**. Each layer catches different failure modes:
- Pre-commit hook: Accidental deletions during local dev
- Integration tests: Breaking changes caught before CI
- CI validation: Cannot merge broken architecture
- Documentation: Provides context for future developers
- Type system (future): Compile-time guarantee

## Alternatives Considered

### Option 1: Rely on code review only
**Rejected**: Humans miss things, especially in large refactors.

### Option 2: Make async queue a separate Go module
**Rejected**: Overkill for single codebase, adds dependency complexity.

### Option 3: Use code generation to create glue code
**Rejected**: Adds build complexity, harder to debug.

### Option 4: Accept risk, fix when broken
**Rejected**: Production incidents are expensive. Prevention is cheaper than cure.

## Related Decisions

- [ADR-001: Async Queue Pattern](ADR-001-async-queue-pattern.md) - Original design decision
- [ADR-003: Correlation ID Tracking](ADR-003-correlation-id-tracking.md) - Status visibility (future)

## References

- Incident Report: Phase 4b async queue deletion (2026-02-02)
- [async-queue-correlation-tracking.md](../../core/async-queue-correlation-tracking.md)
- [async_queue_integration_test.go](internal/capture/async_queue_integration_test.go)
- [validate-architecture.sh](scripts/validate-architecture.sh)

---

## Revision History

- 2026-02-02: Initial ADR after incident
- 2026-02-02: Added Layer 5 (type system enforcement) as future enhancement

---

## Visual Guides

For visual understanding of the architecture:

- 📊 [Async Queue-and-Poll Flow Diagram](../diagrams/async-queue-flow.md) - See the full end-to-end flow
- 🎯 [Correlation ID Lifecycle](../diagrams/correlation-id-lifecycle.md) - Understand command tracking
- 🛡️ [5-Layer Protection Diagram](../diagrams/quality/5-layer-protection.md) - Visualize defense-in-depth
- 🏗️ [System Architecture](../diagrams/system-architecture.md) - See how all pieces fit together

All diagrams use Mermaid and render automatically on GitHub.
