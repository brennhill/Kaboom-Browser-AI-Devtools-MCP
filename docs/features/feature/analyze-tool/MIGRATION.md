---
feature: analyze-tool
type: migration
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Migration: 4-Tool → 5-Tool Architecture

## Overview

This migration expands Kaboom from 4 MCP tools to 5, adding the `analyze` tool for structured analysis and auditing capabilities.

### Before:
```
observe | generate | configure | interact
```

### After:
```
observe | generate | configure | interact | analyze
```

## Rationale

The 5-tool model was designed to:
1. Keep LLM decision-making simple (fewer options)
2. Force intentional design (no kitchen-sink sprawl)
3. Maintain clear semantic boundaries

However, cramming analysis capabilities into `observe` would violate principle #3. Analysis ("what's wrong?") is semantically distinct from observation ("what happened?"). Adding a 5th tool preserves cleaner boundaries while enabling the unified debugging lifecycle.

**5 tools is still minimal** — competitors have 15+ tools. The constraint evolves from "exactly 4" to "minimal tools with clear semantic separation."

## Migration Steps

### Phase 1: Documentation Updates

#### Files to update:

1. **`.claude/docs/architecture.md`**
   - Change "4-Tool Maximum" to "5-Tool Maximum"
   - Add `analyze` to tool table
   - Document semantic boundaries between `observe` and `analyze`

2. **`claude.md`**
   - Update rule #5 from "4-Tool Maximum" to "5-Tool Maximum"
   - Add `analyze` to the tool list

3. **`README.md`** (if tool list is mentioned)
   - Add `analyze` to features

4. **MCP documentation** (if external)
   - Add `analyze` tool specification

### Phase 2: Server Implementation

#### Files to create/modify:

1. **`cmd/browser-agent/analyze.go`** (new)
   - Tool registration
   - Request handling
   - Result correlation

2. **`cmd/browser-agent/mcp.go`** (modify)
   - Add `analyze` to tool list
   - Register analyze handlers

3. **`cmd/browser-agent/pending.go`** (modify)
   - Extend pending query system for analyze requests

4. **`cmd/browser-agent/analyze_test.go`** (new)
   - Unit tests for analyze tool

### Phase 3: Extension Implementation

#### Files to create/modify:

1. **`extension/lib/analyze.js`** (new)
   - Analysis dispatcher
   - Audit runner (axe-core integration)
   - Memory analyzer
   - Security checker

2. **`extension/vendor/axe-core.min.js`** (new)
   - Bundled axe-core library
   - Lazy-loaded on first analyze call

3. **`extension/content.js`** (modify)
   - Handle analyze requests from polling
   - Dispatch to analyze.js

4. **`extension/background.js`** (modify)
   - Register analyze capability
   - Handle cross-origin analysis if needed

5. **`tests/extension/analyze.test.js`** (new)
   - Extension unit tests

### Phase 4: Integration Testing

1. Add integration tests for end-to-end analyze flow
2. Update UAT checklist with analyze scenarios
3. Performance benchmarks for audit operations

## Backward Compatibility

### Breaking Changes

**None.** The `analyze` tool is purely additive:
- Existing tools (`observe`, `generate`, `configure`, `interact`) unchanged
- Existing API contracts preserved
- Existing extension functionality unaffected

### Version Bump

- This is a **minor version bump** (v6.x → v7.0)
- Signals new capability without breaking changes
- MCP clients that don't use `analyze` continue working

## Rollout Plan

### Stage 1: Internal Testing
- Implement analyze tool
- Test with internal AI workflows
- Validate performance targets

### Stage 2: Beta Release
- Release as v7.0-beta
- Document new tool in changelog
- Gather feedback from early adopters

### Stage 3: General Availability
- Release v7.0
- Update marketing site
- Blog post: "Kaboom now supports unified debugging lifecycle"

## Rollback Plan

If issues discovered post-release:

1. **Disable analyze tool** in server (feature flag)
2. **Release patch** removing analyze from tool list
3. **Preserve extension code** (no removal needed, just server-side disable)

Rollback does not affect existing functionality.

## Checklist

### Pre-Implementation
- [ ] Spec reviewed by principal engineer agent
- [ ] Architecture.md constraint update approved
- [ ] claude.md update drafted

### Implementation
- [ ] Server: analyze.go created
- [ ] Server: MCP tool list updated
- [ ] Server: Tests passing
- [ ] Extension: analyze.js created
- [ ] Extension: axe-core bundled
- [ ] Extension: Tests passing
- [ ] Integration tests passing

### Documentation
- [ ] Architecture.md updated
- [ ] claude.md updated
- [ ] README.md updated (if needed)
- [ ] Changelog updated
- [ ] UAT checklist updated

### Release
- [ ] Version bumped to v7.0
- [ ] Beta tested
- [ ] GA released
- [ ] Marketing site updated

## Timeline

| Phase | Scope |
|-------|-------|
| Phase 1 | Documentation updates |
| Phase 2 | Server implementation |
| Phase 3 | Extension implementation |
| Phase 4 | Integration testing & release |

## Dependencies

- axe-core must be bundled (no CDN per Chrome Web Store policy)
- Lighthouse integration may require chrome.debugger permission
- React DevTools globals optional (graceful fallback if absent)
