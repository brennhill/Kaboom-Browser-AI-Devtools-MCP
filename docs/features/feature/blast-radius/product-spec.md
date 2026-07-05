---
doc_type: product-spec
feature_id: feature-blast-radius
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
---

# Blast Radius Product Spec

## TL;DR

- Problem: AI agents edit files without awareness of what depends on them. Renaming a function, changing a type signature, or modifying an export can silently break N downstream files that the AI never checks.
- User value: Every edit gets an instant dependency warning — the AI sees which files import the changed module and can verify them before moving on.
- Binary: `kaboom-hooks blast-radius` (PostToolUse hook)

## Requirements

### BLAST_001: Detect importers on file edit

When an Edit or Write fires, scan the project for files that import, require, or reference the edited file. Report:
- File paths of all direct importers
- Which exported symbols they use (when detectable from the import line)
- Total count of dependents

### BLAST_002: Language-aware import detection

Support import patterns for common languages:
- **Go**: `import "path/to/pkg"`, `import . "pkg"`, `"pkg"` in import blocks
- **TypeScript/JavaScript**: `import ... from './file'`, `require('./file')`, `export ... from './file'`
- **Python**: `from module import ...`, `import module`
- **Rust**: `use crate::module`, `mod module`

Match by file path, not just module name. For Go, match by package path.

### BLAST_003: Cached import graph

Build the import graph once per project and cache it in the session directory. Invalidate the cache when:
- A new file is created (Write to a path that didn't exist)
- An import statement is added or removed (Edit containing import keywords)
- The cache is older than 5 minutes

Cold build must complete in < 200ms for projects up to 5,000 files.

### BLAST_004: Session-aware highlighting

If session-tracking is installed (i.e., `touches.jsonl` exists in the session directory), highlight dependents that the AI has already read or edited this session:
- "Already in session" — the AI visited this file, so it's aware of it
- "Not yet visited" — the AI hasn't seen this file and may need to check it

This prevents the AI from being overwhelmed by long dependency lists — it can focus on the files it hasn't checked yet.

### BLAST_005: Graduated injection

Scale the output based on the number of dependents:
- **0 dependents**: No output (nothing to inject)
- **1-5 dependents**: List all with file paths
- **6-15 dependents**: List first 5, summarize rest as "and N more"
- **16+ dependents**: Summary only: "WARNING: N files depend on this module. Consider the blast radius before changing exported APIs."

### BLAST_006: Skip non-API changes

If the edit does not touch exported symbols (e.g., internal variable rename, comment change, whitespace), skip injection entirely. Detect this by checking whether the `new_string` contains:
- Function/method signatures (exported)
- Type/struct/interface definitions (exported)
- Exported constant/variable declarations
- Import/export statements

### BLAST_007: Performance budget

- Cached graph lookup: < 10ms
- Cold graph build (5,000 files): < 200ms
- Total hook execution: < 50ms (warm), < 250ms (cold)

## Non-Goals

- Transitive dependency analysis (only direct importers, not importers-of-importers)
- Semantic analysis of whether the change actually breaks dependents
- Automatic fixing of downstream breakage
- Full AST parsing (grep-based is sufficient for import detection)
