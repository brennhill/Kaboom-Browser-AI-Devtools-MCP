---
status: proposed
scope: feature/normalized-log-schema
ai-priority: medium
tags: [v7, logging, standardization, eyes]
relates-to: [../backend-log-streaming.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-normalized-log-schema
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Normalized Log Schema

## Overview
Normalized Log Schema automatically parses heterogeneous backend logs (from Go, Node.js, Python, Java) and converts them into a unified structure. Backend services log in wildly different formats: some use JSON, others use text with "[LEVEL]" prefixes, others use syslog format. Kaboom parses each format, extracts structured fields (timestamp, level, message, context), and normalizes them into a canonical format. This enables querying and correlating logs across services written in different languages, each with its own logging convention.

## Problem
Logs from different services use incompatible formats, making cross-service analysis difficult:
- Go service: `time="2026-01-31T10:15:23Z" level=error msg="Payment failed" user_id=123`
- Node.js service: `[10:15:23] ERROR: Payment failed - user_id: 123`
- Python service: `2026-01-31 10:15:23 ERROR - Payment failed - {'user_id': 123}`

Query like "find all ERROR logs with user_id=123" must be custom per service, or logs must be manually reformatted.

## Solution
Normalized Log Schema:
1. **Format Detection:** Identify log format (JSON, structured text, syslog)
2. **Parser Selection:** Choose appropriate parser
3. **Field Extraction:** Extract timestamp, level, message, context
4. **Normalization:** Convert to canonical schema
5. **Enrichment:** Add metadata (source service, hostname)

## User Stories
- As a platform engineer, I want to query logs across all services with consistent syntax so that I don't need custom dashboards for each language
- As a DevOps engineer, I want to find all errors related to user 123 regardless of which service logged them
- As a SRE, I want to identify anomalies (unusual error rates, latency spikes) across all services

## Acceptance Criteria
- [ ] Support parsers for: JSON, text with log level prefix, syslog, CloudWatch format
- [ ] Extract: timestamp (ISO 8601), level, message, service, hostname
- [ ] Preserve original log fields in extra_fields map
- [ ] Support custom field extraction via regex patterns
- [ ] Configurable per service (JSON vs. text vs. syslog)
- [ ] Query normalized logs: `observe({what: 'normalized-logs', level: 'ERROR', service: 'api-server'})`
- [ ] Performance: parse <1ms per log, query <100ms for 1000 logs

## Not In Scope
- Custom DSLs for field extraction (regex patterns only)
- Automatic log level inference
- Sensitive data redaction (application responsibility)

## Data Structures

### Normalized Log Entry
```json
{
  "timestamp": "2026-01-31T10:15:23.456Z",
  "level": "ERROR",
  "message": "Payment failed",
  "service": "payment-service",
  "hostname": "api-prod-01",
  "request_id": "req-123",
  "session_id": "session-abc",
  "fields": {
    "user_id": 123,
    "error_code": "INSUFFICIENT_FUNDS",
    "duration_ms": 245
  },
  "source": {
    "format": "json",
    "original": "...",  // Original log string
    "parser": "json-parser-v1"
  }
}
```

### Parser Configuration
```yaml
services:
  payment-service:
    format: json
    timestamp_field: time
    level_field: level
    message_field: msg
    context_fields: [user_id, error_code, duration_ms]

  api-server:
    format: text
    pattern: '(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z) \[(\w+)\] (.*)'
    groups: [timestamp, level, message]
    context_extraction:
      user_id: '(?:user_id[=:]\s*(\d+))'
      error_code: '(?:error[=:]\s*([A-Z_]+))'
```

## Examples

### Example 1: Querying Across Languages
#### Go service logs:
```json
{"timestamp":"2026-01-31T10:15:23.456Z","level":"ERROR","message":"Payment failed","user_id":123}
```

#### Node.js service logs:
```
[10:15:23] ERROR: Payment failed - user_id: 123
```

#### Kaboom query:
```javascript
observe({
  what: 'normalized-logs',
  level: 'ERROR',
  service: '*',
  field: 'user_id:123'
})
```

**Result:** Both services' logs returned in unified format.

### Example 2: Anomaly Detection
Developer queries: "How many errors in the last hour, by service?"
```javascript
observe({
  what: 'normalized-logs',
  level: 'ERROR',
  since: '2026-01-31T09:15:23Z'
})
// Returns:
// payment-service: 45 errors
// api-server: 12 errors
// inventory-service: 3 errors
```

Payment service has unusual error rate → trigger investigation.

## Parser Support
- **JSON:** Configurable field mapping
- **Text:** Regex-based pattern matching
- **Syslog:** RFC 3164/5424
- **CloudWatch:** Timestamp + message extraction
- **Custom:** Regex pattern per service

## MCP Changes
```javascript
observe({
  what: 'normalized-logs',
  level: 'ERROR',              // Normalized level
  service: 'payment-service',  // Source service
  field: 'user_id:123',        // Extracted field query
  since: timestamp,
  limit: 100
})
```
