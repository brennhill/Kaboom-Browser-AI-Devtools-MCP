---
title: "KaBOOM v0.8.1 Released"
description: "Sync-era protocol cleanup, recording approval UX hardening, installer reliability upgrades, and broader test coverage."
date: 2026-03-06
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.1
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v0']
---

## What's New in v0.8.1

Kaboom v0.8.1 focuses on reliability and consistency across the daemon, extension, and test harness.

### Highlights

- Unified sync-era behavior by removing legacy split-result paths and aligning command/result flow through `/sync`.
- Hardened screen-recording UX with clearer approval status and improved popup/recording state handling.
- Improved command and context-menu state consistency for recording and annotation-related controls.
- Installer hardening updates for safer staging, replacement, and verification flows.
- Broader regression coverage across Go and extension test suites, including updated contracts and goldens.

### Quality Gates

- `make test`
- `make test-js`

Both passed for the `v0.8.1` release cut.

### Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

### Full Changelog

[View v0.8.1 on GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v0.8.1)
