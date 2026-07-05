---
doc_type: flow_map
feature_id: feature-quality-gates
last_reviewed: 2026-07-05
---

# Quality Gates Flow Map

## Setup Flow

```
AI or user calls configure(what="setup_quality_gates")
  -> toolConfigureSetupQualityGates()
  -> resolve project dir from server.GetActiveCodebase()
  -> validate target_dir is within project (security)
  -> if .kaboom.json missing: write default config
  -> read code_standards path from config (existing or default)
  -> if default standards file missing: write starter content
  -> return config_path, standards_path, defaults, suggestions
```

## Quality Gate Enforcement Flow (Hook-Driven)

```
AI calls Edit/Write tool
  -> Claude Code PostToolUse hook fires
  -> kaboom-hooks quality-gate reads JSON from stdin:
     1. Finds .kaboom.json (walks up from edited file)
     2. Reads code_standards doc
     3. Checks file size vs limit
     4. Detects patterns in new code, searches codebase for existing usage
     5. Suggests helper extraction at 2+ instances
     6. Outputs findings as additionalContext (JSON to stdout)
  -> Primary model sees standards + conventions + findings
  -> Fixes violations before proceeding
```

## Binary Architecture

```
kaboom-hooks (standalone, cmd/hooks/)
  ├── quality-gate     -> internal/hook/quality_gate.go
  └── compress-output  -> internal/hook/compress_output.go

kaboom-agentic-browser (MCP server, cmd/browser-agent/)
  └── configure(what="setup_quality_gates")
      -> writes .kaboom.json, kaboom-code-standards.md
      -> installs hooks into .claude/settings.json (references kaboom-hooks, recognizes prior managed hook entries)
```

## Code Paths

| Component | Path |
|-----------|------|
| Hooks binary | `cmd/hooks/main.go` |
| Hooks binary tests | `cmd/hooks/main_test.go` |
| Setup handler | `cmd/browser-agent/tools_configure_quality_gates.go` |
| Setup tests | `cmd/browser-agent/tools_configure_quality_gates_test.go` |
| Quality gate logic | `internal/hook/quality_gate.go` |
| Convention detection | `internal/hook/convention_detect.go` |
| Output compression | `internal/hook/compress_output.go` |
| Hook protocol | `internal/hook/protocol.go` |
| Token tracking | `internal/tracking/token_tracker.go` |
| Installer contract tests | `scripts/install-upgrade-regression.contract.test.mjs` |
| Hooks install test | `scripts/test-install-hooks-only.sh` |
