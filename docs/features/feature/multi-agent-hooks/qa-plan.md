---
doc_type: qa-plan
feature_id: feature-multi-agent-hooks
status: proposed
scope: feature/multi-agent-hooks/qa
ai-priority: medium
tags: [testing, qa, hooks, multi-agent]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
last_reviewed: 2026-06-29
---

# QA Plan: Multi-Agent Hook Protocol

> Verifies that the single `kaboom-hooks` binary auto-detects which artificial intelligence (AI) coding agent is calling it and adapts its input parsing, output format, and session-identifier derivation accordingly. All hooks (quality-gate, blast-radius, session-track, compress-output, decision-guard) work across Claude Code and Gemini CLI without separate binaries. Covers data-leak analysis, agent clarity, simplicity, code-level tests, and step-by-step user acceptance testing (UAT).

---

## 1. Data Leak Analysis

**Goal:** Confirm agent detection reads only environment markers, and that adapting output format introduces no new data exposure.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Detection inputs | Verify `DetectAgent` reads only `GEMINI_SESSION_ID` and `CODEX_SESSION_ID` environment variables | medium |
| DL-2 | Session-identifier source | Verify agent-provided identifiers are truncated to a fixed length and the fallback hashes only process identifier and working directory | medium |
| DL-3 | Output payload parity | Verify the Gemini and Claude output formats carry the same `additionalContext` string and nothing extra | low |
| DL-4 | No network transmission | Verify protocol adaptation makes no network calls | critical |
| DL-5 | Config file writes | Verify quality-gate setup writes hook config only into the project's `.claude/` or `.gemini/` directory | medium |

### Negative Tests (must NOT leak)
- [ ] Detection never reads or transmits arbitrary environment data
- [ ] Agent-provided session identifiers are length-bounded
- [ ] No protocol path performs network I/O
- [ ] Config writes are confined to the project's agent directories

---

## 2. Agent Clarity Assessment

**Goal:** Confirm each agent receives its expected output shape so the injected context is consumed, not dropped.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Claude format | Claude Code receives a flat `additionalContext` field | [ ] |
| CL-2 | Gemini format | Gemini CLI receives `additionalContext` nested under `hookSpecificOutput` | [ ] |
| CL-3 | Empty context suppressed | When there is nothing to inject, no JSON is written at all | [ ] |
| CL-4 | Tool-name acceptance | Both Claude (`Edit`, `Write`, `Bash`) and Gemini (`replace_in_file`, `write_file`, `run_shell_command`) names are recognized | [ ] |
| CL-5 | Default to Claude | With no agent environment marker, output defaults to the Claude format | [ ] |

### Common Agent Misinterpretation Risks
- [ ] Gemini drops the context because it was written in the flat Claude shape
- [ ] Claude ignores context that was nested under `hookSpecificOutput`
- [ ] A hook emits empty JSON instead of nothing, producing protocol noise

---

## 3. Simplicity Assessment

**Goal:** Confirm one binary serves all agents with no per-agent configuration of the hook logic.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run hooks under Claude Code | 0 steps: default format | No — automatic |
| Run hooks under Gemini CLI | 0 steps: detected via env var | No — automatic |
| Install hooks for a project | 1 step: `configure(setup_quality_gates)` writes the right config | No — single action |

### Default Behavior Verification
- [ ] The same binary handles all agents; no agent-specific build exists
- [ ] Hook logic is agent-agnostic; only the input/output protocol layer adapts
- [ ] Absent any marker, behavior matches Claude Code exactly (backward compatible)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Detect Gemini | `GEMINI_SESSION_ID` set | `AgentGemini` | must |
| UT-2 | Detect Codex | `CODEX_SESSION_ID` set | `AgentCodex` | must |
| UT-3 | Detect Claude default | No markers | `AgentClaude` | must |
| UT-4 | Gemini output shape | Non-empty context, Gemini env | Nested `hookSpecificOutput.additionalContext` | must |
| UT-5 | Claude output shape | Non-empty context, no env | Flat `additionalContext` | must |
| UT-6 | Empty context | Empty string | Nothing written, returns nil | must |
| UT-7 | Session identifier from Gemini | `GEMINI_SESSION_ID` | Truncated agent identifier | must |
| UT-8 | Session identifier fallback | No markers | Hash of process identifier + cwd | must |
| UT-9 | Edit-tool name mapping | Both naming conventions | All recognized as edits | must |
| UT-10 | Input parsing | `tool_input` JSON | Common fields extracted; malformed input yields zero values | must |
| UT-11 | Response text extraction | String or object `tool_response` | Output/stdout/content extracted | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end Gemini run | Binary with `GEMINI_SESSION_ID` set | Gemini-shaped output emitted | must |
| IT-2 | End-to-end Claude run | Binary with no markers | Claude-shaped output emitted | must |
| IT-3 | Quality-gate config write | `configure(setup_quality_gates)` detects `.gemini/` | Writes Gemini-format hook config (AfterTool, regex matchers, millisecond timeouts) | must |
| IT-4 | Shared session across hooks | session-track + blast-radius, same agent | Both resolve the same session identifier | must |
| IT-5 | All five hooks under one agent | quality-gate, blast-radius, session-track, compress-output, decision-guard | Each produces its agent-correct output | should |

### 4.3 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Both env markers set | Gemini and Codex both present | Gemini wins (checked first) | should |
| EC-2 | Over-long agent identifier | Identifier longer than the limit | Truncated to the fixed length | must |
| EC-3 | Short agent identifier | Identifier shorter than the limit | Used as-is, not padded | should |
| EC-4 | Unknown tool name | A tool not in the mapping | Classified as "other"; hooks no-op appropriately | should |
| EC-5 | Malformed `tool_input` | Invalid JSON | Falls back to zero-value fields; hook does nothing | must |

---

## 5. UAT Checklist (Human + AI)

> Run the same hook binary under two agents and confirm the output shape adapts.

### Prerequisites
- [ ] `kaboom-hooks` installed
- [ ] Access to both a Claude Code session and a Gemini CLI session (or simulated environment variables)

### Step-by-Step Verification

| # | Step | Human Observes | Expected Result | Pass |
|---|------|----------------|-----------------|------|
| UAT-1 | Run a hook with no agent markers | Output JSON | Flat `additionalContext` (Claude format) | [ ] |
| UAT-2 | Run the same hook with `GEMINI_SESSION_ID` set | Output JSON | Nested under `hookSpecificOutput` (Gemini format) | [ ] |
| UAT-3 | Run a hook that has nothing to inject | Output | Nothing written | [ ] |
| UAT-4 | Configure quality gates in a `.gemini` project | Config file | Gemini-format hooks written (AfterTool, regex matchers) | [ ] |
| UAT-5 | Trigger session-track under Gemini | Output | Recorded and injected in Gemini shape | [ ] |
| UAT-6 | Trigger session-track under Claude | Output | Recorded and injected in Claude shape | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Detection reads env only | Inspect detection inputs | Only the two session-identifier variables read | [ ] |
| DL-UAT-2 | Config confined to project | Inspect written config path | Under `.claude/` or `.gemini/` only | [ ] |
| DL-UAT-3 | No network traffic | Monitor during hook runs | No outbound requests | [ ] |

### Regression Checks
- [ ] Existing Claude Code behavior is unchanged when no markers are present
- [ ] All five hooks still function under Claude Code
- [ ] Session identifiers stay stable within a single agent session

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| Agent Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
