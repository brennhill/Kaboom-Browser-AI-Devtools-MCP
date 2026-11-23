// Purpose: Package redaction — configurable regex-based scrubbing of sensitive data in MCP responses.
// Why: Reduces secret leakage risk in logs, diagnostics, and captured payloads before they reach AI clients.
// Docs: docs/features/feature/redaction-patterns/index.md

/*
Package redaction provides configurable pattern-based scrubbing of sensitive data
from MCP tool responses before they reach AI clients.

Key types:
  - RedactionEngine: thread-safe engine initialized at startup with compiled regex patterns.
  - RedactionPattern: a named regex pattern with replacement template.

Key functions:
  - NewRedactionEngine: compiles patterns from config and returns an immutable engine.
  - Redact: applies all patterns to a tool response, replacing matches with placeholders.
*/
package redaction
