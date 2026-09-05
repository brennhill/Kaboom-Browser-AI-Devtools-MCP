---
feature: driven-tab-group
status: shipped
tool: interact
mode: new_tab, navigate, switch_tab, close_tab
doc_type: product-spec
feature_id: feature-driven-tab-group
last_reviewed: 2026-09-05
---

# Product Spec: Driven Tab Group

## Purpose

Show the user which tabs the agent is holding, in a window where they are not watching any of them.

`kaboom-05ue.6` lets Kaboom drive a tab the user is not looking at. A per-tab badge cannot answer
"which of these twenty tabs is the agent touching" — the user would have to visit each one. A named,
coloured Chrome tab group answers it from the tab strip, collapses out of the way, and takes every
driven tab with it when closed.

## Requirements

| # | Requirement |
| --- | --- |
| 1 | Every tab Kaboom opens, is handed, or auto-tracks joins one group titled `KaBOOM! agent`, coloured purple. |
| 2 | Grouping engages on the first drive with no user action, no toggle, and no prompt. |
| 3 | One group per MCP client session. A new session releases the previous group's tabs and opens a fresh one. |
| 4 | Ending a session, closing the last driven tab, or the daemon exiting leaves no orphan group behind. |
| 5 | A group orphaned by a dead service worker is released on the next drive. |
| 6 | Grouping is never load-bearing: a browser without the tab-group API still drives, ungrouped, with the reason named once. |
| 7 | The terminal workspace group (`KaBOOM!`, orange) is never dissolved by reconciliation. |

## What it deliberately does not do

- **It is visibility, not permission.** Nothing refuses to drive a tab outside the group, and no
  action prompts for consent. The user's lever is the group: close it and Kaboom's tabs are gone.
- **It does not persist across service-worker restarts through storage.** State is reconciled
  against live Chrome, not read from a mirror that goes stale when a worker dies.
- **It does not adopt on a plain `navigate` in an already-tracked tab.** Only tabs Kaboom opened,
  was handed, or auto-tracked join.

## The permission trade, stated

`tabGroups` is a **required** manifest permission. It carries the Chrome warning *"View and manage
your tab groups"*, and Chrome disables an extension on auto-update when an update adds a
warning-bearing permission — but that cost lands only on Chrome Web Store installs that
auto-update. Kaboom is installed unpacked, and the Web Store upload is a manual step that is
neither automated nor in CI, so there is no auto-updating install base to disable.

Declaring it optional was rejected on evidence: `chrome.permissions.request` must be called from
inside a user gesture, which an MV3 service worker never has. Optional therefore means a popup
toggle the user must find first, and a feature that exists to show which tabs the agent holds is
worth nothing while switched off.

**If Kaboom is ever published with a real auto-updating install base, revisit this.** The contract
in `tests/extension/contracts/chrome-platform-limits.test.js` records the reasoning and the
condition that would reverse it.

## Deprecations

The popup toggle that requested `tabGroups` at runtime, and its supporting module
`src/popup/driven-tab-group-permission.ts`, are removed. There is nothing left to grant.

## See also

- [Driven Tab Group index](./index.md)
- [Driven Tab Group Tech Spec](./tech-spec.md)
- [Driven Tab Group QA Plan](./qa-plan.md)
