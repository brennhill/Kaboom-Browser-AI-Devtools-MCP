---
status: proposed
scope: feature/normalized-log-schema
ai-priority: medium
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-normalized-log-schema
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Normalized Log Schema — QA Plan

## Test Scenarios

### Scenario 1: JSON Log Parsing
#### Setup:
- Go service logging JSON format

#### Steps:
1. Go service logs: `{"timestamp":"2026-01-31T10:15:23Z","level":"ERROR","message":"Failed","user_id":123}`
2. Log ingested and parsed
3. Query: `observe({what: 'normalized-logs', service: 'payment-service'})`

#### Expected Result:
- Log normalized to canonical schema
- Timestamp parsed as ISO 8601
- Level: "ERROR"
- Message: "Failed"
- Fields: {user_id: 123}

#### Acceptance Criteria:
- [ ] JSON parsed correctly
- [ ] Timestamp in canonical format
- [ ] All fields extracted
- [ ] User_id accessible in query

---

### Scenario 2: Text Log Parsing with Regex
#### Setup:
- Node.js service logging text format: `[10:15:23] ERROR: Failed - user_id: 123`

#### Steps:
1. Configure regex parser for api-server service
2. Node.js logs with text format
3. Parser applies regex pattern
4. Query normalized logs

#### Expected Result:
- Regex pattern extracts: timestamp, level, message
- Context extraction gets user_id
- Normalized to canonical schema

#### Acceptance Criteria:
- [ ] Regex pattern matches
- [ ] Fields extracted correctly
- [ ] Timestamp parsed
- [ ] Context fields extracted

---

### Scenario 3: Cross-Service Query
#### Setup:
- Go service + Node.js service + Python service, different formats

#### Steps:
1. Each service logs in its native format
2. All services send logs to Kaboom
3. Query: `observe({what: 'normalized-logs', level: 'ERROR'})`

#### Expected Result:
- All ERROR logs returned regardless of format
- All in canonical schema
- Service field identifies source

#### Acceptance Criteria:
- [ ] All formats parsed
- [ ] All ERROR logs returned
- [ ] Service field correct
- [ ] No format-specific differences

---

### Scenario 4: Field-Based Querying
#### Setup:
- Multiple logs from multiple services
- Each has user_id field (extracted from different locations per format)

#### Steps:
1. Go logs with user_id in JSON: `{"user_id": 123}`
2. Node logs with user_id in text: `user_id: 123`
3. Query: `observe({what: 'normalized-logs', field: 'user_id:123'})`

#### Expected Result:
- Both logs returned
- user_id field normalized
- No need for separate queries per format

#### Acceptance Criteria:
- [ ] Both sources found
- [ ] Field query works across formats
- [ ] Value coercion correct (string "123" vs. number 123)

---

### Scenario 5: Malformed Log Handling
#### Setup:
- One log is malformed, others are valid

#### Steps:
1. Send valid JSON log
2. Send invalid JSON log (missing closing brace)
3. Send text log
4. Query logs

#### Expected Result:
- Valid logs parsed and returned
- Malformed log not crashed system
- Error logged but gracefully handled
- Valid logs still queryable

#### Acceptance Criteria:
- [ ] No crash on malformed logs
- [ ] Valid logs parsed
- [ ] Error logged for diagnostics
- [ ] Fallback to raw log if parsing fails

---

### Scenario 6: Timestamp Normalization
#### Setup:
- Logs with timestamps in different formats:
  - Go: `2026-01-31T10:15:23Z`
  - Node: `[10:15:23]` (same date)
  - Python: `2026-01-31 10:15:23`

#### Steps:
1. Each service logs with its timestamp format
2. Query all logs
3. Verify timestamps are in same format
4. Verify ordering is correct

#### Expected Result:
- All timestamps converted to ISO 8601
- Order preserved
- Timezone handling correct
- Comparable for range queries

#### Acceptance Criteria:
- [ ] All timestamps in ISO 8601
- [ ] Ordering correct
- [ ] Timezone converted correctly
- [ ] Range queries work

---

### Scenario 7: Custom Field Extraction
#### Setup:
- Service with custom log format, not standard
- Regex pattern defined for extraction

#### Steps:
1. Configure custom regex pattern
2. Service logs with custom format
3. Query using extracted field

#### Expected Result:
- Regex pattern matches
- Custom fields extracted
- Available in query results

#### Acceptance Criteria:
- [ ] Pattern configuration loaded
- [ ] Pattern matches successfully
- [ ] Fields extracted
- [ ] Queryable

---

### Scenario 8: Performance - High Volume
#### Setup:
- 10K logs/sec from multiple services in different formats

#### Steps:
1. Send 10K logs/sec for 10 seconds (100K logs)
2. Measure parse time, memory, CPU
3. Query all logs

#### Expected Result:
- All logs parsed (<1ms each)
- Memory under control
- CPU <80% during ingestion
- Query still responsive (<100ms)

#### Acceptance Criteria:
- [ ] Parse latency <1ms per log
- [ ] No memory growth >100MB
- [ ] CPU <80%
- [ ] Query latency <100ms

---

### Scenario 9: Format Detection Accuracy
#### Setup:
- Mixed logs: some JSON, some text, some syslog

#### Steps:
1. Send 100 JSON logs
2. Send 100 text logs
3. Send 100 syslog logs
4. Verify format detection accuracy

#### Expected Result:
- Correct parser selected for each log
- Detection is automatic (no config needed)
- >95% accuracy

#### Acceptance Criteria:
- [ ] JSON detected correctly
- [ ] Text detected correctly
- [ ] Syslog detected correctly
- [ ] Accuracy >95%

---

## Acceptance Criteria (Overall)
- [ ] All 9 scenarios pass
- [ ] Cross-service log querying works
- [ ] Field extraction accurate
- [ ] Performance <1ms per log
- [ ] Malformed logs handled gracefully
- [ ] Timestamp normalization correct

## Test Data

### Fixture: Go Service JSON Log
```json
{"timestamp":"2026-01-31T10:15:23.456Z","level":"ERROR","message":"Payment failed","service":"payment-service","user_id":123,"error_code":"INSUFFICIENT_FUNDS"}
```

### Fixture: Node.js Service Text Log
```
[2026-01-31 10:15:23] ERROR: Payment failed - user_id: 123, error_code: INSUFFICIENT_FUNDS
```

### Fixture: Python Service Log
```
2026-01-31 10:15:23,456 ERROR - Payment failed - user_id=123 error_code=INSUFFICIENT_FUNDS
```

## Regression Tests
- [ ] All parser types still work
- [ ] Format detection doesn't regress
- [ ] Cross-service queries still accurate
- [ ] Performance doesn't degrade
- [ ] Backward compatibility with raw logs
