// page_readiness_test.go — Tests command readiness projection from browser lifecycle state.

package page

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestGetPageInfoProjectsCommandReadinessConditions(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(*capture.Capture){
		"ready": func(cap *capture.Capture) {
			capturefixture.Connect(cap)
			capturefixture.SetPilot(cap, true)
			capturefixture.Track(cap, 42, "https://example.test")
			capturefixture.SetTabStatus(cap, "complete")
		},
		"disconnected": func(cap *capture.Capture) {
			capturefixture.SetPilot(cap, true)
			capturefixture.Track(cap, 42, "https://example.test")
			capturefixture.SetTabStatus(cap, "complete")
			capturefixture.Disconnect(cap)
		},
		"pilot disabled": func(cap *capture.Capture) {
			capturefixture.Connect(cap)
			capturefixture.SetPilot(cap, false)
			capturefixture.Track(cap, 42, "https://example.test")
			capturefixture.SetTabStatus(cap, "complete")
		},
		"no tracked tab": func(cap *capture.Capture) {
			capturefixture.Connect(cap)
			capturefixture.SetPilot(cap, true)
			capturefixture.SetTabStatus(cap, "complete")
		},
		"tab loading": func(cap *capture.Capture) {
			capturefixture.Connect(cap)
			capturefixture.SetPilot(cap, true)
			capturefixture.Track(cap, 42, "https://example.test")
			capturefixture.SetTabStatus(cap, "loading")
		},
	} {
		t.Run(name, func(t *testing.T) {
			cap := capture.NewCapture()
			defer cap.Close()
			setup(cap)
			data := testsupport.ExtractMCPJSON(t, GetPageInfo(testsupport.Deps(cap), mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil))
			if data["page_ready_for_commands"] != (name == "ready") {
				t.Fatalf("page_ready_for_commands = %v", data["page_ready_for_commands"])
			}
			if _, ok := data["metadata"].(map[string]any)["data_age_ms"]; !ok {
				t.Fatal("metadata missing data_age_ms")
			}
		})
	}
}

func TestGetPageInfoIncludesTrackedTabActivityOnlyWhenKnown(t *testing.T) {
	t.Parallel()
	for name, expected := range map[string]any{"active": true, "inactive": false, "unknown": nil} {
		t.Run(name, func(t *testing.T) {
			cap := capture.NewCapture()
			defer cap.Close()
			capturefixture.Track(cap, 42, "https://example.test")
			if known, ok := expected.(bool); ok {
				capturefixture.SetTrackedTabActive(cap, known)
			}
			data := testsupport.ExtractMCPJSON(t, GetPageInfo(testsupport.Deps(cap), mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}, nil))
			value, exists := data["is_active"]
			if expected == nil && exists {
				t.Fatalf("unknown activity exposed as %v", value)
			}
			if expected != nil && (!exists || value != expected) {
				t.Fatalf("is_active = %v, want %v", value, expected)
			}
		})
	}
}

func TestGetPageInfoPrefersTrackedURLOverWaterfallFallback(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	defer cap.Close()
	cap.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{{
		URL: "https://old.example.test/app.js", PageURL: "https://old.example.test/page",
	}}, "https://old.example.test/page")
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 3}
	data := testsupport.ExtractMCPJSON(t, GetPageInfo(testsupport.Deps(cap), req, nil))
	if data["url"] != "https://old.example.test/page" {
		t.Fatalf("waterfall fallback URL = %#v", data["url"])
	}

	capturefixture.Track(cap, 42, "https://current.example.test/page")
	data = testsupport.ExtractMCPJSON(t, GetPageInfo(testsupport.Deps(cap), req, nil))
	if data["url"] != "https://current.example.test/page" {
		t.Fatalf("tracked URL did not override waterfall: %#v", data["url"])
	}
}
