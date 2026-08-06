---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Chrome Web Store Submission - Kaboom v5.4.0

## Package
**File:** `/Users/brenn/dev/kaboom/kaboom-extension-v5.4.0-webstore.zip` (319KB)

---

## Basic Information

**Extension Name:**
```
Kaboom
```

**Short Description:** (132 characters max)
```
Agentic Browser Devtools - rapid e2e web development. Stream console logs, network errors to Claude Code, Cursor, Copilot via MCP protocol.
```

**Detailed Description:**
```
Kaboom is a browser extension + local MCP server that streams real-time browser telemetry to AI coding assistants, enabling autonomous debugging and issue resolution.

🔥 What It Does
• Captures console logs, network errors, exceptions, WebSocket events
• Streams telemetry to AI agents via MCP protocol (Model Context Protocol)
• Enables AI-controlled browser automation for debugging
• Provides visual feedback when AI is actively debugging

⚙️ How It Works
1. Install extension and run local MCP server: npx kaboom-mcp
2. Connect your AI assistant (Claude Code, Cursor, Copilot, Windsurf, Zed)
3. Track a browser tab to start capturing telemetry
4. AI sees errors in real-time and can autonomously fix issues

🛡️ Privacy & Security
• 100% localhost-only - no data sent to external servers
• No SaaS, no cloud service, no telemetry
• All data stays on your machine
• Automatic redaction of passwords, API keys, tokens
• Open source (AGPL-3.0): github.com/brennhill/kaboom-agentic-browser-devtools-mcp

✨ Key Features
• MCP protocol integration (emerging standard for AI tooling)
• Async queue architecture (non-blocking browser control)
• Flickering flame favicon (visual indicator when AI is in control)
• Single-tab tracking (explicit opt-in, not全 tabs)
• Zero dependencies (pure Go backend, vanilla JS frontend)

👥 Perfect For
• Developers using AI coding assistants
• Teams adopting AI-first workflows
• Anyone debugging complex browser issues
• Engineers who want AI to fix bugs autonomously

📚 Documentation
• Setup guide: gokaboom.dev/getting-started
• Full docs: gokaboom.dev
• GitHub: github.com/brennhill/kaboom-agentic-browser-devtools-mcp

🔧 Technical Requirements
• Chrome or Brave browser
• Node.js 14+ (for running local MCP server)
• AI coding assistant with MCP support (Claude Code, Cursor, etc.)

🎯 Use Cases
• Autonomous bug fixing
• Real-time error monitoring
• Network debugging
• WebSocket traffic analysis
• Performance investigation
• AI-assisted development

⭐ Open Source
Licensed under AGPL-3.0. Contributions welcome!
```

---

## Category
```
Developer Tools
```

**Subcategories:**
```
Developer Tools > Testing
Developer Tools > Debugging
Productivity > Developer Tools
```

---

## Language
```
English (United States)
```

---

## Screenshots (1280x800 or 640x400)

**Required: 1-5 screenshots**

**Screenshot 1:** Extension popup showing tracking controls
- File: `docs/assets/images/chrome_store/listing/screenshot-1-popup.png` (TODO: Create)
- Caption: "Track a tab and enable AI Web Pilot for autonomous debugging"

**Screenshot 2:** Flickering flame favicon
- File: `docs/assets/images/chrome_store/listing/screenshot-2-flame.png` (TODO: Create)
- Caption: "Visual indicator shows when AI is actively controlling your browser"

**Screenshot 3:** AI debugging in action
- File: `docs/assets/images/chrome_store/listing/screenshot-3-ai-debug.png` (TODO: Create)
- Caption: "AI assistant sees console errors and network failures in real-time"

**Screenshot 4:** Privacy controls
- File: `docs/assets/images/chrome_store/listing/screenshot-4-privacy.png` (TODO: Create)
- Caption: "100% localhost-only - no data sent to external servers"

**Screenshot 5:** MCP integration
- File: `docs/assets/images/chrome_store/listing/screenshot-5-mcp.png` (TODO: Create)
- Caption: "Works with Claude Code, Cursor, Copilot, Windsurf, and any MCP-compatible assistant"

---

## Store Icon (128x128)
**File:** `extension/icons/store-icon-128.png`

---

## Promotional Tile (440x280) - Optional
**File:** `docs/assets/images/chrome_store/listing/promo-tile-440x280.png` (TODO: Create if submitting)

**Suggested design:**
- Kaboom flame logo
- Text: "AI-Native Browser Observability"
- Dark background with brand colors

---

## Small Promotional Tile (220x140) - Optional
**File:** `docs/assets/images/chrome_store/listing/promo-tile-small-220x140.png` (TODO: Create if submitting)

---

## Privacy Policy URL
```
https://gokaboom.dev/privacy/
```

**Note:** Ensure privacy policy page exists and covers:
- Data collection (console logs, network, etc.)
- Data storage (localhost only)
- No external transmission
- Sensitive data redaction
- User control (tracking toggle)

---

## Permissions Justification

**Chrome Web Store requires justification for each permission:**

### `tabs`
```
Required to:
• Query information about the tracked browser tab
• Send captured telemetry messages to the specific tracked tab
• Update tracking status when tabs close or navigate

Used for single-tab tracking feature where user explicitly selects which tab to monitor.
```

### `storage`
```
Required to:
• Persist user preferences (AI Web Pilot toggle, tracked tab ID, log level, etc.)
• Maintain settings across browser restarts
• Synchronize state between popup and background service worker

All storage is local to the user's browser (chrome.storage.local API).
```

### `alarms`
```
Required to:
• Schedule background polling to local MCP server (every 1-2 seconds)
• Maintain connection health checks
• Clean up stale data periodically

Used for periodic tasks that must run even when extension popup is closed.
```

### Host Permissions: `http://localhost/*`, `http://127.0.0.1/*`
```
Required to:
• Send captured browser telemetry to local MCP server running on localhost:7890
• Poll for browser automation commands from AI assistant
• Post results of AI-initiated browser actions

All communication is strictly localhost-only. No external network access requested.
The MCP server runs on the developer's own machine - no cloud service.
```

### Content Scripts: `<all_urls>`
```
Required to:
• Inject telemetry capture logic into the tracked tab's page context
• Observe console.log, console.error, and other browser events
• Capture network request/response data
• Listen for WebSocket events

Content scripts only activate on the single tab the user explicitly tracks.
This is a developer tool - broad access is necessary to debug any web application.
```

---

## Single Purpose Statement
```
Kaboom is a developer tool that captures browser telemetry (console logs, network errors, exceptions, WebSocket events) from a user-selected tab and streams it to AI coding assistants running locally via the MCP protocol, enabling autonomous debugging and issue resolution.
```

---

## Developer Notes (for Google Review Team)

```
## AI Web Pilot Feature

The extension includes an "AI Web Pilot" feature that uses `new Function()` to execute JavaScript in the page context on behalf of AI coding assistants.

### Justification:
• Feature is DISABLED by default
• Requires explicit user opt-in via toggle in extension popup
• Only executes code sent by AI assistant running on user's own machine
• Similar to browser DevTools console or extensions like Tampermonkey
• Essential for AI-assisted browser automation workflow

### Safety Mechanisms:
• "use strict" mode enforcement
• Timeout protection (5-30s max)
• CSP violation detection
• Graceful error handling
• User always in control (can disable anytime)

## Localhost-Only Architecture

All network communication is strictly localhost:
• MCP server runs on user's machine (port 7890)
• No external API calls
• No cloud service
• No data leaves user's network

Verify by inspecting:
• Host permissions: only localhost/127.0.0.1
• All fetch() calls in background.js target serverUrl (default: localhost:7890)
• Privacy policy confirms no external transmission

## Open Source

Full source code: https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp
License: AGPL-3.0
All compiled code matches source (TypeScript → JavaScript via tsc)
```

---

## Website
```
https://gokaboom.dev
```

---

## Support Email
```
support@gokaboom.dev
```
(Or your preferred support email)

---

## Distribution
```
Public
```

---

## Pricing
```
Free
```

---

## Submission Checklist

Before submitting:
- [ ] Privacy policy live at gokaboom.dev/privacy/
- [ ] Screenshots created (5 images, 1280x800)
- [ ] Store icon verified (128x128)
- [ ] Test installation from zip (unzip and load unpacked)
- [ ] Verify manifest.json version is 5.4.0
- [ ] Verify no .map or .d.ts files in zip
- [ ] Test all core features work
- [ ] Review permissions justifications above

---

## Expected Review Timeline
• Initial review: 1-3 business days
• If flagged for manual review: 5-7 business days
• Possible questions about `new Function()` usage

---

## Post-Approval

After approval:
- [ ] Update GitHub release with Web Store link
- [ ] Update gokaboom.dev with "Get on Chrome Web Store" button
- [ ] Announce on social media
- [ ] Add Web Store badge to README.md

---

## Files

**Submission Package:**
`/Users/brenn/dev/kaboom/kaboom-extension-v5.4.0-webstore.zip` (319KB)

**Manifest Version:** 5.4.0

**Permissions:**
- storage
- alarms
- tabs
- host_permissions: localhost only
- content_scripts: <all_urls>
