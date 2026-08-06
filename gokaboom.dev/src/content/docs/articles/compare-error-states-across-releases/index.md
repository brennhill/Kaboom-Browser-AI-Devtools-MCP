---
title: "How to Compare Error States Across Releases"
description: "A friendly guide to checking whether a new release reduced or introduced browser errors using KaBOOM Agentic Devtools."
date: 2026-03-03
authors: [brenn]
tags: [releases, debugging, regression, quality]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['releases', 'debugging', 'regression', 'quality', 'articles', 'compare', 'error', 'states', 'across']
---

Releases should reduce problems, not hide them.

This article shows how to compare error behavior between two runs so you can answer:

“Did this release actually improve things?”

with confidence using *KaBOOM Agentic Devtools**.

<!-- more -->

## Quick Terms

- **Release**: A version of your app shipped to users.
- **Error state**: The set of errors your app throws during a flow.
- **Regression**: A new bug introduced by a change.

## The Problem You Are Solving

Without structured comparison, teams rely on feelings:

- “It seems better.”
- “I didn’t see anything this time.”

You need hard evidence.

## Step-by-Step with KaBOOM Agentic Devtools

### Step 1. Capture baseline run (before)

```js
configure({what: "recording_start"})
// run key flow
configure({what: "recording_stop", recording_id: "rec-before"})
```

### Step 2. Capture candidate run (after)

```js
configure({what: "recording_start"})
// run same flow on new release
configure({what: "recording_stop", recording_id: "rec-after"})
```

### Step 3. Compare error states directly

```js
configure({what: "log_diff", original_id: "rec-before", replay_id: "rec-after"})
observe({what: "log_diff_report", original_id: "rec-before", replay_id: "rec-after"})
```

### Step 4. Validate high-impact endpoints

```js
observe({what: "network_bodies", status_min: 400, limit: 40})
```

## What to Look For

- Error count went down (good).
- Old critical errors disappeared (great).
- New critical errors appeared (investigate now).
- Same error moved earlier/later in flow (timing clue).

## Image and Diagram Callouts

> [Image Idea] “Before vs After” error heatmap by page and severity.

> [Diagram Idea] Release comparison timeline with highlighted newly introduced errors.

## Smart Habit for Every Release

Pick 3 critical flows. Run this comparison every time. *KaBOOM Agentic Devtools** makes this lightweight enough to do regularly.
