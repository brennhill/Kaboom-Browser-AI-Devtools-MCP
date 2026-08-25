// tools_configure.go — Defines the configure MCP mode boundary and its narrow dependencies.
// Why: Acts as the top-level router for all session/runtime configuration actions under the configure tool.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/doctorsupport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/auditlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	qafixturehandler "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qafixture"
	qafixtureshutdown "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qafixture/shutdown"
	qafixturetransport "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qafixture/transport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qualitygates"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/healthreader"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	fixturecontract "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/qafixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const restartSelfSignalDelay = 100 * time.Millisecond

type fixtureRecoveryRunner interface {
	RecoverPending(context.Context) []string
}

func buildQAFixtureHandler(h *ToolHandler) (*qafixturehandler.Handler, error) {
	if h.capture == nil {
		return nil, errors.New("fixture_capture_unavailable")
	}
	registryPath, err := fixtureRegistryPath(h.server.logs.LogFile())
	if err != nil {
		return nil, err
	}
	store := fixturecontract.NewRegistryStore(registryPath, 32)
	registry, notice := store.Load()
	if notice != "" {
		h.stateRecovery.Report(statediag.Diagnostic{
			Name: "fixture_transaction_registry", Detail: notice,
			Fix: "Reconnect the extension and retry configure({what:'qa_fixture',fixture_action:'status'}).",
		})
	}
	handler, err := qafixturehandler.New(qafixturehandler.Deps{
		Context: h.shutdownCtx,
		Execute: func(ctx context.Context, command string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
			return qafixturetransport.Execute(ctx, h.capture, command, params, timeout)
		},
		NewCorrelationID:    func() string { return toolresp.NewCorrelationID("qa_fixture") },
		NewTransactionID:    func() string { return toolresp.NewCorrelationID("fixture_transaction") },
		ExtensionGeneration: func() string { return h.capture.Extension().Snapshot().ExtSessionID },
		Now:                 time.Now,
		Registry:            registry,
		Persist:             store.Save,
		OnNotice: func(notice string) {
			h.stateRecovery.Report(statediag.Diagnostic{
				Name: "fixture_transaction_persistence", Detail: notice,
				Fix: "Check the Kaboom state directory, then restore the active fixture transaction.",
			})
		},
		Diagnostics: h.stateRecovery,
	})
	if err != nil {
		return nil, err
	}
	handler.StartStartupRecovery(h.shutdownCtx, h.capture.Extension().WaitForExtensionConnected)
	h.fixtureRecovery = handler
	return handler, nil
}

func fixtureRegistryPath(logFile string) (string, error) {
	if logFile == "" {
		return statecfg.InRoot("run", "fixture-transactions.json")
	}
	directory := filepath.Dir(logFile)
	if filepath.Base(directory) == "logs" {
		directory = filepath.Dir(directory)
	}
	return filepath.Join(directory, "run", "fixture-transactions.json"), nil
}

func (h *ToolHandler) closeWithFixtureRecovery() {
	qafixtureshutdown.Run(h.fixtureRecovery, h.stateRecovery, h.shutdownCancel)
}

func buildConfigureDispatcher(h *ToolHandler) *toolconfigure.Dispatcher {
	fixtureHandler, fixtureErr := buildQAFixtureHandler(h)
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
			return handleConfigureHealth(h.healthMetrics, h.capture, h.server, h.alertBuffer, h.stateRecovery, req)
		},
		"restart": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return handleConfigureRestart(req)
		},
		"doctor": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			var incidents *incident.Store
			var incidentViews []incident.DoctorView
			if h.server != nil {
				incidents = h.server.incidents
				incidentViews = h.server.incidents.DoctorSnapshot()
			}
			checks := doctorsupport.Checks(h.stateRecovery, incidents)
			if response, handled := doctorsupport.Handle(req, args, incidentViews, version, runtime.GOOS+"-"+runtime.GOARCH, nil); handled {
				return response
			}
			doctorDeps := health.DoctorMCPDeps{
				Metrics:        h.healthMetrics,
				Capture:        h.capture,
				Alerts:         h.alertBuffer,
				DiagnosticHint: h.Guards.DiagnosticHintString,
				ExtraChecks:    checks,
			}
			return health.HandleDoctorMCP(doctorDeps, req, version)
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
				Resetter: newRuntimeResetter(h.capture),
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
			return auditlog.Handle(h.auditTrail, h.auditRecorder, req, args)
		},
		"streaming": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return toolconfigure.HandleStreaming(h.alertBuffer, req, args)
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
			return netrecord.HandleNetworkRecording(h.capture.Telemetry().NetworkBodies(), h.networkRecording, req, args)
		},
		"action_jitter": configureLocal(h.configureLocalDeps, toolconfigure.HandleActionJitter),
		"report_issue": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return issuereport.Handle(h.issueReportDeps, req, args)
		},
		"setup_quality_gates": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return qualitygates.Handle(h.server.activeCodebase, req, args)
		},
		"qa_fixture": func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fixtureErr != nil {
				return mcp.Fail(req, mcp.ErrInternal, "QA fixture handler is unavailable", "Restart Kaboom and inspect configure({what:'doctor'}).")
			}
			return fixtureHandler.Handle(req, args)
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
	alerts *alertbuf.AlertBuffer,
	recovery recoveryDiagnostics,
	req mcp.JSONRPCRequest,
) mcp.JSONRPCResponse {
	if metrics == nil {
		return mcp.Fail(req, mcp.ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}
	response := getHealthResponse(metrics, captureStore, server, alerts, recovery, version)
	return mcp.Succeed(req, "Server health", response)
}

type serverDepsAdapter struct{ s *Server }

func (a *serverDepsAdapter) GetTerminalPort() int {
	if a.s == nil {
		return 0
	}
	return a.s.terminalStatus.Port()
}

func (a *serverDepsAdapter) GetConsoleStats() (int, int, int64) {
	if a.s == nil || a.s.logs == nil {
		return 0, serverdefaults.MaxLogEntries, 0
	}
	return a.s.logs.EntryCount(), a.s.logs.MaxEntries(), a.s.logs.DropCount()
}

type recoveryDiagnostics interface {
	Snapshot() []statediag.Diagnostic
	Stats() statediag.CollectorStats
}

func getHealthResponse(hm *health.Metrics, cap *capture.Capture, server *Server, alerts *alertbuf.AlertBuffer, recovery recoveryDiagnostics, ver string) health.MCPHealthResponse {
	var serverDeps health.ServerDeps
	if server != nil {
		serverDeps = &serverDepsAdapter{s: server}
	}
	var upgrade health.UpgradeProvider
	if server != nil && server.runtime != nil && server.runtime.Upgrade() != nil {
		upgrade = server.runtime.Upgrade()
	}
	response := hm.GetHealth(cap, serverDeps, upgrade, getLaunchModeInfo, alerts, ver)
	if recovery != nil {
		stats := recovery.Stats()
		oldestAge := int64(0)
		for _, diagnostic := range recovery.Snapshot() {
			age := time.Since(diagnostic.FirstSeenAt).Milliseconds()
			if age > oldestAge {
				oldestAge = age
			}
		}
		response.ResourcePressure["doctor_timeline"] = health.ResourcePressure{
			Entries: stats.Recovered, Capacity: stats.RecoveredLimit, DroppedCount: int64(stats.DroppedRecovered),
			OldestAgeMs: oldestAge, ActiveEntries: stats.Active, RecoverableEntries: stats.Recovered,
		}
	}
	return response
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
		NetworkBodies:      func() []types.NetworkBody { return h.capture.Telemetry().NetworkBodies().Snapshot().Bodies },
		AllWebSocketEvents: func() []types.WebSocketEvent { return h.capture.Telemetry().WebSockets().Snapshot().Events },
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
				health := healthreader.New(h.capture).Snapshot()
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
