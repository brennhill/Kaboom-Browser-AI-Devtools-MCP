---
status: proposed
scope: feature/causality-analysis
ai-priority: medium
tags: [v7, analysis]
relates-to: [product-spec.md, ../request-session-correlation/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-causality-analysis
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Causality Analysis — Technical Specification

## Architecture

### Causal Inference Engine
```
Event Timeline
    ↓
Rule-Based Analyzer
    ├─ Temporal Proximity Rules
    ├─ State Change Rules
    ├─ Error Propagation Rules
    ├─ Dependency Rules
    ↓
Confidence Scoring
    ↓
Causal Graph
    ↓
MCP Response
```

### Components
1. **Causal Rules Engine** (`server/analysis/causality.go`)
   - Rule set for inferring causality
   - Configurable confidence thresholds
   - Temporal proximity analysis

2. **Causal Graph Builder** (`server/analysis/causal-graph.go`)
   - Build directed graph of cause → effect
   - Trace error propagation chains
   - Identify branching points

## Implementation Plan

### Phase 1: Rule Definition (Week 1)
1. Define causal rules (temporal, state, error)
2. Implement temporal proximity detection
3. Implement error propagation tracing

### Phase 2: Analysis Engine (Week 2)
1. Implement rule evaluation
2. Score confidence for each candidate
3. Build causal graph

### Phase 3: Query Handler (Week 3)
1. Implement `analyze({what: 'causality', ...})`
2. Return ranked causes
3. Visualize causal graph

## Code References
- **Causality engine:** `/Users/brenn/dev/kaboom/server/analysis/causality.go` (new)
- **Causal graph:** `/Users/brenn/dev/kaboom/server/analysis/causal-graph.go` (new)
- **MCP handler:** `/Users/brenn/dev/kaboom/server/handlers.go` (modified)

## Performance Requirements
- **Causality analysis:** <500ms for 1000-event session
- **Confidence scoring:** <100ms per candidate
- **Graph visualization:** <1s for 50-node graph

## Testing Strategy
- Unit tests for each causal rule
- Integration tests for complex scenarios
- E2E tests with real failing tests

## Risks & Mitigation
1. **False positive causality**
   - Mitigation: Conservative confidence thresholds, require multiple supporting rules
2. **Missed causes**
   - Mitigation: Comprehensive rule set, allow user feedback
3. **Performance on large event sets**
   - Mitigation: Index by event type, limit analysis window
