---
title: SEO Analysis Workflow
description: Run practical SEO checks with KaBOOM by comparing rendered metadata, crawl behavior, and on-page quality signals across your site and competitors.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'seo', 'analysis']
---

# SEO Analysis Workflow

This workflow is for teams that want quick, evidence-based SEO checks from real rendered pages.

## What You Can Validate with KaBOOM

- Page titles and descriptions from rendered DOM
- Heading structure consistency (`h1`, `h2`, `h3`)
- Broken links and redirect chains
- Third-party script weight that hurts search performance
- Core loading behavior that affects crawlability and user experience

## Practical Runbook

1. Start from a target page and 1-2 competitor pages.
2. Use `interact({what:'navigate'})` to open each page.
3. Use `analyze({what:'dom'})` to inspect key elements and metadata.
4. Use `analyze({what:'link_health'})` to detect broken links and status issues.
5. Use `observe({what:'network_waterfall'})` for heavy resources and bottlenecks.
6. Record differences in a shared issue and prioritize by impact.

## Why This Works

KaBOOM checks the page after scripts, styles, and client rendering have loaded. That gives you a more realistic view than static HTML alone.

## Next Steps

- For deeper API behavior checks, continue with [/guides/api-validation/](/guides/api-validation/).
- For performance bottlenecks, continue with [/guides/performance/](/guides/performance/).
