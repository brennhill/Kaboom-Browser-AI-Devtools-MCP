---
title: Annotation + Skills + Terminal Workflow
description: Use annotation sessions, guided skill playbooks, and the built-in terminal loop together so product feedback turns into tested fixes quickly.
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['guides', 'annotation', 'skill', 'terminal', 'workflow']
---

# Annotation + Skills + Terminal Workflow

This page walks through one practical loop:

1. Capture visual feedback with annotations.
2. Use skills to structure what to do next.
3. Validate and ship from the built-in terminal workflow.

> Terminal availability: this workflow is supported on macOS and Linux. The built-in terminal is currently unavailable on Windows.

## Step 1: Capture Feedback with Annotations

Use draw mode to mark real UI problems directly on the page.

- Start draw mode from the launcher or with `interact({what:'draw_mode_start'})`.
- Annotate layout, copy, and interaction issues.
- Pull all notes with `analyze({what:'annotations'})`.
- Drill into one issue with `analyze({what:'annotation_detail', correlation_id:'...'})`.

## Step 2: Route Work with Skills

Use KaBOOM skill playbooks to avoid guessing.

- `debug-triage` for broken behavior
- `ux-audit` for clarity and accessibility issues
- `regression-test` when a fix needs coverage

The output becomes a concrete fix list instead of vague feedback.

## Step 3: Close the Loop with Terminal Tasks

Run commands for verification and release readiness in the same workflow.

- Build and type checks
- Focused tests for touched behavior
- Evidence artifacts when needed

## Suggested Sequence

1. Annotate one page.
2. Group annotations by severity.
3. Run the matching skill for each group.
4. Implement the fix.
5. Validate with terminal + rerun annotations.

That gives product, design, and engineering one shared trail from observation to shipped fix.
