// handlers_coverage_test.go — Unit tests for the configure-local MCP handlers.
// Exercises the pure param-parsing / mode-dispatch / response-shaping logic of each
// handler via a fake Deps, covering happy, error, edge and validation paths.

package toolconfigure

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestNormalizeTelemetryMode(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"off", "off", true},
		{"auto", "auto", true},
		{"full", "full", true},
		{"invalid", "", false},
		{"", "", false},
		{"  off  ", "  off  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := NormalizeTelemetryMode(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Errorf("mode: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestHandleStreamingStatusAndValidation(t *testing.T) {
	buffer := alertbuf.NewAlertBuffer()
	response := HandleStreaming(buffer, newReq(), json.RawMessage(`{"streaming_action":"status"}`))
	isError, text := parseResp(t, response)
	if isError {
		t.Fatalf("streaming status failed: %s", text)
	}
	for _, field := range []string{"config", "notify_count", "pending"} {
		if !strings.Contains(text, `"`+field+`"`) {
			t.Errorf("streaming status missing %q: %s", field, text)
		}
	}
	for _, arguments := range []json.RawMessage{json.RawMessage(`{bad`), json.RawMessage(`{}`)} {
		response = HandleStreaming(buffer, newReq(), arguments)
		if isError, _ := parseResp(t, response); !isError {
			t.Errorf("invalid streaming arguments %s succeeded", arguments)
		}
	}
}

// ---------------------------------------------------------------------------
// Test fakes
// ---------------------------------------------------------------------------

type fakeConfigureDeps struct {
	noiseConfig      *noise.NoiseConfig
	consoleEntries   []types.LogEntry
	networkBodies    []types.NetworkBody
	wsEvents         []types.WebSocketEvent
	tools            []mcp.MCPTool
	moduleExamples   any
	hasCapture       bool
	securityMode     string
	productionParity bool
	rewrites         []string
	telemetryMode    string
	jitterMs         int

	// call-tracking for setters
	setSecurityCalledWith string
	setSecurityRewrites   []string
	setTelemetryCalled    string
	setJitterCalled       int
}

func (f *fakeConfigureDeps) deps() Deps {
	return Deps{
		NoiseConfig:        func() *noise.NoiseConfig { return f.noiseConfig },
		ConsoleEntries:     func() []types.LogEntry { return f.consoleEntries },
		NetworkBodies:      func() []types.NetworkBody { return f.networkBodies },
		AllWebSocketEvents: func() []types.WebSocketEvent { return f.wsEvents },
		ToolsList:          func() []mcp.MCPTool { return f.tools },
		GetToolModuleExamples: func(string) any {
			return f.moduleExamples
		},
		HasCapture: func() bool { return f.hasCapture },
		GetSecurityMode: func() (string, bool, []string) {
			return f.securityMode, f.productionParity, f.rewrites
		},
		SetSecurityMode: func(mode string, rewrites []string) {
			f.setSecurityCalledWith = mode
			f.setSecurityRewrites = rewrites
			f.securityMode = mode
		},
		GetTelemetryMode: func() string { return f.telemetryMode },
		SetTelemetryMode: func(mode string) {
			f.setTelemetryCalled = mode
			f.telemetryMode = mode
		},
		InteractActionSetJitter: func(ms int) {
			f.setJitterCalled = ms
			f.jitterMs = ms
		},
		InteractActionGetJitter: func() int { return f.jitterMs },
	}
}

// parseResp decodes an MCP tool result into (isError, text).
func parseResp(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}

func parseRespJSON(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	isError, text := parseResp(t, response)
	if isError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	if _, payload, found := strings.Cut(text, "\n"); found {
		text = payload
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("invalid result JSON: %v (text=%s)", err, text)
	}
	return result
}

func newReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

// sampleTool builds an MCPTool whose schema exposes a "what" dispatch param with modes.
func sampleTool() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Configure the agent",
		InputSchema: map[string]any{
			"required": []string{"what"},
			"properties": map[string]any{
				"what":  map[string]any{"type": "string", "enum": []string{"telemetry", "noise_rule"}},
				"extra": map[string]any{"type": "string"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// HandleActionJitter
// ---------------------------------------------------------------------------

func TestHandleActionJitter(t *testing.T) {
	tests := []struct {
		name    string
		start   int
		args    string
		wantSet bool
		wantVal int
	}{
		{"no args returns current", 42, ``, false, 42},
		{"set value", 0, `{"action_jitter_ms":100}`, true, 100},
		{"negative clamps to zero", 5, `{"action_jitter_ms":-50}`, true, 0},
		{"over max clamps to 5000", 5, `{"action_jitter_ms":9000}`, true, 5000},
		{"exact max", 0, `{"action_jitter_ms":5000}`, true, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &fakeConfigureDeps{jitterMs: tt.start, setJitterCalled: -1}
			var args json.RawMessage
			if tt.args != "" {
				args = json.RawMessage(tt.args)
			}
			resp := HandleActionJitter(d.deps(), newReq(), args)
			isErr, text := parseResp(t, resp)
			if isErr {
				t.Fatalf("unexpected error response: %s", text)
			}
			if tt.wantSet && d.setJitterCalled != tt.wantVal {
				t.Errorf("InteractActionSetJitter called with %d, want %d", d.setJitterCalled, tt.wantVal)
			}
			if !tt.wantSet && d.setJitterCalled != -1 {
				t.Errorf("InteractActionSetJitter should not have been called")
			}
			if d.jitterMs != tt.wantVal {
				t.Errorf("final jitter = %d, want %d", d.jitterMs, tt.wantVal)
			}
			result := parseRespJSON(t, resp)
			if result["action_jitter_ms"] != float64(tt.wantVal) {
				t.Errorf("response jitter = %#v, want %d", result["action_jitter_ms"], tt.wantVal)
			}
		})
	}

	t.Run("malformed JSON is a lenient status read", func(t *testing.T) {
		d := &fakeConfigureDeps{jitterMs: 17, setJitterCalled: -1}
		result := parseRespJSON(t, HandleActionJitter(d.deps(), mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 42}, json.RawMessage(`{bad`)))
		if result["action_jitter_ms"] != float64(17) || d.setJitterCalled != -1 {
			t.Fatalf("lenient status result = %#v, set=%d", result, d.setJitterCalled)
		}
	})
}

// ---------------------------------------------------------------------------
// HandleTelemetry
// ---------------------------------------------------------------------------

func TestHandleTelemetry(t *testing.T) {
	tests := []struct {
		name    string
		current string
		args    string
		wantErr bool
		wantSet string
	}{
		{"empty returns current", "auto", `{}`, false, ""},
		{"no args returns current", "full", ``, false, ""},
		{"set off", "auto", `{"telemetry_mode":"off"}`, false, "off"},
		{"set full", "off", `{"telemetry_mode":"full"}`, false, "full"},
		{"invalid mode", "off", `{"telemetry_mode":"bogus"}`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &fakeConfigureDeps{telemetryMode: tt.current}
			var args json.RawMessage
			if tt.args != "" {
				args = json.RawMessage(tt.args)
			}
			resp := HandleTelemetry(d.deps(), newReq(), args)
			isErr, _ := parseResp(t, resp)
			if isErr != tt.wantErr {
				t.Fatalf("isError = %v, want %v", isErr, tt.wantErr)
			}
			if d.setTelemetryCalled != tt.wantSet {
				t.Errorf("SetTelemetryMode called with %q, want %q", d.setTelemetryCalled, tt.wantSet)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleSecurityMode
// ---------------------------------------------------------------------------

func TestHandleSecurityMode(t *testing.T) {
	t.Run("no capture returns error", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: false}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"normal"}`))
		isErr, text := parseResp(t, resp)
		if !isErr {
			t.Fatalf("expected error, got: %s", text)
		}
	})

	t.Run("empty mode returns current", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true, securityMode: syncruntime.SecurityModeNormal, productionParity: true}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{}`))
		isErr, text := parseResp(t, resp)
		if isErr {
			t.Fatalf("unexpected error: %s", text)
		}
		if d.setSecurityCalledWith != "" {
			t.Error("SetSecurityMode should not be called for read")
		}
		result := parseRespJSON(t, resp)
		if result["security_mode"] != syncruntime.SecurityModeNormal || result["production_parity"] != true {
			t.Fatalf("security status = %#v", result)
		}
	})

	t.Run("set normal", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"NORMAL"}`))
		isErr, text := parseResp(t, resp)
		if isErr {
			t.Fatalf("unexpected error: %s", text)
		}
		if d.setSecurityCalledWith != syncruntime.SecurityModeNormal {
			t.Errorf("SetSecurityMode called with %q, want normal", d.setSecurityCalledWith)
		}
	})

	t.Run("insecure_proxy without confirm errors", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"insecure_proxy"}`))
		isErr, _ := parseResp(t, resp)
		if !isErr {
			t.Fatal("expected error when confirm missing")
		}
		if d.setSecurityCalledWith != "" {
			t.Error("SetSecurityMode should not be called without confirm")
		}
	})

	t.Run("insecure_proxy with confirm succeeds", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"insecure_proxy","confirm":true}`))
		isErr, text := parseResp(t, resp)
		if isErr {
			t.Fatalf("unexpected error: %s", text)
		}
		if d.setSecurityCalledWith != syncruntime.SecurityModeInsecureProxy {
			t.Errorf("SetSecurityMode called with %q, want insecure_proxy", d.setSecurityCalledWith)
		}
		if len(d.setSecurityRewrites) == 0 {
			t.Error("expected rewrites to be applied for insecure_proxy")
		}
		result := parseRespJSON(t, resp)
		if result["security_mode"] != syncruntime.SecurityModeInsecureProxy || result["production_parity"] != false {
			t.Fatalf("insecure security response = %#v", result)
		}
	})

	t.Run("normal disables insecure mode", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true, securityMode: syncruntime.SecurityModeInsecureProxy}
		result := parseRespJSON(t, HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"normal"}`)))
		if result["security_mode"] != syncruntime.SecurityModeNormal || result["production_parity"] != true || d.setSecurityCalledWith != syncruntime.SecurityModeNormal {
			t.Fatalf("normal security response = %#v, set=%q", result, d.setSecurityCalledWith)
		}
	})

	t.Run("invalid mode errors", func(t *testing.T) {
		d := &fakeConfigureDeps{hasCapture: true}
		resp := HandleSecurityMode(d.deps(), newReq(), json.RawMessage(`{"mode":"chaos"}`))
		isErr, _ := parseResp(t, resp)
		if !isErr {
			t.Fatal("expected error for invalid mode")
		}
	})
}

// ---------------------------------------------------------------------------
// HandleNoise (+ dispatch and action handlers)
// ---------------------------------------------------------------------------

func TestHandleNoise_NotInitialized(t *testing.T) {
	d := &fakeConfigureDeps{noiseConfig: nil}
	resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"list"}`))
	if isErr, _ := parseResp(t, resp); !isErr {
		t.Fatal("nil noise config should error")
	}
}

func TestHandleNoise_Actions(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"list"}`))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("list should succeed: %s", text)
		}
	})

	t.Run("reset", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"reset"}`))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("reset should succeed: %s", text)
		}
	})

	t.Run("auto_detect with empty inputs", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"auto_detect"}`))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("auto_detect should succeed: %s", text)
		}
	})

	t.Run("add valid rule", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		args := `{"action":"add","rules":[{"category":"console","classification":"noise","match_spec":{"message_regex":"^HMR"}}]}`
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(args))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("add should succeed: %s", text)
		}
	})

	t.Run("add rejected regex errors", func(t *testing.T) {
		// Nested quantifiers are rejected by the noise validator (ReDoS guard).
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		args := `{"action":"add","rules":[{"match_spec":{"message_regex":".*+"}}]}`
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(args))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("nested-quantifier regex should be rejected")
		}
	})

	t.Run("remove without rule_id errors", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"remove"}`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("remove without rule_id should error")
		}
	})

	t.Run("remove nonexistent rule errors", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"remove","rule_id":"user_999"}`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("remove of nonexistent rule should error")
		}
	})

	t.Run("remove existing rule succeeds", func(t *testing.T) {
		nc := noise.NewNoiseConfig()
		d := &fakeConfigureDeps{noiseConfig: nc}
		addArgs := `{"action":"add","rules":[{"match_spec":{"message_regex":"^HMR"}}]}`
		HandleNoise(d.deps(), newReq(), json.RawMessage(addArgs))
		// Find the user rule ID.
		var userID string
		for _, r := range nc.ListRules() {
			if r.ID != "" && r.ID[:4] == "user" {
				userID = r.ID
				break
			}
		}
		if userID == "" {
			t.Fatal("expected a user rule to have been added")
		}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"remove","rule_id":"`+userID+`"}`))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("remove of existing rule should succeed: %s", text)
		}
	})

	t.Run("unknown action errors", func(t *testing.T) {
		d := &fakeConfigureDeps{noiseConfig: noise.NewNoiseConfig()}
		resp := HandleNoise(d.deps(), newReq(), json.RawMessage(`{"action":"teleport"}`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("unknown action should error")
		}
	})
}

// ---------------------------------------------------------------------------
// HandleDescribeCapabilities
// ---------------------------------------------------------------------------

func TestHandleDescribeCapabilities(t *testing.T) {
	t.Run("mode without tool errors", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"mode":"telemetry"}`), "1.0")
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("mode without tool should error")
		}
	})

	t.Run("unknown tool errors", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"tool":"nope"}`), "1.0")
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("unknown tool should error")
		}
	})

	t.Run("tool with valid mode", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"tool":"configure","mode":"telemetry"}`), "1.0")
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("valid tool+mode should succeed: %s", text)
		}
	})

	t.Run("tool with invalid mode errors", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"tool":"configure","mode":"nonexistent"}`), "1.0")
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("invalid mode should error")
		}
	})

	t.Run("tool only with examples", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}, moduleExamples: map[string]any{"ex": 1}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"tool":"configure"}`), "1.0")
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("tool only should succeed: %s", text)
		}
	})

	t.Run("summary", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), json.RawMessage(`{"summary":true}`), "1.0")
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("summary should succeed: %s", text)
		}
	})

	t.Run("full no filter", func(t *testing.T) {
		d := &fakeConfigureDeps{tools: []mcp.MCPTool{sampleTool()}}
		resp := HandleDescribeCapabilities(d.deps(), newReq(), nil, "1.0")
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("full listing should succeed: %s", text)
		}
	})
}
