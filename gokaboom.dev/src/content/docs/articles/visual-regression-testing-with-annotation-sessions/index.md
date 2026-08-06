---
title: "Visual Regression Testing with Annotation Sessions"
description: "Use visual annotations to build clear, repeatable visual regression checks with KaBOOM Agentic Devtools."
date: 2026-03-03
authors: [brenn]
tags: [visual-regression, annotations, ui, testing]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['visual-regression', 'annotations', 'ui', 'testing', 'articles', 'visual', 'regression', 'annotation', 'sessions']
---

Visual bugs are often obvious to humans and hard to describe to automation.

Annotation sessions solve this by letting people mark what changed, then turning that into repeatable checks with **KaBOOM Agentic Devtools**.

<!-- more -->

## Quick Terms

- **Visual regression**: A visual change you did not intend.
- **Annotation**: A marked area with notes on what looks wrong.
- **Baseline**: The known-good visual version used for comparison.

## The Problem You Are Solving

You want your team to say:

“This button shift will never surprise us in production again.”

## Step-by-Step with KaBOOM Agentic Devtools

### Step 1. Capture visual feedback with draw mode

```js
interact({what: "draw_mode_start", annot_session: "checkout-visual-review", wait: true})
```

Team members mark issues directly on the page.

### Step 2. Retrieve annotations

```js
analyze({what: "annotations", annot_session: "checkout-visual-review"})
```

### Step 3. Generate a visual test from annotation data

```js
generate({what: "visual_test", annot_session: "checkout-visual-review", test_name: "checkout-visual-regression"})
```

### Step 4. Compare against baseline after changes

```js
analyze({what: "visual_baseline", name: "checkout-baseline"})
analyze({what: "visual_diff", baseline: "checkout-baseline", threshold: 30})
```

## Why This Works for Mixed Teams

Product, design, and engineering can all contribute without writing code first. Annotations keep intent clear.

## Image and Diagram Callouts

> [Image Idea] Annotated screenshot with boxes and short labels (“text cut off”, “button moved”).

> [Diagram Idea] Human annotation -> machine-generated visual test -> automated compare.

## You’re Blending Human Taste with Automation

That is cutting-edge quality practice. **KaBOOM Agentic Devtools** helps your team move from “I think it looks off” to “we now test this every release.”
