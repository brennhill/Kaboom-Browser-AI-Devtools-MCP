---
feature: cpu-network-emulation
status: proposed
doc_type: tech-spec
feature_id: feature-cpu-network-emulation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: CPU/Network Emulation

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Emulation uses Chrome DevTools Protocol (CDP) to apply network and CPU throttling. Extension's background.js calls chrome.debugger API to attach to tab, sends CDP commands (Network.emulateNetworkConditions, Emulation.setCPUThrottlingRate), stores current settings, exposes reset capability.

## Key Components

- **CDP controller (background.js)**: Attach debugger, send CDP commands
- **Profile manager**: Store network profiles (Slow 3G, Fast 3G, etc.) with bandwidth/latency values
- **Settings tracker**: Track current emulation state per tab
- **Configure tool integration**: Add "emulation" action to configure tool
- **Observe integration**: Include emulation status in observe output

## Data Flows

```
Agent: configure({what: "emulation", network: "Slow 3G"})
  → Server receives, validates profile
  → Returns success
  → Agent polls: observe({what: "emulation_status"})
  → Extension applies:
      1. chrome.debugger.attach(tabId)
      2. Send CDP: Network.emulateNetworkConditions({offline: false, downloadThroughput: 400*1024/8, uploadThroughput: 400*1024/8, latency: 400})
      3. Store current settings in extension state
  → Returns confirmation
  → All subsequent network requests throttled
```

## Implementation Strategy

### Network throttling:
- Use CDP Network.emulateNetworkConditions command
- Pre-defined profiles (Slow 3G: 400 Kbps down, 400 up, 400ms latency; Fast 3G: 1.6 Mbps down, 750 up, 150ms; 4G: 4 Mbps down, 3 up, 20ms)
- Custom: accept kbps/latency values, convert to CDP format
- Offline: {offline: true}

### CPU throttling:
- Use CDP Emulation.setCPUThrottlingRate command
- Rate is slowdown multiplier (4 = 4x slower than native)
- Affects JS execution, rendering, parsing

### Settings persistence:
- Store current emulation settings in extension storage
- Re-apply on page reload (listen to webNavigation.onCommitted)
- Reset: send CDP with default values, clear storage

## Edge Cases & Assumptions

- **Edge Case 1**: Debugger already attached (another extension) → **Handling**: Detach existing, attach ours, warn user
- **Edge Case 2**: Tab closed while emulation active → **Handling**: Clean up debugger, remove settings from storage
- **Edge Case 3**: Offline mode prevents page load → **Handling**: Agent must handle error, reset to online
- **Assumption 1**: Chrome supports CDP (Chromium-based browsers only)
- **Assumption 2**: User doesn't manually change DevTools throttling during agent session

## Risks & Mitigations

- **Risk 1**: CDP changes break in future Chrome versions → **Mitigation**: Test across Chrome versions, fallback to manual instruction
- **Risk 2**: Debugger attachment triggers warnings → **Mitigation**: Document expected "DevTools attached" notification
- **Risk 3**: Extreme throttling makes agent timeout → **Mitigation**: Agent must account for slower response times
- **Risk 4**: Multiple tabs conflict → **Mitigation**: Track settings per tab, allow independent throttling

## Dependencies

- Chrome.debugger API (extension permission required)
- Chrome DevTools Protocol
- configure tool

## Performance Considerations

- Debugger attachment: <50ms
- CDP command execution: <10ms
- Network throttling applies immediately to new requests
- CPU throttling affects all JS in tab (including extension content scripts)

## Security Considerations

- Debugger permission is sensitive (full CDP access)
- Emulation settings only affect local browsing, no data sent externally
- Cannot throttle incognito tabs (extension policy)
