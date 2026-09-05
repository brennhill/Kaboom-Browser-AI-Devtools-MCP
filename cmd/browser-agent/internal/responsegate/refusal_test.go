// refusal_test.go — Classifies every mode the sweep could not derive a shape
// from, so a refusal can never be mistaken for an oversight.
//
// PURPOSE: a mode that answers "extension not connected" has not answered. Its
// degraded reply must NOT be frozen as the contract — freezing it would pin a
// shape no connected caller ever receives, and the gate would then defend the
// wrong thing forever. So the sweep refuses those modes and lets them fall to
// the undeclared ratchet.
//
// The danger in that policy is silence: a mode refused because THIS FIXTURE
// forgot something looks exactly like a mode refused because a browser is
// genuinely required. Every refusal is therefore classified from the response
// the handler actually produced, and a refusal that fits no known reason fails
// the control below by name.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsegate

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Refusal kinds. Only the first three are legitimate; anything else is a gap in
// this fixture and fails TestEveryRefusalIsExplained.
const (
	// refusedNeedsBrowser: the handler's own guard reported the live runtime
	// state as disconnected or untracked. Only a connected extension answers.
	refusedNeedsBrowser = "needs a live extension"
	// refusedNeedsArgument: the handler rejected the call for a caller-supplied
	// argument the sweep cannot invent (a selector, a saved name, a recording
	// id). Inventing one would put the fixture's input into the contract.
	refusedNeedsArgument = "needs a caller-supplied argument"
	// refusedNotDriveable: invoking the mode would end or restart this process.
	refusedNotDriveable = "cannot be driven inside a test process"
	// refusedUnexplained: none of the above — an oversight, not a limit.
	refusedUnexplained = "unexplained"
)

// browserMarkers are strings the PRODUCT itself emits when a browser
// precondition fails. Their presence is the handler's own evidence that the
// refusal is about the missing extension, not about this fixture. Each covers a
// separate remediation path: toolguard renders the live runtime state into its
// hint, the tracked-tab guard names the popup, and a stale read is prefixed
// with the degraded banner before its error.
var browserMarkers = []string{
	"extension=DISCONNECTED",
	"Track This Tab",
	"Extension is not connected",
	"extension is installed and the page is open",
}

// notDriveable names the modes the sweep must not invoke at all, with the
// reason. configure/restart signals SIGTERM to its own process 100ms after
// answering, so calling it kills the test binary mid-sweep — the shapes of
// every mode after it would silently vanish.
var notDriveable = map[string]string{
	"configure/restart": "it signals SIGTERM to its own process, which would kill the sweep",
}

// argumentErrorCodes are the structured error codes that mean "the caller did
// not supply something", as opposed to "the browser is not there".
var argumentErrorCodes = []string{
	`"error_code":"missing_param"`,
	`"error_code":"invalid_param"`,
	`"error_code":"invalid_json"`,
	`"error_code":"not_initialized"`,
}

// classifyRefusal reads the response the handler produced and names why no
// shape could be derived from it.
func classifyRefusal(response mcp.JSONRPCResponse) string {
	text := responseText(response)
	for _, marker := range browserMarkers {
		if strings.Contains(text, marker) {
			return refusedNeedsBrowser
		}
	}
	for _, code := range argumentErrorCodes {
		if strings.Contains(text, code) {
			return refusedNeedsArgument
		}
	}
	return refusedUnexplained
}

// responseText returns the first content block of a tool response, or "".
func responseText(response mcp.JSONRPCResponse) string {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

// TestEveryRefusalIsExplained is the control on the refusal policy. Without it,
// a mode this fixture simply forgot to seed would look identical to a mode that
// truly needs a browser, and the undeclared ratchet would quietly absorb it.
func TestEveryRefusalIsExplained(t *testing.T) {
	swept := sweep(t)

	unexplained := make([]string, 0)
	byKind := map[string]int{}
	for _, mode := range sortedKeys(swept.refusals) {
		record := swept.refusals[mode]
		byKind[record.kind]++
		if record.kind == refusedUnexplained {
			unexplained = append(unexplained, mode+": "+firstLine(record.detail))
		}
	}

	if len(unexplained) > 0 {
		t.Fatalf("%d mode(s) were refused for no recognised reason. That is an oversight in the harness, not a browser requirement — either give the mode what it needs in extraArgs, or record it in notDriveable with a reason:\n  %s",
			len(unexplained), strings.Join(unexplained, "\n  "))
	}
	t.Logf("refusals by reason: %d %s, %d %s, %d %s",
		byKind[refusedNeedsBrowser], refusedNeedsBrowser,
		byKind[refusedNeedsArgument], refusedNeedsArgument,
		byKind[refusedNotDriveable], refusedNotDriveable)
}

// TestModesRefusedForNeedingABrowserAreNotDeclared proves the policy holds in
// the checked-in contract: a mode whose only reply here is a degraded
// "extension not connected" must have no frozen shape.
func TestModesRefusedForNeedingABrowserAreNotDeclared(t *testing.T) {
	swept := sweep(t)
	contract := loadContract(t)

	frozen := make([]string, 0)
	for mode, record := range swept.refusals {
		if record.kind != refusedNeedsBrowser {
			continue
		}
		if _, declared := contract.Modes[mode]; declared {
			frozen = append(frozen, mode)
		}
	}
	sort.Strings(frozen)
	if len(frozen) > 0 {
		t.Fatalf("%d mode(s) have a frozen shape but only answer with a degraded 'extension not connected' reply here. That shape is not the one a connected caller receives:\n  %s",
			len(frozen), strings.Join(frozen, "\n  "))
	}
}

// firstLine trims a refusal detail to its first line for legible failures.
func firstLine(detail string) string {
	if index := strings.IndexByte(detail, '\n'); index >= 0 {
		return detail[:index]
	}
	return detail
}

func sortedKeys(records map[string]refusalRecord) []string {
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
