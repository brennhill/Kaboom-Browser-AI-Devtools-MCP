---
feature: human-uat-rig
status: in_progress
tool: none
mode: n/a
doc_type: product-spec
feature_id: feature-human-uat-rig
last_reviewed: 2026-09-05
---

# Product Spec: Human UAT Rig

## Who it is for

The person cutting a release, and anyone who has to answer "does this actually work?"
about a build. Not an agent, and not CI on its own.

## The problem

Kaboom's automated UAT answers a narrower question than its name suggests: it proves
each of 174 modes can be reached and returns a well-formed envelope. It does not prove
any of them returns the right answer. A regression that made `observe/errors` return an
empty list for every page would pass every category in the suite today.

The people who notice that class of failure are users, after release.

## What a person gets

A run through a fixed list of cases, each one a setup and a single question they answer
YES or NO by looking at the browser, a file, or the operating system — never at the
tool's own output. A run produces a machine-readable result: which cases passed, which
failed, and what evidence was captured for each failure.

## What "good" looks like

- A tester who has never seen the codebase can run any case as written, without asking
  what a mode does or improvising a pass condition.
- Every NO is reproducible by someone else from the evidence bundle alone.
- The number of modes with no real question only goes down, and the count is checked by
  the build rather than asserted in a status update.

## Explicitly out of scope

- Replacing automated tests. Every FAIL becomes a failing automated test before it is
  fixed (`kaboom-70n4`); the rig finds what automation missed, it does not substitute
  for it.
- Timing or performance judgements. A person is a poor stopwatch; those stay in the
  automated budgets.
