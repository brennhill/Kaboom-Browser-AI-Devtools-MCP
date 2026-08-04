export type WorkflowCategory =
  'Automation' | 'Debugging' | 'Demo Recording' | 'Interactive Development' | 'Reliability' | 'Quality'

export interface WorkflowPreset {
  id: string
  title: string
  category: WorkflowCategory
  summary: string
  highlights: string[]
  docPath: string
  updatedAt: string
  impactScore: number
  runtime: string
}

export const workflowPresets: WorkflowPreset[] = [
  {
    id: 'autonomous-error-loop',
    title: 'Autonomous Error Loop',
    category: 'Debugging',
    summary:
      'Watch console, network, and WebSocket failures together, patch code, refresh, and verify the fix in one run.',
    highlights: ['observe(errors)', 'observe(network_bodies)', 'interact(refresh)'],
    docPath: '/guides/debug-webapps/',
    updatedAt: '2026-03-01',
    impactScore: 98,
    runtime: '4-8 min'
  },
  {
    id: 'api-contract-watchdog',
    title: 'API Contract Watchdog',
    category: 'Quality',
    summary:
      'Capture real API traffic and validate payload shape, status expectations, and endpoint drift before release.',
    highlights: ['analyze(api_validation)', 'observe(network_waterfall)', 'generate(sarif)'],
    docPath: '/guides/api-validation/',
    updatedAt: '2026-02-25',
    impactScore: 92,
    runtime: '6-12 min'
  },
  {
    id: 'replay-to-test',
    title: 'Replay to Playwright Test',
    category: 'Demo Recording',
    summary:
      'Record a real user flow and generate deterministic Playwright tests with assertions and optional fixtures.',
    highlights: ['configure(recording_start)', 'generate(reproduction)', 'assertion scaffolding'],
    docPath: '/guides/demo-scripts/',
    updatedAt: '2026-02-28',
    impactScore: 95,
    runtime: '5-10 min'
  },
  {
    id: 'demo-readiness-check',
    title: 'Demo Readiness Check',
    category: 'Demo Recording',
    summary:
      'Run scripted click-throughs with visual captures and failure evidence so product demos stay stable under pressure.',
    highlights: ['interact(batch)', 'observe(screenshot)', 'error bundles'],
    docPath: '/guides/product-demos/',
    updatedAt: '2026-02-27',
    impactScore: 87,
    runtime: '3-7 min'
  },
  {
    id: 'interactive-dom-triage',
    title: 'Interactive DOM Triage',
    category: 'Interactive Development',
    summary:
      'Query live DOM state, inspect computed styles, and resolve selector ambiguity directly from natural language.',
    highlights: ['interact(query)', 'analyze(dom)', 'selector diagnostics'],
    docPath: '/reference/interact/',
    updatedAt: '2026-02-23',
    impactScore: 84,
    runtime: '2-6 min'
  },
  {
    id: 'performance-regression-scan',
    title: 'Performance Regression Scan',
    category: 'Reliability',
    summary:
      'Track and compare web vitals snapshots to detect regressions and surface likely offending changes quickly.',
    highlights: ['analyze(performance)', 'baseline diff', 'trace-ready evidence'],
    docPath: '/guides/performance/',
    updatedAt: '2026-03-02',
    impactScore: 90,
    runtime: '6-9 min'
  },
  {
    id: 'security-redaction-audit',
    title: 'Security Redaction Audit',
    category: 'Quality',
    summary: 'Audit captured traffic for leaked secrets, missing headers, and risky patterns with exportable findings.',
    highlights: ['analyze(security)', 'redaction checks', 'SARIF export'],
    docPath: '/guides/security-auditing/',
    updatedAt: '2026-02-26',
    impactScore: 89,
    runtime: '5-11 min'
  },
  {
    id: 'accessibility-sweep',
    title: 'Accessibility Sweep',
    category: 'Quality',
    summary: 'Run WCAG scans, inspect violations in context, and generate issue reports your team can triage fast.',
    highlights: ['analyze(accessibility)', 'annotation detail', 'report generation'],
    docPath: '/guides/accessibility/',
    updatedAt: '2026-02-21',
    impactScore: 82,
    runtime: '4-8 min'
  },
  {
    id: 'noise-aware-observability',
    title: 'Noise-aware Observability',
    category: 'Debugging',
    summary: 'Reduce recurring console noise with rules so high-signal errors stand out during incident response.',
    highlights: ['configure(noise_rule)', 'observe(logs)', 'persistent filters'],
    docPath: '/guides/noise-filtering/',
    updatedAt: '2026-02-20',
    impactScore: 80,
    runtime: '3-6 min'
  },
  {
    id: 'browser-automation-smoke',
    title: 'Browser Automation Smoke',
    category: 'Automation',
    summary:
      'Use robust selector flows for smoke checks against critical funnels and collect screenshot evidence on failure.',
    highlights: ['interact(navigate)', 'interact(click/type)', 'observe(errors)'],
    docPath: '/guides/automate-and-notify/',
    updatedAt: '2026-03-03',
    impactScore: 91,
    runtime: '4-9 min'
  },
  {
    id: 'websocket-incident-replay',
    title: 'WebSocket Incident Replay',
    category: 'Debugging',
    summary:
      'Trace reconnect storms and message drops with timeline context to isolate backend and client failure boundaries.',
    highlights: ['observe(websocket_events)', 'timeline correlation', 'state snapshots'],
    docPath: '/guides/websocket-debugging/',
    updatedAt: '2026-02-24',
    impactScore: 88,
    runtime: '7-14 min'
  },
  {
    id: 'uat-walkthrough-capture',
    title: 'UAT Walkthrough Capture',
    category: 'Automation',
    summary:
      'Capture PM-led acceptance flows, preserve each interaction, and convert into repeatable checks for release gates.',
    highlights: ['recording capture', 'playback verification', 'release evidence'],
    docPath: '/guides/resilient-uat/',
    updatedAt: '2026-02-22',
    impactScore: 86,
    runtime: '8-15 min'
  }
]
