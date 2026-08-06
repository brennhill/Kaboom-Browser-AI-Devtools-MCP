---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Example: How to Request a New Feature (Top 1% Quality)

> **Use this as a template when asking an LLM to implement a new feature**

This example demonstrates how to request a feature that maintains top 1% code quality by following the mandatory 5-gate workflow.

**See also:** [docs/standards/README.md](../standards/docs-system.md) for comprehensive implementation standards covering data models, functions, APIs, error handling, security, concurrency, testing, performance, and more.

---

## Example Feature Request: "Add Request Replay Tool"

### ❌ Bad Request (Skips Quality Process)

```
"Hey, can you add a feature to replay HTTP requests? Just add it to the MCP tools."
```

**Problems:**
- No specification (what exactly should it do?)
- No quality requirements mentioned
- No testing plan
- Skips 5-gate workflow
- Will likely produce low-quality code

---

### ✅ Good Request (Maintains Top 1% Quality)

```markdown
I want to add a new MCP tool that replays captured HTTP requests for debugging.

**Requirements:**
- Follow the mandatory 5-gate feature workflow (.claude/docs/feature-workflow.md)
- Maintain all quality standards (docs/standards/README.md)
- Run `make quality-gate` before every commit
- Keep all files under 800 lines

**Workflow to Follow:**

Gate 1: Create product-spec.md
- Define WHAT users need (problem statement, user stories, success criteria)
- Get my approval before proceeding to Gate 2

Gate 2: Create tech-spec.md
- Define HOW to implement (architecture, data structures, API design)
- Reference similar patterns in codebase
- Get my approval before proceeding to Gate 3

Gate 3: Get principal review
- Review for completeness, consistency, architecture fit
- Check: No files will exceed 800 lines
- Check: Performance targets defined
- Get my approval before proceeding to Gate 4

Gate 4: Create qa-plan.md
- Define unit tests, integration tests, UAT scenarios
- Coverage target: 90%+ for new code
- Get my approval before proceeding to Gate 5

Gate 5: Implementation with TDD
- Write failing tests FIRST
- Implement minimal code to pass tests
- Run `make quality-gate` before committing
- Refactor only if tests still pass

**Context:**
- Review existing MCP tools in cmd/browser-agent/tools_observe.go for patterns
- Check docs/features/feature-navigation.md for similar features
- Follow MCP protocol correctness rules from docs/core/protocol/mcp-correctness.md

**Quality Gates (Mandatory):**
Before implementation, confirm:
- [ ] Will all files be under 800 lines?
- [ ] Can we achieve 90%+ test coverage?
- [ ] Performance acceptable? (HTTP < 0.5ms, memory limits)
- [ ] No new production dependencies?
- [ ] Follows existing MCP tool patterns?

Let me know if you need clarification on any requirements before starting.
```

---

## Why This Works

### Sets Clear Expectations
- ✅ Explicitly mentions 5-gate workflow
- ✅ References quality standards
- ✅ Requires approval at each gate
- ✅ Defines success criteria upfront

### Prevents Common Quality Issues
- ✅ Stops "implementation first, tests later" anti-pattern
- ✅ Forces architectural thinking before coding
- ✅ Requires explicit approval gates
- ✅ References existing patterns to maintain consistency

### Enforces Quality Gates
- ✅ File length checked before implementation
- ✅ Test coverage requirement stated upfront
- ✅ Performance targets defined
- ✅ quality-gate must pass

---

## Template: Generic Feature Request

```markdown
I want to add [FEATURE NAME].

**User Story:**
As a [USER TYPE], I want to [ACTION] so that [BENEFIT].

**Requirements:**
- Follow mandatory 5-gate workflow (.claude/docs/feature-workflow.md)
- Maintain all quality standards (docs/standards/README.md)
- Run `make quality-gate` before every commit
- All files must be under 800 lines
- Test coverage: 90%+ for new code

**Workflow:**

Gate 1: Create docs/features/feature/[feature-name]/product-spec.md
- Define WHAT: Problem, user workflows, success criteria
- **Wait for my approval** before Gate 2

Gate 2: Create docs/features/feature/[feature-name]/tech-spec.md
- Define HOW: Architecture, data structures, implementation steps
- Include performance targets
- **Wait for my approval** before Gate 3

Gate 3: Principal review
- Verify: Completeness, consistency, architecture fit
- Verify: No file will exceed 800 lines
- **Wait for my approval** before Gate 4

Gate 4: Create docs/features/feature/[feature-name]/qa-plan.md
- Define: Unit tests, integration tests, UAT scenarios
- Target: 90%+ coverage
- **Wait for my approval** before Gate 5

Gate 5: Implementation with TDD
- Write tests FIRST (they should fail)
- Implement minimal code to pass tests
- Run `make quality-gate` before committing
- Refactor only if tests still pass

**Context to Review:**
- [Relevant existing code/features]
- [Similar patterns in codebase]
- [Architecture constraints]

**Quality Checklist:**
- [ ] Files under 800 lines?
- [ ] 90%+ test coverage achievable?
- [ ] Performance targets defined and achievable?
- [ ] No new dependencies?
- [ ] Follows existing patterns?

Ask me questions if anything is unclear before starting.
```

---

## Key Elements of a Quality-Focused Prompt

### 1. **Explicit Workflow Reference**
```
"Follow the mandatory 5-gate workflow (.claude/docs/feature-workflow.md)"
```
Forces structured approach, not ad-hoc coding.

### 2. **Quality Standards Upfront**
```
"Maintain all quality standards (kaboom-code-standards.md)"
"Run `make quality-gate` before every commit"
```
Sets bar high from the start.

### 3. **Gate Approval Requirements**
```
"Wait for my approval before Gate 2"
"Wait for my approval before Gate 3"
```
Prevents rushing ahead without validation.

### 4. **Measurable Constraints**
```
"All files must be under 800 lines"
"Test coverage: 90%+ for new code"
"Performance: [specific targets]"
```
Prevents quality drift through specific limits.

### 5. **Context References**
```
"Review existing MCP tools in cmd/browser-agent/tools_observe.go"
"Check docs/features/ for similar features"
```
Ensures consistency with existing codebase.

### 6. **TDD Requirement**
```
"Write tests FIRST (they should fail)"
"Implement minimal code to pass tests"
```
Prevents "code first, test later" anti-pattern.

---

## Real-World Example Prompts

### Example 1: Simple Feature (Network Request Replay)

```markdown
Add a "replay_request" action to the interact tool that replays captured HTTP requests.

**Follow 5-gate workflow:**
1. Create product-spec.md - Get approval
2. Create tech-spec.md - Get approval
3. Principal review - Get approval
4. Create qa-plan.md - Get approval
5. Implement with TDD - Run `make quality-gate`

**Quality Requirements:**
- Add to existing tools_interact.go (currently 256 lines, room for growth)
- 90%+ test coverage
- No performance impact on existing captures
- Response time: < 100ms for replay

**Context:**
- Review tools_interact.go for similar async command patterns
- Check internal/capture/network_bodies.go for request data structure
- Follow async queue pattern (docs/async-queue-correlation-tracking.md)

Start with Gate 1 (product-spec.md). Ask clarifying questions if needed.
```

### Example 2: Complex Feature (Real-Time Error Clustering)

```markdown
Add real-time error clustering to group similar errors as they occur.

**Follow 5-gate workflow (mandatory):**
- Gate 1-5 with approval at each gate
- Strict TDD (tests before code)
- Quality gate must pass

**Architectural Constraints:**
- Will touch: internal/analysis/clustering.go (may need split if >800 lines)
- New tool mode: observe tool "error_clusters" mode
- Performance: Clustering must complete in < 10ms per error
- Memory: Max 10MB for cluster storage

**Questions to Answer in Specs:**
- How many clusters maximum?
- Eviction strategy when full?
- Clustering algorithm (already exists - reuse or new?)
- API response format?

**Quality Checklist:**
- [ ] Will clustering.go stay under 800 lines after changes? (currently 863 - already over!)
- [ ] Can we achieve <10ms clustering performance?
- [ ] Test coverage for edge cases (identical errors, partial matches)?
- [ ] Memory limits enforced?

**Red Flags:**
- If clustering.go gets bigger, MUST split first
- If performance target not achievable, discuss alternatives
- If tests can't reach 90%, design may be untestable

Start with Gate 1. Create product-spec in docs/features/feature/error-clustering/
```

### Example 3: Refactoring Request (Split Large File)

```markdown
Split internal/security/security.go (1054 lines) into smaller focused files.

**This is a refactoring, not a new feature. Process:**

1. Create refactoring plan:
   - What files will be created?
   - How will code be split?
   - What could break?

2. Tag current state:
   - `git tag before-split-security-$(date +%Y%m%d)`

3. Run baseline tests:
   - `go test ./internal/security/... > tests-before.txt`
   - `go test -bench=. ./internal/security/... > bench-before.txt`

4. Split incrementally:
   - Create new files
   - Move code
   - Test after each move
   - Commit frequently

5. Verify nothing broke:
   - `bash scripts/quality/verification/verify-refactor.sh`
   - `go test ./internal/security/...`
   - Compare benchmarks

6. Run quality gate:
   - `make quality-gate`

**Target Structure:**
- security_core.go (<400 lines) - Main Security struct
- security_csp.go (<300 lines) - CSP generation
- security_sri.go (<200 lines) - SRI hashing
- security_audit.go (<300 lines) - Audit trail
- security_config.go (<200 lines) - Configuration

**Safety Measures:**
- Keep old file until new files compile
- Test after each file created
- Use `git mv` to preserve history where possible
- Document what moved where

Ask if you need me to proceed differently.
```

---

## Anti-Patterns to Avoid

### ❌ Bad: "Just Add It"
```
"Add request replay to the interact tool."
```
**Problem:** No spec, no tests, no quality gates

### ❌ Bad: "Skip the Specs"
```
"This is simple, just implement it without specs."
```
**Problem:** Even "simple" features benefit from planning

### ❌ Bad: "Tests Later"
```
"Implement the feature first, we'll add tests after."
```
**Problem:** Violates TDD, tests will be lower quality

### ❌ Bad: "Don't Worry About Quality"
```
"Just get it working, we'll refactor later."
```
**Problem:** "Later" never comes, quality degrades

---

## Pattern: Quality-First Feature Request

**Every feature request should include:**

1. **Reference to 5-gate workflow** ✅
2. **Quality standards requirement** ✅
3. **Approval gates explicitly stated** ✅
4. **Measurable quality constraints** ✅
5. **Context and existing patterns** ✅
6. **TDD requirement** ✅
7. **quality-gate requirement** ✅

**Template structure:**
```markdown
Feature: [NAME]

Requirements:
- 5-gate workflow (mandatory)
- quality-gate must pass
- TDD (tests first)
- Files under 800 lines

Workflow:
- Gate 1-4: Specs (get approval at each)
- Gate 5: TDD implementation

Quality Gates:
- [Specific constraints for this feature]

Context:
- [Existing code to review]
- [Similar patterns]

Start with Gate 1: product-spec.md
```

---

## How the LLM Should Respond

**Good LLM Response:**
```
I'll implement the request replay feature following the 5-gate workflow.

**Gate 1: Product Specification**

I'm creating docs/features/feature/request-replay/product-spec.md

[LLM writes spec]

**Questions before proceeding:**
- Should replay modify requests or replay exactly?
- How should we handle failed replays?
- Should this support batch replay?

Please review the product-spec.md. Once approved, I'll proceed to Gate 2 (tech-spec.md).
```

**Bad LLM Response:**
```
Sure! I'll add replay functionality to tools_interact.go.

[Starts writing code immediately]
```

---

## Quality-Focused Prompt Framework

### For Simple Features (< 500 lines total)

```markdown
Add [FEATURE].

**Workflow:** Follow 5 gates, get approval at each gate
**Quality:** Run `make quality-gate` before commit
**Testing:** TDD, 90%+ coverage
**Files:** All under 800 lines

Context: [Existing code to review]

Start with product-spec.md. Ask questions if unclear.
```

### For Complex Features (> 500 lines total)

```markdown
Add [FEATURE].

**Workflow:** Strict 5-gate process with principal review
**Quality:** Follow `kaboom-code-standards.md`
**Architecture:** May require new packages or file splits
**Performance:** Define targets in tech-spec
**Testing:** Unit + integration tests, 90%+ coverage
**Documentation:** Package doc.go if new package

**Pre-Implementation Questions:**
- Will this fit in existing files or need new ones?
- What's the performance budget?
- Are there similar features to learn from?

Start with Gate 1. Create detailed product-spec.md.
Don't proceed to implementation without approval at each gate.
```

### For Refactoring

```markdown
Refactor [FILE/COMPONENT].

**Process:**
1. Document current state and issues
2. Create refactoring plan with safety measures
3. Tag baseline: `git tag before-refactor-$(date +%Y%m%d)`
4. Split incrementally, test after each change
5. Run `bash scripts/quality/verification/verify-refactor.sh`
6. Run `make quality-gate`

**Safety:**
- Keep old files until new ones compile
- Test after every file created
- Preserve git history with `git mv`
- Compare benchmarks before/after

**Quality:**
- Target: All files under 800 lines
- All tests must still pass
- No performance regressions >20%

Ask if you need me to adjust the approach.
```

---

## Example Session Flow (Ideal)

**User:**
```
Add a network request replay feature to help debug API issues.

Follow 5-gate workflow, maintain top 1% quality.
```

**LLM (Gate 1):**
```
I'll create the product spec for request replay.

**Gate 1: Product Specification**

Creating docs/features/feature/request-replay/product-spec.md

[Shows spec]

**Questions:**
- Should replay modify headers (e.g., remove auth)?
- Replay to same endpoint or allow override?
- Handle rate limits how?

Review product-spec.md. Approve to proceed to tech-spec.
```

**User:** `Looks good. Proceed to Gate 2.`

**LLM (Gate 2):**
```
**Gate 2: Technical Specification**

Creating docs/features/feature/request-replay/tech-spec.md

**Architecture:**
- Add to internal/toolgenerate/dispatcher.go (currently 130 lines, room for ~100 more)
- New method: toolGenerateReplayScript()
- Reuses: internal/capture/network_bodies.go for request data

**Implementation Steps:**
1. Add MCP tool schema to tools_schema.go
2. Add handler to internal/toolgenerate/dispatcher.go
3. Add format "replay" to generate tool
4. Create tests in tools_test.go

**Files Affected:**
- cmd/browser-agent/tools_schema.go (+30 lines)
- cmd/browser-agent/internal/toolgenerate/dispatcher.go (+80 lines)
- cmd/browser-agent/tools_test.go (+50 lines)

All files stay under 800 lines ✅

Review tech-spec.md. Approve to proceed to principal review.
```

**User:** `Approved. Proceed to Gate 3.`

**LLM (Gate 3):**
```
**Gate 3: Principal Review**

Checklist:
✅ Product spec defines clear user value
✅ Tech spec is implementable
✅ Architecture fits existing patterns
✅ No files will exceed 800 lines
✅ Performance acceptable (< 0.5ms per replay)
✅ No new dependencies
✅ Tests will be feasible

Ready for QA plan. Proceed to Gate 4?
```

**User:** `Yes.`

**LLM (Gate 4):**
```
**Gate 4: QA Plan**

Creating docs/features/feature/request-replay/qa-plan.md

**Unit Tests:**
- Test replay script generation
- Test with GET/POST/PUT/DELETE methods
- Test with/without auth headers
- Test with request body
- Edge cases: Empty body, binary data, large payloads

**Integration Tests:**
- Generate replay for captured request
- Execute generated replay script
- Verify result matches expected format

**UAT:**
1. Capture API requests via extension
2. Use MCP to generate replay script
3. Execute script, verify request sent
4. Check server receives identical request

**Coverage Target:** 95% (simple generation logic)

Review qa-plan.md. Approve to start implementation.
```

**User:** `Approved. Implement with TDD.`

**LLM (Gate 5):**
```
**Gate 5: Implementation**

**Step 1: Write Failing Tests**

Creating test in cmd/browser-agent/tools_test.go:

[Shows test code]

Running: `go test ./cmd/browser-agent -run TestGenerateReplay`
Result: ❌ FAIL (expected - no implementation yet)

**Step 2: Implement Minimal Code**

[Shows implementation]

**Step 3: Verify Tests Pass**

Running: `go test ./cmd/browser-agent -run TestGenerateReplay`
Result: ✅ PASS

**Step 4: Run Quality Gate**

Running: `make quality-gate`
Result:
✅ File length: All files under 800 lines
✅ Linting: 0 errors
✅ Type safety: TypeScript strict mode passes
✅ Security: No vulnerabilities
✅ Tests: All pass (224 tests)
✅ TypeScript: All pass

**Ready to commit.**

[Commits with proper message]

Feature complete! All 5 gates passed, quality maintained.
```

---

## Key Takeaways

### To Maintain Top 1% Quality, Your Prompt Must:

1. **Explicitly require 5-gate workflow**
   - Don't assume LLM will follow it
   - State it clearly in the request

2. **Require approval at each gate**
   - "Wait for my approval before Gate X"
   - Prevents rushing to implementation

3. **State quality constraints upfront**
   - File length limits
   - Test coverage requirements
   - Performance targets

4. **Reference existing patterns**
   - Point to similar code
   - Ensures consistency

5. **Require `make quality-gate`**
   - Automated enforcement
   - No guessing about quality

6. **Enforce TDD**
   - "Write tests FIRST"
   - Not optional

---

## Bottom Line: The Magic Prompt Formula

```markdown
Add [FEATURE].

**Follow:** 5-gate workflow (get approval at each gate)
**Quality:** Run `make quality-gate` before commit
**Testing:** TDD (tests first), 90%+ coverage
**Files:** All under 800 lines
**Context:** [Similar features to review]

Start with Gate 1: product-spec.md
```

**That's it!** This simple structure ensures:
- ✅ Structured workflow (5 gates)
- ✅ Quality enforcement (quality-gate)
- ✅ Proper testing (TDD + coverage)
- ✅ File organization (length limits)
- ✅ Consistency (context references)

**Use this template for EVERY feature request to maintain top 1% quality!**
