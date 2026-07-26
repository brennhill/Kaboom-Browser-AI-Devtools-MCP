# Kaboom Phase 1 Audit Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a first-class tracked-site `Audit` workflow that launches from the popup, reuses Kaboom's existing tracked-tab + terminal bridge, invokes one shared audit command/skill, and returns a polished six-lane local report without any Pro/history/hosting features.

**Architecture:** Keep Phase 1 thin and product-shaped. Reuse the existing tracked-tab state, `qa_scan_requested` runtime bridge, terminal side panel, and Kaboom MCP primitives (`page_issues`, `accessibility`, `performance`, `link_health`, `security_audit`, screenshots, exploration) instead of building a new hosted service or a large new Go audit engine. The Phase 1 renderer is structured markdown produced by the shared audit command/skill in the local terminal/sidepanel; extension and server changes only need to launch and nudge that shared workflow consistently.

**Tech Stack:** TypeScript MV3 extension, Go MCP server, bundled skill/plugin markdown assets, Node test runner, Go test, docs/flow maps.

---

## Working Rules

- Keep `qa_scan_requested` as the internal runtime message in Phase 1. Rename user-facing copy to `Audit`, but do not create a second background contract unless tests prove the old message is too limiting.
- Do not expand `analyze(what:"audit")` into the Phase 1 product surface yet. Treat it as a low-level primitive; the user-facing audit is the new prompt-driven workflow.
- Keep Phase 1 local-only. No recurring audits, watch mode, history, hosted artifacts, share links, team workflow, or white-labeling.
- Use terminal markdown as the first report renderer. Do not build hosted HTML or persistence in this phase.
- After every `src/**` edit, run `make compile-ts`.
- For tasks touching `src/background/**` or `src/popup/**`, run `npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`.
- Update docs and flow maps in the same execution wave as the code that changes behavior.

## File Map

### Shared Audit Trigger

- `src/lib/tabs/request-audit.ts`
  Why: Single helper that opens the terminal side panel and then requests the shared audit workflow.
- `src/background/message-handlers.ts`
  Why: Keeps the existing `qa_scan_requested` bridge, but updates the injected prompt text to the new audit workflow.
- `tests/extension/request-audit.test.js`
  Why: Guards the shared two-step trigger contract.
- `tests/extension/message-handlers.test.js`
  Why: Verifies audit-trigger prompt text, fallback behavior, and background routing.

### Popup And Hover Entrypoints

- `extension/popup.html`
  Why: Add the tracked-state `Audit` CTA.
- `extension/popup.css`
  Why: Style the compact tracked-state audit button without breaking the current popup layout.
- `src/popup/tab-tracking.ts`
  Why: Show/hide and wire the `Audit` button when a site is tracked.
- `src/popup/tab-tracking-api.ts`
  Why: Delegate popup-side audit clicks to the shared helper with the tracked page URL.
- `src/content/ui/tracked-hover-launcher.ts`
  Why: Keep the hover action aligned with the new `Audit` terminology and shared trigger path.
- `tests/extension/popup-audit-button.test.js`
  Why: Covers the tracked popup CTA behavior directly.
- `tests/extension/popup-tab-tracking-sync.test.js`
  Why: Verifies tracked-state sync still reveals the right controls.
- `tests/extension/tracked-hover-launcher.test.js`
  Why: Verifies hover-surface wording and runtime behavior.

### Audit Workflow Assets

- `plugin/kaboom-workflows/commands/audit.md`
  Why: Repo-owned slash-command workflow for the Phase 1 audit.
- `plugin/kaboom-workflows/README.md`
  Why: Documents the new audit command and how it differs from the older focused commands.
- `npm/kaboom-agentic-browser/skills/audit/SKILL.md`
  Why: Bundled cross-agent skill for the new audit workflow.
- `npm/kaboom-agentic-browser/skills/qa/SKILL.md`
  Why: Compatibility shim or redirect so existing `qa` references land on the new audit workflow.
- `npm/kaboom-agentic-browser/skills/skills.json`
  Why: Registers the new bundled skill cleanly.
- `tests/packaging/kaboom-audit-workflow.test.js`
  Why: Guards the skill/command/report contract.
- `tests/packaging/kaboom-skills-branding.test.js`
  Why: Extends the bundled-skill smoke coverage to the new audit asset.

### MCP Nudge Copy

- `cmd/browser-agent/handler_tools_call_postprocess.go`
  Why: The fallback nudge should tell the AI to run the audit workflow, not the older QA wording.
- `cmd/browser-agent/handler_tools_call_postprocess_test.go`
  Why: Adds the missing Go regression test for pending-intent warning copy.

### Docs And Flow Maps

- `docs/architecture/flow-maps/audit-workflow.md`
  Why: New canonical map for the Phase 1 audit workflow.
- `docs/architecture/flow-maps/README.md`
  Why: Register the canonical flow map.
- `docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md`
  Why: Update hover-surface behavior and audit terminology.
- `docs/features/feature/auto-fix/index.md`
  Why: Reframe the legacy QA/find-problems feature as the internal bridge behind the new audit workflow.
- `docs/features/feature/auto-fix/flow-map.md`
  Why: Point the feature-local flow map at the new canonical map.
- `docs/features/feature/tab-tracking-ux/index.md`
  Why: Document the tracked popup `Audit` CTA and updated code/test anchors.
- `docs/features/feature-navigation.md`
  Why: Update the feature summary text so agents discover the new audit workflow correctly.

## Chunk 1: Shared Audit Trigger And User-Facing Entry Surfaces

### Task 1: Add A Shared `requestAudit` Helper

**Files:**
- Create: `src/lib/tabs/request-audit.ts`
- Modify: `src/background/message-handlers.ts`
- Test: `tests/extension/request-audit.test.js`
- Test: `tests/extension/message-handlers.test.js`

- [ ] **Step 1: Write the failing tests**

Create `tests/extension/request-audit.test.js` with explicit assertions for the shared two-step contract:

```js
await requestAudit('https://tracked.example/')

assert.deepStrictEqual(sentTypes, [
  'open_terminal_panel',
  'qa_scan_requested',
])
assert.strictEqual(lastAuditMessage.page_url, 'https://tracked.example/')
```

Extend `tests/extension/message-handlers.test.js` so `qa_scan_requested` verifies the injected text points at the audit workflow rather than the old QA wording:

```js
assert.match(injectedText, /audit/i)
assert.match(injectedText, /kaboom\/audit|\/audit|audit skill/i)
assert.doesNotMatch(injectedText, /Find Problems.*QA skill only/)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
node --test tests/extension/request-audit.test.js tests/extension/message-handlers.test.js -v
```

Expected:

- `request-audit.test.js` fails because the helper does not exist yet.
- `message-handlers.test.js` fails because the injected prompt still references the old QA wording.

- [ ] **Step 3: Write the minimal implementation**

Create `src/lib/tabs/request-audit.ts` with a single helper that always tries to open the terminal first, then sends the existing audit request bridge:

```ts
export async function requestAudit(pageUrl?: string): Promise<void> {
  try {
    await chrome.runtime.sendMessage({ type: 'open_terminal_panel' })
  } catch {
    // Best-effort: still attempt the audit request.
  }
  await chrome.runtime.sendMessage({ type: 'qa_scan_requested', page_url: pageUrl })
}
```

Update `src/background/message-handlers.ts` so the prompt text consistently asks for the Phase 1 audit workflow and report shape, for example:

```ts
const AUDIT_PROMPT =
  'Run the Kaboom audit workflow for the tracked site. If slash commands are available, use /kaboom/audit (or /audit fallback). Produce the six-lane Phase 1 report.'
```

Do not rename the runtime message yet. Keep the implementation narrow and preserve the existing fallback path to `/intent`.

- [ ] **Step 4: Verify**

Run:

```bash
make compile-ts
node --test tests/extension/request-audit.test.js tests/extension/message-handlers.test.js -v
npm run typecheck
npx jscpd src/background src/popup --min-lines 8 --min-tokens 60
```

Expected:

- TypeScript compiles cleanly.
- The new helper test passes.
- The message-handler test passes with audit-oriented prompt text.
- No unexpected clone warnings are introduced across `src/background` or `src/popup`.

- [ ] **Step 5: Commit**

```bash
git add src/lib/tabs/request-audit.ts src/background/message-handlers.ts tests/extension/request-audit.test.js tests/extension/message-handlers.test.js
git commit -m "feat: add shared audit trigger helper"
```

### Task 2: Add The Tracked Popup `Audit` CTA And Align The Hover Action

**Files:**
- Modify: `extension/popup.html`
- Modify: `extension/popup.css`
- Modify: `src/popup/tab-tracking.ts`
- Modify: `src/popup/tab-tracking-api.ts`
- Modify: `src/content/ui/tracked-hover-launcher.ts`
- Test: `tests/extension/popup-audit-button.test.js`
- Test: `tests/extension/popup-tab-tracking-sync.test.js`
- Test: `tests/extension/tracked-hover-launcher.test.js`

- [ ] **Step 1: Write the failing tests**

Create `tests/extension/popup-audit-button.test.js` with direct popup CTA assertions:

```js
assert.strictEqual(document.getElementById('tracking-bar-audit').style.display, 'none')
showTrackingState(...)
assert.strictEqual(document.getElementById('tracking-bar-audit').textContent, 'Audit')

auditButton.onclick()
assert.ok(runtimeSendMessage.mock.calls.some(
  (c) => c.arguments[0].type === 'qa_scan_requested'
))
```

Extend:

- `tests/extension/popup-tab-tracking-sync.test.js` to assert tracked-state sync reveals the audit CTA.
- `tests/extension/tracked-hover-launcher.test.js` to assert the hover action title uses `Audit` wording instead of `Find Problems`.

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
node --test tests/extension/popup-audit-button.test.js tests/extension/popup-tab-tracking-sync.test.js tests/extension/tracked-hover-launcher.test.js -v
```

Expected:

- The popup audit button test fails because the button does not exist.
- Existing tracked-state and hover-launcher tests fail once the new assertions are added.

- [ ] **Step 3: Write the minimal implementation**

Add a compact tracked-state button to `extension/popup.html`:

```html
<button id="tracking-bar-audit" class="tracking-bar-audit" type="button">Audit</button>
```

Wire it in `src/popup/tab-tracking.ts` / `src/popup/tab-tracking-api.ts` so the button only appears while a site is tracked and delegates to `requestAudit(trackedTabUrl)`.

Update the hover launcher title/copy in `src/content/ui/tracked-hover-launcher.ts` to the same product wording, for example:

```ts
createActionButton('⚑', 'Audit — run the Kaboom audit workflow', () => {
  void requestAudit(location.href)
})
```

Keep the popup/hover behavior identical: open the terminal side panel first, then request the audit bridge.

- [ ] **Step 4: Verify**

Run:

```bash
make compile-ts
node --test tests/extension/popup-audit-button.test.js tests/extension/popup-tab-tracking-sync.test.js tests/extension/tracked-hover-launcher.test.js -v
npm run typecheck
npx jscpd src/background src/popup --min-lines 8 --min-tokens 60
```

Expected:

- The popup renders the audit CTA only in tracked state.
- Both popup and hover surfaces invoke the shared audit helper successfully.
- TypeScript and duplicate-code checks remain clean.

- [ ] **Step 5: Commit**

```bash
git add extension/popup.html extension/popup.css src/popup/tab-tracking.ts src/popup/tab-tracking-api.ts src/content/ui/tracked-hover-launcher.ts tests/extension/popup-audit-button.test.js tests/extension/popup-tab-tracking-sync.test.js tests/extension/tracked-hover-launcher.test.js
git commit -m "feat: add audit entrypoints for tracked sites"
```

## Chunk 2: Audit Workflow Assets And Operator Nudges

### Task 3: Ship The Repo-Owned Audit Command And Bundled Audit Skill

**Files:**
- Create: `plugin/kaboom-workflows/commands/audit.md`
- Modify: `plugin/kaboom-workflows/README.md`
- Create: `npm/kaboom-agentic-browser/skills/audit/SKILL.md`
- Modify: `npm/kaboom-agentic-browser/skills/qa/SKILL.md`
- Modify: `npm/kaboom-agentic-browser/skills/skills.json`
- Test: `tests/packaging/kaboom-audit-workflow.test.js`
- Test: `tests/packaging/kaboom-skills-branding.test.js`

- [ ] **Step 1: Write the failing tests**

Create `tests/packaging/kaboom-audit-workflow.test.js` with assertions that the new prompt assets exist and expose the Phase 1 contract:

```js
assert.match(command, /^name:\\s+kaboom\\/audit/m)
assert.match(command, /Functionality/)
assert.match(command, /UX Polish/)
assert.match(command, /Accessibility/)
assert.match(command, /Performance/)
assert.match(command, /Release Risk/)
assert.match(command, /SEO/)
assert.match(command, /Fast Wins/)
assert.match(command, /Ship Blockers/)

assert.match(manifest, /"id": "audit"/)
assert.match(auditSkill, /tracked site|current tracked page/i)
assert.match(qaSkill, /audit/i)
```

Extend `tests/packaging/kaboom-skills-branding.test.js` to include `npm/kaboom-agentic-browser/skills/audit/SKILL.md`.

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
node --test tests/packaging/kaboom-audit-workflow.test.js tests/packaging/kaboom-skills-branding.test.js -v
```

Expected:

- The new test fails because the `audit` command/skill files do not exist.
- The branding smoke test fails after the new file assertion is added.

- [ ] **Step 3: Write the minimal implementation**

Create `plugin/kaboom-workflows/commands/audit.md` as the canonical Phase 1 audit workflow. Use a frontmatter contract like:

```yaml
---
name: kaboom/audit
description: Run the Kaboom Phase 1 audit for the current tracked site and return a six-lane local report.
argument: focus_prompt
allowed-tools:
  - mcp__kaboom__observe
  - mcp__kaboom__analyze
  - mcp__kaboom__interact
  - mcp__kaboom__configure
---
```

The command/skill should require:

- tracked-site precondition
- `configure(what:"health")`
- baseline `analyze(what:"page_issues", summary:true)`
- quick page map via `interact(what:"explore_page")` / `interact(what:"list_interactive")`
- six lanes:
  - functionality
  - UX polish
  - accessibility
  - performance
  - release risk
  - SEO

The output skeleton should be fixed and explicit:

```md
# Kaboom Audit Report: [site]
## Overall Score
## Lane Scores
## Executive Summary
## Top Findings
## Fast Wins
## Ship Blockers
## Coverage And Limits
```

Add `npm/kaboom-agentic-browser/skills/audit/SKILL.md` with the same workflow. Reduce `npm/kaboom-agentic-browser/skills/qa/SKILL.md` to a compatibility wrapper that points agents to the audit workflow instead of preserving two separate methodologies.

If a client rejects `name: kaboom/audit`, keep the file path flat and document `/audit` as the fallback invocation in the same patch.

- [ ] **Step 4: Verify**

Run:

```bash
node --test tests/packaging/kaboom-audit-workflow.test.js tests/packaging/kaboom-skills-branding.test.js -v
```

Expected:

- The new command and skill files exist.
- The report contract is present in the prompt assets.
- Bundled skill branding remains Kaboom-only.

- [ ] **Step 5: Commit**

```bash
git add plugin/kaboom-workflows/commands/audit.md plugin/kaboom-workflows/README.md npm/kaboom-agentic-browser/skills/audit/SKILL.md npm/kaboom-agentic-browser/skills/qa/SKILL.md npm/kaboom-agentic-browser/skills/skills.json tests/packaging/kaboom-audit-workflow.test.js tests/packaging/kaboom-skills-branding.test.js
git commit -m "feat: add Kaboom audit workflow assets"
```

### Task 4: Point Pending-Intent Warnings At The Audit Workflow

**Files:**
- Modify: `cmd/browser-agent/handler_tools_call_postprocess.go`
- Create: `cmd/browser-agent/handler_tools_call_postprocess_test.go`

- [ ] **Step 1: Write the failing Go test**

Create `cmd/browser-agent/handler_tools_call_postprocess_test.go` and cover the pending-intent warning explicitly:

```go
func TestMaybeAddPendingIntents_UsesAuditWorkflowCopy(t *testing.T) {
    resp := handler.maybeAddPendingIntents(baseResp)
    text := firstTextFrom(resp)
    if !strings.Contains(text, "audit") {
        t.Fatalf("warning should point to audit workflow, got %q", text)
    }
    if strings.Contains(text, "qa skill") {
        t.Fatalf("warning should no longer point to the old QA skill wording, got %q", text)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./cmd/browser-agent -run TestMaybeAddPendingIntents_UsesAuditWorkflowCopy -v
```

Expected:

- The test fails because the warning still tells the AI to run `page_issues` or the older QA skill.

- [ ] **Step 3: Write the minimal implementation**

Update only the warning string in `cmd/browser-agent/handler_tools_call_postprocess.go`. Preserve:

- existing pending-intent behavior
- nudge count semantics
- `qa_scan` internal action name

The new wording should tell the operator/agent to run the audit workflow, for example:

```go
"ACTION REQUIRED: The user clicked 'Audit' in the browser. Run the Kaboom audit workflow (/kaboom/audit or /audit fallback) for a full six-lane report."
```

- [ ] **Step 4: Verify**

Run:

```bash
go test ./cmd/browser-agent -run 'TestMaybeAddPendingIntents_UsesAuditWorkflowCopy|TestToolsAnalyzePageIssues' -v
```

Expected:

- The new warning-copy regression test passes.
- Existing `page_issues` coverage remains green.

- [ ] **Step 5: Commit**

```bash
git add cmd/browser-agent/handler_tools_call_postprocess.go cmd/browser-agent/handler_tools_call_postprocess_test.go
git commit -m "feat: point audit nudges at the new workflow"
```

## Chunk 3: Docs, Flow Maps, And Final Phase 1 Verification

### Task 5: Update Canonical Docs And Run The Full Targeted Verification Pass

**Files:**
- Create: `docs/architecture/flow-maps/audit-workflow.md`
- Modify: `docs/architecture/flow-maps/README.md`
- Modify: `docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md`
- Modify: `docs/features/feature/auto-fix/index.md`
- Modify: `docs/features/feature/auto-fix/flow-map.md`
- Modify: `docs/features/feature/tab-tracking-ux/index.md`
- Modify: `docs/features/feature-navigation.md`

- [ ] **Step 1: Write the docs changes**

Create `docs/architecture/flow-maps/audit-workflow.md` as the new canonical map for:

- tracked popup `Audit` button
- hover-surface audit action
- `requestAudit` helper
- background terminal/intent bridge
- `/kaboom/audit` command / audit skill
- terminal markdown report output

Update:

- `docs/architecture/flow-maps/README.md` to list the new canonical map
- `docs/features/feature/auto-fix/flow-map.md` to point at `audit-workflow.md`
- `docs/features/feature/auto-fix/index.md` to explain that the legacy QA/find-problems plumbing now powers the product-shaped audit workflow
- `docs/features/feature/tab-tracking-ux/index.md` to add the tracked popup `Audit` CTA to `code_paths`, `test_paths`, and TL;DR
- `docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md` to rename the hover action and describe the shared audit trigger
- `docs/features/feature-navigation.md` to update the `auto-fix` summary text so LLMs discover the new audit shape instead of the older QA-only wording

- [ ] **Step 2: Run the targeted verification suite**

Run:

```bash
make compile-ts
node --test tests/extension/request-audit.test.js tests/extension/popup-audit-button.test.js tests/extension/popup-tab-tracking-sync.test.js tests/extension/tracked-hover-launcher.test.js tests/extension/message-handlers.test.js tests/packaging/kaboom-audit-workflow.test.js tests/packaging/kaboom-skills-branding.test.js -v
go test ./cmd/browser-agent -run 'TestMaybeAddPendingIntents_UsesAuditWorkflowCopy|TestToolsAnalyzePageIssues' -v
npm run typecheck
npx jscpd src/background src/popup --min-lines 8 --min-tokens 60
```

Expected:

- TypeScript compiles cleanly.
- Popup, hover, shared-trigger, background, and packaging tests all pass.
- Go tests for the pending-intent nudge and `page_issues` remain green.
- No duplicate-code regressions are introduced in `src/background` or `src/popup`.

- [ ] **Step 3: Sanity-check the docs cross-reference contract**

Run:

```bash
rg -n "audit-workflow|tracking-bar-audit|kaboom/audit|Audit — run the Kaboom audit workflow" docs/architecture docs/features src extension
```

Expected:

- Hits appear in the new canonical flow map, the updated feature docs, the popup/hover code, and the new prompt asset references.

- [ ] **Step 4: Review the final diff**

Run:

```bash
git diff --stat HEAD~4..HEAD
git status --short
```

Expected:

- Only the planned audit UI, prompt asset, nudge-copy, test, and docs files are present.
- The worktree is clean or contains only the final docs updates awaiting commit.

- [ ] **Step 5: Commit**

```bash
git add docs/architecture/flow-maps/audit-workflow.md docs/architecture/flow-maps/README.md docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md docs/features/feature/auto-fix/index.md docs/features/feature/auto-fix/flow-map.md docs/features/feature/tab-tracking-ux/index.md docs/features/feature-navigation.md
git commit -m "docs: map the Phase 1 Kaboom audit workflow"
```

