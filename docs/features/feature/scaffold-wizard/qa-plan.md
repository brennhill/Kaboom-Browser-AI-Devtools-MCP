---
doc_type: qa-plan
feature_id: feature-scaffold-wizard
status: proposed
scope: feature/scaffold-wizard/qa
ai-priority: medium
tags: [testing, qa, scaffold, composition]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
last_reviewed: 2026-07-05
---

# QA Plan: Scaffold Wizard

> Verifies the two-phase Scaffold Wizard: a deterministic Phase 1 scaffold with verification gates, and a Phase 2 AI live-composition loop that builds the first screen in the browser. Covers data-leak analysis, agent clarity, simplicity, code-level tests, and step-by-step user acceptance testing (UAT).

---

## 1. Data Leak Analysis

**Goal:** Confirm the idea description and project data stay local, and that no secrets are committed or transmitted without explicit user action.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Idea description transmission | Verify the wizard answers post only to the local daemon (`/api/scaffold`), never to a third party | critical |
| DL-2 | Committed secrets | Verify generated `.env.local.example` holds placeholders only, and `.gitignore` excludes real `.env` files | critical |
| DL-3 | Pre-deploy secret scan | Verify the generated pre-deploy gate greps for API keys, tokens, and passwords in source before any deploy | high |
| DL-4 | Third-party connections are user-initiated | Verify GitHub, Supabase, Stripe, deploy, scanner, and error-tracking connections only happen through the provider's own auth flow | high |
| DL-5 | Progress stream contents | Verify the WebSocket progress messages do not carry credentials or full environment dumps | medium |
| DL-6 | Domain setup privilege | Verify the `.kaboom` proxy setup is opt-in and never silently elevates privileges | medium |
| DL-7 | Dependency audit gate | Verify the pre-deploy step runs a dependency audit and blocks on high or critical vulnerabilities | high |

### Negative Tests (must NOT leak)
- [ ] Real `.env` values are never committed
- [ ] The idea description never reaches an external endpoint
- [ ] Progress messages contain no secrets
- [ ] No third-party connection occurs without explicit user consent

---

## 2. Agent Clarity Assessment

**Goal:** Confirm the AI composition agent receives unambiguous context and that generated AI-context files steer downstream agents correctly.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Composition input completeness | The agent receives idea, audience, first feature, installed components, and project path | [ ] |
| CL-2 | Bootstrap skill invariants | The generated skill clearly states Tailwind-first, shadcn-first, theme-tokens-only, one-test-per-component, and `@/` import rules | [ ] |
| CL-3 | Project context file | The generated project context names the stack, conventions, and dev commands unambiguously | [ ] |
| CL-4 | Verification signals | Screenshot-verify results (`pass`, `fix_needed`) are clearly distinct so the agent loops correctly | [ ] |
| CL-5 | Goal-backward criteria | The four levels (exists, substantive, wired, functional) are clear enough to reject boilerplate | [ ] |
| CL-6 | Progress channel tags | `scaffold`, `compose`, `terminal`, and `wizard` channels are clearly separated | [ ] |

### Common Agent Misinterpretation Risks
- [ ] Agent treats "compiles" as "done" and skips visual verification
- [ ] Agent introduces inline styles or a second icon library, violating the bootstrap invariants
- [ ] Agent declares completion before goal-backward verification passes

---

## 3. Simplicity Assessment

**Goal:** Confirm the path from idea to editable first screen is short and that infrastructure steps are skippable.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Idea to first screen | Wizard conversation + one "Create" click | No — already a single guided flow |
| Add backups / deploy / scanner | One click each, all skippable | No — single click per option |
| Edit a composed component | Click the element in the browser (annotation mode) | No — direct manipulation |
| Re-run after a failed step | Engine retries once automatically | No — automatic with one retry |

### Default Behavior Verification
- [ ] Every infrastructure step is skippable
- [ ] "Just me" audience skips the auth, database, and payments questions appropriately
- [ ] The project name auto-generates from the idea and is editable

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Project name slugification | "A Todo App!" | `todo-app` | must |
| UT-2 | Audience gating | audience = "just_me" | Auth/database/payments questions skipped | must |
| UT-3 | Scaffold payload validation | Missing `description` | Rejected with a clear error | must |
| UT-4 | Step verification gate | Step exits non-zero | Retry once, then stream recovery hint | must |
| UT-5 | Dev-server ready parsing | Stdout with a ready URL | Correct URL detected | must |
| UT-6 | Dev-server ready timeout | No ready line within 30s | Timeout error | must |
| UT-7 | AI context file generation | Wizard answers | Project context, bootstrap skill, hooks, MCP config written | must |
| UT-8 | Channel routing | Progress events | Routed to `scaffold` / `compose` / `terminal` / `wizard` | should |
| UT-9 | Portless fallback | sudo denied | Falls back to localhost port and persists the choice | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full Phase 1 scaffold | Endpoint -> PTY engine -> verification gates | Project type-checks and the dev server serves | must |
| IT-2 | Phase 1 to Phase 2 handoff | Dev server ready -> navigate -> compose | Browser points at the running app; composition begins | must |
| IT-3 | Composition loop | Write -> HMR -> `observe` screenshot -> verify | Each component verified before the next | must |
| IT-4 | Goal-backward verification | Edit a class -> confirm HMR | Change visible within two seconds | must |
| IT-5 | Skip-all infrastructure | All optional steps skipped | Project still scaffolds and composes | should |
| IT-6 | Failed step aborts cleanly | Forced step failure | No Phase 2; clear recovery hint streamed | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Phase 1 duration | Wall-clock scaffold time | ~20s | should |
| PT-2 | Per-component cycle | Write to verified | 2-5s | should |
| PT-3 | HMR update | File save to browser update | < 2s | must |
| PT-4 | Full first screen | Phase 2 duration | 30-60s | should |

### 4.4 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | pnpm missing | No pnpm on PATH | Wizard offers an install path before "Create" | must |
| EC-2 | node too old | node < 18 | Prerequisite error with guidance | must |
| EC-3 | Port already in use | Dev port occupied | Engine selects or reports an alternate cleanly | should |
| EC-4 | Layout regression in Phase 2 | Component breaks at 375px | Responsive pass catches and fixes it | should |
| EC-5 | Accessibility violations | Missing labels or alt text | Accessibility pass fixes before completion | should |

---

## 5. UAT Checklist (Human + AI)

> The human drives the wizard in the browser; the AI agent performs Phase 2 composition.

### Prerequisites
- [ ] Kaboom daemon running and serving `/launch`
- [ ] node >= 18 and pnpm on PATH
- [ ] Chrome with the Kaboom extension installed (for annotation mode)

### Step-by-Step Verification

| # | Step | Human Observes | Expected Result | Pass |
|---|------|----------------|-----------------|------|
| UAT-1 | Open `/launch` and answer the wizard | Conversational flow, one step at a time | Each step builds on the last; "Create" appears at the end | [ ] |
| UAT-2 | Click "Create" | Phase 1 progress streams | Steps show spinner then checkmark | [ ] |
| UAT-3 | Wait for Phase 1 to finish | Dev server starts; browser navigates | The running app loads | [ ] |
| UAT-4 | Watch Phase 2 compose | Components appear live | Layout, navigation, and feature build incrementally | [ ] |
| UAT-5 | Inspect the result | First screen reflects the idea | Real layout and content, not boilerplate | [ ] |
| UAT-6 | Edit a Tailwind class in a component | Browser updates | Change visible within two seconds (HMR) | [ ] |
| UAT-7 | Click an element in the browser | Annotation mode reveals source | The clicked element maps to its source file | [ ] |
| UAT-8 | Skip all infrastructure, repeat | Project still builds | Scaffold and composition succeed without backups/deploy | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Local-only scaffold post | Monitor network on "Create" | Request goes to the local daemon only | [ ] |
| DL-UAT-2 | No committed secrets | Inspect the initial commit | Only `.env.local.example` placeholders present | [ ] |
| DL-UAT-3 | Pre-deploy gate | Run the deploy command | Secret-leak and dependency-audit gates execute | [ ] |

### Regression Checks
- [ ] The Kaboom popup and existing tools still work while the wizard runs
- [ ] Generated git hooks auto-fix on commit and block only on human-judgment issues
- [ ] A second scaffold reuses the persisted domain choice without re-prompting

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
