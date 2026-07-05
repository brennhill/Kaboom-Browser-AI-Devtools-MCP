---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# describe_capabilities — Runtime Tool Discovery

**Status:** shipped
**Tool:** configure
**Mode:** describe_capabilities
**Since:** v0.7.12

## Problem

Kaboom exposes 5 tools with 140 modes and 100+ parameters. When an LLM calls `describe_capabilities` without filters, it receives a 42KB payload with every parameter dumped into every mode. The LLM cannot tell which parameters are relevant to a given mode — e.g., `observe/errors` returns 36 params including 18 that belong to screenshot, indexeddb, logs, etc.

This wastes tokens and causes LLMs to send irrelevant parameters, producing confusing error responses.

## Solution

### Per-mode parameter filtering

Each mode now declares exactly which parameters it accepts. When an LLM queries a specific tool/mode, only the relevant params are returned.

**Before:** `observe/errors` → 36 optional params (entire tool schema)
**After:** `observe/errors` → 2 optional params (`scope`, `limit`)

### Mode hints for discovery

Every mode has a one-line `Hint` string surfaced in summary mode, turning a bare name list into a navigable index.

**Before:**
```json
{"modes": ["errors", "error_bundles", "error_clusters"]}
```

**After:**
```json
{"modes": {
  "errors": "Raw JavaScript console errors",
  "error_bundles": "Pre-assembled debug context per error (error + network + actions + logs in time window)",
  "error_clusters": "Group errors by pattern to identify systemic issues"
}}
```

## Usage

### Summary — browse all tools and modes

```json
configure({what: "describe_capabilities", summary: true})
```

Returns `tool → { description, dispatch_param, modes: { mode: hint } }` for all 5 tools. Token-efficient index for routing.

### Tool-level — get all modes and params for one tool

```json
configure({what: "describe_capabilities", tool: "observe"})
```

Returns modes list, per-mode `required`/`optional` param arrays, and param metadata (type, enum, default) for the specified tool.

### Mode-level — get params for one specific mode

```json
configure({what: "describe_capabilities", tool: "observe", mode: "errors"})
```

Returns a flat structure:

```json
{
  "tool": "observe",
  "mode": "errors",
  "required": ["what"],
  "optional": ["limit", "scope"],
  "params": {
    "what": {"type": "string", "enum": [...]},
    "limit": {"type": "number", "default": "100, max 1000"},
    "scope": {"type": "string", "enum": ["current_page", "all"]}
  }
}
```

## Discovery Path

An LLM connecting for the first time follows this path:

1. **Initialize** → `serverInstructions` mentions `kaboom://capabilities` resource
2. **`kaboom://capabilities`** → "Runtime Discovery" section points to `describe_capabilities` with examples
3. **`describe_capabilities(summary=true)`** → mode index with one-line hints per mode
4. **`describe_capabilities(tool=X, mode=Y)`** → exact params for the intended operation

The configure tool description also explicitly mentions the feature:

> Discovery: describe_capabilities — list available modes and per-mode parameters for any tool. Filter with tool and mode params.

Tutorial snippets (`configure(what:"examples")`) include a filtering example.

## Error Handling

| Condition | Response |
|-----------|----------|
| `mode` without `tool` | Error: "'mode' requires 'tool' to be set" |
| Unknown `tool` | Error with hint listing valid tool names |
| Unknown `mode` | Error with hint listing valid modes for that tool |

## Architecture

### Key files

| File | Purpose |
|------|---------|
| `internal/tools/configure/mode_specs.go` | `toolModeSpecs` — per-mode `{Hint, Required, Optional}` for all 5 tools |
| `internal/tools/configure/capabilities.go` | `BuildCapabilitiesSummary`, `BuildCapabilitiesMap`, `BuildCapabilitiesForTool`, `FilterToolByMode` |
| `internal/tools/configure/mode_specs_test.go` | Validates specs match schemas, all modes have hints, no unknown params |
| `cmd/browser-agent/tools_configure.go` | `handleDescribeCapabilities` handler |
| `cmd/browser-agent/tools_configure_capabilities_test.go` | Handler integration tests |
| `cmd/browser-agent/playbooks.go` + `cmd/browser-agent/playbooks_*.go` | `capabilityIndex` plus capability-specific playbook content |
| `cmd/browser-agent/playbooks_resolver.go` | Canonical playbook URI resolution + alias normalization |
| `cmd/browser-agent/tools_configure_tutorial.go` | Tutorial snippet with filtering example |

### Data flow

```
describe_capabilities(tool, mode)
  → BuildCapabilitiesForTool(tools, toolName)
    → buildModeParams(toolName, modes, ...)
      → toolModeSpecs[toolName][mode]  // per-mode filtering
  → FilterToolByMode(toolCap, toolName, mode)
  → JSON response with only relevant params
```

### Adding a new mode

When a new mode is added to any tool schema:

1. Add an entry to `toolModeSpecs` in `mode_specs.go` with `Hint`, `Required`, and `Optional` fields
2. Run `go test ./internal/tools/configure/...` — the `TestToolModeSpecs_AllModesHaveSpecs` and `TestToolModeSpecs_AllModesHaveHints` tests will fail if the entry is missing
3. The `TestToolModeSpecs_NoUnknownParams` test will fail if you list a param that doesn't exist in the schema

## Payload Size Comparison

| Query | Before | After |
|-------|--------|-------|
| Full (no filters) | ~42 KB | ~42 KB (unchanged) |
| Summary | ~10 KB names | ~12 KB with hints |
| Single tool | ~5 KB | ~5 KB (with per-mode filtering) |
| Single mode | N/A | ~500 B |
