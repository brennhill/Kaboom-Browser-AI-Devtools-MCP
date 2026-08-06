---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Examples & Templates

This directory contains examples and templates for maintaining top 1% code quality.

---

## 📋 Templates

### [feature-request-template.md](feature-request-template.md)

**Purpose:** How to request new features while maintaining quality

**Use when:**
- Asking an LLM to implement a new feature
- Creating a feature request for the team
- Onboarding new developers

**Key sections:**
- Bad vs Good request examples
- Quality-focused prompt framework
- Templates for simple/complex features/refactoring
- Real-world session flow example
- Anti-patterns to avoid

**Quick formula:**
```
Follow 5-gate workflow + Run quality-gate + TDD + Files <800 lines + Context
```

---

## 🎯 How to Use

### For Feature Requests

Copy the template from [feature-request-template.md](feature-request-template.md) and customize:

```markdown
Add [YOUR FEATURE].

**Follow:** 5 gates, get approval at each gate
**Quality:** Run `make quality-gate` before commit
**Testing:** TDD, 90%+ coverage
**Files:** All under 800 lines

Context: [Point to relevant existing code]

Start with product-spec.md.
```

### For Refactoring

Use the refactoring template:

```markdown
Refactor [FILE/COMPONENT].

**Process:** Document → Plan → Tag → Split incrementally → Verify
**Safety:** Test after each change, preserve history
**Quality:** Files under 800 lines, no test failures
**Verification:** Run `bash scripts/verify-refactor.sh`
```

---

## 📚 Additional Resources

**Process & Workflow:**
- [.claude/docs/feature-workflow.md](../../.claude/docs/feature-workflow.md) - Mandatory 5-gate process
- [.claude/docs/testing.md](../../.claude/docs/testing.md) - TDD workflow

**Quality Standards:**
- [../../kaboom-code-standards.md](../../kaboom-code-standards.md) - Project quality standards
- [../core/reliability/common-patterns.md](../core/reliability/common-patterns.md) - Canonical implementation patterns
- [../standards/README.md](../standards/README.md) - Comprehensive implementation standards (data models, functions, APIs, error handling, security, concurrency, testing, performance)

---

## 🎓 Learning Path

**New to Kaboom development?**

1. Read [feature-request-template.md](feature-request-template.md) (15 min)
2. Read [.claude/docs/feature-workflow.md](../../.claude/docs/feature-workflow.md) (10 min)
3. Review [the canonical implementation patterns](../core/reliability/common-patterns.md) (5 min)
4. Practice: Request a small feature using the template

**Result:** You'll know how to request features that maintain quality!

---

## 💡 Pro Tips

**Always:**
- ✅ State quality requirements upfront in your prompt
- ✅ Require approval at each gate
- ✅ Reference existing patterns
- ✅ Enforce TDD (tests first)
- ✅ Run `make quality-gate` before committing

**Never:**
- ❌ Skip gates or approvals
- ❌ Let LLM write code before specs
- ❌ Accept "tests later"
- ❌ Merge without quality-gate passing

---

**Last updated:** 2026-02-02
