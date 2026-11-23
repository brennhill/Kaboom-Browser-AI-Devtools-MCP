// submit.go — Issue submission via gh CLI with manual fallback.

package issuereport

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// TargetRepo is the GitHub repository for issue submission.
	TargetRepo = "brennhill/Kaboom-Browser-AI-Devtools-MCP"

	// TargetRepoURL is the full URL for manual filing.
	TargetRepoURL = "https://github.com/" + TargetRepo

	// ghTimeout is the maximum duration for a gh CLI invocation.
	ghTimeout = 30 * time.Second
)

// CommandRunner abstracts exec.CommandContext for testing.
type CommandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, err error)
}

// ExecRunner is the production CommandRunner that delegates to os/exec.
type ExecRunner struct{}

// LookPath delegates to exec.LookPath.
func (ExecRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Run executes a command and returns stdout, stderr, and any error.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// SubmitViaGH attempts to file an issue using the gh CLI.
// If gh is not installed, returns a manual fallback with the formatted body.
func SubmitViaGH(ctx context.Context, report IssueReport, runner CommandRunner) SubmitResult {
	if runner == nil {
		runner = ExecRunner{}
	}

	body := FormatIssueBody(report)

	// Check if gh is available
	if _, err := runner.LookPath("gh"); err != nil {
		return SubmitResult{
			Status:        "manual",
			FormattedBody: body,
			RepoURL:       TargetRepoURL + "/issues/new",
			Hint:          "gh CLI not found — use the formatted body to file the issue directly",
		}
	}

	tmpl := GetTemplate(report.Template)

	// Build gh issue create command
	args := []string{
		"issue", "create",
		"--repo", TargetRepo,
		"--title", report.Title,
		"--body", body,
	}
	if tmpl != nil && len(tmpl.Labels) > 0 {
		args = append(args, "--label", strings.Join(tmpl.Labels, ","))
	}

	ghCtx, cancel := context.WithTimeout(ctx, ghTimeout)
	defer cancel()

	stdout, stderr, err := runner.Run(ghCtx, "gh", args...)
	if err != nil {
		return SubmitResult{
			Status:        "error",
			Method:        "gh_cli",
			Error:         fmt.Sprintf("gh issue create failed: %v; stderr: %s", err, strings.TrimSpace(stderr)),
			FormattedBody: body,
			RepoURL:       TargetRepoURL + "/issues/new",
			Hint:          "gh CLI failed — use the formatted body to file manually",
		}
	}

	issueURL := strings.TrimSpace(stdout)
	return SubmitResult{
		Status:   "submitted",
		Method:   "gh_cli",
		IssueURL: issueURL,
	}
}

// FormatIssueBody renders an IssueReport as GitHub-flavored markdown.
func FormatIssueBody(report IssueReport) string {
	var b strings.Builder

	if report.UserContext != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(report.UserContext)
		b.WriteString("\n\n")
	}

	b.WriteString("## Diagnostics\n\n")
	b.WriteString("### Server\n\n")
	b.WriteString(fmt.Sprintf("- **Version:** %s\n", report.Diagnostics.Server.Version))
	b.WriteString(fmt.Sprintf("- **Uptime:** %.0fs\n", report.Diagnostics.Server.UptimeSeconds))
	b.WriteString(fmt.Sprintf("- **Total calls:** %d\n", report.Diagnostics.Server.TotalCalls))
	b.WriteString(fmt.Sprintf("- **Total errors:** %d\n", report.Diagnostics.Server.TotalErrors))
	b.WriteString(fmt.Sprintf("- **Error rate:** %.1f%%\n", report.Diagnostics.Server.ErrorRatePct))

	b.WriteString("\n### Extension\n\n")
	b.WriteString(fmt.Sprintf("- **Connected:** %t\n", report.Diagnostics.Extension.Connected))
	if report.Diagnostics.Extension.Source != "" {
		b.WriteString(fmt.Sprintf("- **Source:** %s\n", report.Diagnostics.Extension.Source))
	}

	b.WriteString("\n### Platform\n\n")
	b.WriteString(fmt.Sprintf("- **OS:** %s/%s\n", report.Diagnostics.Platform.OS, report.Diagnostics.Platform.Arch))
	b.WriteString(fmt.Sprintf("- **Go:** %s\n", report.Diagnostics.Platform.GoVersion))

	b.WriteString("\n### Buffers\n\n")
	b.WriteString(fmt.Sprintf("- **Console:** %d entries\n", report.Diagnostics.Buffers.ConsoleEntries))
	b.WriteString(fmt.Sprintf("- **Network:** %d entries\n", report.Diagnostics.Buffers.NetworkEntries))
	b.WriteString(fmt.Sprintf("- **Actions:** %d entries\n", report.Diagnostics.Buffers.ActionEntries))

	b.WriteString(fmt.Sprintf("\n---\n*Filed via `configure(what=\"report_issue\")` | template: %s*\n", report.Template))

	return b.String()
}
