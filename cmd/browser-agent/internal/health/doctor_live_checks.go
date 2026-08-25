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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/healthreader"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/commandcontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type doctorCommandRuntime struct {
	lookPath      func(string) (string, error)
	commandOutput func(time.Duration, string, ...string) ([]byte, error)
}

func defaultDoctorCommandRuntime() doctorCommandRuntime {
	return doctorCommandRuntime{
		lookPath: exec.LookPath,
		commandOutput: func(timeout time.Duration, name string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
	}
}

// DoctorCheck represents a single diagnostic check result.
type DoctorCheck struct {
	Name                     string             `json:"name"`
	CorrelationID            string             `json:"correlation_id,omitempty"`
	Fingerprint              string             `json:"fingerprint,omitempty"`
	Status                   string             `json:"status"` // "pass", "warn", "fail"
	Detail                   string             `json:"detail"`
	Fix                      string             `json:"fix,omitempty"`
	Lifecycle                string             `json:"lifecycle,omitempty"`
	FirstSeenAt              string             `json:"first_seen_at,omitempty"`
	LastSeenAt               string             `json:"last_seen_at,omitempty"`
	RecoveredAt              string             `json:"recovered_at,omitempty"`
	Occurrences              int                `json:"occurrences,omitempty"`
	LastSuccessfulTransition string             `json:"last_successful_transition,omitempty"`
	ExpectedNextTransition   string             `json:"expected_next_transition,omitempty"`
	Deadline                 string             `json:"deadline,omitempty"`
	RecoveryAttempt          int                `json:"recovery_attempt,omitempty"`
	RecoveryOutcome          string             `json:"recovery_outcome,omitempty"`
	History                  []DoctorTransition `json:"history,omitempty"`
}

type DoctorTransition struct {
	Lifecycle     string `json:"lifecycle"`
	At            string `json:"at"`
	Event         string `json:"event,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
}

// HandleDoctorHTTP serves the /doctor HTTP endpoint with JSON readiness checks.
func HandleDoctorHTTP(w http.ResponseWriter, cap *capture.Capture, ver string, extraChecks ...DoctorCheck) {
	report := buildDoctorReport(append(RunDoctorChecks(cap), extraChecks...))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":                report.status,
		"ready_for_interaction": report.ready,
		"version":               ver,
		"checks":                report.checks,
	})
}

type doctorReport struct {
	status string
	ready  bool
	checks []DoctorCheck
}

func buildDoctorReport(checks []DoctorCheck) doctorReport {
	report := doctorReport{status: "healthy", ready: true, checks: checks}
	for _, check := range checks {
		if check.Status == "fail" {
			report.status = "unhealthy"
			report.ready = false
		} else if check.Status == "warn" && report.status != "unhealthy" {
			report.status = "degraded"
			report.ready = false
		}
	}
	return report
}

// DoctorMCPDeps bundles the live-state inputs the configure Doctor response is
// assembled from, keeping HandleDoctorMCP within the six-parameter budget.
type DoctorMCPDeps struct {
	Metrics        *Metrics
	Capture        *capture.Capture
	Alerts         *alertbuf.AlertBuffer
	DiagnosticHint func() string
	ExtraChecks    []DoctorCheck
}

// HandleDoctorMCP serves configure Doctor from the same readiness model as HTTP.
func HandleDoctorMCP(deps DoctorMCPDeps, req mcp.JSONRPCRequest, version string) mcp.JSONRPCResponse {
	checks := append(RunDoctorChecks(deps.Capture), BuildResourcePressureChecks(deps.Capture, deps.Alerts)...)
	checks = append(checks, deps.ExtraChecks...)
	if deps.Metrics != nil {
		checks = append(checks, DoctorCheck{Name: "server_uptime", Status: "pass", Detail: fmt.Sprintf("Server running for %s (version %s)", deps.Metrics.GetUptime().Round(time.Second), version)})
	}
	report := buildDoctorReport(checks)
	hint := ""
	if deps.DiagnosticHint != nil {
		hint = deps.DiagnosticHint()
	}
	return mcp.Succeed(req, "Doctor: "+report.status, map[string]any{
		"status": report.status, "ready_for_interaction": report.ready,
		"checks": report.checks, "hint": hint,
	})
}

// RunDoctorChecks runs all live diagnostic checks against the capture instance.
func RunDoctorChecks(cap *capture.Capture) []DoctorCheck {
	return runDoctorChecks(cap, defaultDoctorCommandRuntime())
}

// runDoctorChecks assembles every live check in fixed order: extension
// connectivity, command contract, pilot, tracked tab, circuit breaker, command
// queue, command execution, AI auth, state recovery, and diagnostics lifecycle.
func runDoctorChecks(cap *capture.Capture, runtime doctorCommandRuntime) []DoctorCheck {
	snap := healthreader.New(cap).Snapshot()
	checks := make([]DoctorCheck, 0, 12)
	checks = append(checks, extensionConnectivityCheck(cap, snap))
	checks = append(checks, commandContractCheck(cap)...)
	checks = append(checks, pilotEnabledCheck(cap))
	checks = append(checks, trackedTabCheck(cap))
	checks = append(checks, circuitBreakerCheck(snap))
	checks = append(checks, commandQueueCheck(cap))
	checks = append(checks, commandExecutionDoctorCheck(cap))
	checks = append(checks, runAIAuthDoctorCheck(runtime, "claude"), runAIAuthDoctorCheck(runtime, "codex"))
	checks = append(checks, extensionStateRecoveryChecks(cap)...)
	if diagnosticCheck, ok := extensionDiagnosticLifecycleCheck(cap); ok {
		checks = append(checks, diagnosticCheck)
	}
	return checks
}

// extensionConnectivityCheck reports whether the extension sync channel is live.
func extensionConnectivityCheck(cap *capture.Capture, snap healthreader.Snapshot) DoctorCheck {
	if cap.Extension().IsExtensionConnected() {
		lastSeen := "unknown"
		if !snap.LastPollTime.IsZero() {
			lastSeen = fmt.Sprintf("%.1fs ago", time.Since(snap.LastPollTime).Seconds())
		}
		return DoctorCheck{
			Name: "extension_connected", Status: "pass",
			Detail: "Extension connected (last seen: " + lastSeen + ")",
		}
	}
	return DoctorCheck{
		Name: "extension_connected", Status: "fail",
		Detail: "Extension is not connected",
		Fix:    "Open the Kaboom extension popup and verify it shows 'Connected'. If not, click the extension icon or reload the page.",
	}
}

// commandContractCheck compares the extension's loaded command contract with the
// daemon's. A release version alone cannot distinguish two local builds made
// between version bumps; the generated command registry identity prevents those
// builds from silently losing or misrouting extension commands. It emits no
// check while the extension is disconnected.
func commandContractCheck(cap *capture.Capture) []DoctorCheck {
	if !cap.Extension().IsExtensionConnected() {
		return nil
	}
	if cap.Extension().CommandContractID() == commandcontract.ID {
		return []DoctorCheck{{
			Name: "command_contract", Status: "pass", Detail: "Daemon and extension command contracts match",
		}}
	}
	return []DoctorCheck{{
		Name: "command_contract", Status: "fail",
		Detail: "The loaded extension command contract does not match this daemon build",
		Fix:    "Reload the Kaboom extension so it loads the files packaged with this daemon, then rerun Doctor.",
	}}
}

// pilotEnabledCheck reports AI Web Pilot availability from the extension's view.
func pilotEnabledCheck(cap *capture.Capture) DoctorCheck {
	pilotState := ""
	if status, ok := cap.Extension().GetPilotStatus().(map[string]any); ok {
		pilotState, _ = status["state"].(string)
	}
	switch pilotState {
	case "explicitly_disabled":
		return DoctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot is explicitly disabled — interact actions will fail",
			Fix:    "Enable AI Web Pilot in the extension popup",
		}
	case "assumed_enabled":
		return DoctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot status not yet confirmed; assuming enabled until first sync",
			Fix:    "Open the extension once to confirm pilot settings, then rerun doctor",
		}
	}
	if cap.Extension().IsPilotActionAllowed() {
		return DoctorCheck{Name: "pilot_enabled", Status: "pass", Detail: "AI Web Pilot is enabled"}
	}
	return DoctorCheck{
		Name: "pilot_enabled", Status: "warn",
		Detail: "AI Web Pilot is disabled — interact actions will fail",
		Fix:    "Enable AI Web Pilot in the extension popup",
	}
}

// trackedTabCheck reports whether a browser tab is actively tracked.
func trackedTabCheck(cap *capture.Capture) DoctorCheck {
	tracking, tabID, tabURL := cap.Extension().GetTrackingStatus()
	if tracking && tabID != 0 {
		return DoctorCheck{
			Name: "tracked_tab", Status: "pass",
			Detail: fmt.Sprintf("Tracking tab %d: %s", tabID, tabURL),
		}
	}
	return DoctorCheck{
		Name: "tracked_tab", Status: "warn",
		Detail: "No tab is being tracked — observe and interact may return empty results",
		Fix:    "Navigate to a page in Chrome. The extension auto-tracks the active tab.",
	}
}

// circuitBreakerCheck reports whether the error circuit breaker is open.
func circuitBreakerCheck(snap healthreader.Snapshot) DoctorCheck {
	if !snap.CircuitOpen {
		return DoctorCheck{Name: "circuit_breaker", Status: "pass", Detail: "Circuit breaker closed (healthy)"}
	}
	return DoctorCheck{
		Name: "circuit_breaker", Status: "fail",
		Detail: "Circuit breaker OPEN: " + snap.CircuitReason,
		Fix:    "Extension is sending too many errors. Check observe(errors) for root cause, then use configure(action:'clear',what:'circuit') to reset.",
	}
}

// commandQueueCheck reports pending command-queue depth.
func commandQueueCheck(cap *capture.Capture) DoctorCheck {
	queueDepth := cap.Queries().QueueDepth()
	if queueDepth < 5 {
		detail := "Command queue empty"
		if queueDepth > 0 {
			detail = fmt.Sprintf("Command queue: %d pending", queueDepth)
		}
		return DoctorCheck{Name: "command_queue", Status: "pass", Detail: detail}
	}
	return DoctorCheck{
		Name: "command_queue", Status: "warn",
		Detail: fmt.Sprintf("Command queue has %d pending commands — extension may be falling behind", queueDepth),
		Fix:    "Wait for commands to complete, or check extension connectivity.",
	}
}

// commandExecutionDoctorCheck surfaces command execution reliability as a check.
func commandExecutionDoctorCheck(cap *capture.Capture) DoctorCheck {
	cmdExec := BuildCommandExecutionInfo(cap)
	check := DoctorCheck{
		Name:   "command_execution",
		Status: cmdExec.Status,
		Detail: cmdExec.Detail,
	}
	if cmdExec.Status != "pass" {
		check.Fix = "Inspect observe(what:\"failed_commands\") for recent expiry/timeout/error events and verify extension polling (/sync). If degradation persists, reload the extension or run configure(action:\"restart\")."
	}
	return check
}

// diagnosticLogData mirrors the extension diagnostic log payload fields the
// lifecycle check consumes.
type diagnosticLogData struct {
	Event             string   `json:"event"`
	DroppedCount      int      `json:"dropped_count"`
	LifecycleSequence []string `json:"lifecycle_sequence"`
}

// extensionDiagnosticState accumulates lifecycle events and dropped-entry
// counts while scanning extension log entries.
type extensionDiagnosticState struct {
	events       []string
	droppedCount int
}

func newExtensionDiagnosticState() *extensionDiagnosticState {
	return &extensionDiagnosticState{events: make([]string, 0, 5)}
}

func extensionDiagnosticLifecycleCheck(cap *capture.Capture) (DoctorCheck, bool) {
	if cap == nil {
		return DoctorCheck{}, false
	}
	state := newExtensionDiagnosticState()
	for _, entry := range cap.ExtensionLogs().Entries() {
		state.observeEntry(entry)
	}
	return state.doctorCheck()
}

func (s *extensionDiagnosticState) observeEntry(entry types.ExtensionLog) {
	var data diagnosticLogData
	if json.Unmarshal(entry.Data, &data) != nil {
		return
	}
	event := diagnosticEventName(data.Event, entry.Category, entry.Message)
	if event != "" && (entry.Category == "diagnostic_lifecycle" || entry.Category == "connection") {
		s.recordLifecycleEvent(event, data.LifecycleSequence)
	}
	if entry.Category == "diagnostic_queue" && data.DroppedCount > s.droppedCount {
		s.droppedCount = data.DroppedCount
	}
}

// diagnosticEventName maps legacy connection messages onto lifecycle event
// names when the payload did not carry an explicit event.
func diagnosticEventName(event, category, message string) string {
	if event != "" || category != "connection" {
		return event
	}
	switch message {
	case "Sync connected":
		return "sync_connected"
	case "Sync disconnected":
		return "sync_disconnected"
	case "[Sync] Sync failed, retrying":
		return "sync_failed"
	}
	return event
}

func (s *extensionDiagnosticState) recordLifecycleEvent(event string, sequence []string) {
	if len(sequence) > 0 {
		start := max(0, len(sequence)-5)
		s.events = append([]string(nil), sequence[start:]...)
	} else {
		s.events = append(s.events, event)
	}
	if len(s.events) > 5 {
		s.events = append([]string(nil), s.events[len(s.events)-5:]...)
	}
}

func (s *extensionDiagnosticState) doctorCheck() (DoctorCheck, bool) {
	if len(s.events) == 0 && s.droppedCount == 0 {
		return DoctorCheck{}, false
	}
	detail := "Extension lifecycle: " + strings.Join(s.events, " -> ")
	status := "pass"
	fix := ""
	if s.droppedCount > 0 {
		status = "warn"
		detail += fmt.Sprintf("; %d dropped diagnostic entries", s.droppedCount)
		fix = "Export System Doctor diagnostics and report repeated queue saturation; reduce noisy extension logging if it persists."
	}
	return DoctorCheck{Name: "extension_diagnostics", Status: status, Detail: detail, Fix: fix}, true
}

// stateRecoveryLogData mirrors the extension's state_recovery log payload.
type stateRecoveryLogData struct {
	Name                   string `json:"name"`
	Detail                 string `json:"detail"`
	Fix                    string `json:"fix"`
	Lifecycle              string `json:"lifecycle"`
	CorrelationID          string `json:"correlation_id"`
	ExpectedNextTransition string `json:"expected_next_transition"`
	Deadline               string `json:"deadline"`
	RecoveryAttempt        int    `json:"recovery_attempt"`
	RecoveryOutcome        string `json:"recovery_outcome"`
}

func extensionStateRecoveryChecks(cap *capture.Capture) []DoctorCheck {
	if cap == nil {
		return nil
	}
	byName := make(map[string]DoctorCheck)
	for _, entry := range cap.ExtensionLogs().Entries() {
		recovery, ok := parseStateRecoveryEntry(entry)
		if !ok {
			continue
		}
		applyStateRecoveryEntry(byName, recovery, recoveryTimestamp(entry.Timestamp))
	}
	return sortedRecoveryChecks(byName)
}

// recoveryTimestamp renders a log timestamp as RFC3339Nano, or "" when zero.
func recoveryTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseStateRecoveryEntry decodes one state_recovery log entry, defaulting the
// lifecycle to active and rejecting entries outside the active/recovered pair.
func parseStateRecoveryEntry(entry types.ExtensionLog) (stateRecoveryLogData, bool) {
	if entry.Category != "state_recovery" {
		return stateRecoveryLogData{}, false
	}
	var recovery stateRecoveryLogData
	if json.Unmarshal(entry.Data, &recovery) != nil || recovery.Name == "" {
		return stateRecoveryLogData{}, false
	}
	if recovery.Lifecycle == "" {
		recovery.Lifecycle = string(statediag.LifecycleActive)
	}
	if recovery.Lifecycle != string(statediag.LifecycleActive) &&
		recovery.Lifecycle != string(statediag.LifecycleRecovered) {
		return stateRecoveryLogData{}, false
	}
	return recovery, true
}

// applyStateRecoveryEntry folds one recovery transition into the per-name check
// state, preserving the timeline (history), occurrences, and recovery metadata.
func applyStateRecoveryEntry(byName map[string]DoctorCheck, recovery stateRecoveryLogData, at string) {
	check := byName[recovery.Name]
	if recovery.Lifecycle == string(statediag.LifecycleRecovered) && check.Name == "" {
		return
	}
	check.Name = recovery.Name
	if recovery.CorrelationID != "" {
		check.CorrelationID = recovery.CorrelationID
	}
	if recovery.Detail != "" {
		check.Detail = recovery.Detail
		check.Fix = recovery.Fix
	}
	check.Lifecycle = recovery.Lifecycle
	check.LastSeenAt = at
	event, outcome := recoveryTransition(check.Occurrences, recovery)
	check.History = append(check.History, DoctorTransition{
		Lifecycle: recovery.Lifecycle, At: at, Event: event,
		CorrelationID: check.CorrelationID, Outcome: outcome,
	})
	if len(check.History) > 20 {
		check.History = append([]DoctorTransition(nil), check.History[len(check.History)-20:]...)
	}
	if recovery.Lifecycle == string(statediag.LifecycleActive) {
		applyActiveStateRecovery(&check, recovery, at)
	} else {
		applyRecoveredStateRecovery(&check, at)
	}
	byName[recovery.Name] = check
}

// recoveryTransition labels one history entry: recovered transitions complete a
// recovery, while repeat failures on an existing check are recurrences.
func recoveryTransition(occurrences int, recovery stateRecoveryLogData) (event, outcome string) {
	if recovery.Lifecycle == string(statediag.LifecycleRecovered) {
		return "recovery_completed", "recovered"
	}
	event = "failure_detected"
	outcome = recovery.RecoveryOutcome
	if occurrences > 0 {
		event = "failure_recurred"
	}
	return event, outcome
}

// applyActiveStateRecovery marks a check as actively degraded, bumping the
// occurrence counter and defaulting any missing recovery metadata.
func applyActiveStateRecovery(check *DoctorCheck, recovery stateRecoveryLogData, at string) {
	check.Status = "warn"
	check.RecoveredAt = ""
	check.Occurrences++
	check.ExpectedNextTransition = recovery.ExpectedNextTransition
	if check.ExpectedNextTransition == "" {
		check.ExpectedNextTransition = "state_verified"
	}
	check.Deadline = recovery.Deadline
	check.RecoveryAttempt = recovery.RecoveryAttempt
	if check.RecoveryAttempt == 0 {
		check.RecoveryAttempt = check.Occurrences
	}
	check.RecoveryOutcome = recovery.RecoveryOutcome
	if check.RecoveryOutcome == "" {
		check.RecoveryOutcome = "pending"
	}
	if check.FirstSeenAt == "" {
		check.FirstSeenAt = at
	}
}

// applyRecoveredStateRecovery closes out a check whose state was verified again.
func applyRecoveredStateRecovery(check *DoctorCheck, at string) {
	check.Status = "pass"
	check.RecoveredAt = at
	check.LastSuccessfulTransition = "state_verified"
	check.ExpectedNextTransition = ""
	check.Deadline = ""
	check.RecoveryOutcome = "recovered"
}

// sortedRecoveryChecks emits the accumulated checks in stable name order.
func sortedRecoveryChecks(byName map[string]DoctorCheck) []DoctorCheck {
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

func runAIAuthDoctorCheck(runtime doctorCommandRuntime, tool string) DoctorCheck {
	name := tool + "_auth"
	displayName := strings.ToUpper(tool[:1]) + tool[1:]
	if _, err := runtime.lookPath(tool); err != nil {
		return DoctorCheck{
			Name: name, Status: "pass",
			Detail: displayName + " CLI is not installed (optional)",
		}
	}

	args := []string{"auth", "status"}
	if tool == "codex" {
		args = []string{"login", "status"}
	}
	output, err := runtime.commandOutput(2*time.Second, tool, args...)
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
