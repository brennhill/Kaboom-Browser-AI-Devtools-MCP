---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Full Stack Lab Demo Script (LLM + Kaboom)

## Goal
Show end-to-end Kaboom capabilities on a single demo surface:
- subtitles and action toasts
- usability review and element highlighting
- annotation flow with proposed UI fix
- websocket debugging (third-party payload mismatch)
- dependency vetting (unexpected external load)
- one-command runtime switch from broken to corrected behavior

## Local Setup
Run these in separate terminals:

```bash
# terminal 1: fake third-party websocket server
cd ~/dev/kaboom-site
node scripts/third-party-ws-server.mjs

# terminal 2: demo site
cd ~/dev/kaboom-site
npm run dev -- --host

# terminal 3: kaboom daemon
cd ~/dev/kaboom
./kaboom-agentic-browser --daemon
```

Demo URL:

```text
http://localhost:4321/demo/full-stack-lab.html
```

## Suggested Recording Flow

### 1) Intro + preflight
Prompt to LLM:

```text
Start a new demo recording. Verify Kaboom health. Then navigate to http://localhost:4321/demo/full-stack-lab.html.
Add subtitle: "Today we will debug usability, websocket contracts, and dependency safety in one pass."
```

### 2) Usability issue + highlight + annotation
Prompt to LLM:

```text
Highlight the oversized hero image element. Add subtitle:
"you said this page layout was not good. can you please annotate?"
Enable annotation mode, select the image, and annotate:
"this image is too big"
```

Expected target selector:

```text
#hero-product-image
```

### 3) Second annotation (desired size)
Prompt to LLM:

```text
Add another annotation on the same image with:
"the right size is this"
Set proposed image width to 420px and apply preview.
Narrate with subtitle: "the right size is this"
```

### 4) Websocket debugging (third-party contract mismatch)
Prompt to LLM:

```text
Connect to the websocket feed and inspect raw messages vs parsed notifications.
Explain why the socket URL is valid but UI parsing fails.
Add subtitle: "The socket endpoint is reachable, but payload contract is different from what this UI expects."
```

What to prove:
- websocket connects at `ws://localhost:8787/third-party-feed`
- raw messages arrive
- parsing fails in broken mode because UI expects `payload.message`
- server schema is third-party style (`event_name`, `data`)
- fix should happen in UI parser only

### 5) Dependency vetting
Prompt to LLM:

```text
Refresh dependency loading and review network requests for external scripts.
Add subtitle: "let me check what loads to vet dependencies for security"
Then ask: "Did you intend to load from mysite.xyz?"
```

What to prove:
- page attempts to load `https://mysite.xyz/sdk/browser-client-v3.js`
- this is externally sourced and should be explicitly approved

### 6) One-command post-fix switch (no file edit)
Prompt to LLM:

```text
Switch the demo from broken mode to post-fix mode in one command,
then explain what changed.
```

Preferred one-command call:

```text
interact({what:"execute_js", script:"window.kaboomDemo.applyPostFixMode()"})
```

What changes immediately:
- hero image width is constrained (layout fixed)
- websocket parser normalizes third-party payloads into UI notifications
- malformed/non-JSON messages are handled as warnings instead of crashing

### 7) Verification pass
Prompt to LLM:

```text
Reconnect websocket and verify parsed notifications now appear.
Confirm the hero image no longer breaks layout.
Keep subtitle concise and close recording.
```

### 8) Optional one-command revert (for A/B in same recording)
Prompt to LLM:

```text
Revert to broken mode in one command so we can compare behavior.
```

One-command call:

```text
interact({what:"execute_js", script:"window.kaboomDemo.applyBrokenMode()"})
```

## Runtime API for LLM
Available in page context:

```text
window.kaboomDemo.applyPostFixMode()
window.kaboomDemo.applyBrokenMode()
window.kaboomDemo.setMode("fixed" | "broken")
window.kaboomDemo.getMode()
```

## Optional Closeout Line

```text
"We did not touch the third-party websocket server. We adapted our UI contract safely, improved layout usability, and verified dependency intent."
```
