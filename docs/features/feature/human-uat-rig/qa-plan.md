---
feature: human-uat-rig
status: in_progress
tool: none
mode: n/a
doc_type: qa-plan
feature_id: feature-human-uat-rig
last_reviewed: 2026-09-05
---

# QA Plan: Human UAT Rig

The contract is itself a test, so its QA is mutation: each invariant must be shown to
fail when the thing it guards is violated.

| Check | Mutation | Expected failure |
|---|---|---|
| Coverage | Delete one case from `cases.json` | `TestEverySchemaModeHasExactlyOneCase` names that mode |
| Duplicate coverage | Add a second case for `observe/page` | Same test reports "more than one case" |
| Staleness | Add a case for `interact/mutant_mode` | `TestNoCaseNamesAModeThatNoLongerShips` names it |
| Surfaces | Remove `supervision/stop_button` | `TestEveryNonMCPSurfaceHasACase` names it |
| Wording | Change one question to "Does it work as expected?" | `TestNoQuestionIsUnfalsifiable` quotes the phrase |
| Distinctness | Copy one question onto a second case | `TestNoTwoCasesAskTheSameQuestion` names both ids |
| Runnability | Blank one setup | `TestEveryCaseIsAnswerableAsWritten` names it |
| Control | Point `loadInventory` at a missing file | Every test fails rather than passing empty |

Verified 2026-09-05: staleness, duplicate coverage and distinctness were each confirmed
by applying the mutation, observing the named failure, and restoring.

## Not covered here

Whether a question is *good* — specific, falsifiable, tied to something the tester can
see independently — is not machine-checkable beyond the wording and distinctness guards.
That is a review responsibility, and the standard is written in the tech spec's table
and demonstrated by the shipped cases.
