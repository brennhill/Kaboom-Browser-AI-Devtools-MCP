// Purpose: Tests for bridge timeout enforcement and deadline propagation.
// Docs: docs/features/feature/bridge-restart/index.md

// timeout_test.go — Tests for ToolCallTimeout and ExtractToolAction.
package bridge

import (
	"encoding/json"
	"testing"
	"time"
)

func TestToolCallTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		params   string
		expected time.Duration
	}{
		{"ping gets fast timeout", "ping", `{}`, FastTimeout},
		{"resources/read gets fast timeout", "resources/read", `{}`, FastTimeout},
		{"tools/list gets fast timeout", "tools/list", `{}`, FastTimeout},
		{"observe gets fast timeout", "tools/call", `{"name":"observe","arguments":{"what":"logs"}}`, FastTimeout},
		{"configure gets fast timeout", "tools/call", `{"name":"configure","arguments":{"action":"health"}}`, FastTimeout},
		{"configure replay_sequence gets slow timeout", "tools/call", `{"name":"configure","arguments":{"action":"replay_sequence"}}`, SlowTimeout},
		{"configure playback gets slow timeout", "tools/call", `{"name":"configure","arguments":{"action":"playback"}}`, SlowTimeout},
		{"generate gets fast timeout", "tools/call", `{"name":"generate","arguments":{"format":"reproduction"}}`, FastTimeout},
		{"analyze gets slow timeout", "tools/call", `{"name":"analyze","arguments":{"what":"dom"}}`, SlowTimeout},
		{"interact gets slow timeout", "tools/call", `{"name":"interact","arguments":{"action":"click"}}`, SlowTimeout},
		{"observe screenshot gets slow timeout", "tools/call", `{"name":"observe","arguments":{"what":"screenshot"}}`, SlowTimeout},
		{"observe command_result non-annotation gets fast", "tools/call", `{"name":"observe","arguments":{"what":"command_result","correlation_id":"cmd_123"}}`, FastTimeout},
		{"observe command_result annotation gets blocking poll", "tools/call", `{"name":"observe","arguments":{"what":"command_result","correlation_id":"ann_detail_abc"}}`, BlockingPoll},
		{"malformed params gets fast timeout", "tools/call", `{bad json}`, FastTimeout},
		{"unknown tool gets fast timeout", "tools/call", `{"name":"unknown_tool","arguments":{}}`, FastTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ToolCallTimeout(tc.method, json.RawMessage(tc.params))
			if got != tc.expected {
				t.Errorf("ToolCallTimeout(%s, %s) = %v, want %v", tc.method, tc.params, got, tc.expected)
			}
		})
	}
}

func TestExtractToolAction(t *testing.T) {
	t.Parallel()

	t.Run("non-tools/call returns empty", func(t *testing.T) {
		name, action := ExtractToolAction("ping", json.RawMessage(`{}`))
		if name != "" || action != "" {
			t.Errorf("expected empty, got name=%q action=%q", name, action)
		}
	})

	t.Run("tools/call with action", func(t *testing.T) {
		name, action := ExtractToolAction("tools/call", json.RawMessage(`{"name":"configure","arguments":{"action":"restart"}}`))
		if name != "configure" || action != "restart" {
			t.Errorf("expected configure/restart, got name=%q action=%q", name, action)
		}
	})

	t.Run("tools/call with what", func(t *testing.T) {
		name, action := ExtractToolAction("tools/call", json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`))
		if name != "observe" || action != "logs" {
			t.Errorf("expected observe/logs, got name=%q action=%q", name, action)
		}
	})

	t.Run("malformed params", func(t *testing.T) {
		name, action := ExtractToolAction("tools/call", json.RawMessage(`{bad`))
		if name != "" || action != "" {
			t.Errorf("expected empty for malformed, got name=%q action=%q", name, action)
		}
	})
}
