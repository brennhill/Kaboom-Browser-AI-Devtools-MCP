#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const version = (await fs.readFile(path.join(repoRoot, 'VERSION'), 'utf8')).trim()
const today = new Date().toISOString().slice(0, 10)

const INTERACT_ALIAS_ACTIONS = new Set(['state_save', 'state_load', 'state_list', 'state_delete'])

function dedupe(values) {
  return [...new Set(values)]
}

function extractQuotedStrings(source) {
  return [...source.matchAll(/"([^"]+)"/g)].map((match) => match[1])
}

function extractWhatEnum(schemaSource) {
  const match = schemaSource.match(/"what"\s*:\s*map\[string\]any\{[\s\S]*?"enum"\s*:\s*\[\]string\{([\s\S]*?)\}/m)
  if (!match) throw new Error('Could not find what enum')
  return dedupe(extractQuotedStrings(match[1]))
}

function extractInteractActions(schemaSource) {
  const match = schemaSource.match(/var\s+interactActionSpecs\s*=\s*\[\]InteractActionSpec\{([\s\S]*?)\n\}/m)
  if (!match) throw new Error('Could not find interact action specs')
  return dedupe(
    [...match[1].matchAll(/Name:\s*"([^"]+)"/g)]
      .map((entry) => entry[1])
      .filter((name) => !INTERACT_ALIAS_ACTIONS.has(name))
  )
}

function prettyJSON(value) {
  return JSON.stringify(value, null, 2)
}

const modeDefaults = {
  observe: {
    command_result: { correlation_id: 'cmd_123' },
    recording_actions: { recording_id: 'rec_123' },
    playback_results: { recording_id: 'rec_123' },
    log_diff_report: { original_id: 'rec_123', replay_id: 'rec_456' },
    screenshot: { full_page: true },
    network_bodies: { url: '/api', status_min: 400 },
    websocket_events: { last_n: 20 }
  },
  analyze: {
    dom: { selector: '.error-banner' },
    accessibility: { scope: '#main', tags: ['wcag2a', 'wcag2aa'] },
    security_audit: { checks: ['credentials', 'pii'] },
    link_health: { domain: 'example.com' },
    link_validation: { urls: ['https://example.com', 'https://example.com/docs'] },
    page_summary: { timeout_ms: 10000 },
    annotations: { wait: true, timeout_ms: 60000 },
    annotation_detail: { correlation_id: 'ann_123' },
    api_validation: { operation: 'analyze' },
    draw_session: { file: 'draw-session-2026-03-05.json' },
    computed_styles: { selector: 'button[type="submit"]' },
    form_state: { selector: 'form#checkout' },
    data_table: { selector: 'table' },
    visual_baseline: { name: 'home-baseline' },
    visual_diff: { baseline: 'home-baseline', name: 'home-current' },
    audit: { categories: ['performance', 'accessibility'] }
  },
  configure: {
    store: { key: 'project.current', data: { value: 'checkout-redesign' } },
    load: { key: 'project.current' },
    noise_rule: { noise_action: 'add', category: 'console', message_regex: 'ResizeObserver loop limit exceeded' },
    playback: { recording_id: 'rec_123' },
    log_diff: { original_id: 'rec_123', replay_id: 'rec_456' },
    save_sequence: {
      name: 'checkout-smoke',
      steps: [
        { what: 'navigate', url: 'https://example.com' },
        { what: 'click', selector: 'text=Checkout' }
      ]
    },
    get_sequence: { name: 'checkout-smoke' },
    delete_sequence: { name: 'checkout-smoke' },
    replay_sequence: { name: 'checkout-smoke' },
    security_mode: { mode: 'normal' },
    network_recording: { operation: 'start', domain: 'example.com' },
    action_jitter: { action_jitter_ms: 120 },
    report_issue: { operation: 'draft', title: 'Intermittent checkout timeout' }
  },
  generate: {
    reproduction: { mode: 'playwright', include_screenshots: true },
    test: { test_name: 'checkout-smoke' },
    har: { url: '/api', status_min: 400 },
    csp: { include_report_uri: true },
    sri: { resource_types: ['script', 'stylesheet'] },
    sarif: { scope: '#main' },
    visual_test: { test_name: 'landing-visual-check' },
    annotation_report: { annot_session: 'landing-review' },
    annotation_issues: { annot_session: 'landing-review' },
    test_from_context: { context: 'Checkout button click does nothing' },
    test_heal: { action: 'analyze', test_file: 'tests/e2e/checkout.spec.ts' },
    test_classify: {
      action: 'failure',
      failure: {
        test_name: 'checkout should submit',
        error: 'Timeout 30000ms exceeded while waiting for selector text=Confirm'
      }
    }
  },
  interact: {
    navigate: { url: 'https://example.com' },
    new_tab: { url: 'https://example.com' },
    click: { selector: 'text=Submit' },
    type: { selector: 'label=Email', text: 'user@example.com' },
    select: { selector: '#country', value: 'US' },
    check: { selector: '#terms', checked: true },
    get_text: { selector: 'h1' },
    get_value: { selector: 'input[name="email"]' },
    get_attribute: { selector: 'a.primary', name: 'href' },
    query: { selector: 'button', query_type: 'count' },
    set_attribute: { selector: 'input[name="email"]', name: 'value', value: 'user@example.com' },
    focus: { selector: '#search' },
    scroll_to: { direction: 'bottom' },
    wait_for: { selector: 'main' },
    key_press: { text: 'Enter' },
    paste: { selector: 'textarea', text: 'Pasted from automation' },
    hover: { selector: 'text=Settings' },
    navigate_and_wait_for: { url: 'https://example.com', wait_for: 'main' },
    navigate_and_document: { url: 'https://example.com', include_screenshot: true },
    fill_form_and_submit: {
      fields: [
        { selector: 'input[name="email"]', value: 'user@example.com' },
        { selector: 'input[name="password"]', value: 'hunter2' }
      ],
      submit_selector: 'button[type="submit"]'
    },
    fill_form: {
      fields: [
        { selector: 'input[name="email"]', value: 'user@example.com' }
      ]
    },
    run_a11y_and_export_sarif: { save_to: '.kaboom/reports/a11y.sarif' },
    upload: { file_path: '/tmp/example.png', selector: 'input[type="file"]' },
    draw_mode_start: { annot_session: 'checkout-review' },
    hardware_click: { x: 640, y: 360 },
    activate_tab: { tab_id: 123 },
    switch_tab: { tab_id: 123 },
    close_tab: { tab_id: 123 },
    subtitle: { text: 'Opening checkout flow' },
    execute_js: { script: 'document.title' },
    set_storage: { storage_type: 'local', key: 'theme', value: 'dark' },
    delete_storage: { storage_type: 'local', key: 'theme' },
    set_cookie: { name: 'theme', value: 'dark', domain: 'example.com' },
    delete_cookie: { name: 'theme', domain: 'example.com' },
    batch: { steps: [{ what: 'navigate', url: 'https://example.com' }, { what: 'click', selector: 'text=Sign in' }] },
    clipboard_write: { text: 'Copied by Kaboom' }
  }
}

function expectedShape(tool, mode) {
  if (tool === 'observe') {
    return {
      what: mode,
      items: [{ id: 'sample', summary: '...mode-specific payload...' }],
      metadata: { limit: 100, next_cursor: 'cursor_123' }
    }
  }
  if (tool === 'interact') {
    return {
      action: mode,
      ok: true,
      url: 'https://example.com',
      result: { summary: 'Action completed', mode: mode }
    }
  }
  if (tool === 'analyze') {
    return {
      what: mode,
      status: 'completed',
      result: { summary: 'Analysis completed', findings: [] }
    }
  }
  if (tool === 'generate') {
    return {
      what: mode,
      content: [{ type: 'text', text: 'Generated artifact summary' }],
      artifact: { format: mode, path: '.kaboom/reports/sample.out' }
    }
  }
  return {
    what: mode,
    ok: true,
    result: { summary: 'Configuration updated', mode: mode }
  }
}

function failureExample(tool, mode, baseArgs) {
  const failureArgs = { ...baseArgs }
  let fix

  if ('limit' in failureArgs) {
    failureArgs.limit = '100'
    fix = 'Use `limit` as a number, e.g. `limit: 100`.'
  } else if ('tab_id' in failureArgs) {
    failureArgs.tab_id = '123'
    fix = 'Use `tab_id` as a number, e.g. `tab_id: 123`.'
  } else if ('timeout_ms' in failureArgs) {
    failureArgs.timeout_ms = '10000'
    fix = 'Use `timeout_ms` as a number of milliseconds.'
  } else if ('selector' in failureArgs) {
    failureArgs.selector = 42
    fix = 'Use `selector` as a CSS or semantic selector string.'
  } else if ('url' in failureArgs) {
    failureArgs.url = 404
    fix = 'Use a fully qualified URL string, e.g. `https://example.com`.'
  } else if ('steps' in failureArgs) {
    failureArgs.steps = 'navigate,click'
    fix = 'Use `steps` as an array of action objects.'
  } else if ('recording_id' in failureArgs) {
    failureArgs.recording_id = 123
    fix = 'Use `recording_id` as a string like `rec_123`.'
  } else {
    failureArgs.what = 'not_a_real_mode'
    fix = `Use a valid ${tool} ${tool === 'interact' ? 'action' : 'mode'} value, e.g. \`${mode}\`.`
  }

  return { failureArgs, fix }
}

function buildModeSection(tool, mode) {
  const defaults = modeDefaults[tool]?.[mode] ?? {}
  const baseArgs = tool === 'interact' ? { what: mode, ...defaults } : { what: mode, ...defaults }
  const minimalCall = { tool, arguments: baseArgs }
  const responseShape = expectedShape(tool, mode)
  const { failureArgs, fix } = failureExample(tool, mode, baseArgs)
  const failureCall = { tool, arguments: failureArgs }

  return `### \`${mode}\`

#### Minimal call

\`\`\`json
${prettyJSON(minimalCall)}
\`\`\`

#### Expected response shape

\`\`\`json
${prettyJSON(responseShape)}
\`\`\`

#### Failure example and fix

\`\`\`json
${prettyJSON(failureCall)}
\`\`\`

Fix: ${fix}
`
}

async function readModes() {
  const observeSource = await fs.readFile(path.join(repoRoot, 'internal/schema/observe.go'), 'utf8')
  const analyzeSource = await fs.readFile(path.join(repoRoot, 'internal/schema/analyze.go'), 'utf8')
  const generateSource = await fs.readFile(path.join(repoRoot, 'internal/schema/generate.go'), 'utf8')
  const configureSource = await fs.readFile(path.join(repoRoot, 'internal/schema/configure_properties_core.go'), 'utf8')
  const interactSource = await fs.readFile(path.join(repoRoot, 'internal/schema/interact_actions.go'), 'utf8')

  return {
    observe: extractWhatEnum(observeSource),
    analyze: extractWhatEnum(analyzeSource),
    generate: extractWhatEnum(generateSource),
    configure: extractWhatEnum(configureSource),
    interact: extractInteractActions(interactSource)
  }
}

function pageContent(tool, title, description, modesOrActions) {
  const intro = `Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.`
  const sections = modesOrActions.map((mode) => buildModeSection(tool, mode)).join('\n')
  return `---
title: ${JSON.stringify(title)}
description: ${JSON.stringify(description)}
last_verified_version: ${JSON.stringify(version)}
last_verified_date: ${today}
---

# ${title}

${intro}

## Quick Reference

\`\`\`json
{
  "tool": "${tool}",
  "arguments": {
    "what": "${modesOrActions[0]}"
  }
}
\`\`\`

## Common Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| \`what\` | string | ${tool === 'interact' ? 'Action name to execute.' : 'Mode name to execute.'} |
| \`tab_id\` | number | Optional target browser tab. |
| \`telemetry_mode\` | string | Optional telemetry verbosity: \`off\`, \`auto\`, \`full\`. |

## ${tool === 'interact' ? 'Actions' : 'Modes'}

${sections}`
}

async function writePages() {
  const modes = await readModes()
  const baseDir = path.join(repoRoot, 'gokaboom.dev/src/content/docs/reference/examples')
  await fs.mkdir(baseDir, { recursive: true })

  const pages = [
    {
      tool: 'observe',
      file: 'observe-examples.md',
      title: 'Observe Executable Examples',
      description: 'Runnable examples for every observe mode with response shapes and failure fixes.',
      entries: modes.observe
    },
    {
      tool: 'analyze',
      file: 'analyze-examples.md',
      title: 'Analyze Executable Examples',
      description: 'Runnable examples for every analyze mode with response shapes and failure fixes.',
      entries: modes.analyze
    },
    {
      tool: 'generate',
      file: 'generate-examples.md',
      title: 'Generate Executable Examples',
      description: 'Runnable examples for every generate mode with response shapes and failure fixes.',
      entries: modes.generate
    },
    {
      tool: 'configure',
      file: 'configure-examples.md',
      title: 'Configure Executable Examples',
      description: 'Runnable examples for every configure mode with response shapes and failure fixes.',
      entries: modes.configure
    },
    {
      tool: 'interact',
      file: 'interact-examples.md',
      title: 'Interact Executable Examples',
      description: 'Runnable examples for every interact action with response shapes and failure fixes.',
      entries: modes.interact
    }
  ]

  for (const page of pages) {
    const output = pageContent(page.tool, page.title, page.description, page.entries)
    await fs.writeFile(path.join(baseDir, page.file), output, 'utf8')
  }

  console.log('Generated executable reference examples for all tools.')
}

await writePages()
