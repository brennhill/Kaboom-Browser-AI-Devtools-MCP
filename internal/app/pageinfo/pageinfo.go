// pageinfo.go — Transport-independent tracked-page projection: URL, title, readiness, CSP, focus.
// Why: observe(what="page") and interact's page-context enrichment both need the same facts about
// the tracked tab; the interact path previously re-parsed the MCP response of the observe adapter.
// This use case owns the projection behind narrow ports so MCP assembly stays in the adapters.
// Docs: docs/features/feature/observe/index.md

// Package pageinfo projects the tracked page's state for command readiness.
// It must not import internal/mcp, internal/capture/**, net/http or cmd/**
// (enforced by scripts/contracts/layering).
package pageinfo

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ExtensionState is the extension-connection state the projection reads.
// internal/capture/syncruntime.Runtime satisfies it.
type ExtensionState interface {
	GetTrackingStatus() (enabled bool, tabID int, tabURL string)
	GetTrackedTabTitle() string
	GetCSPStatus() (restricted bool, level string)
	GetTabStatus() string
	IsExtensionConnected() bool
	IsPilotEnabled() bool
	IsTrackedTabActive() (active bool, known bool)
}

// Deps wires the projection to its read owners. Nil fields degrade to empty
// fallbacks; a nil Extension reports an untracked, not-ready page.
type Deps struct {
	Extension ExtensionState
	// LastWaterfallPageURL is the URL fallback when no tab is tracked.
	LastWaterfallPageURL func() string
	// LogEntries supplies the title fallback (newest entry carrying "title").
	LogEntries func() ([]types.LogEntry, []time.Time)
}

// Info is one consistent-enough snapshot of the tracked page.
type Info struct {
	URL, Title    string
	Tracked       bool
	TabID         int
	CSPRestricted bool
	CSPLevel      string
	TabStatus     string
	// ReadyForCommands holds when the extension is connected, pilot is
	// enabled, a tab is tracked and that tab finished loading.
	ReadyForCommands bool
	TabActive        bool
	TabActiveKnown   bool
}

// Service runs the projection.
type Service struct{ deps Deps }

// New binds the projection to its ports.
func New(deps Deps) *Service { return &Service{deps: deps} }

// Read projects the current page state. Each getter locks independently, so
// state may shift between reads; this is acceptable for an advisory readiness
// signal and the next read corrects it.
func (s *Service) Read() Info {
	ext := s.deps.Extension
	if ext == nil {
		return Info{URL: s.waterfallURL(), Title: s.logTitle()}
	}
	enabled, tabID, trackedURL := ext.GetTrackingStatus()
	info := Info{Tracked: enabled, TabID: tabID, URL: trackedURL, Title: ext.GetTrackedTabTitle()}
	if info.URL == "" {
		info.URL = s.waterfallURL()
	}
	if info.Title == "" {
		info.Title = s.logTitle()
	}
	info.CSPRestricted, info.CSPLevel = ext.GetCSPStatus()
	info.TabStatus = ext.GetTabStatus()
	// IsPilotEnabled (defaults false) instead of IsPilotActionAllowed (defaults
	// true during startup) avoids reporting ready before the first sync
	// confirms pilot status.
	info.ReadyForCommands = ext.IsExtensionConnected() && ext.IsPilotEnabled() && enabled && info.TabStatus == "complete"
	info.TabActive, info.TabActiveKnown = ext.IsTrackedTabActive()
	return info
}

func (s *Service) waterfallURL() string {
	if s.deps.LastWaterfallPageURL == nil {
		return ""
	}
	return s.deps.LastWaterfallPageURL()
}

func (s *Service) logTitle() string {
	if s.deps.LogEntries == nil {
		return ""
	}
	entries, _ := s.deps.LogEntries()
	for i := len(entries) - 1; i >= 0; i-- {
		if title, ok := entries[i]["title"].(string); ok && title != "" {
			return title
		}
	}
	return ""
}
