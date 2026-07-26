// Purpose: Handles Stage 4 OS automation: browser PID detection, AppleScript/xdotool/SendKeys file dialog injection.
// Docs: docs/features/feature/file-upload/index.md

package upload

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"fmt"
	"os"
	"runtime"
	"time"
)

func HandleOSAutomation(req OSAutomationInjectRequest, sec *uploadsec.Security) StageResponse {
	if req.FilePath == "" {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.BrowserPID <= 0 {
		detectedPID, err := DetectBrowserPID()
		if err != nil {
			return StageResponse{
				Success: false,
				Stage:   4,
				Error:   err.Error(),
			}
		}
		req.BrowserPID = detectedPID
	}

	result, err := sec.ValidateFilePath(req.FilePath, true)
	if err != nil {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   err.Error(),
		}
	}

	if err := uploadsec.ValidatePathForOSAutomation(result.ResolvedPath); err != nil {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Invalid file path for OS automation: " + err.Error(),
		}
	}

	if _, err := os.Stat(result.ResolvedPath); err != nil {
		if os.IsNotExist(err) {
			return StageResponse{
				Success: false,
				Stage:   4,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Failed to access file: " + req.FilePath,
		}
	}

	resolvedReq := req
	resolvedReq.FilePath = result.ResolvedPath
	return ExecuteOSAutomation(resolvedReq)
}

func DetectBrowserPID() (int, error) {
	switch runtime.GOOS {
	case "darwin":
		return detectBrowserPIDDarwin()
	case "linux":
		return detectBrowserPIDLinux()
	case "windows":
		return detectBrowserPIDWindows()
	default:
		return 0, fmt.Errorf("browser PID auto-detection not supported on %s", runtime.GOOS)
	}
}

func ExecuteOSAutomation(req OSAutomationInjectRequest) StageResponse {
	start := time.Now()
	switch runtime.GOOS {
	case "darwin":
		return executeMacOSAutomation(req, start)
	case "windows":
		return executeWindowsAutomation(req, start)
	case "linux":
		return executeLinuxAutomation(req, start)
	default:
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   fmt.Sprintf("OS automation not supported on %s", runtime.GOOS),
			Suggestions: []string{
				"Use Stage 3 (form interception) instead",
				"Manually upload the file",
			},
		}
	}
}
