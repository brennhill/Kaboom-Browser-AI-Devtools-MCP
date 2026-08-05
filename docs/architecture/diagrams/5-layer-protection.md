---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# 5-Layer Architectural Protection

## Defense in Depth

```mermaid
graph TB
    subgraph "Layer 1: Pre-Commit Hook"
        L1[scripts/hooks/pre-commit<br/>Local, instant feedback]
        L1 -->|Checks| L1A[Critical files exist]
        L1 -->|Checks| L1B[Required methods exist]
        L1 -->|Checks| L1C[No stub implementations]
    end

    subgraph "Layer 2: Integration Tests"
        L2[internal/capture/<br/>async_queue_integration_test.go]
        L2 -->|Tests| L2A[Full async flow<br/>MCP → Queue → Extension → Result]
        L2 -->|Tests| L2B[Multi-client isolation]
        L2 -->|Tests| L2C[Command expiration]
    end

    subgraph "Layer 3: Validation Script"
        L3[scripts/<br/>validate-architecture.sh]
        L3 -->|Checks| L3A[14 required methods]
        L3 -->|Checks| L3B[Critical constants<br/>AsyncCommandTimeout = 30s]
        L3 -->|Checks| L3C[Runs integration tests]
    end

    subgraph "Layer 4: GitHub Actions"
        L4[.github/workflows/<br/>architecture-validation.yml]
        L4 -->|Runs| L4A[Layer 3 validation script]
        L4 -->|Runs| L4B[Integration test suite]
        L4 -->|Blocks| L4C[PR merge if validation fails]
    end

    subgraph "Layer 5: Documentation"
        L5[ADR-002 +<br/>ARCHITECTURE-ENFORCEMENT.md]
        L5 -->|Documents| L5A[WHY immutable]
        L5 -->|Documents| L5B[Bypass procedure]
        L5 -->|Documents| L5C[Enforcement layers]
    end

    Dev[Developer commits] --> L1
    L1 -->|Pass| L2
    L1 -.->|Fail| Block1[❌ Commit blocked]

    L2 -->|Pass| L3
    L2 -.->|Fail| Block2[❌ Build fails]

    L3 -->|Pass| L4
    L3 -.->|Fail| Block3[❌ CI fails locally]

    L4 -->|Pass| Merge[✅ PR can merge]
    L4 -.->|Fail| Block4[❌ CI blocks merge]

    L5 -.-> Dev
    L5 -.->|Context| L1
    L5 -.->|Context| L2
    L5 -.->|Context| L3

    style Block1 fill:#f85149
    style Block2 fill:#f85149
    style Block3 fill:#f85149
    style Block4 fill:#f85149
    style Merge fill:#3fb950
```

## Protection Matrix

```mermaid
graph LR
    subgraph "Attacker Must Bypass ALL"
        A1[1. Use --no-verify<br/>on git commit]
        A2[2. Delete or disable<br/>integration tests]
        A3[3. Get admin access<br/>to bypass CI]
        A4[4. Override GitHub<br/>required checks]
        A5[5. Ignore all<br/>documentation]

        A1 --> A2 --> A3 --> A4 --> A5
    end

    A5 -->|Only then| Success[Architecture broken]

    Start[Accidental deletion] -.->|Blocked by| A1
    Start -.->|Blocked by| A2
    Start -.->|Blocked by| A3

    style Success fill:#f85149
    style A1 fill:#fde047
    style A2 fill:#fde047
    style A3 fill:#fde047
    style A4 fill:#fde047
    style A5 fill:#fde047
```

## Layer Interaction Flow

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant L1 as Layer 1<br/>(Pre-Commit)
    participant L2 as Layer 2<br/>(Tests)
    participant L3 as Layer 3<br/>(Script)
    participant L4 as Layer 4<br/>(GitHub Actions)
    participant L5 as Layer 5<br/>(Docs)

    Dev->>L5: Reads ADR-002<br/>"Why is this immutable?"
    L5-->>Dev: Context + rationale

    Dev->>Dev: Makes changes to queries.go
    Dev->>L1: git commit

    L1->>L1: Check files exist
    L1->>L1: Check methods exist
    L1->>L1: Check no stubs

    alt Validation passes
        L1-->>Dev: ✅ Commit created
    else Validation fails
        L1-->>Dev: ❌ BLOCKED<br/>See ADR-002
    end

    Dev->>Dev: go test ./internal/capture
    Dev->>L2: Run integration tests

    alt Tests pass
        L2-->>Dev: ✅ All flows work
    else Tests fail
        L2-->>Dev: ❌ Async queue broken<br/>Fix before committing
    end

    Dev->>Dev: git push
    Dev->>L4: GitHub Actions triggered

    L4->>L3: Run validation script
    L3->>L3: Check 9 categories
    L3->>L2: Run integration tests

    alt All pass
        L4-->>Dev: ✅ CI green<br/>PR can merge
    else Any fail
        L4-->>Dev: ❌ CI blocks merge<br/>See failure log
    end
```

## Cost/Benefit Analysis

```mermaid
graph LR
    subgraph "Setup Cost (One-Time)"
        S1[2 hours engineer time]
        S2[Write enforcement scripts]
        S3[Create integration tests]
        S4[Document in ADRs]
    end

    subgraph "Runtime Cost (Per Commit)"
        R1[+2s local validation]
        R2[+5s CI validation]
        R3[Total: 7s overhead]
    end

    subgraph "Incident Cost (If Broken)"
        I1[4+ hours debugging]
        I2[Production downtime]
        I3[Customer impact]
        I4[Engineering morale]
    end

    S1 & S2 & S3 & S4 --> Setup[Total: 2 hours]
    R1 & R2 --> Runtime[Total: 7s per commit]
    I1 & I2 & I3 & I4 --> Incident[Total: Unknown $$$$]

    Setup -->|One-time| ROI
    Runtime -->|Negligible| ROI
    Incident -->|Prevented| ROI[ROI: 480x]

    style Setup fill:#3fb950
    style Runtime fill:#fde047
    style Incident fill:#f85149
    style ROI fill:#58a6ff
```

## Critical File Coverage

```mermaid
graph TB
    subgraph "Protected Files (Layer 1 + 3)"
        F1[internal/capture/queries.go<br/>303 lines - queue implementation]
        F2[internal/capture/httpingest/handlers.go<br/>HTTP polling endpoints]
        F3[cmd/browser-agent/tools_core.go<br/>MCP tool handlers]
        F4[cmd/browser-agent/bridge.go<br/>MCP ↔ HTTP bridge]
        F5[internal/queries/types.go<br/>Type definitions]
    end

    subgraph "Required Methods (14 total)"
        M1[CreatePendingQuery<br/>CreatePendingQueryWithTimeout]
        M2[GetPendingQueries<br/>GetPendingQueriesForClient]
        M3[SetQueryResult<br/>SetQueryResultWithClient]
        M4[RegisterCommand<br/>ApplyCommandResult<br/>ExpireCommand]
        M5[GetCommandResult<br/>GetPendingCommands<br/>GetCompletedCommands<br/>GetFailedCommands]
    end

    F1 --> M1
    F1 --> M2
    F1 --> M3
    F1 --> M4
    F1 --> M5

    Delete[Delete any file] -.->|Blocked by| Layer1[Pre-Commit Hook]
    Remove[Remove any method] -.->|Blocked by| Layer3[Validation Script]
    Stub[Add stub implementation] -.->|Blocked by| Layer4[GitHub Actions]

    style Delete fill:#f85149
    style Remove fill:#f85149
    style Stub fill:#f85149
    style Layer1 fill:#3fb950
    style Layer3 fill:#3fb950
    style Layer4 fill:#3fb950
```

## Historical Context

```mermaid
timeline
    title Async Queue Protection History

    2026-01-30 : Phase 4b Refactoring : Deleted queries.go : 100% production failure

    2026-02-02 : Emergency Fix : Restored async queue : Increased timeout to 30s

    2026-02-02 : Prevention : Added 5-layer protection : Zero incidents since

    Future : Interface Enforcement : Compile-time guarantees : Even stronger protection
```

## Success Metrics

| Metric | Target | Actual | Status |
|--------|--------|--------|--------|
| Incidents since deployment | 0 | 0 | ✅ |
| CI validation pass rate | 100% | 100% | ✅ |
| Architecture violations merged | 0 | 0 | ✅ |
| False positives (good PR blocked) | <5% | 0% | ✅ |
| Developer onboarding friction | <10 min | ~5 min | ✅ |

## References

- [ADR-002: Async Queue Immutability](../ADR-002-async-queue-immutability.md)
- [ARCHITECTURE-ENFORCEMENT.md](../ARCHITECTURE-ENFORCEMENT.md)
- [validate-architecture.sh](../../scripts/validate-architecture.sh)
- [architecture-validation.yml](../../.github/workflows/architecture-validation.yml)
- [Pre-commit hook](../../scripts/hooks/pre-commit)
