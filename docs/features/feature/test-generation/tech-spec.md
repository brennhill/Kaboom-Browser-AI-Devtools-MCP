---
feature: test-generation
status: approved
tool: generate
mode: [test_from_context, test_heal, test_classify]
version: 0.7.12
review: review.md
doc_type: tech-spec
feature_id: feature-test-generation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — Technical Specification

## Critical Issue Resolutions

All 10 critical issues from review.md have been addressed:

| Issue ID | Issue | Resolution | Section |
|----------|-------|------------|---------|
| P1-1 | 30-second batch timeout too long | Reduced to 10s | §2 |
| P1-2 | No batch size limits | Added limits table (20 files, 500KB each, 5MB total) | §2 |
| C1-1 | Async pattern incomplete | Changed to blocking pattern (no async needed) | §2 |
| C1-2 | No concurrency limit | Added per-operation limits | §2 |
| D1-1 | error_id scheme undefined | Defined as `err_{timestamp}_{hash8}` | §1 |
| D1-2 | Response format inconsistent | Adapted to mcpJSONResponse pattern | §5 |
| E1-1 | Path validation unspecified | Uses validatePathInDir from ai_persistence.go | §7 |
| S1-1 | File write without toggle | Requires AI Web Pilot enabled | §7 |
| S1-2 | Selector injection risk | Added validateSelector() | §7 |
| M1-1 | File organization unclear | Single testgen.go (split at 800 lines) | §3 |

---

## 1. Tool Parameters

```
Tool: generate
Modes: test_from_context, test_heal, test_classify

Parameters for test_from_context:
  - type (string, required): "test_from_context"
  - context (string, required): "error" | "interaction" | "regression"
  - error_id (string, optional): ID of captured error (format: see Error ID Scheme below)
  - framework (string, optional): "playwright" | "vitest" | "jest" (default: playwright)
  - output_format (string, optional): "file" | "inline" (default: inline)
  - base_url (string, optional): Override base URL in generated tests
  - include_mocks (boolean, optional): Generate network mocks (default: false)

Error ID Scheme (D1-1 Resolution):
  - Format: "err_{timestamp_ms}_{hash8}"
  - Example: "err_1706520600000_a1b2c3d4"
  - timestamp_ms: Unix timestamp in milliseconds when error was captured
  - hash8: First 8 characters of SHA-256 hash of (message + stack + url)
  - If error_id not provided, uses most recent captured error
  - Error IDs are assigned automatically when errors are captured by observe tool

Parameters for test_heal:
  - type (string, required): "test_heal"
  - action (string, required): "analyze" | "repair" | "batch"
  - test_file (string, optional): Path to test file (for analyze/repair)
  - test_dir (string, optional): Directory path (for batch)
  - broken_selectors ([]string, optional): Known broken selectors to heal
  - auto_apply (boolean, optional): Apply fixes automatically (default: false for confidence < 0.9)

Parameters for test_classify:
  - type (string, required): "test_classify"
  - action (string, required): "failure" | "batch"
  - failure (object, optional): Single failure to classify
  - failures ([]object, optional): Multiple failures (for batch)
  - test_output (string, optional): Raw test output to parse
```

## 2. Request Flow & Concurrency Model

| Mode | Expected Duration | Pattern | Timeout |
|------|-------------------|---------|---------|
| `test_from_context.error` | 1-3s | Blocking (WaitForResult) | 5s |
| `test_from_context.interaction` | 2-5s | Blocking (WaitForResult) | 10s |
| `test_from_context.regression` | 2-5s | Blocking (WaitForResult) | 10s |
| `test_heal.analyze` | < 1s | Blocking (WaitForResult) | 3s |
| `test_heal.repair` | < 1s per selector | Blocking (WaitForResult) | 5s |
| `test_heal.batch` | 1-10s | Blocking (WaitForResult) | 10s |
| `test_classify.failure` | < 2s | Blocking (WaitForResult) | 5s |
| `test_classify.batch` | 2-10s | Blocking (WaitForResult) | 10s |

> **Note (P1-1 Resolution):** Batch timeouts reduced from 30s to 10s to align with MCP client expectations. Operations exceeding 10s should be split into smaller batches.

### Batch Operation Limits (P1-2 Resolution)

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max files per batch | 20 | Keeps total time under 10s |
| Max file size | 500KB | Prevents memory pressure |
| Max total batch size | 5MB | Memory safety |
| Max selectors per repair | 50 | DOM query performance |

### Concurrency Limits (C1-2 Resolution)

| Operation | Concurrent Limit | Scope |
|-----------|-----------------|-------|
| test_from_context | 2 | Per client |
| test_heal | 1 | Global (DOM queries) |
| test_classify | 5 | Per client |

Concurrency is enforced via semaphores in the Go server. Excess requests are queued with FIFO ordering.

### Request Flow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  MCP Client │────▶│  Go Server   │────▶│  Extension  │
│  (AI Agent) │     │  tools_core.go    │     │  (Browser)  │
└─────────────┘     └──────────────┘     └─────────────┘
       │                   │                    │
       │ generate          │                    │
       │ {type:test_*}     │                    │
       │──────────────────▶│                    │
       │                   │                    │
       │                   │ For test_heal:     │
       │                   │ Query current DOM  │
       │                   │───────────────────▶│
       │                   │                    │
       │                   │◀───────────────────│
       │                   │ DOM snapshot       │
       │                   │                    │
       │                   │ Generate/heal test │
       │                   │ (server-side)      │
       │                   │                    │
       │◀──────────────────│                    │
       │ Generated test    │                    │
```

## 3. Implementation Architecture

### Go Server Components (M1-1 Resolution)

**Decision:** Keep test-generation logic split by concern and route `generate` test modes through a dedicated `testGenHandler` sub-handler.

**Rationale:** This reduces `ToolHandler` coupling and keeps parsing/dispatch concerns separate from testgen domain logic. The split is now established and preferred.

```
cmd/browser-agent/
├── tools_generate.go                    # generate mode router (calls h.testGen())
├── tools_generate_testgen_handler.go    # testGenHandler wiring/accessor
├── testgen.go                           # test_from_context
├── testgen_heal.go                      # test_heal
├── testgen_classify.go                  # test_classify
├── testgen_provider_adapter.go          # capture/log adapter to internal/testgen
└── tools_core.go                        # ToolHandler composition
```

#### Current handler boundaries:
```go
// tools_generate.go
func (h *ToolHandler) toolGenerate(...)
// Routes "test_from_context", "test_heal", "test_classify" to h.testGen()

// tools_generate_testgen_handler.go
type testGenHandler struct { parent *ToolHandler }

// testgen.go / testgen_heal.go / testgen_classify.go
func (h *testGenHandler) handleGenerateTestFromContext(...)
func (h *testGenHandler) handleGenerateTestHeal(...)
func (h *testGenHandler) handleGenerateTestClassify(...)

// testgen_provider_adapter.go
// Adapts capture/log state for internal/testgen package APIs.
```

### Extension Components (Minimal)

Test generation is primarily server-side. Extension only needed for:
- DOM queries during selector healing
- Current page URL for context

```
extension/lib/
└── (no new files needed - uses existing dom-queries.js)
```

### Data Structures

```go
// testgen.go

// TestFromContextRequest represents generate {format: "test_from_context"} parameters
type TestFromContextRequest struct {
    Context      string `json:"context"`       // "error", "interaction", "regression"
    ErrorID      string `json:"error_id"`      // Optional: specific error to reproduce
    Framework    string `json:"framework"`     // "playwright", "vitest", "jest"
    OutputFormat string `json:"output_format"` // "file", "inline"
    BaseURL      string `json:"base_url"`
    IncludeMocks bool   `json:"include_mocks"`
}

// TestHealRequest represents generate {format: "test_heal"} parameters
type TestHealRequest struct {
    Action          string   `json:"action"`           // "analyze", "repair", "batch"
    TestFile        string   `json:"test_file"`
    TestDir         string   `json:"test_dir"`
    BrokenSelectors []string `json:"broken_selectors"`
    AutoApply       bool     `json:"auto_apply"`
}

// TestClassifyRequest represents generate {format: "test_classify"} parameters
type TestClassifyRequest struct {
    Action     string          `json:"action"` // "failure", "batch"
    Failure    *TestFailure    `json:"failure"`
    Failures   []TestFailure   `json:"failures"`
    TestOutput string          `json:"test_output"`
}

// TestFailure represents a single test failure to classify
type TestFailure struct {
    TestName   string `json:"test_name"`
    Error      string `json:"error"`
    Screenshot string `json:"screenshot"` // base64, optional
    Trace      string `json:"trace"`      // stack trace
    Duration   int64  `json:"duration_ms"`
}

// GeneratedTest represents the output of test generation
type GeneratedTest struct {
    Framework   string            `json:"framework"`
    Filename    string            `json:"filename"`
    Content     string            `json:"content"`
    Selectors   []string          `json:"selectors"`
    Assertions  int               `json:"assertions"`
    Coverage    TestCoverage      `json:"coverage"`
    Metadata    TestGenMetadata   `json:"metadata"`
}

// TestCoverage describes what the generated test covers
type TestCoverage struct {
    ErrorReproduced bool `json:"error_reproduced"`
    NetworkMocked   bool `json:"network_mocked"`
    StateCaptured   bool `json:"state_captured"`
}

// TestGenMetadata provides traceability
type TestGenMetadata struct {
    SourceError  string   `json:"source_error,omitempty"`
    GeneratedAt  string   `json:"generated_at"`
    ContextUsed  []string `json:"context_used"`
}

// HealedSelector represents a repaired selector
type HealedSelector struct {
    OldSelector string  `json:"old_selector"`
    NewSelector string  `json:"new_selector"`
    Confidence  float64 `json:"confidence"`
    Strategy    string  `json:"strategy"`
    LineNumber  int     `json:"line_number"`
}

// HealResult represents selector healing output
type HealResult struct {
    Healed         []HealedSelector `json:"healed"`
    Unhealed       []string         `json:"unhealed"`
    UpdatedContent string           `json:"updated_content,omitempty"`
}

// FailureClassification represents the result of classifying a test failure
type FailureClassification struct {
    Category          string   `json:"category"`
    Confidence        float64  `json:"confidence"`
    Evidence          []string `json:"evidence"`
    RecommendedAction string   `json:"recommended_action"`
    IsRealBug         bool     `json:"is_real_bug"`
    IsFlaky           bool     `json:"is_flaky"`
    IsEnvironment     bool     `json:"is_environment"`
    SuggestedFix      *SuggestedFix `json:"suggested_fix,omitempty"`
}

// SuggestedFix provides actionable fix suggestion
type SuggestedFix struct {
    Type string `json:"type"` // "selector_update", "add_wait", "mock_network", etc.
    Old  string `json:"old,omitempty"`
    New  string `json:"new,omitempty"`
    Code string `json:"code,omitempty"`
}
```

## 4. Test Generation Logic

### 4.1 Test from Error Context

```go
// GenerateTestFromError creates a test that reproduces a captured error
func (h *ToolHandler) GenerateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
    // 1. Get error context from capture
    // 2. Get actions leading up to error
    // 3. Get network requests during error
    // 4. Generate test using existing codegen infrastructure
    // 5. Add assertions for the error condition
}
```

#### Algorithm:
1. Lookup error by ID (or use most recent console error)
2. Get enhanced actions within ±5 seconds of error timestamp
3. Get network requests in same window
4. Use existing `generatePlaywrightScript()` for base test
5. Add error assertion at the end

#### Selector Priority (reuse from codegen.go):
1. `data-testid` / `data-test`
2. ARIA role + name
3. ARIA label
4. Visible text
5. ID attribute
6. CSS path (fallback)

### 4.2 Selector Healing

```go
// HealSelector attempts to find a working replacement for a broken selector
func (h *ToolHandler) HealSelector(oldSelector string, domSnapshot DOMSnapshot) *HealedSelector {
    // 1. Parse old selector to understand intent
    // 2. Search DOM for similar elements
    // 3. Score candidates by stability
    // 4. Return best match with confidence
}
```

#### Healing Strategies:

| Strategy | Description | Confidence Modifier |
|----------|-------------|-------------------|
| `testid_match` | Found matching data-testid | +0.3 |
| `aria_match` | Matched by role + accessible name | +0.2 |
| `text_match` | Same visible text content | +0.1 |
| `attribute_match` | Similar href, action, name attrs | +0.0 |
| `structural_match` | Same position in DOM tree | -0.2 |

#### Confidence Calculation:
```
base_confidence = 0.5
+ strategy_modifier
+ (element_uniqueness > 0.8 ? 0.2 : 0)
+ (attribute_overlap > 0.7 ? 0.1 : 0)
- (DOM_depth_change > 3 ? 0.2 : 0)
```

### 4.3 Failure Classification

```go
// ClassifyFailure analyzes a test failure and categorizes it
func (h *ToolHandler) ClassifyFailure(failure TestFailure) *FailureClassification {
    // 1. Parse error message for patterns
    // 2. Check if selector exists in current DOM
    // 3. Analyze timing patterns
    // 4. Check network conditions
    // 5. Return classification with evidence
}
```

#### Classification Patterns:

| Pattern | Category | Confidence |
|---------|----------|------------|
| "Timeout waiting for selector" + selector missing | `selector_broken` | 0.9 |
| "Timeout waiting for selector" + selector exists | `timing_flaky` | 0.8 |
| "net::ERR_" | `network_flaky` | 0.85 |
| "Expected X to be Y" + values differ | `real_bug` | 0.7 |
| "Element is not attached to DOM" | `timing_flaky` | 0.8 |
| "Element is outside viewport" | `test_bug` | 0.75 |

## 5. Response Schemas (D1-2 Resolution)

All responses use existing `mcpJSONResponse(summary, data)` pattern from tools_core.go.

### test_from_context Response

**Summary:** "Generated Playwright test 'error-form-validation.spec.ts' (3 assertions)"

#### Data:
```json
{
  "test": {
    "framework": "playwright",
    "filename": "error-form-validation.spec.ts",
    "content": "import { test, expect } from '@playwright/test';\n...",
    "selectors": ["#email", "button[type=submit]", ".error"],
    "assertions": 3,
    "coverage": {
      "error_reproduced": true,
      "network_mocked": false,
      "state_captured": true
    }
  },
  "metadata": {
    "source_error": "err_1706520600000_a1b2c3d4",
    "generated_at": "2026-01-29T10:30:00Z",
    "context_used": ["console", "dom", "network"]
  }
}
```

### test_heal Response

**Summary:** "Healed 1/2 selectors in login.spec.ts (1 unhealed, requires manual review)"

#### Data:
```json
{
  "healed": [
    {
      "old_selector": "#old-login-btn",
      "new_selector": "button[data-testid='login']",
      "confidence": 0.95,
      "strategy": "testid_match",
      "line_number": 12
    }
  ],
  "unhealed": [".complex-dynamic-element"],
  "updated_content": "// Full test file with applied fixes (only if auto_apply=true AND confidence >= 0.9)...",
  "summary": {
    "total_broken": 2,
    "healed_auto": 1,
    "healed_manual": 0,
    "unhealed": 1
  }
}
```

### test_classify Response

**Summary:** "Classified as selector_broken (92% confidence) — recommended: heal selector"

#### Data:
```json
{
  "classification": {
    "category": "selector_broken",
    "confidence": 0.92,
    "evidence": [
      "Selector '#submit-btn' not found in current DOM",
      "Similar element found: 'button[type=submit]'",
      "Element was likely renamed"
    ],
    "recommended_action": "heal",
    "is_real_bug": false,
    "is_flaky": false,
    "is_environment": false
  },
  "suggested_fix": {
    "type": "selector_update",
    "old": "#submit-btn",
    "new": "button[type=submit]"
  }
}
```

## 6. Error Handling

### Error Codes

| Code | Meaning | Retry Guidance |
|------|---------|----------------|
| `no_error_context` | No errors captured to generate test from | Trigger error first, then retry |
| `no_actions_captured` | No user actions recorded | Interact with page first |
| `test_file_not_found` | Specified test file doesn't exist | Check path |
| `invalid_test_syntax` | Test file has syntax errors | Fix syntax first |
| `selector_not_parseable` | Cannot parse broken selector | Use valid CSS selector |
| `dom_query_failed` | Failed to query current DOM | Check page is loaded |
| `classification_uncertain` | Could not classify with confidence | Provide more context |

### Graceful Degradation

1. **Missing error context** → Generate test from most recent actions only
2. **No network data** → Skip network assertions, add TODO comment
3. **Selector healing fails** → Return unhealed list with manual review guidance
4. **Classification uncertain** → Return `unknown` with evidence for human review

## 7. Security Considerations

### Path Validation (E1-1 Resolution)

Test file paths MUST be validated using existing `validatePathInDir()` from ai_persistence.go:

```go
// validateTestFilePath ensures path is within allowed directories
func validateTestFilePath(path string) error {
    // Must use validatePathInDir from ai_persistence.go
    // Allowed directories: project directory (from .kaboom or cwd)
    // Denied: paths starting with .., absolute paths outside project
    return validatePathInDir(path, getProjectDir())
}
```

| Path | Result |
|------|--------|
| `tests/login.spec.ts` | ✓ Allowed |
| `../../../etc/passwd` | ✗ Denied: traversal |
| `/etc/passwd` | ✗ Denied: absolute outside project |
| `tests/../tests/x.ts` | ✓ Allowed (normalized within project) |

### File Write Security Model (S1-1 Resolution)

#### File writes via `test_heal.repair` with `auto_apply: true` require:
1. AI Web Pilot toggle MUST be enabled (same as `interact` tool)
2. Path MUST pass `validateTestFilePath()` check
3. Confidence MUST be >= 0.9 for auto-apply

**Rationale:** test_heal modifies user files, equivalent to `interact` tool risk level.

```go
func (h *ToolHandler) handleTestHealRepair(req TestHealRequest) (*mcpResponse, error) {
    // 1. Check AI Web Pilot enabled
    if !h.isAIWebPilotEnabled() {
        return nil, fmt.Errorf("ai_web_pilot_disabled")
    }

    // 2. Validate path
    if err := validateTestFilePath(req.TestFile); err != nil {
        return nil, fmt.Errorf("path_not_allowed: %w", err)
    }

    // 3. Only write if auto_apply AND high confidence
    if req.AutoApply && healResult.Confidence >= 0.9 {
        // Write file
    }
}
```

### Selector Validation (S1-2 Resolution)

All selectors from `broken_selectors` parameter MUST be validated before DOM queries:

```go
// validateSelector ensures selector is safe for DOM query
func validateSelector(selector string) error {
    // Max length
    if len(selector) > 1000 {
        return fmt.Errorf("selector_too_long")
    }

    // Disallow script injection patterns
    dangerous := []string{"javascript:", "<script", "onerror=", "onload="}
    for _, pattern := range dangerous {
        if strings.Contains(strings.ToLower(selector), pattern) {
            return fmt.Errorf("selector_injection_detected")
        }
    }

    // Basic CSS selector syntax check (must start with valid chars)
    if !regexp.MustCompile(`^[a-zA-Z#.\[\*]`).MatchString(selector) {
        return fmt.Errorf("invalid_selector_syntax")
    }

    return nil
}
```

### Secret Detection

Scan generated tests for potential secrets, redact with `[REDACTED]`:

```go
var secretPatterns = []string{
    `(?i)api.?key`,
    `(?i)auth.?token`,
    `(?i)password`,
    `(?i)secret`,
    `Bearer [A-Za-z0-9-._~+/]+=*`,
    `(?i)aws.?access`,
    `(?i)aws.?secret`,
    `ghp_[A-Za-z0-9]{36}`,           // GitHub personal token
    `gho_[A-Za-z0-9]{36}`,           // GitHub OAuth token
    `-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`,
}
```

## 8. Integration Points

### With Existing Tools

| Tool | Integration |
|------|-------------|
| `observe` | Source of error context, actions, network data |
| `analyze` | Source of regression baselines for test generation |
| `interact` | Recorded user actions become test steps |
| `configure` | Test generation preferences (framework, output dir) |

### With External Systems

- **Playwright** — Generated tests are valid Playwright syntax
- **Vitest/Jest** — Alternative output formats for unit tests
- **CI/CD** — Tests can be run in any standard test runner

## 9. Performance Considerations

1. **DOM snapshot caching** — Cache DOM for selector healing (10 second TTL)
2. **Lazy test parsing** — Only parse test files when healing requested
3. **Batch optimization** — Process multiple selectors in single DOM query
4. **Streaming for batch** — Return results incrementally for large directories

## 10. Dependencies

### Required
- Existing Kaboom infrastructure (Go server, extension, MCP)
- Existing codegen.go (Playwright script generation)
- Existing DOM query infrastructure

### Optional
- None (all processing is server-side)

## 11. Implementation Phases

### Phase 1: test_from_context.error (Priority: HIGH)
- Generate Playwright tests from captured console errors
- Reuse existing codegen.go infrastructure
- Add error assertions

### Phase 2: test_heal.repair (Priority: HIGH)
- Single selector healing
- DOM query for candidate elements
- Confidence scoring

### Phase 3: test_classify.failure (Priority: MEDIUM)
- Pattern-based classification
- Evidence collection
- Suggested fixes

### Phase 4: Batch operations (Priority: LOW)
- test_heal.batch for directories
- test_classify.batch for test runs
- Async pattern with correlation IDs

## 12. New Error Codes

Add to tools_core.go:

```go
const (
    ErrNoErrorContext      = "no_error_context"
    ErrNoActionsCaptured   = "no_actions_captured"
    ErrTestFileNotFound    = "test_file_not_found"
    ErrInvalidTestSyntax   = "invalid_test_syntax"
    ErrSelectorNotParseable = "selector_not_parseable"
    ErrDOMQueryFailed      = "dom_query_failed"
    ErrClassificationUncertain = "classification_uncertain"
)
```

## 13. Testing Strategy

### Unit Tests (testgen_test.go)

| Test ID | Test Case | Expected |
|---------|-----------|----------|
| TG-001 | Generate test from single error | Valid Playwright test |
| TG-002 | Generate test with network context | Includes waitForResponse |
| TG-003 | Heal testid selector | High confidence match |
| TG-004 | Heal aria selector | Medium confidence match |
| TG-005 | Classify timeout as selector_broken | Category matches |
| TG-006 | Classify assertion failure as real_bug | is_real_bug=true |
| TG-007 | Secret detection in generated test | Value redacted |
| TG-008 | No actions → appropriate error | no_actions_captured |

### Integration Tests

| Test ID | Test Case | Expected |
|---------|-----------|----------|
| TG-INT-001 | End-to-end error→test→run | Test reproduces error |
| TG-INT-002 | Heal broken selector in real test | Test passes after heal |
| TG-INT-003 | Classify real test failure | Correct category |
