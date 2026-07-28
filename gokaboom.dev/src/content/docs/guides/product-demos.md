---
title: Run a Product Demo with AI
description: Use KaBOOM's subtitle narration and browser control to deliver polished, narrated product demos — no slides, no scripts, just live software.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'product', 'demos']
---

## The Problem with Product Demos

Product demos are high-stakes. You're tabbing between slides and your app, fumbling with browser windows, narrating and clicking simultaneously. One wrong click and the audience sees a 500 error instead of your killer feature.

What if your AI could drive the browser while narrating every step — like closed captions for a live demo?

That's exactly what KaBOOM does. Your AI reads a natural language script, controls the browser with `interact`, and overlays narration text as subtitles. You sit back and talk to the audience.

## Prep: Dial in the Demo Settings

Before the demo, configure the extension for a clean presentation:

**Turn off Action Toasts** — These are the blue/green notification banners that appear when the AI performs actions. Great for development, distracting during a demo.

1. Click the KaBOOM extension icon in your toolbar
2. Uncheck **Action Toasts**

<!-- Screenshot: Extension popup with Action Toasts toggle unchecked -->

**Keep Subtitles on** — These are the narration overlays at the bottom of the viewport (like closed captions). Make sure **Subtitles** is checked.

<!-- Screenshot: Extension popup with Subtitles toggle checked -->

That's it. Now the audience sees your app and the narration — nothing else.

## Write a Natural Language Script

Forget Playwright syntax. Forget Selenium selectors. Write your demo script in plain English and let the AI translate it into `interact` calls.

Here's an example script you'd give your AI:

```text
Demo: Acme Project Management

1. Navigate to https://app.acme.dev
   Subtitle: "Welcome to Acme — project management that gets out of your way."

2. Click the "New Project" button
   Subtitle: "Creating a new project takes one click."

3. Type "Q4 Marketing Campaign" in the project name field
   Subtitle: "Give it a name, and Acme sets up your workspace automatically."

4. Click "Create"
   Subtitle: "Boards, timelines, and team channels — all created instantly."

5. Wait 2 seconds, then scroll to the timeline view
   Subtitle: "The timeline populates with suggested milestones based on your project type."

6. Click on the "Launch Day" milestone
   Subtitle: "Every milestone is clickable. Drill into details, assign owners, set dependencies."

7. Clear subtitle
   Subtitle: ""
```

The AI translates each step into `interact` calls with composable subtitles:

```js
interact({what: "navigate", url: "https://app.acme.dev",
          subtitle: "Welcome to Acme — project management that gets out of your way."})

interact({what: "click", selector: "text=New Project",
          subtitle: "Creating a new project takes one click."})

interact({what: "type", selector: "placeholder=Project name",
          text: "Q4 Marketing Campaign",
          subtitle: "Give it a name, and Acme sets up your workspace automatically."})
```

Every `interact` action accepts a `subtitle` parameter — narration is composable, not a separate step. One tool call moves the browser _and_ updates the narration simultaneously.

<!-- Screenshot: Browser with subtitle overlay visible at bottom of viewport during a demo -->

## Why This Beats Slides

**It's live software.** The audience watches real interactions with real data. No mocked screenshots, no "imagine this works" hand-waving.

**It's resilient.** KaBOOM uses semantic selectors — `text=New Project`, `placeholder=Email`, `aria-label=Close` — not brittle CSS paths. If your designer moves a button, the demo still works.

**It's repeatable.** Save the script, run it again tomorrow. Same demo, same timing, same narration. Hand it to a colleague and they run the same polished demo.

**It's recoverable.** If something unexpected happens, your AI can observe the page state and adapt. It's not a rigid recording — it's an intelligent agent.

## Pro Tips for Polished Demos

**Use `reason` for your own reference.** When toasts are on (during rehearsal), the `reason` parameter labels each action so you can verify the script is hitting the right elements:

```js
interact({what: "click", selector: "text=Create",
          reason: "Create the new project"})
```

<!-- Screenshot: Action toast showing "Create the new project" during rehearsal -->

**Pace with pauses.** Add explicit waits between steps so the audience can absorb each screen:

```js
interact({what: "wait_for", selector: "text=Timeline", timeout_ms: 3000})
```

**Discover elements first.** Not sure what selectors to use? Run `list_interactive` on any page to get a full inventory of clickable and typeable elements with suggested selectors:

```js
interact({what: "list_interactive"})
```

**Save checkpoints.** Before the demo, save the starting state so you can reset instantly if you need to run it again:

```js
interact({what: "save_state", snapshot_name: "demo-start"})
```

Reset anytime with:

```js
interact({what: "load_state", snapshot_name: "demo-start", include_url: true})
```

**Clear subtitles at the end.** Send an empty string to remove the narration bar when you're done:

```js
interact({what: "subtitle", text: ""})
```

## Checklist

Before you go live:

- [ ] KaBOOM server running (`npx kaboom-agentic-browser`)
- [ ] Extension connected (green icon)
- [ ] AI Web Pilot enabled (extension popup toggle)
- [ ] Action Toasts **off**
- [ ] Subtitles **on**
- [ ] Starting state saved
- [ ] Script tested end-to-end at least once
- [ ] Browser zoom set for projector/screen share

Now light it up.
