---
feature: driven-tab-group
status: shipped
tool: interact
doc_type: tech-spec
feature_id: feature-driven-tab-group
last_reviewed: 2026-09-05
---

# Tech Spec: Driven Tab Group

## Components

| Component | Responsibility |
| --- | --- |
| `src/background/tab-groups/driven-tab-group.ts` | The whole feature: adoption, reconciliation, session rotation, release, degradation reporting. |
| `src/background/commands/helpers.ts` | `persistTrackedTab` — the funnel that makes a tab the tracked tab, and therefore where the four auto-track recovery paths reach adoption. |
| `src/background/exec/browser-actions.ts` | `new_tab`, `navigate` with `new_tab`, `switch_tab`, `close_tab` entry points. |
| `src/background/runtime-state/connection-generation.ts` | The generation counter that keys a group to one MCP client session. |
| `src/background/runtime-state/connection-state.ts` | `subscribeExtensionConnection` — the edge that releases the group when the daemon exits. |
| `src/background/ui/terminal-workspace.ts` | Consumes `canGroupTabs()` for the terminal's own group. |
| `extension/manifest.json` | Declares `tabGroups` as required. |

## Group identity

| Property | Value | Why |
| --- | --- | --- |
| Title | `KaBOOM! agent` | Distinct from the terminal workspace group (`KaBOOM!`) so reconciliation, which matches on title, never dissolves the terminal's tabs. |
| Colour | purple | Distinct from the terminal group's orange. |
| Collapsed | false on creation | A collapsed group hides the thing it exists to show. |

## The one helper that owns the invariant

`adoptTabIntoDrivenGroup(tabId, entryPoint)` is the only path into the group. Permission resolution,
reconciliation, session rotation and labelling all happen inside it. A new entry point cannot join
the group while forgetting to reconcile — the failure CLAUDE.md rule 19 was written for.

Adoption lives inside `persistTrackedTab` rather than at its call sites for the same reason: the
four auto-track recovery paths (`auto_tracked_active_tab`, `auto_tracked_random_tab`,
`auto_tracked_new_tab`, and the `tryAutoTrackFallback` retry) all pass through that funnel, and a
fifth added later inherits adoption without knowing about it.

## Capability detection, split by need

| Function | Requires | Used by |
| --- | --- | --- |
| `canCreateTabGroups()` | `chrome.tabs.group`, `chrome.tabGroups.update` | `canGroupTabs()`, exported for the terminal workspace, which only ever creates |
| `tabGroupApisPresent()` | the above plus `chrome.tabs.ungroup`, `chrome.tabGroups.query` | `groupingBlockedBy()`, which needs reconciliation and release too |

Splitting them matters because the terminal workspace would otherwise be blocked by the absence of
APIs it never calls.

## Why in-memory state, not storage

The owned group id lives in module memory and is reconciled against live Chrome before every use. A
storage mirror goes stale the moment the worker dies without flushing — the same failure that left
`TERMINAL_UI_STATE='open'` forever and suppressed the flame indefinitely (CLAUDE.md rule 18).

Three live checks replace the mirror:

| Check | Catches |
| --- | --- |
| `chrome.tabGroups.get` before reusing a remembered id | A group the user dissolved by hand |
| `chrome.tabGroups.query({ title })`, once per worker | An orphan from a dead worker or a dead daemon |
| `subscribeExtensionConnection` on the sync connection edge | The daemon exiting, without waiting for another drive |

Reconciliation runs once per service-worker lifetime, not per drive: the query is a Chrome round
trip and the orphan set cannot change between drives of the same worker.

## Failure modes

| Failure | Behaviour | Reported as |
| --- | --- | --- |
| Browser has no tab-group API | The tab still opens and is still driven, ungrouped | `tab_groups_api_unavailable` on the diagnostic queue |
| Chrome refuses the grouping call (e.g. a tab is being dragged) | The drive completes, ungrouped | `group_failed: <browser error>` |
| Invalid tab id | Refused before any Chrome call | `invalid_tab_id`, returned to the caller |
| Last driven tab closed | Group id forgotten; the next adoption opens a fresh group | — |

No failure here fails the drive. The degradation reason is logged once, never swallowed.

## Known gaps

- A plain `navigate` in an already-tracked tab does not adopt. Only tabs Kaboom opened, was handed,
  or auto-tracked join the group, so a tab the user tracked before the first drive stays outside it
  until one of those paths runs.

## See also

- [Driven Tab Group index](./index.md)
- [Driven Tab Group Product Spec](./product-spec.md)
- [Driven Tab Group QA Plan](./qa-plan.md)
