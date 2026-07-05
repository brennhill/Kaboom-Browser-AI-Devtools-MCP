---
feature: Tab Recording
status: proposed
tool: interact, observe
mode: screen_recording_start, screen_recording_stop, saved_videos
version: 0.7.12
doc_type: product-spec
feature_id: feature-tab-recording
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Tab Recording

## Problem Statement

### Developers waste time communicating visual bugs and demonstrating features.

Today's workflow for sharing what happened in a browser:

1. **Screen recording tools** (Loom, OBS) — separate app, separate upload, separate link. Context is lost: no console errors, no network data, no performance metrics alongside the video.
2. **Screenshots** — static, miss the sequence of events, require manual annotation.
3. **Text descriptions** — "click the button, then the form breaks" — ambiguous, incomplete, back-and-forth.

**Result:** Bug reports take 3 rounds of clarification. Product demos require a separate tool. QA handoffs lose context.

---

## Solution

**Tab Recording** captures the browser tab as a WebM video, triggered by the AI via MCP or manually from the extension popup.

### The workflow:

1. Start recording (AI says `interact({action: "screen_recording_start", name: "checkout bug"})` or user clicks Record in popup)
2. Interact with the page normally — Kaboom's subtitles and action toasts are captured in the video automatically
3. Stop recording → video saved to `~/.kaboom/recordings/` with a metadata sidecar file
4. AI can list recordings via `observe({what: "saved_videos"})` and reference them by name

### Why this belongs in Kaboom:

- **Subtitles and toasts are already in-page** — they appear in the recorded video for free, providing narration without extra tooling
- **Zero extra apps** — no Loom, no OBS, no browser extension conflicts
- **AI-controlled** — the AI can start/stop recording as part of a test flow or debugging session
- **Paired with telemetry** — the video sits alongside console logs, network data, and performance metrics already in Kaboom

---

## User Workflows

### Workflow 1: AI-Triggered Recording (Bug Reproduction)

```
1. Developer: "Record yourself reproducing the checkout bug"
2. AI calls interact({action: "screen_recording_start", name: "checkout bug repro"})
3. AI navigates to checkout, fills form, triggers the bug
4. Kaboom subtitles narrate each step in the video
5. AI calls interact({action: "screen_recording_stop"})
6. Video saved: ~/.kaboom/recordings/checkout-bug-repro--2026-02-07-1423.webm
7. Developer shares the video — no Loom needed
```

### Workflow 2: Manual Recording (Product Demo)

```
1. User clicks "Record" button in extension popup
2. Popup shows recording indicator with elapsed time
3. User walks through the product feature
4. User clicks "Stop" in popup
5. Video saved with auto-generated name: recording--2026-02-07-1430.webm
6. User shares the file
```

### Workflow 3: AI Lists Saved Videos

```
1. Developer: "What recordings do we have?"
2. AI calls observe({what: "saved_videos"})
3. Returns:
   - checkout-bug-repro--2026-02-07-1423.webm (2m 34s, 18MB)
   - site-demo--2026-02-07-1430.webm (5m 12s, 41MB)
   - login-flow--2026-02-06-0900.webm (1m 05s, 7MB)
4. Developer: "Show me the checkout one" → AI references the file path
```

### Workflow 4: Named Recording with AI Narration

```
1. Developer: "Record a demo of the new dashboard, narrate what you're doing"
2. AI calls interact({action: "screen_recording_start", name: "dashboard demo"})
3. AI navigates and uses subtitle to narrate:
   interact({action: "navigate", url: "/dashboard", subtitle: "Opening the new dashboard"})
   interact({action: "click", selector: "text=Analytics", subtitle: "Clicking into analytics view"})
4. Subtitles render in the page → captured in the video
5. AI calls interact({action: "screen_recording_stop"})
6. Result: a narrated product demo video, no Loom, no voiceover needed
```

---

## Core Requirements

### R1: MCP-Triggered Recording

#### Start:

- [ ] `interact({action: "screen_recording_start"})` — starts recording the active tab at default 15fps
- [ ] `interact({action: "screen_recording_start", name: "checkout bug"})` — starts with a user-provided name
- [ ] `interact({action: "screen_recording_start", name: "smooth demo", fps: 30})` — starts at specified framerate
- [ ] `fps` parameter: optional, default `15`. Valid range: `5`–`60`.
  - `5` — minimal CPU, ~2MB/min (static content, bug evidence)
  - `15` — default, ~4MB/min (bug repros, most demos)
  - `30` — smooth, ~8MB/min (polished product demos)
  - `60` — high fidelity, ~15MB/min (animation/transition debugging)
- [ ] Returns: `{status: "recording", name: "checkout-bug--2026-02-07-1423", path: "~/.kaboom/recordings/checkout-bug--2026-02-07-1423.webm", fps: 15}`
- [ ] If already recording, returns error: `"RECORD_START: Already recording. Stop current recording first."`

#### Stop:
- [ ] `interact({action: "screen_recording_stop"})` — stops recording, saves video + metadata
- [ ] Returns: `{status: "saved", name: "checkout-bug--2026-02-07-1423", path: "...", duration_seconds: 154, size_bytes: 18400000}`
- [ ] If not recording, returns error: `"RECORD_STOP: No active recording."`

#### Name Resolution:
- [ ] User provides name → sanitize to filesystem-safe slug (lowercase, hyphens, no special chars)
- [ ] Append `--{ISO8601-short}` timestamp: `checkout-bug--2026-02-07-1423`
- [ ] No name provided → `recording--2026-02-07-1423`

### R2: Manual Recording from Popup

#### UI:
- [ ] New "Record" section in popup (below existing toggles)
- [ ] Button: "Start Recording" (red circle icon)
- [ ] While recording:
  - Button changes to "Stop Recording" (red square icon)
  - Elapsed time displayed: `0:00`, `0:01`, ... `2:34`
  - Optional: name input field (empty = auto-name)
- [ ] Recording state synced to `chrome.storage.local` so popup reopening shows correct state

#### Behavior:
- [ ] Popup start → triggers same recording pipeline as MCP `screen_recording_start`
- [ ] Popup stop → triggers same pipeline as MCP `screen_recording_stop`
- [ ] If MCP starts a recording, popup shows recording state
- [ ] If popup starts a recording, MCP `screen_recording_stop` can stop it (and vice versa)
- [ ] Single source of truth: one shared recording state

### R3: Video Listing

#### MCP API:
- [ ] `observe({what: "saved_videos"})` — list all recordings
- [ ] `observe({what: "saved_videos", url: "checkout"})` — filter by name substring
- [ ] `observe({what: "saved_videos", last_n: 5})` — most recent N recordings

#### Response:
```json
{
  "recordings": [
    {
      "name": "checkout-bug-repro--2026-02-07-1423",
      "file": "checkout-bug-repro--2026-02-07-1423.webm",
      "path": "/Users/brenn/.kaboom/recordings/checkout-bug-repro--2026-02-07-1423.webm",
      "created_at": "2026-02-07T14:23:00Z",
      "duration_seconds": 154,
      "size_bytes": 18400000,
      "url": "https://myapp.com/checkout"
    }
  ],
  "total": 3,
  "storage_used_bytes": 66400000
}
```

#### Implementation:
- [ ] Server globs `~/.kaboom/recordings/*.webm`
- [ ] Reads sidecar `*_meta.json` for each video
- [ ] Returns sorted by `created_at` descending (newest first)

### R4: File Storage

**Directory:** `~/.kaboom/recordings/` (created on first recording)

#### Files per recording:
```
~/.kaboom/recordings/
  checkout-bug-repro--2026-02-07-1423.webm        # video
  checkout-bug-repro--2026-02-07-1423_meta.json    # metadata sidecar
```

#### Metadata sidecar format:
```json
{
  "name": "checkout-bug-repro--2026-02-07-1423",
  "display_name": "checkout bug repro",
  "created_at": "2026-02-07T14:23:00Z",
  "duration_seconds": 154,
  "size_bytes": 18400000,
  "url": "https://myapp.com/checkout",
  "tab_id": 12345,
  "resolution": "1920x1080",
  "format": "video/webm;codecs=vp8"
}
```

#### Naming convention:
- `{slug}--{YYYY-MM-DD-HHmm}.webm`
- `{slug}--{YYYY-MM-DD-HHmm}_meta.json`
- Slug: user-provided name sanitized (lowercase, spaces→hyphens, strip special chars)
- Default slug when no name: `recording`

### R5: Subtitle & Toast Capture

- [ ] Kaboom subtitles (`interact({action: "subtitle"})`) render as DOM elements in the page
- [ ] Action toasts (from `reason` param) render as DOM elements in the page
- [ ] `chrome.tabCapture` captures the rendered tab content, including these DOM overlays
- [ ] No extra work needed — subtitles appear in the video automatically
- [ ] This is the key differentiator: AI-narrated recordings without voiceover

### R6: Recording Indicator

#### In popup:
- [ ] Red dot + "Recording" label + elapsed time when active
- [ ] Visible immediately when popup opens during active recording

#### In page (optional, v6.1):
- [ ] Small recording indicator overlay (red dot + timer) in corner of page
- [ ] Non-intrusive, similar to Loom's recording indicator
- [ ] Can be disabled in settings

### R7: Limitations (v6)

- [ ] **Memory guard at 100MB** — recording auto-stops and saves when chunks exceed 100MB in memory. At 15fps (~4MB/min) that's ~25 min. At 60fps (~15MB/min) that's ~7 min. Recording is saved with `truncated: true`, not lost.
- [ ] **One recording at a time** — concurrent recordings not supported
- [ ] **Active tab only** — records the tab Kaboom is connected to
- [ ] **No audio** — video only (tab audio is a future enhancement)
- [ ] **Chrome/Chromium only** — `tabCapture` API is Chrome-specific

---

## API Surface

### interact (2 new actions)

| Action | Parameters | Returns |
|--------|-----------|---------|
| `screen_recording_start` | `name?: string`, `fps?: number` (default 15, range 5–60) | `{status, name, path, fps}` |
| `screen_recording_stop` | — | `{status, name, path, duration_seconds, size_bytes}` |

### observe (1 new mode)

| Mode | Parameters | Returns |
|------|-----------|---------|
| `saved_videos` | `url?: string`, `last_n?: number` | `{recordings: [...], total, storage_used_bytes}` |

---

## Out of Scope (v6)

- **Audio recording** — tab audio capture requires `tabCapture` with audio constraints; adds complexity. Future enhancement.
- **Multi-tab recording** — one tab at a time is sufficient for v6.
- **Cloud upload/sharing** — files saved locally; user shares manually.
- **Video editing/trimming** — use external tools if needed.
- **Streaming video to MCP client** — files saved to disk, referenced by path.
- **Chunked recording** — v6 holds video in memory; future version streams chunks for longer recordings.
- **Recording playback in browser** — user opens the .webm file in any video player.

## Future (v6.1+)

- **Chunked streaming** — stream video chunks to server during recording for 30+ minute sessions
- **Tab audio** — capture audio alongside video
- **In-page recording indicator** — visual overlay showing recording state
- **Auto-recording on test flows** — automatically record during `interact` test sequences
- **Recording annotations** — add timestamped markers during recording

---

## Success Criteria

### Functional
- AI can start/stop recordings via `interact()`
- User can start/stop recordings from extension popup
- Named recordings with timestamps saved to `~/.kaboom/recordings/`
- Metadata sidecar written alongside each video
- `observe({what: "saved_videos"})` lists all recordings with metadata
- Subtitles and action toasts visible in recorded video
- Popup shows recording state (indicator + elapsed time)
- Single recording pipeline shared between MCP and popup

### Non-Functional
- Recording overhead: < 10% CPU during capture
- Video quality: readable text at 1080p
- File size: ~3-5MB per minute (WebM VP8)
- Start latency: < 500ms from command to first frame
- Stop + save latency: < 2s for a 5-minute recording
- No data loss on clean stop

---

## Next Steps

1. **Tech Spec** — architecture, state machine, sequence diagrams, edge cases
2. **QA Plan** — test scenarios for start/stop, naming, popup, listing, edge cases
3. **Implementation** — manifest changes, extension recording logic, Go server endpoints, popup UI
