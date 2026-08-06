---
title: "Reproduce ‘Works Locally, Fails in CI’ Browser Bugs"
description: "A practical walkthrough for reproducing browser bugs that appear in Continuous Integration but not on local machines."
date: 2026-03-03
authors: [brenn]
tags: [ci, debugging, testing, regression]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['ci', 'debugging', 'testing', 'regression', 'articles', 'reproduce', 'works', 'locally', 'fails', 'browser', 'bugs']
---

“Works on my machine” is not a joke. It is a real mismatch between environments.

**Continuous Integration (CI)** means automated checks run on a shared server for every code change. CI overview: https://en.wikipedia.org/wiki/Continuous_integration

This guide shows how to make CI-only browser bugs reproducible with *KaBOOM Agentic Devtools**.

<!-- more -->

## Quick Terms

- **CI**: Automated build/test pipeline.
- **Reproduction**: A repeatable sequence that triggers the bug.
- **Regression**: A bug that appears after a change that used to work.

## The Problem You Are Solving

You want one deterministic answer:

“What exact browser sequence fails in CI, and how do we replay it locally?”

## Step-by-Step with KaBOOM Agentic Devtools

### Step 1. Capture the failing flow

```js
configure({what: "recording_start"})
// run the scenario
configure({what: "recording_stop", recording_id: "rec-ci-fail"})
```

### Step 2. Export a reproduction script

```js
generate({what: "reproduction", save_to: "./tmp/ci-repro.spec.ts", include_screenshots: true})
```

Now you have concrete evidence, not memory.

### Step 3. Replay and inspect logs + network

```js
configure({what: "playback", recording_id: "rec-ci-fail"})
observe({what: "errors", limit: 50})
observe({what: "network_bodies", status_min: 400, limit: 30})
```

### Step 4. Compare before and after the fix

```js
configure({what: "log_diff", original_id: "rec-ci-fail", replay_id: "rec-ci-fixed"})
```

## Why This Works

You are removing ambiguity:

- same steps,
- same timing,
- same evidence trail.

That is exactly what flaky bug hunting needs.

## Image and Diagram Callouts

> [Diagram Idea] “CI failure loop”: failing run -> record -> generate reproduction -> replay -> fix -> compare.

> [Image Idea] Annotated timeline screenshot with error + network failure + user action.

## Final Nudge

If you can reproduce the issue, you can fix the issue. *KaBOOM Agentic Devtools** helps you turn “mystery CI failure” into a clear, teachable workflow.
