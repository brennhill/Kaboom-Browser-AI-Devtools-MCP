// runtime.go — Builds validated daemon runtime configuration without process exits.

package startupconfig

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeconfig"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// Runtime is the validated configuration consumed by daemon composition.
type Runtime struct {
	Port             int
	LogFile          string
	MaxEntries       int
	APIKey           string
	BridgeMode       bool
	DaemonMode       bool
	ParallelMode     bool
	UploadAutomation bool
	UploadSecurity   *uploadsec.Security
	Warnings         []string
}

// BuildRuntime validates parsed flags and resolves filesystem-backed startup
// values without terminating the process.
func BuildRuntime(flags runtimeflags.Values, now time.Time, pid int) (Runtime, error) {
	if err := ValidatePort(flags.Port); err != nil {
		return Runtime{}, err
	}
	uploadSecurity, err := BuildUploadSecurity(flags.EnableOSUpload, flags.UploadDir, flags.UploadDenyPatterns)
	if err != nil {
		return Runtime{}, fmt.Errorf("upload security configuration: %w", err)
	}
	stateDir, err := NormalizeStateDir(flags.StateDir)
	if err != nil {
		return Runtime{}, fmt.Errorf("state directory: %w", err)
	}
	stateDir, warnings, err := runtimeconfig.ApplyParallelStateDir(flags.ParallelMode, stateDir, now, pid)
	if err != nil {
		return Runtime{}, fmt.Errorf("parallel setup: %w", err)
	}
	logFile, logWarning := ResolveLogFile(flags.LogFile)
	if logWarning != "" {
		warnings = append(warnings, logWarning)
	}
	return Runtime{
		Port: flags.Port, LogFile: logFile, MaxEntries: flags.MaxEntries,
		APIKey:     flags.APIKey,
		BridgeMode: flags.BridgeMode, DaemonMode: flags.DaemonMode,
		ParallelMode: flags.ParallelMode, UploadAutomation: flags.EnableOSUpload,
		UploadSecurity: uploadSecurity, Warnings: warnings,
	}, nil
}

// ValidatePort rejects values outside the TCP port range before any startup
// mode performs process or network side effects.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d (must be 1-65535)", port)
	}
	return nil
}
