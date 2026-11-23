// server_state.go — The daemon-owned state the five-tool runtime reads.
// Why: ToolHandler used to hold *main.Server, which pinned the whole composition
// root inside package main — nothing outside it could build a dispatcher, so the
// response-shape harness could only reach observe. ServerState names the eight
// stores the runtime actually touches, so the runtime composes anywhere the
// stores can be constructed.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// Package toolruntime composes the five MCP tools (observe, analyze, generate,
// configure, interact) over the daemon's captured state.
package toolruntime

import (
	annotationruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/runtime"
	appruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/daemonlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/activecodebase"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/doctor/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/doctor/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/listenport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/resetter"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/warningqueue"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/intent"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters/status"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// StateDiagnostics is the Doctor collector the runtime reports recoverable
// failures to. The daemon owns the concrete collector; both sides name it here
// so there is exactly one declaration of what a collector must do.
type StateDiagnostics interface {
	statediag.Reporter
	statediag.Resolver
	Snapshot() []statediag.Diagnostic
	Stats() statediag.CollectorStats
}

// ServerState is every daemon-owned store the five-tool runtime reads. Each
// field is optional: a zero store makes the modes that read it fail their guard
// rather than panic, which is what lets the response-shape harness compose the
// runtime over a seeded fixture instead of a running daemon.
type ServerState struct {
	// Version is the daemon build reported in health, doctor and capabilities.
	Version string
	// Runtime carries start time, release checks and the upgrade provider.
	Runtime *appruntime.Runtime
	// SessionProjectPath roots the on-disk session store.
	SessionProjectPath string

	Logs              *logstore.Store
	Warnings          *warningqueue.Queue
	Incidents         *incident.Store
	PushInbox         *push.PushInbox
	ActiveCodebase    *activecodebase.Store
	AnnotationRuntime *annotationruntime.Owner
	ListenPort        *listenport.Store
	UploadSecurity    *uploadsec.Security
	TerminalStatus    *terminalstatus.Store
	IntentStore       *terminalintent.Store
	StateRecovery     StateDiagnostics
}

// UsageTracker exposes the per-call telemetry tracker the operational API and
// the shutdown flush read.
func (h *ToolHandler) UsageTracker() *telemetry.UsageTracker { return h.usageTracker }

// AuditInfo renders the health-metrics audit block, or nil before metrics exist.
func (h *ToolHandler) AuditInfo() any {
	if h.healthMetrics == nil {
		return nil
	}
	return h.healthMetrics.BuildAuditInfo()
}

// StateRecovery exposes the Doctor collector so the /doctor route can project it.
func (h *ToolHandler) StateRecovery() StateDiagnostics { return h.stateRecovery }

// NewRuntimeResetter binds the coordinated capture reset to a capture store. It
// is the single wiring both configure(what='clear') and the CI /clear route use,
// so the two can never reset different owners.
func NewRuntimeResetter(captured *capture.Capture) *resetter.Resetter {
	return resetter.New(resetter.Dependencies{
		Extension:     captured.Extension(),
		Telemetry:     captured.Telemetry(),
		Performance:   captured.Performance(),
		ExtensionLogs: captured.ExtensionLogs(),
	})
}

// Capture exposes the capture store this handler was composed over.
func (h *ToolHandler) Capture() *capture.Capture { return h.capture }
