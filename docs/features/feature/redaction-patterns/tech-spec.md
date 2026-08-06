---
status: proposed
scope: feature/redaction-patterns/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-redaction-patterns
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-redaction-patterns.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Redaction Patterns Review](./index.md).

# Technical Spec: Configurable Redaction Patterns (Feature 19)

## Overview

Kaboom's redaction engine scrubs sensitive data from MCP tool responses before they reach AI clients. Currently, redaction uses a fixed set of built-in patterns loaded at startup. This feature adds user-defined regex patterns via an MCP tool, supporting custom patterns for domain-specific sensitive data (internal account numbers, proprietary tokens, PII formats), with configurable replacement strategies (mask, hash, remove) and per-field targeting.

---

## Requirements

### Functional Requirements

1. **User-defined patterns** — AI agents can add custom redaction patterns at runtime without server restart
2. **Replacement strategies** — Support mask (partial reveal), hash (deterministic replacement), and remove (complete deletion) strategies
3. **Per-field targeting** — Patterns can target specific JSON paths (e.g., `$.user.ssn`) or apply globally
4. **Pattern priority** — Explicit ordering when multiple patterns could match the same content
5. **Named groups** — Regex patterns can use named capture groups to preserve partial content (e.g., keep card type prefix)
6. **Validation feedback** — Invalid patterns (malformed regex, PCRE-only features) return clear error messages
7. **Pattern persistence** — Patterns persist for the server session; no disk persistence required

### Non-Functional Requirements

1. **Performance** — Redaction of a 50KB response must complete in under 10ms
2. **Memory** — Custom pattern storage must not exceed 500KB
3. **Thread safety** — Pattern updates must be safe for concurrent tool calls
4. **Backward compatibility** — Built-in patterns continue to work unchanged

---

## Data Model

### Go Structs

```go
// RedactionStrategy defines how matched content is replaced.
type RedactionStrategy string

const (
    StrategyMask   RedactionStrategy = "mask"   // Show first N and last M chars
    StrategyHash   RedactionStrategy = "hash"   // SHA-256 truncated, prefixed with pattern name
    StrategyRemove RedactionStrategy = "remove" // Replace with [REDACTED:name]
)

// RedactionPatternConfig represents a user-configured redaction rule.
type RedactionPatternConfig struct {
    ID          string            `json:"id"`                    // Unique identifier (auto-generated if empty)
    Name        string            `json:"name"`                  // Human-readable name (e.g., "internal-account")
    Pattern     string            `json:"pattern"`               // RE2 regex pattern
    Strategy    RedactionStrategy `json:"strategy"`              // mask, hash, or remove
    Replacement string            `json:"replacement,omitempty"` // Custom replacement (overrides strategy)
    Priority    int               `json:"priority"`              // Higher = matched first (default: 0)
    Enabled     bool              `json:"enabled"`               // Toggle without deleting

    // Mask strategy options
    MaskConfig *MaskConfig `json:"mask_config,omitempty"`

    // Field targeting (nil = global)
    FieldPaths []string `json:"field_paths,omitempty"` // JSON paths to target (e.g., "$.user.ssn")

    // Metadata
    CreatedAt time.Time `json:"created_at"`
    Source    string    `json:"source"` // "builtin", "user", "auto"
}

// MaskConfig controls partial masking behavior.
type MaskConfig struct {
    ShowFirst int    `json:"show_first"` // Characters to show at start (default: 4)
    ShowLast  int    `json:"show_last"`  // Characters to show at end (default: 4)
    MaskChar  string `json:"mask_char"`  // Replacement character (default: "*")
}

// CompiledRedactionPattern holds a pre-compiled pattern ready for matching.
type CompiledRedactionPattern struct {
    Config      RedactionPatternConfig
    Regex       *regexp.Regexp
    FieldRegex  []*regexp.Regexp // Compiled JSON path patterns
    ValidateFn  func(string) bool // Optional post-match validation (e.g., Luhn)
}

// RedactionConfigResponse is returned by configure_redaction list action.
type RedactionConfigResponse struct {
    Patterns  []RedactionPatternConfig `json:"patterns"`
    Stats     RedactionStats           `json:"stats"`
    Builtins  int                      `json:"builtin_count"`
    Custom    int                      `json:"custom_count"`
}

// RedactionStats tracks redaction activity.
type RedactionStats struct {
    TotalRedactions    int64            `json:"total_redactions"`
    RedactionsByPattern map[string]int64 `json:"by_pattern"`    // pattern ID -> count
    LastRedactionAt    *time.Time       `json:"last_redaction_at,omitempty"`
}
```

### JSON Schema for Pattern Definitions

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "minLength": 1,
      "maxLength": 64,
      "pattern": "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
      "description": "Kebab-case identifier for the pattern"
    },
    "pattern": {
      "type": "string",
      "minLength": 1,
      "maxLength": 1024,
      "description": "RE2-compatible regex pattern"
    },
    "strategy": {
      "type": "string",
      "enum": ["mask", "hash", "remove"],
      "default": "remove"
    },
    "replacement": {
      "type": "string",
      "maxLength": 256,
      "description": "Custom replacement text (overrides strategy)"
    },
    "priority": {
      "type": "integer",
      "minimum": -100,
      "maximum": 100,
      "default": 0
    },
    "enabled": {
      "type": "boolean",
      "default": true
    },
    "mask_config": {
      "type": "object",
      "properties": {
        "show_first": {"type": "integer", "minimum": 0, "maximum": 20, "default": 4},
        "show_last": {"type": "integer", "minimum": 0, "maximum": 20, "default": 4},
        "mask_char": {"type": "string", "minLength": 1, "maxLength": 1, "default": "*"}
      }
    },
    "field_paths": {
      "type": "array",
      "items": {"type": "string", "pattern": "^\\$\\."},
      "maxItems": 20,
      "description": "JSON paths to target (e.g., $.user.ssn)"
    }
  },
  "required": ["name", "pattern"]
}
```

---

## API Surface

### MCP Tool: `configure_redaction`

```json
{
  "name": "configure_redaction",
  "description": "Configure custom redaction patterns for sensitive data. Patterns use RE2 regex syntax (Go's regexp). Supports mask (partial reveal), hash (deterministic), and remove strategies. Use field_paths to target specific JSON fields or leave empty for global matching.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["add", "update", "remove", "enable", "disable", "list", "test", "clear"],
        "description": "Action to perform on redaction patterns"
      },
      "patterns": {
        "type": "array",
        "items": {"$ref": "#/definitions/pattern"},
        "description": "Patterns to add or update (for 'add' and 'update' actions)"
      },
      "pattern_id": {
        "type": "string",
        "description": "Pattern ID (for 'remove', 'enable', 'disable' actions)"
      },
      "test_input": {
        "type": "string",
        "description": "Sample text to test patterns against (for 'test' action)"
      },
      "test_patterns": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Specific pattern IDs to test (empty = all patterns)"
      }
    },
    "required": ["action"]
  }
}
```

### Action Behaviors

#### `add` — Add New Patterns

Adds one or more custom redaction patterns. Each pattern is validated (regex compiles, no PCRE-only features) before being added.

##### Request:
```json
{
  "action": "add",
  "patterns": [
    {
      "name": "internal-account",
      "pattern": "ACC-[0-9]{8}",
      "strategy": "mask",
      "mask_config": {"show_first": 4, "show_last": 2}
    },
    {
      "name": "employee-id",
      "pattern": "EMP(?P<dept>[A-Z]{2})-[0-9]{6}",
      "strategy": "hash",
      "priority": 10
    }
  ]
}
```

##### Response:
```json
{
  "action": "added",
  "added": [
    {"id": "user_internal-account_a1b2c3", "name": "internal-account", "valid": true},
    {"id": "user_employee-id_d4e5f6", "name": "employee-id", "valid": true}
  ],
  "errors": [],
  "total_patterns": 12
}
```

#### `update` — Modify Existing Pattern

Updates an existing pattern by ID. Partial updates are supported; omitted fields retain their current values.

##### Request:
```json
{
  "action": "update",
  "patterns": [
    {
      "id": "user_internal-account_a1b2c3",
      "priority": 50,
      "enabled": false
    }
  ]
}
```

#### `remove` — Delete Pattern

Removes a custom pattern by ID. Built-in patterns cannot be removed (returns error).

##### Request:
```json
{
  "action": "remove",
  "pattern_id": "user_internal-account_a1b2c3"
}
```

##### Response:
```json
{
  "action": "removed",
  "pattern_id": "user_internal-account_a1b2c3",
  "name": "internal-account"
}
```

##### Error (built-in):
```json
{
  "action": "remove",
  "error": "Cannot remove built-in pattern 'aws-key'. Use 'disable' to turn it off temporarily."
}
```

#### `enable` / `disable` — Toggle Pattern

Enables or disables a pattern without deleting it. Works for both built-in and custom patterns.

##### Request:
```json
{
  "action": "disable",
  "pattern_id": "builtin_ssn"
}
```

#### `list` — Show All Patterns

Returns all configured patterns with statistics.

##### Response:
```json
{
  "action": "listed",
  "patterns": [
    {
      "id": "builtin_aws-key",
      "name": "aws-key",
      "pattern": "AKIA[0-9A-Z]{16}",
      "strategy": "remove",
      "priority": 100,
      "enabled": true,
      "source": "builtin",
      "match_count": 3
    },
    {
      "id": "user_internal-account_a1b2c3",
      "name": "internal-account",
      "pattern": "ACC-[0-9]{8}",
      "strategy": "mask",
      "mask_config": {"show_first": 4, "show_last": 2},
      "priority": 0,
      "enabled": true,
      "source": "user",
      "created_at": "2025-01-26T10:30:00Z",
      "match_count": 47
    }
  ],
  "stats": {
    "total_redactions": 156,
    "by_pattern": {
      "builtin_aws-key": 3,
      "user_internal-account_a1b2c3": 47
    },
    "last_redaction_at": "2025-01-26T10:45:00Z"
  },
  "builtin_count": 10,
  "custom_count": 2
}
```

#### `test` — Test Patterns Against Sample Input

Tests patterns against sample text without performing actual redaction. Returns which patterns would match and what the output would look like.

##### Request:
```json
{
  "action": "test",
  "test_input": "Account ACC-12345678 belongs to employee EMPHR-123456",
  "test_patterns": []
}
```

##### Response:
```json
{
  "action": "tested",
  "input": "Account ACC-12345678 belongs to employee EMPHR-123456",
  "matches": [
    {
      "pattern_id": "user_internal-account_a1b2c3",
      "pattern_name": "internal-account",
      "matched_text": "ACC-12345678",
      "position": {"start": 8, "end": 20},
      "replacement": "ACC-****78"
    },
    {
      "pattern_id": "user_employee-id_d4e5f6",
      "pattern_name": "employee-id",
      "matched_text": "EMPHR-123456",
      "position": {"start": 42, "end": 54},
      "replacement": "[HASH:employee-id:a7f3...]"
    }
  ],
  "output": "Account ACC-****78 belongs to employee [HASH:employee-id:a7f3...]"
}
```

#### `clear` — Remove All Custom Patterns

Removes all user-defined patterns, reverting to built-ins only.

##### Response:
```json
{
  "action": "cleared",
  "removed_count": 5,
  "remaining_builtin_count": 10
}
```

---

## Implementation Details

### Pattern Compilation and Caching

```go
// RedactionManager handles pattern lifecycle and matching.
type RedactionManager struct {
    mu       sync.RWMutex
    patterns []CompiledRedactionPattern  // Sorted by priority (descending)
    stats    RedactionStats

    // Cache for field path extraction
    fieldCache map[string][]string  // JSON path -> extracted values
}

// Compile compiles a pattern config into a ready-to-use matcher.
func (m *RedactionManager) Compile(config RedactionPatternConfig) (*CompiledRedactionPattern, error) {
    // Validate pattern is RE2-compatible
    regex, err := regexp.Compile(config.Pattern)
    if err != nil {
        return nil, fmt.Errorf("invalid regex pattern: %w", err)
    }

    // Compile field path patterns (convert JSONPath to regex)
    var fieldRegexes []*regexp.Regexp
    for _, path := range config.FieldPaths {
        fieldRegex, err := compileJSONPath(path)
        if err != nil {
            return nil, fmt.Errorf("invalid field path %q: %w", path, err)
        }
        fieldRegexes = append(fieldRegexes, fieldRegex)
    }

    // Add validation function for patterns that need it
    var validateFn func(string) bool
    if config.Name == "credit-card" || strings.Contains(config.Pattern, "[0-9]{4}[- ]?[0-9]{4}") {
        validateFn = luhnValidateMatch
    }

    return &CompiledRedactionPattern{
        Config:     config,
        Regex:      regex,
        FieldRegex: fieldRegexes,
        ValidateFn: validateFn,
    }, nil
}

// AddPatterns adds patterns and recompiles the sorted list.
func (m *RedactionManager) AddPatterns(configs []RedactionPatternConfig) ([]string, []error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    var added []string
    var errors []error

    for _, config := range configs {
        // Generate ID if not provided
        if config.ID == "" {
            config.ID = fmt.Sprintf("user_%s_%s", config.Name, randomID(6))
        }
        config.CreatedAt = time.Now()
        config.Source = "user"

        compiled, err := m.Compile(config)
        if err != nil {
            errors = append(errors, fmt.Errorf("pattern %q: %w", config.Name, err))
            continue
        }

        m.patterns = append(m.patterns, *compiled)
        added = append(added, config.ID)
    }

    // Re-sort by priority (descending)
    sort.Slice(m.patterns, func(i, j int) bool {
        return m.patterns[i].Config.Priority > m.patterns[j].Config.Priority
    })

    return added, errors
}
```

### Replacement Strategies

```go
// applyStrategy applies the configured replacement strategy to a match.
func applyStrategy(pattern CompiledRedactionPattern, match string) string {
    config := pattern.Config

    // Custom replacement takes precedence
    if config.Replacement != "" {
        return expandNamedGroups(config.Replacement, pattern.Regex, match)
    }

    switch config.Strategy {
    case StrategyMask:
        return maskString(match, config.MaskConfig)
    case StrategyHash:
        return hashString(match, config.Name)
    case StrategyRemove:
        fallthrough
    default:
        return fmt.Sprintf("[REDACTED:%s]", config.Name)
    }
}

// maskString applies partial masking.
func maskString(s string, config *MaskConfig) string {
    if config == nil {
        config = &MaskConfig{ShowFirst: 4, ShowLast: 4, MaskChar: "*"}
    }

    runes := []rune(s)
    length := len(runes)

    // Handle short strings
    if length <= config.ShowFirst+config.ShowLast {
        return strings.Repeat(config.MaskChar, length)
    }

    prefix := string(runes[:config.ShowFirst])
    suffix := string(runes[length-config.ShowLast:])
    maskLen := length - config.ShowFirst - config.ShowLast

    return prefix + strings.Repeat(config.MaskChar, maskLen) + suffix
}

// hashString creates a deterministic hash-based replacement.
func hashString(s string, patternName string) string {
    hash := sha256.Sum256([]byte(s))
    // Use first 8 chars of hex for brevity
    shortHash := hex.EncodeToString(hash[:])[:8]
    return fmt.Sprintf("[HASH:%s:%s]", patternName, shortHash)
}

// expandNamedGroups expands $name and ${name} references in replacement string.
func expandNamedGroups(replacement string, regex *regexp.Regexp, match string) string {
    submatches := regex.FindStringSubmatch(match)
    if submatches == nil {
        return replacement
    }

    names := regex.SubexpNames()
    result := replacement

    for i, name := range names {
        if name != "" && i < len(submatches) {
            result = strings.ReplaceAll(result, "$"+name, submatches[i])
            result = strings.ReplaceAll(result, "${"+name+"}", submatches[i])
        }
    }

    return result
}
```

### Per-Field Pattern Matching

```go
// RedactJSON applies patterns to specific JSON fields or globally.
func (m *RedactionManager) RedactJSON(input json.RawMessage) json.RawMessage {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var data interface{}
    if err := json.Unmarshal(input, &data); err != nil {
        // Fallback to string-level redaction
        return json.RawMessage(m.RedactString(string(input)))
    }

    // Apply field-targeted patterns first
    for _, pattern := range m.patterns {
        if !pattern.Config.Enabled {
            continue
        }
        if len(pattern.Config.FieldPaths) > 0 {
            data = m.redactFields(data, pattern)
        }
    }

    // Re-serialize and apply global patterns
    output, _ := json.Marshal(data)
    result := m.RedactString(string(output))

    return json.RawMessage(result)
}

// redactFields applies a pattern only to matching JSON paths.
func (m *RedactionManager) redactFields(data interface{}, pattern CompiledRedactionPattern) interface{} {
    for _, path := range pattern.Config.FieldPaths {
        // Walk the JSON structure and redact matching fields
        data = walkAndRedact(data, path, pattern, "")
    }
    return data
}

// walkAndRedact recursively walks JSON and redacts at matching paths.
func walkAndRedact(data interface{}, targetPath string, pattern CompiledRedactionPattern, currentPath string) interface{} {
    switch v := data.(type) {
    case map[string]interface{}:
        result := make(map[string]interface{})
        for key, value := range v {
            newPath := currentPath + "." + key
            if currentPath == "" {
                newPath = "$." + key
            }

            if pathMatches(newPath, targetPath) {
                // This field matches - redact its string value
                if str, ok := value.(string); ok {
                    result[key] = applyPatternToString(pattern, str)
                } else {
                    result[key] = value
                }
            } else {
                // Recurse into nested structures
                result[key] = walkAndRedact(value, targetPath, pattern, newPath)
            }
        }
        return result

    case []interface{}:
        result := make([]interface{}, len(v))
        for i, item := range v {
            arrayPath := fmt.Sprintf("%s[%d]", currentPath, i)
            result[i] = walkAndRedact(item, targetPath, pattern, arrayPath)
        }
        return result

    default:
        return data
    }
}

// pathMatches checks if a current path matches a target path pattern.
// Supports wildcards: $.users[*].email matches $.users[0].email, $.users[1].email, etc.
func pathMatches(current, target string) bool {
    // Convert target to regex: $.users[*].email -> ^\$\.users\[\d+\]\.email$
    regexPattern := regexp.QuoteMeta(target)
    regexPattern = strings.ReplaceAll(regexPattern, `\[\*\]`, `\[\d+\]`)
    regexPattern = strings.ReplaceAll(regexPattern, `\*`, `[^.]+`)
    regexPattern = "^" + regexPattern + "$"

    matched, _ := regexp.MatchString(regexPattern, current)
    return matched
}
```

### Priority and Ordering

Patterns are processed in priority order (highest first). When multiple patterns could match overlapping content:

1. Higher priority patterns match first
2. Once content is redacted by one pattern, it is not re-matched by subsequent patterns
3. Equal priority patterns are processed in order of addition (FIFO)

```go
// RedactString applies all enabled patterns in priority order.
func (m *RedactionManager) RedactString(input string) string {
    if input == "" {
        return ""
    }

    result := input
    redactedRanges := make([]struct{ start, end int }, 0)

    for _, pattern := range m.patterns {
        if !pattern.Config.Enabled {
            continue
        }

        // Skip field-targeted patterns for global string redaction
        if len(pattern.Config.FieldPaths) > 0 {
            continue
        }

        result = pattern.Regex.ReplaceAllStringFunc(result, func(match string) string {
            // Check if this range was already redacted
            matchStart := strings.Index(result, match)
            for _, r := range redactedRanges {
                if matchStart >= r.start && matchStart < r.end {
                    return match // Already redacted, skip
                }
            }

            // Apply validation if present
            if pattern.ValidateFn != nil && !pattern.ValidateFn(match) {
                return match
            }

            // Track statistics
            m.incrementStat(pattern.Config.ID)

            replacement := applyStrategy(pattern, match)
            redactedRanges = append(redactedRanges, struct{ start, end int }{
                matchStart, matchStart + len(replacement),
            })

            return replacement
        })
    }

    return result
}
```

---

## Built-in Patterns vs Custom Patterns

### Built-in Patterns

Built-in patterns ship with the server and are always loaded. They have:

- IDs prefixed with `builtin_`
- Priority 100 (high, to catch sensitive data before custom patterns)
- Source: `"builtin"`
- Cannot be removed (only disabled)

| Name | Pattern | Strategy | Priority | Validation |
|------|---------|----------|----------|------------|
| `aws-key` | `AKIA[0-9A-Z]{16}` | remove | 100 | - |
| `bearer-token` | `Bearer [A-Za-z0-9\-._~+/]+=*` | remove | 100 | - |
| `basic-auth` | `Basic [A-Za-z0-9+/]+=*` | remove | 100 | - |
| `jwt` | `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+` | remove | 100 | - |
| `github-pat` | `(ghp_[A-Za-z0-9]{36,}\|github_pat_[A-Za-z0-9_]{36,})` | remove | 100 | - |
| `private-key` | `-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----` | remove | 100 | - |
| `credit-card` | `\b([0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4})\b` | mask | 90 | Luhn |
| `ssn` | `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b` | mask | 90 | - |
| `api-key` | `(?i)(api[_-]?key\|apikey\|secret[_-]?key)\s*[:=]\s*\S+` | remove | 80 | - |
| `session-cookie` | `(?i)(session\|sid\|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}` | remove | 80 | - |

### Custom Patterns

Custom patterns are added at runtime via `configure_redaction`. They have:

- IDs prefixed with `user_`
- Default priority 0 (can be set 100 to -100)
- Source: `"user"`
- Can be removed, updated, enabled, or disabled

### Priority Guidelines

| Priority Range | Use Case |
|----------------|----------|
| 100+ | Reserved for built-ins (critical security) |
| 50-99 | Custom patterns that must run before most built-ins |
| 1-49 | High-priority custom patterns |
| 0 | Default custom pattern priority |
| -1 to -49 | Lower-priority patterns (run after defaults) |
| -50 to -100 | Catch-all patterns (run last) |

---

## Performance Considerations

### Regex Compilation

- Patterns are compiled once when added and cached
- Recompilation only occurs on add/update/remove operations
- Invalid patterns fail fast at add time, not at match time

### Matching Performance

```go
// Benchmark targets (50KB response, 20 patterns):
// - String redaction: < 5ms
// - JSON redaction with field paths: < 10ms
// - Pattern test: < 1ms

const (
    maxPatterns       = 100   // Maximum custom patterns allowed
    maxPatternSize    = 1024  // Maximum regex pattern length
    maxFieldPaths     = 20    // Maximum field paths per pattern
    maxResponseSize   = 1<<20 // 1MB max response for redaction
)
```

### Caching Strategy

```go
type RedactionCache struct {
    mu    sync.RWMutex
    cache map[uint64]string // hash(input + patternVersion) -> redacted output

    maxSize    int   // Maximum cache entries (default: 1000)
    patternVer int64 // Incremented on pattern changes
}

// Get returns cached redaction result if available.
func (c *RedactionCache) Get(input string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    key := hashKey(input, c.patternVer)
    result, ok := c.cache[key]
    return result, ok
}

// Set stores a redaction result in cache.
func (c *RedactionCache) Set(input, output string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Evict if at capacity (LRU would be better, but simple is fine here)
    if len(c.cache) >= c.maxSize {
        // Clear half the cache
        count := 0
        for k := range c.cache {
            delete(c.cache, k)
            count++
            if count >= c.maxSize/2 {
                break
            }
        }
    }

    key := hashKey(input, c.patternVer)
    c.cache[key] = output
}

// Invalidate clears cache when patterns change.
func (c *RedactionCache) Invalidate() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.patternVer++
    c.cache = make(map[uint64]string)
}
```

### Memory Budget

| Component | Max Memory |
|-----------|------------|
| Compiled patterns (100 max) | 200KB |
| Pattern configs | 100KB |
| Field path cache | 50KB |
| Redaction cache | 100KB |
| Statistics | 50KB |
| **Total** | **500KB** |

---

## Testing Strategy

### Unit Tests

#### Pattern Compilation

1. Valid RE2 pattern compiles successfully
2. PCRE-only features (lookahead, lookbehind) return clear error
3. Empty pattern returns error
4. Pattern exceeding max length returns error
5. Named groups are extracted correctly

#### Replacement Strategies

6. Mask strategy respects ShowFirst/ShowLast config
7. Mask strategy handles strings shorter than ShowFirst+ShowLast
8. Hash strategy produces consistent output for same input
9. Hash strategy produces different output for different inputs
10. Remove strategy uses pattern name in placeholder
11. Custom replacement overrides strategy
12. Named group expansion works in custom replacement

#### Per-Field Matching

13. Field path `$.user.ssn` matches only that field
14. Field path `$.users[*].email` matches all array elements
15. Field path with wildcard `$.*.secret` matches any top-level key
16. Non-matching paths leave content unchanged
17. Nested field paths work correctly

#### Priority and Ordering

18. Higher priority patterns match first
19. Already-redacted content is not re-matched
20. Equal priority patterns process in FIFO order
21. Disabled patterns are skipped

#### Built-in Patterns

22. All built-in patterns are loaded at startup
23. Built-in patterns cannot be removed (returns error)
24. Built-in patterns can be disabled
25. Disabled built-ins can be re-enabled

#### Validation Functions

26. Credit card pattern requires Luhn validation
27. Valid Luhn numbers are redacted
28. Invalid Luhn numbers are not redacted
29. Custom patterns can have validation functions

### Integration Tests

30. `configure_redaction` add action creates pattern
31. `configure_redaction` update action modifies pattern
32. `configure_redaction` remove action deletes custom pattern
33. `configure_redaction` list action returns all patterns with stats
34. `configure_redaction` test action shows matches without persisting
35. `configure_redaction` clear action removes all custom patterns
36. Patterns persist across multiple tool calls in same session
37. Patterns are applied to all MCP tool responses

### Performance Tests

38. 50KB response redacts in < 10ms with 20 patterns
39. Pattern compilation completes in < 5ms
40. Adding 100 patterns completes in < 50ms
41. Cache hit returns in < 0.1ms

### Edge Cases

42. Empty input returns empty output
43. No patterns returns input unchanged
44. Malformed JSON falls back to string redaction
45. Very long match (> 10KB) is handled correctly
46. Unicode in patterns and content works correctly
47. Concurrent pattern updates and redactions are thread-safe
48. Pattern with only field paths doesn't affect global strings
49. Pattern with no matches doesn't error

---

## File Locations

| File | Purpose |
|------|---------|
| `cmd/browser-agent/redaction.go` | Existing redaction engine (extend) |
| `cmd/browser-agent/redaction_config.go` | New: Pattern configuration management |
| `cmd/browser-agent/redaction_test.go` | Existing tests (extend) |
| `cmd/browser-agent/redaction_config_test.go` | New: Configuration tests |
| `cmd/browser-agent/tools_core.go` | Add `configure_redaction` tool registration |

---

## Migration Notes

The existing `RedactionEngine` in `redaction.go` will be extended rather than replaced:

1. Rename `RedactionEngine` to `RedactionManager` for clarity
2. Add pattern management methods (Add, Update, Remove, etc.)
3. Add field-path targeting to the redaction pipeline
4. Add statistics tracking
5. Add caching layer
6. Existing `NewRedactionEngine(configPath)` continues to work for file-based config
7. New `NewRedactionManager()` constructor for MCP-configured patterns

The MCP tool `configure_redaction` provides runtime configuration, while the JSON config file provides startup defaults. Both sources are merged with MCP-added patterns taking precedence for same-named patterns.
