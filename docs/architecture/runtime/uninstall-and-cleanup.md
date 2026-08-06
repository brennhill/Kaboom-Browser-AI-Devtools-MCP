---
doc_type: flow_map
flow_id: uninstall-and-cleanup
status: active
last_reviewed: 2026-06-10
owners:
  - Brenn
entrypoints:
  - scripts/setup/uninstall.sh
  - scripts/setup/uninstall.ps1
  - npm/kaboom-agentic-browser/lib/uninstall.js:executeUninstall
code_paths:
  - scripts/setup/uninstall.sh
  - scripts/setup/uninstall.ps1
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/skills.js
  - cmd/browser-agent/native_install.go
  - internal/state/paths.go
  - internal/identity/mcp.go
test_paths:
  - tests/cli/uninstall-script.test.cjs
  - tests/cli/uninstall.test.cjs
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
---

# Uninstall and Cleanup

## Scope

Covers full removal of Kaboom from a user machine across both distribution
channels, reversing every artifact the install flow creates (see
[Installer Binary Path and Manual Extension Handoff](./installer-binary-path-and-manual-extension-handoff.md)
for the install-side flow):

1. One-liner uninstall: `scripts/setup/uninstall.sh` (macOS/Linux) and
   `scripts/setup/uninstall.ps1` (Windows) — counterparts to `install.sh`/`install.ps1`.
2. npm wrapper uninstall: `kaboom-agentic-browser --uninstall`
   (`lib/uninstall.js`) — removes MCP client config entries and managed
   skills only; it does not remove curl-installed binaries or autostart
   registrations.

## Artifact Coverage (shell uninstallers)

| Artifact | Created by | Removed by |
| --- | --- | --- |
| `~/.kaboom/bin/*` binaries + runtime state (`logs/`, `run/`, `projects/`, `recordings/`, `settings/`, `install_id`) | install.sh / daemon (`internal/state/paths.go`) | `remove_state_root` (honors `KABOOM_STATE_DIR`, `XDG_STATE_HOME`; `--keep-data` keeps everything except `bin/` and `run/`) |
| `~/KaboomAgenticDevtoolExtension` (or `KABOOM_EXTENSION_DIR`) | install.sh / install.ps1 | `remove_path "$EXT_DIR"` (browser-side unload is a documented manual step) |
| macOS LaunchAgent `~/Library/LaunchAgents/com.kaboom.daemon.plist` | install.sh `register_autostart` | `launchctl bootout gui/$UID/com.kaboom.daemon` + file removal |
| Linux systemd unit `~/.config/systemd/user/kaboom.service` (+ `default.target.wants` symlink) and XDG `~/.config/autostart/kaboom.desktop` | install.sh `register_autostart` | `systemctl --user disable --now` + file removal |
| Shell rc PATH lines marked `# kaboom` (`.zshrc`, `.bashrc`, `.profile`, fish config) | install.sh `register_path` | `clean_path_lines` (all four files — user may have switched shells) |
| MCP client config entries (canonical key `kaboom-browser-devtools` + legacy `kaboom*`/`gasoline*`/`strum*` keys) in Claude Code, Claude Desktop, Cursor, Windsurf, VS Code, Gemini CLI, Antigravity, OpenCode, Zed | `native_install.go --install` | `claude mcp remove` + `strip_mcp_entries` (in-place JSON edit with `.kaboom-uninstall.bak` backup; shared settings files are never deleted) |
| Managed agent skills in `~/.claude/skills`, `~/.codex/skills`, `~/.gemini/skills` | npm postinstall / `install-bundled-skills.sh` / `claude_skill/install.sh` | marker-scan removal (`<!-- kaboom\|gasoline\|strum-managed-skill`); unmanaged skills untouched |
| Legacy artifacts: `~/kaboom-upload-dir`, `~/kaboom-logs.jsonl`, `~/kaboom-crash.log`, `~/.kaboom-*.pid`, `~/.kaboom-settings.json`, `~/.gasoline`, `~/.strum`, OS config-dir root (`~/Library/Application Support/kaboom` / `~/.config/kaboom` / `%APPDATA%\kaboom`) | older versions / daemon defaults | explicit legacy sweep (skipped with `--keep-data`) |
| Running daemons (port 7890) | `--install` / autostart | anchored-name process stop (full binary names only — never bare `gasoline`/`strum` substrings) |

## Primary Flow (uninstall.sh)

1. Parse flags (`--yes`, `--dry-run`, `--keep-data`); print removal plan.
2. Confirm: interactive prompt on a TTY; non-interactive runs require `--yes`
   (fail-closed for `curl | bash`).
3. Stop daemons (anchored process-name match, TERM then KILL).
4. Unregister autostart (LaunchAgent / systemd / XDG, per platform).
5. Remove MCP client entries: `claude mcp remove` for Claude Code; in-place
   JSON edits (python3, node fallback, manual-instructions fallback) for
   file-based clients. Shared settings files (Zed/Gemini/OpenCode) are edited,
   never deleted; every edited file gets a `.kaboom-uninstall.bak` backup.
6. Remove managed skills by marker scan (both `<id>/SKILL.md` and `<id>.md`
   layouts).
7. Strip `# kaboom` PATH lines from all candidate rc files.
8. Remove extension dir, state roots, and legacy artifacts.
10. Print manual steps: remove the unpacked extension in `chrome://extensions`,
    restart the terminal, delete `.bak` backups once verified.

## Safety Invariants

- All recursive deletes go through `safe_rm_rf`, which refuses empty,
  relative, `/`, and `$HOME` targets.
- Dry-run mode (`--dry-run`) performs zero mutations (regression-tested).
- Non-TTY runs without `--yes` abort before any mutation (regression-tested).
- Client config files are edited by key, never unlinked (regression-tested —
  contrast with the npm `lib/uninstall.js` whole-file unlink behavior, tracked
  as a known issue).

## Tests

- `tests/cli/uninstall-script.test.cjs` — static artifact-coverage contract
  plus behavioral sandbox runs (`--yes`, no-flag abort, `--dry-run`,
  `--keep-data`) under an isolated `$HOME` with stubbed
  `launchctl`/`systemctl`/`pgrep`/`claude`/`curl`.
- `npm/kaboom-agentic-browser/lib/uninstall.test.js` — npm-channel uninstall.
- `tests/cli/uninstall.test.cjs` — npm packaging/branding regression guards.

## Related

- Feature docs: `docs/features/feature/enhanced-cli-config/`
- Install-side canonical map:
  [Installer Binary Path and Manual Extension Handoff](./installer-binary-path-and-manual-extension-handoff.md)
