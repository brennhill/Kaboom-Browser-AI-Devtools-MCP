---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Architecture Enforcement - Async Queue Immutability

**Last Updated**: 2026-02-02
**Status**: Active
**Coverage**: 5 layers of defense

---

## TL;DR

The async queue-and-poll architecture is **IMMUTABLE** and protected by **5 enforcement layers**:

1. 🛡️ **Pre-Commit Hook** - Blocks bad commits locally
2. 🧪 **Integration Tests** - Exercises full flow
3. 🤖 **CI Validation Script** - Comprehensive checks
4. 📋 **GitHub Actions** - Cannot merge broken architecture
5. 📚 **Documentation** - Context for future developers

**To break the architecture, you'd need to bypass ALL 5 layers.**

---

## Why Enforce Architecture?

On 2026-02-02, we deleted the async queue implementation during refactoring. This caused:
- ❌ 100% failure rate in production
- ❌ All browser automation timeouts
- ❌ Complete loss of AI ↔ extension communication

**Cost**: 4 hours of debugging + emergency fix + production downtime

**Prevention cost**: 30 minutes setup + 5 seconds per CI run

**Prevention is 480x cheaper than cure.**

---

## The 5 Layers

### Layer 1: Pre-Commit Hook 🛡️

**File**: `.git/hooks/pre-commit`

**What it checks**:
- ✅ Critical files exist ([queries.go](internal/capture/queries.go), [handlers.go](internal/capture/httpingest/handlers.go), [tools_core.go](cmd/browser-agent/tools_core.go))
- ✅ Required methods exist (CreatePendingQuery, GetCommandResult, etc.)
- ✅ No stub implementations

**When it runs**: Before every `git commit`

**Can be bypassed**: Yes (`git commit --no-verify`)

**Strength**: Immediate feedback, catches accidents

**Example output**:
```bash
❌ COMMIT BLOCKED: Critical file 'internal/capture/queries.go' is missing!

   This file implements the async queue-and-poll pattern.
   Without it, ALL interact() commands will timeout.

   See docs/architecture/decisions/ADR-002-async-queue-immutability.md
```

**Install**:
```bash
# Already installed, but if missing:
chmod +x .git/hooks/pre-commit
```

---

### Layer 2: Integration Tests 🧪

**File**: `internal/capture/async_queue_integration_test.go`

**What it tests**:
```
MCP creates command
  ↓ CreatePendingQueryWithTimeout()
Extension polls
  ↓ GetPendingQueries()
Extension executes
  ↓ SetQueryResult()
MCP retrieves status
  ↓ GetCommandResult()
✅ Full flow verified
```

**When it runs**: `go test ./internal/capture`

**Can be bypassed**: No (would need to delete test file)

**Strength**: Exercises real code paths, catches runtime errors

**Run locally**:
```bash
go test -v ./internal/capture -run TestAsyncQueueIntegration
```

**Example output**:
```bash
=== RUN   TestAsyncQueueIntegration
    ✅ Full async queue flow verified: create → poll → execute → result → retrieve
--- PASS: TestAsyncQueueIntegration (0.00s)
```

---

### Layer 3: Architecture Validation Script 🤖

**File**: `scripts/quality/verification/validate-architecture.sh`

**What it checks**:
1. 📁 **Critical files** - 6 files must exist
2. 🔧 **Required methods** - 14 methods in queries.go
3. 🌐 **HTTP handlers** - 4 endpoints in handlers.go
4. 🛠️ **MCP tools** - 5 tool handlers in tools_core.go
5. 🚫 **No stubs** - Real implementations only
6. 🧪 **Integration tests** - Must exist and pass
7. ⚙️ **Constants** - AsyncCommandTimeout = 30s
8. 📚 **Documentation** - ADRs and guides exist

**When it runs**:
- Manually: `./scripts/quality/verification/validate-architecture.sh`
- CI: Every PR/push

**Can be bypassed**: No (runs in CI)

**Strength**: Comprehensive, clear errors, automated

**Run locally**:
```bash
./scripts/quality/verification/validate-architecture.sh
```

**Example output**:
```bash
🏗️  Validating Kaboom architecture...

1️⃣  Checking critical files...
   ✅ internal/capture/queries.go
   ✅ internal/capture/httpingest/handlers.go
   ✅ cmd/browser-agent/tools_core.go

2️⃣  Checking required methods in queries.go...
   ✅ CreatePendingQuery
   ✅ GetCommandResult
   [... 12 more ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Architecture validation PASSED
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

### Layer 4: GitHub Actions 📋

**File**: `.github/workflows/architecture-validation.yml`

**What it does**:
- Runs validation script in CI
- Runs integration tests
- Checks for stub implementations
- Warns if critical files modified
- **Blocks PR merge** if validation fails

**When it runs**: Every PR/push to stable/UNSTABLE/develop

**Can be bypassed**: No (required status check)

**Strength**: Automated, visible in PR, blocks merge

**View results**: Check "Actions" tab in GitHub PR

**Example PR status**:
```
✅ Architecture Validation - Passed
   All critical components present and functional
```

---

### Layer 5: Documentation 📚

**Files**:
- [ADR-002-async-queue-immutability.md](../decisions/ADR-002-async-queue-immutability.md) - WHY immutable
- [async-queue-correlation-tracking.md](../../core/protocol/async-queue-correlation-tracking.md) - Implementation
- Inline comments in critical files

**What it provides**:
- Context for future developers
- Rationale for enforcement
- Instructions for modification
- Bypass procedure

**Strength**: Educates, prevents accidental changes

**Read before modifying**:
```bash
cat docs/architecture/decisions/ADR-002-async-queue-immutability.md
```

---

## Quick Reference

### ✅ I want to verify architecture locally

```bash
# Run all checks
./scripts/quality/verification/validate-architecture.sh

# Run integration tests
go test -v ./internal/capture -run TestAsyncQueueIntegration
```

### ⚠️ I need to modify the async queue

1. **Read ADR-002** - Understand why it's immutable
2. **Create new ADR** - Explain WHY change is needed
3. **Get approval** - 2+ senior engineers
4. **Update enforcement** - All 5 layers
5. **Add tests** - Cover new behavior
6. **Document migration** - Update ADRs

### 🚨 I accidentally broke the architecture

1. **Don't panic** - We have backups
2. **Check git history** - `git log --oneline -- internal/capture/queries.go`
3. **Restore from commit** - `git checkout <commit> -- internal/capture/queries.go`
4. **Run validation** - `./scripts/quality/verification/validate-architecture.sh`
5. **Run tests** - `go test ./internal/capture`

### 🔓 I need to bypass pre-commit hook

**DON'T.** But if you must:
```bash
git commit --no-verify
```

**Then immediately**:
1. Create ADR explaining why
2. Get approval before pushing
3. Update enforcement layers
4. Add compensating tests

---

## Enforcement Matrix

| Layer | Local | CI | Blockable | Compile-Time | Runtime | Context |
|-------|-------|----|-----------|--------------|---------| --------|
| Pre-commit hook | ✅ | ❌ | ✅ | ❌ | ❌ | ⚠️ |
| Integration tests | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| Validation script | ✅ | ✅ | ❌ | ❌ | ✅ | ⚠️ |
| GitHub Actions | ❌ | ✅ | ✅ | ❌ | ✅ | ⚠️ |
| Documentation | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |

**Legend**:
- ✅ = Provides this protection
- ❌ = Does not provide this protection
- ⚠️ = Partial (provides some context)

**Reading**: To break ALL protections, attacker must:
1. Bypass pre-commit (`--no-verify`)
2. Delete/disable integration tests
3. Bypass CI validation (requires admin)
4. Override required GitHub Actions status
5. Ignore documentation warnings

**This requires malicious intent, not accident.**

---

## Maintenance

### When to update

Update enforcement layers when:
- ✏️ Adding new critical files
- ✏️ Adding new required methods
- ✏️ Changing async queue behavior
- ✏️ Refactoring internal structure

### How to update

1. **Pre-commit hook**: Edit `.git/hooks/pre-commit`
   - Add to `CRITICAL_FILES` array
   - Add to method checks

2. **Integration tests**: Edit `async_queue_integration_test.go`
   - Add test case for new behavior
   - Update `TestAsyncQueueArchitectureInvariants`

3. **Validation script**: Edit `scripts/quality/verification/validate-architecture.sh`
   - Update `CRITICAL_FILES` array
   - Update `REQUIRED_METHODS` array

4. **GitHub Actions**: Usually no changes needed (runs script)

5. **Documentation**: Update ADR-002 and this file

### Testing enforcement

```bash
# Test pre-commit hook
rm internal/capture/queries.go  # Delete critical file
git add -A
git commit -m "test"  # Should block
git checkout internal/capture/queries.go  # Restore

# Test validation script
./scripts/quality/verification/validate-architecture.sh  # Should pass

# Test integration tests
go test ./internal/capture -run TestAsyncQueueIntegration  # Should pass
```

---

## FAQ

### Q: Why so many layers?

**A**: Defense in depth. Each layer catches different failure modes:
- Pre-commit: Accidental deletions during local dev
- Tests: Breaking changes caught before CI
- Script: Comprehensive validation
- GitHub Actions: Cannot merge broken code
- Docs: Context for future developers

### Q: Can I disable a layer temporarily?

**A**: Only with ADR approval. Disabling enforcement defeats the purpose.

### Q: What if I legitimately need to refactor?

**A**: Follow the bypass procedure in ADR-002:
1. Create ADR explaining change
2. Get approval
3. Update ALL enforcement layers
4. Add compensating tests

### Q: Is this overkill?

**A**: No. We lost 4 hours to production incident. Prevention is cheap.

### Q: What's the performance impact?

**A**: ~5 seconds per CI run. Negligible compared to incident cost.

### Q: Can't we just be more careful?

**A**: Humans make mistakes. Automation doesn't.

---

## Success Metrics

Since implementing enforcement (2026-02-02):
- ✅ Zero async queue deletions
- ✅ Zero architecture violations merged
- ✅ 100% CI pass rate for validation
- ✅ Zero "how do I restore this?" questions

**Goal**: Zero architecture incidents for 12 months.

---

## Support

**Questions?** Ask:
- "How do I restore the async queue implementation?"
- "Why is my commit blocked?"
- "How do I modify async queue architecture?"

**Issues?**
- Check [ADR-002](../decisions/ADR-002-async-queue-immutability.md)
- Run `./scripts/quality/verification/validate-architecture.sh`
- Check GitHub Actions logs

**Emergencies?**
- Restore from git: `git checkout <last-good-commit> -- internal/capture/`
- Deploy v5.3.0 (last known good)
- Escalate to on-call engineer

---

## Related Documents

- [ADR-001: Async Queue Pattern](../decisions/ADR-001-async-queue-pattern.md) - Original design
- [ADR-002: Async Queue Immutability](../decisions/ADR-002-async-queue-immutability.md) - This enforcement
- [async-queue-correlation-tracking.md](../../core/protocol/async-queue-correlation-tracking.md) - Implementation
- [async_queue_integration_test.go](internal/capture/async_queue_integration_test.go) - Tests

---

**Remember**: The async queue is not "just code" - it's the **foundation** of Kaboom.
Treat it like infrastructure, not like a feature.
