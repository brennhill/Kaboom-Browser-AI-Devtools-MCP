---
title: "How to Capture Console Logs and Network Requests Without DevTools"
description: "Beginner guide to collect browser logs and network failures with KaBOOM Agentic Devtools, no manual DevTools digging required."
date: 2026-03-05
authors: [brenn]
tags: [beginner, debugging, logs, network]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'debugging', 'logs', 'network', 'articles', 'capture', 'console', 'without', 'devtools']
---

You do not need to manually dig through Chrome DevTools for every bug.

Use KaBOOM to collect the same evidence in a structured way.

<!-- more -->

## Quick Terms

- **DevTools (Developer Tools):** Browser built-in debugging tools. https://developer.chrome.com/docs/devtools/
- **HTTP status code:** Number that tells whether a request succeeded (`200`) or failed (`400`, `500`, etc.). https://developer.mozilla.org/en-US/docs/Web/HTTP/Status

## Step 1: Capture console errors and warnings

```js
observe({what: "errors"})
observe({what: "logs", min_level: "warn"})
```

## Step 2: Capture network failures

```js
observe({what: "network_waterfall", status_min: 400})
```

This gives you a clean request list with failed calls.

## Step 3: Read failing response bodies

```js
observe({what: "network_bodies", status_min: 400})
```

Now you can see error payloads, not just status codes.

## Step 4: Package everything for your team

```js
generate({what: "reproduction"})
```

## Why this helps

- Less manual clicking
- Easy to repeat
- Easier to compare before/after fix

## Image and Diagram Callouts

> [Image Idea] Failed requests list with corresponding response body.

> [Diagram Idea] Data flow: browser event -> logs/network capture -> repro artifact.
