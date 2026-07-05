---
status: proposed
scope: feature/normalized-log-schema
ai-priority: medium
tags: [v7, logging, standardization]
relates-to: [product-spec.md, ../backend-log-streaming/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-normalized-log-schema
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Normalized Log Schema — Technical Specification

## Architecture

### Log Processing Pipeline
```
Raw Log Entry
    ↓
Format Detection (JSON? Text? Syslog?)
    ↓
Parser Selection
    ↓
Field Extraction (timestamp, level, message, context)
    ↓
Normalization (convert to canonical format)
    ↓
Enrichment (add service, hostname, request_id)
    ↓
Normalized Log Store
    ↓
Queryable via MCP
```

### Components
1. **Format Detector** (`server/parsing/format-detector.go`)
   - Heuristics: JSON (starts with {), syslog (starts with timestamp), text (other)

2. **Parser Factory** (`server/parsing/parser.go`)
   - Instantiate appropriate parser based on format and service config

3. **Parsers** (`server/parsing/parsers/`)
   - `json_parser.go`: Configured field mapping
   - `text_parser.go`: Regex-based extraction
   - `syslog_parser.go`: RFC 3164/5424 support
   - `cloudwatch_parser.go`: CloudWatch log format

4. **Normalizer** (`server/parsing/normalizer.go`)
   - Convert parsed fields to canonical schema
   - Validate and coerce types
   - Handle timezone conversion

5. **Log Query Handler** (`server/handlers.go`)
   - Query normalized logs with filtering
   - Support field-based queries

## Implementation Plan

### Phase 1: Core Parsers (Week 1)
1. Implement JSON parser with field mapping
2. Implement text parser with regex support
3. Implement format detection heuristics
4. Test with real logs from Go, Node.js, Python

### Phase 2: Normalization (Week 2)
1. Implement normalizer (canonical schema)
2. Add enrichment (service, hostname, request_id extraction)
3. Implement type coercion (numbers, timestamps, enums)

### Phase 3: Configuration (Week 3)
1. Define service parser configuration format (YAML)
2. Allow per-service parser customization
3. Support regex field extraction patterns

### Phase 4: Query Handler (Week 4)
1. Implement MCP query handler for normalized logs
2. Support filtering by level, service, field values
3. Performance testing with mixed log types

## API Changes

### Service Parser Configuration (YAML)
```yaml
log_parsers:
  payment-service:
    format: json
    timestamp_field: time
    level_field: level
    message_field: msg
    service_field: null  # Use default "payment-service"
    hostname_field: hostname
    context_fields: [user_id, error_code, duration_ms]

  api-server:
    format: text
    pattern: '(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z) \[(\w+)\] (.*)'
    groups:
      timestamp: 0
      level: 1
      message: 2
    context_extraction:
      user_id: 'user_id[=:]\s*(\d+)'
      request_id: 'req[=:]\s*([a-z0-9\-]+)'

  syslog-service:
    format: syslog
    rfc: 5424  # or 3164
```

### MCP Query Handler
```go
type NormalizedLogsRequest struct {
    Level      string                 // "ERROR", "WARN", etc.
    Service    string                 // "payment-service", "*"
    Message    string                 // Substring match
    Fields     map[string]interface{} // Field queries: {"user_id": 123}
    Since      *time.Time
    Until      *time.Time
    Limit      int
    Cursor     string
}

type NormalizedLogsResponse struct {
    Logs       []NormalizedLogEntry
    NextCursor string
    Total      int
}
```

## Code References
- **Format detector:** `/Users/brenn/dev/kaboom/server/parsing/format-detector.go` (new)
- **Parsers:** `/Users/brenn/dev/kaboom/server/parsing/parsers/` (new directory)
- **Normalizer:** `/Users/brenn/dev/kaboom/server/parsing/normalizer.go` (new)
- **Config:** `/Users/brenn/dev/kaboom/config/log-parsers.yaml` (new)
- **MCP handler:** `/Users/brenn/dev/kaboom/server/handlers.go` (modified)
- **Tests:** `/Users/brenn/dev/kaboom/server/parsing/parsing_test.go` (new)

## Performance Requirements
- **Parse latency:** <1ms per log entry
- **Query latency:** <100ms for 1000 logs
- **Format detection:** <0.1ms per log
- **No memory overhead:** Parsed logs use same schema

## Testing Strategy

### Unit Tests
- Each parser tested with real logs from respective language/framework
- Format detection accuracy
- Type coercion (string to number, ISO 8601 parsing)
- Regex pattern matching

### Integration Tests
- Parser configuration loading
- Mixed log types in same query
- Field extraction accuracy
- Performance under load (10K logs/sec)

### E2E Tests
- Real services logging in different formats
- Query across all services
- Field-based filtering works correctly

## Dependencies
- **Regex support:** (Go stdlib)
- **JSON support:** (Go stdlib)
- **Syslog RFC library:** (external, lightweight)

## Risks & Mitigation
1. **Regex performance on malformed logs**
   - Mitigation: Timeout regex, fallback to text parser
2. **Parser configuration errors**
   - Mitigation: Validate config at startup, fail fast
3. **Type coercion failures**
   - Mitigation: Keep original value if coercion fails
4. **Field extraction mismatches**
   - Mitigation: Log parse failures with original log snippet

## Configuration Validation
- Validate YAML syntax at startup
- Test regex patterns with sample logs
- Verify field mappings
- Report errors to user before starting

## Backward Compatibility
- Raw logs still stored for debugging
- Normalized schema is additive (doesn't replace raw logs)
- Can disable normalization per service if needed
- Gradual rollout: enable per service
