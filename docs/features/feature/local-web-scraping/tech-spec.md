---
feature: local-web-scraping
status: proposed
doc_type: tech-spec
feature_id: feature-local-web-scraping
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Local Web Scraping

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Local web scraping is a workflow pattern, not a new technical feature. It uses existing tools (`interact` for navigation/execution/waits, `observe` for captured state, `analyze` for DOM snapshots, and `configure` store/load for persistence) orchestrated by the AI agent. The key insight: the user's active browser session provides authentication while the extension provides telemetry and control.

## Key Components

- **Session persistence**: Browser maintains cookies, localStorage, sessionStorage across agent operations
- **Navigate action**: Move between pages within authenticated session
- **Execute_js action**: Extract data via custom JavaScript in page context
- **wait_for action**: Wait for dynamic content to load before extraction
- **Store action**: Persist scraped data via configure store when needed

## Data Flows

```
User logs in manually (session established)
  → Agent: observe({what: "page"}) — confirms logged in
  → Agent: interact({what: "navigate", url: "/data-page"})
  → Agent: interact({what: "wait_for", selector: ".data-table", timeout_ms: 5000})
  → Agent: interact({what: "execute_js", code: "/* extract table */"})
  → Agent receives extracted data
  → Agent: configure({what: "store", store_action: "save", key: "...", data: {...}})
  → Server stores structured data in session store
```

## Implementation Strategy

**No new server/extension code required.** This is a documentation and workflow pattern feature.

### Agent workflow for scraping:
1. Verify authentication: Use observe({what: "page"}) to check for login indicators
2. Navigate to target: Use interact({what: "navigate"})
3. Wait for content: Use interact({what: "wait_for"}) for AJAX-loaded elements, then optionally snapshot with analyze({what:"dom"})
4. Extract data: Use interact({what: "execute_js"}) with custom extraction code
5. Handle pagination: Loop with click next + wait for content
6. Rate limit: Add delays between requests (agent responsibility)
7. Persist/export: Use configure store/load or return extracted data inline

### Session handling:
- Browser automatically maintains cookies across navigation
- No special session management needed
- If session expires, agent detects (login page appears), alerts user

### Dynamic content handling:
- Use interact({what: "wait_for"}) to detect when elements appear
- Use execute_js to poll for AJAX completion indicators
- Set appropriate timeouts (2-10s) for slow-loading content

## Edge Cases & Assumptions

- **Edge Case 1**: Session expires mid-scrape → **Handling**: Agent detects login page, returns error, user must re-authenticate
- **Edge Case 2**: AJAX content never loads → **Handling**: wait_for times out, agent handles error
- **Edge Case 3**: Pagination changes URL structure → **Handling**: Agent detects URL pattern, adjusts navigation
- **Edge Case 4**: Rate limiting by target site → **Handling**: Agent must implement delays, handle 429 errors
- **Assumption 1**: User manually authenticates before scraping starts
- **Assumption 2**: Target site allows scraping (ethical/legal responsibility)

## Risks & Mitigations

- **Risk 1**: Agent scrapes too fast, triggers rate limits → **Mitigation**: Document rate limiting best practices
- **Risk 2**: Session expires, scrape fails → **Mitigation**: Agent checks authentication before each operation
- **Risk 3**: Dynamic content timing issues → **Mitigation**: Use wait_for with bounded timeout/retry
- **Risk 4**: Violating site's ToS → **Mitigation**: User responsibility, document ethical scraping guidelines

## Dependencies

- Existing interact tool (navigate, execute_js)
- Existing analyze tool (`what:"dom"`)
- Existing configure tool (`store` / `load`)
- Browser session management (cookies, storage)

## Performance Considerations

- Navigation: 1-5s per page load
- Data extraction: <500ms per page (execute_js is fast)
- Large datasets: Agent should batch, avoid memory issues
- Rate limiting: Agent should add 1-2s delays between pages

## Security Considerations

- Scraping uses user's authenticated session (user's permissions apply)
- No elevation of privilege
- Agent cannot bypass authentication (requires manual login)
- Scraped data subject to same redaction rules as logs
- User responsible for securing exported JSON files
- Cannot scrape cross-origin content (same-origin policy enforced)
