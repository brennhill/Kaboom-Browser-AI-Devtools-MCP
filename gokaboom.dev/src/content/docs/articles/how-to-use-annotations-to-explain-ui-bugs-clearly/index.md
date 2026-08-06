---
title: "How to Use Annotations to Explain UI Bugs Clearly"
description: "A beginner-friendly method for using visual annotations in KaBOOM Agentic Devtools to describe UI problems precisely."
date: 2026-03-05
authors: [brenn]
tags: [beginner, annotations, ui, debugging]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['beginner', 'annotations', 'ui', 'debugging', 'articles', 'use', 'explain', 'bugs', 'clearly']
---

Good bug reports are specific. Great bug reports are visual.

Annotations let you mark exactly what is wrong on the page.

<!-- more -->

## Quick Terms

- **UI (User Interface):** What users see and click.
- **Selector:** A way to point to a specific page element.

## Step 1: Open draw mode

Start annotation mode from your KaBOOM controls.

## Step 2: Mark each issue

For each problem:

1. Draw a box around the element
2. Write one short, concrete note
3. Repeat for every issue

Example note style:

- "Button label is cut off on mobile"
- "Error text overlaps input field"

## Step 3: Capture annotations as data

```js
analyze({what: "annotations"})
```

## Step 4: Send annotated context into your workflow

Use the annotation output to drive fixes, tests, or handoff notes.

## Step 5: Re-check after changes

Re-open the page and verify each annotation is resolved.

## Tips that make annotations much better

- One issue per annotation
- Write what is wrong, where, and expected behavior
- Avoid vague notes like "looks weird"

## Image and Diagram Callouts

> [Image Idea] Annotated page with 2-3 example callouts.

> [Diagram Idea] Annotation lifecycle: mark -> export -> fix -> verify.
