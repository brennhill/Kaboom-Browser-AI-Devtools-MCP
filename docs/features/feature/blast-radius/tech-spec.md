---
doc_type: tech-spec
feature_id: feature-blast-radius
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/blast_radius.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/blast_radius_test.go
  - cmd/hooks/main_test.go
---

# Blast Radius Tech Spec

## TL;DR

- Design: Grep-based import graph cached in session directory, invalidated on structural edits
- Key constraints: < 50ms warm, < 250ms cold, no AST parsing, zero dependencies
- Rollout risk: Low — additive hook, no changes to existing code

## Requirement Mapping

- BLAST_001 -> `internal/hook/blast_radius.go:RunBlastRadius()` — main entry point
- BLAST_002 -> `internal/hook/blast_radius.go:buildImportGraph()` — per-language import scanning
- BLAST_003 -> `internal/hook/blast_radius.go:buildImportGraph()` + `loadCachedGraph()`
- BLAST_004 -> `internal/hook/blast_radius.go:annotateWithSession()` — reads session touches
- BLAST_005 -> `internal/hook/blast_radius.go:formatDependents()` — graduated output
- BLAST_006 -> `internal/hook/blast_radius.go:touchesExportedSymbol()` — export detection
- BLAST_007 -> benchmarked in tests

## Import Graph

### Structure

```go
// ImportGraph maps each file to the files that import it (reverse edges).
type ImportGraph struct {
    // Dependents maps a file path to the list of files that import it.
    Dependents map[string][]string `json:"dependents"`
    // BuiltAt is the cache timestamp.
    BuiltAt    time.Time           `json:"built_at"`
    // FileCount is the number of source files scanned.
    FileCount  int                 `json:"file_count"`
}
```

### Build algorithm

1. Walk the project tree (skip `.git`, `node_modules`, `vendor`, `dist`, `build`, hidden dirs)
2. For each source file (`.go`, `.ts`, `.tsx`, `.js`, `.jsx`, `.py`, `.rs`):
   a. Read file content (skip files > 100KB)
   b. Extract import paths using language-specific regex
   c. Resolve import path to file path (relative to project root)
   d. Record reverse edge: `graph.Dependents[imported_file] = append(..., importing_file)`
3. Write graph to `~/.kaboom/sessions/<id>/graph.json`

### Import regex patterns

```go
var importPatterns = map[string][]*regexp.Regexp{
    ".go": {
        regexp.MustCompile(`"([^"]+)"`),  // inside import blocks
    },
    ".ts,.tsx,.js,.jsx": {
        regexp.MustCompile(`(?:import|export)\s+.*?from\s+['"]([^'"]+)['"]`),
        regexp.MustCompile(`require\(['"]([^'"]+)['"]\)`),
    },
    ".py": {
        regexp.MustCompile(`(?:from|import)\s+([\w.]+)`),
    },
    ".rs": {
        regexp.MustCompile(`(?:use|mod)\s+([\w:]+)`),
    },
}
```

### Path resolution

Import paths are resolved relative to the importing file's directory. For Go, package paths are matched against directory structure. For TS/JS, `./` and `../` relative imports are resolved; bare specifiers (npm packages) are ignored.

### Cache invalidation

The cached graph is invalidated when:
- `graph.json` is older than 5 minutes
- The edit's `new_string` contains import/require/from/use keywords (structural change)
- The tool is Write (new file creation always invalidates)

## Hook Logic

```
kaboom-hooks blast-radius:
  1. Parse hook input (tool_name, tool_input)
  2. Skip if tool_name is not Edit or Write
  3. Extract file_path and new_string
  4. Skip if new_string doesn't touch exported symbols (BLAST_006)
  5. Load or build import graph (BLAST_003)
  6. Look up dependents of edited file
  7. If session-tracking installed, annotate dependents (BLAST_004)
  8. Format output with graduated detail (BLAST_005)
  9. Write additionalContext to stdout
```

## Cross-Hook Integration

### Reading from session-tracking (optional)

```go
sessionDir := session.Dir() // shared session directory
if touches, err := session.ReadTouches(sessionDir); err == nil {
    // Annotate dependents with session context
    for i, dep := range dependents {
        if session.WasFileRead(sessionDir, dep) {
            dependents[i].InSession = true
        }
    }
}
```

If `session-tracking` is not installed, `ReadTouches` returns an empty list and blast-radius works standalone.

### Writing for other hooks

The import graph is written to the shared session directory. Other hooks (e.g., future refactoring hooks) can read `graph.json` without rebuilding.

## Export Detection (BLAST_006)

```go
var exportedSymbolPatterns = map[string][]*regexp.Regexp{
    ".go": {
        regexp.MustCompile(`^func\s+[A-Z]`),           // exported function
        regexp.MustCompile(`^type\s+[A-Z]\w+\s+`),     // exported type
        regexp.MustCompile(`^\s*[A-Z]\w+\s*=`),         // exported var/const
    },
    ".ts,.tsx,.js,.jsx": {
        regexp.MustCompile(`^export\s+`),               // export keyword
        regexp.MustCompile(`module\.exports`),           // CommonJS export
    },
    ".py": {
        regexp.MustCompile(`^def\s+[a-z]`),             // function definition
        regexp.MustCompile(`^class\s+[A-Z]`),           // class definition
    },
}
```

The hook checks if `new_string` contains any of these patterns. If not, the edit is internal-only and no blast radius warning is needed.

## Output Examples

**Small blast radius (3 dependents):**
```
[Blast Radius] 3 files import this module:
  internal/server/routes.go (already in session)
  cmd/browser-agent/main.go (not yet visited)
  internal/capture/httpingest/handlers.go (not yet visited)
Verify these files are compatible with your changes.
```

**Medium blast radius (8 dependents):**
```
[Blast Radius] 8 files import this module:
  internal/server/routes.go (already in session)
  cmd/browser-agent/main.go (not yet visited)
  internal/capture/httpingest/handlers.go (already in session)
  internal/capture/sync.go (not yet visited)
  internal/capture/websocket.go (not yet visited)
  ...and 3 more
Verify these files are compatible with your changes.
```

**Large blast radius (25+ dependents):**
```
[Blast Radius] WARNING: 25 files depend on this module. Consider the blast radius before changing exported APIs.
```

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Warm graph lookup | < 10ms | JSON unmarshal of cached graph.json |
| Cold graph build | < 200ms | Concurrent file walk + regex scan |
| Export detection | < 2ms | Regex match on new_string |
| Session annotation | < 5ms | Scan touches.jsonl |
| Total (warm) | < 50ms | |
| Total (cold) | < 250ms | |
