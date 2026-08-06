# Framework Smoke Fixtures

This directory contains source fixtures used by smoke module `29-framework-selector-resilience.sh`.

Goals:
- Use real framework runtimes (React, Vue, Svelte, Next.js), not hand-faked DOM pages.
- Regenerate selector ids/classes on each page load to stress selector resilience.
- Keep fixture visual styling on-brand with Kaboom (including the flame mark) while preserving stable test semantics.
- Model hard automation failure modes:
  - delayed hydration and event binding
  - overlay interception (`Accept Cookies`)
  - SPA route remount churn (stale handles)
  - async/delayed content (`Async Save`)
  - virtualized/lazy deep targets (`Framework Target 80`)
- Keep stable user-facing semantics (`placeholder=Enter name`, `text=Submit Profile`) for agent workflows.

Build command:

```bash
npm run smoke:build-framework-fixtures
```

Parity gate command (full scenario repeated 3x per framework):

```bash
npm run smoke:framework-parity
```

Combined framework + annotation parity suite:

```bash
npm run smoke:annotation-parity-suite
```

Repeated benchmark with pass-rate threshold:

```bash
npm run smoke:annotation-parity-benchmark
```

Optional repeat controls:

```bash
FRAMEWORK_RESILIENCE_FULL_REPEATS=3 FRAMEWORK_SELECTOR_REFRESH_CYCLES=3 bash scripts/uat/runners/smoke-test.sh --only 29
```

Generated output is written to:

- `cmd/browser-agent/internal/testpages/pages/frameworks/react.html`
- `cmd/browser-agent/internal/testpages/pages/frameworks/vue.html`
- `cmd/browser-agent/internal/testpages/pages/frameworks/svelte.html`
- `cmd/browser-agent/internal/testpages/pages/frameworks/next/`
- corresponding `*.bundle.js` assets
