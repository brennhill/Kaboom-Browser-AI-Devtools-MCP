---
status: proposed
scope: feature/code-navigation-modification
ai-priority: high
tags: [v7, testing, code-repair]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-code-navigation-modification
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Code Navigation & Modification — QA Plan

## Test Scenarios

### Scenario 1: Search for Function by Name
**Objective:** Verify code_search finds functions by pattern

#### Setup:
- Test project with multiple components
- PaymentForm.tsx contains: `const handleSubmit()`, `const handlePayment()`
- Payment.ts contains: `async function processPayment()`

#### Steps:
1. Call `interact({what: "code_search", pattern: "handlePayment|processPayment"})`
2. Verify results include all 3 functions with correct file paths
3. Verify relevance scores (exact match > partial match)
4. Verify line numbers correct

#### Expected Result:
- Search returns 3 matches: PaymentForm.tsx (2 matches), Payment.ts (1 match)
- Line numbers point to actual function definitions
- Relevance score ranks exact matches higher

#### Acceptance Criteria:
- [ ] All matching functions found
- [ ] No false positives
- [ ] Results sorted by relevance
- [ ] Search completes <100ms

---

### Scenario 2: Search with Language Filter
**Objective:** Verify language filtering works correctly

#### Setup:
- Project contains mixed files: *.ts, *.tsx, *.py, *.go
- All contain pattern "async" or "timeout"

#### Steps:
1. Call `interact({what: "code_search", pattern: "timeout", language: "typescript"})`
2. Verify only .ts and .tsx files returned
3. Call with `language: "python"`, verify only .py files returned
4. Call with no language filter, verify all languages returned

#### Expected Result:
- Language filter strictly enforced
- No cross-language false positives

#### Acceptance Criteria:
- [ ] Filter applied correctly per language
- [ ] No files from other languages in results

---

### Scenario 3: Read File with Line Range
**Objective:** Verify code_read returns correct subset of lines

#### Setup:
- PaymentForm.tsx has 150 lines total
- Target: read lines 40-70

#### Steps:
1. Call `interact({what: "code_read", file_path: "src/components/PaymentForm.tsx", lines: [40, 70]})`
2. Verify returned content contains exactly 31 lines (40-70 inclusive)
3. Verify line numbers annotated correctly
4. Verify content accurate

#### Expected Result:
- Returned lines are exactly 40-70
- Line numbers match source
- No truncation or extra lines

#### Acceptance Criteria:
- [ ] Line range respected
- [ ] Content accurate
- [ ] Line numbers correct
- [ ] Read completes <50ms

---

### Scenario 4: Read Entire File
**Objective:** Verify code_read without line range returns full file

#### Setup:
- Target file: src/utils/helpers.ts with 75 lines

#### Steps:
1. Call `interact({what: "code_read", file_path: "src/utils/helpers.ts"})`
2. Verify all 75 lines returned
3. Verify first and last lines correct

#### Expected Result:
- Full file contents returned
- All lines present and correct

#### Acceptance Criteria:
- [ ] Complete file returned
- [ ] No truncation
- [ ] Correct git_status reported

---

### Scenario 5: Replace Lines in File
**Objective:** Verify code_modify replace operation

#### Setup:
- PaymentForm.tsx contains handleSubmit function with race condition (missing dependency array)
- Target: lines 45-50

#### Steps:
1. Read original: `interact({what: "code_read", file_path: "src/components/PaymentForm.tsx", lines: [45, 50]})`
2. Record original content
3. Call `interact({what: "code_modify", operation: "replace", line_range: [45, 50], new_content: "...fixed code..."})`
4. Verify response shows git_diff
5. Verify git_staged: true
6. Call `git diff` to confirm changes staged
7. Read file again: verify new content present

#### Expected Result:
- Lines 45-50 replaced with new content
- Changes staged to git
- Diff shows exact changes
- Original content accessible via git history

#### Acceptance Criteria:
- [ ] Exact lines replaced
- [ ] Indentation preserved or corrected
- [ ] git add executed
- [ ] Rollback possible

---

### Scenario 6: Insert Code After Line
**Objective:** Verify insert_after operation

#### Setup:
- Target file needs debugging statement added
- Insert after line 35

#### Steps:
1. Call `interact({what: "code_modify", operation: "insert_after", line: 35, new_content: "console.debug('...');"})`
2. Verify response shows new lines added
3. Read file: verify debug statement at line 36
4. Verify indentation matches surrounding code

#### Expected Result:
- Debug statement inserted at correct location
- Indentation correct
- File now has one more line

#### Acceptance Criteria:
- [ ] Inserted at correct line
- [ ] Indentation matched
- [ ] No syntax errors introduced

---

### Scenario 7: Delete Lines
**Objective:** Verify delete operation

#### Setup:
- File has unused import statements (lines 3-5)
- Target: delete these lines

#### Steps:
1. Call `interact({what: "code_modify", operation: "delete", line_range: [3, 5]})`
2. Verify response shows lines deleted
3. Read file: verify old imports gone
4. Verify file structure intact (no orphaned braces)

#### Expected Result:
- Lines deleted correctly
- File structure valid
- Indentation correct

#### Acceptance Criteria:
- [ ] Correct lines deleted
- [ ] No syntax errors
- [ ] File structure valid

---

### Scenario 8: Auto-Test After Modification
**Objective:** Verify auto_test flag runs tests after change

#### Setup:
- PaymentForm.test.ts exists and passes initially
- Modification made to PaymentForm.tsx

#### Steps:
1. Call `interact({what: "code_modify", operation: "replace", ..., auto_test: true})`
2. Response should include test_results
3. Verify tests ran: passed, failed, duration_ms present
4. If modification broke tests: verify failure shown
5. If modification fixed tests: verify newly passing tests shown

#### Expected Result:
- Tests run immediately after modification
- Results included in response
- Developer can see impact of change

#### Acceptance Criteria:
- [ ] Tests executed
- [ ] Results captured
- [ ] Pass/fail counts accurate
- [ ] Duration measured

---

### Scenario 9: Run Tests Independently
**Objective:** Verify run_tests command executes and captures results

#### Setup:
- Test file: src/components/PaymentForm.test.ts
- Tests currently pass (baseline)

#### Steps:
1. Call `interact({what: "run_tests", test_file: "src/components/PaymentForm.test.ts"})`
2. Verify command executed: "npm test -- PaymentForm.test.ts"
3. Verify results captured: tests_passed, tests_failed, duration_ms
4. Verify output includes test names and assertions
5. Modify source file to break a test
6. Run tests again: verify failure captured

#### Expected Result:
- Test command executed correctly
- Results parsed accurately
- Pass/fail counts match actual
- Output captured for debugging

#### Acceptance Criteria:
- [ ] Tests executed
- [ ] Results parsed correctly
- [ ] Failures captured with assertion details
- [ ] Timeout enforced (30s default)

---

### Scenario 10: Rollback Single Modification
**Objective:** Verify code_rollback reverts a change

#### Setup:
- Modification made: mod-20260131-101523-001
- File staged with changes

#### Steps:
1. Record current file content
2. Call `interact({what: "code_rollback", modification_id: "mod-20260131-101523-001"})`
3. Verify response: status: "rolled_back"
4. Read file: verify original content restored
5. Check git status: changes should be unstaged/reverted

#### Expected Result:
- File reverted to pre-modification state
- Git working tree clean
- Original content accessible

#### Acceptance Criteria:
- [ ] File reverted exactly
- [ ] No partial reverts
- [ ] Git status updated

---

### Scenario 11: Modification Log with Correlation ID
**Objective:** Verify modifications recorded with correlation_id

#### Setup:
- Modification made with correlation_id: "bug-payment-timeout-001"
- Multiple files modified

#### Steps:
1. Perform modifications: code_modify x3 with same correlation_id
2. Call `observe({what: "modification_log", correlation_id: "bug-payment-timeout-001"})`
3. Verify all 3 modifications returned
4. Verify each has correct correlation_id
5. Verify timestamps in chronological order

#### Expected Result:
- All modifications linked to correlation_id
- Full audit trail of changes for bug fix
- Can trace bug → observations → code changes → tests

#### Acceptance Criteria:
- [ ] All modifications recorded
- [ ] Correlation ID correctly indexed
- [ ] Timestamps accurate
- [ ] Reason/explanation present

---

### Scenario 12: Security: Path Traversal Prevention
**Objective:** Verify path traversal attacks blocked

#### Setup:
- Attacker attempts to access files outside repo

#### Steps:
1. Call `interact({what: "code_read", file_path: "../../../etc/passwd"})`
2. Verify error returned: "Path outside repository root"
3. Call `interact({what: "code_read", file_path: "/etc/passwd"})`
4. Verify error: "Absolute paths not allowed"
5. Call with legitimate path: works fine

#### Expected Result:
- Malicious paths rejected
- Legitimate paths work

#### Acceptance Criteria:
- [ ] Path traversal attempts blocked
- [ ] Error messages clear
- [ ] No side effects

---

### Scenario 13: Search Performance with Large Codebase
**Objective:** Verify search performance on large project (50K files)

#### Setup:
- Simulate large project or use real project with many files

#### Steps:
1. Call `interact({what: "code_search", pattern: "handlePayment"})`
2. Record search time
3. Verify completes in <100ms
4. Verify result count reasonable

#### Expected Result:
- Search fast even on large codebase
- Results limited to prevent overwhelming output

#### Acceptance Criteria:
- [ ] Search <100ms
- [ ] Results limited intelligently
- [ ] No timeout

---

### Scenario 14: Git Staging & Diff Generation
**Objective:** Verify changes properly staged and diff generated

#### Setup:
- Modification made to file

#### Steps:
1. Call code_modify with modification
2. Verify response includes git_diff
3. Verify git_staged: true
4. Run `git status`: file should appear as staged
5. Run `git diff --staged`: output should match response diff

#### Expected Result:
- Changes staged to git
- Diff format standard (unified diff)
- Developer can review with normal git tools

#### Acceptance Criteria:
- [ ] Changes staged
- [ ] Diff generation accurate
- [ ] Git tools show changes

---

### Scenario 15: Indentation Preservation
**Objective:** Verify code modifications preserve or correctly adjust indentation

#### Setup:
- File uses 2-space indentation
- Modification spans multiple indentation levels

#### Steps:
1. Read target lines: verify indentation
2. Perform modification that changes indentation level
3. Read modified lines: verify indentation correct
4. Run tests: verify no syntax errors from indentation issues

#### Expected Result:
- Indentation preserved when not changing nesting level
- Indentation adjusted when adding/removing nesting
- No mixed tabs/spaces

#### Acceptance Criteria:
- [ ] Indentation correct
- [ ] No syntax errors
- [ ] Code still runs

---

## Acceptance Criteria (Overall)
- [ ] All scenarios pass on Linux, macOS, Windows
- [ ] Search completes <100ms for projects <50K files
- [ ] File read completes <50ms
- [ ] Modifications staged but not committed
- [ ] Git integration works (add, diff, checkout)
- [ ] All changes audited with correlation_id
- [ ] Rollback works for all modification types
- [ ] Path traversal prevented
- [ ] Test execution reliable with <30s timeout
- [ ] Line numbers always accurate
- [ ] Indentation preserved

## Test Data

### Sample Project Structure
```
src/
  components/
    PaymentForm.tsx (150 lines)
    ShoppingCart.tsx (200 lines)
    UserProfile.tsx (120 lines)
  api/
    payment.ts (80 lines)
    user.ts (60 lines)
  utils/
    helpers.ts (75 lines)
  hooks/
    usePayment.ts (90 lines)
__tests__/
  PaymentForm.test.ts (120 lines)
  ShoppingCart.test.ts (150 lines)
package.json
```

### Test Files Content
- PaymentForm.tsx: includes race condition (missing dependency array)
- payment.ts: missing timeout handling
- helpers.ts: unused imports
- Test files: comprehensive coverage, all passing

## Regression Tests

**Critical:** After each change, verify:
1. File search doesn't corrupt .gitignore
2. Code modifications don't introduce syntax errors
3. Rollback fully reverts changes
4. git add completes without prompting
5. Large files (>10MB) rejected
6. Binary files skipped in search
7. Modification log never loses entries
8. Correlation IDs never collide
9. Line numbers never off-by-one

## Performance Baseline

| Operation | Target | Measured | Status |
|-----------|--------|----------|--------|
| code_search (50K files) | <100ms | _ | _ |
| code_read (<100KB file) | <50ms | _ | _ |
| code_modify | <100ms | _ | _ |
| run_tests | <30s | _ | _ |
| code_rollback | <100ms | _ | _ |
| git staging | <50ms | _ | _ |

## Known Limitations

- [ ] No language-specific AST parsing (line-based edits only)
- [ ] No automatic code formatting (preserve existing style)
- [ ] No multi-file refactoring
- [ ] No compilation/build integration
- [ ] No IDE integration (separate CLI feature)
- [ ] Test parsing: best-effort (pytest, jest, mocha only)
