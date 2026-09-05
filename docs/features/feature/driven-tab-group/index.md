---
doc_type: feature_index
feature_id: feature-driven-tab-group
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - src/background/tab-groups/driven-tab-group.ts
  - src/background/commands/helpers.ts
  - src/background/ui/terminal-workspace.ts
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
| Adoption | `new_tab` and `navigate` with `new_tab` call `adoptTabIntoDrivenGroup` directly. `switch_tab` and the four auto-track recovery paths reach it through `persistTrackedTab`, the funnel that makes a tab the tracked tab. |
| Release | `close_tab` calls `noteDrivenTabClosed`; a daemon disconnect calls `endDrivenTabGroupSession`. |
| Reconciliation | The first drive of each service-worker lifetime queries live `chrome.tabGroups` for groups titled `KaBOOM! agent` and ungroups any that is not the live session's. |

## The permission

`tabGroups` is a **required** permission in `extension/manifest.json`, so grouping works
from the first drive with no user action, no toggle and no prompt.

That permission does carry the Chrome warning *"View and manage your tab groups"*, and
Chrome disables an extension on auto-update when an update adds a warning-bearing
permission. That cost lands only on installs that auto-update from the Chrome Web Store.
Kaboom is installed unpacked (README: *Load unpacked*) and the Web Store upload is a
manual step that is neither automated nor in CI, so there is no auto-updating install
base to disable.

The alternative was rejected on evidence, not preference. Declaring it optional and
requesting it at runtime cannot work from the background: Chrome requires
`permissions.request` to be called *"from inside a user gesture, like a button's click
handler"*, and an MV3 service worker never has one. Optional therefore means a popup
toggle the user has to find first — and a feature whose entire purpose is showing which
tabs the agent holds is worth nothing while switched off.

**If Kaboom is ever published to the Web Store with a real install base, revisit this.**
The choice then is a one-time re-approval prompt on the update that ships it, or moving
back to an opt-in toggle. The contract in
`tests/extension/contracts/chrome-platform-limits.test.js` records the reasoning.

Grouping is still never load-bearing. On a browser with no tab-group API the drive
proceeds ungrouped and the reason is named once on the diagnostic queue:

| Reason | Cause |
| --- | --- |
| `tab_groups_api_unavailable` | The browser has no `chrome.tabs.group` / `chrome.tabGroups`. |
| `group_failed: <browser error>` | Chrome refused the grouping call (for example while a tab is being dragged). |

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

- A plain `navigate` in an already-tracked tab does not adopt; only tabs Kaboom opened, was
  handed, or auto-tracked join the group.
