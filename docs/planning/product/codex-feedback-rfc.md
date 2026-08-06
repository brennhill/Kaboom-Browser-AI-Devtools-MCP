---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Codex Agent Feedback RFC & Issue Checklist

Date: 2026-02-16  
Agent: Codex (GPT-5)

This RFC captures concrete tool improvements based on live end-to-end usage of Kaboom MCP (browse, interact, observe, analyze, generate, recording, async command polling).

## Goals
- Make agent workflows deterministic and easier to automate.
- Reduce tool-call overhead and ambiguity in browser targeting.
- Improve resilience to transient extension/transport failures.
- Standardize API contracts so tool orchestration is reliable.

## Non-Goals
- Redesigning the product UX.
- Replacing existing primitives; this RFC focuses on additive and compatibility-safe improvements.

## Issue Checklist

## 1) Strict Tool Contracts and Stable Result Schemas
Problem:
- Some actions have inconsistent parameter requirements and response shapes, increasing orchestration complexity.

Proposal:
- Enforce explicit, versioned request/response schemas per tool/action.
- Standardize error envelope (`code`, `message`, `retryable`, `details`).
- Add schema conformance tests for every action.

Acceptance checklist:
- [ ] All tools expose validated request schemas.
- [ ] All tool responses include stable top-level fields.
- [ ] Error responses are contract-consistent.
- [ ] CI includes contract tests covering success and error modes.

Suggested issue title:
- `Tool contract hardening: strict params, stable response schema, uniform error envelope`

## 2) Explicit Tab/Session Targeting Defaults
Problem:
- Active-tab fallback can cause commands to execute on unintended pages.

Proposal:
- Require explicit `tab_id` or `session_id` for interaction/analyze actions, with an opt-in `use_active_tab=true` escape hatch.
- Return resolved tab/session metadata in every command result.

Acceptance checklist:
- [ ] All tab-sensitive actions support explicit target IDs.
- [ ] Missing target yields a deterministic validation error.
- [ ] Results include `resolved_tab_id`, `resolved_url`, and session context.

Suggested issue title:
- `Deterministic targeting: require explicit tab/session for browser actions`

## 3) Durable Async Command Lifecycle and Progress Model
Problem:
- Async command results can disappear quickly; polling is fragile.

Proposal:
- Introduce configurable retention TTL for completed commands.
- Add `progress` and `phase` fields for long-running commands.
- Add `observe(command_history)` and resumable polling semantics.

Acceptance checklist:
- [ ] Completed command results retained for configurable TTL.
- [ ] Long-running jobs emit progress states.
- [ ] Command history query supports filtering and pagination.

Suggested issue title:
- `Async reliability: durable command history, progress states, resumable polling`

## 4) Built-In Retry/Reconnect Semantics for Transient Failures
Problem:
- Transient MCP/extension disconnects cause expired or failed commands.

Proposal:
- Classify transient failures (`EOF`, `timeout`, temporary disconnect) and auto-retry with bounded backoff.
- Annotate failures with retry hints and reason codes.

Acceptance checklist:
- [ ] Retry policy for explicitly retryable failure classes.
- [ ] Result metadata includes retry attempts and final failure classification.
- [ ] Extension reconnect path replays safe queued commands.

Suggested issue title:
- `Transport resilience: automatic retries and reconnect-safe command execution`

## 5) Preflight `doctor` Command
Problem:
- Agents spend extra calls diagnosing connection, tracking, and readiness state.

Proposal:
- Add a single `doctor` command reporting extension connectivity, tracked tab, queue health, stale state, and suggested remediation.

Acceptance checklist:
- [ ] `doctor` command returns a pass/fail summary with sub-checks.
- [ ] Each failed check includes actionable remediation text.
- [ ] Agent-readable `ready_for_interaction` boolean is included.

Suggested issue title:
- `Add doctor preflight command for environment readiness and diagnostics`

## 6) Noise Management and Structured Agent Logs
Problem:
- Repetitive logs obscure actionable signal.

Proposal:
- Add default noise suppression profiles plus structured log fields (`category`, `component`, `dedupe_key`, `severity`).
- Add server-side deduplication and suppression metrics.

Acceptance checklist:
- [ ] Built-in log suppression profile enabled by default (configurable).
- [ ] Log events include structured classification fields.
- [ ] Observe APIs expose suppressed-count metrics.

Suggested issue title:
- `Improve observability signal: structured logs, default noise suppression, dedupe metrics`

## 7) Predictable `generate` API Behavior
Problem:
- Generation endpoints have parameter inconsistencies and unclear ignored fields.

Proposal:
- Normalize `generate` subcommand contracts and reject unknown parameters with clear validation errors.
- Ensure generated outputs include rationale for empty tests/scripts.

Acceptance checklist:
- [ ] `generate` subcommands have strict per-mode validation.
- [ ] Unknown params fail fast with actionable errors.
- [ ] Empty output includes machine-readable explanation.

Suggested issue title:
- `Normalize generate API: strict mode-specific params and deterministic output diagnostics`

## 8) High-Level Workflow Primitives
Problem:
- Common workflows require many low-level calls and polling loops.

Proposal:
- Add bundled primitives (examples):
  - `navigate_and_wait_for`
  - `search_and_open`
  - `fill_form_and_submit`
  - `run_a11y_and_export_sarif`

Acceptance checklist:
- [ ] At least 3 workflow primitives implemented with explicit wait semantics.
- [ ] Primitive outputs include the underlying action trace.
- [ ] Timeouts and retries configurable per primitive.

Suggested issue title:
- `Add high-level workflow primitives to reduce multi-step orchestration overhead`

## 9) Richer Action Outcome Metadata
Problem:
- Action responses sometimes lack context needed for robust follow-up decisions.

Proposal:
- Return canonical action metadata on every result:
  - `final_url`
  - `resolved_tab_id`
  - `selector_resolution`
  - `dom_changes`
  - `timing_breakdown_ms`

Acceptance checklist:
- [ ] Shared action-result schema includes context fields above.
- [ ] `selector_resolution` captures fallback/semantic selector decisions.
- [ ] `dom_changes` summaries are always present for DOM mutating actions.

Suggested issue title:
- `Enrich action results with URL/tab context, selector resolution, and DOM-change summaries`

## 10) Machine-Readable Tool Schema and Capability Versioning Endpoint
Problem:
- Agents need to adapt safely as tool signatures evolve.

Proposal:
- Add capability endpoint (for example `describe_capabilities`) returning:
  - tool/action schemas
  - required/optional params
  - semantic version and compatibility notes
  - deprecations and migration hints

Acceptance checklist:
- [ ] Endpoint returns full machine-readable contract data.
- [ ] Includes semantic version and compatibility metadata.
- [ ] CI gate validates docs/schema parity.

Suggested issue title:
- `Expose machine-readable capabilities endpoint with semantic versioning and deprecation metadata`

## Suggested Rollout Sequence
1. Foundation: #1, #2, #10  
2. Reliability: #3, #4, #5  
3. Signal quality: #6, #9  
4. UX acceleration: #7, #8

## Success Metrics
- 30% reduction in average tool calls per completed agent task.
- 50% reduction in transient-failure-caused task interruptions.
- >95% deterministic replay rate for scripted agent workflows.
- <5% invalid-call rate due to schema ambiguity.
