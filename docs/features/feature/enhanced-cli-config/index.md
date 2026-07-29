---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - internal/configdiscovery/mcp.go
  - cmd/browser-agent/main.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/internal/cli/cli_output.go
  - cmd/browser-agent/internal/health/doctor.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolconfigure/deps.go
  - cmd/browser-agent/internal/health/doctor_fastpath_telemetry.go
  - internal/diag/output.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/handlers.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/snippets.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/playbooks.go
  - Makefile
  - scripts/release/version/version-sync.mjs
  - .github/workflows/validate-versions.yml
  - .github/workflows/cut-release.yml
  - .github/workflows/release.yml
  - scripts/release/build-crx.js
  - cmd/browser-agent/internal/nativeinstall/installer.go
  - cmd/browser-agent/internal/nativeinstall/codex.go
  - npm/kaboom-agentic-browser/lib/extension.js
  - npm/kaboom-agentic-browser/lib/browser.js
  - npm/kaboom-agentic-browser/lib/health.js
  - npm/kaboom-agentic-browser/lib/daemon.js
  - npm/kaboom-agentic-browser/lib/output.js
  - scripts/install.sh
  - scripts/install.ps1
  - scripts/install-bundled-skills.sh
  - scripts/clean-old-daemons.sh
  - scripts/rebuild.sh
  - scripts/uninstall.sh
  - scripts/uninstall.ps1
  - server/scripts/install.js
  - npm/kaboom-agentic-browser/bin/kaboom-agentic-browser
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/doctor.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/kill-daemon.js
  - npm/kaboom-agentic-browser/lib/skills.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - npm/kaboom-agentic-browser/lib/output.js
  - npm/kaboom-agentic-browser/lib/auto-approve.js
  - npm/kaboom-agentic-browser/lib/codex-config.js
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/internal/nativeinstall/installer_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/tools_interface_check_test.go
  - cmd/browser-agent/main_flags_test.go
  - cmd/browser-agent/main_io_unit_test.go
  - cmd/browser-agent/main_helpers_more_test.go
  - cmd/browser-agent/internal/health/health_coverage_test.go
  - cmd/browser-agent/server_reliability_integration_test.go
  - scripts/smoke-tests/23-doctor-preflight.sh
  - cmd/browser-agent/config_parallel_test.go
  - internal/configdiscovery/mcp_test.go
  - cmd/browser-agent/stdout_protocol_boundary_test.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/tutorial_test.go
  - cmd/browser-agent/internal/nativeinstall/installer_test.go
  - cmd/browser-agent/internal/nativeinstall/codex_test.go
  - cmd/browser-agent/internal/nativeinstall/config_test.go
  - cmd/browser-agent/internal/nativeinstall/open_test.go
  - cmd/browser-agent/internal/nativeinstall/connect_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/auto-approve.test.js
  - npm/kaboom-agentic-browser/lib/codex-config.test.js
  - npm/kaboom-agentic-browser/lib/output.test.js
  - npm/kaboom-agentic-browser/lib/extension.test.js
  - npm/kaboom-agentic-browser/lib/browser.test.js
  - npm/kaboom-agentic-browser/lib/health.test.js
  - npm/kaboom-agentic-browser/lib/daemon.test.js
  - npm/kaboom-agentic-browser/lib/doctor.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - npm/kaboom-agentic-browser/lib/kill-daemon.test.js
  - npm/kaboom-agentic-browser/lib/skills.test.js
  - npm/kaboom-agentic-browser/lib/no-compatibility.test.js
  - tests/packaging/kaboom-packaging-branding.test.js
  - tests/extension/release/install-script-extension-source.test.js
  - server/scripts/install.test.js
  - tests/extension/release/release-extension-zip.test.js
  - tests/extension/release/release-extension-crx-fallback.test.js
  - tests/extension/release/manifest-startup-integrity.test.js
  - tests/cli/lifecycle/server-install-hardening.test.cjs
  - tests/cli/runtime/cli-integration.test.cjs
  - tests/cli/runtime/config.test.cjs
  - tests/cli/runtime/doctor.test.cjs
  - tests/cli/lifecycle/install.test.cjs
  - tests/cli/lifecycle/uninstall.test.cjs
  - tests/cli/lifecycle/uninstall-script.test.cjs
  - tests/cli/lifecycle/clean-old-daemons-branding.test.cjs
  - tests/cli/lifecycle/install-script-safety.test.cjs
  - tests/cli/contracts/operator-script-branding.test.cjs
  - scripts/release/canonical-installer-scripts.test.mjs
  - scripts/release/version/version-sync.test.mjs
  - tests/extension/contracts/tooling-contracts.test.js
  - cmd/browser-agent/internal/cli/cli_test.go
  - cmd/browser-agent/internal/cli/cli_coverage_extra_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Enhanced Cli Config

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: cli
- Location: `docs/features/feature/enhanced-cli-config`
- `cmd/browser-agent/config.go` owns both flag parsing and the runtime mode policy those flags drive.
- `VERSION` is the only human-edited release version. `make bump-version NEW_VERSION=X.Y.Z`, `make sync-version`, and `make validate-versions` all delegate to one explicit transactional implementation.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENHANCED_CLI_CONFIG_001
- FEATURE_ENHANCED_CLI_CONFIG_002
- FEATURE_ENHANCED_CLI_CONFIG_003

## Code and Tests

Daemon updates are performed through the supported installer and CLI paths
listed above. The popup no longer exposes a dormant one-click update flow:
its `/upgrade/nonce` and `/upgrade/install` calls had no server routes or
OpenAPI contract.

- Native configuration discovery and installation recognize and write only the
  canonical `kaboom-browser-devtools` MCP identity; unrelated server entries
  are preserved without migration-specific handling.
- PyPI wrapper config helpers now converge on `merge_kaboom_config(...)`, and packaged `.egg-info` metadata now exposes only Kaboom package names, entry points, and repo URLs.
- The npm wrapper recognizes, installs, diagnoses, approves, stops, and
  uninstalls only canonical Kaboom identities and state paths. It does not
  retain migration branches for historical server names, skill markers,
  process names, config keys, config paths, or PID files.
- Skill-install and doctor output report only fields produced by the canonical
  installers; obsolete legacy-removal counters and warning renderers are gone.
- Server postinstall now validates `kaboom-browser-devtools` on `/health` reuse checks and points manual extension loading at `KABOOM_EXTENSION_DIR` / `~/KaboomAgenticDevtoolExtension`.
- Server postinstall process discovery and health gating use only canonical
  Kaboom binary and service identities.
- Shell and PowerShell install, rebuild, daemon cleanup, skill installation,
  and uninstall scripts now operate only on canonical Kaboom binaries,
  processes, state, skill markers, server identities, and client config
  locations. They contain no historical-brand cleanup or alternate config-key
  branches; the authored installer scripts are regression-checked below 800
  lines.
- Uninstall removes only installed artifacts; telemetry environment controls
  are not treated as artifacts, and the shell uninstaller retains no beacon or
  dead version-capture path.
- Managed Codex skills keep YAML frontmatter as the first document block; the
  Kaboom ownership/version marker is inserted immediately after frontmatter so
  Codex validation and safe managed cleanup both recognize the installed file.
- Install now also fixes the Claude Code `claude mcp add-json` invocation (JSON passed as a positional arg, not stdin) and adds **Codex CLI** as a supported client (`~/.codex/config.toml`, TOML; honors `$CODEX_HOME`).
- Native `--install` has the same Codex support as the npm entry point.
  `--install codex` selects Codex explicitly; unknown positional targets fail
  instead of silently configuring every JSON client. The native TOML writer
  preserves comments and unrelated tables and atomically replaces only the
  canonical Kaboom server block.
- Daemon setup diagnostics use one canonical CLI entry point, `--doctor`; the duplicate `--check` facade is rejected.
- Runtime help uses one canonical configure mode, `tutorial`; the duplicate `examples` mode is rejected.
- Tutorial context receives its three live browser signals through an explicit
  dependency value composed at startup; it no longer requires ToolHandler to
  mirror tracking or pilot APIs for configure.
- Direct CLI diagnostics and formatted results use the writer injected through
  `cli.RuntimeConfig`; production supplies the canonical diagnostic sink, and
  tests use owned buffers without replacing process stdout or stderr.

## Tool Auto-Approve (default-ON)

Install trusts **all** Kaboom MCP tools by default in every client that exposes a verified config-file mechanism, so the user is never prompted. It is merge-safe (preserves existing config, dedupes, creates keys/file when absent, fails loud on genuine write errors), and uninstall removes every entry it added.

| Client | Config-file mechanism (verified) | Implemented |
| --- | --- | --- |
| Claude Code | `~/.claude/settings.json` → `permissions.allow += "mcp__kaboom-browser-devtools"` (bare rule = all tools) — [docs](https://code.claude.com/docs/en/permissions) | Yes |
| Gemini CLI | `mcpServers.<name>.trust = true` — [docs](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md) | Yes |
| OpenCode | `permission["kaboom-browser-devtools_*"] = "allow"` — [docs](https://opencode.ai/docs/permissions/) | Yes |
| Zed | `agent.tool_permissions.tools["mcp:kaboom-browser-devtools:<tool>"] = {default:"allow"}` (per-tool; no server wildcard) — [docs](https://zed.dev/docs/ai/tool-permissions) | Yes |
| Codex CLI | `[mcp_servers.kaboom-browser-devtools] default_tools_approval_mode = "approve"` — [docs](https://developers.openai.com/codex/mcp) | Yes |
| Claude Desktop | none (no official `autoApprove` field; UI approval only) | UI-only |
| Cursor | none (mcp.json has no trust field; UI Run Modes) — [docs](https://cursor.com/docs/context/mcp) | UI-only |
| Windsurf | none (mcp_config.json has no trust field; UI Turbo) | UI-only |
| VS Code | only a **global** `chat.tools.global.autoApprove` (trusts every server) — out of scope for a Kaboom-scoped installer — [docs](https://code.visualstudio.com/docs/agents/approvals) | UI-only |
| Antigravity | none in mcp_config.json (auto-approve is a separate UI-managed policy) | UI-only |
