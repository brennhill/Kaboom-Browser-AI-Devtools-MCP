// pageinfo_test.go — Tests the transport-independent tracked-page projection.
// Docs: docs/features/feature/observe/index.md

package pageinfo

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

type fakeExtension struct {
	connected, pilot, tracked bool
	tabID                     int
	url, title, status        string
	cspRestricted             bool
	cspLevel                  string
	active, activeKnown       bool
}

func (f fakeExtension) GetTrackingStatus() (bool, int, string) { return f.tracked, f.tabID, f.url }
func (f fakeExtension) GetTrackedTabTitle() string             { return f.title }
func (f fakeExtension) GetCSPStatus() (bool, string)           { return f.cspRestricted, f.cspLevel }
func (f fakeExtension) GetTabStatus() string                   { return f.status }
func (f fakeExtension) IsExtensionConnected() bool             { return f.connected }
func (f fakeExtension) IsPilotEnabled() bool                   { return f.pilot }
func (f fakeExtension) IsTrackedTabActive() (bool, bool)       { return f.active, f.activeKnown }

func ready() fakeExtension {
	return fakeExtension{connected: true, pilot: true, tracked: true, tabID: 42, url: "https://a.test", title: "A", status: "complete"}
}

func TestReadProjectsCommandReadiness(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*fakeExtension){
		"ready":          func(*fakeExtension) {},
		"disconnected":   func(f *fakeExtension) { f.connected = false },
		"pilot disabled": func(f *fakeExtension) { f.pilot = false },
		"no tracked tab": func(f *fakeExtension) { f.tracked = false },
		"tab loading":    func(f *fakeExtension) { f.status = "loading" },
	}
	for name, mutate := range cases {
		ext := ready()
		mutate(&ext)
		info := New(Deps{Extension: ext}).Read()
		if info.ReadyForCommands != (name == "ready") {
			t.Fatalf("%s: ReadyForCommands = %v", name, info.ReadyForCommands)
		}
	}
}

func TestReadFallsBackToWaterfallURLAndLogTitle(t *testing.T) {
	t.Parallel()
	ext := fakeExtension{}
	service := New(Deps{
		Extension:            ext,
		LastWaterfallPageURL: func() string { return "https://old.test/page" },
		LogEntries: func() ([]types.LogEntry, []time.Time) {
			return []types.LogEntry{{"title": "Older"}, {"title": "Newest"}, {"message": "no title"}}, nil
		},
	})
	info := service.Read()
	if info.URL != "https://old.test/page" || info.Title != "Newest" {
		t.Fatalf("fallback info = %+v", info)
	}

	ext = ready()
	info = New(Deps{Extension: ext, LastWaterfallPageURL: func() string { return "https://old.test/page" }}).Read()
	if info.URL != "https://a.test" || info.Title != "A" || info.TabID != 42 {
		t.Fatalf("tracked info = %+v", info)
	}
}

func TestReadCarriesCSPAndActivity(t *testing.T) {
	t.Parallel()
	ext := ready()
	ext.cspRestricted, ext.cspLevel = true, "script_exec"
	ext.active, ext.activeKnown = false, true
	info := New(Deps{Extension: ext}).Read()
	if !info.CSPRestricted || info.CSPLevel != "script_exec" || info.TabActive || !info.TabActiveKnown {
		t.Fatalf("info = %+v", info)
	}
}

func TestReadWithoutExtensionReportsUntracked(t *testing.T) {
	t.Parallel()
	if info := New(Deps{}).Read(); info.Tracked || info.ReadyForCommands || info.URL != "" {
		t.Fatalf("degraded info = %+v", info)
	}
}
