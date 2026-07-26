// @ts-nocheck
/**
 * @fileoverview gokaboom-domain-contract.test.js — Guards site metadata/domain contracts.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

describe('gokaboom domain contracts', () => {
  test('site metadata generators emit gokaboom.dev and KaBOOM branding', () => {
    const astroConfig = read('gokaboom.dev/astro.config.mjs')
    const indexMarkdown = read('gokaboom.dev/src/pages/index.md.ts')
    const slugMarkdown = read('gokaboom.dev/src/pages/[...slug].md.ts')
    const nestedMarkdown = read('gokaboom.dev/src/pages/markdown/[...slug].md.ts')

    assert.match(astroConfig, /site:\s*'https:\/\/gokaboom\.dev'/)
    assert.match(astroConfig, /title:\s*'KaBOOM'/)
    assert.match(astroConfig, /alt:\s*'KaBOOM'/)
    assert.doesNotMatch(astroConfig, /cookwithgasoline\.com|STRUM Agentic Devtools/)

    assert.match(indexMarkdown, /canonical: https:\/\/gokaboom\.dev\//)
    assert.match(indexMarkdown, /'KaBOOM MCP'/)
    assert.doesNotMatch(indexMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(slugMarkdown, /canonical: https:\/\/gokaboom\.dev\$\{canonicalPath\}/)
    assert.match(slugMarkdown, /'KaBOOM MCP'/)
    assert.doesNotMatch(slugMarkdown, /cookwithgasoline\.com|STRUM MCP/)

    assert.match(nestedMarkdown, /canonical: https:\/\/gokaboom\.dev/)
    assert.match(nestedMarkdown, /'KaBOOM MCP'/)
    assert.doesNotMatch(nestedMarkdown, /cookwithgasoline\.com|STRUM MCP/)
  })

  test('content contract tooling uses gokaboom naming', () => {
    const newScriptPath = path.join(REPO_ROOT, 'scripts/docs/site/check-gokaboom-content-contract.mjs')
    const oldScriptPath = path.join(REPO_ROOT, 'scripts/docs/check-cookwithgasoline-content-contract.mjs')

    assert.ok(fs.existsSync(newScriptPath))
    assert.ok(!fs.existsSync(oldScriptPath))

    const packageJson = read('package.json')
    const scriptSource = read('scripts/docs/site/check-gokaboom-content-contract.mjs')

    assert.match(packageJson, /check-gokaboom-content-contract\.mjs/)
    assert.doesNotMatch(packageJson, /check-cookwithgasoline-content-contract\.mjs/)
    assert.match(scriptSource, /gokaboom\.dev/)
    assert.doesNotMatch(scriptSource, /cookwithgasoline\.com/)
  })

  test('public seo metadata uses KaBOOM and gokaboom.dev', () => {
    const seoMeta = read('gokaboom.dev/public/seo-meta.html')
    const robots = read('gokaboom.dev/public/robots.txt')

    assert.match(seoMeta, /KaBOOM/)
    assert.match(seoMeta, /https:\/\/gokaboom\.dev\//)
    assert.doesNotMatch(seoMeta, /STRUM|Strum|Gasoline|cookwithgasoline|getstrum/)

    assert.match(robots, /Kaboom/)
    assert.doesNotMatch(robots, /STRUM|Strum|Gasoline|cookwithgasoline|getstrum/)
  })

  test('site helper utilities use gokaboom.dev for canonical paths and analytics filtering', () => {
    const markdownPaths = read('gokaboom.dev/src/utils/markdownPaths.ts')
    const analytics = read('gokaboom.dev/src/components/Analytics.astro')

    assert.match(markdownPaths, /https:\/\/gokaboom\.dev/)
    assert.doesNotMatch(markdownPaths, /cookwithgasoline\.com/)

    assert.match(analytics, /gokaboom\.dev/)
    assert.doesNotMatch(analytics, /cookwithgasoline\.com/)
  })

  test('site install docs batch 1 uses Kaboom naming and gokaboom.dev contracts', () => {
    const files = [
      'gokaboom.dev/src/content/docs/agent-install-guide.md',
      'gokaboom.dev/src/content/docs/downloads.md',
      'gokaboom.dev/src/content/docs/getting-started.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|~\/\.gasoline|GasolineAgenticDevtoolExtension/
      )
    }
  })

  test('site docs batch 2 removes legacy brand copy', () => {
    const files = [
      'gokaboom.dev/src/content/docs/alternatives.md',
      'gokaboom.dev/src/content/docs/architecture.md',
      'gokaboom.dev/src/content/docs/discover-workflows.mdx',
      'gokaboom.dev/src/content/docs/execute-scripts.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|How\.gasoline|\.gasoline/
      )
    }
  })

  test('site docs batch 3 removes legacy brand copy', () => {
    const files = [
      'gokaboom.dev/src/content/docs/features.md',
      'gokaboom.dev/src/content/docs/privacy.md',
      'gokaboom.dev/src/content/docs/security.md',
      'gokaboom.dev/src/content/docs/troubleshooting.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-agentic-browser|gokaboom\.dev|kaboom-mcp/)
      assert.doesNotMatch(
        source,
        /STRUM|Strum|Gasoline|cookwithgasoline|getstrum|gasoline-agentic-browser|gasoline-agentic-devtools|\.gasoline|~\/\.gasoline|~\/\.strum|STRUM_TELEMETRY|usestrum/
      )
    }
  })

  test('mcp integration docs use KaBOOM branding and kaboom commands', () => {
    const files = [
      'gokaboom.dev/src/content/docs/mcp-integration/index.md',
      'gokaboom.dev/src/content/docs/mcp-integration/claude-code.md',
      'gokaboom.dev/src/content/docs/mcp-integration/claude-desktop.md',
      'gokaboom.dev/src/content/docs/mcp-integration/cursor.md',
      'gokaboom.dev/src/content/docs/mcp-integration/windsurf.md',
      'gokaboom.dev/src/content/docs/mcp-integration/zed.md',
      'gokaboom.dev/src/content/docs/mcp-integration/gemini.md',
      'gokaboom.dev/src/content/docs/mcp-integration/opencode.md',
      'gokaboom.dev/src/content/docs/mcp-integration/antigravity.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|kaboom-agentic-browser|"kaboom"/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('guides batch 4 uses KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/src/content/docs/guides/accessibility.md',
      'gokaboom.dev/src/content/docs/guides/noise-filtering.md',
      'gokaboom.dev/src/content/docs/guides/security-auditing.md',
      'gokaboom.dev/src/content/docs/guides/websocket-debugging.md',
      'gokaboom.dev/src/content/docs/guides/seo-analysis.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('reference and guide batch 5 use KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/src/content/docs/reference/index.md',
      'gokaboom.dev/src/content/docs/guides/debug-webapps.md',
      'gokaboom.dev/src/content/docs/guides/product-demos.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('guide batch 6 uses KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/src/content/docs/guides/demo-scripts.md',
      'gokaboom.dev/src/content/docs/guides/performance.md',
      'gokaboom.dev/src/content/docs/guides/replace-selenium.md',
      'gokaboom.dev/src/content/docs/guides/api-validation.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('guide batch 7 uses KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/src/content/docs/guides/automate-and-notify.md',
      'gokaboom.dev/src/content/docs/guides/resilient-uat.md',
      'gokaboom.dev/src/content/docs/guides/visual-evidence-standards.md',
      'gokaboom.dev/src/content/docs/guides/tracks/engineering.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('reference batch 8 uses KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/src/content/docs/reference/configure.md',
      'gokaboom.dev/src/content/docs/reference/generate.md',
      'gokaboom.dev/src/content/docs/reference/interact.md',
      'gokaboom.dev/src/content/docs/reference/observe.md',
      'gokaboom.dev/src/content/docs/guides/annotation-skill-terminal-workflow.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('reference examples batch 9 use KaBOOM naming', () => {
    const files = [
      'gokaboom.dev/src/content/docs/reference/examples/generate-examples.md',
      'gokaboom.dev/src/content/docs/reference/examples/interact-examples.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom|\.kaboom/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('blog batch 10 uses KaBOOM naming and Kaboom repo links', () => {
    const files = [
      'gokaboom.dev/src/content/docs/blog/v5-7-5-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-8-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v6-0-0-beta-release.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom-agentic-browser|Kaboom-Browser-AI-Devtools-MCP/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('remaining release posts use KaBOOM naming and Kaboom repo links', () => {
    const files = [
      'gokaboom.dev/src/content/docs/blog/v0-7-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v0-8-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v1-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v2-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v3-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v4-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-1-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-2-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-2-1-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-2-5-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-3-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-4-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-4-1-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-4-3-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-5-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-6-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-7-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-7-4-release.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom-agentic-browser|Kaboom-Browser-AI-Devtools-MCP/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('public assets batch 11 use KaBOOM branding', () => {
    const files = [
      'gokaboom.dev/public/llms.txt',
      'gokaboom.dev/public/demo/index.html',
      'gokaboom.dev/public/demo/chaos-atlas.html',
      'gokaboom.dev/public/demo/full-stack-lab.html',
      'gokaboom.dev/public/demo/incident-board.html'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom-agentic-browser|Kaboom-Browser-AI-Devtools-MCP|gokaboom\.dev/)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('site shell batch 12 uses KaBOOM branding and kaboom footer classes', () => {
    const files = [
      'gokaboom.dev/src/components/Landing.astro',
      'gokaboom.dev/src/components/Footer.astro',
      'gokaboom.dev/src/styles/custom.css',
      'gokaboom.dev/public/images/landing/overlay-tool-palette-snapshot.svg',
      'gokaboom.dev/public/images/landing/hover-widget-snapshot.svg',
      'gokaboom.dev/src/assets/diagrams/architecture-main.svg',
      'gokaboom.dev/src/assets/diagrams/seo-content-plan-funnel.svg',
      'gokaboom.dev/src/assets/diagrams/kaboom-live-spec-architecture.svg',
      'gokaboom.dev/src/assets/diagrams/kaboom-trace-spec-architecture.svg',
      'gokaboom.dev/src/assets/diagrams/kaboom-replay-spec-architecture.svg',
      'gokaboom.dev/src/assets/diagrams/kaboom-logs-spec-architecture.svg',
      'gokaboom.dev/src/assets/diagrams/demo-automation-spec-architecture.svg',
      'gokaboom.dev/src/assets/diagrams/demo-app-spec-navigation.svg'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom|gokaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline|gasoline-footer/
      )
    }

    const landing = read('gokaboom.dev/src/components/Landing.astro')
    assert.match(landing, /Kaboom-Browser-AI-Devtools-MCP/)

    const footer = read('gokaboom.dev/src/components/Footer.astro')
    const footerStyles = read('gokaboom.dev/src/styles/custom.css')
    assert.match(footer, /kaboom-footer/)
    assert.match(footerStyles, /kaboom-footer/)
    assert.doesNotMatch(footer, /gasoline-footer/)
    assert.doesNotMatch(footerStyles, /gasoline-footer/)
  })

  test('articles batch 13 uses KaBOOM naming in the first content cluster', () => {
    const files = [
      'gokaboom.dev/src/content/docs/articles.mdx',
      'gokaboom.dev/src/content/docs/articles/identify-render-blocking-assets-and-slow-routes.md',
      'gokaboom.dev/src/content/docs/articles/local-first-demo-recording-for-product-teams.md',
      'gokaboom.dev/src/content/docs/articles/production-debugging-runbook-with-kaboom.md',
      'gokaboom.dev/src/content/docs/articles/use-mcp-for-browser-aware-debugging.md',
      'gokaboom.dev/src/content/docs/articles/debug-websocket-real-time-apps.md',
      'gokaboom.dev/src/content/docs/articles/reproduce-works-locally-fails-in-ci-browser-bugs.md',
      'gokaboom.dev/src/content/docs/articles/build-reusable-qa-macros-with-batch-sequences.md',
      'gokaboom.dev/src/content/docs/articles/compare-error-states-across-releases.md',
      'gokaboom.dev/src/content/docs/articles/prevent-credential-and-pii-leaks-during-debugging.md',
      'gokaboom.dev/src/content/docs/articles/security-audit-for-browser-workflows.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('articles batch 14 uses KaBOOM naming and Kaboom install commands in setup guides', () => {
    const files = [
      'gokaboom.dev/src/content/docs/articles/how-to-install-kaboom-in-5-minutes.md',
      'gokaboom.dev/src/content/docs/articles/how-to-connect-kaboom-to-claude-code.md',
      'gokaboom.dev/src/content/docs/articles/how-to-connect-kaboom-to-cursor.md',
      'gokaboom.dev/src/content/docs/articles/how-to-connect-kaboom-to-windsurf.md',
      'gokaboom.dev/src/content/docs/articles/cursor-kaboom-interactive-web-development.md',
      'gokaboom.dev/src/content/docs/articles/claude-code-kaboom-fast-bug-triage-setup.md',
      'gokaboom.dev/src/content/docs/articles/how-to-use-kaboom-if-you-are-not-a-developer.md',
      'gokaboom.dev/src/content/docs/articles/how-to-record-your-first-product-demo-with-kaboom.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom-agentic-browser|Kaboom-Browser-AI-Devtools-MCP|\.kaboom|"kaboom"/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser-devtools-mcp|gasolineAgenticDevtoolExtension|\.gasoline/
      )
    }
  })

  test('articles batch 15 uses KaBOOM naming in the workflow guide cluster', () => {
    const files = [
      'gokaboom.dev/src/content/docs/articles/annotation-driven-ux-reviews-for-engineering-teams.md',
      'gokaboom.dev/src/content/docs/articles/api-validation-for-frontend-teams.md',
      'gokaboom.dev/src/content/docs/articles/catch-third-party-script-regressions-fast.md',
      'gokaboom.dev/src/content/docs/articles/core-web-vitals-regression-triage.md',
      'gokaboom.dev/src/content/docs/articles/debug-broken-forms-labels-aria-validation.md',
      'gokaboom.dev/src/content/docs/articles/detect-api-contract-drift-before-production.md',
      'gokaboom.dev/src/content/docs/articles/fix-login-redirect-loops-and-session-bugs.md',
      'gokaboom.dev/src/content/docs/articles/generate-csp-policy-from-real-traffic.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('articles batch 16 uses KaBOOM naming in the remaining article cluster', () => {
    const files = [
      'gokaboom.dev/src/content/docs/articles/generate-playwright-tests-from-real-user-sessions.md',
      'gokaboom.dev/src/content/docs/articles/how-to-capture-console-logs-and-network-without-devtools.md',
      'gokaboom.dev/src/content/docs/articles/how-to-run-your-first-browser-debug-session.md',
      'gokaboom.dev/src/content/docs/articles/how-to-share-reproducible-bug-evidence-with-your-team.md',
      'gokaboom.dev/src/content/docs/articles/how-to-use-annotations-to-explain-ui-bugs-clearly.md',
      'gokaboom.dev/src/content/docs/articles/mcp-tools-vs-traditional-test-runners.md',
      'gokaboom.dev/src/content/docs/articles/run-accessibility-audits-in-ci-and-export-sarif.md',
      'gokaboom.dev/src/content/docs/articles/visual-regression-testing-with-annotation-sessions.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /STRUM|Gasoline|cookwithgasoline|getstrum|gasoline-mcp|gasoline-agentic-browser|\.gasoline/
      )
    }
  })

  test('blog batch 17 removes final gasoline identifiers from release snippets', () => {
    const files = [
      'gokaboom.dev/src/content/docs/blog/v1-0-0-release.md',
      'gokaboom.dev/src/content/docs/blog/v5-5-0-release.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /KaBOOM|Kaboom|kaboom-agentic-browser|"kaboom"|\.kaboom/i)
      assert.doesNotMatch(source, /\bgasoline\b|Gasoline|gasoline-mcp|gasoline-agentic-browser|\.gasoline/)
    }
  })
})
