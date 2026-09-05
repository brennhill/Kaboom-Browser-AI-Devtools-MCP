---
doc_type: feature_index
feature_id: feature-human-uat-rig
status: in_progress
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - scripts/uat/human/cases.json
  - scripts/uat/human/inventory/inventory.go
  - scripts/uat/human/runner/main.go
  - scripts/uat/human/runner/session.go
  - scripts/uat/human/runner/prompt.go
  - scripts/uat/human/runner/record.go
  - scripts/uat/human/runner/mcpclient.go
test_paths:
  - scripts/contracts/humanuat/main_test.go
  - scripts/uat/human/inventory/inventory_test.go
  - scripts/uat/human/runner/session_test.go
  - scripts/uat/human/runner/prompt_test.go
  - scripts/uat/human/runner/record_test.go
  - scripts/uat/human/runner/mcpclient_test.go
---

# Human UAT Rig

A person is the oracle for every mode.

## Why it exists

The existing connected UAT (`scripts/tests/browser/cat-33-connected-action-coverage.sh`)
invokes all 174 schema modes, but for 151 of them the pass condition is only "the
response is not an MCP error". 95 modes are touched by no other category, so nothing
anywhere asserts what they return. Worse, `framework.sh`'s `connected_fixture_url()`
hardcodes `interact.html`, so every mode is invoked against a page with no console
errors, no network activity, no websocket traffic, no long tasks and no layout shift.
`observe/errors` returns empty, `analyze/error_clusters` has nothing to cluster,
`observe/network_waterfall` is blank — and each one passes, because empty is not an
error.

Reachability is not behavior. A browser cannot tell you whether the answer was right;
a person looking at the page can.

## What it is

| Piece | State |
|---|---|
| Case inventory (`scripts/uat/human/cases.json`) | This change |
| Inventory contract (`scripts/contracts/humanuat`) | This change |
| Runner: present a case, capture a verdict, emit a machine-readable result | `kaboom-9ohw` |
| Release gate: no release without a complete run | `kaboom-3cju` |
| Coverage ratchet | `kaboom-hjbg` |
| Evidence bundle per case | `kaboom-vnqq` |
| Every FAIL becomes a failing automated test first | `kaboom-70n4` |

## The inventory

One case per shipped MCP mode plus one per user-facing surface that has no MCP mode at
all — popup toggles, the terminal panel, the side panel, draw mode, the supervision
overlay and its Stop button, the driven tab group, recording, keyboard shortcuts,
context menus, Doctor and first-run install.

Each case is a `setup` and a `question`:

```json
{
  "id": "observe/screenshot",
  "kind": "mcp_mode",
  "tool": "observe",
  "mode": "screenshot",
  "setup": "Take a screenshot of a page scrolled halfway down.",
  "question": "Does the image show what is on screen right now, and does its coordinate_frame let you click a button you can see in the image?"
}
```

The `setup` exists to create something to find. A mode run against a page with nothing
to report returns an empty result, and an empty result is not an error — which is the
exact hole this rig closes.

## Decisions

- **The denominator is derived, not maintained.** The contract reads
  `cmd/browser-agent/testdata/mcp-tools-list.golden.json` — the same document
  `tools/list` returns — so a mode added to the schema fails the build until somebody
  writes the question a person answers for it. A hand-kept second list would drift, and
  a denominator that can silently omit a mode measures nothing.
- **Non-MCP surfaces are hand-listed**, because nothing in the schema knows about them.
  That is precisely why they are the parts most likely to ship untested, so the list is
  in the contract itself rather than in the data file it checks.
- **Unfalsifiable questions are a build failure.** "Does it work?" is answered yes by a
  mode that returned nothing. The contract rejects a vocabulary of assertions that
  cannot come out NO ("works", "as expected", "successfully", "verify that"), rejects a
  question reused across two cases — two modes sharing a question are not being told
  apart — and rejects a setup too short to say what to do first.
- **A mode that cannot be judged by looking needs a stated observable proxy**, written
  into the question rather than improvised at run time: the file that appears, the value
  that changes on the next call, the behaviour that differs afterwards.

## Found while writing the inventory

Writing a falsifiable question per mode is itself a review of the surface. Two
discrepancies surfaced that no gate catches:

- `kaboom-abuj` (P1) — `configure/report_issue` files a real GitHub issue against the
  product repo when `operation: "submit"`, while its own mode spec says "nothing is
  submitted for you". An agent trusting the spec can file a public issue from a user's
  machine while exploring. The case for that mode deliberately exercises `preview` only.
- `kaboom-n3si` (P2) — the analyze reference documents a `history` mode the schema does
  not expose. `make verify-llm` checks schema → doc, not doc → schema, so a documented
  mode that does not exist costs every agent that reads the doc a call and a retry.

## Related

- `kaboom-nbxn` — the epic
- `kaboom-hikz` — the reachability-only sweep this supersedes; every mode listed there
  as sweep-only gets a real behavioral question here
- `kaboom-xcfs` — 1 of 34 UAT categories runs in CI

## Running it

```bash
make uat-human                 # every unanswered case, resumable
make uat-human FILTER=observe/ # one slice
make uat-human-list            # what is still unanswered
```

The runner drives `kaboom-mcp` from PATH — the binary a user's agent drives, not
the Go functions behind it — and appends one JSON record per case to
`uat-runs/<date>.jsonl`. Rerunning the same command skips what is already
answered, so a 194-case pass can be done over several sittings.

Each record carries the request sent, the response received, the person's
verdict and note, the evidence paths, and the build SHA that was judged. Two
runs of the same inventory diff line by line, so a regression appears as a
verdict flipping rather than as a reordered file.

Four answers, and no default: PASS, FAIL, BLOCKED, SKIP. An empty line is not a
pass — holding return through a run leaves every case unanswered rather than
green. FAIL and BLOCKED require a note, because a red case without one cannot be
turned into a regression test. SKIP is recorded but never counts as coverage.

Evidence (screenshot, console, network) is captured before and after every call
and beside every surface case. A probe that cannot run writes a `.error` file
saying why: an empty evidence directory is ambiguous between "capture failed"
and "capture was off", and those lead to opposite conclusions about a FAIL.
