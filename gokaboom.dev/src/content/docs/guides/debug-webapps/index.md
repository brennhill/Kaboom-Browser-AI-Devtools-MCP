---
title: Debug a Web App at Full Speed
description: Let your AI see everything — WebSocket messages, runtime errors, network failures, and screenshots — so it can diagnose bugs in seconds.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'debugging', 'webapps']
---

## Debugging is a Data Problem

When a bug hits, you open DevTools, flip between the Console, Network, and Elements tabs, copy-paste errors into your AI, and describe what you see. You're the bottleneck. You're manually shuttling context between the browser and the AI.

KaBOOM removes you from that loop. Your AI sees the browser directly — every console error, every network failure, every WebSocket message, even what the page looks like. You describe the bug once, and the AI has everything it needs.

## Step 1: Let the AI See the Errors

Start with the broad picture. Ask your AI to check for errors:

```js
observe({what: "errors"})
```

This returns deduplicated console errors with stack traces. But the real power is **error bundles** — pre-assembled debugging context for each error:

```js
observe({what: "error_bundles", window_seconds: 5})
```

Each bundle includes the error _plus_ the network requests, user actions, and console logs that happened within 5 seconds of it. It's like handing the AI a complete incident report instead of a single stack trace.

## Step 2: See WebSocket Traffic

Real-time apps (chat, dashboards, collaborative editors) run on WebSockets. Traditional debugging tools make these invisible. KaBOOM captures the full message stream.

Check active connections:

```js
observe({what: "websocket_status"})
```

See the messages flowing through:

```js
observe({what: "websocket_events", last_n: 20})
```

Filter by connection or direction:

```js
observe({what: "websocket_events", connection_id: "ws-3", direction: "incoming"})
```

Your AI can now see exactly what the server is pushing to the client — out-of-order messages, malformed payloads, dropped connections — all the things you'd normally miss.

## Step 3: Inspect Network Requests and Responses

See the full request waterfall:

```js
observe({what: "network_waterfall", limit: 30})
```

Drill into request and response bodies:

```js
observe({what: "network_bodies", url: "/api/users", status_min: 400})
```

That filter shows only failing API calls to `/api/users` — complete with request payload and error response. No more "what did the server return?" guessing.

## Step 4: Show the AI What You See

Sometimes the bug is visual — a layout broken, a spinner stuck, a modal behind an overlay. Let the AI see it:

```js
observe({what: "screenshot"})
```

The AI receives a screenshot of the current viewport. Combine this with the error data and the AI can correlate "the button is invisible" with "there's a z-index CSS error in the console."

<!-- Screenshot: A page with a visible bug (e.g., broken layout) alongside the observe screenshot output -->

## Step 5: Get the Full Timeline

For complex bugs where timing matters, pull the merged timeline:

```js
observe({what: "timeline"})
```

This interleaves errors, network requests, user actions, and console logs in chronological order. The AI sees _exactly_ what happened, in _exactly_ what order.

## Putting It Together: A Real Debugging Session

Here's what a typical exchange looks like:

**You:** "The dashboard is showing stale data after I switch teams. Can you figure out why?"

**AI checks errors:**
```js
observe({what: "error_bundles", window_seconds: 5})
```
> Found: `TypeError: Cannot read property 'id' of undefined` in `teamStore.js:47`, occurring right after a `PUT /api/teams/switch` that returned 200.

**AI checks the network response:**
```js
observe({what: "network_bodies", url: "/api/teams/switch"})
```
> The response body has `{team: null}` instead of the expected team object. The API returned 200 but with empty data.

**AI checks WebSocket:**
```js
observe({what: "websocket_events", last_n: 5, direction: "incoming"})
```
> The WebSocket subscription is still broadcasting data for the _old_ team. No re-subscribe happened after the team switch.

**AI diagnosis:** "The team switch API returns successfully but with a null team object. The WebSocket subscription isn't re-established after switching. The frontend crashes trying to read `.id` from the null team."

Three `observe` calls. Ten seconds. Root cause identified.

## Cut Through the Noise

Real browsers are noisy. Extension errors, analytics failures, third-party script warnings — they drown out the signal. Use noise filtering:

```js
configure({what: "noise_rule", noise_action: "auto_detect"})
```

KaBOOM scans your current errors and identifies the noise (extension errors, analytics, framework internals). After auto-detect, `observe({what: "errors"})` returns only the errors that matter.

You can also add manual rules:

```js
configure({what: "noise_rule", noise_action: "add",
           pattern: "analytics\\.google", reason: "GA noise"})
```

## Quick Reference

| What you need | Command |
|---|---|
| Console errors with context | `observe({what: "error_bundles"})` |
| WebSocket messages | `observe({what: "websocket_events"})` |
| Failed API calls | `observe({what: "network_bodies", status_min: 400})` |
| Visual state | `observe({what: "screenshot"})` |
| Full timeline | `observe({what: "timeline"})` |
| Performance metrics | `observe({what: "vitals"})` |
| Filter noise | `configure({what: "noise_rule", noise_action: "auto_detect"})` |

Your AI doesn't need you to copy-paste from DevTools anymore. Just point it at the browser and let it work.
