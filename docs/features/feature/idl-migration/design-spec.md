---
status: draft
priority: tier-2
relates-to: [normalized-event-schema]
blocks: []
last-updated: 2026-02-28
issue: 303
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# IDL Migration for Go/TS Boundary — Design Specification

**Goal:** Replace hand-maintained Go structs and TypeScript interfaces with a single
Interface Definition Language (IDL) source of truth, generating both sides automatically.

---

## 1. Current State

### 1.1 What Exists Today

The Go/TS boundary has two distinct contract surfaces:

**Wire types** (data payloads between extension and daemon):
- `internal/types/wire_enhanced_action.go` -> `src/types/wire/wire-enhanced-action.ts`
- `internal/types/wire_network.go` -> `src/types/wire/wire-network.ts`
- `internal/types/wire_websocket_event.go` -> `src/types/wire/wire-websocket-event.ts`
- `internal/performance/wire_performance.go` -> `src/types/wire/wire-performance-snapshot.ts`

These are already partially automated. Go structs are the source of truth.
`scripts/build/generate-wire-types.js` generates the TS side. `scripts/contracts/check-wire-drift.js`
validates alignment. The TS files carry a `// THIS FILE IS GENERATED` header.

**Tool schemas** (MCP tool input schemas, served to LLM clients):
- `internal/schema/observe.go` — 200 lines of `map[string]any` literals
- `internal/schema/interact.go` — 347 lines of `map[string]any` literals
- `internal/schema/configure.go` — 271 lines of `map[string]any` literals
- `internal/schema/analyze.go` — 159 lines of `map[string]any` literals
- `internal/schema/generate.go` — 196 lines of `map[string]any` literals

These are Go-only (served via MCP JSON-RPC). No TS generation needed, but they are
the largest source of untyped, error-prone code in the project. Every property is a
`map[string]any` with string-keyed type/description/enum values — essentially
hand-written JSON Schema in Go maps.

**Extension-internal types** (not crossing the wire, TS-only):
- `src/background/dom/dom-types.ts` — `DOMResult`, `DOMActionParams`, etc.

These are TS-only and do not need IDL coverage.

### 1.2 Pain Points

1. **Untyped schema definitions.** Tool schemas use `map[string]any` 301 times across
   schema files. A typo in a property name, type string, or enum value compiles fine
   but breaks at runtime. The invariants tests (`invariants_test.go`) catch structural
   issues (no combiners, valid JSON round-trip) but cannot verify semantic correctness.

2. **Manual wire sync overhead.** Adding a field to a wire type requires editing the Go
   struct, then either running the generator or manually editing TS. The generator
   (`generate-wire-types.js`) is 406 lines with hardcoded override tables
   (`TYPE_OVERRIDES`, `OPTIONAL_OVERRIDES`, `SERVER_ONLY_COMMENTS`,
   `STRUCT_COMMENT_OVERRIDES`, `FILE_DESCRIPTIONS`) that must be maintained alongside
   the Go source.

3. **Two validation scripts doing similar work.** `generate-wire-types.js --check` and
   `check-wire-drift.js` both validate Go/TS alignment but use different parsing
   strategies and different configuration. Adding a new wire type pair requires updating
   both scripts' `WIRE_PAIRS` arrays.

4. **No response type contracts.** Wire types define request shapes, but server response
   shapes (the JSON returned by tool handlers) have no schema. They are ad-hoc
   `map[string]any` built in each handler.

5. **Schema/dispatch enum divergence risk.** `interactActions` in `interact.go` defines
   the enum for `what`, but the actual dispatch `switch` in the handler is a separate
   list. Tests verify alignment, but this is defensive rather than structural.

---

## 2. IDL Format Evaluation

### 2.1 Candidates

| Format | Go codegen | TS codegen | JSON Schema output | Learning curve | Dep footprint |
|--------|-----------|-----------|-------------------|---------------|---------------|
| **Protocol Buffers** | Excellent (protoc-gen-go) | Good (ts-proto, protobuf-ts) | Indirect (proto -> JSON Schema plugin) | Medium | protoc binary + plugins |
| **JSON Schema** | Fair (go-jsonschema, quicktype) | Good (json-schema-to-typescript) | Native | Low | Node-only tooling |
| **Custom DSL** | Full control | Full control | Custom emitter | Low (project-specific) | Zero external deps |
| **TypeSpec** | Community plugin | Native | Native | Medium | Node runtime |
| **CUE** | Native Go integration | Limited | Native | Medium-high | Go binary |

### 2.2 Recommendation: JSON Schema as IDL

**Primary choice: JSON Schema files as the single source of truth.**

Rationale:

1. **Zero new binary deps.** Aligns with the project's zero-deps philosophy. Code
   generators can be Node scripts (already in the toolchain) or Go `go generate`
   directives. No `protoc` binary or plugin ecosystem to manage.

2. **Already the output format.** MCP tool schemas are JSON Schema. The current code
   hand-writes JSON Schema as Go maps. Moving the source of truth to `.json` files
   means the Go schemas become generated constants, not hand-written maps.

3. **Mature TS codegen.** `json-schema-to-typescript` is battle-tested and produces
   readonly interfaces matching current TS conventions.

4. **Go codegen is straightforward.** For wire types, we need Go structs with json tags.
   A custom Go generator (< 300 lines) can read JSON Schema and emit structs. For tool
   schemas, we emit `map[string]any` constants from the JSON directly — or better, embed
   the JSON files and deserialize at init time.

5. **Human-readable and diffable.** JSON Schema files are easy to review in PRs.
   Property additions are single-line diffs.

### 2.3 Rejected Alternatives

**Protocol Buffers:** Adds a binary toolchain dependency (`protoc`). Proto3 does not
natively express JSON Schema constraints (enum descriptions, number ranges). Would
require a proto-to-JSON-Schema bridge for MCP schema output, adding complexity.

**Custom DSL:** Attractive for zero deps, but invents a language. Maintenance burden
shifts from "sync two files" to "maintain a parser and two emitters." Not worth it
for the current ~15 wire types and ~5 tool schemas.

**CUE:** Excellent type system, but limited TS ecosystem. Would require custom TS
emitter. Better suited for configuration validation than API contracts.

---

## 3. Code Generation Strategy

### 3.1 Source Files

```
idl/
  wire/
    enhanced-action.schema.json
    network.schema.json
    websocket-event.schema.json
    performance.schema.json
  tools/
    observe.schema.json
    interact.schema.json
    configure.schema.json
    analyze.schema.json
    generate.schema.json
```

### 3.2 What Gets Generated

| Source | Generated artifact | Replaces |
|--------|-------------------|----------|
| `idl/wire/*.schema.json` | `internal/types/wire_*.go` (Go structs) | Hand-written Go wire structs |
| `idl/wire/*.schema.json` | `src/types/wire/wire-*.ts` (TS interfaces) | Generated TS interfaces |
| `idl/tools/*.schema.json` | `internal/schema/*_gen.go` (schema constants) | Hand-written `map[string]any` |

### 3.3 What Stays Manual

- **Tool handler dispatch logic** (`internal/tools/*/handler.go`) — business logic, not schema.
- **Extension-internal TS types** (`dom-types.ts`, etc.) — not crossing the wire.
- **MCP protocol types** (`internal/mcp/types.go`) — external spec, not project-controlled.
- **Server-only enrichment fields** — documented as comments in generated TS (current pattern preserved).
- **Invariant tests** (`invariants_test.go`) — still validate generated output.

### 3.4 Generator Implementation

Two generators, both Node scripts (consistent with existing `generate-wire-types.js`):

**`scripts/generate-from-idl.js`** (replaces `generate-wire-types.js`):
- Reads `idl/wire/*.schema.json`
- Emits Go structs to `internal/types/wire_*_gen.go` and `internal/performance/wire_*_gen.go`
- Emits TS interfaces to `src/types/wire/wire-*.ts`
- Handles type mapping (JSON Schema -> Go types, JSON Schema -> TS types)
- Server-only comments and override tables move into the schema files as
  `x-server-only` and `x-ts-type-override` extension fields

**`scripts/generate-tool-schemas.js`** (new):
- Reads `idl/tools/*.schema.json`
- Emits Go files that embed and expose the schema as `map[string]any` via `json.Unmarshal`
- Alternatively: Go files that `//go:embed` the JSON and unmarshal at init

### 3.5 Schema Extension Fields

JSON Schema allows `x-` prefixed extension fields. We use:

```json
{
  "x-go-package": "types",
  "x-go-name": "WireEnhancedAction",
  "x-server-only-fields": ["test_ids", "source"],
  "x-ts-type-override": {
    "direction": "'incoming' | 'outgoing'"
  },
  "x-optional-override": ["fetch_start", "response_end"]
}
```

This moves the hardcoded override tables from `generate-wire-types.js` into the
schema itself, co-locating intent with definition.

---

## 4. Migration Plan

### Phase 1: Wire Types (Low Risk)

**Scope:** Convert 4 wire type pairs from Go-source-of-truth to JSON-Schema-source-of-truth.

1. Write JSON Schema files for the 4 existing wire type groups (13 total structs).
2. Build `scripts/generate-from-idl.js` that produces identical Go and TS output
   to what exists today.
3. Validate: `diff` generated output against current files. Must be byte-identical
   (modulo the `_gen.go` suffix and updated header comments).
4. Delete `scripts/build/generate-wire-types.js` and `scripts/contracts/check-wire-drift.js`.
   The new generator subsumes both.
5. Update `Makefile` targets: `generate-wire-types` -> `generate-from-idl`.

**Exit criteria:** `make check-wire-drift` passes using the new generator. All existing
tests pass. No behavioral change.

### Phase 2: Tool Schemas (Medium Risk)

**Scope:** Convert 5 tool schema files from hand-written Go maps to generated code.

1. Extract current schemas: run `AllTools()`, marshal to JSON, write to `idl/tools/`.
   This produces the initial JSON Schema files that exactly match current behavior.
2. Build `scripts/generate-tool-schemas.js` that reads JSON and emits Go.
3. Replace `internal/schema/{observe,interact,configure,analyze,generate}.go` with
   generated `*_gen.go` files. Keep `schema.go` (the `AllTools()` entry point) manual.
4. Validate: schema invariant tests still pass. MCP `tools/list` output is byte-identical.

**Exit criteria:** `make test` passes. MCP clients see identical tool schemas.

### Phase 3: Response Types (Future, Out of Scope)

Define JSON Schema for tool response shapes. Generate Go response builders and TS
consumer types. This is a larger effort and should be a separate issue.

---

## 5. Toolchain Requirements

### 5.1 Build Integration

```makefile
# New targets
generate-from-idl:
    @node scripts/generate-from-idl.js

generate-tool-schemas:
    @node scripts/generate-tool-schemas.js

# Updated compile-ts dependency
compile-ts: generate-from-idl generate-tool-schemas generate-dom-primitives
    @npx tsc

# Drift check (CI)
check-idl-drift:
    @node scripts/generate-from-idl.js --check
    @node scripts/generate-tool-schemas.js --check
```

### 5.2 CI Validation

Add to `check-invariants` target:
- `check-idl-drift` — verifies generated files match IDL sources.
- Schema validation — `ajv` or similar validates that IDL files are valid JSON Schema
  (can be a Node script, < 50 lines, using `ajv` as a dev dependency).

### 5.3 Developer Workflow

1. Edit `idl/**/*.schema.json`.
2. Run `make compile-ts` (which triggers generation).
3. Generated Go and TS files appear in the working tree.
4. Commit both IDL source and generated output (generated files are tracked in git
   so downstream consumers do not need the generator toolchain).

### 5.4 Pre-commit Hook

Add IDL drift check to `pre-commit` hook. If `idl/` files are staged but generated
files are not, warn and abort.

---

## 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| JSON Schema codegen produces subtly different Go structs | Medium | High | Byte-identical diff validation in Phase 1 before cutover |
| JSON Schema cannot express all current type nuances (pointer semantics, `map[string]any` values) | Low | Medium | Extension fields (`x-go-type`, `x-ts-type-override`) handle edge cases |
| Tool schema JSON files become large and hard to navigate | Low | Low | One file per tool, consistent with current one-file-per-tool Go pattern |
| Generator bugs cause CI failures | Medium | Low | Generator has its own test suite; `--check` mode catches regressions |
| Increased build time from generation step | Low | Low | Generation is fast (< 1s for all files); already running `generate-wire-types` |
| Team unfamiliarity with JSON Schema authoring | Low | Medium | Existing schemas are already JSON Schema (just written as Go maps); format is the same |

### 6.1 Non-Risks

- **Breaking MCP clients:** Generation produces identical JSON Schema output. No client-visible change.
- **Breaking the zero-deps rule:** JSON Schema files are plain JSON. Generators are Node scripts (already in toolchain). No new production dependencies in Go or extension.
- **Big-bang migration:** Phased approach means Phase 1 can ship independently and be validated before Phase 2 begins.

---

## 7. Success Metrics

- Wire type drift checks replaced by single `generate-from-idl.js --check` (removes 2 scripts).
- Tool schema files go from ~1170 lines of `map[string]any` to ~5 generated files + ~5 JSON Schema sources.
- Adding a new wire field: edit 1 JSON file, run `make compile-ts`. Previously: edit Go file, run generator, verify TS.
- Adding a new tool parameter: edit 1 JSON file, run `make compile-ts`. Previously: edit Go `map[string]any`, hope you got the types right.
- Zero `map[string]any` in `internal/schema/` (all generated or embedded).

---

## 8. Open Questions

1. **Embed vs generate for tool schemas?** Go 1.16+ `//go:embed` could load JSON at
   init time, avoiding code generation entirely for tool schemas. Trade-off: simpler
   build but runtime JSON parse on startup (negligible for 5 small files).

2. **Should generated files have a `_gen.go` suffix or replace existing filenames?**
   `_gen.go` is idiomatic Go but means import paths and `go vet` directives may need
   updating. Replacing in-place is simpler but obscures which files are generated.

3. **Dev dependency for JSON Schema validation?** Adding `ajv` as a dev dependency for
   CI schema validation adds a Node dependency. Alternative: a Go-based validator run
   via `go generate`. Either is acceptable; Node is more consistent with existing toolchain.
