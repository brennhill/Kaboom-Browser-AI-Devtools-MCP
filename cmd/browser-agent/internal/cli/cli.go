// cli.go — Implements standalone CLI mode execution flow, daemon bootstrap, and tool call dispatch.
// Why: Enables scriptable local usage without requiring direct MCP client integration.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli/parser"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// RuntimeConfig holds values injected from the main package at startup.
type RuntimeConfig struct {
	DefaultPort      int
	MaxPostBodySize  int64
	DiagnosticOutput io.Writer

	// DaemonOps — callbacks into the main package for daemon lifecycle.
	IsServerRunning    func(port int) bool
	WaitForServer      func(port int, timeout time.Duration) bool
	DaemonProcessArgv0 func(exePath string) string
}

// CLIToolNames lists valid tool names for CLI mode detection.
var CLIToolNames = map[string]bool{
	"observe":   true,
	"analyze":   true,
	"generate":  true,
	"configure": true,
	"interact":  true,
}

// CLIConfig holds resolved CLI configuration.
type CLIConfig struct {
	Port    int
	Format  string
	Timeout int // milliseconds
}

// IsCLIMode returns true if the first argument is a known tool name.
func IsCLIMode(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return CLIToolNames[args[0]]
}

// Run is the main CLI flow. Returns exit code.
func Run(args []string, rc RuntimeConfig) int {
	cfg, remaining := ResolveCLIConfig(args, rc)
	output := rc.DiagnosticOutput
	if output == nil {
		output = os.Stderr
	}

	if len(remaining) < 2 {
		fmt.Fprintf(output, "Usage: kaboom <tool> <action> [flags]\n")
		fmt.Fprintf(output, "  Tools: observe, analyze, generate, configure, interact\n")
		fmt.Fprintf(output, "  Example: kaboom observe errors --limit 50\n")
		return 2
	}

	tool := remaining[0]
	action := remaining[1]
	toolArgs := remaining[2:]

	// Parse tool-specific arguments
	mcpArgs, err := parser.ParseCLIArgs(tool, action, toolArgs)
	if err != nil {
		fmt.Fprintf(output, "Error: %v\n", err)
		return 2
	}

	// Ensure daemon is running and get base URL
	baseURL, err := EnsureDaemon(cfg.Port, rc)
	if err != nil {
		fmt.Fprintf(output, "Error: %v\n", err)
		return 1
	}

	// Long-running modes get extended timeout
	timeout := cfg.Timeout
	if tool == "analyze" && parser.NormalizeAction(action) == "accessibility" {
		timeout = 35000
	}
	if tool == "observe" && parser.NormalizeAction(action) == "command_result" && timeout < 60000 {
		timeout = 60000
	}

	// Call the tool
	result, err := CallTool(baseURL, tool, mcpArgs, timeout, rc.MaxPostBodySize)
	if err != nil {
		fmt.Fprintf(output, "Error: %v\n", err)
		return 1
	}

	return FormatResult(output, cfg.Format, tool, parser.NormalizeAction(action), result)
}

// ResolveCLIConfig resolves config from defaults < env < flags, stripping global flags.
func ResolveCLIConfig(args []string, rc RuntimeConfig) (CLIConfig, []string) {
	cfg := CLIConfig{
		Port:    rc.DefaultPort,
		Format:  "human",
		Timeout: 15000,
	}

	ApplyCLIEnvOverrides(&cfg)

	remaining := ApplyCLIFlagOverrides(args, &cfg)

	return cfg, remaining
}

// ApplyCLIEnvOverrides applies KABOOM_PORT and KABOOM_FORMAT environment variables.
func ApplyCLIEnvOverrides(cfg *CLIConfig) {
	if envPort := os.Getenv("KABOOM_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}
	if envFormat := os.Getenv("KABOOM_FORMAT"); envFormat != "" {
		cfg.Format = envFormat
	}
}

// ApplyCLIFlagOverrides strips --port, --format, --timeout flags and applies their values.
func ApplyCLIFlagOverrides(args []string, cfg *CLIConfig) []string {
	remaining := args

	var portStr string
	portStr, remaining = CLIParseFlag(remaining, "--port")
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}

	var format string
	format, remaining = CLIParseFlag(remaining, "--format")
	if format != "" {
		cfg.Format = format
	}

	var timeoutStr string
	timeoutStr, remaining = CLIParseFlag(remaining, "--timeout")
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			cfg.Timeout = t
		}
	}

	return remaining
}

// CLIParseFlag extracts a global CLI flag value from args, returning the value
// and remaining tool-specific arguments.
func CLIParseFlag(args []string, flag string) (string, []string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			val := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return val, remaining
		}
	}
	return "", args
}

// EnsureDaemon checks if the server is running and spawns it if needed.
// Returns the base URL (e.g., "http://127.0.0.1:7890").
func EnsureDaemon(port int, rc RuntimeConfig) (string, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	if rc.IsServerRunning(port) {
		return baseURL, nil
	}

	// Spawn daemon
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(exe, "--daemon", "--port", fmt.Sprintf("%d", port)) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI opens browser with known URL
	cmd.Args[0] = rc.DaemonProcessArgv0(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to become ready
	if !rc.WaitForServer(port, 4*time.Second) {
		return "", fmt.Errorf("daemon started but not responding on port %d after 4s", port)
	}

	return baseURL, nil
}
