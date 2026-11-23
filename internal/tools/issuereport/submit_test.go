// submit_test.go — Tests for issue submission via gh CLI and body formatting.

package issuereport

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fakeRunner implements CommandRunner for testing.
type fakeRunner struct {
	lookPathErr error
	stdout      string
	stderr      string
	runErr      error
	lastArgs    []string
}

func (r *fakeRunner) LookPath(_ string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return "/usr/bin/gh", nil
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	r.lastArgs = append([]string{name}, args...)
	return r.stdout, r.stderr, r.runErr
}

func TestSubmitViaGH_GHNotFound(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{lookPathErr: errors.New("not found")}
	report := IssueReport{
		Template: "bug",
		Title:    "Test issue",
	}
	result := SubmitViaGH(context.Background(), report, runner)
	if result.Status != "manual" {
		t.Fatalf("status = %q, want manual", result.Status)
	}
	if result.FormattedBody == "" {
		t.Fatal("formatted_body is empty")
	}
	if result.RepoURL == "" {
		t.Fatal("repo_url is empty")
	}
	if result.Hint == "" {
		t.Fatal("hint is empty")
	}
}

func TestSubmitViaGH_Success(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{
		stdout: "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/42\n",
	}
	report := IssueReport{
		Template: "bug",
		Title:    "Test bug",
		Diagnostics: DiagnosticData{
			Server: ServerDiag{Version: "0.7.10"},
		},
	}
	result := SubmitViaGH(context.Background(), report, runner)
	if result.Status != "submitted" {
		t.Fatalf("status = %q, want submitted", result.Status)
	}
	if result.Method != "gh_cli" {
		t.Fatalf("method = %q, want gh_cli", result.Method)
	}
	if result.IssueURL != "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/42" {
		t.Fatalf("issue_url = %q", result.IssueURL)
	}
}

func TestSubmitViaGH_GHError(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{
		runErr: errors.New("exit status 1"),
		stderr: "authentication required",
	}
	report := IssueReport{
		Template: "bug",
		Title:    "Test bug",
	}
	result := SubmitViaGH(context.Background(), report, runner)
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if result.Method != "gh_cli" {
		t.Fatalf("method = %q, want gh_cli", result.Method)
	}
	if !strings.Contains(result.Error, "authentication required") {
		t.Fatalf("error = %q, want to contain stderr", result.Error)
	}
	if result.FormattedBody == "" {
		t.Fatal("formatted_body should be set on error for manual fallback")
	}
}

func TestSubmitViaGH_IncludesLabels(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{
		stdout: "https://github.com/example/issues/1\n",
	}
	report := IssueReport{
		Template: "bug",
		Title:    "Test bug with labels",
	}
	SubmitViaGH(context.Background(), report, runner)

	argsStr := strings.Join(runner.lastArgs, " ")
	if !strings.Contains(argsStr, "--label") {
		t.Fatalf("expected --label in args: %v", runner.lastArgs)
	}
	if !strings.Contains(argsStr, "bug") {
		t.Fatalf("expected bug label in args: %v", runner.lastArgs)
	}
}

func TestFormatIssueBody_ContainsAllSections(t *testing.T) {
	t.Parallel()
	report := IssueReport{
		Template:    "bug",
		Title:       "Test issue",
		UserContext: "Something went wrong",
		Diagnostics: DiagnosticData{
			Server:    ServerDiag{Version: "0.7.10", UptimeSeconds: 120, TotalCalls: 50, TotalErrors: 2, ErrorRatePct: 4.0},
			Extension: ExtensionDiag{Connected: true, Source: "popup"},
			Platform:  PlatformDiag{OS: "darwin", Arch: "arm64", GoVersion: "go1.22.0"},
			Buffers:   BufferStats{ConsoleEntries: 10, NetworkEntries: 5, ActionEntries: 3},
		},
	}
	body := FormatIssueBody(report)

	checks := []string{
		"## Description",
		"Something went wrong",
		"## Diagnostics",
		"### Server",
		"0.7.10",
		"### Extension",
		"true",
		"### Platform",
		"darwin/arm64",
		"go1.22.0",
		"### Buffers",
		"10 entries",
		"configure(what=\"report_issue\")",
		"bug", // template name
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("body missing %q", check)
		}
	}
}

func TestFormatIssueBody_NoDescriptionWhenEmpty(t *testing.T) {
	t.Parallel()
	report := IssueReport{
		Template: "bug",
		Title:    "Test",
	}
	body := FormatIssueBody(report)
	if strings.Contains(body, "## Description") {
		t.Fatal("body should not contain Description when user_context is empty")
	}
}

// The outbound payload is pinned field by field. Everything below leaves the
// user's machine for a PUBLIC repository, so a new diagnostic added to
// FormatIssueBody, or a new gh flag, has to be added here deliberately rather
// than shipping unnoticed.
func TestSubmitViaGHSendsExactlyTheDocumentedPayload(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{stdout: "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/7\n"}
	report := IssueReport{
		Template:    "bug",
		Title:       "Click fails on modal",
		UserContext: "Happens after the popup opens",
		Diagnostics: DiagnosticData{
			Server:    ServerDiag{Version: "0.9.1", UptimeSeconds: 90, TotalCalls: 12, TotalErrors: 1, ErrorRatePct: 8.3},
			Extension: ExtensionDiag{Connected: true, Source: "sess_abc"},
			Platform:  PlatformDiag{OS: "darwin", Arch: "arm64", GoVersion: "go1.25.0"},
			Buffers:   BufferStats{ConsoleEntries: 3, NetworkEntries: 2, ActionEntries: 1},
		},
	}

	wantBody := "## Description\n\n" +
		"Happens after the popup opens\n\n" +
		"## Diagnostics\n\n" +
		"### Server\n\n" +
		"- **Version:** 0.9.1\n" +
		"- **Uptime:** 90s\n" +
		"- **Total calls:** 12\n" +
		"- **Total errors:** 1\n" +
		"- **Error rate:** 8.3%\n" +
		"\n### Extension\n\n" +
		"- **Connected:** true\n" +
		"- **Source:** sess_abc\n" +
		"\n### Platform\n\n" +
		"- **OS:** darwin/arm64\n" +
		"- **Go:** go1.25.0\n" +
		"\n### Buffers\n\n" +
		"- **Console:** 3 entries\n" +
		"- **Network:** 2 entries\n" +
		"- **Actions:** 1 entries\n" +
		"\n---\n*Filed via `configure(what=\"report_issue\")` | template: bug*\n"

	if body := FormatIssueBody(report); body != wantBody {
		t.Fatalf("outbound issue body changed.\n got:\n%s\nwant:\n%s", body, wantBody)
	}

	SubmitViaGH(context.Background(), report, runner)

	want := []string{
		"gh", "issue", "create",
		"--repo", TargetRepo,
		"--title", "Click fails on modal",
		"--body", wantBody,
		"--label", "bug,user-reported",
	}
	if !reflect.DeepEqual(runner.lastArgs, want) {
		t.Fatalf("outbound command changed.\n got: %#v\nwant: %#v", runner.lastArgs, want)
	}
}

func TestSubmitViaGH_TargetRepo(t *testing.T) {
	t.Parallel()
	if TargetRepo != "brennhill/Kaboom-Browser-AI-Devtools-MCP" {
		t.Fatalf("TargetRepo = %q, want brennhill/Kaboom-Browser-AI-Devtools-MCP", TargetRepo)
	}
}
