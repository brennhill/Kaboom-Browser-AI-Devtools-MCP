// commands.go — Dispatches CLI commands to tool-specific argument parsers.
// Why: Keeps top-level CLI parser intent explicit while detailed parsing lives in focused modules.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package parser

import (
	"fmt"
	"strings"
)

// ParseCLIArgs dispatches to the correct tool parser based on tool name.
func ParseCLIArgs(tool, action string, args []string) (map[string]any, error) {
	action = NormalizeAction(action)
	switch tool {
	case "observe":
		return ParseObserveArgs(action, args)
	case "analyze":
		return ParseAnalyzeArgs(action, args)
	case "generate":
		return ParseGenerateArgs(action, args)
	case "configure":
		return ParseConfigureArgs(action, args)
	case "interact":
		return ParseInteractArgs(action, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

// NormalizeAction converts CLI-style kebab-case to MCP snake_case.
func NormalizeAction(action string) string {
	return strings.ReplaceAll(action, "-", "_")
}
