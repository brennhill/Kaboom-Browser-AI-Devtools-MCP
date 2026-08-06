#!/usr/bin/env node

import { build } from 'esbuild'
import { compile } from 'svelte/compiler'
import { mkdir, readFile, rm, writeFile, cp } from 'node:fs/promises'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const execFileAsync = promisify(execFile)
const thisFile = fileURLToPath(import.meta.url)
const smokeDir = path.dirname(thisFile)
const projectRoot = path.resolve(smokeDir, '..', '..', '..')
const sourceDir = path.join(smokeDir, 'framework-fixtures')
const harnessRootDir = path.join(projectRoot, 'cmd', 'browser-agent', 'internal', 'testpages', 'pages')
const outputDir = path.join(harnessRootDir, 'frameworks')
const tempDir = path.join(projectRoot, '.tmp-framework-fixtures')
const nextFixtureAppDir = path.join(sourceDir, 'next-app')

function pageTemplate({ title, mountId, scriptName }) {
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>${title}</title>
  <style>
    :root {
      --kaboom-ink: #17171d;
      --kaboom-muted: #4b5563;
      --kaboom-warm-100: #fff7ed;
      --kaboom-warm-200: #ffedd5;
      --kaboom-warm-300: #fed7aa;
      --kaboom-warm-500: #f97316;
      --kaboom-warm-600: #ea580c;
      --kaboom-panel: #ffffff;
      --kaboom-border: #fdba74;
      --kaboom-shadow: rgba(234, 88, 12, 0.18);
    }
    body {
      margin: 0;
      font-family: "Avenir Next", "Inter", "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top right, rgba(249, 115, 22, 0.12), transparent 38%),
        linear-gradient(140deg, var(--kaboom-warm-100), #fff);
      color: var(--kaboom-ink);
      min-height: 100vh;
      padding: 1.6rem 1.2rem 2.4rem;
    }
    .kaboom-brand {
      width: min(100%, 760px);
      margin: 0 auto 0.9rem;
      display: flex;
      align-items: center;
      gap: 0.65rem;
      color: #7c2d12;
      font-size: 0.9rem;
      font-weight: 700;
      letter-spacing: 0.02em;
    }
    .kaboom-brand-mark {
      width: 22px;
      height: 22px;
      flex: 0 0 22px;
    }
    .kaboom-brand small {
      color: var(--kaboom-muted);
      font-weight: 600;
    }
    .fixture-shell {
      line-height: 1.4;
      max-width: 760px;
      margin: 0 auto;
      padding: 1.25rem 1.25rem 1.3rem;
      border: 1px solid var(--kaboom-border);
      border-radius: 14px;
      background: var(--kaboom-panel);
      box-shadow: 0 14px 32px -24px var(--kaboom-shadow);
    }
    .fixture-shell h1 {
      margin-top: 0;
      margin-bottom: 0.3rem;
      color: #9a3412;
      font-size: 1.44rem;
      letter-spacing: -0.01em;
    }
    .fixture-shell p {
      color: var(--kaboom-muted);
    }
    .fixture-shell label {
      display: block;
      font-weight: 600;
      margin-top: 0.75rem;
      color: #9a3412;
    }
    .fixture-shell input {
      width: 100%;
      max-width: 360px;
      padding: 0.5rem 0.6rem;
      border: 1px solid #fdba74;
      border-radius: 6px;
      background: #fffefc;
      color: var(--kaboom-ink);
    }
    .fixture-shell button {
      display: inline-block;
      margin-top: 0.8rem;
      padding: 0.55rem 1rem;
      border: none;
      border-radius: 6px;
      background: linear-gradient(180deg, var(--kaboom-warm-500), var(--kaboom-warm-600));
      color: #fff;
      cursor: pointer;
      font-weight: 700;
      box-shadow: 0 6px 16px -10px rgba(154, 52, 18, 0.75);
    }
    .fixture-nav {
      display: flex;
      gap: 0.5rem;
      margin-bottom: 0.75rem;
    }
    .fixture-nav button {
      margin-top: 0.15rem;
    }
    #virtual-list {
      margin-top: 1rem;
      max-width: 420px;
      height: 140px;
      overflow: auto;
      border: 1px solid #fdba74;
      border-radius: 8px;
      padding: 0.5rem;
      background: #fffbf6;
    }
    .virtual-row {
      font-size: 0.88rem;
      color: #6b7280;
      padding: 0.2rem 0;
    }
    #consent-modal {
      position: fixed;
      inset: 0;
      background: rgba(23, 23, 29, 0.55);
      z-index: 2147483600;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-direction: column;
      color: #fff;
      gap: 0.6rem;
      padding: 1rem;
      text-align: center;
    }
    #consent-modal button {
      background: linear-gradient(180deg, var(--kaboom-warm-500), var(--kaboom-warm-600));
      margin-top: 0;
    }
  </style>
</head>
<body>
  <header class="kaboom-brand">
    <svg class="kaboom-brand-mark" viewBox="0 0 128 128" aria-hidden="true" focusable="false">
      <defs>
        <linearGradient id="brandFlame" x1="0%" y1="100%" x2="0%" y2="0%">
          <stop offset="0%" stop-color="#f97316"></stop>
          <stop offset="55%" stop-color="#fb923c"></stop>
          <stop offset="100%" stop-color="#fbbf24"></stop>
        </linearGradient>
        <linearGradient id="brandInnerFlame" x1="0%" y1="100%" x2="0%" y2="0%">
          <stop offset="0%" stop-color="#fbbf24"></stop>
          <stop offset="100%" stop-color="#fef3c7"></stop>
        </linearGradient>
      </defs>
      <circle cx="64" cy="64" r="60" fill="#121212"></circle>
      <path d="M64 16 C40 40, 28 60, 28 80 C28 100, 44 116, 64 116 C84 116, 100 100, 100 80 C100 60, 88 40, 64 16 Z" fill="url(#brandFlame)"></path>
      <path d="M64 48 C52 60, 44 72, 44 84 C44 96, 52 104, 64 104 C76 104, 84 96, 84 84 C84 72, 76 60, 64 48 Z" fill="url(#brandInnerFlame)"></path>
    </svg>
    <span>Kaboom Framework Smoke</span>
    <small>${title}</small>
  </header>
  <div id="${mountId}"></div>
  <script defer src="./${scriptName}"></script>
</body>
</html>
`
}

async function buildReactBundle() {
  await build({
    entryPoints: [path.join(sourceDir, 'react-entry.jsx')],
    outfile: path.join(outputDir, 'react.bundle.js'),
    bundle: true,
    minify: true,
    sourcemap: false,
    format: 'iife',
    platform: 'browser',
    target: ['es2020'],
    define: {
      'process.env.NODE_ENV': '"production"'
    }
  })
}

async function buildVueBundle() {
  await build({
    entryPoints: [path.join(sourceDir, 'vue-entry.js')],
    outfile: path.join(outputDir, 'vue.bundle.js'),
    bundle: true,
    minify: true,
    sourcemap: false,
    format: 'iife',
    platform: 'browser',
    target: ['es2020']
  })
}

async function buildSvelteBundle() {
  const source = await readFile(path.join(sourceDir, 'SmokeSvelteApp.svelte'), 'utf8')
  const compiled = compile(source, {
    filename: 'SmokeSvelteApp.svelte',
    generate: 'dom',
    css: 'injected',
    dev: false,
    compatibility: {
      componentApi: 4
    }
  })

  const compiledPath = path.join(tempDir, 'SmokeSvelteApp.compiled.js')
  const entryPath = path.join(tempDir, 'svelte-entry.js')

  await writeFile(compiledPath, compiled.js.code, 'utf8')
  await writeFile(
    entryPath,
    `import App from './SmokeSvelteApp.compiled.js'
const mount = document.getElementById('svelte-root')
if (!mount) {
  throw new Error('missing #svelte-root mount node')
}
new App({ target: mount })
`,
    'utf8'
  )

  await build({
    entryPoints: [entryPath],
    outfile: path.join(outputDir, 'svelte.bundle.js'),
    bundle: true,
    minify: true,
    sourcemap: false,
    format: 'iife',
    platform: 'browser',
    target: ['es2020']
  })
}

async function buildNextFixture() {
  const env = {
    ...process.env,
    NEXT_TELEMETRY_DISABLED: '1'
  }
  const nextOutDir = path.join(nextFixtureAppDir, 'out')
  const nextOutStaticDir = path.join(nextOutDir, '_next')
  const harnessStaticDir = path.join(harnessRootDir, '_next')
  await rm(path.join(nextFixtureAppDir, '.next'), { recursive: true, force: true })
  await rm(nextOutDir, { recursive: true, force: true })

  await execFileAsync('npx', ['next', 'build'], {
    cwd: nextFixtureAppDir,
    env
  })

  await rm(path.join(outputDir, 'next'), { recursive: true, force: true })
  await cp(nextOutDir, path.join(outputDir, 'next'), { recursive: true })
  await rm(harnessStaticDir, { recursive: true, force: true })
  await cp(nextOutStaticDir, harnessStaticDir, { recursive: true })
}

async function writeHtmlFixtures() {
  await writeFile(
    path.join(outputDir, 'react.html'),
    pageTemplate({ title: 'React Fixture', mountId: 'react-root', scriptName: 'react.bundle.js' }),
    'utf8'
  )
  await writeFile(
    path.join(outputDir, 'vue.html'),
    pageTemplate({ title: 'Vue Fixture', mountId: 'vue-root', scriptName: 'vue.bundle.js' }),
    'utf8'
  )
  await writeFile(
    path.join(outputDir, 'svelte.html'),
    pageTemplate({ title: 'Svelte Fixture', mountId: 'svelte-root', scriptName: 'svelte.bundle.js' }),
    'utf8'
  )
  await writeFile(
    path.join(outputDir, 'index.html'),
    `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Framework Fixture Index</title>
  <style>
    :root {
      --kaboom-ink: #17171d;
      --kaboom-muted: #4b5563;
      --kaboom-warm-100: #fff7ed;
      --kaboom-warm-500: #f97316;
      --kaboom-warm-600: #ea580c;
      --kaboom-border: #fdba74;
      --kaboom-shadow: rgba(234, 88, 12, 0.18);
    }
    body {
      margin: 0;
      font-family: "Avenir Next", "Inter", "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top right, rgba(249, 115, 22, 0.12), transparent 38%),
        linear-gradient(140deg, var(--kaboom-warm-100), #fff);
      color: var(--kaboom-ink);
      min-height: 100vh;
      padding: 1.8rem 1.2rem 2.5rem;
    }
    .index-shell {
      width: min(100%, 760px);
      margin: 0 auto;
      border: 1px solid var(--kaboom-border);
      border-radius: 14px;
      background: #fff;
      box-shadow: 0 14px 32px -24px var(--kaboom-shadow);
      padding: 1.25rem;
    }
    .kaboom-brand {
      display: flex;
      align-items: center;
      gap: 0.65rem;
      margin-bottom: 0.8rem;
      color: #7c2d12;
      font-size: 0.95rem;
      font-weight: 700;
      letter-spacing: 0.02em;
    }
    .kaboom-brand-mark {
      width: 22px;
      height: 22px;
      flex: 0 0 22px;
    }
    h1 {
      margin: 0 0 0.35rem;
      color: #9a3412;
      font-size: 1.5rem;
      letter-spacing: -0.01em;
    }
    p {
      margin-top: 0;
      color: var(--kaboom-muted);
    }
    ul {
      margin: 0;
      padding-left: 1.1rem;
    }
    li + li {
      margin-top: 0.5rem;
    }
    a {
      color: var(--kaboom-warm-600);
      font-weight: 700;
      text-decoration: none;
    }
    a:hover {
      text-decoration: underline;
    }
  </style>
</head>
<body>
  <main class="index-shell">
    <header class="kaboom-brand">
      <svg class="kaboom-brand-mark" viewBox="0 0 128 128" aria-hidden="true" focusable="false">
        <defs>
          <linearGradient id="indexBrandFlame" x1="0%" y1="100%" x2="0%" y2="0%">
            <stop offset="0%" stop-color="#f97316"></stop>
            <stop offset="55%" stop-color="#fb923c"></stop>
            <stop offset="100%" stop-color="#fbbf24"></stop>
          </linearGradient>
          <linearGradient id="indexBrandInnerFlame" x1="0%" y1="100%" x2="0%" y2="0%">
            <stop offset="0%" stop-color="#fbbf24"></stop>
            <stop offset="100%" stop-color="#fef3c7"></stop>
          </linearGradient>
        </defs>
        <circle cx="64" cy="64" r="60" fill="#121212"></circle>
        <path d="M64 16 C40 40, 28 60, 28 80 C28 100, 44 116, 64 116 C84 116, 100 100, 100 80 C100 60, 88 40, 64 16 Z" fill="url(#indexBrandFlame)"></path>
        <path d="M64 48 C52 60, 44 72, 44 84 C44 96, 52 104, 64 104 C76 104, 84 96, 84 84 C84 72, 76 60, 64 48 Z" fill="url(#indexBrandInnerFlame)"></path>
      </svg>
      <span>Kaboom Framework Smoke</span>
    </header>
    <h1>Framework Fixture Index</h1>
    <p>Selector resilience fixtures with unified Kaboom branding.</p>
    <ul>
      <li><a href="./react.html">React fixture</a></li>
      <li><a href="./vue.html">Vue fixture</a></li>
      <li><a href="./svelte.html">Svelte fixture</a></li>
      <li><a href="./next/">Next fixture</a></li>
    </ul>
  </main>
</body>
</html>
`,
    'utf8'
  )
}

async function main() {
  await mkdir(outputDir, { recursive: true })
  await rm(tempDir, { recursive: true, force: true })
  await mkdir(tempDir, { recursive: true })

  await Promise.all([buildReactBundle(), buildVueBundle(), buildSvelteBundle()])
  await buildNextFixture()
  await writeHtmlFixtures()
  await rm(tempDir, { recursive: true, force: true })

  console.log(`framework fixtures built in ${outputDir}`)
}

main().catch((error) => {
  console.error('failed to build framework fixtures:', error)
  process.exit(1)
})
