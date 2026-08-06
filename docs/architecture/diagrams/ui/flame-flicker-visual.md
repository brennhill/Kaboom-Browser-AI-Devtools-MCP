---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flickering Flame Favicon Visual Indicator

## State Machine

```mermaid
stateDiagram-v2
    [*] --> NotTracking: Page loads

    NotTracking --> TrackingOnly: User clicks<br/>"Track This Tab"
    TrackingOnly --> NotTracking: User clicks<br/>"Stop Tracking"

    TrackingOnly --> AIPilotActive: User toggles<br/>"AI Web Pilot" ON
    AIPilotActive --> TrackingOnly: User toggles<br/>"AI Web Pilot" OFF

    NotTracking: Original Site Favicon
    TrackingOnly: Static Glowing Flame
    AIPilotActive: Flickering Flame (8 frames)

    note right of NotTracking
        Favicon: Site's original icon
        No Kaboom indicator
    end note

    note right of TrackingOnly
        Favicon: icon-glow.svg
        Static flame with ring
        No animation
    end note

    note right of AIPilotActive
        Favicon: 8-frame cycle
        150ms per frame = 1.2s full cycle
        Visible even when tab hidden
        Flame grows/shrinks 85% → 112%
    end note
```

## Animation Sequence

```mermaid
graph LR
    F1[Frame 1<br/>85% tiny<br/>dark orange<br/>glow 2px] -->|150ms| F2[Frame 2<br/>92% small<br/>orange<br/>glow 3px]

    F2 -->|150ms| F3[Frame 3<br/>100% normal<br/>yellow<br/>glow 4.5px]

    F3 -->|150ms| F4[Frame 4<br/>105% medium<br/>bright yellow<br/>glow 6px]

    F4 -->|150ms| F5[Frame 5<br/>112% PEAK<br/>pale yellow<br/>glow 8px]

    F5 -->|150ms| F6[Frame 6<br/>105% medium<br/>bright yellow<br/>glow 6px]

    F6 -->|150ms| F7[Frame 7<br/>96% small-med<br/>yellow<br/>glow 4px]

    F7 -->|150ms| F8[Frame 8<br/>92% small<br/>orange<br/>glow 3px]

    F8 -->|150ms| F1

    style F5 fill:#fef08a
    style F1 fill:#fb923c
    style F3 fill:#fde047
```

## Message Flow

```mermaid
sequenceDiagram
    participant User
    participant Popup as Kaboom Popup
    participant BG as Background Script
    participant Storage as chrome.storage.local
    participant Content as Content Script<br/>(Tracked Tab)
    participant Favicon as Favicon Replacer

    User->>Popup: Clicks "Track This Tab"
    Popup->>Storage: Set trackedTabId = 123

    Storage->>BG: storage.onChanged event
    BG->>Content: sendMessage('trackingStateChanged',<br/>{isTracked: true, aiPilotEnabled: false})
    Content->>Favicon: updateFavicon({isTracked: true, ...})
    Favicon->>Favicon: replaceFaviconWithFlame(false)
    Note over Favicon: Show static flame

    User->>Popup: Toggles "AI Web Pilot" ON
    Popup->>BG: sendMessage('setAiWebPilotEnabled', true)
    BG->>Storage: Set aiWebPilotEnabled = true
    BG->>BG: postSettings() to server

    Storage->>BG: storage.onChanged event
    BG->>Content: sendMessage('trackingStateChanged',<br/>{isTracked: true, aiPilotEnabled: true})
    Content->>Favicon: updateFavicon({isTracked: true, ...})
    Favicon->>Favicon: startFlicker()
    Note over Favicon: Start 8-frame animation<br/>setInterval(150ms)

    loop Every 150ms
        Favicon->>Favicon: Update href to next frame
        Note over Favicon: Browser renders new icon
    end
```

## SVG Frame Structure

```mermaid
graph TB
    subgraph "SVG Components (Per Frame)"
        BG[Background Circle<br/>r=60, fill=#1a1a1a<br/>CONSTANT]
        Ring[Colored Ring<br/>r=62, stroke matches flame<br/>CHANGES per frame]
        Flame[Flame Paths<br/>Scaled vertically<br/>CHANGES per frame]
        GlowF[Flame Glow Filter<br/>feGaussianBlur<br/>CHANGES per frame]
        GlowR[Ring Glow Filter<br/>feGaussianBlur<br/>CHANGES per frame]
    end

    Flame -->|Uses| GlowF
    Ring -->|Uses| GlowR

    GlowF -.->|stdDeviation| Intensity[2px → 8px<br/>Proportional to flame size]
    GlowR -.->|stdDeviation| Intensity

    Flame -.->|transform| Anchor[translate(64, 116)<br/>scale(1, Y)<br/>translate(-64, -116)]

    style BG fill:#1a1a1a,color:#fff
    style Ring fill:#fde047
    style Flame fill:#fb923c
    style GlowF fill:#fef08a
    style GlowR fill:#fef08a
```

## Performance

```mermaid
graph TD
    subgraph "Browser Constraints"
        B1[Favicon update throttling<br/>Browser limits to ~150-300ms]
        B2[SVG parsing<br/>~1ms per frame]
        B3[Tab UI render<br/>~10-50ms]
    end

    subgraph "Our Implementation"
        I1[setInterval 150ms<br/>Request new frame]
        I2[8 static SVG files<br/>~1KB each = 8KB total]
        I3[URL string swap<br/>Negligible CPU]
    end

    I1 --> B1
    I2 --> B2
    I3 --> B3

    B1 & B2 & B3 --> Actual[Actual flicker rate:<br/>~150-300ms effective]

    style Actual fill:#3fb950
```

## Browser Compatibility

| Feature | Chrome | Brave | Edge | Firefox | Safari |
|---------|--------|-------|------|---------|--------|
| SVG Favicon | ✅ | ✅ | ✅ | ✅ | ⚠️ Limited |
| Dynamic Update | ✅ | ✅ | ✅ | ✅ | ❌ |
| Hidden Tab Flicker | ✅ | ✅ | ✅ | ✅ | N/A |
| setInterval in BG | ✅ | ✅ | ✅ | ✅ | ✅ |

**Safari note:** SVG favicons supported but dynamic updates may not show in tab bar.

## Implementation Details

### Why setInterval (not requestAnimationFrame)?

```mermaid
graph TB
    subgraph "requestAnimationFrame"
        RAF1[Pauses when tab hidden]
        RAF2[Synced to display refresh]
        RAF3[No flicker in tab bar ❌]
    end

    subgraph "setInterval"
        SI1[Runs even when tab hidden ✅]
        SI2[Independent of display]
        SI3[Flicker visible in tab bar ✅]
    end

    Goal[Goal: Visible when<br/>working in other tabs] --> SI3

    style Goal fill:#58a6ff
    style SI3 fill:#3fb950
    style RAF3 fill:#f85149
```

### Flame Growth (Bottom-Anchored)

```mermaid
graph TB
    subgraph "Wrong: Center-Scaled"
        W1[Flame grows in all directions]
        W2[Bottom moves down ❌]
        W3[Looks unnatural]
    end

    subgraph "Correct: Bottom-Anchored"
        C1[translate to bottom: y=116]
        C2[scale vertically: scale 1, Y]
        C3[translate back: -y=116]
        C4[Bottom stays fixed ✅]
        C5[Flame grows UPWARD]
    end

    C1 --> C2 --> C3 --> C4 --> C5

    style C5 fill:#3fb950
    style W2 fill:#f85149
```

## Visual Design Rationale

### Color Temperature Physics

```mermaid
graph LR
    subgraph "Real Fire Physics"
        P1[Cool flame: 800°C<br/>Dark red/orange]
        P2[Medium flame: 1000°C<br/>Orange/yellow]
        P3[Hot flame: 1200°C<br/>Bright yellow]
        P4[Hottest: 1400°C<br/>Pale yellow/white]
    end

    subgraph "Our Animation"
        A1[Frame 1: #fb923c<br/>Dark orange]
        A2[Frame 3: #fbbf24<br/>Yellow]
        A3[Frame 4: #fde047<br/>Bright yellow]
        A4[Frame 5: #fef08a<br/>Pale yellow PEAK]
    end

    P1 -.-> A1
    P2 -.-> A2
    P3 -.-> A3
    P4 -.-> A4

    style A4 fill:#fef08a
    style P4 fill:#fef08a
```

### Glow Intensity

```mermaid
graph TD
    Size[Flame Size] -->|Directly proportional| Glow[Glow Radius]

    Glow --> G1[85% flame → 2px glow]
    Glow --> G2[92% flame → 3px glow]
    Glow --> G3[100% flame → 4.5px glow]
    Glow --> G4[105% flame → 6px glow]
    Glow --> G5[112% PEAK → 8px glow<br/>4x larger than tiny]

    G5 --> Effect[Massive pulsing<br/>Very noticeable]

    style G5 fill:#fef08a
    style Effect fill:#3fb950
```

## User Experience

```mermaid
journey
    title Developer Using Kaboom with AI Pilot

    section Working on Bug
      Open buggy page: 5: Developer
      Click "Track This Tab": 5: Developer
      See static flame: 4: Developer
      Toggle AI Pilot ON: 5: Developer
      See flickering flame: 5: Developer

    section AI Debugging
      AI finds error: 3: AI Agent
      AI suggests fix: 4: AI Agent
      AI applies fix: 5: AI Agent
      Developer sees flame flickering: 5: Developer
      Knows AI is working: 5: Developer

    section Multitasking
      Switch to email tab: 5: Developer
      Glance at tabs: 5: Developer
      See flame still flickering: 5: Developer
      Know AI still debugging: 5: Developer
      Peace of mind: 5: Developer
```

## References

- [Implementation: favicon-replacer.ts](../../src/content/favicon-replacer.ts)
- [Icon frames: extension/icons/](../../extension/icons/)
- [Tests: favicon-replacer.test.js](../../tests/extension/branding/favicon-replacer.test.js)
- [Product Spec: Tab Tracking UX](../../../features/feature/tab-tracking-ux/product-spec.md)
