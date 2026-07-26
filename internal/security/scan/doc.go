// doc.go — Package documentation for the aggregate security scanner.
// Purpose: Security check implementations are split by concern into focused files.
// Why: Reduces monolithic check logic and improves maintainability/testability.
// Docs: docs/features/feature/redaction-patterns/index.md
// Docs: docs/features/feature/security-hardening/index.md

// Package scan runs the aggregate security audit over captured network bodies,
// waterfall entries and console output, and emits Findings with one coherent
// severity model.
//
// Layout:
//   - types.go:                 Finding, Input, Result and the Scanner handle
//   - scan.go:                  check dispatch, URL/severity filtering, summaries
//   - checks_cookies.go:        session-cookie attribute checks
//   - checks_headers.go:        required response security headers
//   - checks_pii.go:            PII pattern detection in payloads
//   - checks_transport_auth.go: transport encryption and unauthenticated PII
//   - checks_network.go:        adapts netflag origin flags into Findings
//   - credentials.go:           URL/body/console credential scanning pipeline
//   - credentials_patterns.go:  credential regex catalogue, redaction helpers
//
// Why credentials live here rather than in their own package: every check hangs
// off *Scanner and returns Finding, and scan.go's dispatch table plus the
// defaultChecks registry mean adding or removing a check is always a coordinated
// edit with the orchestrator. Splitting credentials out would either invert into
// an import cycle (credentials needs Finding, scan needs credentials) or force a
// one-type package to break it — indirection that removes no coordinated edit.
// The PII check also shares redactSecret with the credential scanner.
//
// scan depends on internal/security/httpsec for URL/cookie primitives and
// internal/security/netflag for suspicious-origin detection.
package scan
