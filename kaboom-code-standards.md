# Kaboom Code Standards

> Quality gate rules for this codebase. Reviewed automatically on every Edit/Write.
> Only flag clear violations — not style preferences.

## File Structure

- Max 800 lines per file. If a file exceeds this, it must be split.
- One concept per file. If a file has two unrelated concerns, split them.
- No orphan code. Dead code, commented-out blocks, and unused imports must be removed.
- File headers required: `// filename.go — Purpose summary.`

## Naming Conventions

- Functions: verb-phrase — `buildResponse`, `parseArgs`, `validateToken`.
- Types/structs: noun-phrase — `ToolHandler`, `QueryResult`, `SessionStore`.
- Booleans: predicate-phrase — `isReady`, `hasExpired`, `canRetry`.
- Constants: describe the value's purpose, not its content — `maxRetries` not `three`.
- Avoid abbreviations except well-known ones (URL, ID, HTTP, JSON, API, MCP).
- All JSON fields use snake_case. No exceptions.

### ToolHandler Naming Convention

- `tool*` for top-level MCP mode/action entry points (e.g. `toolObserve`, `toolConfigureClear`).
- `handle*` for sub-action dispatch handlers on subsidiary types (e.g. `handleRecordStart`).
- Unprefixed methods are internal utilities (e.g. `finalizeResponseEnrichment`).

## Error Handling

- Always handle errors explicitly. Never silently ignore error return values.
- Use structured error messages: `{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}`
- Errors should be actionable — tell the caller what went wrong and how to fix it.
- Use the `fail()` helper with appropriate error codes from `tools_errors.go`.

## Duplication & Reuse

- 3+ similar lines = extract a helper. If you see the same logic repeated, it should be a function.
- Before writing a new utility, check if one exists. Search the codebase for similar function signatures.
- Run `npx jscpd src/background src/popup --min-lines 3 --min-tokens 60` for TS duplication.

## Structural Patterns

- 3+ switch/case branches dispatching to similar logic → extract to a handler map or strategy pattern.
- 3+ sequential phases (setup, execute, cleanup) → use `commandBuilder` pattern (see `tools_interact_command_builder.go`).
- Nested callbacks or deeply indented logic (4+ levels) → extract inner blocks into named functions.
- God functions (50+ lines doing multiple things) → split into focused sub-functions.

## Go-Specific

- Zero production dependencies. No `go get` for production code.
- Append-only I/O on hot paths.
- Single-pass eviction (never loop-remove-recheck).
- Wire types (`wire_*.go` and `wire-*.ts`) are the source of truth for HTTP payloads. Changes to either side MUST update the counterpart.

## TypeScript-Specific

- No `any`. TypeScript strict mode, no implicit any.
- No dynamic imports in service worker (`background/`).
- No circular dependencies.
- Content scripts must be bundled (MV3 limitation).
- All `fetch()` needs try/catch + `response.ok` check.
- Run `make compile-ts` after ANY `src/` change.

## Testing

- Write tests first (TDD) when adding new functions.
- Use deterministic tests — no sleep-based timing, use mocks/fakes/controlled clocks.
- Each bug fix must include a regression test that fails before and passes after.
- Test the contract (inputs/outputs), not the implementation details.

## Security

- Never log secrets, tokens, API keys, or credentials.
- Validate all external input at system boundaries.
- All data stays local — no external transmission.

## MCP Architecture

- 5 tools only: observe, generate, configure, interact, analyze.
- New modes register in the dispatch registry (`*_registry.go`), not inline.
- Use `method()` wrapper for ToolHandler methods in registries.
- MCP responses use the canonical `internal/mcp/toolresp` package directly.
- WebSocket < 0.1ms, HTTP < 0.5ms performance budgets.

## Documentation Contract

- Every feature ships with docs in `docs/features/feature/<name>/`.
- Feature index must include `code_paths`, `test_paths`, `last_reviewed`.
- Refactors update the owning feature index `code_paths`, `test_paths`, and
  `last_reviewed` fields in the same change.
