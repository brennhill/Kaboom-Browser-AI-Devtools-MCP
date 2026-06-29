---
doc_type: product-spec
feature_id: feature-scaffold-wizard
status: proposed
feature: scaffold-wizard
owners: []
created: 2026-06-29
updated: 2026-06-29
last_reviewed: 2026-06-29
links:
  index: ./index.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  standards: ./code-quality-standards.md
---

# Scaffold Wizard (Product Spec)

> A browser-based wizard that takes a user from "I want to build X" to a real, editable first screen running in the browser in under ninety seconds. A fast deterministic scaffold builds the project, then an artificial intelligence (AI) agent live-composes the opening user interface while the user watches.

## Problem

Starting a new web application is slow and full of decisions that do not matter to a beginner: which framework, which styling system, which package manager, how to wire linting, tests, deploys, and backups. Existing scaffolding tools stop at boilerplate — a spinning logo and a "click to edit" placeholder. The user is left staring at a blank app with no idea what to do next, and no sense that anything was actually built for *their* idea.

Kaboom already runs a local daemon and a browser extension that can observe and drive a page. That makes a richer experience possible: instead of dumping boilerplate into a folder, Kaboom can stand up a real project and then build the first screen live in the browser, so the user sees their idea become a running application step by step.

**The gap:** there is no path from a plain-language idea to a meaningful, editable first screen that the user watches being built, with quality gates, backups, and deploy wiring already in place.

## Solution

A two-phase **Scaffold Wizard** served at `GET /launch` by the Kaboom daemon:

- **Phase 1 — Automated Scaffold (~20 seconds).** Fast and deterministic, no AI needed. Creates a Vite plus React plus TypeScript project, installs Tailwind CSS and shadcn/ui, applies a strict quality baseline (TypeScript strict mode, ESLint, Prettier, dead-code detection, Vitest, git hooks), generates AI context files, initializes git, optionally backs up to GitHub and wires a deploy platform, then starts the dev server and points the browser at it.

- **Phase 2 — AI Live Composition (~60 seconds).** An AI agent uses Kaboom's own tools (`interact`, `observe`, `analyze`) to build the first real screen: it plans a layout, writes components one at a time, lets Vite Hot Module Replacement (HMR) render each change instantly, screenshots to verify, and corrects until the screen reflects the user's described idea. The user watches the app assemble itself in real time.

The wizard itself is a guided conversation, not a form. Each step builds on the previous answer and explains why it matters, so infrastructure choices (backups, deploy, security scanning, error tracking) are understood rather than surprising.

### Key design decisions

- **Opinionated stack, zero choices.** React plus Vite, TypeScript strict, Tailwind CSS, shadcn/ui, and pnpm are chosen for the user. The goal is speed to editing, not framework selection. This stack also makes live composition and annotation-mode editing reliable because components map cleanly to source files.

- **The browser is the canvas.** Composition happens in the running app, not a terminal. Each component write triggers an HMR update and a screenshot-verify cycle, mirroring a live-design experience.

- **Quality from minute one.** Every generated project ships with auto-fixing pre-commit hooks, a test floor, and a pre-deploy security scan, so the codebase stays clean without the user configuring anything.

## User Stories

- As a non-expert builder, I want to describe my idea in plain language and watch a real first screen appear, so that I feel I built something, not just generated boilerplate.
- As a builder, I want backups, deploys, and security scanning set up for me with brief explanations, so that I understand my project's safety net without researching tools.
- As a builder, I want the AI agent to verify each component visually as it writes it, so that the result actually looks right rather than merely compiling.
- As a returning user, I want the wizard to skip lessons I have already seen, so that experienced builders are not slowed down.
- As a developer handed a Kaboom-scaffolded project, I want the project's conventions encoded in AI context files and git hooks, so that any agent or teammate stays consistent.

## Wizard Flow

The conversation has three parts and ends with a single "Create" action.

1. **What are you building? (~30s)** Free-text idea, audience ("Just me" / "My team" / "Public users"), most important first feature, and an auto-generated project name.
2. **What does your app need? (~30s)** Audience-gated questions about accounts, data storage, and payments, each of which installs a best-of-breed managed service (Supabase for auth, database, and storage; Stripe for payments). Every option is skippable.
3. **Set up your safety net (~30s)** Back up to GitHub, choose a deploy platform, connect a security scanner, and opt into production error tracking. Each step is one click and skippable.

The "Create" screen summarizes exactly what is about to happen before anything runs.

## Build Phases

### Phase 1: Automated Scaffold

A sequence of deterministic steps, each with a verification gate (directory exists, dependency installed, type-check passes, dev server returns HTTP 200). A failed step retries once, then streams an error with a recovery suggestion. Progress streams to the wizard over a WebSocket.

### Phase 2: AI Live Composition

The agent works outside-in: layout shell, navigation, the primary feature component(s), realistic sample content, a responsive pass at mobile width, and an accessibility pass. Each component runs the loop: write file, HMR update, screenshot, verify, fix if needed, then move on. A final goal-backward verification confirms the user can edit a component and see the change within two seconds.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Serve a conversational wizard at `GET /launch` from the Kaboom daemon | must |
| R2 | Accept a scaffold request and stream progress over a WebSocket | must |
| R3 | Phase 1 produces a Vite + React + TypeScript + Tailwind + shadcn project that type-checks and serves | must |
| R4 | Each Phase 1 step has an automated verification gate with one retry | must |
| R5 | Generate AI context files (project context, bootstrap skill, hooks, MCP config) into the project | must |
| R6 | Phase 2 composes the first screen using `interact` / `observe` / `analyze` with screenshot verification | must |
| R7 | Goal-backward verification confirms the user can edit and see changes before declaring done | must |
| R8 | Audience-gated backend questions install Supabase / Stripe and are skippable | should |
| R9 | Optional GitHub backup, deploy wiring, security scanner, and error tracking, each skippable | should |
| R10 | Contextual lessons appear alongside composition without blocking progress | could |
| R11 | Generated projects ship auto-fixing pre-commit hooks, a test floor, and a pre-deploy security scan | must |
| R12 | Pretty local URLs via portless with a graceful fallback to a localhost port | could |

## Non-Goals

- The wizard does NOT support arbitrary framework choice; the stack is fixed by design.
- The wizard does NOT write backend server code; persistence is via managed-service client SDKs.
- The wizard does NOT deploy automatically; deploy is a separate, user-initiated command.
- Out of scope: native mobile or desktop targets.
- Out of scope: migrating an existing project into the Kaboom conventions.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| Wizard conversation time | 60-90s | Short enough to feel effortless |
| Phase 1 scaffold | ~20s | Deterministic, parallelizable installs |
| Phase 2 first screen | 30-60s | One component per HMR/screenshot cycle |
| HMR update after a file write | < 2s | Live composition must feel real-time |
| Dev-server ready detection | < 30s timeout | Fail fast with a clear error if the server never boots |

## Security and Privacy

- The wizard runs entirely on the local daemon; the idea description and answers never leave the machine except when the user explicitly connects a third-party service.
- Domain setup that needs elevated privileges (the `.kaboom` proxy) is opt-in, explained, and falls back to a localhost port if declined or unavailable.
- Generated projects include a pre-deploy secret-leak check and a dependency-audit gate so nothing ships with known vulnerabilities or committed credentials.
- Connecting GitHub, Supabase, Stripe, deploy platforms, scanners, or error trackers is always user-initiated through the provider's own authentication flow.

## Edge Cases

- **pnpm or node missing.** The wizard checks prerequisites before showing "Create" and offers an actionable install path.
- **A Phase 1 step fails twice.** The wizard stops, streams the error with a recovery suggestion, and does not proceed to Phase 2.
- **The dev server never reaches ready.** Detection times out at thirty seconds and surfaces a clear error.
- **The user skips every infrastructure step.** The project still scaffolds and composes; backups and deploys can be added later.
- **`.kaboom` domain setup is declined or sudo fails.** The dev server falls back to a localhost port and records the choice so the user is not asked again.
- **Phase 2 produces a broken layout.** The screenshot-verify step catches it and the agent fixes it before continuing.

## Dependencies

- **Depends on:** the Kaboom daemon HTTP server (serves `/launch`), the WebSocket terminal server (streams progress), and the five MCP tools used during composition.
- **Depended on by:** annotation mode and the generated project's hooks and bootstrap skill, which assume the conventions the scaffold establishes.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Default deploy platform and scanner | open | Pending partner terms; "Recommended" badge drives the default |
| OI-2 | Consolidate the progress WebSocket onto the main port | open | Currently a separate port; not required for v1 |
| OI-3 | Detect returning users to reduce lessons | open | Cookie or local-storage signal; privacy-friendly default |
