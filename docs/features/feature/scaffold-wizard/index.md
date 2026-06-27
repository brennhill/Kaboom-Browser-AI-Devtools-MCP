# Scaffold Wizard

status: spec
last_reviewed: 2026-03-28

## Overview

A browser-based scaffold wizard that lets users create a new web app from inside Kaboom. Two phases: a fast automated scaffold builds the empty project and starts the dev server, then an AI agent takes over and **live-composes the first real UI in the browser** — the user watches components appear, layouts form, and content populate in real-time.

Goal: go from "I want to build X" to a meaningful, editable app in the browser — not boilerplate, a real first screen — in under 90 seconds.

## Design Influences

**Pencil** (pencil.dev) — Live composition. Pencil lets you watch Claude build a design in real-time inside the app via atomic batch operations, progressive rendering, placeholder indicators, and screenshot verification loops. We apply this same pattern: the user watches their app being built in the browser, not in a terminal.

**Superpowers** (github.com/obra/superpowers) — Socratic refinement and anti-rationalization. The wizard guides users through progressive follow-ups before any code runs. Post-scaffold, a bootstrap skill orients the AI agent to project conventions.

**Get Shit Done** (github.com/gsd-build/get-shit-done) — Defense-in-depth verification. Every step has an automated check. Goal-backward verification confirms the user can actually edit and see changes. Four-level artifact checks (Exists -> Substantive -> Wired -> Functional) catch stubs and broken wiring.

## User Flow

### Conversational Wizard (not a form)

The wizard is a guided conversation, not a wall of inputs. Each step builds on the previous answer. The wizard handles both the creative decisions (what to build) and the infrastructure setup (where to save, how to deploy), explaining *why* at each step so the user understands what's being set up and isn't surprised later.

#### Part 1: What Are You Building? (~30 seconds)

**Step 1: "What are you building?"**
Free text. Examples shown as inspiration: "a todo app", "a dashboard for my sales data", "a landing page for my startup".

**Step 2: "Who is it for?"**
Three choices: "Just me" / "My team" / "Public users"
This shapes the scaffold — auth scaffolding for team/public, simpler for personal.

**Step 3: "What's the most important first feature?"**
Free text, one thing. Forces the user to prioritize before code exists. This becomes the seed for the AI composition phase.

**Step 4: Project name**
Auto-generated from Step 1 (slugified). Editable, but most users won't touch it.

#### Part 2: What Does Your App Need? (~30 seconds)

Based on the audience (Step 2), the wizard asks about backend needs. Each question auto-installs the best-of-breed tool. Every option is skippable with "not yet" — users can add these later.

**Step 5: "Does your app need user accounts?"**
Only shown for "My team" or "Public users" audiences. Skipped for "Just me."

Two choices:
- **"Yes"** → installs Supabase Auth
- **"Not yet"** → skipped, but Supabase SDK is still installed (they'll want the database anyway)

*"We'll set up login, signup, and session management. Users get a real account system from day one."*

What gets installed:
- `@supabase/supabase-js` + `@supabase/auth-ui-react`
- `src/lib/supabase.ts` — client initialization with env var for project URL + anon key
- `src/components/AuthGuard.tsx` — wraps protected routes
- `src/components/LoginPage.tsx` — shadcn-styled login/signup form with email + social options
- `.env.local.example` — with `VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY` placeholders
- Wizard offers "Create Supabase project" button → opens supabase.com/dashboard

**Step 6: "Does your app need to store data?"**
Only shown for "My team" or "Public users" audiences. For "Just me," local storage is the default (no question asked, but `src/lib/storage.ts` wrapper is generated).

Two choices:
- **"Yes"** → installs Supabase database (Postgres)
- **"Not yet"** → skipped (Supabase SDK already installed from auth step; easy to add later)

*"We'll connect a database so your data persists across devices and users."*

What gets installed:
- Supabase client (shared with auth — one SDK, one project, no duplication)
- `src/lib/database.ts` — typed query helpers with row-level security
- `src/types/database.ts` — type definitions (generated from Supabase schema if connected)
- Example: if first feature is "todo list," generates `src/hooks/useTodos.ts` with CRUD operations

**Step 7: "Will you need to accept payments?"**
Only shown for "Public users" audience.

Two choices:
- **"Yes"** → installs Stripe
- **"Not yet"** → skipped

*"We'll wire up Stripe so you can charge for your product. Free until you start processing payments."*

What gets installed:
- `@stripe/stripe-js` + `@stripe/react-stripe-js`
- `src/lib/stripe.ts` — Stripe initialization
- `src/components/PricingTable.tsx` — shadcn-styled pricing component (placeholder)
- `.env.local.example` updated with `VITE_STRIPE_PUBLISHABLE_KEY`

#### Opinionated Backend Stack

We choose for them based on audience. The goal is zero backend code — everything is a client-side SDK call to a managed service.

| Need | "Just me" | "My team" / "Public" | Why this choice |
|------|-----------|---------------------|-----------------|
| **Auth** | None | Supabase Auth | Free tier generous (50K MAU). Auth + database in one service. Open source. |
| **Database** | localStorage wrapper | Supabase (Postgres) | Same service as auth. One SDK. Row-level security. Realtime subscriptions. |
| **Payments** | None | Stripe | Industry standard. Best docs. Free until revenue. |
| **File storage** | None | Supabase Storage | Already connected. S3-compatible. |
| **Email** | None | Supabase Edge Functions + Resend | Added via bootstrap skill when user asks, not scaffolded by default |

**Why Supabase as the only BaaS:**
- Auth + database + storage + realtime + edge functions = one account, one SDK, one dashboard
- No migration path to worry about — everything is in one place from day one
- Open source (can self-host later if needed)
- Generous free tier (500MB database, 1GB storage, 50K MAU)
- TypeScript SDK with full type generation from schema
- Row-level security = auth-aware queries without writing backend code
- Revenue share via Supabase partner program

No alternatives offered. One BaaS, one path. Users who outgrow Supabase have graduated from Kaboom.

#### Revenue Share: Backend Services

| Service | Rev Share | Notes |
|---------|-----------|-------|
| **Supabase** | Yes — partner program | Auth + database + storage in one. $25/mo Pro plan conversion. |
| **Stripe** | No direct referral | Every Stripe-enabled app = Stripe revenue = co-marketing goodwill |
| **Resend** | Yes — referral program | Email sending, when user needs it later |

At scale: Supabase alone is the highest-value conversion — most projects will need both auth and database ($25/mo Pro plan).

#### Part 3: Set Up Your Safety Net (~30 seconds)

After the backend questions, the wizard handles infrastructure. Same pattern — brief explainer, one-click setup, skippable.

**Step 8: "Let's back up your code"**

*"We'll save your project to GitHub so you never lose your work. It's free and takes 10 seconds."*

- If `gh` is installed and authenticated: show green checkmark, "Connected as @username"
- If `gh` is installed but not authenticated: "Sign in to GitHub" button → runs `gh auth login` in embedded terminal
- If `gh` not installed: "Install GitHub CLI" link + brief instructions
- Skip option: "I'll do this later" (small text, not prominent)

On completion: creates private repo, pushes initial code. The wizard shows: *"Your code is backed up. Every change you make will be auto-saved."*

**Step 9: "Where should we deploy?"**
*"Pick where your app goes live. All are free to start. One click to deploy anytime."*

Three cards showing deploy platforms (whichever has the best rev share deal is marked "Recommended"):
- Vercel — "Best for React apps. Instant preview URLs on every change."
- Cloudflare — "Fastest global network. Generous free tier."
- Netlify — "Simple and reliable. Great for getting started."

Each card has a "Connect" button that runs the platform's auth flow. Skip option available.

**Step 10: "Keep your code secure"**
*"We'll scan for vulnerabilities before every deploy. Connect a scanner for continuous monitoring."*

Cards for scanner partners (best rev share deal marked "Recommended"):
- Codacy — "Code quality + security in one. Auto-reviews every change."
- Snyk — "Best at catching vulnerable dependencies."
- SonarCloud — "Deep analysis. Free for open source."

Each has a "Connect" button. Skip option available.

**Step 11: "Track errors in production"**
*"Your app catches errors automatically during development. Want to know about errors after you deploy?"*

Only shown if a deploy platform was selected in Step 9. Cards:
- Sentry — "Industry standard. See every error with full context."
- BetterStack — "Errors + uptime + logs in one dashboard."

Each has a "Connect" button. Skip option available.

**Then: "Create" button.**

The wizard summarizes what's about to happen:
```
Ready to create "todo-app":
  ✓ React + TypeScript + Tailwind + shadcn
  ✓ Supabase (auth + database)
  ✓ Backed up to GitHub (private)
  ✓ Deploys to Vercel
  ✓ Security scans by Codacy
  ✓ Error tracking by Sentry
  → Click Create to start building
```

Total wizard time: ~60-90 seconds (30s creative + 30-60s infrastructure). Every infrastructure step is skippable but explained. Everything after this is automated.

### Phase 1: Automated Scaffold (~20 seconds)

Fast, deterministic, no AI needed. Sets up the full project with everything the wizard configured:

1. Create Vite + React + TypeScript project
2. Install dependencies (pnpm)
3. Add Tailwind CSS v4 + shadcn/ui + lucide-react
4. Apply quality baseline (tsconfig strict, eslint, prettier, vitest)
5. Generate CLAUDE.md, bootstrap skill, hooks, deploy configs
6. Git init + initial commit
7. Push to GitHub (if connected in Step 5)
8. Connect deploy platform (if selected in Step 6)
9. Connect security scanner (if selected in Step 7)
10. Start dev server
11. Detect ready port, navigate browser to it
12. Kaboom auto-tracks the new tab

Progress streams to wizard UI via WebSocket. Each step shows: spinner → checkmark.

### Phase 2: AI Live Composition (the Pencil Experience)

This is the magic. Once the dev server is running and showing a blank app, an AI agent takes over and builds the first real UI — live, in the browser, while the user watches.

The agent uses Kaboom's own tools (`interact`, `observe`, `analyze`) to:

1. **Read the user's description and first feature** from the wizard context
2. **Plan the layout** — header, main content area, feature section, footer
3. **Write components incrementally** — each file write triggers Vite HMR, the browser updates instantly
4. **Verify each step visually** — `observe(what='screenshot')` after each component, check for layout issues
5. **Correct and iterate** — if something looks wrong, fix it and re-verify

**What the user sees:**

```
Phase 1: Setting up project...     ✓ (15s)
Phase 2: Building your app...

  [Browser shows the app updating live]

  ✓ Created layout structure          (header, sidebar, main)
  ✓ Added navigation component        (logo, menu items)
  → Building "drag and drop" feature   (writing TodoList.tsx...)
  ○ Adding sample content
  ○ Final polish
```

The browser is the canvas. Each component write → HMR update → screenshot verify cycle takes 2-5 seconds. A full first screen builds out over 30-60 seconds, with visible progress at every step — exactly like watching Pencil compose a design, but with a real running app.

### How It Maps to Pencil's Mechanisms

| Pencil | Kaboom Phase 2 |
|--------|---------------|
| `batch_design` (25 atomic ops) | Write component file → HMR renders it instantly |
| WASM re-render after each batch | Vite HMR updates browser after each file write |
| `placeholder: true` frames | Skeleton/placeholder components rendered first, then populated |
| `get_screenshot` → verify → correct | `observe(what='screenshot')` → check layout → fix if needed |
| Fine-grained tool streaming | File writes stream to disk, HMR picks up partial saves |
| Multiple small batches = progressive | One component at a time, each immediately visible |

### The Composition Loop

The AI agent runs this loop for each component:

```
1. Write file (e.g., src/components/TodoList.tsx)
   → Vite HMR fires, browser updates
2. Screenshot the page
   → observe(what='screenshot')
3. Verify layout
   → No overlapping elements? Text visible? Responsive?
4. If issues: fix and goto 2
5. If good: next component
```

This is design-time TDD applied to real code. Build, verify, fix — the user sees every iteration.

## Opinionated Stack

We choose for them. The point is speed to editing, not framework selection.

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Framework | React + Vite | Largest ecosystem, best AI training data, fastest HMR |
| Language | TypeScript (strict) | Better AI completions, catches errors early |
| Styling | Tailwind CSS v4 | Utility-first, annotation-mode friendly — class edits = instant visual feedback via HMR |
| Components | shadcn/ui | Copy-paste into user source tree (not node_modules), excellent defaults, annotation mode can locate and edit actual files |
| Package manager | pnpm | Fast, disk-efficient, deterministic lockfile |
| Directory | `~/kaboom-projects/<slug>` | Discoverable, no decision paralysis |

### Why This Stack for Live Composition + Annotation Mode

- **Tailwind** — changing `bg-blue-500` to `bg-red-500` is a single class attribute edit, HMR reflects it instantly. The AI agent can iterate on styling in real-time.
- **shadcn/ui** — components live in `src/components/ui/` (not `node_modules`), so both the AI agent and annotation mode can locate and edit actual source files.
- **Vite** — sub-100ms HMR means each component write is instantly visible. This is what makes the live composition feel real-time.
- **React** — component boundaries map cleanly to DOM elements, making both AI composition and annotation-to-source-file mapping reliable.

## Architecture

```
+----------------------------------+
|  Browser: localhost:7890/launch   |
|  +----------------------------+  |
|  |  Conversational Wizard     |  |
|  |  4 guided steps + Create   |  |
|  +-------------+--------------+  |
+-----------------|----------------+
                  | POST /api/scaffold
                  v
+----------------------------------+     PHASE 1: Automated (~15s)
|  Kaboom Daemon (port 7890)     |
|  +----------------------------+  |
|  |  Scaffold Handler          |  |
|  |  1. pnpm create vite       |  |
|  |  2. Install tailwind/shadcn|  |
|  |  3. Quality baseline       |  |
|  |  4. Generate hooks + skill |  |
|  |  5. pnpm dev               |  |
|  +-------------+--------------+  |
+-----------------|----------------+
                  | interact(navigate)
                  v
+----------------------------------+     PHASE 2: AI Live Composition (~60s)
|  Browser: localhost:5173          |
|  +----------------------------+  |
|  |  AI agent builds UI live:  |  |
|  |  write component → HMR    |  |
|  |  → screenshot → verify    |  |
|  |  → next component         |  |
|  |  (user watches it happen)  |  |
|  +----------------------------+  |
|  Kaboom tracking + annotation  |
+----------------------------------+

WebSocket (port 7891):
+----------------------------------+
| Multiplexed channels:            |
| - scaffold: Phase 1 progress     |
| - compose:  Phase 2 AI progress  |
| - terminal: raw PTY output       |
| - wizard:   user interactions    |
+----------------------------------+

Generated project hooks:
+----------------------------------+
|  .claude/hooks/                  |
|  - session-start.js  (bootstrap) |
|  - context-monitor.js (warnings) |
|  - statusline.js     (dashboard) |
+----------------------------------+
```

## Components to Build

### 1. Wizard Landing Page

**Route:** `GET /launch`
**Served by:** Kaboom daemon HTTP server (port 7890)
**Content:** Static HTML/CSS/JS — minimal, no framework needed for the wizard itself

UI design:
- Conversational flow, one question at a time (not a form)
- Each step slides in after the previous is answered
- Dark theme matching Kaboom popup aesthetic
- Progress bar showing 4 steps
- "Create" button appears after Step 4
- Phase 1 progress replaces wizard questions
- Phase 2 shows split view: browser preview + composition progress log

### 2. Scaffold API Endpoint

**Route:** `POST /api/scaffold`
**Request body:**
```json
{
  "description": "a todo app with drag and drop",
  "audience": "just_me",
  "first_feature": "drag and drop reordering",
  "name": "todo-app"
}
```

**Response:** HTTP 202 Accepted with a channel ID. All progress streams over the WebSocket on port 7891:
```json
{"status": "accepted", "channel": "scaffold-todo-app-1710884400"}
```

### 3. Phase 1: Scaffold Engine

Executes in sequence inside a PTY session. Each step has a verification gate.

| Step | Command | Verification |
|------|---------|-------------|
| 1. Create project | `pnpm create vite <dir> --template react-ts` | Directory exists, `package.json` present |
| 2. Install deps | `pnpm install` | `node_modules/` exists, exit code 0 |
| 3. Add Tailwind | `pnpm add tailwindcss @tailwindcss/vite` | Package in `node_modules/` |
| 4. Add shadcn | `pnpm dlx shadcn@latest init --defaults` | `src/components/ui/` exists |
| 5. Install shadcn components | `pnpm dlx shadcn@latest add button card input label select checkbox textarea separator sheet tabs scroll-area alert badge toast skeleton navigation-menu dropdown-menu avatar` | All component files exist in `src/components/ui/` |
| 5b. Install icons | `pnpm add lucide-react` | Package in `node_modules/` |
| 6. Quality baseline | Write config files, path aliases, theme, minimal App.tsx shell | `tsc --noEmit` passes |
| 7. Generate AI context | Write CLAUDE.md, bootstrap skill, hooks, .mcp.json | Files exist |
| 8. Git init | `git init && git add -A && git commit` | `.git/` exists |
| 9. Start dev server | `pnpm dev` | HTTP 200 on detected port |

If any verification fails, retry once. If still failing, stream error with recovery suggestion.

### 4. Phase 2: AI Composition Engine

After Phase 1 completes and the browser shows the running app, the AI agent takes over. It receives:

**Input context:**
- User's description ("a todo app with drag and drop")
- Audience ("just me")
- First feature ("drag and drop reordering")
- Available shadcn components (from the install)
- Project directory path

**Composition strategy:**

The agent works **outside-in**: layout shell first, then sections, then detail.

1. **Layout structure** — write `src/components/Layout.tsx` with header, main area, optional sidebar. HMR fires, browser shows the skeleton.

2. **Navigation** — write `src/components/Header.tsx` with project name, nav items. Screenshot, verify header renders.

3. **Primary feature** — build the first feature component(s) based on Step 3 of the wizard. For "drag and drop reordering": write `src/components/TodoList.tsx`, `src/components/TodoItem.tsx`. Each file write → HMR → screenshot → verify.

4. **Sample content** — populate with realistic placeholder data so the app looks alive, not empty.

5. **Responsive check** — resize viewport to mobile width (375px) via `interact(what='resize', width=375)` + screenshot. Fix any components that break at mobile. Resize back.

6. **Accessibility check** — run `analyze(what='accessibility')`. Fix WCAG violations (missing alt text, contrast issues, missing labels) before declaring done.

7. **Final commit** — `git add -A && git commit -m "feat: initial UI composition"`

**Composition WebSocket messages:**
```json
{"channel": "compose", "phase": "layout", "status": "writing", "file": "src/components/Layout.tsx"}
{"channel": "compose", "phase": "layout", "status": "hmr_update"}
{"channel": "compose", "phase": "layout", "status": "screenshot", "result": "pass"}
{"channel": "compose", "phase": "navigation", "status": "writing", "file": "src/components/Header.tsx"}
{"channel": "compose", "phase": "feature", "status": "writing", "file": "src/components/TodoList.tsx"}
{"channel": "compose", "phase": "feature", "status": "screenshot", "result": "fix_needed", "issue": "text not visible"}
{"channel": "compose", "phase": "feature", "status": "fixing", "file": "src/components/TodoList.tsx"}
{"channel": "compose", "phase": "complete", "components_created": 5, "duration_s": 45}
```

### 5. Quality Baseline (Phase 1)

Applied during Phase 1, before dev server start.

#### Full Dev Toolchain (all auto-installed in Phase 1)

Every tool below is installed, configured, and wired into git hooks. The user never sets any of this up.

| Tool | Purpose | Auto-fix? | Config File |
|------|---------|-----------|-------------|
| **TypeScript** | Type safety | No (requires judgment) | `tsconfig.json` |
| **ESLint** | Code quality + security | Yes (`--fix`) | `eslint.config.js` |
| **Prettier** | Formatting | Yes (`--write`) | `.prettierrc` |
| **knip** | Dead code removal | Yes (`--fix`) | `knip.config.ts` |
| **Vitest** | Unit tests + coverage | No | `vitest.config.ts` |
| **Husky** | Git hooks manager | — | `.husky/` |
| **eslint-plugin-security** | Security patterns (eval, innerHTML) | Partial | (in eslint config) |
| **eslint-plugin-react-hooks** | React rules of hooks | Yes | (in eslint config) |

**tsconfig.json** — strict mode:
- `strict: true`
- `noUncheckedIndexedAccess: true`
- `noUnusedLocals: true`
- `noUnusedParameters: true`
- Path aliases: `"@/*": ["./src/*"]` (prevents `../../../../` import hell)

**vite.config.ts** — matching path alias via `resolve.alias`

**ESLint** (`eslint.config.js`, flat config):
- `@eslint/js` recommended
- `typescript-eslint` strict
- `eslint-plugin-react-hooks` recommended
- `eslint-plugin-security` recommended
- Auto-fixable rules run on commit, remaining warnings block

**Prettier** (`.prettierrc`):
- Single quotes, no semicolons, 2-space indent, trailing commas
- Runs as `prettier --write` on commit (auto-fixes, never blocks)

**knip** (`knip.config.ts`):
- Detects unused exports, files, and dependencies
- Runs as `knip --fix` on commit (auto-removes, re-stages)
- shadcn `components/ui/` ignored (may look unused before being composed)

**Husky** (`.husky/`):
- `pre-commit`: prettier → eslint --fix → knip --fix → re-stage → tsc → eslint --max-warnings 0 → component invariants
- `pre-push`: vitest with 60% coverage floor
- `pre-deploy` (custom): pnpm audit + security scan

The key principle: **auto-fix everything that can be auto-fixed, block only on things requiring human judgment.** Users never see formatting diffs, dead export warnings, or simple lint issues — they're fixed silently on commit.

#### Curated Component Library

shadcn/ui components pre-installed to cover 90% of first-screen needs. The AI agent in Phase 2 composes from these instead of writing raw divs.

| Category | Components |
|----------|-----------|
| Layout | `card`, `separator`, `sheet`, `tabs`, `scroll-area` |
| Forms | `input`, `button`, `label`, `select`, `checkbox`, `textarea` |
| Feedback | `alert`, `badge`, `toast`, `skeleton` |
| Navigation | `navigation-menu`, `dropdown-menu`, `avatar` |

**Icons:** `lucide-react` pre-installed (the icon library shadcn uses). Bootstrap skill enforces: use Lucide, don't inline SVGs, don't add another icon library.

#### Tailwind Theme with Design Tokens

The scaffold generates a Tailwind theme using CSS variables for brand colors, not hardcoded hex values:

```css
/* src/index.css */
@theme {
  --color-primary: #2563eb;
  --color-secondary: #64748b;
  --color-accent: #f59e0b;
  --color-background: #ffffff;
  --color-foreground: #0f172a;
}
```

Bootstrap skill enforces: use `bg-primary`, `text-foreground`, etc. — never `bg-[#2563eb]` or inline hex colors.

#### Testing

**Vitest** pre-configured with:
- `vitest.config.ts` wired with `happy-dom`, `@/` path aliases, coverage reporter
- `pnpm test` runs all tests
- One example test generated to prove the setup works
- Phase 2 AI agent writes one test per composed component

#### Minimal App.tsx Shell

Just enough for the dev server to show something. Phase 2 replaces this with real content.

#### Git

`git init` + `.gitignore` + initial commit ("scaffold: <project name>")

### 6. AI Context Generation (Bootstrap Skill)

Generated into the project so any AI agent immediately understands it.

**`.claude/CLAUDE.md`** — project context for Claude Code:
- Project name and description (from wizard Step 1)
- Audience (from Step 2)
- First feature / current focus (from Step 3)
- Stack summary (React 19, Vite, TypeScript strict, Tailwind v4, shadcn/ui)
- Key directories and what lives where
- Dev commands (`pnpm dev`, `pnpm build`, `pnpm lint`, `pnpm tsc`)
- Conventions: component naming, file organization, Tailwind-first styling

**`.claude/skills/kaboom-dev/SKILL.md`** — bootstrap skill (Superpowers pattern):
- Teaches the agent how to use annotation mode for visual editing
- References the kaboom MCP tools for browser interaction
- Anti-rationalization table for common AI shortcuts

**Bootstrap skill enforced rules (anti-slop invariants):**

| Rule | Enforcement | Rationalization it blocks |
|------|-------------|--------------------------|
| **Tailwind-first** | No inline `style={}`. No CSS modules. Use Tailwind classes. | "I'll just add a quick inline style for this one thing" |
| **shadcn-first** | If shadcn has a component, use it. Don't create custom equivalents. | "I'll build a quick custom button, it's simpler" |
| **Theme tokens only** | Use `bg-primary`, `text-foreground`. Never `bg-[#2563eb]` or hardcoded hex. | "I'll just use the hex value directly, it's faster" |
| **Lucide icons only** | Use `lucide-react`. No inline SVGs. No other icon libraries. | "I'll paste this SVG inline, it's just one icon" |
| **200 LOC max per file** | Extract when a component exceeds 200 lines. | "I'll keep it all in one file for now, it's easier to follow" |
| **No barrel exports** | Import directly from component files. No `index.ts` re-exports. | "I'll add an index.ts for cleaner imports" |
| **`@/` imports only** | Use `@/components/...`, never `../../../`. | "Relative path is fine for now" |
| **One test per component** | Every new `.tsx` gets a `.test.tsx`. No exceptions. | "I'll add tests later" |
| **No dead code** | If knip reports a dead export, remove it immediately. Don't leave unused code. | "I might need it later" |
| **Responsive from birth** | Every component must not break at 375px width. | "I'll handle mobile later" |
| **Accessible by default** | Form inputs need labels. Images need alt text. Interactive elements need focus states. | "Accessibility can come later" |

**`.mcp.json`** — pre-wires Kaboom as an MCP server so tools are available immediately.

### 7. Dev Server with Pretty URLs (Portless)

The scaffold uses **portless** (Apache-2.0, 1 dependency, 280KB) to give every project a named local URL instead of `localhost:5173`.

**What the user sees:** `http://todo-app.kaboom` instead of `http://localhost:5173`

The project name from the wizard becomes the subdomain with a `.kaboom` TLD. Multiple projects run simultaneously: `todo-app.kaboom`, `dashboard.kaboom`, `landing-page.kaboom`.

**Phase 1 installs portless globally** (if not already installed):
```bash
pnpm add -g portless
```

**One-time `.kaboom` domain setup** (during first scaffold):

The wizard tries the premium path first, explains what it needs, and falls back gracefully:

```
Step: Set up your dev domain

We'd like to give your app a clean URL: http://todo-app.kaboom

This requires a one-time system change to route .kaboom
domains to your machine. Your password is needed for this.

  [Set up .kaboom domains]     [Skip, use default]
```

If they click "Set up":
```bash
sudo portless proxy start --tld kaboom
```
→ All future projects get `http://project-name.kaboom`

If they click "Skip" or sudo fails:
→ Falls back to `http://todo-app.localhost:1355` (works everywhere, no permissions needed)

The wizard stores which path succeeded in `~/.kaboom/config.json` so it never asks again.

**Dev server starts via portless (both paths):**

```bash
# If .kaboom is set up:
portless todo-app pnpm dev
# → http://todo-app.kaboom

# If .kaboom failed/skipped:
portless todo-app pnpm dev
# → http://todo-app.localhost:1355
```

Same command either way — portless handles the routing based on whether the proxy is running.

```json
{
  "scripts": {
    "dev": "portless todo-app vite",
    "dev:raw": "vite"
  }
}
```

**Detection:** After `portless` starts:
- Parse stdout for the ready URL (either `.kaboom` or `.localhost:1355`)
- Fallback: poll both URLs
- Timeout after 30 seconds with error

**Benefits:**
- **Best case:** `http://todo-app.kaboom` — clean, branded, no port numbers
- **Fallback:** `http://todo-app.localhost:1355` — still named, still works, zero permissions
- **Either way:** better than `localhost:5173`
- **Multi-project** — multiple apps run simultaneously on different names
- **Stable** — URL doesn't change between restarts
- **One command** — `portless todo-app pnpm dev` works regardless of which path was set up

### 8. Goal-Backward Verification

After Phase 2 completes, run a final verification pass. Don't check "did commands succeed" — check "can the user achieve the goal?"

**The goal:** User sees a meaningful first screen and can click an element to edit it.

**Verification chain (work backward from goal):**

1. **Functional:** Edit a component (change a Tailwind class), verify HMR updates the browser within 2 seconds
2. **Wired:** Components import each other correctly, Tailwind classes resolve, shadcn components render
3. **Substantive:** The UI reflects the user's description — not Vite boilerplate, not empty shells. Real layout, real content.
4. **Exists:** All expected files present (Layout, Header, feature components, CLAUDE.md, bootstrap skill)

If any level fails, the AI agent fixes it (same compose loop) before declaring completion.

### 9. WebSocket Architecture

The wizard connects via WebSocket on **port 7891** (existing terminal server), multiplexed with channel tags:

**Channels:**
- `scaffold` — Phase 1 step progress (step status, verification results)
- `compose` — Phase 2 AI composition progress (file writes, HMR, screenshots, fixes)
- `terminal` — raw PTY output (expandable for power users)
- `wizard` — user interactions (wizard answers, create action)

**Future consolidation:** port 7891 can be merged onto 7890 by exempting WebSocket connections from the main server's timeouts. Not needed for v1.

## Hooks (Generated Into Project)

The scaffold generates Claude Code hooks into the project's `.claude/` directory. These provide runtime intelligence during development sessions.

### Hook 1: Session Start — Bootstrap Context Injection

**Event:** `SessionStart` (fires on startup, `/clear`, context compaction)
**Pattern:** Superpowers' single-hook bootstrap

Injects the bootstrap skill as `additionalContext` so the AI agent always knows project conventions, even after context resets.

### Hook 2: Context Monitor — Warn Before Quality Degrades

**Event:** `PostToolUse` (fires after every tool call)
**Pattern:** GSD's two-stage bridge architecture

The status line hook writes context metrics to a bridge file. After every tool use, the context monitor reads it:
- **35% remaining** → warn: "Wrap up, commit, consider fresh session."
- **25% remaining** → critical: "Stop, save your work."
- Debounce: 5 tool uses between warnings (unless severity escalates)

### Hook 3: Status Line — Live Project Dashboard

**Event:** `statusLine` (continuous)

```
tracking: localhost:5173 | 2 errors | HMR: ok | context: [████████░░] 65%
```

Writes the context metrics bridge file that Hook 2 reads.

### What We Don't Generate

- **No PreToolUse guard** — don't nag users in a fresh project.
- **No update checker** — scaffold is one-time, not a versioned dependency.

## Prerequisites

- `node` >= 18 on PATH
- `pnpm` on PATH (wizard offers to install if missing: `npm install -g pnpm`)
- Kaboom daemon running (serves the wizard — always true if user opened `/launch`)
- Chrome with Kaboom extension installed (for annotation mode; Phase 1 works without it)

The wizard checks these before showing "Create" and shows clear, actionable errors.

## Quality Gates & Invariants

The scaffold ships with automated quality enforcement so the codebase stays clean from day one. No setup required — it's all in place before the user writes their first line.

### Pre-commit Hook (catches slop before it lands)

Generated as `.husky/pre-commit` (or simple git hook):

```bash
pnpm prettier --write .     # Auto-fix formatting
pnpm eslint --fix            # Auto-fix lint issues
pnpm lint:dead               # Auto-remove dead exports/files/deps (knip --fix)
git add -u                   # Re-stage auto-fixes
pnpm tsc --noEmit            # Type errors block commit (no auto-fix — requires human judgment)
pnpm eslint --max-warnings 0 # Remaining lint warnings block commit
```

The hook auto-fixes what it can (formatting, simple lint issues, dead code), re-stages the fixes, then blocks on things that need human judgment (type errors, complex lint issues).

**Dead code detection via knip** — `knip` (ISC license, open source) is installed as a dev dependency and configured in `knip.config.ts`:

```typescript
// knip.config.ts
export default {
  entry: ['src/main.tsx'],
  project: ['src/**/*.{ts,tsx}'],
  ignore: ['src/components/ui/**'],  // shadcn components may look unused before they're composed into features
  ignoreDependencies: ['@tailwindcss/vite'],  // Vite plugin referenced in config, not imported
}
```

`pnpm lint:dead` runs `knip --fix --include exports,files,dependencies` and **auto-removes** dead code on commit. No manual cleanup needed — the hook fixes it, re-stages the changes, and the commit goes through clean. This prevents the codebase from accumulating unused exports, orphaned files, or phantom dependencies over time.

For reporting without auto-fix: `pnpm lint:dead:check` runs `knip --include exports,files,dependencies` (no `--fix`).

Phase 1 installs it: `pnpm add -D knip` alongside the other dev dependencies.

**Component invariant check** — a lightweight script (`scripts/check-components.sh`) that verifies:
- Every `.tsx` in `src/components/` has a default export
- No inline `style={}` attributes (Tailwind-first enforcement)
- No CSS module imports (use Tailwind, not `.module.css`)
- No `any` types
- No hardcoded hex colors in className strings (use theme tokens)
- No `../` imports (use `@/` path aliases)
- No files over 200 LOC

This is ~30 lines of grep, not a framework. Runs in <1 second.

### Vitest with Coverage Floor (catches regressions)

Pre-configured in Phase 1. Phase 2's AI agent writes one test per composed component.

- `vitest` runs on `pnpm test`
- Coverage threshold: **60% lines** (low bar, but enforced — prevents "zero tests forever")
- Pre-push hook runs `pnpm test` — broken tests block push
- Bootstrap skill instructs the AI to write tests for every new component
- `vitest.config.ts` pre-wired with happy-dom, path aliases, coverage reporter

### Pre-Deploy Security Scan (catches vulnerabilities before they ship)

Before any deploy, a security scan runs automatically. This is the last gate — nothing ships with known vulnerabilities.

**Frontend scan** (runs locally, no account needed):
- `pnpm audit` — checks dependencies for known CVEs, blocks on high/critical
- ESLint security rules (`eslint-plugin-security`) — catches common patterns: `eval()`, `innerHTML`, unsanitized URLs, prototype pollution
- Kaboom's own `analyze(what='security_audit')` — scans the running app for XSS vectors, insecure headers, mixed content, open redirects

**Backend scan** (when applicable):
- `pnpm audit` covers server-side deps too
- `.env` leak check — verifies no secrets in committed files (grep for API keys, tokens, passwords in source)
- CSP header generation — `generate(what='csp')` creates a Content Security Policy from observed traffic

**Cloud security scanner** (recommended partner — revenue share opportunity):

The scaffold recommends connecting a cloud scanner for ongoing monitoring. Pre-configured integration with partner TBD:

| Scanner | Fit | Partner Program |
|---------|-----|----------------|
| **Codacy** | Code quality + security in one. Auto-reviews PRs. | Revenue share via referral program |
| **Snyk** | Best dependency vulnerability scanning. Free tier generous. | Partner program for developer tools |
| **SonarCloud** | Deep static analysis, security hotspots. Free for open source. | Technology partner program |
| **Socket.dev** | Supply chain security — detects malicious packages. | Startup program |

The wizard's "Ready" screen and the deploy output both suggest connecting a scanner:

```
Security scan passed. 0 vulnerabilities found.

Tip: Connect a cloud scanner for continuous monitoring:
  → pnpm security:setup    (guided setup for Codacy/Snyk/SonarCloud)
```

`pnpm security:setup` is an interactive script that walks the user through connecting their preferred scanner, creates the config file, and (if they have a GitHub repo) sets up the GitHub App integration.

**Revenue share strategy:** Same model as deploy platforms — every Kaboom project recommends a scanner, negotiate referral terms with all four. Codacy and Snyk are the strongest candidates (both have active partner programs for dev tools).

### Auto-Commit Hook (keeps work saved)

The scaffold configures automatic commits to prevent lost work. A PostToolUse hook (or file watcher) detects meaningful changes and prompts:

**How it works:**
- After every 5 file saves (or 5 minutes of edits), the hook checks `git diff --stat`
- If there are uncommitted changes, it auto-commits with a descriptive message: `auto: update TodoList component and add drag handlers`
- Commits go to a local branch — never auto-pushed
- The user always sees: `[auto-saved 2 minutes ago]` in the status line

This ensures the user never loses more than a few minutes of work, even if they never think about git.

### GitHub Onboarding

The scaffold guides users toward GitHub for backup and collaboration.

**During Phase 1** (if `git` is on PATH):
- `git init` runs automatically (already in the scaffold)
- If `gh` CLI is installed, check if authenticated: `gh auth status`

**In the "Ready" screen** (after Phase 2):

```
Your app is running! Next steps:
  → Edit with annotation mode (click any element)
  → Deploy: pnpm deploy
  → Back up to GitHub: pnpm github:setup    (free, keeps your code safe)
```

**`pnpm github:setup`** is an interactive script:
1. Check if `gh` CLI is installed → if not, offer to install: `brew install gh` / link to download
2. Check if authenticated → if not, run `gh auth login`
3. Create a private repo: `gh repo create <project-name> --private --source=. --push`
4. Set up branch protection on `main` (require PR, require status checks)
5. If a deploy platform is connected, enable preview deploys on PR

**Revenue share:** GitHub doesn't have a direct referral program, but driving users to GitHub → GitHub Actions → GitHub Copilot is a natural funnel. Worth a conversation about co-marketing.

### The Invariant Chain

```
Edit code → auto-commit (every 5 saves or 5 minutes)
         → pre-commit (types + lint + format + dead code + component invariants)
         → push (tests + 60% coverage floor)
         → pre-deploy (security scan: audit + ESLint security + .env leak check)
         → deploy (one command)
         → cloud scanner (continuous monitoring via Codacy/Snyk/SonarCloud)
```

Every step is automated, zero-config, and blocks on failure. The user never sets up CI — it's in the git hooks from minute one.

## One-Command Deploy

The scaffold includes deploy configs for three platforms. Default TBD based on partnership terms (revenue share negotiations in progress with all three).

### Supported Platforms

**Vercel** (default candidate):
- `vercel.json` pre-configured for Vite+React (zero-config)
- `pnpm deploy:vercel` → `vercel --prod`
- Preview deploys on every push via GitHub integration
- Partner program offers revenue share for platforms driving signups

**Cloudflare Pages:**
- `wrangler.toml` pre-configured
- `pnpm deploy:cloudflare` → `wrangler pages deploy dist`
- Free tier is generous (unlimited bandwidth)
- Workers Launchpad program for startups

**Netlify:**
- `netlify.toml` pre-configured
- `pnpm deploy:netlify` → `netlify deploy --prod`
- Technology partner program with revenue share
- Hungry for market share, may offer best terms

### Deploy Setup

All three configs are generated. The wizard's "Ready" screen offers:

```
Your app is running! Next steps:
  → Edit with annotation mode (click any element)
  → Deploy: pnpm deploy        (first time sets up hosting)
```

`pnpm deploy` is an alias that auto-detects which platform is configured. First run is interactive (links project, creates account if needed). After that, it's one command.

### Revenue Share Strategy

Every Kaboom-scaffolded project recommends a deploy platform, security scanner, and GitHub. At scale, this is significant referral volume across three revenue streams.

#### Partner Revenue Breakdown

| Provider | Rev Share? | Model | Priority |
|----------|-----------|-------|----------|
| **Vercel** | Yes | % of referred paid plans via partner program | High — best React/Vite fit, active program |
| **Netlify** | Yes | Referral fees via technology partner program | High — hungry for share, may offer best terms |
| **Cloudflare** | Indirect | Workers Launchpad credits, co-marketing | Medium — generous free tier limits conversion |
| **Codacy** | Yes | % of first year via referral program | High — code quality + security in one, natural fit |
| **Snyk** | Yes | Referral fees on Team/Enterprise plans | High — best dependency scanning, strong partner program |
| **SonarCloud** | Likely | Partner program exists, terms negotiable | Medium — strong product, less established referral |
| **Socket.dev** | Unknown | Startup, likely negotiable | Low — niche (supply chain), small install base |
| **GitHub** | No | Co-marketing only, no per-user payments | Include anyway — essential UX, indirect funnel to Actions/Copilot |
| **GitLab** | Yes | Partner program with referral commissions | Backup — if GitHub co-marketing doesn't materialize |

#### Revenue Streams Per Scaffolded Project

```
Deploy platform:    $X/month per converted user (Vercel/Netlify)
Security scanner:   $Y/year per converted user (Codacy/Snyk)
Source hosting:     $0 (GitHub) — but co-marketing value + user stickiness
```

At 1,000 scaffolded projects/month, even 5% conversion to paid tiers generates meaningful recurring revenue across all three streams.

#### Negotiation Approach

1. Reach out to Vercel, Netlify, Codacy, Snyk simultaneously — leverage competition
2. Default to whichever gives best terms in each category (deploy, scanner)
3. Make all options available — user picks, but "Recommended" badge drives defaults
4. Track referral attribution via UTM params or partner-specific install flags
5. GitHub: pursue co-marketing partnership even without rev share — it's table stakes for the UX

## Production Observability (Post-Scaffold)

Kaboom captures everything during development — but what about after deploy? The scaffold pre-wires production observability so the user's app is never a black box.

### Error Tracking

The scaffold pre-installs a lightweight error boundary and recommends connecting an error tracking service:

**Built-in (no account needed):**
- React error boundary in `src/components/ErrorBoundary.tsx` — catches render crashes, shows friendly fallback
- `window.onerror` + `window.onunhandledrejection` handlers in `src/lib/error-reporter.ts`
- During dev: errors flow to Kaboom automatically (already works)
- During production: errors log to console (baseline) or forward to a service (if connected)

**Recommended service (wizard Step 7b — alongside security scanner):**

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **Sentry** | Industry standard error tracking. Generous free tier (5K events/month). | Yes — partner program, referral fees |
| **BetterStack** | Error tracking + uptime + logging in one. Clean UI. | Yes — partner program |
| **LogRocket** | Session replay + error tracking. See what user did before crash. | Yes — referral program |

The wizard offers this as part of the infrastructure setup:

*"We'll catch errors automatically during development. Want to track errors in production too? (Free to start)"*

One-click connect, pre-wired in the scaffold. The error reporter checks for a `VITE_SENTRY_DSN` (or equivalent) env var — if set, errors forward to the service; if not, console-only.

### Analytics

For "My team" and "Public users" audiences (wizard Step 2), the scaffold offers basic analytics:

**Recommended service:**

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **PostHog** | Product analytics + session replay. Open source option. Generous free tier. | Yes — partner program |
| **Plausible** | Privacy-first, lightweight. Good for simple sites. | Affiliate program |
| **Umami** | Self-hosted option. Free forever. | No (open source) |

Pre-wired as `src/lib/analytics.ts` — a thin wrapper that no-ops if no service is configured. Never blocks rendering. GDPR-friendly defaults (no cookies until consent).

### Uptime Monitoring

For deployed apps, the wizard recommends uptime monitoring:

*"Want to know if your app goes down? We'll ping it every minute. (Free to start)"*

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **BetterStack** | Uptime + incident pages + alerting. Beautiful status pages. | Yes — partner program |
| **UptimeRobot** | Simple, free for 50 monitors. | Affiliate program |

One-click setup creates a monitor for the deploy URL.

### Revenue Opportunity

Every scaffolded project that connects to production services = recurring referral revenue across up to 6 partners:

```
Deploy:      Vercel / Netlify / Cloudflare     (1 per project)
Security:    Codacy / Snyk / SonarCloud        (1 per project)
Errors:      Sentry / BetterStack / LogRocket  (1 per project)
Analytics:   PostHog / Plausible               (1 per project, team/public only)
Uptime:      BetterStack / UptimeRobot         (1 per project)
Source:      GitHub                            (co-marketing)
```

At scale, a single scaffolded project can generate referral revenue from 4-5 services simultaneously.

## Learn While You Build

The wizard and AI composition phases are teaching moments. Users are watching their app get built — that's when they're most receptive to learning *why* things are done a certain way.

### Contextual Lessons During Phase 2

As the AI agent composes each component, the wizard UI shows a brief lesson panel alongside the progress:

```
┌─ Building your app ──────────────────────────────────────┐
│                                                          │
│  ✓ Created Layout.tsx                                    │
│  → Building Header.tsx                                   │
│                                                          │
│  ┌─ Learn ────────────────────────────────────────────┐  │
│  │ 💡 Components                                      │  │
│  │                                                    │  │
│  │ The AI just created a "Header" component — a       │  │
│  │ reusable piece of UI. In React, you build apps     │  │
│  │ by composing small components together, like       │  │
│  │ LEGO bricks. Each component is one file.           │  │
│  │                                                    │  │
│  │ Your Header component is at:                       │  │
│  │ src/components/Header.tsx                          │  │
│  │                                                    │  │
│  │ Try clicking it in the browser after we're done    │  │
│  │ — annotation mode will show you the code.          │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### Lesson Catalog

Each lesson is triggered by what the AI agent is doing at that moment:

| AI Action | Lesson Topic | Key Concept |
|-----------|-------------|-------------|
| Writing Layout.tsx | **Components** | Apps are built from small reusable pieces |
| Adding Tailwind classes | **Styling with Tailwind** | Utility classes instead of writing CSS files |
| Importing shadcn Button | **Component libraries** | Don't build everything from scratch |
| Adding `useState` | **State** | How components remember things |
| Writing a `.test.tsx` | **Testing** | How to know your code works |
| Checking mobile viewport | **Responsive design** | Making apps work on any screen size |
| Running accessibility check | **Accessibility** | Building apps everyone can use |
| Using `@/` imports | **Project structure** | How files are organized and why |
| Setting up error boundary | **Error handling** | What happens when things go wrong |
| Adding environment variables | **Configuration** | Keeping secrets out of code |

### Lesson Design Principles

- **3 sentences max.** If it takes longer to explain, link to a full guide.
- **Show the file path.** Always point to where the concept lives in their code.
- **End with an action.** "Try clicking it", "Change the color and watch it update", "Run `pnpm test` to see it pass."
- **Never block progress.** Lessons appear alongside composition, never pause it.
- **Dismissable.** "Got it" button collapses the lesson. "Don't show tips" disables all.
- **Progressive.** First project gets all lessons. If we detect the user has built before (cookie/localStorage), show fewer.

### Post-Composition Learning Path

After Phase 2 completes, the "Ready" screen includes a learning section:

```
Your app is running! Here's what was built:

📁 Project structure
  src/components/  — your UI components (click any to edit)
  src/lib/         — shared utilities
  src/App.tsx      — the main entry point

🎯 Try these first:
  1. Click any element in the browser → see the code that makes it
  2. Change a color class (e.g., bg-blue-500 → bg-red-500) → watch it update instantly
  3. Run "pnpm test" in terminal → see your tests pass
  4. Run "pnpm deploy" → put it on the internet

📚 Want to learn more?
  → React basics (link)
  → Tailwind CSS guide (link)
  → How components work (link)
```

### Annotation Mode as a Teaching Tool

Annotation mode is inherently educational — the user clicks a UI element and sees the source code that produces it. The scaffold's bootstrap skill enhances this:

- When the user clicks an element, the AI agent explains what the code does (if they ask)
- The agent references the relevant lesson: "This uses `useState` — remember the State lesson from when we built it?"
- Component names in the source match what was shown during composition, creating continuity

This turns every editing session into an implicit tutorial. The user learns React/Tailwind/TypeScript by doing, not by reading docs.

## Terminal-First Architecture: Decorated Commands

The terminal (xterm on port 7891) is the **single source of truth**. The user always has complete control. Everything flows through it — whether the user types directly or clicks a button in the Kaboom UI.

### How It Works

The Kaboom browser UI provides contextual action buttons. When clicked, they compose a **decorated prompt** — a well-crafted command with full project context — and inject it into the xterm PTY. The user sees it appear in the terminal as if they typed it. Claude Code receives and executes it.

```
┌─ Kaboom UI (browser) ────────────────────────────────┐
│                                                      │
│  Your app: todo-app    [2 errors]  [HMR: ok]        │
│                                                      │
│  Quick actions:                                      │
│  [Add a page]  [Add a feature]  [Fix errors]        │
│  [Deploy]  [Run tests]  [Security scan]             │
│                                                      │
├─ Terminal ──────────────────────────────────────────┤
│  $ Add a new page called "contacts" to the app.     │  ← injected by button click
│  Use the existing Layout component and shadcn        │
│  form components. Store contact data in Supabase     │
│  with RLS policies. Write a test.                    │
│                                                      │
│  Claude: I'll create the contacts page...            │  ← Claude Code executing
│  Writing src/pages/ContactsPage.tsx...               │
│  Writing src/hooks/useContacts.ts...                 │
│                                                      │
└──────────────────────────────────────────────────────┘
         ↕ same WebSocket, same PTY session
```

The user can:
- Click buttons → decorated prompt injected into terminal
- Type directly in terminal → works exactly the same (skills + hooks provide context)
- Edit a decorated prompt before hitting enter → full control
- Cancel mid-execution → Ctrl+C works as expected
- Mix button clicks and typing freely

### Prompt Decoration

When the UI composes a command, it decorates it with project-aware context the user wouldn't think to include:

**Raw user intent:** "add a contact form"

**Decorated prompt (what gets injected into terminal):**
```
Add a contact form page to the app. Requirements:
- Use the existing Layout component for page structure
- Use shadcn form components (input, label, button, textarea)
- Style with Tailwind theme tokens (bg-primary, text-foreground)
- Store submissions in Supabase with RLS policies for the authenticated user
- Add form validation with descriptive error messages
- Include a success toast using shadcn toast component
- Write a test for the form submission flow
- Ensure the page works at 375px mobile viewport
```

The decoration pulls from:
- **Code quality standards** — `.kaboom/code-quality-standards.md` is appended to every decorated prompt. Contains all file size limits, naming conventions, styling rules, testing requirements, and "what NOT to do" rules. This is the non-negotiable baseline.
- **Project config** — stack, database, auth setup (from CLAUDE.md)
- **Available components** — what shadcn components are installed
- **Current page state** — what's on screen (from `observe(what='page')`)
- **Active errors** — if there are console errors, mention them
- **File size check** — if any file the AI will touch is over 300 lines, the decoration adds: "WARNING: {file} is {n} lines. Split it before adding more code."
- **Bootstrap skill rules** — Tailwind-first, shadcn-first, test per component, etc.

The code quality standards file (`code-quality-standards.md`) ships with every scaffold and is the source of truth for all quality rules. It covers: file size limits (300 LOC), one component per file, naming conventions, import rules (`@/` only, no barrels), Tailwind-only styling, theme tokens, shadcn-first, component patterns (no `any`, no `as`, destructured props), Supabase data patterns (RLS, hooks), error handling (boundaries + toast), testing (behavior-focused, no snapshots), accessibility (labels, alt text, focus), git discipline, and an explicit "what NOT to do" section.

### Action Buttons

The Kaboom UI shows contextual buttons based on project state:

| Button | When Shown | Decorated Prompt Includes |
|--------|-----------|--------------------------|
| **Add a page** | Always | Layout component, routing, shadcn, Supabase, test, mobile check |
| **Add a feature** | Always | Available components, database schema, auth context |
| **Fix errors** | When `observe(errors)` > 0 | Actual error messages, affected components, stack traces |
| **Deploy** | Deploy platform connected | Security scan first, then deploy command |
| **Run tests** | Always | `pnpm test` with coverage report |
| **Security scan** | Always | `pnpm audit` + Kaboom `analyze(what='security_audit')` |
| **Improve this page** | Tab tracked | Screenshot of current page, accessibility audit results |
| **Make it mobile-friendly** | Tab tracked | Current viewport screenshot + 375px screenshot comparison |

### Skills and Hooks (Shipped with Kaboom Install)

When Kaboom installs, it ships with skills and hooks that make this work even without the Kaboom UI — so users typing directly in Claude Code get the same quality:

**Shipped skills** (in `claude_skill/kaboom/`):
- Already have the 5-tool reference (observe, analyze, generate, configure, interact)
- Workflow guides already teach best practices for debugging, automation, testing

**Scaffolded project skills** (generated into `.claude/skills/kaboom-dev/`):
- Bootstrap skill with anti-rationalization table
- Project-specific conventions (stack, components, database, auth)
- These survive `/clear` via the SessionStart hook

**Scaffolded project hooks:**
- **SessionStart** — injects bootstrap skill (project context always loaded)
- **PostToolUse** — context monitor (warns before quality degrades)
- **statusLine** — live dashboard (tracking URL, errors, HMR, context usage)

The combination means: button clicks get decorated prompts → great results. Direct typing gets skill context + MCP tools → also great results. The terminal is always the interface. The UI just makes it easier to compose good prompts.

### Why Terminal-First Matters

- **Transparency** — the user sees exactly what's being sent to the AI. No magic, no hidden prompts.
- **Control** — the user can edit, cancel, or override anything before it executes.
- **Learning** — users see well-crafted prompts and learn how to write better ones themselves.
- **Fallback** — if the UI breaks, the terminal still works. Skills and hooks don't depend on the UI.
- **Universal** — works with any AI agent in the terminal (Claude Code, Codex, Gemini CLI), not just our UI.

## Context Menu: Right-Click Actions on Kaboom Projects

When the user is on a Kaboom project (localhost with `<meta name="kaboom-project">` tag), the extension dynamically adds context menu items. These only appear on Kaboom projects — not on any other site.

### Detection

During scaffold, the Vite plugin injects a meta tag into `index.html`:

```html
<meta name="kaboom-project" content="todo-app" data-stack="react,supabase,stripe">
```

The content script checks for this on every page load:

```typescript
const kaboomMeta = document.querySelector('meta[name="kaboom-project"]')
if (kaboomMeta) {
  chrome.runtime.sendMessage({ type: 'kaboom_project_detected',
    name: kaboomMeta.content,
    stack: kaboomMeta.dataset.stack
  })
}
```

The background script creates/removes context menu items based on this signal.

### Context Menu Items

When on a Kaboom project, right-click shows:

```
┌─────────────────────────────┐
│  Kaboom                    → │
│  ├── Add feature            │
│  ├── Add page               │
│  ├── Update this element    │
│  ├── Fix errors             │
│  ├── Improve this page      │
│  └── Deploy                 │
└─────────────────────────────┘
```

When NOT on a Kaboom project (or not on localhost), the "Kaboom" submenu doesn't appear at all.

### Inline Chat Box

When the user clicks a context menu item, a small chat box appears **at the click position** as a content script overlay:

```
┌─────────────────────────────────────────┐
│  Add feature                        [×] │
│  ┌───────────────────────────────────┐  │
│  │ What feature do you want to add? │  │
│  │                                   │  │
│  │ a contact form with email         │  │
│  │ validation                        │  │
│  └───────────────────────────────────┘  │
│                              [Send →]   │
└─────────────────────────────────────────┘
```

Design:
- Appears at right-click coordinates (anchored to where they clicked)
- Floating overlay with shadow, dark theme matching extension popup
- Auto-focused text input — user can type immediately
- Enter key or "Send" button submits
- Escape or [×] dismisses
- Small, non-intrusive — roughly 350px wide

### Click-to-Element Context

For "Update this element" specifically, the element the user right-clicked on provides additional context:

1. The content script captures the right-clicked element's:
   - CSS selector path
   - Component name (from React devtools data attributes or `data-component` if present)
   - Current text content
   - Current Tailwind classes
   - Source file path (from source map or annotation mode mapping)

2. This context is included in the decoration:

**User right-clicks a header and selects "Update this element", types:** "make it bigger and add a subtitle"

**Decorated prompt sent to terminal:**
```
Update the element at selector "h1.text-2xl" in src/components/Header.tsx.
Currently: <h1 class="text-2xl font-bold text-foreground">Todo App</h1>
User request: make it bigger and add a subtitle.
Use Tailwind classes for sizing. Keep the theme tokens.
```

### Flow: Right-Click → Chat → Terminal → HMR

```
1. User right-clicks on their Kaboom app
2. Selects "Add feature" from context menu
3. Chat box appears at click position
4. User types "dark mode toggle" and hits Enter
5. Chat box closes
6. Decorated prompt is composed:
   "Add a dark mode toggle to the app. Use Tailwind's dark: variant.
    Add a toggle button in the Header using shadcn Switch component.
    Persist preference in localStorage. Apply dark class to html element.
    Write a test for the toggle behavior."
7. Prompt is injected into the xterm PTY on port 7891
8. Claude Code executes in the terminal (user can watch or switch to browser)
9. Files are written → Vite HMR fires → browser updates live
10. User sees dark mode toggle appear on the page
```

### Context Menu Lifecycle

```typescript
// background script
chrome.runtime.onMessage.addListener((msg) => {
  if (msg.type === 'kaboom_project_detected') {
    chrome.contextMenus.create({
      id: 'kaboom-menu',
      title: 'Kaboom',
      contexts: ['page', 'selection', 'link', 'image'],
      documentUrlPatterns: ['http://localhost/*']
    })
    chrome.contextMenus.create({
      id: 'kaboom-add-feature',
      parentId: 'kaboom-menu',
      title: 'Add feature',
      contexts: ['page']
    })
    chrome.contextMenus.create({
      id: 'kaboom-update-element',
      parentId: 'kaboom-menu',
      title: 'Update this element',
      contexts: ['page', 'selection', 'image']
    })
    // ... other items
  }

  if (msg.type === 'kaboom_project_left') {
    chrome.contextMenus.removeAll()
  }
})
```

Only active on `http://localhost/*` pages that have the `kaboom-project` meta tag. Completely invisible on all other sites.

## Zero-Token Status Overlays

The extension already has subtitle and toast infrastructure (content script overlays). We use these to narrate what Kaboom is doing — without any LLM calls. The daemon sends commands directly through the `/sync` command pipeline.

### How It Works

The daemon watches its own state and pushes display commands to the extension via the existing sync loop:

```
Daemon detects: file written to src/components/Header.tsx
    ↓
Daemon pushes sync command: { type: "subtitle", text: "Writing Header component..." }
    ↓
Extension receives on next /sync poll (1 second)
    ↓
Content script shows subtitle at bottom of viewport
    ↓
3 seconds later: auto-dismisses
```

No LLM involved. The daemon already knows what's happening (it's running the scaffold, proxying tool calls, watching file changes). It just maps events to human-readable messages.

### Aggregated Usage Beacon (every 5 minutes)

Kaboom now emits a structured `usage_summary` rollup every 5 minutes when there has been activity in the window.

```json
{
  "event": "usage_summary",
  "iid": "f7a2c1e9b4d8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:35:00Z",
  "v": "0.8.3",
  "os": "darwin-arm64",
  "channel": "stable",
  "window_m": 5,
  "tool_stats": [
    {
      "family": "observe",
      "name": "errors",
      "tool": "observe:errors",
      "count": 12,
      "latency_avg_ms": 45,
      "latency_max_ms": 230
    },
    {
      "family": "interact",
      "name": "click",
      "tool": "interact:click",
      "count": 24,
      "error_count": 1,
      "latency_avg_ms": 1200,
      "latency_max_ms": 3500
    }
  ],
  "async_outcomes": {
    "complete": 7,
    "error": 1,
    "timeout": 1
  }
}
```

This rollup is a coarse aggregate, not the only telemetry source. Kaboom also emits raw `tool_call`, `first_tool_call`, `session_start`, `session_end`, and `app_error` events. The rollup exists to make aggregate dashboards cheaper and faster.

Nothing from tool arguments or user content should appear in this payload. Tool identity, counts, error counts, and latency aggregates are sufficient for the dashboard.

### Event-to-Subtitle Mapping

Hardcoded in the daemon — a simple map from internal events to display text:

| Daemon Event | Subtitle Text | Duration |
|-------------|---------------|----------|
| Scaffold step: create_project | "Creating your project..." | until next |
| Scaffold step: install_deps | "Installing dependencies..." | until next |
| Scaffold step: add_tailwind | "Setting up Tailwind CSS..." | until next |
| Scaffold step: add_shadcn | "Adding UI components..." | until next |
| Scaffold step: quality_baseline | "Configuring code quality tools..." | until next |
| Scaffold step: git_init | "Initializing version control..." | until next |
| Scaffold step: dev_server_start | "Starting dev server..." | until next |
| Phase 2: file written | "Building {component name}..." | 3s |
| Phase 2: screenshot taken | "Checking layout..." | 2s |
| Phase 2: fix applied | "Fixing {issue}..." | 3s |
| Phase 2: complete | "Your app is ready!" | 5s |
| Deploy started | "Deploying to {platform}..." | until complete |
| Deploy complete | "Live at {url}" | 5s |
| Tests running | "Running tests..." | until complete |
| Tests passed | "All tests passed" | 3s |
| Tests failed | "{n} test(s) failed" | 5s |
| Security scan | "Scanning for vulnerabilities..." | until complete |
| Error detected | "Error in {file}" | 5s (toast, not subtitle) |

### Toasts vs Subtitles

- **Subtitles** — bottom of viewport, translucent, for ongoing status. "Building Header component..." Replaces the previous subtitle.
- **Toasts** — top-right corner, shadcn-style, for discrete events. "All tests passed" or "Error in Header.tsx". Stack up to 3, auto-dismiss.

### Implementation

The daemon already has the sync command mechanism. Adding subtitles is a new command type:

```go
// In the scaffold engine or tool handler:
func (s *Server) pushSubtitle(text string) {
    s.capture.PushCommand(capture.Command{
        Type:   "subtitle",
        Params: map[string]any{"text": text, "duration_ms": 3000},
    })
}

func (s *Server) pushToast(text string, level string) {
    s.capture.PushCommand(capture.Command{
        Type:   "toast",
        Params: map[string]any{"text": text, "level": level},
    })
}
```

The extension already handles subtitle and toast commands from the sync response. No new extension code needed — just new command producers in the daemon.

### File Watcher for AI Agent Actions

During Phase 2 (AI composition) and during normal development with decorated commands, the daemon can watch for file changes in the project directory and show subtitles automatically:

```go
// Watch ~/kaboom-projects/todo-app/src/ for changes
// On file write: push subtitle "Building {filename}..."
// On test file write: push subtitle "Writing test for {component}..."
// Debounce: 500ms (Vite HMR writes multiple files quickly)
```

This means even when the user is working with Claude Code directly in the terminal (no UI buttons), they still see narration in the browser. The daemon watches the filesystem, not the AI — zero tokens.

### Why Zero-Token Matters

- Subtitles during scaffold: the daemon knows every step (it's running them). No AI needed.
- Subtitles during AI composition: the daemon watches file writes. No AI needed.
- Subtitles during normal development: file watcher + event-to-text map. No AI needed.
- Toasts for errors/tests: the daemon already captures these events. No AI needed.

The user gets a narrated experience — "Building Header...", "Running tests...", "Deployed!" — without spending a single extra token. The AI is only used for the actual work (writing code), not for telling the user what it's doing.

## Entry Point: Extension Popup

The scaffold wizard is discoverable from the extension popup's "no tab tracked" state — the cold-start moment when the user has Kaboom installed but nothing to point it at.

**When no tab is tracked, the popup shows two buttons:**

```
┌─────────────────────────────────┐
│  Tab Control                    │
│                                 │
│  ┌───────────────────────────┐  │
│  │     Track This Tab        │  │  ← primary, existing behavior
│  └───────────────────────────┘  │
│  Click to start monitoring      │
│  this tab's console, network,   │
│  and errors                     │
│                                 │
│  ┌───────────────────────────┐  │
│  │    Build Something New    │  │  ← secondary, navigates to /launch
│  └───────────────────────────┘  │
│  Create a new app from scratch  │
│  with AI                        │
└─────────────────────────────────┘
```

**When a tab is tracked:** both buttons disappear, replaced by the tracking bar and post-lovable tools (recording, capture controls, etc.). The `/launch` URL still works for power users who want to create another project.

**"Build Something New" behavior:** opens `localhost:7890/launch` in a new tab and closes the popup. The wizard takes over from there.

## Anonymous Telemetry

Product telemetry to understand adoption and usage without tracking individuals. No cookies, no PII, no consent banners. GDPR-compliant by design.

### Principles

1. **No PII.** No IP addresses stored, no email, no machine fingerprints, no names, no URLs.
2. **No cookies.** Server-side event counting only.
3. **Random install ID only.** A random hex string (`iid`) generated once at install time, stored at `~/.kaboom/install_id`. Not derived from hardware, username, or any machine property. Cannot be reversed to identify a person. Allows tracking usage patterns per install over time without knowing who the install belongs to.
4. **Opt-out available.** `KABOOM_TELEMETRY=off` env var disables all beacons. Documented in CLAUDE.md.
5. **Transparent.** Every beacon call is visible in source code, never hidden.

### Install ID Generation

Generated once during first daemon startup. Never changes. Pure random, not derived from anything:

```go
// On first startup, if ~/.kaboom/install_id doesn't exist:
func generateInstallID() string {
    b := make([]byte, 6) // 12 hex chars
    rand.Read(b)
    id := hex.EncodeToString(b)
    os.WriteFile(filepath.Join(kaboomDir, "install_id"), []byte(id), 0600)
    return id
}
```

The `iid` field is included in every beacon event (not just usage_summary) so all events from the same install can be correlated:
- "This install scaffolded 3 projects, uses observe:errors most, upgraded from 0.8.1 to 0.9.0"
- NOT: "This person at this IP with this email did X"

### Telemetry Endpoint

A single endpoint hosted at `https://t.gokaboom.dev/v1/event` (or self-hosted equivalent):

```
POST /v1/event
Content-Type: application/json

{
  "event": "tool_call",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:01Z",
  "v": "0.8.3",
  "os": "darwin-arm64",
  "channel": "stable",
  "family": "observe",
  "name": "page",
  "tool": "observe:page",
  "outcome": "success"
}
```

**Response:** `202 Accepted`

The canonical contract lives in:

- [docs/core/app-metrics.md](/Users/brenn/dev/gasoline/docs/core/app-metrics.md)

The metrics service flattens these events into Cloudflare Analytics Engine rows for dashboard queries.

### Events

| Event | Fired When | Primary purpose |
|-------|-----------|-----------------|
| `tool_call` | One meaningful tool invocation completes | tool usage, latency, and error analysis |
| `first_tool_call` | The first tool call ever for an install | activation analysis |
| `session_start` | A new session begins | active-session analysis |
| `session_end` | A session ends or rotates | session depth and duration |
| `usage_summary` | 5-minute activity rollup | cheap aggregate dashboards |
| `app_error` | Product/runtime failure outside a normal tool call | reliability analysis |

Scaffold-specific UX should reuse the canonical Kaboom contract. It should not invent a separate telemetry event namespace inside this feature spec.

### What We DON'T Track

- IP addresses (not logged, not stored)
- User identity (no account, no login, no machine ID)
- File contents, project names, descriptions, or any user-generated content
- URLs being tracked/debugged
- Error messages or stack traces from the user's app
- Anything that could identify a specific person or project

### Implementation

**Daemon beacon** (Go):
```go
// Anonymous telemetry — disable with KABOOM_TELEMETRY=off
func fireStructuredBeacon(fields map[string]any) {
    if os.Getenv("KABOOM_TELEMETRY") == "off" {
        return
    }
    go func() {
        // fire-and-forget POST to t.gokaboom.dev/v1/event
        // 2s timeout, ignore errors
    }()
}
```

Kaboom telemetry is daemon-owned. The daemon emits the canonical event set with the shared envelope and install/session identity.

### Analytics Dashboard

The current app telemetry dashboard is the hosted `/app` surface in the metrics worker. Key views should come from the canonical contract:

- monthly active installs
- activation via `first_tool_call`
- tool and subtool usage
- session depth and session duration
- latency and error rates by tool
- async outcome rates
- install-level drilldown
- behavior segments and co-usage analysis

### Telemetry Endpoint Hosting

Options:
- **Cloudflare Worker** — current approach. Structured events flattened into Analytics Engine rows.
- **Self-hosted Go** — possible if it preserves the same contract and normalized storage model.

Recommend Cloudflare Worker for production because it already matches the deployed metrics dashboard and contract.

## Open Questions

- **Error recovery**: if Phase 1 fails mid-way, clean up partial directory or leave for debugging?
- **Project gallery**: should `/launch` also list existing kaboom-projects for re-opening?
- **Port conflicts**: if 5173 is taken, Vite auto-picks another — need to parse actual port from stdout
- **Template variants**: start with one template (React + shadcn), add more later?
- **AI agent selection**: which model for Phase 2 composition? Needs to be fast (Sonnet?) but capable of good UI decisions
- **Offline Phase 2**: can Phase 2 work without an API key? (fallback: static template instead of AI composition)
- **Pencil integration**: offer a "Design first in Pencil → export → scaffold with that design" flow? Pencil has a shadcn design kit
- **Fine-grained streaming**: use Anthropic's `fine-grained-tool-streaming` beta so file writes stream in real-time?
- **Composition depth**: how many components should Phase 2 create? Just the first feature, or a full page?
- **Deploy default**: which platform to default to? Depends on rev share negotiations
- **Security scanner default**: which scanner to recommend? Depends on rev share negotiations with Codacy/Snyk/SonarCloud/Socket
- **Auto-commit frequency**: every 5 saves / 5 minutes — is this too aggressive? Too quiet? Should it be configurable?
- **GitHub push**: should `pnpm github:setup` also set up GitHub Actions CI, or are git hooks enough for v1?
- **CI generation**: should we generate a GitHub Actions workflow that runs the full invariant chain on every PR?

## Success Criteria

- Phase 1 (scaffold to running dev server): < 20 seconds
- Phase 2 (AI composition to meaningful first screen): < 60 seconds
- Total wizard to editable app: < 90 seconds on broadband
- The first screen reflects the user's description — not boilerplate
- Each Phase 2 component is visible in the browser within 3 seconds of being written (HMR)
- Screenshot verification catches and fixes layout issues before declaring completion
- Annotation mode works immediately on all composed components
- All generated code passes `tsc --noEmit` and `eslint` with zero errors
- Pre-commit hook blocks commits with type errors, lint warnings, formatting issues, or component invariant violations
- Component invariant check blocks inline styles, `any` types, hardcoded hex colors, `../` imports, and files over 200 LOC
- Vitest runs with 60% coverage floor enforced on push
- Phase 2 composition passes accessibility audit (`analyze(what='accessibility')`) with zero WCAG violations
- Phase 2 composition passes mobile responsive check (375px viewport, no broken layouts)
- All composed components use shadcn where applicable, Tailwind classes only, theme tokens only, Lucide icons only
- Pre-deploy security scan blocks on high/critical CVEs and catches XSS, secrets in source, missing CSP
- `pnpm deploy` works first try with no manual config on all three platforms
- `pnpm github:setup` creates a private repo and pushes in one interactive flow
- Auto-commit keeps work saved — user never loses more than 5 minutes of edits
- `pnpm security:setup` walks user through connecting a cloud scanner
- User never needs to touch a terminal — everything happens from the wizard
- Goal-backward verification passes: Exists → Substantive → Wired → Functional
- Session-start hook survives `/clear` — agent always knows project conventions
- Context monitor fires warnings at 35% and 25% remaining
- Telemetry beacons fire on install, daemon start, wizard steps, scaffold complete, and first deploy
- All beacons are fire-and-forget (never block the user), respect `KABOOM_TELEMETRY=off`
- No PII in any telemetry event — no IPs, no user IDs, no project content
- Funnel visibility: can measure install → scaffold → deploy conversion
