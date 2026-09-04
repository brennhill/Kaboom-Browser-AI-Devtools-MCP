---
doc_type: feature_index
feature_id: feature-driven-tab-group
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-04
code_paths:
  - src/background/tab-groups/driven-tab-group.ts
  - src/background/exec/browser-actions.ts
  - src/background/runtime-state/connection-state.ts
  - src/background/runtime-state/connection-generation.ts
  - extension/manifest.json
test_paths:
  - tests/extension/tab-groups/driven-tab-group.test.js
  - tests/extension/tab-groups/browser-action-group-adoption.test.js
  - tests/extension/tab-groups/tab-groups-fixture.js
---

# Driven tab group

Every tab Kaboom drives lives in one named, coloured Chrome tab group, so the user can see at a
glance which tabs the agent holds and separate them from their own browsing.

## Why this exists

`kaboom-05ue.6` makes Kaboom drive tabs in the background, which means the user is by definition
not looking at the driven tab. A per-tab badge cannot answer "which of these tabs is the agent
touching" for a window of twenty tabs. A group can: it is named, it is coloured, it collapses,
and closing it takes every driven tab with it.

The group is **visibility, not permission**. Nothing refuses to drive a tab outside the group,
and no action prompts for consent. The user's lever is the group itself — close it and Kaboom's
tabs are gone.

## What it does

| Behaviour | Detail |
| --- | --- |
| Group identity | Title `KaBOOM! agent`, colour purple, never collapsed on creation. Distinct from the terminal workspace group (`KaBOOM!`, orange) so reconciliation never dissolves the terminal's tabs. |
| One group per session | The group is keyed to the daemon connection generation. A new generation is a new MCP client session: the old group is released and a fresh one opened. |
| Adoption | `new_tab`, `navigate` with `new_tab`, and `switch_tab` (which carries `set_tracked`) all call `adoptTabIntoDrivenGroup`. |
| Release | `close_tab` calls `noteDrivenTabClosed`; a daemon disconnect calls `endDrivenTabGroupSession`. |
| Reconciliation | The first drive of each service-worker lifetime queries live `chrome.tabGroups` for groups titled `KaBOOM! agent` and ungroups any that is not the live session's. |

## The permission

`tabGroups` is already declared under `optional_permissions` in `extension/manifest.json`. It is
requested with `chrome.permissions.request` on the **first drive**, never at install, and at most
once per service-worker lifetime. Every refusal path degrades to today's ungrouped driving and
names its reason on the diagnostic queue:

| Reason | Cause |
| --- | --- |
| `tab_groups_api_unavailable` | The browser has no `chrome.tabs.group` / `chrome.tabGroups`. |
| `permission_request_unavailable` | No `chrome.permissions.request` to call. |
| `permission_request_failed` | Chrome rejected the request — most often because a service worker carries no user gesture. The browser error is logged alongside. |
| `permission_denied` | The user refused, or a prior request in this worker already failed. |
| `group_failed: <browser error>` | Chrome refused the grouping call (for example while a tab is being dragged). |

A denial is logged once per distinct reason rather than once per action, and the grant is re-read
live on every drive, so a permission granted later starts working without another prompt.

## Why in-memory state, not storage

The owned group id is held in module memory and reconciled against live `chrome.tabGroups`
before every use. A storage mirror goes stale the moment the worker dies without flushing — the
same failure that left `TERMINAL_UI_STATE='open'` forever (CLAUDE.md rule 18). Three live checks
replace it:

- `chrome.tabGroups.get` before reusing a remembered group id, so a group the user dissolved is
  replaced rather than driven at.
- `chrome.tabGroups.query({ title })` once per worker, so an orphan from a dead worker or a dead
  daemon is released on the next drive.
- `subscribeExtensionConnection` on the sync connection edge, so the daemon exiting releases the
  group without waiting for another drive.

## One helper owns the invariant

`adoptTabIntoDrivenGroup(tabId, entryPoint)` is the only place a tab joins the group; permission
resolution, reconciliation, session rotation and labelling all happen inside it. A new entry
point cannot join the group while forgetting to reconcile, which is the failure rule 19 was
written for (`applyRootFolder` omitting an `unmountPanel()` another caller remembered).

## Known gaps

- `persistTrackedTab` in `src/background/commands/helpers.ts` auto-tracks a tab during target
  recovery (`auto_tracked_active_tab`, `auto_tracked_random_tab`, `auto_tracked_new_tab`). Those
  hand-overs do not yet adopt; adding the call inside `persistTrackedTab` covers all of them at
  once.
- `chrome.permissions.request` requires a user gesture, which an MV3 service worker does not
  carry. Until an extension-page entry point requests it, grouping only engages for users who
  granted `tabGroups` through another surface, and every other user gets a named degrade.
- A plain `navigate` in an already-tracked tab does not adopt; only tabs Kaboom opened or was
  handed join the group.
