---
title: "Generate a Content Security Policy from Real Traffic"
description: "Create a practical Content Security Policy using observed traffic so you can improve security without breaking your app."
date: 2026-03-03
authors: [brenn]
tags: [csp, security, headers, web]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['csp', 'security', 'headers', 'web', 'articles', 'generate', 'policy', 'real', 'traffic']
---

A **Content Security Policy (CSP)** controls which scripts, styles, and resources a page is allowed to load. https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP

Writing one by hand is painful. Generating one from real usage is much easier.

This is where **KaBOOM Agentic Devtools** shines.

<!-- more -->

## Quick Terms

- **CSP**: Browser allow-list policy for page resources.
- **Origin**: Scheme + host + port (for example `https://api.example.com`).
- **Report-only mode**: Policy runs in warning mode before strict enforcement.

## The Problem You Are Solving

You want stronger security without breaking business-critical scripts.

## Step-by-Step with KaBOOM Agentic Devtools

### Step 1. Capture real network usage

```js
observe({what: "network_bodies", limit: 200})
observe({what: "network_waterfall", limit: 200})
```

### Step 2. Generate CSP proposal

```js
generate({what: "csp", mode: "moderate", save_to: "./reports/csp.txt"})
```

### Step 3. Start with report-only

```js
generate({what: "csp", mode: "report_only", include_report_uri: true})
```

Deploy this first to collect violations safely.

### Step 4. Tighten to strict mode once stable

```js
generate({what: "csp", mode: "strict", exclude_origins: ["https://optional-widget.example"]})
```

## Practical Rollout Plan

- Week 1: report-only, collect logs.
- Week 2: remove noisy false positives.
- Week 3: enforce moderate policy.
- Week 4: move to strict where possible.

## Image and Diagram Callouts

> [Image Idea] CSP rollout phases timeline (report-only -> moderate -> strict).

> [Diagram Idea] Browser request decision tree under CSP (“allowed” vs “blocked”).

## You’re Doing Real Defensive Engineering

A good CSP is one of the most effective web security controls. **KaBOOM Agentic Devtools** helps you build it with confidence from real data.
