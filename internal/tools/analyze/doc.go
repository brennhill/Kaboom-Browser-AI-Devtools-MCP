// Purpose: Package analyze — implementation helpers for the analyze MCP tool's inspection and diff modes.
// Why: Centralizes analysis logic to keep handler behavior consistent across command paths.
// Docs: docs/features/feature/analyze-tool/index.md

/*
Package analyze provides the implementation for the analyze MCP tool, which performs
active analysis of browser state.

Key types:
  - Deps: interface declaring dependencies required from the host server.
  - ComputedStylesArgs: parsed arguments for computed styles queries.
  - FormsArgs: parsed arguments for form discovery queries.
  - LinkValidationParams: parameters for server-side link verification.

Key functions:
  - ParseComputedStylesArgs: validates computed styles query parameters.
  - ParseFormsArgs: validates form discovery query parameters.
  - ValidateLinksServerSide: validates batches of URLs concurrently with SSRF-safe transport.
  - ParseVisualBaselineArgs: validates visual baseline save/diff parameters.

Subpackages:
  - imagediff: pure-Go screenshot pixel diffing behind CompareImages/WriteDiffImage.
*/
package analyze
