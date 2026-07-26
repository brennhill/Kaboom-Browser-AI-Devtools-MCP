// Purpose: Package capabilities — builds the describe_capabilities response by introspecting MCP tool schemas.
// Why: This code is about every tool, not about configuring one, so it sits beside the
// configure tool rather than inside it. It shares no symbol with configure's own audit,
// boundary and argument-rewrite helpers.
// Docs: docs/features/feature/config-profiles/index.md
// Docs: docs/features/describe_capabilities.md

/*
Package capabilities turns a set of MCP tool schemas into the machine-readable
capability metadata returned by configure(action="describe_capabilities").

Key functions:
  - BuildCapabilitiesSummary: compact per-tool summary (description, dispatch param, mode hints).
  - BuildCapabilitiesMap: full per-tool metadata including param details and per-mode params.
  - BuildCapabilitiesForTool: the full map for a single named tool.
  - FilterToolByMode: flattens one mode's entry out of a tool's capability map.

File layout:
  - capabilities.go: the four entry points and the assembly that combines schema and specs.
  - schema.go: all raw JSON-schema reading (dispatch param, mode enum, param details).
  - modespecs.go plus one file per MCP tool: the static per-mode required/optional tables.
*/
package capabilities
