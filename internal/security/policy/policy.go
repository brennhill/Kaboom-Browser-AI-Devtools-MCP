// policy.go — Persistent security config schema and the manual-only mutation guards.
// Purpose: Manages persistent security configuration including whitelisted origins and flagging severity.
// Why: Separates policy persistence and loading from runtime security mode management.
// Docs: docs/features/feature/security-hardening/index.md

// Package policy owns the security trust boundary: which execution mode the
// process is in, whether a caller may mutate persistent security config (it may
// not — every mutation entry point is deliberately blocked and audited), and the
// in-memory audit trail of those decisions.
//
// Sibling packages depend on it one way: csp records whitelist overrides through
// LogEvent and quotes EditInstruction in its remediation text. policy imports no
// sibling.
package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type Config struct {
	Version             string            `json:"version"`
	WhitelistedOrigins  []string          `json:"whitelisted_origins"`
	MinFlaggingSeverity string            `json:"min_flagging_severity"`
	Notes               map[string]string `json:"notes,omitempty"`
}

type AuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Action       string    `json:"action"`
	Origin       string    `json:"origin,omitempty"`
	Reason       string    `json:"reason"`
	Persistent   bool      `json:"persistent"`
	Source       string    `json:"source"`
	MCPSessionID string    `json:"mcp_session_id,omitempty"`
}

var configPathOverride = ""

func configPath() string {
	if configPathOverride != "" {
		return configPathOverride
	}

	path, err := state.SecurityConfigFile()
	if err != nil {
		if legacyPath, legacyErr := state.LegacySecurityConfigFile(); legacyErr == nil {
			return legacyPath
		}
		return ""
	}

	return path
}

func setConfigPath(path string) {
	configPathOverride = path
}

func EditInstruction() string {
	path := configPath()
	if path == "" {
		return "Edit the security configuration file manually"
	}
	return "Edit " + path + " manually"
}

// AddToWhitelist is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func AddToWhitelist(origin string) error {
	return blockMutation("add_to_whitelist", origin, "")
}

// SetMinSeverity is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func SetMinSeverity(severity string) error {
	return blockMutation("set_min_severity", "", severity)
}

// ClearWhitelist is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func ClearWhitelist() error {
	return blockMutation("clear_whitelist", "", "")
}

func blockMutation(action string, origin string, detail string) error {
	reason := mutationReason()
	if detail != "" {
		reason = fmt.Sprintf("%s (%s=%q)", reason, action, detail)
	}
	err := errors.New(reason + " - " + EditInstruction())

	LogEvent(AuditEvent{
		Timestamp:  time.Now(),
		Action:     "security_config_mutation_blocked",
		Origin:     origin,
		Reason:     err.Error(),
		Persistent: false,
		Source:     "security_config_policy",
	})

	return err
}

func mutationReason() string {
	if IsMCPMode() {
		return "security config updates are manual-only in MCP mode and require human review"
	}
	if !IsInteractiveTerminal() {
		return "security config updates are manual-only in non-interactive environments"
	}
	return "security config updates are manual-only; interactive mutation commands are intentionally disabled"
}
