---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Development Guide

This guide covers local development setup, quality checks, and CI pipeline for Kaboom.

## Quick Start

```bash
# Install dependencies
npm ci

# Install git hooks (recommended)
make install-hooks

# Run all checks
make ci-local
```

## Quality Gates

### Linting

Run all linting checks:

```bash
make lint
```

Individual checks:

```bash
# Go linting
make lint-go          # go vet + golangci-lint

# JavaScript linting
make lint-js          # ESLint with security rules
npx eslint extension/ tests/extension/
```

### Security Checks

Run security analysis:

```bash
make security-check
```

This runs:
- **gosec** on Go code (HIGH severity threshold)
- **ESLint security plugin** on JavaScript code

Install the pinned Go security scanners used by CI:

```bash
make install-security-tools
```

### Testing

```bash
# Go tests
make test-go-quick     # Fast lane: internal + sharded cmd tests (-short)
make test-go-long      # Full Go test suite (sharded cmd tests)
make test              # Alias for test-go-long
make test-race         # Go tests with race detection
make test-cover        # Go tests with coverage (95% minimum)

# JavaScript tests
make test-js           # Extension unit tests

# All tests
make test-all          # Go + JavaScript tests
```

### Full Verification

Run all quality gates:

```bash
make verify-all        # lint + security + tests with coverage
```

## Git Hooks

### Installation

```bash
make install-hooks
```

This installs:
- **pre-commit**: ESLint, go vet, gosec (on staged files)
- **pre-push**: Full ci-local suite

### Pre-commit Hook

Runs automatically before each commit:
- ESLint on JavaScript files (errors and warnings)
- go vet on Go files
- gosec on Go files (HIGH severity only)

### Pre-push Hook

Runs automatically before each push:
- Full `make ci-local` suite

## CI Pipeline

The GitHub Actions CI workflow (`.github/workflows/ci.yml`) runs on:
- Push to `stable` or `UNSTABLE` branches
- Pull requests to `stable`
- Nightly schedule (6 AM UTC)
- Manual workflow dispatch

### Jobs

| Job | Description | Gate |
|-----|-------------|------|
| `go` | Go vet, race tests, coverage | 95% coverage minimum |
| `javascript` | ESLint, Prettier, TypeScript, tests | All warnings fail |
| `security` | gosec, ESLint security | HIGH severity fails |
| `build` | Multi-platform build | 15MB binary size limit |
| `fuzz` | Fuzz testing (nightly only) | 30s per target |

### Local CI Mirror

Run the same checks locally:

```bash
make ci-local          # Runs ci-go, ci-js, ci-security
```

## Common Tasks

### Fix Linting Issues

```bash
# Auto-fix formatting
make format-fix

# Manual ESLint fixes
npx eslint extension/ --fix
```

### Security Audit

```bash
# Full security check
make security-check

# Verbose gosec output
gosec -exclude=G104,G114,G204,G301,G304,G306 ./cmd/browser-agent/
```

### Update Dependencies

```bash
# JavaScript
npm update

# Verify no Go dependencies
make verify-zero-deps
```

## Makefile Targets Reference

| Target | Description |
|--------|-------------|
| `test-go-quick` | Fast Go lane (`-short`) with sharded `cmd/browser-agent` |
| `test-go-long` | Full Go lane with sharded `cmd/browser-agent` |
| `test-go-sharded` | Run only `cmd/browser-agent` tests in parallel shards |
| `lint` | Run all linters (lint-go + lint-js) |
| `lint-go` | Go vet + golangci-lint |
| `lint-js` | ESLint on extension code |
| `security-check` | gosec + ESLint security rules |
| `pre-commit` | lint + security-check |
| `verify-all` | lint + security + tests with coverage |
| `ci-local` | Full local CI suite |
| `install-hooks` | Install git hooks |

## Troubleshooting

### ESLint security/detect-object-injection Warnings

This rule flags dynamic property access. Safe patterns require explicit disable comments:

```javascript
// eslint-disable-next-line security/detect-object-injection -- key from Object.keys iteration
result[key] = value
```

### gosec Not Found

Install the same pinned scanners used by CI:

```bash
make install-security-tools
```

### Coverage Below 95%

Add tests for uncovered code paths. Check coverage report:

```bash
go test -coverprofile=coverage.out ./cmd/browser-agent/...
go tool cover -html=coverage.out
```
