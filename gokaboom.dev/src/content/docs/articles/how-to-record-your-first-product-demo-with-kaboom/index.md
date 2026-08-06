---
title: "How to Record Your First Product Demo with KaBOOM"
description: "Step-by-step beginner guide to recording a clean, repeatable product demo workflow with KaBOOM Agentic Devtools."
date: 2026-03-05
authors: [brenn]
tags: [beginner, demos, recording]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'demos', 'recording', 'articles', 'record', 'first', 'product', 'demo', 'kaboom']
---

A great demo should be repeatable, not stressful.

Here is a clean first setup using **KaBOOM Agentic Devtools**.

<!-- more -->

## Step 1: Choose a narrow demo goal

Pick one simple story, for example:

- "Create a new project"
- "Find and fix one bug"
- "Run one automated check"

## Step 2: Start recording

Use KaBOOM recording controls to begin a session.

## Step 3: Follow a script, not improvisation

Keep it short:

1. Open the page
2. Show the problem or task
3. Show the action
4. Show the result

## Step 4: Capture proof during the demo

```js
observe({what: "errors"})
observe({what: "network_waterfall", status_min: 400})
```

This proves the demo reflects real runtime behavior.

## Step 5: Stop recording and save

Keep filename and notes clear so teammates can replay context quickly.

## Bonus: generate a reproducible script

```js
generate({what: "reproduction"})
```

Now your demo doubles as a technical artifact.

## Image and Diagram Callouts

> [Image Idea] Demo timeline with key moments marked.

> [Diagram Idea] Demo structure: context -> action -> proof -> outcome.
