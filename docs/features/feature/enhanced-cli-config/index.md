---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/main.go
  - cmd/browser-agent/internal/cli/cli_output.go
  - cmd/browser-agent/internal/health/doctor.go
  - cmd/browser-agent/internal/health/doctor_fastpath_telemetry.go
  - internal/diag/output.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/handlers.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/snippets.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/playbooks.go
  - Makefile
  - scripts/release/build-crx.js
  - cmd/browser-agent/native_install.go
  - npm/kaboom-agentic-browser/lib/extension.js
  - npm/kaboom-agentic-browser/lib/browser.js
  - npm/kaboom-agentic-browser/lib/health.js
  - npm/kaboom-agentic-browser/lib/daemon.js
  - npm/kaboom-agentic-browser/lib/output.js
  - scripts/install.sh
  - scripts/install.ps1
  - scripts/uninstall.sh
  - scripts/uninstall.ps1
  - server/scripts/install.js
  - npm/kaboom-agentic-browser/bin/kaboom-agentic-browser
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/doctor.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - npm/kaboom-agentic-browser/lib/output.js
  - npm/kaboom-agentic-browser/lib/auto-approve.js
  - npm/kaboom-agentic-browser/lib/codex-config.js
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/stdout_protocol_boundary_test.go
  - cmd/browser-agent/internal/toolconfigure/tutorial/tutorial_test.go
  - cmd/browser-agent/native_install_test.go
  - cmd/browser-agent/native_install_open_test.go
  - cmd/browser-agent/native_install_connect_test.go
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
  - tests/packaging/kaboom-packaging-branding.test.js
  - tests/extension/install-script-extension-source.test.js
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/cli/server-install-hardening.test.cjs
  - tests/cli/cli-integration.test.cjs
  - tests/cli/config.test.cjs
  - tests/cli/doctor.test.cjs
  - tests/cli/install.test.cjs
  - tests/cli/uninstall.test.cjs
  - tests/cli/uninstall-script.test.cjs
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Enhanced Cli Config

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: cli
- Location: `docs/features/feature/enhanced-cli-config`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENHANCED_CLI_CONFIG_001
- FEATURE_ENHANCED_CLI_CONFIG_002
- FEATURE_ENHANCED_CLI_CONFIG_003

## Code and Tests

- npm wrapper installs now register `kaboom-browser-devtools` and remove legacy `kaboom-*`, `gasoline-*`, and `strum-*` MCP entries during install/update/uninstall.
- npm wrapper config helpers now converge on `mergeKaboomConfig(...)`, and doctor treats legacy MCP keys as non-OK until customers reinstall.
- PyPI wrapper config helpers now converge on `merge_kaboom_config(...)`, and packaged `.egg-info` metadata now exposes only Kaboom package names, entry points, and repo URLs.
- Platform npm packages now ship `kaboom-agentic-browser` and `kaboom-hooks` binaries while preserving legacy cleanup for customer machines.
- Server postinstall now validates `kaboom-browser-devtools` on `/health` reuse checks and points manual extension loading at `KABOOM_EXTENSION_DIR` / `~/KaboomAgenticDevtoolExtension`.
- Install now also fixes the Claude Code `claude mcp add-json` invocation (JSON passed as a positional arg, not stdin) and adds **Codex CLI** as a supported client (`~/.codex/config.toml`, TOML; honors `$CODEX_HOME`).

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
