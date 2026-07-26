// Purpose: Injects file paths into Windows native file dialogs via PowerShell SendKeys automation.
// Why: Provides the Windows-specific implementation of OS-level upload automation.
package osauto

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func executeWindowsAutomation(req upload.OSAutomationInjectRequest, start time.Time) upload.StageResponse {
	safePath := uploadsec.SanitizeForSendKeys(req.FilePath)
	psPath := strings.ReplaceAll(safePath, `"`, "`\"")
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Start-Sleep -Milliseconds 500
# Type the file path into the file name field
[System.Windows.Forms.SendKeys]::SendWait("%s")
Start-Sleep -Milliseconds 300
# Press Enter
[System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
`, psPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script) // #nosec G204 -- path sanitized
	output, err := cmd.CombinedOutput()
	if err != nil {
		return upload.StageResponse{
			Success:    false,
			Stage:      4,
			Error:      fmt.Sprintf("PowerShell automation failed: %v. Output: %s", err, string(output)),
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				"Ensure a file dialog is open in Chrome",
				"Run with administrator privileges if needed",
			},
		}
	}

	return upload.StageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via PowerShell/SendKeys",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
