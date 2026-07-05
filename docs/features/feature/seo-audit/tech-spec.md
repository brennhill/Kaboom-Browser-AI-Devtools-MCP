---
feature: seo-audit
status: proposed
doc_type: tech-spec
feature_id: feature-seo-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: SEO Audit

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

SEO Audit adds `observe({what: "seo_audit"})` -- validates page SEO health by checking metadata, heading structure, links, images, and structured data. Implemented in inject.js using DOM queries and meta tag inspection.

## Key Components

**Audit Categories**:

1. **Metadata**: Checks for title tag (length 30-60 chars), meta description (length 120-160 chars), canonical URL, Open Graph tags (og:title, og:description, og:image), Twitter Card tags.

2. **Heading Structure**: Validates heading hierarchy (h1 present, only one h1, no skipped levels h1->h3). Counts headings per level.

3. **Links**: Identifies broken links (href="#" or href=""), external links without rel="noopener", links without descriptive text ("click here", "read more").

4. **Images**: Checks all images have alt text. Flags decorative images without empty alt (`alt=""`). Identifies large images (> 200KB) without lazy loading.

5. **Structured Data**: Detects JSON-LD schema markup. Validates basic structure (parseable JSON). Reports schema types found (Article, Product, Organization, etc.).

**Finding Classification**: `critical` (missing title/description), `high` (broken h1 hierarchy), `medium` (missing alt text), `low` (optimization opportunities).

## Data Flows

```
AI calls observe({what: "seo_audit"})
  |
  v
inject.js queries DOM
  -> Read title, meta tags
  -> Traverse headings (h1-h6)
  -> Query all links and images
  -> Parse JSON-LD scripts
  |
  v
Validate and classify findings
  |
  v
Return audit report
```

## Implementation Strategy

**Extension files**:
- `extension/lib/seo-audit.js` (new): DOM queries, validation logic
- Constants: Recommended title length (30-60), description length (120-160)

**Server files**:
- `cmd/browser-agent/queries.go`: Add handler

**Trade-offs**:
- Basic validation only (not full Lighthouse-level SEO audit)
- JSON-LD parsing only (not Microdata or RDFa)

## Edge Cases & Assumptions

- **No title tag**: Critical finding
- **Multiple h1 tags**: High severity (violates hierarchy)
- **JSON-LD parse errors**: Flagged as warning, not critical

## Risks & Mitigations

**Risk**: Large pages with 1000+ images take too long.
**Mitigation**: Cap image audit at 200 images. Remaining summarized.

## Dependencies

- DOM API
- JSON parsing for structured data

## Performance Considerations

| Metric | Target |
|--------|--------|
| Audit execution time | < 150ms |
| Memory impact | < 1MB |

## Security Considerations

- Read-only DOM access
- JSON parsing uses try/catch for safety
