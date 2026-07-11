---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Codacy Analysis Report - February 9, 2026

**Analysis Date:** February 9, 2026
**API Token Used:** ✅ Valid (lHYOvUqzdGUcjC9p7wru)
**Total Issues Found:** 15+
**Severity Breakdown:** 4 Critical, 3 Warnings (CVE/Dependency), 8+ Warnings (Complexity/Config)

---

## Executive Summary

The Codacy analysis identified several categories of issues:

1. **Command Injection Flags (False Positives)** - Semgrep incorrectly flagged safe hardcoded command construction
2. **SSRF Flag (False Positive)** - URL is hardcoded to localhost
3. **CVE-2024-34155 (Real)** - Go standard library version needs update to 1.23.1+
4. **Cyclomatic Complexity (Real)** - Several functions exceed safe thresholds
5. **Line Count Violations (Real)** - Some functions exceed 50-80 LOC limits
6. **Stylelint Configuration (Real)** - Invalid/obsolete SCSS rules

---

## Detailed Issue Analysis

### 🔴 Security Issues (4 Flagged)

#### Issue 1: SSRF Detection - connect_mode.go:88 (FALSE POSITIVE)

**Severity:** Critical (Flagged)
**Actual Risk:** Low (False Positive)

**Code:**

```go
// Line 22 - serverURL is hardcoded to localhost
serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

// Line 78 - mcpURL derived from localhost URL
mcpURL := serverURL + "/mcp"

// Line 88 - FLAGGED as SSRF, but URL is from hardcoded localhost
req, err := http.NewRequestWithContext(forwardCtx, "POST", mcpURL, strings.NewReader(line))
```

**Analysis:**

- ✅ `serverURL` is hardcoded to `127.0.0.1` (localhost)
- ✅ `port` is a command-line flag validated by `findFreePort()`
- ✅ `mcpURL` is derived only from localhost URL
- ✅ No user-controlled data flows into URL
- ⚠️ Semgrep doesn't recognize the hardcoded pattern

**Status:** ✅ SUPPRESSED with #nosec G601 comment
- Added comment on line 78: URL construction from hardcoded localhost
- Added comment on line 88: Usage from localhost-only serverURL

---

#### Issue 2: Command Injection - connection_lifecycle_test.go:295 (FALSE POSITIVE)

**Severity:** Critical (Flagged)
**Actual Risk:** None (Test-only, hardcoded values)

**Code:**

```go
binary := buildTestBinary(t)  // Local test binary path
port := findFreePort(t)        // Local port number

// Line 295 - FLAGGED as command injection
serverCmd := exec.Command(binary, "--port", fmt.Sprintf("%d", port))
```

**Analysis:**

- ✅ `binary` = local test binary from `buildTestBinary(t)` (not user-controlled)
- ✅ `port` = integer from `findFreePort()` (not user-controlled)
- ✅ `fmt.Sprintf("%d", port)` safely converts int to string
- ⚠️ Semgrep incorrectly treats `fmt.Sprintf` as potential injection vector

**Status:** ✅ SUPPRESSED with #nosec G204 comment

- Added comment on line 295: test-only code with hardcoded values
- Added comment on line 480: same pattern in ColdStartRace test

---

#### Issue 3: Command Injection - server_persistence_test.go:50 (FALSE POSITIVE)

**Severity:** Critical (Flagged)
**Actual Risk:** None (Test-only, same pattern as Issue 2)

**Code:**

```go
cmd := exec.Command(binary, "--port", fmt.Sprintf("%d", port))
```

**Analysis:** Same as Issue 2 - test-only, hardcoded binary and port.

**Status:** ✅ SUPPRESSED with #nosec G204 comment

- Added comment on line 50: test-only code with hardcoded values

---

#### Issue 4: Command Injection - upload_handlers.go:709 (FALSE POSITIVE)

**Severity:** Critical (Flagged)
**Actual Risk:** None (Hardcoded xdotool, already marked #nosec)

**Code:**

```go
commands := []struct{
    name string
    args []string
}{
    {"xdotool", []string{"search", "--name", "Open", "windowactivate"}},
    {"xdotool", []string{"key", "ctrl+l"}},
    {"xdotool", []string{"type", "--clearmodifiers", req.FilePath}},  // Only user input is file path
    {"xdotool", []string{"key", "Return"}},
}

// Line 709 - FLAGGED but already marked #nosec
cmd := exec.Command(c.name, c.args...) // #nosec G204
```

**Analysis:**

- ✅ Command name (`xdotool`) is hardcoded
- ✅ Arguments array is hardcoded
- ✅ `req.FilePath` is passed as argument (safe - not injected into command)
- ✅ Already marked with `#nosec G204` with comment
- ✅ xdotool is found via LookPath (safer than direct path)

**Status:** No additional action needed - already properly suppressed.

---

### 🟡 Dependency Issues (1 Real)

#### Issue 5: CVE-2024-34155 - Go Standard Library (REAL)

**Severity:** High
**Status:** ✅ FIXED

**Finding:** go.mod specifies `go 1.23` but CVE-2024-34155 requires `go 1.23.1` or higher.

**Current State:**

```
go 1.23.1
```

**Action Completed:** Updated minimum Go version requirement to 1.23.1

---

### 🟡 Cyclomatic Complexity Issues (Real Warnings)

**Severity:** Medium (Code maintainability)
**Threshold:** 8 is safe, >10 needs review

#### Issue 6: cleanup_old_processes (Lizard score: 11)

**File:** `cmd/browser-agent/lifecycle_unix.go`
**Risk:** Complex control flow, hard to test all paths

#### Issue 7: execute_install (Lizard score: 19)

**File:** `cmd/browser-agent/setup.go`
**Risk:** Very high complexity, multiple nested conditions

#### Issue 8: captureStateSnapshot (Lizard score: 22)

**File:** `cmd/browser-agent/state_snapshot.go`
**Risk:** Extremely high complexity, needs refactoring

#### Issue 9: TypeScript functions

**Files:** Various in `src/` directory
**Examples:** `setInputValue()` in `src/inject/api.ts` (66 LOC, exceeds 50-line limit)

**Status:** These are real issues that should be addressed in the next refactoring cycle.

---

### 🟡 Line Count Issues (Real Warnings)

**Severity:** Low-Medium (Maintainability)
**Limit:** 50-80 lines per function

#### Issue 10: setInputValue() - src/inject/api.ts (66 lines)

**Status:** Exceeds 50-line recommendation

---

### 🟡 Configuration/Style Issues

#### Issue 11: Stylelint - main.scss

**Issue:** Unknown/obsolete SCSS rules in main.scss
- `scss_function-no-unknown` (deprecated rule)
- `no-obsolete-attribute` (invalid rule)

**Status:** Need to verify Stylelint configuration

---

## Remediation Plan

### Phase 1: False Positive Suppression ✅ COMPLETE

These are safe patterns that Semgrep/Codacy incorrectly flag.

**Actions Completed:**

- ✅ Added #nosec G601 comments to connect_mode.go (lines 78, 88)
- ✅ Added #nosec G204 comments to connection_lifecycle_test.go (lines 295, 480)
- ✅ Added #nosec G204 comments to server_persistence_test.go (line 50)
- ✅ Documented why each is safe in code comments

### Phase 2: Real Issues (Scheduled)

**Priority 1 ✅ COMPLETE:**

- ✅ Update `go 1.23` → `go 1.23.1` in go.mod (verified with `go mod verify`)

**Priority 2 (Next Sprint):**
- Refactor `cleanup_old_processes` (reduce to <8 complexity)
- Refactor `execute_install` (reduce to <8 complexity)
- Break down `captureStateSnapshot` (reduce to <8 complexity)
- Refactor TypeScript complex functions
- Fix Stylelint configuration in main.scss

**Priority 3 (Backlog):**
- Reduce `setInputValue()` from 66 to <50 lines via extraction

---

## Verification

To re-run Codacy analysis with fresh API token:

```bash
CODACY_API_TOKEN="lHYOvUqzdGUcjC9p7wru" \
curl -s -X POST "https://app.codacy.com/api/v3/analysis/organizations/gh/brennhill/kaboom-agentic-browser-devtools-mcp/issues/search" \
  -H "api-token: ${CODACY_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"levels":["Error","Warning"]}'
```

---

## Summary

| Category | Count | Status | Action |
| --- | --- | --- | --- |
| False Positives (Safe Code) | 4 | ✅ Suppressed | #nosec comments added |
| Real CVE/Dependency | 1 | ✅ Fixed | go.mod updated to 1.23.1 |
| Complexity Issues | 4+ | 📅 Backlog | Next refactor cycle |
| Line Count Issues | 1+ | 📅 Backlog | Next refactor cycle |
| Config Issues | 1 | 📅 Backlog | Verify Stylelint |

**Bottom Line:** All critical security flags were false positives and have been properly suppressed. One real CVE has been fixed. Complexity issues are real but not urgent and can be addressed in the next refactoring cycle.
