// handler_test.go — Tests combined-audit category validation.

package combinedaudit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	observecore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type auditScannerStub struct{}

func (auditScannerStub) HandleSecurityAudit(json.RawMessage, []types.NetworkBody, []types.LogEntry, []string, []types.NetworkWaterfallEntry) (any, error) {
	return scan.Result{}, nil
}

func auditTestDeps() Deps {
	captured := capture.NewCapture()
	return Deps{
		Analyze: toolanalyze.Deps{
			GetTrackingStatus:       func() (bool, int, string) { return false, 0, "" },
			NetworkBodies:           func() []types.NetworkBody { return nil },
			NetworkWaterfallEntries: func() []types.NetworkWaterfallEntry { return nil },
			ConsoleSecurityEntries:  func() []types.LogEntry { return nil },
			SecurityScanner:         func() toolanalyze.SecurityScannerInterface { return auditScannerStub{} },
		},
		Observe: observecore.Deps{
			Capture: captured,
			ExecuteA11yQuery: func(string, []string, any, bool) (json.RawMessage, error) {
				return json.RawMessage(`{"violations":[]}`), nil
			},
			DiagnosticHintString: func() string { return "" },
		},
	}
}

func TestValidateCategories(t *testing.T) {
	t.Parallel()
	if categories, invalid := validateCategories(nil); invalid != "" || len(categories) != 4 {
		t.Fatalf("defaults = %v, invalid = %q", categories, invalid)
	}
	if _, invalid := validateCategories([]string{"performance", "unknown"}); invalid != "unknown" {
		t.Fatalf("invalid category = %q", invalid)
	}
}

func TestHandleReturnsFilteredAndSummaryAuditContracts(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for name, args := range map[string]json.RawMessage{
		"filtered": json.RawMessage(`{"categories":["security"]}`),
		"summary":  json.RawMessage(`{"summary":true}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := Handle(auditTestDeps(), req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError || len(result.Content) == 0 {
				t.Fatalf("audit response = %s, err=%v", response.Result, err)
			}
			for _, field := range []string{"categories", "overall_score", "timestamp", "recommendations"} {
				if !strings.Contains(result.Content[0].Text, `"`+field+`"`) {
					t.Errorf("audit response missing %q: %s", field, result.Content[0].Text)
				}
			}
			if name == "summary" && !strings.Contains(result.Content[0].Text, `"findings_count"`) {
				t.Fatalf("summary response missing findings_count: %s", result.Content[0].Text)
			}
		})
	}
}

func TestHandleRejectsInvalidCategoryAndJSON(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for _, args := range []json.RawMessage{json.RawMessage(`{"categories":["missing"]}`), json.RawMessage(`{bad`)} {
		response := Handle(auditTestDeps(), req, args)
		var result mcp.MCPToolResult
		if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError {
			t.Fatalf("expected error response for %s: %s, err=%v", args, response.Result, err)
		}
	}
}
