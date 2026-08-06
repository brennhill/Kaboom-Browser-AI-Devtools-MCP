---
doc_type: reference
status: draft
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Articles Image and Diagram Backlog (2026-03-03)

## Purpose

Consolidated visual callouts for new articles so we can triage design/illustration work later.

## Backlog

| Priority | Article Slug | Visual Type | Requested Visual |
| --- | --- | --- | --- |
| P1 | /blog/detect-api-contract-drift-before-production/ | Diagram | Validation flow: app action -> observe network bodies -> analyze API validation -> generate regression test |
| P1 | /blog/detect-api-contract-drift-before-production/ | Image | Before/after JSON response with changed field highlighted |
| P1 | /blog/debug-websocket-real-time-apps/ | Diagram | Timeline showing incoming message followed by error milliseconds later |
| P2 | /blog/debug-websocket-real-time-apps/ | Image | WebSocket status panel with state, close code, message counts |
| P1 | /blog/fix-login-redirect-loops-and-session-bugs/ | Diagram | Redirect chain ladder from login to callback back to login |
| P2 | /blog/fix-login-redirect-loops-and-session-bugs/ | Image | Cookie/session comparison table: broken vs working |
| P1 | /blog/reproduce-works-locally-fails-in-ci-browser-bugs/ | Diagram | CI failure loop: record -> reproduce -> replay -> fix -> compare |
| P2 | /blog/reproduce-works-locally-fails-in-ci-browser-bugs/ | Image | Annotated evidence timeline screenshot |
| P1 | /blog/compare-error-states-across-releases/ | Image | Before/after release error heatmap by severity |
| P2 | /blog/compare-error-states-across-releases/ | Diagram | Release timeline with newly introduced errors marked |
| P1 | /blog/catch-third-party-script-regressions-fast/ | Image | Network waterfall with third-party bottlenecks highlighted |
| P2 | /blog/catch-third-party-script-regressions-fast/ | Diagram | First-party vs third-party load budget split |
| P1 | /blog/generate-playwright-tests-from-real-user-sessions/ | Image | Recorded flow panel mapped to generated Playwright file |
| P2 | /blog/generate-playwright-tests-from-real-user-sessions/ | Diagram | Generate-test loop for regression protection |
| P1 | /blog/visual-regression-testing-with-annotation-sessions/ | Image | Annotated UI screenshot with labeled visual defects |
| P2 | /blog/visual-regression-testing-with-annotation-sessions/ | Diagram | Annotation-to-test pipeline |
| P1 | /blog/build-reusable-qa-macros-with-batch-sequences/ | Image | Macro library view with tags and last-run info |
| P2 | /blog/build-reusable-qa-macros-with-batch-sequences/ | Diagram | Author once, replay many times workflow |
| P1 | /blog/run-accessibility-audits-in-ci-and-export-sarif/ | Diagram | CI stage with accessibility scan and SARIF export |
| P2 | /blog/run-accessibility-audits-in-ci-and-export-sarif/ | Image | SARIF example with plain-language explanation |
| P1 | /blog/debug-broken-forms-labels-aria-validation/ | Image | Good vs bad label-input linkage example |
| P2 | /blog/debug-broken-forms-labels-aria-validation/ | Diagram | Form submit lifecycle from input to feedback |
| P1 | /blog/annotation-driven-ux-reviews-for-engineering-teams/ | Diagram | Feedback lifecycle: annotate -> issue -> implement -> verify |
| P2 | /blog/annotation-driven-ux-reviews-for-engineering-teams/ | Image | Annotated homepage review screenshot |
| P1 | /blog/core-web-vitals-regression-triage/ | Image | LCP/INP/CLS before-after mini dashboard |
| P2 | /blog/core-web-vitals-regression-triage/ | Diagram | Fix-impact ladder for user-perceived speed |
| P1 | /blog/identify-render-blocking-assets-and-slow-routes/ | Image | Render-blocking waterfall with highlighted requests |
| P2 | /blog/identify-render-blocking-assets-and-slow-routes/ | Diagram | Route performance map with color-coded load times |
| P1 | /blog/security-audit-for-browser-workflows/ | Image | Security finding matrix by severity and area |
| P2 | /blog/security-audit-for-browser-workflows/ | Diagram | Secure request path and risk checkpoints |
| P1 | /blog/generate-csp-policy-from-real-traffic/ | Diagram | CSP rollout phases: report-only -> moderate -> strict |
| P2 | /blog/generate-csp-policy-from-real-traffic/ | Diagram | Browser allow/block decision tree under CSP |
| P1 | /blog/prevent-credential-and-pii-leaks-during-debugging/ | Diagram | Data safety pipeline: capture -> sanitize -> review -> share |
| P2 | /blog/prevent-credential-and-pii-leaks-during-debugging/ | Image | Safe vs unsafe debugging artifact examples |
| P1 | /blog/api-validation-for-frontend-teams/ | Diagram | Frontend API validation loop from traffic to tests |
| P2 | /blog/api-validation-for-frontend-teams/ | Image | Endpoint contract pass/fail matrix |
| P1 | /blog/use-mcp-for-browser-aware-debugging/ | Diagram | MCP debugging flow from prompt to verified fix |
| P2 | /blog/use-mcp-for-browser-aware-debugging/ | Image | Before/after workflow comparison (manual vs MCP-assisted) |
| P1 | /blog/mcp-tools-vs-traditional-test-runners/ | Diagram | Hybrid model: MCP discovery + test runner enforcement |
| P2 | /blog/mcp-tools-vs-traditional-test-runners/ | Image | Decision matrix for workflow choice |
| P1 | /blog/claude-code-kaboom-fast-bug-triage-setup/ | Diagram | Claude triage loop with Kaboom tool calls |
| P2 | /blog/claude-code-kaboom-fast-bug-triage-setup/ | Image | MCP config screen snippet |
| P1 | /blog/cursor-kaboom-interactive-web-development/ | Diagram | Interactive dev loop: edit -> inspect -> fix -> verify |
| P2 | /blog/cursor-kaboom-interactive-web-development/ | Image | Cursor assistant panel with observe/generate calls |
| P1 | /blog/local-first-demo-recording-for-product-teams/ | Diagram | Demo production flow: record -> review -> replay -> export |
| P2 | /blog/local-first-demo-recording-for-product-teams/ | Image | Demo chapter timeline with key moments |
| P1 | /blog/production-debugging-runbook-with-kaboom/ | Diagram | Incident runbook flowchart from alert to verified fix |
| P2 | /blog/production-debugging-runbook-with-kaboom/ | Image | Incident board screenshot with status columns |

## Notes

- Start with all **P1** visuals first; they unblock understanding of core workflows.
- Keep a consistent visual language (same icon set, colors, and annotation style) across the full article set.
