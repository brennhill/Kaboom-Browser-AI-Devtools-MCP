---
status: active
scope: feature/navigation
ai-priority: high
tags: [features, navigation, index, lookup]
relates-to: [FEATURE-INDEX.md, README.md]
canonical: true
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-04-03
---

# Feature Navigation Index

**For LLM Agents:** Quick lookup to find feature documentation folders and their key files.

**Last updated:** 2026-04-03

---

## Quick Lookup: Find a Feature's Docs

### Format

Each entry shows:
- **Feature name** — Unique identifier for the feature
- **Folder path** — Where docs live (`docs/features/feature/<name>/`)
- **Status** — shipped | proposed
- **Key files** — What's available in this folder

### Shipped Features (Production)

Features with active code implementations referencing their feature docs.

| Feature | Folder | Files | Purpose |
|---------|--------|-------|---------|
| auto-fix | `feature/auto-fix/` | index.md | Phase 1 tracked-site audit workflow, `/kaboom/audit` assets, and shared popup/hover Audit bridge |
| ai-capture-control | `feature/ai-capture-control/` | product-spec.md, qa-plan.md, tech-spec.md | AI-driven capture control for selective telemetry |
| ai-web-pilot | `feature/ai-web-pilot/` | product-spec.md, qa-plan.md, tech-spec.md, test-plan.md | AI Web Pilot browser automation framework |
| analyze-tool | `feature/analyze-tool/` | product-spec.md, qa-plan.md, tech-spec.md, uat-guide.md, MIGRATION.md | Analyze tool for DOM, accessibility, security, and performance |
| verification-contracts | `feature/verification-contracts/` | index.md | Versioned QA assertions, required evidence, and explicit verdicts |
| annotated-screenshots | `feature/annotated-screenshots/` | product-spec.md, qa-plan.md, tech-spec.md | Draw-mode annotation overlay for visual feedback |
| app-telemetry | `feature/app-telemetry/` | index.md | Privacy-bounded app event, session, and usage-summary reporting |
| api-key-auth | `feature/api-key-auth/` | product-spec.md, qa-plan.md, tech-spec.md | API key authentication for daemon access |
| api-schema | `feature/api-schema/` | product-spec.md, qa-plan.md, tech-spec.md | Schema validation for API requests and responses |
| backend-log-streaming | `feature/backend-log-streaming/` | product-spec.md, qa-plan.md, tech-spec.md | Real-time backend log streaming to browser context |
| binary-format-detection | `feature/binary-format-detection/` | product-spec.md, qa-plan.md, tech-spec.md | Detect and handle binary response bodies |
| bridge-restart | `feature/bridge-restart/` | product-spec.md, tech-spec.md, test-plan.md | Force-restart daemon when unresponsive via `configure(what="restart")` |
| browser-extension-enhancement | `feature/browser-extension-enhancement/` | product-spec.md, qa-plan.md, tech-spec.md | MV3 extension enhancements and lifecycle management |
| ci-infrastructure | `feature/ci-infrastructure/` | product-spec.md, qa-plan.md, tech-spec.md, business-pitch.md | CI/CD pipeline infrastructure and automation |
| code-navigation-modification | `feature/code-navigation-modification/` | product-spec.md, qa-plan.md, tech-spec.md | Code navigation and modification helpers |
| cold-start-queuing | `feature/cold-start-queuing/` | index.md | Queue MCP requests during daemon cold start |
| config-profiles | `feature/config-profiles/` | product-spec.md, qa-plan.md, tech-spec.md | Named configuration profiles for different environments |
| deployment-watchdog | `feature/deployment-watchdog/` | product-spec.md, qa-plan.md, tech-spec.md | Monitor deployments for regressions |
| enhanced-cli-config | `feature/enhanced-cli-config/` | product-spec.md, qa-plan.md, tech-spec.md, implementation-plan.md | Enhanced CLI configuration management |
| enhanced-wcag-audit | `feature/enhanced-wcag-audit/` | product-spec.md, qa-plan.md, tech-spec.md | Enhanced WCAG accessibility auditing |
| enterprise-audit | `feature/enterprise-audit/` | product-spec.md, qa-plan.md, tech-spec.md | Enterprise-grade audit logging and compliance |
| error-clustering | `feature/error-clustering/` | product-spec.md, qa-plan.md, tech-spec.md | Cluster similar errors for noise reduction |
| file-upload | `feature/file-upload/` | index.md | File upload automation via interact tool |
| har-export | `feature/har-export/` | product-spec.md, qa-plan.md, tech-spec.md | HAR format export for network traffic |
| historical-snapshots | `feature/historical-snapshots/` | product-spec.md, qa-plan.md, tech-spec.md | Point-in-time state snapshots |
| interact-explore | `feature/interact-explore/` | product-spec.md, qa-plan.md, tech-spec.md | AI exploration suite for browser interaction |
| issue-reporting | `feature/issue-reporting/` | product-spec.md, qa-plan.md, tech-spec.md | Opt-in issue reporting via configure(what="report_issue") |
| link-health | `feature/link-health/` | product-spec.md, qa-plan.md, tech-spec.md, test-plan.md | Link health checking and validation |
| mcp-persistent-server | `feature/mcp-persistent-server/` | index.md | Persistent daemon mode for long-lived MCP server |
| noise-filtering | `feature/noise-filtering/` | product-spec.md, qa-plan.md, tech-spec.md, test-plan.md | Console and network noise suppression rules |
| normalized-event-schema | `feature/normalized-event-schema/` | product-spec.md, qa-plan.md, tech-spec.md | Normalized schema for browser events |
| normalized-log-schema | `feature/normalized-log-schema/` | product-spec.md, qa-plan.md, tech-spec.md | Normalized schema for log entries |
| observe | `feature/observe/` | product-spec.md, qa-plan.md, tech-spec.md | Core observe tool for browser telemetry retrieval |
| pagination | `feature/pagination/` | product-spec.md, qa-plan.md, tech-spec.md | Cursor-based pagination for large result sets |
| performance-audit | `feature/performance-audit/` | product-spec.md, qa-plan.md, tech-spec.md | Performance auditing and Web Vitals analysis |
| quality-gates | `feature/quality-gates/` | index.md | Automated code quality gates via configure(what="setup_quality_gates") |
| persistent-memory | `feature/persistent-memory/` | product-spec.md, qa-plan.md, tech-spec.md | Persistent key-value store across sessions |
| playback-engine | `feature/playback-engine/` | product-spec.md | Recording playback and replay engine |
| project-isolation | `feature/project-isolation/` | product-spec.md, qa-plan.md, tech-spec.md | Per-project data isolation |
| push-alerts | `feature/push-alerts/` | product-spec.md, qa-plan.md, tech-spec.md | Push-based streaming alerts for errors and anomalies |
| query-dom | `feature/query-dom/` | product-spec.md, qa-plan.md, tech-spec.md | DOM querying and element inspection |
| query-service | `feature/query-service/` | product-spec.md, qa-plan.md, tech-spec.md | Central query routing and execution service |
| rate-limiting | `feature/rate-limiting/` | product-spec.md, qa-plan.md, tech-spec.md | Request rate limiting and throttling |
| redaction-patterns | `feature/redaction-patterns/` | product-spec.md, qa-plan.md, tech-spec.md | PII and sensitive data redaction patterns |
| reproduction-scripts | `feature/reproduction-scripts/` | product-spec.md, qa-plan.md, tech-spec.md | Automated bug reproduction script generation |
| ring-buffer | `feature/ring-buffer/` | product-spec.md, qa-plan.md, tech-spec.md | Ring buffer for bounded memory telemetry storage |
| sarif-export | `feature/sarif-export/` | product-spec.md, qa-plan.md, tech-spec.md | SARIF format export for accessibility reports |
| security-hardening | `feature/security-hardening/` | product-spec.md, qa-plan.md, tech-spec.md | Security hardening for daemon and extension |
| self-testing | `feature/self-testing/` | product-spec.md, qa-plan.md, tech-spec.md | Self-testing and health check infrastructure |
| tab-recording | `feature/tab-recording/` | product-spec.md, qa-plan.md, tech-spec.md | Tab video/audio recording capture |
| tab-tracking-ux | `feature/tab-tracking-ux/` | product-spec.md, qa-plan.md, tech-spec.md | Tab tracking UX and multi-tab management |
| terminal | `feature/terminal/` | index.md | In-browser terminal widget with dedicated server on port+1 |
| test-generation | `feature/test-generation/` | product-spec.md, qa-plan.md, tech-spec.md, uat-guide.md | E2E test generation from browser sessions |
| transient-capture | `feature/transient-capture/` | index.md | Capture transient UI elements (tooltips, toasts) |
| ttl-retention | `feature/ttl-retention/` | product-spec.md, qa-plan.md, tech-spec.md | TTL-based data retention and eviction |

### Proposed Features

Features that are documented but not yet implemented in code.

| Feature | Folder | Files | Purpose |
|---------|--------|-------|---------|
| a11y-tree-snapshots | `feature/a11y-tree-snapshots/` | product-spec.md, qa-plan.md, tech-spec.md | Accessibility tree snapshot capture |
| advanced-filtering | `feature/advanced-filtering/` | feature-proposal.md, qa-plan.md, tech-spec.md | Advanced filtering for observe queries |
| agentic-cicd | `feature/agentic-cicd/` | product-spec.md, qa-plan.md, tech-spec.md | Agentic CI/CD pipeline integration |
| agentic-e2e-repair | `feature/agentic-e2e-repair/` | product-spec.md, qa-plan.md, tech-spec.md | Agentic E2E test repair and self-healing |
| auto-paste-screenshots | `feature/auto-paste-screenshots/` | product-spec.md, qa-plan.md | Auto-paste screenshots into conversations |
| backend-control | `feature/backend-control/` | product-spec.md, qa-plan.md, tech-spec.md | Backend service control and orchestration |
| backend-log-ingestion | `feature/backend-log-ingestion/` | product-spec.md, qa-plan.md, tech-spec.md | Backend log ingestion from multiple sources |
| batch-sequences | `feature/batch-sequences/` | design-spec.md | Saved and replayable action sequences |
| behavioral-baselines | `feature/behavioral-baselines/` | product-spec.md, qa-plan.md, tech-spec.md | Behavioral baseline comparison and regression detection |
| best-practices-audit | `feature/best-practices-audit/` | product-spec.md, qa-plan.md, tech-spec.md | Web best practices auditing |
| browser-push | `feature/browser-push/` | product-spec.md, qa-plan.md, tech-spec.md | Browser push notification testing |
| budget-thresholds | `feature/budget-thresholds/` | product-spec.md, qa-plan.md, tech-spec.md | Performance and resource budget thresholds |
| buffer-clearing | `feature/buffer-clearing/` | product-spec.md, tech-spec.md | Selective buffer clearing operations |
| causal-diffing | `feature/causal-diffing/` | product-spec.md, qa-plan.md, tech-spec.md | Causal diff analysis between states |
| causality-analysis | `feature/causality-analysis/` | product-spec.md, qa-plan.md, tech-spec.md | Root cause analysis via causal chains |
| compressed-diffs | `feature/compressed-diffs/` | product-spec.md, qa-plan.md, tech-spec.md | Compressed diff format for large payloads |
| context-streaming | `feature/context-streaming/` | product-spec.md, qa-plan.md, tech-spec.md | Streaming context delivery to AI agents |
| cpu-network-emulation | `feature/cpu-network-emulation/` | product-spec.md, qa-plan.md, tech-spec.md | CPU and network throttling emulation |
| csp-safe-execution | `feature/csp-safe-execution/` | index.md | CSP-safe script execution strategies |
| cursor-pagination | `feature/cursor-pagination/` | feature-proposal.md, qa-plan.md, tech-spec.md | Cursor pagination design proposal |
| custom-event-api | `feature/custom-event-api/` | product-spec.md, qa-plan.md, tech-spec.md | Custom event emission and capture API |
| design-audit-archival | `feature/design-audit-archival/` | product-spec.md, qa-plan.md, tech-spec.md | Design audit archival and versioning |
| dialog-handling | `feature/dialog-handling/` | product-spec.md, qa-plan.md, tech-spec.md | Browser dialog (alert, confirm, prompt) handling |
| dom-diffing | `feature/dom-diffing/` | tech-spec.md | DOM diff computation between snapshots |
| dom-fingerprinting | `feature/dom-fingerprinting/` | product-spec.md, qa-plan.md, tech-spec.md | DOM element fingerprinting for stable selectors |
| drag-drop-automation | `feature/drag-drop-automation/` | product-spec.md, qa-plan.md, tech-spec.md | Drag and drop interaction automation |
| dynamic-exposure | `feature/dynamic-exposure/` | product-spec.md, qa-plan.md, tech-spec.md | Dynamic tool exposure based on context |
| e2e-testing-integration | `feature/e2e-testing-integration/` | product-spec.md, qa-plan.md | E2E testing framework integration |
| environment-manipulation | `feature/environment-manipulation/` | product-spec.md, qa-plan.md, tech-spec.md | Browser environment manipulation (storage, cookies) |
| error-bundling | `feature/error-bundling/` | tech-spec.md | Error bundling with surrounding context |
| flow-recording | `feature/flow-recording/` | product-spec.md, qa-plan.md, tech-spec.md | User flow recording and replay |
| form-filling | `feature/form-filling/` | product-spec.md, qa-plan.md, tech-spec.md | Automated form filling and submission |
| kaboom-ci | `feature/kaboom-ci/` | product-spec.md, qa-plan.md, tech-spec.md | Kaboom CI runner for headless testing |
| git-event-tracking | `feature/git-event-tracking/` | product-spec.md, qa-plan.md, tech-spec.md | Git event tracking and correlation |
| idl-migration | `feature/idl-migration/` | design-spec.md | IDL-based schema migration |
| in-browser-agent-panel | `feature/in-browser-agent-panel/` | product-spec.md, qa-plan.md, tech-spec.md | In-browser agent control panel UI |
| interception-deferral | `feature/interception-deferral/` | product-spec.md, qa-plan.md, tech-spec.md | Request interception and deferral |
| local-web-scraping | `feature/local-web-scraping/` | product-spec.md, qa-plan.md, tech-spec.md | Local web scraping capabilities |
| memory-enforcement | `feature/memory-enforcement/` | product-spec.md, qa-plan.md, tech-spec.md | Memory usage enforcement and limits |
| multiline-rich-editor | `feature/multiline-rich-editor/` | product-spec.md, qa-plan.md, tech-spec.md | Rich editor multiline text insertion |
| page-structure-detection | `feature/page-structure-detection/` | design-spec.md | Page structure and layout detection |
| perf-experimentation | `feature/perf-experimentation/` | product-spec.md, tech-spec.md | Performance experimentation framework |
| performance-budget | `feature/performance-budget/` | product-spec.md, qa-plan.md, tech-spec.md | Performance budget definition and enforcement |
| pr-preview-exploration | `feature/pr-preview-exploration/` | product-spec.md, qa-plan.md, tech-spec.md | PR preview environment exploration |
| push-regression | `feature/push-regression/` | product-spec.md, qa-plan.md, tech-spec.md | Push-based regression detection |
| read-only-mode | `feature/read-only-mode/` | product-spec.md, qa-plan.md, tech-spec.md | Read-only mode for safe observation |
| reproduction-enhancements | `feature/reproduction-enhancements/` | product-spec.md, qa-plan.md, tech-spec.md | Enhanced reproduction script capabilities |
| request-session-correlation | `feature/request-session-correlation/` | product-spec.md, qa-plan.md, tech-spec.md | Request-session correlation tracking |
| self-healing-tests | `feature/self-healing-tests/` | product-spec.md, qa-plan.md, tech-spec.md | Self-healing test selector repair |
| seo-audit | `feature/seo-audit/` | product-spec.md, qa-plan.md, tech-spec.md | SEO audit and analysis |
| spa-route-measurement | `feature/spa-route-measurement/` | product-spec.md, qa-plan.md, tech-spec.md | SPA route transition measurement |
| state-time-travel | `feature/state-time-travel/` | product-spec.md, qa-plan.md, tech-spec.md | State save/restore time travel debugging |
| subtitle | `feature/subtitle/` | product-spec.md | Narration subtitle overlay for actions |
| temporal-graph | `feature/temporal-graph/` | product-spec.md, qa-plan.md, tech-spec.md | Temporal event graph visualization |
| test-execution-capture | `feature/test-execution-capture/` | product-spec.md, qa-plan.md, tech-spec.md | Test execution capture and reporting |
| timeline-search | `feature/timeline-search/` | product-spec.md, qa-plan.md, tech-spec.md | Timeline search across captured events |
| tool-allowlisting | `feature/tool-allowlisting/` | product-spec.md, qa-plan.md, tech-spec.md | Tool allowlisting for restricted environments |
| visual-semantic-bridge | `feature/visual-semantic-bridge/` | product-spec.md, qa-plan.md, tech-spec.md | Visual-semantic bridge for screenshot analysis |
| web-vitals | `feature/web-vitals/` | product-spec.md, qa-plan.md, tech-spec.md | Core Web Vitals measurement and reporting |
| workflow-integration | `feature/workflow-integration/` | product-spec.md, qa-plan.md, tech-spec.md | Workflow tool integration (GitHub, Jira) |

---

## Summary

| Status | Count |
|--------|-------|
| Shipped | 51 |
| Proposed | 60 |
| **Total** | **111** |

---

## Navigation by Use Case

### I need to understand what a feature does
> Read the feature folder's **product-spec.md**

### I need to implement a feature
> Read in order: product-spec.md -> tech-spec.md -> qa-plan.md

### I need to test a feature
> Read **qa-plan.md** for test scenarios, then check if a review file exists for known issues

### I need to find the codebase implementation
> Read tech-spec.md and look for "**Code References**" section or `filename:line_number` format

### I need to understand design decisions
> Read the feature review file or check `docs/adrs/ADR-<feature-name>.md`

---

## Folder Structure Template

Each feature folder should contain:

```
feature/<feature-name>/
├── product-spec.md         # Requirements & user stories
├── tech-spec.md            # Implementation details
├── qa-plan.md              # Test scenarios & acceptance criteria
├── <feature>-review.md     # Optional: Principal engineer review
└── [Optional files]
    ├── ADR-<feature>.md    # Links here (actually in /docs/adrs/)
    ├── implementation-plan.md
    ├── MIGRATION.md
    └── [Other docs]
```

---

## How to Add a New Feature

1. Create folder: `docs/features/feature/<feature-name>/`
2. Add product-spec.md (copy template from `docs/templates/FEATURE-TEMPLATE.md`)
3. Add tech-spec.md
4. Add qa-plan.md
5. Create ADR at `docs/adrs/ADR-<feature-name>.md`
6. Update **FEATURE-INDEX.md** with new entry
7. Verify YAML frontmatter on all files
8. Get spec review before implementation

---

## Related Documents

- **FEATURE-INDEX.md** — Status table of all features
- **README.md** — Comprehensive features guide for LLMs
- `docs/core/RELEASE.md` — What version introduced each feature
- `.claude/refs/architecture.md` — System-wide design patterns
