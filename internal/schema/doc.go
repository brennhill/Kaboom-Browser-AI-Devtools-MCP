// Purpose: Package schema — MCP tool schema definitions for all five tools (observe, analyze, generate, configure, interact).
// Why: Keeps tool interfaces strict and synchronized across server, extension, and clients.
// Docs: docs/features/feature/api-schema/index.md

/*
Package schema defines the MCP tool input schemas for all five Kaboom tools and
assembles them into the tools/list response.

Tools whose parameter set is small enough to state in one file (observe,
analyze, generate) are defined here. The two tools with a multi-group parameter
surface own a subpackage each, so their property groups evolve without
enlarging this package:

  - internal/schema/configure: the configure tool schema and its core/runtime property groups.
  - internal/schema/interact: the interact tool schema, its five property groups, and the
    canonical interact action registry (interact.ActionSpecs) that internal/tools/configure
    reads to build describe_capabilities mode specs.

Key functions:
  - AllTools: returns all five tool definitions, in tools/list order.
  - configure.ToolSchema / interact.ToolSchema: the two subpackage tool definitions.
  - interact.ActionSpecs: canonical interact action registry (name, hint, required/optional params).
*/
package schema
