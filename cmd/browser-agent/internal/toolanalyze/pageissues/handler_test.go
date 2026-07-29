// handler_test.go — Tests page-issue evidence normalization.

package pageissues

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type securityScannerStub struct {
	result any
	err    error
}

func (s securityScannerStub) HandleSecurityAudit(
	_ json.RawMessage,
	_ []types.NetworkBody,
	_ []types.LogEntry,
	_ []string,
	_ []types.NetworkWaterfallEntry,
) (any, error) {
	return s.result, s.err
}

func TestHandleAggregatesRequestedChecks(t *testing.T) {
	deps := toolanalyze.Deps{
		GetTrackingStatus: func() (bool, int, string) {
			return true, 7, "https://app.example.test"
		},
		NetworkBodies: func() []types.NetworkBody {
			return []types.NetworkBody{
				{URL: "/ok", Status: 200},
				{URL: "/missing", Method: "GET", Status: 404},
				{URL: "/broken", Method: "POST", Status: 503},
			}
		},
		NetworkWaterfallEntries: func() []types.NetworkWaterfallEntry { return nil },
		LogEntries: func() []types.LogEntry {
			return []types.LogEntry{
				{"level": "info", "message": "ignored"},
				{"level": "warn", "message": "deprecated"},
				{"level": "error", "message": "crashed"},
			}
		},
		ConsoleSecurityEntries: func() []types.LogEntry { return nil },
		ExecuteA11yQuery: func(string, []string, any, bool) (json.RawMessage, error) {
			return json.RawMessage(`{"violations":[
				{"impact":"serious","id":"label","description":"Missing label","nodes":[{},{}]},
				{"impact":"minor","id":"hint","description":"Missing hint","nodes":[{}]}
			]}`), nil
		},
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`11`)}
	resp := Handle(deps, req, json.RawMessage(`{
		"summary":true,
		"categories":["console_errors","network_failures","accessibility"],
		"limit":1
	}`))
	if resp.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", resp.Error)
	}
	text := string(resp.Result)
	for _, want := range []string{
		"Page issues summary",
		`\"total_issues\":3`,
		`\"high\":1`,
		`\"medium\":2`,
		"https://app.example.test",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("response missing %q: %s", want, text)
		}
	}
}

func TestHandleReportsCheckerErrorAndNoTracking(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`12`)}
	noTracking := Handle(toolanalyze.Deps{
		GetTrackingStatus: func() (bool, int, string) { return false, 0, "" },
	}, req, nil)
	if !strings.Contains(string(noTracking.Result), "No tab is being tracked") {
		t.Fatalf("unexpected no-tracking response: %s", noTracking.Result)
	}

	deps := toolanalyze.Deps{
		GetTrackingStatus:       func() (bool, int, string) { return true, 1, "https://example.test" },
		NetworkBodies:           func() []types.NetworkBody { return nil },
		NetworkWaterfallEntries: func() []types.NetworkWaterfallEntry { return nil },
		LogEntries:              func() []types.LogEntry { return nil },
		ConsoleSecurityEntries:  func() []types.LogEntry { return nil },
		ExecuteA11yQuery: func(string, []string, any, bool) (json.RawMessage, error) {
			return nil, errors.New("extension unavailable")
		},
	}
	resp := Handle(deps, req, json.RawMessage(`{"categories":["accessibility"]}`))
	for _, want := range []string{"Page issues scan complete", "extension unavailable", "checks_completed"} {
		if !strings.Contains(string(resp.Result), want) {
			t.Errorf("response missing %q: %s", want, resp.Result)
		}
	}
}

func TestCollectSecurityIssuesNormalizesAndLimitsFindings(t *testing.T) {
	t.Parallel()

	findings := []scan.Finding{
		{Severity: "high", Check: "headers", Title: "Missing CSP", Evidence: "header absent"},
		{Severity: "low", Check: "cookies", Title: "Weak cookie", Evidence: "SameSite absent"},
	}
	deps := toolanalyze.Deps{
		SecurityScanner: func() toolanalyze.SecurityScannerInterface {
			return securityScannerStub{result: scan.Result{Findings: findings}}
		},
	}
	issues, err := collectSecurityIssues(deps, sharedPageData{tabURL: "https://example.test"}, 1)
	if err != nil {
		t.Fatalf("collectSecurityIssues() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("collectSecurityIssues() returned %d issues, want 1", len(issues))
	}
	if issues[0]["check"] != "headers" || issues[0]["title"] != "Missing CSP" {
		t.Fatalf("normalized issue = %#v", issues[0])
	}
}

func TestCollectSecurityIssuesHandlesUnavailableInvalidAndFailedScanners(t *testing.T) {
	t.Parallel()

	noScanner, err := collectSecurityIssues(toolanalyze.Deps{
		SecurityScanner: func() toolanalyze.SecurityScannerInterface { return nil },
	}, sharedPageData{}, 10)
	if err != nil || noScanner != nil {
		t.Fatalf("nil scanner result = %#v, %v; want nil, nil", noScanner, err)
	}

	invalid, err := collectSecurityIssues(toolanalyze.Deps{
		SecurityScanner: func() toolanalyze.SecurityScannerInterface {
			return securityScannerStub{result: "unexpected"}
		},
	}, sharedPageData{}, 10)
	if err != nil || invalid != nil {
		t.Fatalf("invalid scanner result = %#v, %v; want nil, nil", invalid, err)
	}

	wantErr := errors.New("scan failed")
	_, err = collectSecurityIssues(toolanalyze.Deps{
		SecurityScanner: func() toolanalyze.SecurityScannerInterface {
			return securityScannerStub{err: wantErr}
		},
	}, sharedPageData{}, 10)
	if !errors.Is(err, wantErr) {
		t.Fatalf("scanner error = %v, want %v", err, wantErr)
	}
}

func TestCollectNetworkFailuresCapsAndMapsSeverity(t *testing.T) {
	t.Parallel()
	issues := collectNetworkFailures([]types.NetworkBody{
		{URL: "/ok", Status: 200},
		{URL: "/missing", Status: 404},
		{URL: "/broken", Status: 503},
	}, 1)
	if len(issues) != 1 || issues[0]["severity"] != "medium" || issues[0]["status"] != 404 {
		t.Fatalf("issues = %#v", issues)
	}
}
