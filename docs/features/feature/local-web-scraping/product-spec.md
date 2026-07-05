---
feature: local-web-scraping
status: proposed
tool: interact
mode: scraping
version: 0.7.12
doc_type: product-spec
feature_id: feature-local-web-scraping
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Local Web Scraping

## Problem Statement

AI agents often need to scrape data from web applications where authentication is required (logged-in sessions, internal tools, enterprise apps). Traditional scraping tools can't access authenticated content. Agents need to leverage the user's active browser session to scrape data from pages they're already logged into, perform multi-step workflows (navigation, pagination, form submission), and extract structured data.

## Solution

Add scraping capabilities as a guided workflow pattern over existing tools. The agent uses `interact` actions (navigate, wait_for, execute_js) with `observe` (page/network state) and `analyze({what:"dom"})` snapshots to scrape data from authenticated sessions. No new MCP tool is required.

## Requirements

- Scrape data from pages using user's active authentication session
- Handle multi-page workflows (login → navigate → paginate → extract)
- Extract structured data via DOM queries or execute_js
- Handle dynamic content (wait for elements, AJAX loading)
- Support pagination (next button, infinite scroll)
- Respect rate limiting (configurable delays between requests)
- Export scraped data in JSON format via generate tool
- Maintain session cookies across multi-step workflows

## Out of Scope

- Bypassing CAPTCHA (requires human intervention)
- Scraping cross-origin iframes (security boundary)
- Distributed scraping (multiple tabs/browsers)
- Proxy/VPN configuration for scraping
- JavaScript rendering for headless scraping (already supported via active tab)

## Success Criteria

- Agent can log into app, navigate to dashboard, extract table data
- Agent can scrape paginated results (e.g., 100 pages of listings)
- Agent can handle AJAX-loaded content (wait for elements)
- Scraped data is structured, exported as JSON

## User Workflow

1. User manually logs into web app in Chrome
2. Agent observes current page: `observe({what: "page"})`
3. Agent navigates to target page: `interact({action: "navigate", url: "..."})`
4. Agent waits for content: `analyze({what: "dom", selector: ".data-table", wait: true})`
5. Agent extracts data: `interact({action: "execute_js", code: "return Array.from(document.querySelectorAll('tr')).map(r => r.innerText)"})`
6. Agent handles pagination: `interact({action: "execute_js", code: "document.querySelector('.next').click()"})`, repeat extraction
7. Agent persists extracted data with `configure({action: "store", store_action: "save", ...})` or returns structured data inline to the caller.

## Examples

### Scrape authenticated dashboard table:
```json
// Navigate to dashboard
interact({action: "navigate", url: "https://app.example.com/dashboard"})

// Wait for table to load
interact({action: "wait_for", selector: "table.data", timeout_ms: 5000})

// Extract table data
interact({action: "execute_js", code: `
  return Array.from(document.querySelectorAll('table.data tr')).map(row => ({
    id: row.cells[0].innerText,
    name: row.cells[1].innerText,
    status: row.cells[2].innerText
  }))
`})
```

### Handle pagination:
```json
// Scrape page 1, then loop
let allData = [];
for (let page = 1; page <= 10; page++) {
  // Extract current page
  let data = interact({action: "execute_js", code: "/* extract */"})
  allData.push(...data);
  
  // Click next
  interact({action: "execute_js", code: "document.querySelector('.next-page').click()"})
  
  // Wait for new content
  interact({action: "wait_for", selector: ".page-loading", timeout_ms: 2000})
}
```

### Export scraped data:
```json
configure({action: "store", store_action: "save", namespace: "scraping", key: "scraped_data", data: {"rows": allData}})
```

---

## Notes

- Not a new tool — uses existing interact, observe, analyze, and configure capabilities
- Leverages user's browser session (cookies, localStorage, auth tokens)
- Respects same-origin policy and CORS
- Agent responsible for rate limiting and ethical scraping practices
