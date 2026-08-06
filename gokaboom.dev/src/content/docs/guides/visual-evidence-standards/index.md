---
title: Visual Evidence Standards
description: Naming, alt text, and capture standards for screenshots and diagrams in KaBOOM docs.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'visual', 'evidence', 'standards']
---

Use this standard for every how-to, guide, and article.

## File Naming Convention

Use kebab-case with a stable pattern:

`<topic>-<step>-<artifact>-<yyyymmdd>.png`

Examples:

- `checkout-debug-step-1-console-errors-20260305.png`
- `api-validation-step-3-schema-diff-20260305.png`
- `security-audit-summary-diagram-20260305.svg`

## Alt Text Standard

Every image must have meaningful alt text.

Template:

`<Artifact type> showing <what changed or matters> for <task/context>.`

Examples:

- `Screenshot showing failed checkout request with 422 schema mismatch for cart update.`
- `Diagram showing triage flow from observe to analyze to generate in KaBOOM.`

## Required Visual Set per How-To

Minimum visual set for each how-to page:

1. One context screenshot (where the user starts).
2. One action screenshot (the key action or command).
3. One outcome screenshot (result, fix, or exported artifact).

## Diagram Standard

Use diagrams when sequence matters.

Required elements:

- Entry point
- 2-4 core steps
- Exit/output
- Tool calls shown as `observe`, `interact`, `analyze`, `generate`, or `configure`

## Storage and CDN

Store originals in the media repo, then publish optimized web formats to CDN.

- Master source: lossless PNG/SVG/WebM in media repo
- Delivery: optimized WebP/AVIF/SVG via CDN
- Keep docs repo references stable with absolute CDN URLs
