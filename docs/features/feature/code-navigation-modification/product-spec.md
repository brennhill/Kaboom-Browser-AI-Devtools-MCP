---
status: proposed
scope: feature/code-navigation-modification
ai-priority: high
tags: [v7, code-repair, hands, developer-experience]
relates-to: [../backend-control/product-spec.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-code-navigation-modification
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Code Navigation & Modification

## Overview
Code Navigation & Modification enables Kaboom to inspect, navigate, and directly modify application code in response to discovered bugs or performance issues. When an AI agent identifies a bug through observation and testing, it can now locate the responsible code, understand context, and propose or apply fixes directly. This feature bridges the gap between "problem identified" and "fix committed," enabling autonomous or semi-autonomous repair workflows.

## Problem
Current Kaboom workflow:
1. AI observes failure (network error, wrong DOM, timeout)
2. AI reports the issue to developer
3. Developer manually navigates codebase, finds root cause
4. Developer edits files, re-tests, commits

This is time-consuming and prevents full autonomous repair. Developers spend hours navigating codebases, especially in large projects with many services. AI lacks programmatic access to source code and can't apply fixes.

## Solution
Code Navigation & Modification adds:
1. **Code Discovery** — Locate files by pattern, component name, or error stack trace
2. **Code Reading** — Inspect file contents, understand dependencies, follow imports
3. **Code Modification** — Apply targeted fixes to files (add logging, fix logic, update config)
4. **Verification** — Re-run tests after fix, capture results
5. **Commit Generation** — Propose commits with AI-written explanations

All operations are:
- **Safe** — Changes only happen with explicit opt-in or prior agreement
- **Testable** — Changes validated by running affected tests immediately
- **Reversible** — Previous versions available for rollback
- **Auditable** — All changes logged with correlation to triggering observation

## User Stories
- As an AI agent, I want to locate the React component causing a rendering error so that I can understand the bug's root cause
- As an AI agent, I want to add detailed console.error() logging at specific locations so that future runs capture diagnostics
- As an AI agent, I want to apply a specific fix (e.g., fix race condition in useEffect hook) so that I can autonomously repair the bug
- As a developer, I want to review all proposed changes before they're committed so that I maintain code quality
- As an AI agent, I want to run the affected test suite immediately after a change so that I can verify the fix worked
- As a developer, I want to see the correlation between an observed bug and the code fix applied so that I can learn from AI repairs

## Acceptance Criteria
- [ ] Kaboom can locate files by regex pattern, component name, error stack trace
- [ ] Kaboom can read and display file contents with line numbers
- [ ] Kaboom can apply targeted edits: insert, replace, delete specific lines/blocks
- [ ] All code changes are git-staged (not committed) without explicit permission
- [ ] Code modifications include comment linking to correlation_id that triggered the fix
- [ ] After modification, Kaboom can run relevant test files
- [ ] Performance: file search <100ms, file read <50ms, modification <100ms
- [ ] Support for JavaScript/TypeScript, Python, Go, and plain text config files
- [ ] Rollback to previous version in <500ms

## Not In Scope
- Automatic commit without developer review
- Compilation/build step integration (just code changes)
- Multi-file refactoring (single file edits only)
- Code formatting enforcement (preserve existing style)
- Language-specific complexity (AST parsing, type checking)
- IDE integration (this tool is Kaboom's own code interface)

## Data Structures

### File Discovery Result
```json
{
  "matches": [
    {
      "path": "src/components/PaymentForm.tsx",
      "type": "file",
      "size_bytes": 2450,
      "language": "typescript",
      "relevance_score": 0.95,
      "matches_in_file": [
        {
          "line": 42,
          "content": "const handleSubmit = async () => {",
          "context": "Function definition matching pattern 'handleSubmit'"
        }
      ]
    }
  ],
  "total_matches": 3,
  "search_time_ms": 45
}
```

### Code Edit Operation
```json
{
  "file_path": "src/components/PaymentForm.tsx",
  "operation": "replace",
  "line_range": [42, 68],
  "original_content": "const handleSubmit = async () => {\n  // ... original code",
  "new_content": "const handleSubmit = async () => {\n  console.debug('handleSubmit called', {timestamp: Date.now()});\n  // ... fixed code",
  "correlation_id": "bug-payment-timeout-001",
  "reason": "Add debugging to diagnose timeout issue",
  "test_command": "npm test -- PaymentForm.test.tsx"
}
```

### Code Modification Record
```json
{
  "modification_id": "mod-20260131-101523-001",
  "timestamp": "2026-01-31T10:15:23.456Z",
  "correlation_id": "bug-payment-timeout-001",
  "file": "src/components/PaymentForm.tsx",
  "operation": "replace",
  "lines_affected": 5,
  "diff": "+ console.debug(...)\n- // old error handling",
  "git_diff_hash": "a1b2c3d4e5f6",
  "test_results": {
    "command": "npm test -- PaymentForm.test.tsx",
    "passed": 5,
    "failed": 0,
    "duration_ms": 3200
  },
  "committed": false,
  "staged": true
}
```

## Examples

### Example 1: Find & Fix React State Bug
```javascript
// AI observes: "Cart count doesn't update when item added"
// AI runs code discovery:
await interact({
  action: "code_search",
  pattern: "addToCart|useCart",
  language: "typescript",
  path: "src/"
});
// Returns: src/hooks/useCart.ts, src/components/ShoppingCart.tsx

// AI reads useCart.ts
const content = await interact({
  action: "code_read",
  file_path: "src/hooks/useCart.ts"
});

// AI discovers: setCart() called but state not updated
// AI applies fix:
await interact({
  action: "code_modify",
  file_path: "src/hooks/useCart.ts",
  operation: "replace",
  line_range: [45, 50],
  new_content: `
    const addItem = (item) => {
      const updatedCart = [...cart, item];
      setCart(updatedCart);  // FIX: Was missing this line
      return updatedCart;
    };
  `,
  correlation_id: "bug-cart-count-001",
  reason: "State update was missing in addItem function"
});

// AI runs tests
await interact({
  action: "run_tests",
  test_file: "src/hooks/useCart.test.ts",
  correlation_id: "bug-cart-count-001"
});
// Output: 8 passed, 0 failed in 2.1s
```

### Example 2: Add Debugging After Error
```javascript
// AI observes: "Payment timeout in production"
// Backend logs show: "POST /api/payments/process → 504 after 5.2s"
// AI wants to add more diagnostics

await interact({
  action: "code_modify",
  file_path: "src/api/payment.ts",
  operation: "insert_after",
  line: 23,
  content: `
    console.debug('[Payment API]', {
      timestamp: new Date().toISOString(),
      method: 'processPayment',
      userId: userId,
      retryCount: retries
    });
  `,
  correlation_id: "prod-payment-timeout-001"
});
```

### Example 3: Fix Race Condition in useEffect
```javascript
// AI observes: "useEffect running twice, creating duplicate requests"
// Root cause: Missing dependency array

await interact({
  action: "code_read",
  file_path: "src/components/UserProfile.tsx",
  lines: [30, 50]
});
// Returns: useEffect hook without dependency array

await interact({
  action: "code_modify",
  file_path: "src/components/UserProfile.tsx",
  operation: "replace",
  line_range: [32, 38],
  original_content: `
    useEffect(() => {
      fetchUser();
    });
  `,
  new_content: `
    useEffect(() => {
      fetchUser();
    }, [userId]);  // FIX: Added dependency array
  `,
  correlation_id: "bug-duplicate-requests-001",
  reason: "useEffect missing dependency array causing duplicates"
});
```

## MCP Tool Changes

### New `interact()` mode: code_search
```javascript
interact({
  action: "code_search",
  pattern: "handleSubmit|async.*payment",  // Regex
  language: "typescript|javascript|python",  // Optional filter
  path: "src/",  // Optional directory
  limit: 50  // Max results
})
→ {
    matches: [
      {path: "src/components/PaymentForm.tsx", line: 42, content: "..."},
      {path: "src/api/payment.ts", line: 15, content: "..."}
    ]
  }
```

### New `interact()` mode: code_read
```javascript
interact({
  action: "code_read",
  file_path: "src/components/PaymentForm.tsx",
  lines: [40, 70]  // Optional range
})
→ {
    path: "src/components/PaymentForm.tsx",
    language: "typescript",
    content: "const handleSubmit = async () => {...}",
    line_numbers: [40, 41, 42, ...]
  }
```

### New `interact()` mode: code_modify
```javascript
interact({
  action: "code_modify",
  file_path: "src/components/PaymentForm.tsx",
  operation: "replace",  // or "insert_before", "insert_after", "delete"
  line_range: [45, 68],
  new_content: "...",
  correlation_id: "bug-id",
  reason: "Fix race condition in payment handler",
  test_command: "npm test -- PaymentForm.test.tsx"
})
→ {
    status: "success",
    lines_changed: 24,
    git_staged: true,
    test_results: {
      passed: 8,
      failed: 0,
      duration_ms: 2100
    }
  }
```

### New `interact()` mode: run_tests
```javascript
interact({
  action: "run_tests",
  test_file: "src/components/PaymentForm.test.ts",
  correlation_id: "bug-id"
})
→ {
    status: "success",
    tests_passed: 8,
    tests_failed: 0,
    duration_ms: 3200,
    output: "..." // Test output
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
    lines_restored: 24
  }
```
