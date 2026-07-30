// doctor_live_checks.go — Implements live doctor checks for HTTP and MCP surfaces.
// Why: Centralizes runtime readiness evaluation separate from setup preflight checks.

package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

var doctorLookPath = exec.LookPath

var doctorCommandOutput = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// DoctorCheck represents a single diagnostic check result.
type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "warn", "fail"
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// HandleDoctorHTTP serves the /doctor HTTP endpoint with JSON readiness checks.
func HandleDoctorHTTP(w http.ResponseWriter, cap *capture.Capture, ver string, extraChecks ...DoctorCheck) {
	checks := RunDoctorChecks(cap)
	checks = append(checks, extraChecks...)

	overallStatus := "healthy"
	readyForInteraction := true
	for _, c := range checks {
		if c.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		}
		if c.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":                overallStatus,
		"ready_for_interaction": readyForInteraction,
		"version":               ver,
		"checks":                checks,
	})
}

// RunDoctorChecks runs all live diagnostic checks against the capture instance.
func RunDoctorChecks(cap *capture.Capture) []DoctorCheck {
	checks := make([]DoctorCheck, 0, 11)
	snap := capture.NewHealthReader(cap).Snapshot()

	// 1. Extension connectivity.
	if cap.Extension().IsExtensionConnected() {
		lastSeen := "unknown"
		if !snap.LastPollTime.IsZero() {
			lastSeen = fmt.Sprintf("%.1fs ago", time.Since(snap.LastPollTime).Seconds())
		}
		checks = append(checks, DoctorCheck{
			Name: "extension_connected", Status: "pass",
			Detail: "Extension connected (last seen: " + lastSeen + ")",
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name: "extension_connected", Status: "fail",
			Detail: "Extension is not connected",
			Fix:    "Open the Kaboom extension popup and verify it shows 'Connected'. If not, click the extension icon or reload the page.",
		})
	}

	// 2. Pilot enabled/assumed/disabled.
	pilotState := ""
	if status, ok := cap.Extension().GetPilotStatus().(map[string]any); ok {
		pilotState, _ = status["state"].(string)
	}
	switch pilotState {
	case "explicitly_disabled":
		checks = append(checks, DoctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot is explicitly disabled — interact actions will fail",
			Fix:    "Enable AI Web Pilot in the extension popup",
		})
	case "assumed_enabled":
		checks = append(checks, DoctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot status not yet confirmed; assuming enabled until first sync",
			Fix:    "Open the extension once to confirm pilot settings, then rerun doctor",
		})
	default:
		if cap.Extension().IsPilotActionAllowed() {
			checks = append(checks, DoctorCheck{
				Name: "pilot_enabled", Status: "pass",
				Detail: "AI Web Pilot is enabled",
			})
		} else {
			checks = append(checks, DoctorCheck{
				Name: "pilot_enabled", Status: "warn",
				Detail: "AI Web Pilot is disabled — interact actions will fail",
				Fix:    "Enable AI Web Pilot in the extension popup",
			})
		}
	}

	// 3. Tracked tab.
	tracking, tabID, tabURL := cap.Extension().GetTrackingStatus()
	if tracking && tabID != 0 {
		checks = append(checks, DoctorCheck{
			Name: "tracked_tab", Status: "pass",
			Detail: fmt.Sprintf("Tracking tab %d: %s", tabID, tabURL),
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name: "tracked_tab", Status: "warn",
			Detail: "No tab is being tracked — observe and interact may return empty results",
			Fix:    "Navigate to a page in Chrome. The extension auto-tracks the active tab.",
		})
	}

	// 4. Circuit breaker.
	if !snap.CircuitOpen {
		checks = append(checks, DoctorCheck{
			Name: "circuit_breaker", Status: "pass",
			Detail: "Circuit breaker closed (healthy)",
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name: "circuit_breaker", Status: "fail",
			Detail: "Circuit breaker OPEN: " + snap.CircuitReason,
			Fix:    "Extension is sending too many errors. Check observe(errors) for root cause, then use configure(action:'clear',what:'circuit') to reset.",
		})
	}

	// 5. Command queue.
	queueDepth := cap.Queries().QueueDepth()
	if queueDepth < 5 {
		detail := "Command queue empty"
		if queueDepth > 0 {
			detail = fmt.Sprintf("Command queue: %d pending", queueDepth)
		}
		checks = append(checks, DoctorCheck{
			Name: "command_queue", Status: "pass", Detail: detail,
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name: "command_queue", Status: "warn",
			Detail: fmt.Sprintf("Command queue has %d pending commands — extension may be falling behind", queueDepth),
			Fix:    "Wait for commands to complete, or check extension connectivity.",
		})
	}

	// 6. Command execution reliability.
	cmdExec := BuildCommandExecutionInfo(cap)
	cmdExecCheck := DoctorCheck{
		Name:   "command_execution",
		Status: cmdExec.Status,
		Detail: cmdExec.Detail,
	}
	if cmdExec.Status != "pass" {
		cmdExecCheck.Fix = "Inspect observe(what:\"failed_commands\") for recent expiry/timeout/error events and verify extension polling (/sync). If degradation persists, reload the extension or run configure(action:\"restart\")."
	}
	checks = append(checks, cmdExecCheck)
	checks = append(checks, runAIAuthDoctorCheck("claude"), runAIAuthDoctorCheck("codex"))
	checks = append(checks, extensionStateRecoveryChecks(cap)...)

	return checks
}

func extensionStateRecoveryChecks(cap *capture.Capture) []DoctorCheck {
	if cap == nil {
		return nil
	}
	type recoveryData struct {
		Name   string `json:"name"`
		Detail string `json:"detail"`
		Fix    string `json:"fix"`
	}
	byName := make(map[string]DoctorCheck)
	for _, entry := range cap.ExtensionLogs().Entries() {
		if entry.Category != "state_recovery" {
			continue
		}
		var recovery recoveryData
		if json.Unmarshal(entry.Data, &recovery) != nil || recovery.Name == "" || recovery.Detail == "" {
			continue
		}
		byName[recovery.Name] = DoctorCheck{
			Name: recovery.Name, Status: "warn", Detail: recovery.Detail, Fix: recovery.Fix,
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	checks := make([]DoctorCheck, 0, len(names))
	for _, name := range names {
		checks = append(checks, byName[name])
	}
	return checks
}

func runAIAuthDoctorCheck(tool string) DoctorCheck {
	name := tool + "_auth"
	displayName := strings.ToUpper(tool[:1]) + tool[1:]
	if _, err := doctorLookPath(tool); err != nil {
		return DoctorCheck{
			Name: name, Status: "pass",
			Detail: displayName + " CLI is not installed (optional)",
		}
	}

	args := []string{"auth", "status"}
	if tool == "codex" {
		args = []string{"login", "status"}
	}
	output, err := doctorCommandOutput(2*time.Second, tool, args...)
	normalized := strings.ToLower(string(output))
	compact := strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(normalized)
	if strings.Contains(normalized, "keychain") {
		return DoctorCheck{
			Name: name, Status: "fail",
			Detail: displayName + " authentication failed because the local keychain is unavailable",
			Fix:    "Repair or reset the login keychain, then sign in to " + displayName + " again.",
		}
	}
	if strings.Contains(normalized, "api key") ||
		strings.Contains(normalized, "access token") ||
		strings.Contains(compact, `"authmethod":"apikey"`) {
		return DoctorCheck{
			Name: name, Status: "warn",
			Detail: displayName + " is authenticated with API billing rather than a subscription",
			Fix:    "Sign out and sign in with your subscription account, or explicitly confirm API billing in the terminal.",
		}
	}
	if tool == "claude" &&
		strings.Contains(compact, `"loggedin":true`) &&
		strings.Contains(compact, `"authmethod":"claude.ai"`) {
		return DoctorCheck{
			Name: name, Status: "pass",
			Detail: "Claude subscription authentication is active",
		}
	}
	if tool == "codex" && strings.Contains(normalized, "logged in using chatgpt") {
		return DoctorCheck{
			Name: name, Status: "pass",
			Detail: "Codex ChatGPT subscription authentication is active",
		}
	}
	if err != nil || strings.Contains(normalized, "not logged in") {
		return DoctorCheck{
			Name: name, Status: "warn",
			Detail: displayName + " is not authenticated",
			Fix:    "Open the Kaboom terminal and sign in to " + displayName + " with your subscription account.",
		}
	}
	return DoctorCheck{
		Name: name, Status: "warn",
		Detail: displayName + " authentication provider could not be determined",
		Fix:    "Run " + tool + " authentication status manually before starting a billed session.",
	}
}
