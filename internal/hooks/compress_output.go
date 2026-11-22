// compress_output.go — Output compression for Claude Code PostToolUse hooks.
// Detects test runner and build output patterns, compresses to summary + errors.

package hook

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	minLinesForCompression  = 50
	minLinesForGenericTrunc = 100
	genericHeadLines        = 30
	genericTailLines        = 20
)

// CompressResult holds the outcome of output compression.
type CompressResult struct {
	Category     string
	Compressed   string
	TokensBefore int
	TokensAfter  int
	OrigLines    int
	CompLines    int
}

// CompressOutput detects patterns in Bash tool output and compresses verbose results.
// Returns nil if no compression was applied (output too short or no pattern matched).
func CompressOutput(input Input) *CompressResult {
	if input.ToolName != "Bash" {
		return nil
	}

	output := input.ResponseText()
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	if totalLines < minLinesForCompression {
		return nil
	}

	fields := input.ParseToolInput()
	command := fields.Command

	category, compressed := detectAndCompress(lines, command)

	if compressed == "" && totalLines <= minLinesForGenericTrunc {
		return nil
	}

	if compressed == "" && totalLines > minLinesForGenericTrunc {
		category = "generic_truncation"
		head := strings.Join(lines[:genericHeadLines], "\n")
		tail := strings.Join(lines[totalLines-genericTailLines:], "\n")
		compressed = fmt.Sprintf("%s\n\n...truncated (%d total lines)\n\n%s", head, totalLines, tail)
	}

	if compressed == "" {
		return nil
	}

	compLines := len(strings.Split(compressed, "\n"))
	tokensBefore := len(output) / 4
	tokensAfter := len(compressed) / 4

	return &CompressResult{
		Category:     category,
		Compressed:   compressed,
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		OrigLines:    totalLines,
		CompLines:    compLines,
	}
}

// FormatContext returns the additionalContext string for the hook output.
func (r *CompressResult) FormatContext() string {
	return fmt.Sprintf("[Output compressed: %d lines -> %d lines, ~%d -> ~%d tokens]\n\n%s",
		r.OrigLines, r.CompLines, r.TokensBefore, r.TokensAfter, r.Compressed)
}

// --- Compressor registry ---

// compressorEntry defines a single output compressor with its match condition and handler.
type compressorEntry struct {
	category string
	match    func(cmd string, lines []string) bool
	compress func(lines []string) string
}

// compressors is the ordered registry of output compressors. Order matters:
// cargo test must come before pytest because pytest's regex (`\d+ passed`) also
// matches cargo test's "test result:" summary line, causing a false category match.
var compressors = []compressorEntry{
	// Test runners.
	{"test_output", matchGoTest, compressGoTest},
	{"test_output", matchJest, compressJestVitest},
	{"test_output", matchCargoTest, compressCargoTest},
	{"test_output", matchPytest, compressPytest},
	// Build tools.
	{"build_output", matchGoBuild, compressByGoBuild},
	{"build_output", matchMake, compressByMake},
	{"build_output", matchTsc, compressByTsc},
	{"build_output", matchNpmBuild, compressByNpmBuild},
	{"build_output", matchCargoBuild, compressByCargoBuild},
}

// detectAndCompress tries each registered compressor and returns the first match.
func detectAndCompress(lines []string, command string) (string, string) {
	cmd := strings.ToLower(command)
	for _, c := range compressors {
		if c.match(cmd, lines) {
			if result := c.compress(lines); result != "" {
				return c.category, result
			}
		}
	}
	return "", ""
}

// --- Match functions ---

var (
	makeCmdPattern  = regexp.MustCompile(`^make\b`)
	goErrorPattern  = regexp.MustCompile(`.*\.go:\d+:\d+:.*`)
	durationPattern = regexp.MustCompile(`(\d+\.\d+s)`)
	pytestPattern   = regexp.MustCompile(`\d+ passed|\d+ failed|\d+ error`)
)

func matchGoTest(cmd string, lines []string) bool {
	return strings.Contains(cmd, "go test") || hasGoTestMarkers(lines)
}

func matchJest(cmd string, lines []string) bool {
	return containsAny(cmd, "jest", "vitest") || hasJestMarkers(lines)
}

func matchCargoTest(cmd string, lines []string) bool {
	return strings.Contains(cmd, "cargo test") || hasCargoTestMarkers(lines)
}

func matchPytest(cmd string, lines []string) bool {
	return strings.Contains(cmd, "pytest") || hasPytestMarkers(lines)
}

func matchGoBuild(cmd string, _ []string) bool {
	return containsAny(cmd, "go build", "go vet")
}

func matchMake(cmd string, _ []string) bool {
	return makeCmdPattern.MatchString(cmd)
}

func matchTsc(cmd string, _ []string) bool {
	return strings.Contains(cmd, "tsc")
}

func matchNpmBuild(cmd string, _ []string) bool {
	return containsAny(cmd, "npm run build", "webpack")
}

func matchCargoBuild(cmd string, _ []string) bool {
	return strings.Contains(cmd, "cargo build")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// --- Marker detection (content-based, no command needed) ---

func hasGoTestMarkers(lines []string) bool {
	for _, l := range lines {
		s := strings.TrimSpace(l)
		if strings.HasPrefix(s, "--- PASS:") || strings.HasPrefix(s, "--- FAIL:") || strings.HasPrefix(s, "ok \t") {
			return true
		}
	}
	return false
}

func hasJestMarkers(lines []string) bool {
	for _, l := range lines {
		if strings.Contains(l, "Test Suites:") || strings.Contains(l, "Tests:") {
			return true
		}
	}
	return false
}

func hasPytestMarkers(lines []string) bool {
	end := len(lines)
	start := end - 20
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:end] {
		if pytestPattern.MatchString(l) {
			return true
		}
	}
	return false
}

func hasCargoTestMarkers(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "test result:") {
			return true
		}
	}
	return false
}

// --- Test runner compressors (complex, bespoke logic) ---

func compressGoTest(lines []string) string {
	scan := scanGoTest(lines)
	if len(scan.passed) == 0 && len(scan.failed) == 0 && len(scan.summaryLines) == 0 {
		return ""
	}
	return renderGoTestSummary(scan)
}

type goTestScan struct {
	passed       []string
	failed       []string
	summaryLines []string
	failDetails  map[string]string
	durations    []string
}

func scanGoTest(lines []string) goTestScan {
	var scan goTestScan
	scan.failDetails = map[string]string{}
	var currentRun string
	var pendingErrors []string

	for _, line := range lines {
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "=== RUN"):
			parts := strings.Fields(s)
			if len(parts) > 2 {
				currentRun = parts[len(parts)-1]
			}
			pendingErrors = nil
		case strings.HasPrefix(s, "--- PASS:"):
			scan.passed = append(scan.passed, s)
			currentRun = ""
			pendingErrors = nil
		case strings.HasPrefix(s, "--- FAIL:"):
			scan.failed = append(scan.failed, s)
			if len(pendingErrors) > 0 {
				scan.failDetails[s] = pendingErrors[0]
			}
			currentRun = ""
			pendingErrors = nil
		case strings.HasPrefix(s, "FAIL") && !strings.HasPrefix(s, "--- FAIL:"):
			scan.summaryLines = append(scan.summaryLines, s)
		case strings.HasPrefix(s, "ok "):
			scan.summaryLines = append(scan.summaryLines, s)
			if m := durationPattern.FindString(s); m != "" {
				scan.durations = append(scan.durations, m)
			}
		default:
			if isPendingErrorLine(s, currentRun) {
				pendingErrors = append(pendingErrors, s)
			}
		}
	}

	return scan
}

func isPendingErrorLine(s, currentRun string) bool {
	return currentRun != "" && s != "" && !strings.HasPrefix(s, "===")
}

func renderGoTestSummary(scan goTestScan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "go test summary: %d passed, %d failed", len(scan.passed), len(scan.failed))
	if len(scan.durations) > 0 {
		fmt.Fprintf(&b, "\nduration: %s", scan.durations[len(scan.durations)-1])
	}
	if len(scan.failed) > 0 {
		b.WriteString("\n\nFAILED TESTS:")
		for _, f := range scan.failed {
			fmt.Fprintf(&b, "\n  %s", f)
			if detail, ok := scan.failDetails[f]; ok {
				fmt.Fprintf(&b, "\n    %s", detail)
			}
		}
	}
	if len(scan.summaryLines) > 0 {
		b.WriteString("\n")
		for _, s := range scan.summaryLines {
			fmt.Fprintf(&b, "\n%s", s)
		}
	}

	return b.String()
}

func compressJestVitest(lines []string) string {
	var summary, failFiles []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if containsAny(s, "Test Suites:", "Tests:", "Snapshots:", "Time:") {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "FAIL ") && strings.Contains(s, "/") {
			failFiles = append(failFiles, s)
		}
	}
	if len(summary) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("jest/vitest summary:")
	if len(failFiles) > 0 {
		b.WriteString("\nFAILURES:")
		for _, f := range failFiles {
			fmt.Fprintf(&b, "\n  %s", f)
		}
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressPytest(lines []string) string {
	var summary, failures []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if pytestPattern.MatchString(s) {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "FAILED ") || strings.HasPrefix(s, "ERROR ") {
			failures = append(failures, s)
		}
	}
	if len(summary) == 0 && len(failures) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("pytest summary:")
	for _, f := range failures {
		fmt.Fprintf(&b, "\n%s", f)
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressCargoTest(lines []string) string {
	var summary, failures []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "test result:") {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "test ") && strings.Contains(s, "FAILED") {
			failures = append(failures, s)
		}
	}
	if len(summary) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("cargo test summary:")
	if len(failures) > 0 {
		b.WriteString("\nFAILURES:")
		for _, f := range failures {
			fmt.Fprintf(&b, "\n  %s", f)
		}
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

// --- Build tool compressors (uniform predicate pattern) ---

// compressByPredicate extracts matching lines and formats them as "header: N issue(s):".
func compressByPredicate(lines []string, header string, match func(string) bool) string {
	var matched []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if match(s) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d issue(s):", header, len(matched))
	for _, m := range matched {
		fmt.Fprintf(&b, "\n%s", m)
	}
	return b.String()
}

func compressByGoBuild(lines []string) string {
	return compressByPredicate(lines, "go build/vet", func(s string) bool {
		return goErrorPattern.MatchString(s) || strings.HasPrefix(s, "#")
	})
}

func compressByMake(lines []string) string {
	return compressByPredicate(lines, "make", func(s string) bool {
		return strings.Contains(s, "Error") || strings.Contains(s, "make: ***") ||
			strings.Contains(strings.ToLower(s), "warning:")
	})
}

func compressByTsc(lines []string) string {
	return compressByPredicate(lines, "tsc", func(s string) bool {
		return strings.Contains(s, "error TS")
	})
}

func compressByNpmBuild(lines []string) string {
	return compressByPredicate(lines, "build", func(s string) bool {
		return strings.Contains(s, "ERROR") || strings.Contains(s, "Module not found")
	})
}

func compressByCargoBuild(lines []string) string {
	return compressByPredicate(lines, "cargo build", func(s string) bool {
		return strings.Contains(s, "error[E")
	})
}
