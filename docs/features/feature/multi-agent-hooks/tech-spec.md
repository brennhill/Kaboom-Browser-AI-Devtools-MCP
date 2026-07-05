---
doc_type: tech-spec
feature_id: feature-multi-agent-hooks
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/protocol.go
  - cmd/hooks/main.go
  - cmd/browser-agent/tools_configure_quality_gates.go
test_paths:
  - internal/hook/protocol_test.go
  - cmd/hooks/main_test.go
  - cmd/browser-agent/tools_configure_quality_gates_test.go
---

# Multi-Agent Hook Protocol Tech Spec

## TL;DR

- Design: Auto-detect agent via env vars, adapt output JSON format in `WriteOutput()`
- Key constraints: ~20 lines of code change in protocol.go, zero impact on hook logic
- Rollout risk: Very low — additive change, existing Claude Code output is the default

## Requirement Mapping

- MULTI_AGENT_001 -> `internal/hook/protocol.go:DetectAgent()`
- MULTI_AGENT_002 -> `internal/hook/protocol.go:WriteOutput()` switch on agent
- MULTI_AGENT_003 -> No change needed — input format is already compatible
- MULTI_AGENT_004 -> `internal/hook/session_store.go:SessionID()` checks env vars first
- MULTI_AGENT_005 -> `cmd/browser-agent/tools_configure_quality_gates.go` writes to both config files
- MULTI_AGENT_006 -> Future work, tracked separately
- MULTI_AGENT_007 -> No change needed — already one binary

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                  kaboom-hooks binary                │
│                                                      │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐ │
│  │ quality-gate │  │ blast-radius │  │ session-track│ │
│  └──────┬──────┘  └──────┬───────┘  └──────┬──────┘ │
│         │                │                  │        │
│         └────────────────┼──────────────────┘        │
│                          │                           │
│              ┌───────────▼───────────┐               │
│              │  internal/hook/       │               │
│              │  (agent-agnostic      │               │
│              │   logic layer)        │               │
│              └───────────┬───────────┘               │
│                          │                           │
│              ┌───────────▼───────────┐               │
│              │  protocol.go          │               │
│              │  DetectAgent()        │               │
│              │  WriteOutput()        │               │
│              │  (agent-specific I/O) │               │
│              └───────────────────────┘               │
└──────────────────────────────────────────────────────┘
         │                          │
    Claude Code                Gemini CLI
    PostToolUse                AfterTool
  .claude/settings.json     .gemini/settings.json
```

## Agent Detection

```go
// Agent identifies which AI coding agent is calling the hook.
type Agent string

const (
    AgentClaude Agent = "claude"
    AgentGemini Agent = "gemini"
    AgentCodex  Agent = "codex"
)

// DetectAgent identifies the calling agent from environment variables.
func DetectAgent() Agent {
    if os.Getenv("GEMINI_SESSION_ID") != "" {
        return AgentGemini
    }
    if os.Getenv("CODEX_SESSION_ID") != "" {
        return AgentCodex
    }
    return AgentClaude
}
```

## Output Adaptation

```go
func WriteOutput(w io.Writer, context string) error {
    if context == "" {
        return nil
    }
    agent := DetectAgent()
    var out any
    switch agent {
    case AgentGemini:
        // SPEC:gemini-cli-hooks — nested under hookSpecificOutput
        out = map[string]any{
            "hookSpecificOutput": map[string]string{
                "additionalContext": context,
            },
        }
    default:
        // SPEC:claude-code-hooks — flat additionalContext
        out = Output{AdditionalContext: context}
    }
    return json.NewEncoder(w).Encode(out)
}
```

## Session ID Enhancement

```go
func SessionID() string {
    // Prefer agent-provided session ID when available.
    if id := os.Getenv("GEMINI_SESSION_ID"); id != "" {
        return id[:16] // truncate to consistent length
    }
    if id := os.Getenv("CODEX_SESSION_ID"); id != "" {
        return id[:16]
    }
    // Claude Code fallback: derive from (ppid, cwd).
    ppid := os.Getppid()
    cwd, _ := os.Getwd()
    h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", ppid, cwd)))
    return hex.EncodeToString(h[:8])
}
```

## Gemini CLI Configuration

When `configure(what="setup_quality_gates")` detects `.gemini/` in the project, it writes:

```json
{
  "hooks": {
    "AfterTool": [
      {
        "matcher": "write_file|replace_in_file|edit_file",
        "hooks": [
          {"name": "quality-gate", "type": "command", "command": "kaboom-hooks quality-gate", "timeout": 10000},
          {"name": "blast-radius", "type": "command", "command": "kaboom-hooks blast-radius", "timeout": 10000},
          {"name": "decision-guard", "type": "command", "command": "kaboom-hooks decision-guard", "timeout": 10000},
          {"name": "session-track", "type": "command", "command": "kaboom-hooks session-track", "timeout": 10000}
        ]
      },
      {
        "matcher": "run_shell_command",
        "hooks": [
          {"name": "compress-output", "type": "command", "command": "kaboom-hooks compress-output", "timeout": 10000},
          {"name": "session-track", "type": "command", "command": "kaboom-hooks session-track", "timeout": 10000}
        ]
      }
    ]
  }
}
```

Note differences from Claude Code config:
- Event name: `AfterTool` not `PostToolUse`
- Matcher: regex (`write_file|replace_in_file`) not string match (`Edit|Write`)
- Tool names: Gemini uses `write_file`, `replace_in_file`, `run_shell_command` not `Edit`, `Write`, `Bash`
- Timeout: milliseconds (10000) not seconds (10)

## Tool Name Mapping

| Claude Code | Gemini CLI | Kaboom hook |
|-------------|-----------|---------------|
| `Read` | `read_file` | session-track |
| `Edit` | `replace_in_file` | quality-gate, blast-radius, decision-guard, session-track |
| `Write` | `write_file` | quality-gate, blast-radius, decision-guard, session-track |
| `Bash` | `run_shell_command` | compress-output, session-track |

The hook logic checks `tool_name` but needs to accept both naming conventions:

```go
func isEditTool(name string) bool {
    switch name {
    case "Edit", "Write", "write_file", "replace_in_file", "edit_file":
        return true
    }
    return false
}
```

## Testing

- Unit tests for `DetectAgent()` with mock env vars
- Unit tests for `WriteOutput()` verifying both JSON formats
- Integration test: run hooks binary with `GEMINI_SESSION_ID` set, verify Gemini output format
- Integration test: run without env vars, verify Claude Code output format

## Performance Impact

Zero — `os.Getenv()` is < 1μs, JSON structure difference is negligible.
