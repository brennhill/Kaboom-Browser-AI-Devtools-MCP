// tools_configure.go — Defines the configure MCP mode boundary and its narrow dependencies.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/auditlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qualitygates"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const restartSelfSignalDelay = 100 * time.Millisecond

var replayMu sync.Mutex

func buildConfigureDispatcher(h *ToolHandler) *toolconfigure.Dispatcher {
	return toolconfigure.NewDispatcher(map[string]toolconfigure.Handler{
		"store": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.configureSessions.Store(req, args)
		},
		"load": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.configureSessions.Load(req, args)
		},
		"diff_sessions": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.configureSessions.Diff(req, args)
		},
		"health": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureHealth(h.healthMetrics, h.capture, h.server, req)
		},
		"restart": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureRestart(req)
		},
		"doctor": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureDoctor(
				h.healthMetrics,
				h.capture,
				h.Guards.DiagnosticHintString,
				recoveryDoctorChecks(h.stateRecovery),
				req,
			)
		},
		"noise_rule": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			rewrittenArgs, err := cfg.RewriteNoiseRuleArgs(args)
			if err != nil {
				return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
			}
			return toolconfigure.HandleNoise(h.configureLocalDeps, req, rewrittenArgs)
		},
		"clear": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return toolconfigure.HandleClear(toolconfigure.ClearTargets{
				Capture:  h.capture,
				Resetter: capture.NewStateResetter(h.capture),
				ClearLogs: func() int {
					count := h.server.logs.EntryCount()
					h.server.logs.ClearEntries()
					return count
				},
				Inbox:       h.server.pushInbox,
				Annotations: h.annotationStore,
			}, req, args)
		},
		"audit_log": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureAuditLog(h.auditTrail, h.auditRecorder, req, args)
		},
		"streaming": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureStreaming(h.alertBuffer, req, args)
		},
		"test_boundary_start": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.testBoundaries.Start(req, args)
		},
		"test_boundary_end": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.testBoundaries.End(req, args)
		},
		"event_recording_start": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.recordingHandler.EventRecordingStart(req, args)
		},
		"event_recording_stop": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.recordingHandler.EventRecordingStop(req, args)
		},
		"playback": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.recordingHandler.Playback(req, args)
		},
		"log_diff": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.recordingHandler.LogDiff(req, args)
		},
		"telemetry": configureLocal(h.configureLocalDeps, toolconfigure.HandleTelemetry),
		"describe_capabilities": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return toolconfigure.HandleDescribeCapabilities(h.configureLocalDeps, req, args, version)
		},
		"tutorial": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return tutorial.HandleTutorial(h.tutorialDeps, req, args, playbooks.TutorialFailureRecoveryPlaybooks())
		},
		"save_sequence": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.sequences.Save(req, args)
		},
		"get_sequence": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.sequences.Get(req, args)
		},
		"list_sequences": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.sequences.List(req, args)
		},
		"delete_sequence": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.sequences.Delete(req, args)
		},
		"replay_sequence": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.sequences.Replay(req, args)
		},
		"security_mode": configureLocal(h.configureLocalDeps, toolconfigure.HandleSecurityMode),
		"network_recording": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)
		},
		"action_jitter": configureLocal(h.configureLocalDeps, toolconfigure.HandleActionJitter),
		"report_issue": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return issuereport.Handle(h.issueReportDeps, req, args)
		},
		"setup_quality_gates": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return qualitygates.Handle(h.server, req, args)
		},
	})
}

func configureLocal(
	deps toolconfigure.Deps,
	fn func(toolconfigure.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse,
) toolconfigure.Handler {
	return func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(deps, req, args)
	}
}

func handleConfigureHealth(
	metrics *health.Metrics,
	captureStore *capture.Capture,
	server *Server,
	req mcp.JSONRPCRequest,
) mcp.JSONRPCResponse {
	if metrics == nil {
		return mcp.Fail(req, mcp.ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}
	response := getHealthResponse(metrics, captureStore, server, version)
	return mcp.Succeed(req, "Server health", response)
}

func handleConfigureDoctor(
	metrics *health.Metrics,
	captureStore *capture.Capture,
	diagnosticHint func() string,
	extraChecks []health.DoctorCheck,
	req mcp.JSONRPCRequest,
) mcp.JSONRPCResponse {
	checks := health.RunDoctorChecks(captureStore)
	checks = append(checks, extraChecks...)
	if metrics != nil {
		uptime := metrics.GetUptime()
		checks = append(checks, health.DoctorCheck{
			Name: "server_uptime", Status: "pass",
			Detail: fmt.Sprintf("Server running for %s (version %s)", uptime.Round(time.Second), version),
		})
	}
	overallStatus := "healthy"
	readyForInteraction := true
	for _, check := range checks {
		if check.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		} else if check.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}
	return mcp.Succeed(req, "Doctor: "+overallStatus, map[string]any{
		"status": overallStatus, "ready_for_interaction": readyForInteraction,
		"checks": checks, "hint": diagnosticHint(),
	})
}

func recoveryDoctorChecks(diagnostics interface{ Snapshot() []statediag.Diagnostic }) []health.DoctorCheck {
	if diagnostics == nil {
		return nil
	}
	snapshot := diagnostics.Snapshot()
	checks := make([]health.DoctorCheck, 0, len(snapshot))
	for _, diagnostic := range snapshot {
		checks = append(checks, health.DoctorCheck{
			Name: diagnostic.Name, Status: "warn", Detail: diagnostic.Detail, Fix: diagnostic.Fix,
		})
	}
	return checks
}

type serverDepsAdapter struct{ s *Server }

func (a *serverDepsAdapter) GetTerminalPort() int {
	if a.s == nil {
		return 0
	}
	return a.s.getTerminalPort()
}

func (a *serverDepsAdapter) GetConsoleStats() (int, int, int64) {
	if a.s == nil || a.s.logs == nil {
		return 0, defaultMaxEntries, 0
	}
	return a.s.logs.EntryCount(), a.s.logs.MaxEntries(), a.s.logs.DropCount()
}

const defaultMaxEntries = 1000

func getHealthResponse(hm *health.Metrics, cap *capture.Capture, server *Server, ver string) health.MCPHealthResponse {
	var serverDeps health.ServerDeps
	if server != nil {
		serverDeps = &serverDepsAdapter{s: server}
	}
	var upgrade health.UpgradeProvider
	if binaryUpgradeState != nil {
		upgrade = binaryUpgradeState
	}
	return hm.GetHealth(cap, serverDeps, upgrade, getLaunchModeInfo, ver)
}

func getLaunchModeInfo() health.LaunchModeInfo {
	lm := launchmode.Current()
	return health.LaunchModeInfo{
		Mode: lm.Mode, Reason: lm.Reason, ParentProcess: lm.ParentProcess,
	}
}

func buildConfigureLocalDeps(h *ToolHandler) toolconfigure.Deps {
	return toolconfigure.Deps{
		NoiseConfig: func() *noise.NoiseConfig { return h.noiseConfig },
		ConsoleEntries: func() []types.LogEntry {
			snapshot := h.server.logs.Entries()
			entries := make([]types.LogEntry, len(snapshot))
			for index, entry := range snapshot {
				entries[index] = types.LogEntry(entry)
			}
			return entries
		},
		NetworkBodies:      func() []types.NetworkBody { return h.capture.Telemetry().GetNetworkBodies() },
		AllWebSocketEvents: func() []types.WebSocketEvent { return h.capture.Telemetry().GetAllWebSocketEvents() },
		ToolsList:          schema.AllTools,
		GetToolModuleExamples: func(toolName string) any {
			if examples := h.toolCatalog.Examples(toolName); len(examples) > 0 {
				return examples
			}
			return nil
		},
		HasCapture:      func() bool { return h.capture != nil },
		GetSecurityMode: func() (string, bool, []string) { return h.capture.Extension().GetSecurityMode() },
		SetSecurityMode: func(mode string, rewrites []string) {
			h.capture.Extension().SetSecurityMode(mode, rewrites)
		},
		GetTelemetryMode:        func() string { return h.server.logs.TelemetryMode() },
		SetTelemetryMode:        h.server.logs.SetTelemetryMode,
		InteractActionSetJitter: h.interactRuntime.SetJitter,
		InteractActionGetJitter: h.interactRuntime.GetJitter,
	}
}

func buildTutorialDeps(h *ToolHandler) *tutorial.Deps {
	return &tutorial.Deps{
		GetTrackingStatus: func() (bool, int, string) {
			if h.capture == nil {
				return false, 0, ""
			}
			return h.capture.Extension().GetTrackingStatus()
		},
		GetPilotStatus: func() any {
			if h.capture == nil {
				return nil
			}
			return h.capture.Extension().GetPilotStatus()
		},
		IsExtensionConnected: func() bool {
			return h.capture != nil && h.capture.Extension().IsExtensionConnected()
		},
	}
}

func buildIssueReportDeps(h *ToolHandler) issuereport.HandlerDeps {
	return issuereport.HandlerDeps{
		Collect: func(template, title, userContext string) issuereport.IssueReport {
			report := issuereport.IssueReport{Template: template, Title: title, UserContext: userContext}
			report.Diagnostics.Server.Version = version
			report.Diagnostics.Platform.OS = runtime.GOOS
			report.Diagnostics.Platform.Arch = runtime.GOARCH
			report.Diagnostics.Platform.GoVersion = runtime.Version()
			if h.healthMetrics != nil {
				report.Diagnostics.Server.UptimeSeconds = h.healthMetrics.GetUptime().Seconds()
				audit := h.healthMetrics.BuildAuditInfo()
				report.Diagnostics.Server.TotalCalls = audit.TotalCalls
				report.Diagnostics.Server.TotalErrors = audit.TotalErrors
				report.Diagnostics.Server.ErrorRatePct = audit.ErrorRatePct
			}
			if h.capture != nil {
				health := capture.NewHealthReader(h.capture).Snapshot()
				report.Diagnostics.Extension.Connected = health.ConnectionCount > 0
				report.Diagnostics.Extension.Source = health.ExtSessionID
				report.Diagnostics.Buffers.NetworkEntries = health.NetworkBodyCount
				report.Diagnostics.Buffers.ActionEntries = health.ActionCount
			}
			if h.server != nil {
				report.Diagnostics.Buffers.ConsoleEntries = h.server.logs.EntryCount()
			}
			return report
		},
		Sanitize: func(report issuereport.IssueReport) issuereport.IssueReport {
			if h.redactionEngine == nil {
				return report
			}
			return issuereport.NewSanitizer(h.redactionEngine).SanitizeReport(report)
		},
		Submit: func(report issuereport.IssueReport) issuereport.SubmitResult {
			return issuereport.SubmitViaGH(h.shutdownCtx, report, h.issueCommandRunner)
		},
	}
}

func extractErrorMessage(response mcp.JSONRPCResponse) string {
	if message := replay.ErrorMessage(response); message != "" {
		return message
	}
	return "unknown error"
}

func handleConfigureAuditLog(
	trail *audit.AuditTrail,
	recorder *audit.Recorder,
	req mcp.JSONRPCRequest,
	args json.RawMessage,
) mcp.JSONRPCResponse {
	result, problem := auditlog.New(trail).Execute(args)
	if problem != nil {
		switch problem.Kind {
		case auditlog.Unavailable:
			return mcp.Fail(req, mcp.ErrNotInitialized, problem.Message, "Internal error — do not retry")
		case auditlog.InvalidJSON:
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+problem.Message, "Fix JSON syntax and call again")
		case auditlog.InvalidOperation:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use operation: analyze, report, or clear", mcp.WithParam("operation"))
		default:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use RFC3339 format, for example 2026-02-17T15:04:05Z", mcp.WithParam("since"))
		}
	}

	switch result.Operation {
	case "clear":
		recorder.ResetSessions()
		return mcp.Succeed(req, "Audit log cleared", map[string]any{
			"status": "ok", "operation": result.Operation, "cleared": result.Cleared,
		})
	case "analyze":
		return mcp.Succeed(req, "Audit log analysis", map[string]any{
			"status": "ok", "operation": result.Operation, "summary": result.Summary,
		})
	default:
		return mcp.Succeed(req, "Audit log entries", map[string]any{
			"status": "ok", "operation": result.Operation, "entries": result.Entries, "count": result.Count,
		})
	}
}

func handleConfigureStreaming(
	alertBuffer *alertbuf.AlertBuffer,
	req mcp.JSONRPCRequest,
	args json.RawMessage,
) mcp.JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	var params struct {
		Action          string   `json:"action"`
		Events          []string `json:"events"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		URLFilter       string   `json:"url"`
		SeverityMin     string   `json:"severity_min"`
	}
	if resp, stop := mcp.ParseArgs(req, rewritten, &params); stop {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.Action, "action", "Add the 'action' parameter and call again"); blocked {
		return resp
	}
	result := alertBuffer.Stream.Configure(
		params.Action,
		params.Events,
		params.ThrottleSeconds,
		params.URLFilter,
		params.SeverityMin,
	)
	return mcp.Succeed(req, "Streaming configuration", result)
}

func handleConfigureRestart(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	resp := mcp.Succeed(req, "Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})
	util.SafeGo(func() {
		time.Sleep(restartSelfSignalDelay)
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	})
	return resp
}
