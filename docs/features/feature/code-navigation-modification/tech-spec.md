---
status: proposed
scope: feature/code-navigation-modification
ai-priority: high
tags: [v7, code-repair, developer-experience]
relates-to: [product-spec.md, ../backend-control/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-code-navigation-modification
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Code Navigation & Modification — Technical Specification

## Architecture

### System Diagram
```
┌─────────────────────────────────────────────────────┐
│  Kaboom MCP Server (Go)                           │
│  ┌───────────────────────────────────────────────┐  │
│  │ Code Router                                   │  │
│  │ - Dispatch code_search, code_read, code_modify│  │
│  │ - Validate file paths (security)              │  │
│  │ - Enforce git staging (no auto-commit)        │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ File Searcher                                 │  │
│  │ - Regex pattern matching                      │  │
│  │ - Language-based filtering                    │  │
│  │ - .gitignore aware (skip node_modules, etc.)  │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ File Reader & Analyzer                        │  │
│  │ - Syntax highlighting                         │  │
│  │ - Dependency extraction                       │  │
│  │ - Function boundary detection                 │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Code Editor                                   │  │
│  │ - Line-based edits (replace, insert, delete)  │  │
│  │ - Preserve indentation & formatting           │  │
│  │ - Generate diff for review                    │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Git Integration                               │  │
│  │ - Stage changes (git add)                     │  │
│  │ - Generate diffs for review                   │  │
│  │ - Rollback support (git checkout)             │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Test Runner                                   │  │
│  │ - Execute test commands                       │  │
│  │ - Capture output & results                    │  │
│  │ - Log correlation_id with results             │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Modification Logger                           │  │
│  │ - Record all code changes                     │  │
│  │ - Track correlation_id → modifications        │  │
│  │ - Support rollback by modification_id         │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────┐
│  Local Filesystem + Git Repository                 │
│ - Source files (read + write)                       │
│ - .git/ (staging, diff, history)                    │
└─────────────────────────────────────────────────────┘
```

### Data Flow: Find & Fix Bug
```
1. AI observes: network timeout in PaymentForm
2. AI calls: interact({what: "code_search", pattern: "PaymentForm|handlePayment"})
3. Searcher scans src/ directory, finds matches
4. AI calls: interact({what: "code_read", file_path: "src/components/PaymentForm.tsx"})
5. Reader returns file content with line numbers
6. AI analyzes, identifies: missing timeout handling
7. AI calls: interact({what: "code_modify", operation: "insert_after", line: 35, content: "..."})
8. Editor inserts code, stages file via git add
9. Modification logger records: file, lines_changed, correlation_id, reason
10. AI calls: interact({what: "run_tests", test_file: "...", correlation_id: "..."})
11. Test runner executes, captures results
12. Results show: 8 passed, 0 failed → fix verified
13. Developer reviews staged changes, commits or reverts
```

## Implementation Plan

### Phase 1: Core File Operations (Week 1)
1. **File Searcher**
   - Implement recursive directory walk with .gitignore awareness
   - Regex pattern matching across file contents
   - Language detection by file extension
   - Result ranking by relevance (match count, line proximity)

2. **File Reader**
   - Read file contents with UTF-8 validation
   - Line number annotation
   - Optional range limiting (e.g., lines 40-70)
   - Syntax-aware context (detect functions, classes)

3. **File Editor**
   - Implement line-based edits (replace, insert_before, insert_after, delete)
   - Preserve indentation
   - Generate unified diff format
   - Validate syntax (basic: no orphaned braces)

### Phase 2: Git & Test Integration (Week 2)
1. **Git Integration**
   - Implement `git add` for code changes
   - Generate diffs for code_modify responses
   - Implement rollback via `git checkout`
   - Prevent auto-commit (explicit opt-in only)

2. **Test Runner**
   - Execute arbitrary shell commands (npm test, pytest, go test)
   - Capture stdout/stderr
   - Parse test results (pytest, jest, mocha formats)
   - Timeout enforcement (30s default)

3. **Modification Logger**
   - Record all code changes to `.kaboom/modifications.jsonl`
   - Index by correlation_id for full-stack tracing
   - Support rollback by modification_id

### Phase 3: Safety & Review (Week 3)
1. **Security Checks**
   - Enforce path traversal prevention (no ../ escapes)
   - Limit to repository root directory
   - Validate file modifications don't introduce syntax errors
   - Warn on risky operations (delete entire files, etc.)

2. **Review Workflow**
   - All changes staged (not committed)
   - Developer can review via git diff
   - Explicit approval needed for commit
   - Audit trail of all changes

3. **Performance Optimization**
   - Cache file index with TTL 30s
   - Implement fast regex matching for large files
   - Lazy-load file content (don't read entire repo)

## API Changes

### New `interact()` mode: code_search
```javascript
interact({
  action: "code_search",
  pattern: "handleSubmit|async.*payment",  // Regex
  language: "typescript|javascript|python",  // Optional
  path: "src/",  // Optional subdirectory
  limit: 50,
  exclude_patterns: ["node_modules", ".next", "dist"]
})
→ {
    matches: [
      {
        path: "src/components/PaymentForm.tsx",
        line: 42,
        context: "const handleSubmit = async () => {",
        relevance_score: 0.95
      },
      {
        path: "src/api/payment.ts",
        line: 15,
        context: "async function handlePayment(data) {",
        relevance_score: 0.88
      }
    ],
    total_matches: 2,
    search_time_ms: 45
  }
```

### New `interact()` mode: code_read
```javascript
interact({
  action: "code_read",
  file_path: "src/components/PaymentForm.tsx",
  lines: [40, 70]  // Optional: return only lines 40-70
})
→ {
    path: "src/components/PaymentForm.tsx",
    language: "typescript",
    content: "const handleSubmit = async () => {\n  ...\n}",
    lines: [40, 41, 42, ..., 70],
    total_lines: 150,
    git_status: "modified"  // or "untracked", "unchanged"
  }
```

### New `interact()` mode: code_modify
```javascript
interact({
  action: "code_modify",
  file_path: "src/components/PaymentForm.tsx",
  operation: "replace",  // "replace" | "insert_before" | "insert_after" | "delete"
  line_range: [45, 68],  // For replace/delete
  line: 35,  // For insert_before/insert_after
  new_content: "const handler = async () => {\n  // fix\n};",
  correlation_id: "bug-payment-timeout-001",
  reason: "Add timeout handling to payment form",
  auto_test: true  // Optionally run tests after
})
→ {
    status: "success",
    file_path: "src/components/PaymentForm.tsx",
    lines_changed: 24,
    git_diff: "diff --git a/src/components/PaymentForm.tsx ...",
    git_staged: true,
    test_results: {
      passed: 8,
      failed: 0,
      duration_ms: 2100
    } || null
  }
```

### New `interact()` mode: run_tests
```javascript
interact({
  action: "run_tests",
  test_file: "src/components/PaymentForm.test.ts",
  test_command: "npm test -- PaymentForm",  // Optional override
  correlation_id: "bug-payment-timeout-001",
  timeout_ms: 30000
})
→ {
    status: "success",
    command: "npm test -- PaymentForm.test.ts",
    tests_passed: 8,
    tests_failed: 0,
    duration_ms: 2100,
    output: "[PASS] src/components/PaymentForm.test.ts\n...",
    correlation_id: "bug-payment-timeout-001"
  }
```

### New `interact()` mode: code_rollback
```javascript
interact({
  action: "code_rollback",
  modification_id: "mod-20260131-101523-001"
})
→ {
    status: "rolled_back",
    file: "src/components/PaymentForm.tsx",
    lines_restored: 24,
    git_output: "Updated 1 path from the index"
  }
```

### New `observe()` mode: modification_log
```javascript
observe({
  what: "modification_log",
  correlation_id: "bug-payment-timeout-001"
})
→ {
    modifications: [
      {
        modification_id: "mod-20260131-101523-001",
        timestamp: "2026-01-31T10:15:23.456Z",
        file: "src/components/PaymentForm.tsx",
        operation: "replace",
        lines_affected: [45, 68],
        reason: "Add timeout handling",
        test_results: {passed: 8, failed: 0}
      }
    ]
  }
```

## Code References

### New files to create:
- `cmd/server/code/searcher.go` — File search engine
- `cmd/server/code/reader.go` — File reader
- `cmd/server/code/editor.go` — Code editor with line-based edits
- `cmd/server/code/git.go` — Git integration (staging, diff, rollback)
- `cmd/server/code/tests.go` — Test runner
- `cmd/server/code/logger.go` — Modification logger

### Existing files to modify:
- `cmd/server/mcp/server.go` — Add code_* action handlers
- `cmd/server/mcp/observe.go` — Add modification_log mode

## Performance Requirements
- File search: <100ms for typical project (5K files)
- File read: <50ms for files <100KB
- Code modify: <100ms (mostly I/O)
- Test execution: varies, but timeout at 30s
- Modification log query: <50ms

## Testing Strategy

### Unit Tests
1. Test file searcher with regex patterns
2. Test file reader line extraction
3. Test code editor: replace, insert, delete operations
4. Test indentation preservation
5. Test rollback logic

### Integration Tests
1. Create test project with sample files
2. Test code_search finds expected files
3. Test code_read returns correct content
4. Test code_modify stages changes in git
5. Test run_tests executes and captures results
6. Test code_rollback reverts changes
7. Test modification_log records operations

### E2E Tests
1. Full workflow: observe bug → search code → read file → modify → run tests → verify
2. Multiple modifications to same file
3. Rollback and retry
4. Correlation ID propagation through full stack

## Dependencies
- Git must be installed (for staging, rollback)
- Test runner available (npm, pytest, go test, etc.)
- Read/write access to repository

## Security Considerations

1. **Path Traversal Prevention**
   - Validate all file paths are within repository root
   - Reject paths with `../` or absolute paths
   - Use `filepath.Clean()` and `filepath.Rel()`

2. **File System Limits**
   - Maximum file size: 10MB
   - Maximum search results: 1000
   - Maximum files in result: 100

3. **Syntax Validation**
   - Basic validation: matching braces, parentheses
   - Do NOT attempt language-specific parsing (no AST)
   - Warn on suspicious patterns (code injection, shell escapes)

4. **Audit Trail**
   - All modifications logged with timestamp, correlation_id, reason
   - Cannot delete modification logs
   - Replay-able via modification_id

## Configuration

Services should support `/.kaboom/code` endpoint to customize:
- Allowed languages (typescript, python, go, etc.)
- Test commands for each language
- Paths to exclude from search (.gitignore aware by default)
- Maximum file size for edits
