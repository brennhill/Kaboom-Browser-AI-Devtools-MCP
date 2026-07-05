---
doc_type: qa_plan
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.8.1
last_verified_date: 2026-03-21
---

# QA Plan

## Automated Gates

1. `go test ./cmd/browser-agent -run Terminal -count=1`
2. `go test ./internal/pty/...`
3. `node --test tests/extension/sidepanel-terminal.test.js`
4. `npm run docs:check:strict`

## Manual Checks

1. Open the terminal side panel, minimize the terminal region, restore it, and close the browser side panel.
2. Verify redraw (`↻`) reloads the iframe without killing the session.
3. Type in terminal while annotation auto-write is triggered and confirm queued behavior.
4. Simulate WebSocket disconnect during queued submit and confirm submit resumes after reconnect.
5. Confirm terminal health reports `terminal_port` when running and `0` when unavailable.

## Regression Focus

- Focus theft while user is typing in xterm.
- Frame-write concurrency corruption under ping/output/control traffic.
- PTY session loss across page refresh.
- Main daemon stability when terminal bind/start fails.

## Linked Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- Feature Index: [index.md](./index.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Flow Maps:
  - [terminal-side-panel-host.md](../../../architecture/flow-maps/terminal-side-panel-host.md)
  - [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
