---
doc_type: writing_standard
scope: content
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
---

# Kaboom Content Style and Voice Guide (Research-Backed)

## Goal

Write tutorials and articles that are:

- Easy for first-time users to follow
- Easy for search engines to understand
- Easy for language models (LLMs) and agents to parse reliably

This guide is the default writing standard for **KaBOOM Agentic Devtools** content.

Visual standards companion: [gokaboom.dev visual evidence guide](../../gokaboom.dev/src/content/docs/guides/visual-evidence-standards/index.md)
CI gates companion: [docs quality CI gates](./docs-quality-ci-gates.md)

## Audience and Reading Assumptions

- Primary audience: semi-technical and non-technical users trying agentic tools for the first time
- Assume zero acronym knowledge
- Assume users scan before they read deeply

Write so a user can get value in under 60 seconds, then go deeper if needed.

## Research-Based Principles

## 1) Put user intent first

- Start with the real task the reader is trying to complete.
- Answer: Who is this for? What problem does it solve? Why now?
- Do not write content made mainly for ranking.

Why: Google’s people-first guidance prioritizes useful, trustworthy content that satisfies user intent.
Source: https://developers.google.com/search/docs/fundamentals/creating-helpful-content

## 2) Optimize for scanning

- Front-load the answer in the intro.
- Use specific headings and short sections.
- Keep paragraphs short (1 to 4 lines in rendered docs).
- Use bullets for steps, options, and outcomes.

Why: Most users scan web pages; they do not read every word linearly.
Source: https://github.com/USAGov/plainlanguage/blob/gh-pages/content/howto/guidelines/web.md

## 3) Use plain language

- Prefer common words over internal jargon.
- Keep sentences direct and concrete.
- Use active voice.
- Avoid filler and abstract phrases.

Why: Plain English improves comprehension and completion speed across literacy levels.
Sources:
- https://www.gov.uk/guidance/content-design/writing-for-gov-uk
- https://developers.google.com/style
- https://learn.microsoft.com/en-us/style-guide/welcome/

## 4) Define terms and acronyms on first use

- First use: full term + acronym in parentheses.
- Later uses: acronym only is fine.
- If a term is niche, add a one-line definition.

Example: `Static Analysis Results Interchange Format (SARIF)`

Why: Reduces confusion and prevents silent drop-off for new users.
Source: https://www.gov.uk/guidance/style-guide/a-to-z-of-gov-uk-style

## 5) Structure by intent (Diataxis)

Pick one intent per page:

- Tutorial: learn by doing
- How-to: solve a specific problem
- Reference: exact behavior/arguments/options
- Explanation: concepts and tradeoffs

Why: Mixing intents makes docs harder to navigate and harder to retrieve.
Source: https://diataxis.fr/

## 6) Keep procedures chunked and bounded

- Prefer 3 to 7 steps per section.
- Each step should contain one primary action.
- For longer flows, split into phases with clear checkpoints.

Why: Smaller procedural chunks reduce cognitive load and failure rate.
Source: https://learn.microsoft.com/en-us/style-guide/procedures-instructions/writing-step-by-step-instructions

## 7) Support readability and accessibility

- Use descriptive link text (not “click here”).
- Keep line length comfortable for long prose.
- Use heading hierarchy consistently (`H2 -> H3`).

Why: Better legibility helps all users and improves machine parsing.
Source: https://www.w3.org/WAI/WCAG21/Understanding/visual-presentation.html

## 8) Keep SEO fundamentals clean

- Use unique, descriptive page titles.
- Write concise meta descriptions tied to user task.
- Use meaningful heading text and internal links.

Why: Clear titles/headings improve indexing and match quality.
Sources:
- https://developers.google.com/search/docs/fundamentals/seo-starter-guide
- https://developers.google.com/search/docs/appearance/title-link

## 9) Keep LLM/agent consumption explicit

- Publish stable markdown output for each page.
- Keep section headings predictable.
- Use lists/tables for parameters, steps, and decisions.

Why: Markdown structure improves deterministic extraction by agents.
Source: https://blog.cloudflare.com/markdown-for-agents/

## Kaboom Voice Model

## Voice traits (always on)

- Friendly: clear and welcoming, never condescending
- Practical: focused on getting the task done now
- Precise: specific UI names, commands, and expected results
- Honest: call out limits, risks, and common failure modes

## Tone by context

- Beginner tutorials: encouraging, slower pace, extra definitions
- Troubleshooting: calm, direct, evidence-first
- Reference docs: neutral, terse, exact
- Product updates/blog: energetic but factual, no hype language

## Brand language

- Preferred product name: **KaBOOM Agentic Devtools**
- Use “agentic” only when it clarifies capability, not as filler
- Avoid buzzwords like “synergy,” “revolutionary,” and “game-changing”

## Tutorial Page Blueprint

Use this default order for tutorials/how-to articles:

1. `What you’ll do` (1 short paragraph)
2. `Who this is for`
3. `Before you start` (prerequisites)
4. `Step-by-step`
5. `Check your result` (verification signals)
6. `If something goes wrong` (top 3 fixes)
7. `Next step` (one logical continuation)
8. `Glossary` (only if needed)

## Writing Rules (Operational)

- Title format: `How to <outcome> with KaBOOM Agentic Devtools`
- Intro length: 2 to 4 sentences
- Paragraph length: max ~70 words when possible
- Sentence target: usually 8 to 20 words
- One heading = one user question
- One paragraph = one idea
- Use numbered lists for ordered actions
- Use bullets for unordered facts/options
- Every screenshot must have intent-focused alt text

## Acronym and Term Policy

- Spell out all acronyms on first mention.
- Keep a term consistent across the page (do not alternate synonyms).
- If a term may be unfamiliar, define it in plain language in one sentence.

Example:

- `Accessible Rich Internet Applications (ARIA)` helps screen readers understand UI semantics.
- `WebSocket` is a persistent, two-way browser-server connection.

## Quality Gate (Pre-Publish Checklist)

- [ ] Title states outcome and target audience clearly
- [ ] Intro explains problem and payoff in under 4 sentences
- [ ] Acronyms are expanded on first use
- [ ] Steps are ordered, testable, and reproducible
- [ ] Each major section has a clear user-intent heading
- [ ] Includes verification criteria (“what success looks like”)
- [ ] Includes troubleshooting for likely failure points
- [ ] Uses descriptive links and image alt text
- [ ] Frontmatter includes `title`, `description`, and `date` where required
- [ ] Markdown structure is clean for LLM parsing (no giant unbroken blocks)

## Automated Enforcement

These checks are enforced in CI and can be run locally:

- `npm run docs:lint:style-contract`
  - Enforces article/blog metadata (`date`, `authors`, `tags`)
  - Enforces `<!-- more -->` excerpt marker on articles
  - Enforces how-to title + step-by-step format
  - Enforces acronym expansion on first use in `how-to-*` articles
- `npm run docs:lint:vale`
  - Enforces plain-language and link-text rules via Vale

Combined gate:

- `npm run docs:ci`

## What to Avoid

- Vague openings (“In today’s fast-paced world…”)
- Long conceptual sections before the first useful action
- Unexplained acronyms (for example: ARIA, SARIF, CSP, DOM)
- Marketing-heavy claims without concrete proof
- Mixing tutorial, reference, and announcement content in one page

## Sources

- Google Search: Helpful, reliable, people-first content
  - https://developers.google.com/search/docs/fundamentals/creating-helpful-content
- Google Search: SEO starter guide
  - https://developers.google.com/search/docs/fundamentals/seo-starter-guide
- Google Search: Title links
  - https://developers.google.com/search/docs/appearance/title-link
- Google Developer Style Guide
  - https://developers.google.com/style
  - https://developers.google.com/style/translation
  - https://developers.google.com/style/voice
- GOV.UK writing guidance
  - https://www.gov.uk/guidance/content-design/writing-for-gov-uk
  - https://www.gov.uk/guidance/style-guide/a-to-z-of-gov-uk-style
- Microsoft Writing Style Guide
  - https://learn.microsoft.com/en-us/style-guide/welcome/
  - https://learn.microsoft.com/en-us/style-guide/procedures-instructions/writing-step-by-step-instructions
- U.S. Plain Language Guidelines
  - https://www.plainlanguage.gov/
  - https://github.com/USAGov/plainlanguage/blob/gh-pages/content/howto/guidelines/web.md
- Diataxis documentation framework
  - https://diataxis.fr/
- WCAG 2.1 visual presentation
  - https://www.w3.org/WAI/WCAG21/Understanding/visual-presentation.html
- Cloudflare: Markdown for agents
  - https://blog.cloudflare.com/markdown-for-agents/
