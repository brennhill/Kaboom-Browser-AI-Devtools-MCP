---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Gemini Agent Feedback & Suggestions

Date: 2026-02-16
Agent: Gemini 2.0 Flash (CLI)

Based on my experience using Kaboom MCP, here are key architectural suggestions to improve the experience for AI agents and reduce cognitive load.

## 1. Action Bundling (Wait-Until Logic)
Currently, a typical navigation and inspection flow requires multiple turns:
1. `interact(what="navigate", ...)`
2. `observe(what="command_result", ...)` (multiple times)
3. `interact(what="list_interactive")`

**Suggestion:** Add a `wait_for` parameter to `interact` actions.
**Example:** `interact(what="navigate", url="...", wait_for="#login-button")`
**Benefit:** Collapses multi-turn handshakes into atomic operations, reducing latency and error rates.

## 2. Semantic DOM Snapshots
`list_interactive` returns a large number of elements which can be token-intensive.

**Suggestion:** Provide a `get_semantic_summary` tool that returns a curated tree of:
- Primary Navigation.
- Page Headings (H1).
- Visible inputs and primary CTA buttons.
- Global UI State (e.g., "Modal Open", "Error Banner Visible").

**Benefit:** Significant reduction in token usage and faster "Think" cycles for the agent.

## 3. Passive Telemetry in Tool Responses
Agents currently only find out about background errors (e.g., a new 500 error or TypeError) if they proactively poll `observe`.

**Suggestion:** Include a `telemetry_summary` field in the metadata of *every* tool response.
**Example:** 
```json
{
  "status": "success",
  "metadata": {
    "new_errors_since_last_call": 1,
    "last_error": "TypeError: Cannot read property 'id' of null"
  }
}
```
**Benefit:** Provides a "nervous system" for the agent, allowing immediate reaction to regressions.

## 4. Visual Grounding (Coordinates)
For agents with vision capabilities, correlating text elements with visual layouts is critical.

**Suggestion:** Include bounding box coordinates `(x, y, w, h)` for elements in `list_interactive` and `get_semantic_summary`.
**Benefit:** Enables agents to perform precise visual-spatial reasoning and handle non-standard interactive elements.

## Phase 2: Advanced Agent Autonomy

### 5. DOM Mutation Deltas (Change Feeds)
**Suggestion:** A `watch_dom(selector)` tool that notifies the agent only when a specific part of the UI updates.
**Benefit:** Reduces polling waste and allows agents to "idle" during long-running background processes.

### 6. Network Mocking & Interception
**Suggestion:** An `interact(what="mock_response")` tool to override API responses locally.
**Benefit:** Enables parallel full-stack debugging by allowing the agent to verify frontend fixes against simulated backend states.

### 7. Self-Healing Tool Calls
**Suggestion:** Automatic fallback logic for selectors (e.g., if a CSS selector fails, try a text-based match).
**Benefit:** Increases agent resilience against dynamic UI changes (React re-renders, etc.).

### 8. Interactive Macro Recording (Replay)
**Suggestion:** A `save_sequence` tool to record and replay common navigation paths.
**Benefit:** Fast state recovery and reliable reproduction of complex multi-step bugs.

### 9. LLM-Native Log Aggregation
**Suggestion:** `observe(what="summarized_logs")` to group repetitive noise (like heartbeats) automatically.
**Benefit:** Drastically reduces token consumption and highlights the actual "signal" in noisy logs.

## Phase 3: Reliability & Debugging

### 10. Zombie Connection State
**Observation:** After a period of activity, the tool interface reported "Not connected" for all calls (`configure`, `observe`), despite the server process (`kaboom`) running and listening on port 7890.
**Impact:** Total blocker for long-running agent sessions.
**Suggestion:** Implement a "Health Probe" that can force-reset the transport layer or provide a dedicated `reconnect()` tool action.

## Phase 4: Final Power User Vetting

### 11. Network Body Filtering
**Suggestion:** Add optional `query` or `filter` parameters to `observe(what="network_bodies")`.
**Benefit:** Prevents token blowout by allowing agents to request only specific JSON keys (e.g., just the `user` object) from large GraphQL/REST responses.

### 12. WebSocket Message Inspection
**Suggestion:** Add `observe(what="websocket_messages", connection_id="...")` to see the actual message history.
**Benefit:** Enables debugging of real-time data flows (chat, live notifications) which are currently invisible to the agent beyond connection status.

### 13. Sidecar Headless Chrome Support
**Suggestion:** Support a background "Sidecar" browser instance.
**Benefit:** Allows agents to run heavy audits or background tasks without hijacking the user's active browser tab or moving their focus.

### 14. Asynchronous Audit Pattern
**Observation:** Heavy audits (like Accessibility on complex pages) currently time out.
**Suggestion:** Move `analyze` tools to the same `queued` pattern used by `interact`.
**Benefit:** Prevents blocking the agent while long-running checks complete and allows for progress polling.

