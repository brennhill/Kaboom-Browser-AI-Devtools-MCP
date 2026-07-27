// handler_test.go — Tests page-issue evidence normalization.

package pageissues

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestCollectNetworkFailuresCapsAndMapsSeverity(t *testing.T) {
	t.Parallel()
	issues := collectNetworkFailures([]capture.NetworkBody{
		{URL: "/ok", Status: 200},
		{URL: "/missing", Status: 404},
		{URL: "/broken", Status: 503},
	}, 1)
	if len(issues) != 1 || issues[0]["severity"] != "medium" || issues[0]["status"] != 404 {
		t.Fatalf("issues = %#v", issues)
	}
}
