---
feature: driven-tab-group
status: shipped
tool: interact
doc_type: qa-plan
feature_id: feature-driven-tab-group
last_reviewed: 2026-09-05
---

# QA Plan: Driven Tab Group

## Automated coverage

All tests run against `tests/extension/tab-groups/tab-groups-fixture.js`, an in-memory Chrome
tab/tab-group world. Nothing here touches a real browser.

### Adoption and identity — `tests/extension/tab-groups/driven-tab-group.test.js`

| Behaviour | Test |
| --- | --- |
| A tab Kaboom opened joins a titled, purple, uncollapsed group | `a tab Kaboom opened joins a titled, coloured group (new_tab)` |
| A tab handed over joins the same group, not a second one | `a tab the user hands over joins the same group (switch_tab)` |
| A tracked-tab hand-over joins the same group | `a tracked-tab hand-over joins the same group (set_tracked)` |
| An invalid tab id is refused before any Chrome call | `a nonsense tab id is refused without touching Chrome` |

### No permission gate — same file

| Behaviour | Test |
| --- | --- |
| Grouping engages on the first drive with no user action | `grouping engages on the first drive with no user action` |
| The worker never calls `permissions.request` (it has no user gesture to spend) | `the worker never calls permissions.request — it has no user gesture to spend` |
| A browser with no tab-group API degrades with its own named reason | `a browser without the tab-group APIs degrades with its own reason` |

### Session lifecycle — same file

| Behaviour | Test |
| --- | --- |
| Ending the session ungroups every driven tab | `ending the session ungroups every driven tab` |
| The daemon exiting ends the group without another drive | `the daemon going away ends the group without another drive` |
| A new connection generation retires the previous group | `a new MCP client session retires the previous group` |
| Closing the last driven tab forgets the group | `closing the last driven tab forgets the group instead of driving at a dead id` |
| A group the user dissolved is replaced, not reused | `a group the user dissolved is replaced, not reused` |

### Reconciliation reads live Chrome — same file

| Behaviour | Test |
| --- | --- |
| An orphan group from a dead worker is released on the first drive, with zero storage reads | `an orphan group from a dead worker is released on the first drive` |
| The terminal workspace group is left alone | `reconciliation leaves the terminal workspace group alone` |
| The live session's own group is never released | `reconciliation never releases the group of the live session` |
| Reconciliation runs once per worker, not per drive | `reconciliation runs once per worker, not on every drive` |

### Entry-point routing — `tests/extension/tab-groups/browser-action-group-adoption.test.js`

| Behaviour | Test |
| --- | --- |
| The `persistTrackedTab` funnel adopts, so no recovery path can forget | `persistTrackedTab adopts the tab it just made tracked` |
| A hand-over adopts exactly once, producing one group not two | `a tab is adopted exactly once when switch_tab hands it over` |
| `new_tab` adopts | `new_tab puts the tab Kaboom opened into the group` |
| `navigate` with `new_tab` uses the same path | `navigate with new_tab uses the same adoption path` |
| `switch_tab` adopts | `switch_tab adopts the tab the user handed over` |
| Opened and handed-over tabs share one group | `a tab opened then handed over shares one group` |
| `close_tab` on the last driven tab leaves no group | `close_tab on the last driven tab leaves no group behind` |
| A browser that cannot group still drives | `a browser that cannot group tabs never breaks the drive` |
| A Chrome grouping failure still drives | `a Chrome grouping failure never breaks the drive` |

### Platform contract — `tests/extension/contracts/chrome-platform-limits.test.js`

Holds `tabGroups` to being a **required** manifest permission, and records both the reasoning and
the condition that would reverse it (a real Web Store install base).

## Verified manually, not automatically

| Behaviour | How to check |
| --- | --- |
| The group is visible and legible in a real tab strip | Drive two tabs in a window of twenty; the purple `KaBOOM! agent` group should be findable at a glance. |
| Closing the group closes every driven tab | Close the group header in Chrome. |
| The terminal workspace group survives a driven session end | Open the terminal, drive tabs, end the session; the orange `KaBOOM!` group must remain. |
| No permission prompt appears on a fresh unpacked install | Load unpacked, drive a tab, observe no dialog. |

## Not covered today

| Gap | Consequence if wrong |
| --- | --- |
| The fixture models Chrome's tab-group API; no test runs against real Chrome. | A behaviour Chrome has and the fixture does not (group ordering, cross-window moves, drag-in-progress refusals) would pass here and fail in the browser. |
| A plain `navigate` in an already-tracked tab is untested because it deliberately does not adopt. | If adoption were added there later, nothing would catch a double-adoption. |
| Nothing tests two concurrent MCP client sessions in one browser. | The generation counter is the only thing separating them; a race between two sessions rotating groups is unmodelled. |
| The Web Store auto-update disable path is reasoning, not a test. | If Kaboom is ever published with an auto-updating install base, nothing fails to warn that the next update disables the extension. |

## See also

- [Driven Tab Group index](./index.md)
- [Driven Tab Group Product Spec](./product-spec.md)
- [Driven Tab Group Tech Spec](./tech-spec.md)
