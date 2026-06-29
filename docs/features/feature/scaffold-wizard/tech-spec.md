---
doc_type: tech-spec
feature_id: feature-scaffold-wizard
status: proposed
feature: scaffold-wizard
owners: []
last_reviewed: 2026-06-29
links:
  index: ./index.md
  product: ./product-spec.md
  qa: ./qa-plan.md
  standards: ./code-quality-standards.md
---

# Scaffold Wizard Tech Spec

> Plain language only. Describes HOW the two-phase scaffold and live-composition flow works at a high level.

## TL;DR

- Design: a static wizard at `GET /launch` posts to a scaffold endpoint that runs a deterministic build in a pseudo-terminal (PTY), then hands off to an AI agent that composes the first screen through Kaboom's own tools.
- Key constraints: each build step is gated by an automated check; progress streams over a WebSocket; the running browser app is the verification surface.
- Rollout risk: medium-high — orchestrates external tooling (pnpm, git, deploy and scanner CLIs) and an AI loop, so failure handling and verification gates are central.

## Architecture Overview

The wizard is a static HyperText Markup Language (HTML), Cascading Style Sheets (CSS), and JavaScript page served by the Kaboom daemon at `GET /launch`. It needs no framework of its own. When the user clicks "Create", the page posts the collected answers to a scaffold endpoint, which returns immediately with a channel identifier. All subsequent progress streams over a WebSocket, multiplexed by channel tag, so the user interface stays responsive while long-running commands execute.

Two phases run in sequence:

1. **Phase 1 (automated):** a scaffold engine runs shell commands inside a PTY session, gating each step on a verification check.
2. **Phase 2 (AI composition):** once the dev server serves the running app, an AI agent composes the first screen by writing component files and verifying each through Kaboom's `observe`, `analyze`, and `interact` tools.

## Key Components

**Wizard landing page (`GET /launch`).** Static assets served by the daemon. Renders the conversational flow one question at a time, posts the final payload, then switches to a progress view for Phase 1 and a split browser-plus-log view for Phase 2.

**Scaffold endpoint (`POST /api/scaffold`).** Accepts the wizard payload — idea description, audience, first feature, project name, and the chosen infrastructure options. Responds `202 Accepted` with a channel identifier and runs the build asynchronously. The request body uses `snake_case` fields consistent with Kaboom's JSON conventions.

**Phase 1 scaffold engine.** Executes an ordered list of steps inside a PTY: create the Vite project, install dependencies, add Tailwind and shadcn, install a curated component set and icons, apply the quality baseline (strict `tsconfig`, ESLint flat config, Prettier, dead-code detection, Vitest), generate AI context files, initialize git, optionally push to GitHub and wire a deploy platform and scanner, and start the dev server. Each step has a verification gate; a failed gate retries once, then streams a recovery hint and aborts.

**AI context generation.** Writes project-local context so any agent immediately understands the project: a project context file (stack, audience, first feature, conventions, dev commands), a bootstrap skill encoding anti-slop invariants (Tailwind-first, shadcn-first, theme tokens only, one test per component, `@/` imports, file-size limits), Claude Code hooks (session-start bootstrap, post-tool-use context monitor, status line), and an MCP configuration that pre-wires Kaboom as a server.

**Phase 2 composition engine.** Receives the idea, audience, first feature, the list of installed components, and the project path. It composes outside-in and runs a per-component loop: write the file, let Vite HMR render it, screenshot via `observe`, verify the layout, fix and re-verify on failure, then proceed. It performs a responsive pass at mobile width and an accessibility pass before the final commit.

**Goal-backward verification.** After composition, the engine verifies the user's actual goal rather than command success: edit a Tailwind class and confirm HMR updates within two seconds (functional), confirm components and styles resolve (wired), confirm the screen reflects the idea rather than boilerplate (substantive), and confirm all expected files exist (exists).

**WebSocket progress fabric.** A single connection multiplexed by channel tag: `scaffold` (Phase 1 step status), `compose` (Phase 2 file writes, HMR, screenshots, fixes), `terminal` (raw PTY output for power users), and `wizard` (user interactions).

## Data Flow

```
Browser: GET /launch  (conversational wizard)
  |  user answers steps, clicks Create
  v
POST /api/scaffold  { description, audience, first_feature, name, options... }
  |  202 Accepted { channel }
  v
Phase 1 scaffold engine (PTY)
  |  step -> run command -> verify gate -> stream {channel: scaffold}
  |  ... create, install, tailwind, shadcn, baseline, context, git, dev server
  v
Dev server ready (HTTP 200) -> navigate browser to project URL
  |
  v
Phase 2 composition engine (AI agent)
  |  write component -> Vite HMR -> observe(screenshot) -> verify
  |  fix-and-reverify loop -> next component
  |  responsive pass (375px) -> accessibility pass -> final commit
  |  stream {channel: compose}
  v
Goal-backward verification -> Ready screen
```

## Implementation Strategy

**Daemon (Go):** add the `/launch` static handler and the `POST /api/scaffold` endpoint. The scaffold engine drives a PTY and emits structured progress messages; the endpoint returns a channel identifier immediately and never blocks the HTTP response on the long-running build.

**Generated project (TypeScript):** the quality baseline and AI context files are templates emitted during Phase 1. The bootstrap skill and hooks are copied into the project's agent-configuration directory so they survive context resets in downstream sessions.

**Composition (AI agent + MCP tools):** Phase 2 reuses the existing five-tool surface. No new tool is required; the agent writes files on disk and verifies through `observe` and `analyze`.

**Trade-offs:**
- Static wizard versus a framework: a static page keeps the wizard itself dependency-free and instantly served by the daemon.
- Separate progress port versus the main port: a separate WebSocket avoids the main server's request timeouts during long builds; consolidation is deferred.
- One component per HMR cycle versus batch writes: incremental writes make progress visible and keep verification granular, at the cost of more round trips.

## Edge Cases and Assumptions

### Edge Cases

- **Missing prerequisites (node, pnpm):** the wizard checks before showing "Create" and offers an install path.
- **A verification gate fails twice:** the engine streams a recovery hint and stops before Phase 2.
- **Dev server never ready:** detection times out at thirty seconds with a clear error.
- **`.kaboom` domain setup declined or sudo fails:** the dev server falls back to a localhost port and the choice is persisted.
- **Phase 2 layout regression:** the screenshot-verify step catches it and the agent corrects before continuing.
- **All infrastructure skipped:** the project still scaffolds and composes; integrations can be added later.

### Assumptions

- A1: the Kaboom daemon is running, since the user reached `/launch` through it.
- A2: the project directory under the user's projects root is writable.
- A3: Vite HMR is fast enough (sub-two-second) for live composition to feel real-time.
- A4: the curated shadcn component set covers most first-screen needs, so the agent composes rather than writing raw markup.

## Risks and Mitigations

### Risk 1: External tooling drift
- Description: pnpm, shadcn, or a deploy or scanner CLI changes its interface and breaks a step.
- Mitigation: each step is gated by an explicit verification check, so a broken step fails loudly with a recovery hint rather than silently producing a bad project.

### Risk 2: AI composition produces boilerplate or slop
- Description: the agent emits generic or low-quality output.
- Mitigation: the bootstrap skill encodes anti-slop invariants, the per-component loop verifies visually, and goal-backward verification rejects boilerplate before completion.

### Risk 3: Long builds block the user interface
- Description: synchronous command execution freezes the wizard.
- Mitigation: the scaffold endpoint returns a channel immediately and all progress streams over the WebSocket; commands run in a PTY off the request path.

### Risk 4: Privileged domain setup friction
- Description: the `.kaboom` proxy needs elevated privileges the user cannot grant.
- Mitigation: domain setup is opt-in and explained, with a guaranteed localhost-port fallback.

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Wizard render and post | instant | Static assets, single POST |
| Phase 1 scaffold | ~20s | Deterministic steps, parallel installs where possible |
| Phase 2 per component | 2-5s | Write -> HMR -> screenshot -> verify |
| HMR update | < 2s | Vite incremental rebuild |
| Dev-server ready detection | < 30s | Parse stdout for the ready URL; poll as fallback |
