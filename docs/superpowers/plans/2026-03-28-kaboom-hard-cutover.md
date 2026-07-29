# Kaboom Hard-Cutover Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all `Kaboom` and `Kaboom` branding, packaging, runtime contracts, and installer-managed state with one hard-cutover identity: `Kaboom` on `gokaboom.dev`, with the original flame icon restored.

**Architecture:** Execute the rename as a sequence of small, bounded commits. Separate visible branding, path/package renames, runtime-contract changes, and cleanup logic so each patch stays reviewable and each behavior change gets its own failing test first. Avoid hand-editing generated outputs until the last regeneration pass.

**Tech Stack:** TypeScript MV3 extension, Astro site, Go MCP server/CLI, npm wrapper packages, PyPI wrapper packages, Node test runner, Python unittest, Go test.

---

## Working Rules

- Keep each commit at **<=500 changed LOC** including tests and docs.
- Do not manually edit generated outputs:
  - `gokaboom.dev/dist/**`
  - `gokaboom.dev/.astro/**`
  - `gokaboom.dev/dist/**`
  - `gokaboom.dev/.astro/**`
  - `pypi/**/**/*.egg-info/**`
  - compiled extension bundles under `extension/*.js` generated from `src/**`
- After every `src/**` edit, run `make compile-ts`.
- For any runtime contract change touching TS/Go payloads, run `make check-wire-drift`.
- For tasks touching `src/background/**` or `src/popup/**`, run `npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`.
- Before each implementation task, inspect direct callers and related tests with `rg`.

## File Map

### Extension And UI

- `extension/manifest.json`
- `extension/icons/*`
- `src/sidepanel.ts`
- `src/options.ts`
- `src/popup/logo-motion.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `tests/extension/branding/logo-motion.test.js`
- `tests/extension/terminal-sidepanel/sidepanel-terminal.test.js`
- `tests/extension/ui-controls/tracked-hover-launcher.test.js`
- `tests/extension/popup-shell/options.test.js`

### Site And Domain

- `gokaboom.dev/astro.config.mjs`
- `gokaboom.dev/src/assets/logo.svg`
- `gokaboom.dev/src/assets/logo-animated.svg`
- `gokaboom.dev/src/components/Head.astro`
- `gokaboom.dev/src/components/Footer.astro`
- `gokaboom.dev/src/components/Landing.astro`
- `gokaboom.dev/src/components/RotatingHero.astro`
- `gokaboom.dev/src/content/docs/*.md`
- `gokaboom.dev/src/content/docs/*.mdx`
- `gokaboom.dev/src/pages/*.ts`
- `gokaboom.dev/src/pages/markdown/[...slug].md.ts`
- `scripts/docs/site/check-gokaboom-content-contract.mjs`

### Root Metadata And Build Paths

- `package.json`
- `Makefile`
- `eslint.config.js`
- `CLAUDE.md`
- `CONTRIBUTING.md`

### npm Wrapper

- `npm/kaboom-agentic-browser/package.json`
- `npm/kaboom-agentic-browser/README.md`
- `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser`
- `npm/kaboom-agentic-browser/bin/kaboom-hooks`
- `npm/kaboom-agentic-browser/lib/config.js`
- `npm/kaboom-agentic-browser/lib/install.js`
- `npm/kaboom-agentic-browser/lib/uninstall.js`
- `npm/kaboom-agentic-browser/lib/postinstall-skills.js`
- `npm/kaboom-agentic-browser/lib/skills.js`
- `npm/kaboom-agentic-browser/lib/install.test.js`
- `npm/kaboom-agentic-browser/lib/uninstall.test.js`
- `npm/*/package.json`

### PyPI Wrapper

- `pypi/kaboom-agentic-browser/pyproject.toml`
- `pypi/kaboom-agentic-browser/README.md`
- `pypi/kaboom-agentic-browser/kaboom_agentic_browser/*.py`
- `pypi/kaboom-agentic-browser/tests/*.py`
- `pypi/kaboom-agentic-browser-*/pyproject.toml`
- `pypi/kaboom-agentic-browser-*/README.md`
- `pypi/kaboom-agentic-browser-*/kaboom_agentic_browser_*/*.py`

### Runtime Contracts

- `src/lib/daemon-http.ts`
- `src/lib/telemetry-beacon.ts`
- `src/types/runtime-messages.ts`
- `src/types/global.d.ts`
- `src/inject/api.ts`
- `src/inject/index.ts`
- `src/inject/settings.ts`
- `src/options.ts`
- `cmd/browser-agent/internal/playbooks/resource_catalog.go`
- `cmd/browser-agent/openapi.json`
- `cmd/browser-agent/openapi.go`
- `cmd/browser-agent/setup.html`
- `cmd/browser-agent/docs.html`
- `cmd/browser-agent/logs.html`
- `internal/diag/debug_file.go`
- `cmd/browser-agent/native_install.go`
- `cmd/browser-agent/main_connection_stop.go`
- `cmd/browser-agent/native_install_config_test.go`
- `cmd/browser-agent/handler_unit_test.go`

### Docs And Flow Maps

- `docs/architecture/flow-maps/README.md`
- `docs/architecture/flow-maps/gokaboom-content-publishing-and-agent-markdown.md`
- `docs/architecture/flow-maps/terminal-side-panel-host.md`
- `docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md`
- `docs/features/feature/terminal/index.md`
- `docs/features/feature/terminal/flow-map.md`
- `docs/features/feature/terminal/product-spec.md`
- `docs/features/feature/terminal/tech-spec.md`
- `docs/features/feature/tab-tracking-ux/index.md`
- `docs/features/feature/tab-tracking-ux/flow-map.md`

## Chunk 1: Visible Brand Shell

### Task 1: Restore Flame Icon And Rename Extension Shell

**Budget:** <=500 changed LOC

**Files:**
- Modify: `extension/icons/icon.svg`
- Modify: `extension/icons/icon-glow.svg`
- Modify: `extension/icons/logo-animated.svg`
- Modify: `extension/manifest.json`
- Modify: `src/popup/logo-motion.ts`
- Test: `tests/extension/branding/logo-motion.test.js`
- Test: `tests/extension/popup-shell/popup-features.test.js`

- [ ] **Step 1: Write the failing tests**
  Add assertions that the extension shell uses `Kaboom` labels and the flame asset paths instead of Kaboom-specific assets.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test tests/extension/branding/logo-motion.test.js tests/extension/popup-shell/popup-features.test.js`

- [ ] **Step 3: Implement the minimal branding change**
  Restore flame-based SVG assets and update manifest/popup logo-motion references to `Kaboom`.

- [ ] **Step 4: Compile and rerun**
  Run: `make compile-ts`
  Run: `node --test tests/extension/branding/logo-motion.test.js tests/extension/popup-shell/popup-features.test.js`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: restore flame icon and kaboom extension shell"`

### Task 2: Rename Terminal, Hover, And Options Surfaces

**Budget:** <=500 changed LOC

**Files:**
- Modify: `src/sidepanel.ts`
- Modify: `src/content/ui/tracked-hover-launcher.ts`
- Modify: `src/options.ts`
- Test: `tests/extension/terminal-sidepanel/sidepanel-terminal.test.js`
- Test: `tests/extension/ui-controls/tracked-hover-launcher.test.js`
- Test: `tests/extension/popup-shell/options.test.js`

- [ ] **Step 1: Write the failing tests**
  Add assertions for `Kaboom` labels, terminal fallback copy, hover launcher strings, and options diagnostics text.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test tests/extension/terminal-sidepanel/sidepanel-terminal.test.js tests/extension/ui-controls/tracked-hover-launcher.test.js tests/extension/popup-shell/options.test.js`

- [ ] **Step 3: Implement the minimal string rename**
  Replace visible Kaboom/Kaboom copy with `Kaboom` in the three UI entry points only.

- [ ] **Step 4: Verify**
  Run: `make compile-ts`
  Run: `node --test tests/extension/terminal-sidepanel/sidepanel-terminal.test.js tests/extension/ui-controls/tracked-hover-launcher.test.js tests/extension/popup-shell/options.test.js`
  Run: `npm run typecheck`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename extension ui surfaces to kaboom"`

### Task 3: Rename Site Shell Copy And Core Flame Assets

**Budget:** <=500 changed LOC

**Files:**
- Modify: `gokaboom.dev/src/assets/logo.svg`
- Modify: `gokaboom.dev/src/assets/logo-animated.svg`
- Modify: `gokaboom.dev/src/components/Head.astro`
- Modify: `gokaboom.dev/src/components/Footer.astro`
- Modify: `gokaboom.dev/src/components/Landing.astro`
- Modify: `gokaboom.dev/src/components/RotatingHero.astro`
- Modify: `gokaboom.dev/src/content/docs/index.mdx`
- Create: `tests/site/kaboom-brand-shell.test.js`

- [ ] **Step 1: Write the failing test**
  Create a site branding test that asserts the shell surfaces say `Kaboom`, reference `gokaboom.dev`, and use the flame logo assets.

- [ ] **Step 2: Run test to verify it fails**
  Run: `node --test tests/site/kaboom-brand-shell.test.js`

- [ ] **Step 3: Implement the minimal site shell rename**
  Update only the site shell and homepage entry copy, keeping deeper docs/content for later tasks.

- [ ] **Step 4: Verify**
  Run: `node --test tests/site/kaboom-brand-shell.test.js`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename site shell to kaboom"`

## Chunk 2: Site Path And Root Metadata

### Task 4: Rename Site Directory And Root Script References

**Budget:** <=500 changed LOC

**Files:**
- Move: `gokaboom.dev` -> `gokaboom.dev`
- Modify: `package.json`
- Modify: `Makefile`
- Modify: `eslint.config.js`
- Test: `tests/site/kaboom-brand-shell.test.js`

- [ ] **Step 1: Extend the failing test**
  Add assertions or helper checks that reference the new `gokaboom.dev` path and fail while the old path is still canonical.

- [ ] **Step 2: Run test to verify it fails**
  Run: `node --test tests/site/kaboom-brand-shell.test.js`

- [ ] **Step 3: Rename the site root and update path references**
  Use `git mv` for the directory rename, then update root script/build references to the new path.

- [ ] **Step 4: Verify**
  Run: `node --test tests/site/kaboom-brand-shell.test.js`
  Run: `npm run docs:lint:content-contract`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename site root to gokaboom"`

### Task 5: Rename Site Metadata Generators And Domain Contracts

**Budget:** <=500 changed LOC

**Files:**
- Modify: `gokaboom.dev/astro.config.mjs`
- Modify: `gokaboom.dev/src/pages/[...slug].md.ts`
- Modify: `gokaboom.dev/src/pages/index.md.ts`
- Modify: `gokaboom.dev/src/pages/llms-full.txt.ts`
- Modify: `gokaboom.dev/src/pages/llms.txt.ts`
- Modify: `gokaboom.dev/src/pages/markdown/[...slug].md.ts`
- Move: `scripts/docs/site/check-gokaboom-content-contract.mjs` -> `scripts/docs/site/check-gokaboom-content-contract.mjs`
- Create: `tests/site/gokaboom-domain-contract.test.js`

- [ ] **Step 1: Write the failing test**
  Assert that canonical URLs and generated markdown surfaces emit `gokaboom.dev` and `Kaboom`, not the older domains.

- [ ] **Step 2: Run test to verify it fails**
  Run: `node --test tests/site/gokaboom-domain-contract.test.js`

- [ ] **Step 3: Implement the minimal domain-contract rename**
  Update metadata generators and the content-contract script path/logic to the new domain identity.

- [ ] **Step 4: Verify**
  Run: `node --test tests/site/gokaboom-domain-contract.test.js`
  Run: `npm run docs:lint:content-contract`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename site domain contracts to gokaboom"`

### Task 6: Rename Root Package Metadata And Public Repo URLs

**Budget:** <=500 changed LOC

**Files:**
- Modify: `package.json`
- Modify: `CONTRIBUTING.md`
- Modify: `CLAUDE.md`
- Create: `tests/cli/contracts/root-metadata-branding.test.cjs`

- [ ] **Step 1: Write the failing test**
  Assert root metadata points at Kaboom naming and `gokaboom.dev`.

- [ ] **Step 2: Run test to verify it fails**
  Run: `node --test tests/cli/contracts/root-metadata-branding.test.cjs`

- [ ] **Step 3: Implement the minimal metadata rename**
  Update package metadata and root docs that define the repo/site-facing public identity.

- [ ] **Step 4: Verify**
  Run: `node --test tests/cli/contracts/root-metadata-branding.test.cjs`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename root metadata to kaboom"`

## Chunk 3: npm And PyPI Packaging

### Task 7: Rename npm Wrapper Package Identity

**Budget:** <=500 changed LOC

**Files:**
- Move: `npm/kaboom-agentic-browser` -> `npm/kaboom-agentic-browser`
- Modify: `npm/kaboom-agentic-browser/package.json`
- Modify: `npm/kaboom-agentic-browser/README.md`
- Move: `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser` -> `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser`
- Move: `npm/kaboom-agentic-browser/bin/kaboom-hooks` -> `npm/kaboom-agentic-browser/bin/kaboom-hooks`
- Test: `npm/kaboom-agentic-browser/lib/install.test.js`
- Test: `npm/kaboom-agentic-browser/lib/uninstall.test.js`

- [ ] **Step 1: Write the failing tests**
  Add assertions for new package/bin names and help text; do not touch cleanup logic yet.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test npm/kaboom-agentic-browser/lib/install.test.js npm/kaboom-agentic-browser/lib/uninstall.test.js`

- [ ] **Step 3: Implement the minimal npm identity rename**
  Rename the package directory and wrapper-facing package/bin metadata.

- [ ] **Step 4: Verify**
  Run: `node --test npm/kaboom-agentic-browser/lib/install.test.js npm/kaboom-agentic-browser/lib/uninstall.test.js`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename npm wrapper to kaboom"`

### Task 8: Rename npm Platform Package Metadata

**Budget:** <=500 changed LOC

**Files:**
- Modify: `npm/darwin-arm64/package.json`
- Modify: `npm/darwin-x64/package.json`
- Modify: `npm/linux-arm64/package.json`
- Modify: `npm/linux-x64/package.json`
- Modify: `npm/win32-x64/package.json`
- Modify: `npm/publish.sh`
- Test: `tests/cli/lifecycle/install.test.cjs`
- Test: `tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 1: Write the failing tests**
  Extend CLI tests to assert platform package names/descriptions and publish script references use Kaboom names.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test tests/cli/lifecycle/install.test.cjs tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 3: Implement the minimal platform metadata rename**
  Update the platform package descriptors and publish script references only.

- [ ] **Step 4: Verify**
  Run: `node --test tests/cli/lifecycle/install.test.cjs tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename npm platform packages to kaboom"`

### Task 9: Rename PyPI Common Package Identity

**Budget:** <=500 changed LOC

**Files:**
- Move: `pypi/kaboom-agentic-browser` -> `pypi/kaboom-agentic-browser`
- Modify: `pypi/kaboom-agentic-browser/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser/README.md`
- Move: `pypi/kaboom-agentic-browser/kaboom_agentic_browser` -> `pypi/kaboom-agentic-browser/kaboom_agentic_browser`
- Modify: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/__main__.py`
- Modify: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py`
- Test: `pypi/kaboom-agentic-browser/tests/test_install.py`
- Test: `pypi/kaboom-agentic-browser/tests/test_uninstall.py`

- [ ] **Step 1: Write the failing tests**
  Update Python tests to expect Kaboom package identity and command references.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_install.py pypi/kaboom-agentic-browser/tests/test_uninstall.py`

- [ ] **Step 3: Implement the minimal common-package rename**
  Rename the common PyPI package directory and public-facing metadata references.

- [ ] **Step 4: Verify**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_install.py pypi/kaboom-agentic-browser/tests/test_uninstall.py`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename pypi wrapper to kaboom"`

### Task 10: Rename PyPI Platform Package Metadata

**Budget:** <=500 changed LOC

**Files:**
- Move: `pypi/kaboom-agentic-browser-darwin-arm64` -> `pypi/kaboom-agentic-browser-darwin-arm64`
- Move: `pypi/kaboom-agentic-browser-darwin-x64` -> `pypi/kaboom-agentic-browser-darwin-x64`
- Move: `pypi/kaboom-agentic-browser-linux-arm64` -> `pypi/kaboom-agentic-browser-linux-arm64`
- Move: `pypi/kaboom-agentic-browser-linux-x64` -> `pypi/kaboom-agentic-browser-linux-x64`
- Move: `pypi/kaboom-agentic-browser-win32-x64` -> `pypi/kaboom-agentic-browser-win32-x64`
- Modify: `pypi/kaboom-agentic-browser-darwin-arm64/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser-darwin-x64/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser-linux-arm64/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser-linux-x64/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser-win32-x64/pyproject.toml`
- Modify: `pypi/kaboom-agentic-browser-*/README.md`
- Move: `pypi/kaboom-agentic-browser-*/kaboom_agentic_browser_*` -> `pypi/kaboom-agentic-browser-*/kaboom_agentic_browser_*`
- Test: `pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 1: Write the failing test**
  Extend platform cleanup/metadata tests to expect Kaboom package names and descriptions.

- [ ] **Step 2: Run test to verify it fails**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 3: Implement the minimal platform metadata rename**
  Update platform package metadata and README references without touching cleanup semantics yet.

- [ ] **Step 4: Verify**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "brand: rename pypi platform packages to kaboom"`

## Chunk 4: Runtime Contract Rename

### Task 11: Rename TS Header And Storage Contracts

**Budget:** <=500 changed LOC

**Files:**
- Modify: `src/lib/daemon-http.ts`
- Modify: `src/options.ts`
- Modify: `src/types/runtime-messages.ts`
- Modify: `src/lib/constants.ts`
- Test: `tests/extension/popup-shell/options.test.js`
- Test: `tests/extension/contracts/tooling-contracts.test.js`

- [ ] **Step 1: Write the failing tests**
  Assert header names, storage keys, and visible option persistence names use Kaboom contracts.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test tests/extension/popup-shell/options.test.js tests/extension/contracts/tooling-contracts.test.js`

- [ ] **Step 3: Implement the minimal contract rename**
  Rename TS-side headers/storage/message constants without touching the window API yet.

- [ ] **Step 4: Verify**
  Run: `make compile-ts`
  Run: `node --test tests/extension/popup-shell/options.test.js tests/extension/contracts/tooling-contracts.test.js`
  Run: `npm run typecheck`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename ts runtime contracts to kaboom"`

### Task 12: Rename Injected Developer API To `window.__kaboom`

**Budget:** <=500 changed LOC

**Files:**
- Modify: `src/inject/api.ts`
- Modify: `src/inject/index.ts`
- Modify: `src/types/global.d.ts`
- Test: `tests/extension/injection/inject-context-api-actions.test.js`
- Test: `tests/extension/injection/inject-v5-wiring.test.js`

- [ ] **Step 1: Write the failing tests**
  Assert the browser-exposed API is `window.__kaboom` and that old `window.__kaboom` references are removed.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test tests/extension/injection/inject-context-api-actions.test.js tests/extension/injection/inject-v5-wiring.test.js`

- [ ] **Step 3: Implement the minimal API rename**
  Rename only the injected developer API and its type exports.

- [ ] **Step 4: Verify**
  Run: `make compile-ts`
  Run: `node --test tests/extension/injection/inject-context-api-actions.test.js tests/extension/injection/inject-v5-wiring.test.js`
  Run: `npm run typecheck`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename inject api to kaboom"`

### Task 13: Rename Go Headers, Resource URIs, And Setup Strings

**Budget:** <=500 changed LOC

**Files:**
- Modify: `cmd/browser-agent/internal/playbooks/resource_catalog.go`
- Modify: `cmd/browser-agent/openapi.go`
- Modify: `cmd/browser-agent/openapi.json`
- Modify: `cmd/browser-agent/setup.html`
- Modify: `cmd/browser-agent/docs.html`
- Modify: `cmd/browser-agent/logs.html`
- Modify: `internal/diag/debug_file.go`
- Test: `cmd/browser-agent/handler_unit_test.go`
- Test: `cmd/browser-agent/native_install_config_test.go`

- [ ] **Step 1: Write the failing tests**
  Update Go tests to expect `kaboom://` resources, `X-Kaboom-*` headers, and Kaboom setup strings.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `go test ./cmd/browser-agent -run 'Test(Resources|NativeInstall|ReadResource|Handle.*)'`

- [ ] **Step 3: Implement the minimal Go contract rename**
  Rename resource URIs, setup HTML strings, header references, and debug prefixes.

- [ ] **Step 4: Verify**
  Run: `go test ./cmd/browser-agent -run 'Test(Resources|NativeInstall|ReadResource|Handle.*)'`
  Run: `make check-wire-drift`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "refactor: rename go runtime contracts to kaboom"`

## Chunk 5: Aggressive Cleanup Semantics

### Task 14: Add npm Install Cleanup For Kaboom/Kaboom Artifacts

**Budget:** <=500 changed LOC

**Files:**
- Modify: `npm/kaboom-agentic-browser/lib/install.js`
- Modify: `npm/kaboom-agentic-browser/lib/postinstall-skills.js`
- Modify: `npm/kaboom-agentic-browser/lib/skills.js`
- Test: `npm/kaboom-agentic-browser/lib/install.test.js`
- Test: `tests/cli/lifecycle/install.test.cjs`

- [ ] **Step 1: Write the failing tests**
  Add install tests that create legacy Kaboom/Kaboom managed artifacts and assert Kaboom install removes them before writing new config/skills.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test npm/kaboom-agentic-browser/lib/install.test.js tests/cli/lifecycle/install.test.cjs`

- [ ] **Step 3: Implement the minimal install cleanup**
  Add one shared cleanup helper that removes first-party managed legacy artifacts and old state during install/postinstall.

- [ ] **Step 4: Verify**
  Run: `node --test npm/kaboom-agentic-browser/lib/install.test.js tests/cli/lifecycle/install.test.cjs`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "feat: wipe legacy installs before kaboom npm install"`

### Task 15: Add npm Uninstall And Update Cleanup

**Budget:** <=500 changed LOC

**Files:**
- Modify: `npm/kaboom-agentic-browser/lib/uninstall.js`
- Modify: `npm/kaboom-agentic-browser/lib/cli.js`
- Test: `npm/kaboom-agentic-browser/lib/uninstall.test.js`
- Test: `tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 1: Write the failing tests**
  Add uninstall/update-oriented tests that assert Kaboom removes Kaboom, Kaboom, and Kaboom managed entries and skills.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `node --test npm/kaboom-agentic-browser/lib/uninstall.test.js tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 3: Implement the minimal uninstall/update cleanup**
  Reuse the cleanup helper and ensure update paths invoke cleanup before reinstall.

- [ ] **Step 4: Verify**
  Run: `node --test npm/kaboom-agentic-browser/lib/uninstall.test.js tests/cli/lifecycle/uninstall.test.cjs`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "feat: wipe legacy installs on kaboom uninstall and update"`

### Task 16: Add PyPI Cleanup For Install And Uninstall

**Budget:** <=500 changed LOC

**Files:**
- Modify: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py`
- Modify: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py`
- Modify: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py`
- Test: `pypi/kaboom-agentic-browser/tests/test_install.py`
- Test: `pypi/kaboom-agentic-browser/tests/test_uninstall.py`
- Test: `pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 1: Write the failing tests**
  Add Python tests for wipe-and-go cleanup of Kaboom/Kaboom state, skills, and config before install and during uninstall.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_install.py pypi/kaboom-agentic-browser/tests/test_uninstall.py pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 3: Implement the minimal Python cleanup**
  Add a shared cleanup helper in the Python wrapper and call it from install/uninstall flows.

- [ ] **Step 4: Verify**
  Run: `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_install.py pypi/kaboom-agentic-browser/tests/test_uninstall.py pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "feat: wipe legacy installs in pypi kaboom flows"`

### Task 17: Add Go Native Cleanup And Stop-Path Cleanup

**Budget:** <=500 changed LOC

**Files:**
- Modify: `cmd/browser-agent/native_install.go`
- Modify: `cmd/browser-agent/main_connection_stop.go`
- Modify: `cmd/browser-agent/main_connection_force_cleanup_strategies.go`
- Test: `cmd/browser-agent/native_install_test.go`
- Test: `cmd/browser-agent/test_daemon_cleanup_test.go`

- [ ] **Step 1: Write the failing tests**
  Add Go tests that assert Kaboom native install/stop flows kill and remove Kaboom/Kaboom managed state as part of wipe-and-go behavior.

- [ ] **Step 2: Run tests to verify they fail**
  Run: `go test ./cmd/browser-agent -run 'Test(NativeInstall|DaemonCleanup|Stop.*)'`

- [ ] **Step 3: Implement the minimal Go cleanup**
  Add native cleanup helpers for managed state and daemon naming variants.

- [ ] **Step 4: Verify**
  Run: `go test ./cmd/browser-agent -run 'Test(NativeInstall|DaemonCleanup|Stop.*)'`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "feat: wipe legacy managed state in kaboom native flows"`

## Chunk 6: Docs, Content, And Final Audit

### Task 18: Update Canonical Flow Maps And Feature Pointers

**Budget:** <=500 changed LOC

**Files:**
- Modify: `docs/architecture/flow-maps/README.md`
- Move: `docs/architecture/flow-maps/gokaboom-content-publishing-and-agent-markdown.md` -> `docs/architecture/flow-maps/gokaboom-content-publishing-and-agent-markdown.md`
- Modify: `docs/architecture/flow-maps/gokaboom-content-publishing-and-agent-markdown.md`
- Modify: `docs/architecture/flow-maps/terminal-side-panel-host.md`
- Modify: `docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md`
- Modify: `docs/features/feature/terminal/index.md`
- Modify: `docs/features/feature/terminal/flow-map.md`
- Modify: `docs/features/feature/terminal/product-spec.md`
- Modify: `docs/features/feature/terminal/tech-spec.md`
- Modify: `docs/features/feature/tab-tracking-ux/index.md`
- Modify: `docs/features/feature/tab-tracking-ux/flow-map.md`

- [ ] **Step 1: Write a failing docs contract check if needed**
  If existing docs checks do not cover the renamed flow-map references, add the smallest failing assertion to a docs script test or create `tests/cli/contracts/docs-branding-contract.test.cjs`.

- [ ] **Step 2: Run the docs check to verify it fails**
  Run: `npm run docs:lint:content-contract`

- [ ] **Step 3: Implement the docs flow-map rename**
  Update canonical map references, feature pointers, and `last_reviewed`/code/test path anchors to Kaboom naming.

- [ ] **Step 4: Verify**
  Run: `npm run docs:lint:content-contract`

- [ ] **Step 5: Commit**
  Commit: `git commit -m "docs: update kaboom flow maps and feature pointers"`

### Task 19: Rename Site Content Corpus In Small Batches

**Budget:** <=500 changed LOC per commit

**Files:**
- Modify batch 1: `gokaboom.dev/src/content/docs/agent-install-guide.md`, `downloads.md`, `getting-started.md`
- Modify batch 2: `alternatives.md`, `architecture.md`, `discover-workflows.mdx`, `execute-scripts.md`
- Modify batch 3: `features.md`, `privacy.md`, `security.md`, `troubleshooting.md`

- [ ] **Step 1: Write or extend the failing site branding test**
  Expand `tests/site/gokaboom-domain-contract.test.js` to scan the touched content files for disallowed `Kaboom`/`Kaboom`/old-domain strings.

- [ ] **Step 2: Run test to verify it fails**
  Run: `node --test tests/site/gokaboom-domain-contract.test.js`

- [ ] **Step 3: Implement one content batch only**
  Keep each batch below 500 changed LOC; if a batch exceeds the cap, split it again before committing.

- [ ] **Step 4: Verify**
  Run: `node --test tests/site/gokaboom-domain-contract.test.js`
  Run: `npm run docs:lint:content-contract`

- [ ] **Step 5: Commit each batch**
  Commit message pattern: `docs: rename site content batch N to kaboom`

### Task 20: Final Legacy-Name Audit And Regeneration

**Budget:** <=500 changed LOC plus generated artifact refresh

**Files:**
- Modify: `tests/cli/contracts/root-metadata-branding.test.cjs`
- Modify: `tests/site/gokaboom-domain-contract.test.js`
- Modify: any last small stragglers found by scan
- Regenerate: compiled extension outputs and site outputs after code changes are complete

- [ ] **Step 1: Add the final failing audit**
  Add a final scan test that rejects surviving first-party `Kaboom`, `Kaboom`, `gokaboom`, and `gokaboom` strings outside approved historical/spec files.

- [ ] **Step 2: Run the final audit to verify it fails**
  Run: `node --test tests/cli/contracts/root-metadata-branding.test.cjs tests/site/gokaboom-domain-contract.test.js`

- [ ] **Step 3: Clear remaining legacy names in small follow-up commits**
  Only touch the files identified by the scan; split into multiple sub-500 LOC commits if necessary.

- [ ] **Step 4: Regenerate built artifacts**
  Run: `make compile-ts`
  Run: `npm run docs:lint:content-contract`
  Run: site build command after the path rename is complete

- [ ] **Step 5: Final verification commit**
  Commit: `git commit -m "chore: finalize kaboom hard cutover"`

## Final Verification Checklist

- `make compile-ts`
- `npm run typecheck`
- `npm run docs:lint:content-contract`
- `node --test tests/extension/branding/logo-motion.test.js tests/extension/terminal-sidepanel/sidepanel-terminal.test.js tests/extension/ui-controls/tracked-hover-launcher.test.js tests/extension/popup-shell/options.test.js`
- `node --test tests/site/kaboom-brand-shell.test.js tests/site/gokaboom-domain-contract.test.js`
- `node --test tests/cli/lifecycle/install.test.cjs tests/cli/lifecycle/uninstall.test.cjs tests/cli/contracts/root-metadata-branding.test.cjs`
- `node --test npm/kaboom-agentic-browser/lib/install.test.js npm/kaboom-agentic-browser/lib/uninstall.test.js`
- `python3 -m unittest pypi/kaboom-agentic-browser/tests/test_install.py pypi/kaboom-agentic-browser/tests/test_uninstall.py pypi/kaboom-agentic-browser/tests/test_platform_cleanup.py`
- `go test ./cmd/browser-agent -run 'Test(NativeInstall|DaemonCleanup|Stop.*|Resources|ReadResource|Handle.*)'`
- `make check-wire-drift`
- `npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`

## Notes

- If a single task discovers more than 500 LOC of fallout, stop and split the task before implementation.
- Treat generated artifacts as the last step in a chunk, not hand-maintained sources.
- Do not attempt state migration. Cleanup is destructive for first-party managed legacy state by design.
