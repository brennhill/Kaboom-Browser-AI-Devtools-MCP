// doc.go — Package documentation for observation-driven CSP generation.
// Purpose: Implements CSP origin accumulation and policy generation from observed runtime resource usage.
// Why: Produces enforceable security policies grounded in real traffic instead of static guesswork.
// Docs: docs/features/feature/security-hardening/index.md

// Package csp accumulates the origins a page actually loaded and turns them into
// a Content-Security-Policy grounded in observed traffic rather than guesswork.
//
// Layout:
//   - types.go:    core models and constants
//   - store.go:    observation accumulation and bounded storage
//   - generate.go: policy generation pipeline
//   - helpers.go:  directive shaping, confidence and dev-pollution filtering
//   - audit.go:    the audit block echoed back in a Response
//   - tooling.go:  MCP/tool-facing adapters and override hooks
//
// csp depends on internal/security/policy for the audit trail and the
// human-review remediation text attached to session-only whitelist overrides.
package csp
