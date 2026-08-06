---
title: "How to Run Your First Browser Debug Session with KaBOOM"
description: "A beginner-friendly walkthrough for your first complete browser debugging session with KaBOOM Agentic Devtools."
date: 2026-03-05
authors: [brenn]
tags: [beginner, debugging, workflow]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'debugging', 'workflow', 'articles', 'run', 'first', 'browser', 'session']
---

This is the simplest full debug loop you can run with **KaBOOM Agentic Devtools**.

You will go from "something is broken" to "I can prove where it breaks."

<!-- more -->

## Quick Terms

- **Bug triage:** Quickly finding and classifying what is broken.
- **DOM (Document Object Model):** The structured page content your browser renders.

## Step 1: Open the page with the bug

Start on the exact page where the issue happens.

## Step 2: Capture baseline errors

```js
observe({what: "errors"})
observe({what: "logs", min_level: "warn"})
```

## Step 3: Reproduce the issue intentionally

Use explicit interaction steps:

```js
interact({what: "navigate", url: "https://your-app.example"})
```

Then click/type the steps that trigger the bug.

## Step 4: Check failing requests

```js
observe({what: "network_waterfall", status_min: 400})
observe({what: "network_bodies", status_min: 400})
```

## Step 5: Capture page evidence

```js
observe({what: "screenshot", full_page: true})
```

## Step 6: Generate a repro artifact

```js
generate({what: "reproduction"})
```

Now you have reproducible evidence for engineering, quality assurance (QA), or support.

## Image and Diagram Callouts

> [Image Idea] Side-by-side of errors, failing request, and screenshot evidence.

> [Diagram Idea] Debug loop: reproduce -> observe -> isolate -> artifact.
