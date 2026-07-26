---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Privacy Justifications for Chrome Web Store

## Data Collection & Usage

### What data is collected?
**Developer tool telemetry from the browser tab you explicitly track:**
- Console logs (console.log, console.error, etc.)
- Network request/response data (URLs, status codes, headers, bodies)
- JavaScript exceptions and stack traces
- WebSocket events and messages
- User interactions (clicks, form submissions)
- Performance metrics
- DOM snapshots (when explicitly requested by AI)
- Screenshots (only when explicitly enabled in settings)

### Where does this data go?
**100% localhost only** - All data is sent exclusively to:
```
http://localhost:7890
```

The MCP server runs **on your own machine**. No cloud service, no external APIs, no remote servers.

**Verification:**
- Check extension manifest.json: `"host_permissions": ["http://localhost/*", "http://127.0.0.1/*"]`
- Check background.js: All `fetch()` calls use `serverUrl` (defaults to localhost:7890)
- Network tab: Filter by localhost - you'll only see local requests

### How is sensitive data protected?
**Automatic redaction of:**
- Password field values → `[redacted]`
- Credit card inputs → `[redacted]`
- SSN fields → `[redacted]`
- Authorization headers → `[redacted]`
- Cookie values → `[redacted]`
- API keys, tokens, secrets → `[redacted]`

**Implementation:**
- `extension/lib/page/serialize.js` (lines 107-134): Sensitive input detection
- `extension/lib/constants.js` (lines 10-17, 39): Header sanitization
- Checks input `type`, `autocomplete`, and `name` attributes
- Filters headers matching: `*token*`, `*secret*`, `*key*`, `*password*`

### Who can access this data?
**Only you** - The data exists in three places, all under your control:
1. **Browser memory** (extension buffers, cleared on close)
2. **MCP server memory** (ring buffers, cleared on restart)
3. **Local disk** (only if you use `--log-file` flag, stored at path you specify)

No one else can access this data. Not Google, not Kaboom developers, not third parties.

---

## Permission Justifications

### Why `tabs` permission?
**Purpose:** Track a specific browser tab and send messages to it.

**What we do:**
- `chrome.tabs.query()` - Find the tracked tab by ID
- `chrome.tabs.get()` - Get URL/title of tracked tab for display in popup
- `chrome.tabs.sendMessage()` - Send captured telemetry to the tracked tab's content script

**What we DON'T do:**
- ❌ Track all tabs automatically
- ❌ Read tabs you haven't explicitly selected
- ❌ Modify tabs without your permission

**User control:** You explicitly click "Track This Tab" to enable monitoring. One tab at a time.

### Why `storage` permission?
**Purpose:** Remember your settings across browser restarts.

**What we store (all in `chrome.storage.local`):**
- `trackedTabId` - Which tab you're monitoring
- `aiWebPilotEnabled` - Whether AI Web Pilot is ON/OFF
- `serverUrl` - Localhost server address (default: localhost:7890)
- `logLevel` - Which console levels to capture (error, warn, log, debug)
- `screenshotOnError` - Whether to auto-screenshot on errors
- `networkWaterfallEnabled` - Whether to capture request timing data

**What we DON'T store:**
- ❌ Your browsing history
- ❌ Passwords or credentials
- ❌ Personal information
- ❌ Captured telemetry (that goes to localhost server only)

**User control:** All settings have toggles in the extension popup.

### Why `alarms` permission?
**Purpose:** Background timers for polling localhost server.

**What we do:**
- Poll `http://localhost:7890/pending-queries` every 1-2 seconds
- Post captured events in batches (debounced to reduce requests)
- Health check connection to localhost server

**What we DON'T do:**
- ❌ Contact external servers
- ❌ Track time-based user behavior
- ❌ Send data on a schedule to anyone

**User control:** Polling only happens when extension is installed. Uninstall to stop.

### Why `host_permissions` for localhost?
**Purpose:** Send captured telemetry to your local MCP server.

**Requested hosts:**
- `http://localhost/*`
- `http://127.0.0.1/*`

**What we do:**
- POST captured console logs to `localhost:7890/logs`
- POST network data to `localhost:7890/network-body`
- GET pending commands from `localhost:7890/pending-queries`
- POST command results to `localhost:7890/dom-result`

**What we DON'T do:**
- ❌ Request access to any external websites
- ❌ Send data to any remote server
- ❌ Contact the internet

**Verification:** Open DevTools Network tab while Kaboom is running - you'll only see requests to localhost:7890.

### Why content script on `<all_urls>`?
**Purpose:** Capture telemetry from any web application being debugged.

**What we do:**
- Inject `content.bundled.js` into pages to observe console, network, etc.
- **Only on the ONE tab you explicitly track** (not all tabs)
- Capture errors, logs, network failures for AI debugging

**What we DON'T do:**
- ❌ Inject into tabs you haven't tracked
- ❌ Modify page content or behavior
- ❌ Track your browsing across sites
- ❌ Collect data from all tabs

**Why `<all_urls>`?** This is a **developer tool** - you need to debug any web application (localhost, staging, production, any domain). Restricting to specific domains would make Kaboom useless.

**User control:**
1. Explicitly click "Track This Tab" to start
2. Click "Stop Tracking" to stop
3. Only ONE tab is monitored at a time

---

## Privacy Principles

### 1. Explicit Opt-In
**You choose:**
- Which tab to track (not automatic)
- Whether to enable AI Web Pilot (default: OFF)
- Whether to capture screenshots (default: OFF)
- Whether to save logs to disk (default: OFF, in-memory only)

### 2. Localhost-Only Architecture
**Guarantee:**
- All communication stays on your machine
- No external network requests
- No cloud service, no SaaS
- No vendor lock-in

**How to verify:**
```bash
# Monitor network traffic while Kaboom runs
lsof -i -n -P | grep kaboom
# You'll only see localhost:7890
```

### 3. Automatic Sensitive Data Redaction
**Protected:**
- Passwords, API keys, tokens, secrets
- Credit card numbers, SSNs
- Authorization headers, cookies
- Any field matching common patterns

**How it works:**
- Input field detection (type="password", autocomplete="cc-number", etc.)
- Header filtering (Authorization, Cookie, X-API-Key, etc.)
- Automatic replacement with `[redacted]` before sending to localhost

### 4. Transparency
**Open source:**
- Full source code: <https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp>
- License: AGPL-3.0
- All code is readable (no obfuscation)
- Community can audit and verify privacy claims

**What you can inspect:**
- `extension/manifest.json` - Permissions requested
- `extension/background/server.js` - Where data is sent (localhost only)
- `extension/lib/page/serialize.js` - Sensitive data redaction
- `extension/lib/constants.js` - Header filtering rules

### 5. User Control
**You can:**
- Enable/disable tracking anytime
- Enable/disable AI Web Pilot anytime
- Toggle individual features (screenshots, network waterfall, etc.)
- Uninstall extension to stop all data collection
- Inspect all captured data (it's on your localhost server)

---

## Compliance

### GDPR Compliance
✅ **Compliant** - No personal data leaves user's device
- Data processor: You (data stays on your machine)
- Data controller: You (you decide what to capture and store)
- Right to deletion: Uninstall extension or restart MCP server
- Right to access: All data available via localhost API

### CCPA Compliance
✅ **Compliant** - Not applicable (no sale of personal information, no external collection)

### SOC 2 / Enterprise Requirements
✅ **Compliant:**
- Customer-controlled storage
- No third-party data processors
- Audit trail (correlation IDs for all commands)
- localhost-only architecture
- Open source (auditable)

---

## Privacy Policy Summary

**For gokaboom.dev/privacy/ page:**

```markdown
# Kaboom Privacy Policy

Last updated: February 2, 2026

## TL;DR
Kaboom never sends your data anywhere. Everything stays on your machine. No cloud, no external servers, no telemetry.

## What We Collect
When you use Kaboom, the browser extension captures:
- Console logs from the tab you track
- Network requests and responses
- JavaScript errors and exceptions
- WebSocket events
- User interactions (optional)
- Screenshots (optional, disabled by default)

## Where It Goes
100% localhost only. All data is sent to:
```
<http://localhost:7890>
```

The MCP server runs on YOUR machine. We (Kaboom developers) cannot access this data.

## What We Automatically Redact
Before sending to localhost:
- Passwords → [redacted]
- API keys, tokens, secrets → [redacted]
- Credit card numbers, SSNs → [redacted]
- Authorization headers, cookies → [redacted]

## What We Never Do
❌ Send data to external servers
❌ Upload to cloud
❌ Share with third parties
❌ Track you across websites
❌ Collect analytics or telemetry
❌ Require account creation

## Your Control
You decide:
- Which tab to track (explicit opt-in)
- Whether to enable AI Web Pilot (default: OFF)
- Whether to save logs to disk (default: OFF)
- What features to enable

You can uninstall anytime to stop all collection.

## Open Source
Full source code: https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp
License: AGPL-3.0
Community can audit our privacy claims.

## Contact
Questions? support@gokaboom.dev
```

---

## For Chrome Web Store Review Team

**Summary for reviewers:**

This extension:
- ✅ Only communicates with localhost (host_permissions verify this)
- ✅ Requires explicit user opt-in for tracking (not automatic)
- ✅ Redacts sensitive data automatically (see lib/serialize.js)
- ✅ Open source (fully auditable)
- ✅ No external network access
- ✅ No analytics, no telemetry, no cloud service

**The `<all_urls>` content script is necessary because:**
- This is a developer tool
- Developers need to debug any web application (localhost, staging, production)
- Restricting to specific domains would make the tool useless
- Content script only runs on the ONE tab user explicitly tracks

**The `new Function()` usage is for:**
- AI Web Pilot feature (browser automation by AI assistants)
- Disabled by default
- Requires explicit user toggle
- Similar to DevTools console or Tampermonkey
- Essential for AI-assisted debugging workflow

---

## User Trust Indicators

**In the extension:**
- ✅ Connection status shows "localhost:7890" (not a remote URL)
- ✅ "Track THIS Tab" button (explicit, not automatic)
- ✅ AI Web Pilot toggle (disabled by default, with warning)
- ✅ Each feature has clear description in popup

**In the documentation:**
- ✅ Setup guide shows `npx kaboom-mcp` (local server)
- ✅ Privacy policy emphasizes localhost-only
- ✅ GitHub README shows open source license
- ✅ Architecture diagrams show no external connections

**Competitive advantage:**
Unlike SaaS observability tools (Sentry, LogRocket), Kaboom keeps ALL data on your infrastructure. This is our key differentiator.
