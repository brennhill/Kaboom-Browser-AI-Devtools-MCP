---
status: proposed
scope: feature/causality-analysis
ai-priority: medium
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-causality-analysis
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Causality Analysis — QA Plan

## Test Scenarios

### Scenario 1: Network Failure → Test Failure
#### Setup:
- Test that waits for network response before checking DOM

#### Steps:
1. XHR request fails at 10:15:20
2. Test waits for DOM element that never arrives
3. Assertion fails at 10:15:30
4. Call `analyze({what: 'causality', event_id: 'assertion-failed-123'})`

#### Expected Result:
- Network request identified as likely cause
- Confidence >0.85
- Reason: "404 response precedes assertion failure by 10s"

#### Acceptance Criteria:
- [ ] Network request identified as cause
- [ ] Confidence >0.80
- [ ] Temporal proximity detected (10s window)

---

### Scenario 2: Feature Flag → Behavior Change
#### Setup:
- Feature flag toggled
- Behavior changes after toggle

#### Steps:
1. Feature flag 'new_checkout' toggled to TRUE at 10:15:10
2. User navigates to checkout at 10:15:20
3. New UI renders instead of old UI
4. Call `analyze({what: 'causality', event_id: 'page-rendered-456'})`

#### Expected Result:
- Feature flag identified as cause
- Confidence >0.75
- Reason: "Feature flag change precedes behavior change"

#### Acceptance Criteria:
- [ ] Feature flag identified
- [ ] Confidence >0.70
- [ ] State change detection works

---

### Scenario 3: Error Propagation Chain
#### Setup:
- Database error cascades through service chain

#### Steps:
1. Database times out (10:15:15)
2. Payment service returns 503 (10:15:17)
3. API server returns 502 (10:15:19)
4. Frontend shows error (10:15:25)
5. Call `analyze({what: 'causality', event_id: 'frontend-error'})`

#### Expected Result:
- Database timeout identified as root cause
- Confidence decreases with distance (0.95 → 0.85 → 0.70)
- Chain visible: DB error → Service error → Frontend error

#### Acceptance Criteria:
- [ ] Root cause identified (database)
- [ ] Chain traced through services
- [ ] Confidence decreases appropriately

---

### Scenario 4: False Positive Prevention
#### Setup:
- Two unrelated events with temporal proximity

#### Steps:
1. Event A: User clicks button (10:15:20)
2. Event B: Unrelated API timeout (10:15:25)
3. Call `analyze({what: 'causality', event_id: 'B'})`

#### Expected Result:
- Event A NOT identified as cause
- Low confidence (<0.40) due to lack of causal relationship
- No spurious link

#### Acceptance Criteria:
- [ ] Unrelated events not linked
- [ ] Confidence threshold prevents false positives
- [ ] Multiple rules required for high confidence

---

### Scenario 5: Multiple Candidate Causes
#### Setup:
- Event with multiple potential causes

#### Steps:
1. Network request fails AND feature flag changes AND environment variable changes
2. Assertion fails
3. All three are potential causes
4. Call `analyze({what: 'causality', ...})`

#### Expected Result:
- All three candidates returned
- Ranked by confidence
- Each has clear reason
- Developer can review and choose

#### Acceptance Criteria:
- [ ] All causes identified
- [ ] Ranked by confidence
- [ ] Reasons provided for each
- [ ] Results limit to top N

---

## Acceptance Criteria (Overall)
- [ ] All 5 scenarios pass
- [ ] Network failures traced to effects
- [ ] State changes detected
- [ ] Error chains propagated
- [ ] False positives minimized
- [ ] Performance <500ms for analysis

## Test Data

### Fixture: Simple Causal Chain
```
[10:15:20] Network error: XHR GET /api/data → timeout
[10:15:21] Custom event: backend:error (timeout)
[10:15:22] Backend log: Error: timeout
[10:15:25] Frontend action: retry button clicked
[10:15:26] Assertion: expect(retry).visible → PASSED (dependency resolved)
```

## Regression Tests
- [ ] Known causal patterns always detected
- [ ] Confidence scores stable
- [ ] Performance doesn't degrade with more rules
- [ ] False positive rate <1%
