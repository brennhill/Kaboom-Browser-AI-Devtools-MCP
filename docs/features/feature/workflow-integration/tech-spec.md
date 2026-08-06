---
status: shipped
scope: feature/workflow-integration/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-workflow-integration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-workflow-integration.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Workflow Integration Review](./index.md).

# Technical Spec: Workflow Integration

## Purpose

The AI-driven development loop today is manual at its boundaries: the agent writes code, the developer reloads the page, the agent checks for regressions. The "write code → observe impact" cycle works, but it doesn't connect to the broader development workflow — PRs, CI, commit messages, and team communication.

Workflow integration bridges Kaboom's real-time observations with the developer's existing tools: Git commits get performance annotations, PRs get automated performance summaries, and the agent's session history becomes part of the project record. The AI's work becomes auditable and transparent without requiring the human to watch it work.

---

## Opportunity & Business Value

**PR-level visibility**: When an AI agent makes performance-affecting changes across 15 commits, the human reviewer sees a single PR summary: "Net impact: +120ms load time, +45KB bundle size, fixed 3 a11y violations." This makes AI-generated PRs reviewable by humans who weren't watching the session.

**Team-scale AI adoption**: In a team of 5 developers each running AI agents, workflow integration gives engineering managers visibility into what the AI is doing to their performance budgets. Without it, AI-generated changes are black boxes until someone notices the production metrics moved.

**CI/CD integration**: Performance budgets that gate merges ("load time must stay under 2s") are meaningless if they're only checked in production. Kaboom can provide local-dev performance gates that catch regressions before code reaches CI, where the feedback latency is measured in minutes, not seconds.

**Interoperability with existing tools**: GitHub PR comments, Slack notifications, and Lighthouse CI are already in most teams' workflows. Kaboom doesn't replace these — it feeds them. A performance delta in a PR comment uses the same format developers already understand from Lighthouse CI.

**Audit trail for AI work**: When an AI agent modifies code, the session timeline (what it observed, what actions it took, what performance impact resulted) becomes a structured log. This is valuable for debugging "why did the AI do that?" questions that arise during code review.

---

## How It Works

### Three Integration Points

The workflow integration has three levels, each building on the previous:

**Level 1: Session Summary** (server-side, always available)
After a session ends (server shutdown or explicit `end_session` call), the server generates a structured summary of everything that happened: performance deltas, errors encountered, a11y issues found, regressions detected and resolved, and resources changed. This summary is stored via persistent memory and available via the `load_session_context` tool in the next session.

**Level 2: PR Annotation** (MCP tool, agent-triggered)
A new MCP tool `generate_pr_summary` produces a markdown-formatted performance summary suitable for inclusion in a PR description or comment. The agent calls this when it's done working and ready to submit changes.

**Level 3: Git Hook Integration** (developer-configured, optional)
A post-commit hook that calls Kaboom's summary endpoint and appends performance annotations to commit messages or generates a `.kaboom/session-report.md` that the developer can reference in PRs.

### Session Summary Generation

When the server shuts down (or the agent explicitly ends the session), it compiles:

1. **Performance delta**: Compare the first snapshot of the session against the last. Report net change in load time, FCP, LCP, CLS, and transfer size.
2. **Error delta**: Errors that appeared during the session (new errors), errors that were resolved (present at start, gone at end), and errors that persisted throughout.
3. **A11y delta**: Accessibility violations found during the session that weren't present at baseline.
4. **Resource changes**: Net change in script count, stylesheet count, total bundle size.
5. **Session metadata**: Duration, number of page reloads observed, number of performance checks run.

### PR Summary Tool

The `generate_pr_summary` MCP tool takes no parameters and returns a markdown document:

```markdown
## Performance Impact

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Load Time | 1.2s | 1.4s | +200ms (+17%) |
| FCP | 0.8s | 0.9s | +100ms (+13%) |
| LCP | 1.0s | 1.1s | +100ms (+10%) |
| CLS | 0.02 | 0.02 | — |
| Bundle Size | 340KB | 385KB | +45KB (+13%) |

### Resource Changes
- **Added**: `chart-library.js` (98KB), `analytics.js` (12KB)
- **Removed**: `legacy-charts.js` (65KB)
- **Net**: +45KB

### Errors
- Fixed: `TypeError: Cannot read property 'map' of undefined` (dashboard.js:142)
- New: None

### Accessibility
- No new violations detected
- Existing: 2 color contrast issues (unchanged)

---
*Generated by Kaboom from 12 performance samples across 8 page loads*
```

The agent can include this in a PR description, append it to a commit message, or post it as a PR comment via `gh pr comment`.

### Git Hook Format

For teams that want automatic annotations, a post-commit hook queries the running Kaboom server:

```bash
# In .git/hooks/post-commit or via lefthook:
curl -s http://localhost:3663/v4/session-summary | jq -r '.one_liner' >> .git/COMMIT_EDITMSG
```

The one-liner format: `[perf: +200ms load, +45KB bundle | errors: -1 fixed | a11y: clean]`

This is informational — it doesn't block the commit.

---

## Data Model

### Session Summary

Stored in persistent memory at `.kaboom/sessions/latest.json`:
- Session start time and end time
- First performance snapshot and last performance snapshot (for delta computation)
- Errors observed: array of `{ message, source, firstSeen, lastSeen, resolved }`
- A11y violations observed: array of `{ ruleId, count, firstSeen }`
- Resource changes: `{ added: [], removed: [], resized: [] }`
- Reload count: How many page loads the extension reported
- Performance check count: How many times `check_performance` was called
- Regression count: How many push regression alerts were generated

### Summary Archive

Previous session summaries are stored in `.kaboom/sessions/archive/` as `<timestamp>.json`. Capped at 20 entries (FIFO). This gives the agent historical context: "Over the last 5 sessions, bundle size grew from 280KB to 385KB."

---

## Server Integration

### Shutdown Hook

The server's shutdown sequence (after flushing persistent memory) generates the session summary and writes it. This happens before the process exits.

### HTTP Endpoint

`GET /v4/session-summary` returns the current session's summary-in-progress. This is useful for git hooks that query during an active session (the summary updates in real-time as observations accumulate).

### MCP Tool: `generate_pr_summary`

Returns the full markdown summary. The agent typically calls this right before creating a PR:

```
Agent: I've finished the dashboard optimization. Let me generate a performance summary for the PR.
[calls generate_pr_summary]
Agent: Here's the PR with the performance impact included.
[calls gh pr create with the summary in the body]
```

---

## Edge Cases

- **No performance snapshots in session**: Summary reports "No performance data collected" and omits the performance delta table.
- **Server crash (no clean shutdown)**: Summary isn't generated. The next session starts fresh. The background flush goroutine (from persistent memory) may have saved partial data.
- **Very short session (< 10 seconds)**: Summary is still generated but marked as "insufficient data" if fewer than 2 snapshots exist.
- **Multiple URLs observed**: Summary includes deltas for up to 5 URLs (sorted by most reloads). URLs with only 1 snapshot are excluded (no delta possible).
- **Agent never calls generate_pr_summary**: The session summary is still saved to disk. It can be retrieved in the next session or read from the file directly.
- **Concurrent sessions (two server instances)**: Each generates its own summary. Archive deduplicates by timestamp.

---

## Performance Constraints

- Session summary generation: under 50ms (aggregating in-memory data)
- PR summary markdown generation: under 10ms (string formatting)
- HTTP endpoint response: under 20ms
- Archive storage: under 500KB total (20 sessions × ~25KB each)

---

## Test Scenarios

1. Session with 2 snapshots → summary shows correct performance delta
2. Session with errors fixed → summary lists resolved errors
3. Session with new a11y violations → summary includes them
4. `generate_pr_summary` returns valid markdown table
5. One-liner format contains performance delta and error status
6. Shutdown generates summary at `.kaboom/sessions/latest.json`
7. Archive capped at 20 entries, oldest removed
8. No snapshots → "No performance data" in summary
9. Single snapshot → marked "insufficient data"
10. Multiple URLs → top 5 by reload count included
11. HTTP endpoint returns in-progress summary during active session
12. Session summary includes reload count and duration
13. Resource changes (added/removed) appear in summary
14. Bundle size delta calculated correctly
15. Previous session summary available via `load_session_context`

---

## File Locations

Server implementation: `cmd/browser-agent/workflow.go` (summary generation, HTTP endpoint, MCP tool).

Persistent storage: `.kaboom/sessions/latest.json` and `.kaboom/sessions/archive/<timestamp>.json`.

Tests: `cmd/browser-agent/workflow_test.go`.
