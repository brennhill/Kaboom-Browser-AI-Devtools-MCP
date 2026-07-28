---
title: Automate Browser Tasks and Notify People
description: Use KaBOOM to automate repetitive browser workflows — fill forms, scrape results, send emails — and notify your team when it's done.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'automate', 'notify']
---

## Beyond Debugging: KaBOOM as Your Browser Autopilot

KaBOOM isn't just for debugging. The `interact` tool gives your AI full browser control — click, type, navigate, read page content. Combine that with your AI's ability to reason about what it sees, and you get a general-purpose browser automation agent.

The real power? Chain the automation with a notification step. Run a report, extract the results, and send them to someone — all from a single natural language instruction.

## Example: Generate a Report and Email It via Gmail

Here's the scenario: every Monday, you pull a performance report from your analytics dashboard and email the highlights to your team. Let's automate that.

**The instruction you give your AI:**

```text
1. Go to https://analytics.example.com/dashboard
2. Click "This Week" in the date range picker
3. Click "Generate Report"
4. Wait for the report to load
5. Read the summary numbers: total users, conversion rate, revenue
6. Open Gmail and compose a new email
7. Send to team@example.com
8. Subject: "Weekly Performance Report"
9. Body: Include the summary numbers you extracted, formatted nicely
10. Send it
```

**What the AI does under the hood:**

### Phase 1: Extract the data

```js
interact({what: "navigate", url: "https://analytics.example.com/dashboard"})

interact({what: "click", selector: "text=This Week"})

interact({what: "click", selector: "text=Generate Report"})

interact({what: "wait_for", selector: "text=Total Users", timeout_ms: 10000})

interact({what: "get_text", selector: ".report-summary"})
// -> "Total Users: 14,230 | Conversion: 3.2% | Revenue: $48,700"
```

### Phase 2: Compose and send the email

```js
interact({what: "navigate", url: "https://mail.google.com"})

interact({what: "click", selector: "text=Compose"})

interact({what: "type", selector: "aria-label=To recipients",
          text: "team@example.com"})

interact({what: "key_press", selector: "aria-label=To recipients",
          text: "Tab"})

interact({what: "type", selector: "placeholder=Subject",
          text: "Weekly Performance Report"})

interact({what: "click", selector: "aria-label=Message Body"})

interact({what: "type", selector: "aria-label=Message Body",
          text: "Hi team,\n\nHere are this week's numbers:\n\n- Total Users: 14,230\n- Conversion Rate: 3.2%\n- Revenue: $48,700\n\nBest,\nAutomated Report"})

interact({what: "click", selector: "aria-label=Send"})
```

Done. Your team gets the email. You didn't open a browser.

## Why This Works (and Why It's Safe)

**You're already logged in.** KaBOOM uses your real browser with your real sessions. If you're logged into Gmail, the AI can compose an email. No API keys, no OAuth tokens, no service accounts.

**AI Web Pilot is opt-in.** The `interact` tool only works when you've explicitly enabled AI Web Pilot in the extension popup. It's off by default.

<!-- Screenshot: Extension popup showing the AI Web Pilot toggle -->

**Everything is visible.** The AI drives your actual browser — you can watch every action happen in real time. Nothing runs headlessly or behind the scenes. If the AI clicks something wrong, you see it and can intervene.

**Use subtitles to narrate.** Add `subtitle` to each action so you can follow along:

```js
interact({what: "click", selector: "text=Compose",
          subtitle: "Opening Gmail compose window"})
```

## More Automation Ideas

The same pattern — navigate, interact, extract, notify — works for dozens of workflows:

**Fill out timesheets:**
```text
"Go to our timesheet app, fill in 8 hours for each weekday this week
under project KaBOOM', and submit."
```

**Monitor a competitor's pricing page:**
```text
"Navigate to competitor.com/pricing, read the pricing tiers,
compare them to our pricing page, and email me a summary
of any changes."
```

**Submit a form across multiple systems:**
```text
"Take the data from the spreadsheet at drive.google.com/xyz,
and for each row, go to our internal tool and create a new
support ticket with the customer name and issue description."
```

**Check deployment status and notify Slack:**
```text
"Go to our Vercel dashboard, check if the latest deployment
succeeded, get the deployment URL, then open Slack and post
the status in #engineering."
```

## Tips for Reliable Automation

**Use `wait_for` before interacting.** Pages load asynchronously. Don't assume elements exist immediately:

```js
interact({what: "wait_for", selector: "text=Compose", timeout_ms: 5000})
interact({what: "click", selector: "text=Compose"})
```

**Use `list_interactive` when you're unsure.** If you're automating a new app for the first time, let the AI discover the elements:

```js
interact({what: "list_interactive"})
```

**Check for errors after critical steps.** Verify nothing broke:

```js
observe({what: "errors"})
```

**Save checkpoints before risky actions.** Before clicking "Send" or "Submit," save state so you can review:

```js
interact({what: "save_state", snapshot_name: "before-send"})
```

**Handle Gmail's quirks.** Gmail uses augmented `aria-label` values that include keyboard shortcuts (e.g., `"Send ‪(⌘Enter)‬"`). KaBOOM's `aria-label=` selector handles this automatically with starts-with matching — `aria-label=Send` matches the full label.

## The Pattern

Every browser automation task follows the same structure:

1. **Navigate** to the starting page
2. **Interact** with the UI (click, type, select)
3. **Observe** to extract data or verify state
4. **Navigate** to the notification channel (email, Slack, etc.)
5. **Interact** to compose and send the notification

No new tools. No new APIs. No new credentials. Just your browser, your AI, and KaBOOM connecting them.
