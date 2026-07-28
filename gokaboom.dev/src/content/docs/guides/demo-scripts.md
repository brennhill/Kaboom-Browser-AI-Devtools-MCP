---
title: Create and Replay a Natural Language Demo Script
description: Write a demo in plain English, run it with AI, replay it perfectly every time. A step-by-step guide to reusable demo scripts.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'demo', 'scripts']
---

## What Is a Natural Language Demo Script?

A natural language demo script is a plain text file that describes your product demo step by step — in English, not code. Instead of Selenium or Playwright, you write what you'd tell a colleague who's running the demo for you.

The AI reads the script, drives the browser, and overlays narration using KaBOOM's subtitle system. You watch it run, or you narrate over the top.

```text
Write script (text file) -> Give to AI -> AI drives browser -> Replay anytime
```

## Step 1: Write the Script

Create a text file. Describe what happens on screen and what the audience should see:

```text
# Acme Dashboard Demo
# Audience: Enterprise prospects
# Duration: ~3 minutes

## Setup
- Navigate to https://app.acme.dev/login
- Log in with demo@acme.dev / DemoPass123
- Wait for the dashboard to load
- Subtitle: ""

## Section 1: The Dashboard
- Subtitle: "This is your command center. Every project, every metric, one screen."
- Wait 3 seconds
- Subtitle: "Let's create something."

## Section 2: Create a Project
- Click "New Project"
- Subtitle: "One click to start a new project."
- Type "Q4 Marketing Campaign" in the project name field
- Select "Marketing" from the team dropdown
- Subtitle: "Pick a team, and Acme assigns the right workflows automatically."
- Click "Create"
- Wait for the project page to load
- Subtitle: "Boards, timelines, and team channels — created instantly."
- Wait 3 seconds

## Section 3: The Timeline
- Click "Timeline" in the sidebar
- Subtitle: "The timeline populates with suggested milestones based on your project type."
- Wait 2 seconds
- Click on the "Launch Day" milestone
- Subtitle: "Every milestone is interactive. Assign owners, set dependencies, track progress."
- Wait 3 seconds

## Wrap Up
- Navigate to the dashboard
- Subtitle: "That's Acme. Projects that manage themselves."
- Wait 4 seconds
- Subtitle: ""
```

That's your entire demo. Save it as `demo-enterprise.txt`.

## Step 2: Run It

Open your AI tool (Claude Code, Cursor, etc.) with KaBOOM connected. Paste or reference the script:

```text
Run this demo script. Use KaBOOM's interact tool for browser actions
and composable subtitles for narration.

[paste script or reference file path]
```

The AI translates each line into `interact` calls:

```js
interact({what: "navigate", url: "https://app.acme.dev/login"})

interact({what: "type", selector: "label=Email", text: "demo@acme.dev"})
interact({what: "type", selector: "label=Password", text: "DemoPass123"})
interact({what: "click", selector: "text=Log In"})
interact({what: "wait_for", selector: "text=Dashboard", timeout_ms: 10000})

interact({what: "subtitle", text: ""})

interact({what: "subtitle",
          text: "This is your command center. Every project, every metric, one screen."})
```

<!-- Screenshot: Browser during demo with subtitle overlay visible -->

Every action + subtitle happens in the same tool call where possible, keeping narration synchronized with browser actions.

## Step 3: Make It Replayable

### Save a Starting Checkpoint

Before the first run, save the browser state:

```text
Before running the demo, save the current state as "demo-start".
```

The AI calls:
```js
interact({what: "save_state", snapshot_name: "demo-start"})
```

### Reset Between Runs

Before each replay:

```text
Load the "demo-start" state and navigate to the saved URL.
Then run the demo script.
```

The AI calls:
```js
interact({what: "load_state", snapshot_name: "demo-start", include_url: true})
```

Then executes the full script from the top. Same flow, same timing, same narration. Every time.

### Version for Different Audiences

Keep multiple scripts:

```text
demo-enterprise.txt     — Full features, business metrics focus
demo-engineering.txt    — API details, performance data, WebSocket traffic
demo-quick.txt          — 90-second highlight reel
demo-investor.txt       — Growth metrics, market positioning
```

Same product. Same KaBOOM setup. Different scripts for different audiences.

## Step 4: Polish the Script

### Pacing

Add explicit waits so the audience absorbs each screen:

```text
- Click "Create"
- Wait for the project page to load
- Wait 3 seconds                        <- Let them look
- Subtitle: "Boards, timelines, and team channels — created instantly."
```

### Transitions

Clear the subtitle between sections to signal a new topic:

```text
- Subtitle: ""
- Wait 1 second
- Subtitle: "Now let's look at real-time collaboration."
```

### Discovery

Not sure what's on a page? Add a discovery step during development:

```text
- List all interactive elements on this page (for development only — remove before demo)
```

The AI calls `list_interactive` and shows you every clickable and typeable element with suggested selectors. Use this to refine your script.

### Error Handling

Add fallback instructions for known issues:

```text
- Click "Generate Report"
- Wait for the report to load (up to 10 seconds)
- If the page shows a loading spinner after 10 seconds, refresh and try again
```

The AI handles this intelligently — it waits, checks the page state, and adapts.

## Complete Script Template

```text
# [Product Name] Demo
# Audience: [Who is watching]
# Duration: [Expected time]
# Prerequisites: [Account created, data populated, etc.]

## Setup
- Navigate to [starting URL]
- [Login steps if needed]
- Save state as "demo-ready"
- Subtitle: ""

## Section 1: [Topic]
- Subtitle: "[Opening narration]"
- [Actions: click, type, navigate]
- Wait [N] seconds
- Subtitle: "[Explanation of what they just saw]"

## Section 2: [Topic]
- Subtitle: ""
- Wait 1 second
- Subtitle: "[Transition narration]"
- [Actions]
- Subtitle: "[Key takeaway]"

## Wrap Up
- Navigate to [final screen]
- Subtitle: "[Closing message]"
- Wait 4 seconds
- Subtitle: ""
```

## Quick Reference

| Task | How |
|---|---|
| Show narration | `Subtitle: "Your text"` in script |
| Hide narration | `Subtitle: ""` in script |
| Pause between steps | `Wait N seconds` |
| Save starting point | `Save state as "demo-start"` |
| Reset for replay | `Load state "demo-start"` |
| Find elements | `List interactive elements` |
| Handle slow loads | `Wait for [element] (up to N seconds)` |

Write it once. Replay it forever. Focus on the story, not the staging.
