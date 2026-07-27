// handler.go — Handler for configure(what="setup_quality_gates").
// Scaffolds .kaboom.json and kaboom-code-standards.md for code quality gate enforcement.
// Docs: docs/features/feature/quality-gates/index.md

package qualitygates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hook"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type Deps interface {
	GetActiveCodebase() string
}

const defaultDuplicateThreshold = 3

const kaboomHookQualityGate = "kaboom-hooks quality-gate"
const kaboomHookCompressOutput = "kaboom-hooks compress-output"
const kaboomHookSessionTrack = "kaboom-hooks session-track"
const kaboomHookBlastRadius = "kaboom-hooks blast-radius"
const kaboomHookDecisionGuard = "kaboom-hooks decision-guard"

// toolConfigureSetupQualityGates handles configure(what="setup_quality_gates").
// Creates .kaboom.json and kaboom-code-standards.md in the target directory.
func Handle(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TargetDir string `json:"target_dir"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Resolve the project root directory.
	projectDir := d.GetActiveCodebase()
	if projectDir == "" {
		return mcp.Fail(req, mcp.ErrNotInitialized,
			"No active codebase set. Cannot determine project directory.",
			"Set active_codebase via configure(what='store', key='active_codebase', data='<path>') first")
	}

	targetDir := projectDir
	if params.TargetDir != "" {
		targetDir = params.TargetDir
	}

	// Security: target_dir must be within the project directory.
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid target_dir: "+err.Error(), "Provide a valid directory path", mcp.WithParam("target_dir"))
	}
	absProject, _ := filepath.Abs(projectDir)
	if !strings.HasPrefix(absTarget+string(filepath.Separator), absProject+string(filepath.Separator)) && absTarget != absProject {
		return mcp.Fail(req, mcp.ErrPathNotAllowed,
			"target_dir is outside the project directory",
			"Use a path within "+absProject, mcp.WithParam("target_dir"))
	}

	// Ensure target directory exists.
	if stat, err := os.Stat(absTarget); err != nil || !stat.IsDir() {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"target_dir does not exist or is not a directory: "+absTarget,
			"Provide an existing directory path", mcp.WithParam("target_dir"))
	}

	configPath := filepath.Join(absTarget, hook.KaboomConfigFile)
	configExisted := false
	standardsCreated := false
	standardsPath := ""

	// Write .kaboom.json if it doesn't exist.
	if _, err := os.Stat(configPath); err == nil {
		configExisted = true
	} else {
		cfg := hook.KaboomConfig{
			CodeStandards:      hook.DefaultCodeStandardsFile,
			FileSizeLimit:      hook.DefaultFileSizeLimit,
			DuplicateThreshold: defaultDuplicateThreshold,
		}
		cfgJSON, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return mcp.Fail(req, mcp.ErrInternal, "Failed to marshal config: "+err.Error(), "Internal error — do not retry")
		}
		cfgJSON = append(cfgJSON, '\n')
		if err := os.WriteFile(configPath, cfgJSON, 0644); err != nil {
			return mcp.Fail(req, mcp.ErrInternal, "Failed to write "+hook.KaboomConfigFile+": "+err.Error(), "Check file system permissions")
		}
	}

	// Determine code_standards path from config.
	codeStandardsRef := hook.DefaultCodeStandardsFile
	if configExisted {
		existingCfg, err := os.ReadFile(configPath)
		if err == nil {
			var parsed hook.KaboomConfig
			if json.Unmarshal(existingCfg, &parsed) == nil && parsed.CodeStandards != "" {
				codeStandardsRef = parsed.CodeStandards
			}
		}
	}

	// Only create the standards file if it's the default name (not a custom path).
	if codeStandardsRef == hook.DefaultCodeStandardsFile {
		standardsPath = filepath.Join(absTarget, hook.DefaultCodeStandardsFile)
		if _, err := os.Stat(standardsPath); err != nil {
			if err := os.WriteFile(standardsPath, []byte(defaultCodeStandardsContent), 0644); err != nil {
				return mcp.Fail(req, mcp.ErrInternal, "Failed to write "+hook.DefaultCodeStandardsFile+": "+err.Error(), "Check file system permissions")
			}
			standardsCreated = true
		}
	}

	// Install Claude Code hooks into .claude/settings.json.
	hooksInstalled, settingsPath, hookErr := installClaudeCodeHooks(absTarget)

	// Build response.
	defaults := map[string]any{
		"code_standards":      codeStandardsRef,
		"file_size_limit":     hook.DefaultFileSizeLimit,
		"duplicate_threshold": defaultDuplicateThreshold,
	}

	suggestions := buildQualityGateSuggestions(configExisted, standardsCreated, codeStandardsRef, hooksInstalled)

	responseData := map[string]any{
		"config_path":     configPath,
		"config_existed":  configExisted,
		"defaults":        defaults,
		"suggestions":     suggestions,
		"hooks_installed": hooksInstalled,
		"settings_path":   settingsPath,
	}
	if hookErr != nil {
		responseData["hooks_error"] = hookErr.Error()
	}
	if standardsPath != "" {
		responseData["standards_path"] = standardsPath
		responseData["standards_created"] = standardsCreated
	}

	summary := buildSetupSummary(configExisted, hooksInstalled, hookErr)

	return mcp.Succeed(req, summary, responseData)
}

// buildQualityGateSuggestions returns contextual suggestions based on setup state.
func buildQualityGateSuggestions(configExisted, standardsCreated bool, codeStandardsRef string, hooksInstalled bool) []string {
	suggestions := []string{}

	if !configExisted {
		suggestions = append(suggestions,
			"Edit .kaboom.json to customize quality gate thresholds",
			"Set code_standards to your existing conventions doc if one exists",
		)
	}
	if standardsCreated {
		suggestions = append(suggestions,
			"Edit kaboom-code-standards.md to add your project's coding patterns and conventions",
		)
	}
	if codeStandardsRef != hook.DefaultCodeStandardsFile {
		suggestions = append(suggestions,
			"Your config points to "+codeStandardsRef+" — ensure this file exists",
		)
	}
	if hooksInstalled {
		suggestions = append(suggestions,
			"Hooks installed — quality gate runs automatically on every Edit/Write, output compression on every Bash",
			"Optionally add a Haiku prompt hook for belt-and-suspenders AI review",
		)
	}

	return suggestions
}

// buildSetupSummary returns a human-readable summary of what was done.
func buildSetupSummary(configExisted, hooksInstalled bool, hookErr error) string {
	parts := []string{}
	if configExisted {
		parts = append(parts, "existing config preserved")
	} else {
		parts = append(parts, "config + standards created")
	}
	if hooksInstalled {
		parts = append(parts, "hooks installed to .claude/settings.json")
	} else if hookErr != nil {
		parts = append(parts, "hooks failed: "+hookErr.Error())
	} else {
		parts = append(parts, "hooks already installed")
	}
	return "Quality gates: " + strings.Join(parts, ", ")
}

// installClaudeCodeHooks writes Kaboom hook entries into .claude/settings.json.
// Merges with existing settings — does not overwrite. Returns (installed, settingsPath, error).
// If hooks are already present, returns (false, path, nil).
func installClaudeCodeHooks(projectDir string) (bool, string, error) {
	settingsDir := filepath.Join(projectDir, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Read existing settings.
	var settings map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, settingsPath, fmt.Errorf("invalid JSON in %s: %v", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return false, settingsPath, err
	} else {
		settings = make(map[string]any)
	}

	// Check if already installed.
	if containsManagedHooks(settings) {
		return false, settingsPath, nil
	}

	// Ensure hooks.PostToolUse exists.
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}
	postToolUse, _ := hooks["PostToolUse"].([]any)

	// Add hooks for Edit/Write: quality gate, blast radius, decision guard, session track.
	postToolUse = append(postToolUse, map[string]any{
		"matcher": "Edit|Write",
		"hooks": []any{
			map[string]any{"type": "command", "command": kaboomHookQualityGate, "timeout": 10},
			map[string]any{"type": "command", "command": kaboomHookBlastRadius, "timeout": 10},
			map[string]any{"type": "command", "command": kaboomHookDecisionGuard, "timeout": 10},
			map[string]any{"type": "command", "command": kaboomHookSessionTrack, "timeout": 10},
		},
	})

	// Add session tracking for Read.
	postToolUse = append(postToolUse, map[string]any{
		"matcher": "Read",
		"hooks": []any{
			map[string]any{"type": "command", "command": kaboomHookSessionTrack, "timeout": 10},
		},
	})

	// Add output compression + session tracking for Bash.
	postToolUse = append(postToolUse, map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{"type": "command", "command": kaboomHookCompressOutput, "timeout": 10},
			map[string]any{"type": "command", "command": kaboomHookSessionTrack, "timeout": 10},
		},
	})

	hooks["PostToolUse"] = postToolUse

	// Write back.
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return false, settingsPath, fmt.Errorf("cannot create %s: %v", settingsDir, err)
	}
	out, _ := json.MarshalIndent(settings, "", "  ")
	out = append(out, '\n')
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return false, settingsPath, fmt.Errorf("cannot write %s: %v", settingsPath, err)
	}

	return true, settingsPath, nil
}

// containsManagedHooks checks if .claude/settings.json already has managed hook commands.
func containsManagedHooks(settings map[string]any) bool {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return false
	}
	postToolUse, _ := hooks["PostToolUse"].([]any)
	for _, entry := range postToolUse {
		entryMap, _ := entry.(map[string]any)
		hooksList, _ := entryMap["hooks"].([]any)
		for _, h := range hooksList {
			hMap, _ := h.(map[string]any)
			if cmd, _ := hMap["command"].(string); strings.Contains(cmd, "kaboom-hooks") || strings.Contains(cmd, "kaboom-hooks") || strings.Contains(cmd, "kaboom hook ") {
				return true
			}
		}
	}
	return false
}

// defaultCodeStandardsContent is the starter content for kaboom-code-standards.md.
// Rules are written to be actionable by an LLM reviewer (Haiku): each rule has a
// specific trigger condition and a concrete action. Vague rules generate false positives.
const defaultCodeStandardsContent = `# Code Standards

> Quality gate rules for automated code review. Each rule has a trigger (when to flag)
> and an action (what to do instead). Only flag clear violations — not style preferences.

## File Structure

- **Max 800 lines per file.** If a file exceeds this, it must be split.
- **One concept per file.** If a file has two unrelated concerns, split them.
- **No orphan code.** Dead code, commented-out blocks, and unused imports must be removed.

## Naming Conventions

Functions: verb-phrase describing the action — ` + "`" + `buildResponse` + "`" + `, ` + "`" + `parseArgs` + "`" + `, ` + "`" + `validateToken` + "`" + `.
Types/structs/classes: noun-phrase — ` + "`" + `ToolHandler` + "`" + `, ` + "`" + `QueryResult` + "`" + `, ` + "`" + `SessionStore` + "`" + `.
Booleans: predicate-phrase — ` + "`" + `isReady` + "`" + `, ` + "`" + `hasExpired` + "`" + `, ` + "`" + `canRetry` + "`" + `.
Constants: describe the value's purpose, not its content — ` + "`" + `maxRetries` + "`" + ` not ` + "`" + `three` + "`" + `.
Avoid abbreviations except well-known ones (URL, ID, HTTP, JSON, API).

## Error Handling

- Always handle errors explicitly. Never silently ignore error return values.
- Use structured error messages: "{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}"
- Errors should be actionable — tell the caller what went wrong and how to fix it.

## Duplication & Reuse

- **3+ similar lines = extract a helper.** If you see the same logic repeated, it should be a function.
- **Before writing a new utility, check if one exists.** Search the codebase for similar function signatures.
- **Prefer composition over inheritance.** Small, focused functions composed together beat deep class hierarchies.

## Structural Patterns

- **3+ switch/case branches dispatching to similar logic** → extract to a handler map or strategy pattern.
- **3+ sequential phases (setup, execute, cleanup)** → use a builder or command pattern if one exists in the codebase.
- **Nested callbacks or deeply indented logic (4+ levels)** → extract inner blocks into named functions.
- **God functions (50+ lines doing multiple things)** → split into focused sub-functions.

## Testing

- New functions should have tests. Bug fixes must include a regression test.
- Use deterministic tests — no sleep-based timing, no flaky network calls.
- Test the contract (inputs/outputs), not the implementation details.

## Security

- Never log secrets, tokens, API keys, or credentials.
- Validate all external input at system boundaries (user input, API responses, file reads).
- Do not trust internal data structures to be valid — defensive checks at module boundaries.

## Add Your Project Patterns Below

<!-- Add project-specific patterns here. Be specific — vague rules cause false positives.

Good: "Request validation must use validateAndRespond() from internal/util/guards.go.
       Do not write inline if/else validation chains."

Bad:  "Use good patterns." (too vague, will flag everything)

-->
`
