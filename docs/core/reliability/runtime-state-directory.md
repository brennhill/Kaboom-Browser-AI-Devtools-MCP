---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Runtime State Directory

## Problem

Kaboom previously wrote several runtime files directly into the user home root (for example `~/kaboom-logs.jsonl` and `~/.kaboom-7890.pid`).

This caused two issues:

1. Home-directory clutter and user complaints.
2. Unclear ownership of files when Kaboom runs as a long-lived server process independent of any single project checkout.

## Decision

All Kaboom runtime artifacts now live under a single dotfolder: **`~/.kaboom`**.

Resolution order:

1. `KABOOM_STATE_DIR` env var (if set) — overrides everything
2. `XDG_STATE_HOME/kaboom` (if `XDG_STATE_HOME` is set — Linux users who set this expect it)
3. `~/.kaboom` — cross-platform default

This works the same on macOS, Linux, and Windows, and is more discoverable than platform-specific paths like `~/Library/Application Support/kaboom`.

## Layout

```
~/.kaboom/
  logs/
    kaboom.jsonl          # server lifecycle and debug logs
    crash.log               # panic crash output
  run/
    kaboom-<port>.pid     # per-port PID file
  recordings/               # saved recording metadata and video sidecars
  screenshots/              # captured screenshots
  settings/
    extension-settings.json # extension settings cache
  security/
    security.json           # security policy configuration
  projects/
    Users/brenn/dev/myapp/  # mirrored abs path (minus leading /)
      meta.json
      baselines/
      noise/
      errors/
      ...
```

## Project-Scoped Persistence

AI persistence data is stored centrally under `~/.kaboom/projects/{abs-path}/` (with the leading `/` stripped from the absolute project path). This keeps all Kaboom state in one place rather than scattering `.kaboom/` directories inside each project.

## Canonical Path Contract

All reads, writes, and cleanup operations use the canonical state root. Kaboom
does not inspect, migrate, or remove files in historical locations.
