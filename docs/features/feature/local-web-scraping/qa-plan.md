---
feature: local-web-scraping
doc_type: qa-plan
feature_id: feature-local-web-scraping
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Local Web Scraping

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** N/A (workflow pattern, no new code)

**Integration tests:** Scraping workflows
- [ ] Test navigate → extract data workflow
- [ ] Test paginated scraping (5 pages)
- [ ] Test wait for AJAX content before extraction
- [ ] Test session persistence across navigation
- [ ] Test JSON export of scraped data

**Edge case tests:** Error handling
- [ ] Test session expiration detection (login page appears)
- [ ] Test interact wait_for timeout on missing element
- [ ] Test extraction with malformed data

### Security/Compliance Testing

**Data leak tests:** Verify scraped data handling
- [ ] Test scraped data subject to redaction rules
- [ ] Test exported JSON doesn't include sensitive headers

#### Permission tests:
- [ ] Test scraping requires AI Web Pilot toggle enabled

---

## Human UAT Walkthrough

### Scenario 1: Scrape Authenticated Dashboard Table (Happy Path)
1. Setup:
   - Manually log into test app with dashboard containing data table
   - Enable AI Web Pilot toggle
2. Steps:
   - [ ] Verify logged in: `observe({what: "page"})` shows dashboard
   - [ ] Navigate: `interact({action: "navigate", url: "https://testapp.local/dashboard"})`
   - [ ] Wait for table: `analyze({what: "dom", selector: "table.data", wait: true})`
   - [ ] Extract data: `interact({action: "execute_js", code: "return Array.from(document.querySelectorAll('table.data tr')).map(r => r.innerText)"})`
   - [ ] Verify data returned (array of rows)
- [ ] Persist extracted data: `configure({action: "store", store_action: "save", namespace: "scraping", key: "extracted", data: {"rows": extracted}})`
3. Expected Result: Table data extracted and exported as JSON
4. Verification: Open exported JSON file, verify contains table data

### Scenario 2: Paginated Scraping (Multiple Pages)
1. Setup:
   - Test app with paginated results (10 items per page, 5 pages)
2. Steps:
   - [ ] Navigate to results page
   - [ ] Extract page 1 data
   - [ ] Click next: `interact({action: "execute_js", code: "document.querySelector('.next-page').click()"})`
- [ ] Wait for page 2 to load: `interact({action: "wait_for", selector: ".page-2", timeout_ms: 5000})`
   - [ ] Extract page 2 data
   - [ ] Repeat for pages 3-5
   - [ ] Aggregate all data
   - [ ] Export combined dataset
3. Expected Result: All 50 items scraped across 5 pages
4. Verification: Exported JSON contains 50 unique items

### Scenario 3: AJAX-Loaded Content
1. Setup:
   - Page with content loaded via AJAX after initial render
2. Steps:
   - [ ] Navigate to page
   - [ ] Attempt immediate extraction (should fail, content not loaded)
- [ ] Wait for AJAX: `interact({action: "wait_for", selector: ".ajax-loaded", timeout_ms: 5000})`
   - [ ] Extract data after wait
3. Expected Result: Data extracted only after AJAX completes
4. Verification: Extracted data is complete (not empty)

### Scenario 4: Session Expiration (Error Path)
1. Setup:
   - Log into app with short session timeout (or manually expire session)
2. Steps:
   - [ ] Start scraping workflow
   - [ ] Mid-scrape, session expires (manually clear cookies or wait for timeout)
   - [ ] Attempt navigation
   - [ ] Agent observes login page instead of expected page
3. Expected Result: Agent detects session expiration, returns error
4. Verification: Agent reports authentication failure, prompts re-login

### Scenario 5: Rate Limiting Handling
1. Setup:
   - Scrape site with aggressive rate limiting (or mock rate limit)
2. Steps:
   - [ ] Scrape multiple pages rapidly (no delay)
   - [ ] Receive 429 Too Many Requests error
   - [ ] Agent implements delay (2s between requests)
   - [ ] Resume scraping with delays
3. Expected Result: With delays, scraping succeeds
4. Verification: Network shows successful requests after implementing delays

---

## Regression Testing

- Test interact navigate/execute_js still work for non-scraping workflows
- Test analyze({what: "dom"}) still works for standard DOM snapshots
- Test configure store/load still works for non-scraping data

---

## Performance/Load Testing

- Test scrape 100 pages (should complete in ~5-10 minutes with rate limiting)
- Test extract large dataset (10,000 rows) in single page
- Verify no memory leaks during long scraping sessions
