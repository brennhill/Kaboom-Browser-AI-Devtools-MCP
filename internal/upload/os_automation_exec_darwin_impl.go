// Purpose: Injects file paths into macOS native file dialogs via AppleScript automation.
// Why: Provides the macOS-specific implementation of OS-level upload automation.
package upload

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func executeMacOSAutomation(req OSAutomationInjectRequest, start time.Time) StageResponse {
	safePath := uploadsec.SanitizeForAppleScript(req.FilePath)

	script := fmt.Sprintf(`tell application "System Events"
	delay 0.5
	keystroke "g" using {command down, shift down}
	delay 0.5
	keystroke "%s"
	delay 0.3
	key code 36
	delay 0.5
	key code 36
end tell`, safePath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// #nosec G204 -- script is built from sanitized file path
	cmd := exec.CommandContext(ctx, "osascript", "-e", script) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- input sanitized
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("AppleScript failed: %v", err)
		if len(output) > 0 {
			errMsg += " Output: " + string(output)
		}
		termApp := os.Getenv("TERM_PROGRAM")
		if termApp == "" {
			termApp = "your terminal"
		}
		errMsg += fmt.Sprintf(". Fix: System Settings > Privacy & uploadsec.Security > Accessibility > enable %s", termApp)
		return StageResponse{
			Success:    false,
			Stage:      4,
			Error:      errMsg,
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				fmt.Sprintf("Grant Accessibility permissions: System Settings > Privacy & uploadsec.Security > Accessibility > enable %s", termApp),
				"Ensure a file dialog is open in Chrome",
			},
		}
	}

	return StageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via AppleScript",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
