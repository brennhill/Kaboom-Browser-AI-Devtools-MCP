---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Feature Documentation Index

Each feature in `/docs/features/feature/` should include:
- Product spec (user-facing, requirements, deprecations)
- Technical spec (implementation, code mapping)
- ADRs (if any)

## Example Structure for a Feature

```
/docs/features/feature/noise-filtering/
  - product-spec.md
  - tech-spec.md
  - ADRS.md (if any)
```

## Cross-links
- Each feature spec should link to its ADR(s) and core specs as appropriate.
- Core specs should reference features where relevant.
