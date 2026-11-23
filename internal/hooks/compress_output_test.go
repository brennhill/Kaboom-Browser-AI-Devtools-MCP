// compress_output_test.go — Tests for output compression hook logic.

package hook

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func makeInput(toolName, command, output string) Input {
	ti, _ := json.Marshal(map[string]string{"command": command})
	tr, _ := json.Marshal(output)
	return Input{
		ToolName:     toolName,
		ToolInput:    ti,
		ToolResponse: tr,
	}
}

func TestCompressOutput_NotBash(t *testing.T) {
	t.Parallel()
	in := makeInput("Edit", "", "some output\n")
	if result := CompressOutput(in); result != nil {
		t.Error("expected nil for non-Bash tool")
	}
}

func TestCompressOutput_EmptyOutput(t *testing.T) {
	t.Parallel()
	in := makeInput("Bash", "ls", "")
	if result := CompressOutput(in); result != nil {
		t.Error("expected nil for empty output")
	}
}

func TestCompressOutput_ShortOutput(t *testing.T) {
	t.Parallel()
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	in := makeInput("Bash", "echo hello", strings.Join(lines, "\n"))
	if result := CompressOutput(in); result != nil {
		t.Error("expected nil for short output (<50 lines)")
	}
}

func TestCompressOutput_GoTest_AllPass(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("=== RUN   TestFunc%d", i))
		lines = append(lines, fmt.Sprintf("--- PASS: TestFunc%d (0.01s)", i))
	}
	lines = append(lines, "ok  \tgithub.com/test/pkg\t1.234s")

	in := makeInput("Bash", "go test ./...", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "test_output" {
		t.Errorf("Category = %q, want test_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "60 passed, 0 failed") {
		t.Errorf("missing pass count in: %s", result.Compressed)
	}
	if !strings.Contains(result.Compressed, "1.234s") {
		t.Errorf("missing duration in: %s", result.Compressed)
	}
	if result.CompLines >= result.OrigLines {
		t.Errorf("compressed (%d) should be < original (%d)", result.CompLines, result.OrigLines)
	}
}

func TestCompressOutput_GoTest_WithFailures(t *testing.T) {
	t.Parallel()
	lines := []string{
		"=== RUN   TestGood",
		"--- PASS: TestGood (0.01s)",
		"=== RUN   TestBad",
		"    main_test.go:42: expected 5, got 3",
		"--- FAIL: TestBad (0.02s)",
		"FAIL\tgithub.com/test/pkg\t0.03s",
	}
	// Pad to >50 lines.
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("=== RUN   TestPad%d", i))
		lines = append(lines, fmt.Sprintf("--- PASS: TestPad%d (0.00s)", i))
	}

	in := makeInput("Bash", "go test ./...", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if !strings.Contains(result.Compressed, "FAILED TESTS:") {
		t.Error("missing FAILED TESTS section")
	}
	if !strings.Contains(result.Compressed, "TestBad") {
		t.Error("missing failed test name")
	}
	if !strings.Contains(result.Compressed, "expected 5, got 3") {
		t.Error("missing error detail")
	}
}

func TestCompressOutput_JestVitest(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("  PASS src/test%d.test.ts", i))
	}
	lines = append(lines, "FAIL src/broken.test.ts")
	lines = append(lines, "Test Suites:  1 failed, 55 passed, 56 total")
	lines = append(lines, "Tests:       2 failed, 120 passed, 122 total")
	lines = append(lines, "Time:        3.456 s")

	in := makeInput("Bash", "npx jest", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "test_output" {
		t.Errorf("Category = %q, want test_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "jest/vitest summary:") {
		t.Error("missing jest/vitest header")
	}
	if !strings.Contains(result.Compressed, "FAIL src/broken.test.ts") {
		t.Error("missing failure file")
	}
}

func TestCompressOutput_GoBuild_Errors(t *testing.T) {
	t.Parallel()
	var lines []string
	lines = append(lines, "# github.com/test/pkg")
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("./main.go:%d:5: undefined: Foo%d", i+1, i))
	}

	in := makeInput("Bash", "go build ./...", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "build_output" {
		t.Errorf("Category = %q, want build_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "issue(s):") {
		t.Error("missing error count")
	}
}

func TestCompressOutput_Tsc_Errors(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("src/foo.ts(%d,5): error TS2304: Cannot find name 'x'", i))
	}

	in := makeInput("Bash", "npx tsc --noEmit", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "build_output" {
		t.Errorf("Category = %q, want build_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "tsc:") {
		t.Error("missing tsc header")
	}
}

func TestCompressOutput_Make_Errors(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("compiling step %d...", i))
	}
	lines = append(lines, "make: *** [Makefile:42: build] Error 2")

	in := makeInput("Bash", "make build", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if !strings.Contains(result.Compressed, "make:") {
		t.Error("missing make header")
	}
}

func TestCompressOutput_GenericTruncation(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 150; i++ {
		lines = append(lines, fmt.Sprintf("verbose output line %d", i))
	}

	in := makeInput("Bash", "some-unknown-command", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result for >100 lines")
	}
	if result.Category != "generic_truncation" {
		t.Errorf("Category = %q, want generic_truncation", result.Category)
	}
	if !strings.Contains(result.Compressed, "...truncated (150 total lines)") {
		t.Errorf("missing truncation notice in: %s", result.Compressed)
	}
	if !strings.Contains(result.Compressed, "verbose output line 0") {
		t.Error("missing head lines")
	}
	if !strings.Contains(result.Compressed, "verbose output line 149") {
		t.Error("missing tail lines")
	}
}

func TestCompressOutput_NoCompressionForMediumUnknown(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 80; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}

	in := makeInput("Bash", "some-command", strings.Join(lines, "\n"))
	if result := CompressOutput(in); result != nil {
		t.Error("expected nil for 80-line unknown output (between 50-100)")
	}
}

func TestCompressOutput_FormatContext(t *testing.T) {
	t.Parallel()
	r := &CompressResult{
		OrigLines:    100,
		CompLines:    5,
		TokensBefore: 2000,
		TokensAfter:  50,
		Compressed:   "summary here",
	}
	ctx := r.FormatContext()
	if !strings.Contains(ctx, "100 lines -> 5 lines") {
		t.Errorf("missing line counts in: %s", ctx)
	}
	if !strings.Contains(ctx, "summary here") {
		t.Errorf("missing compressed content in: %s", ctx)
	}
}

func TestCompressOutput_Pytest(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("test_module.py::test_%d PASSED", i))
	}
	lines = append(lines, "FAILED test_module.py::test_bad - AssertionError")
	lines = append(lines, "3 passed, 1 failed in 2.5s")

	in := makeInput("Bash", "pytest", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "test_output" {
		t.Errorf("Category = %q, want test_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "pytest summary:") {
		t.Error("missing pytest header")
	}
}

func TestCompressOutput_CargoTest(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 55; i++ {
		lines = append(lines, fmt.Sprintf("test test_%d ... ok", i))
	}
	lines = append(lines, "test test_bad ... FAILED")
	lines = append(lines, "test result: FAILED. 55 passed; 1 failed; 0 ignored")

	in := makeInput("Bash", "cargo test", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression result")
	}
	if result.Category != "test_output" {
		t.Errorf("Category = %q, want test_output", result.Category)
	}
	if !strings.Contains(result.Compressed, "cargo test summary:") {
		t.Error("missing cargo test header")
	}
}

func TestCompressOutput_ContentDetection_NoCommand(t *testing.T) {
	t.Parallel()
	// Even without "go test" in command, detect by output markers.
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("=== RUN   TestFunc%d", i))
		lines = append(lines, fmt.Sprintf("--- PASS: TestFunc%d (0.01s)", i))
	}
	lines = append(lines, "ok  \tgithub.com/test/pkg\t1.0s")

	in := makeInput("Bash", "make test", strings.Join(lines, "\n"))
	result := CompressOutput(in)
	if result == nil {
		t.Fatal("expected compression via content detection")
	}
	if result.Category != "test_output" {
		t.Errorf("Category = %q, want test_output", result.Category)
	}
}

func TestBuildCompressorsKeepOnlyActionableLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		got   string
		keeps []string
		drops string
	}{
		{
			name:  "npm",
			got:   compressByNpmBuild([]string{"building", "ERROR bundle failed", "Module not found: x"}),
			keeps: []string{"ERROR bundle failed", "Module not found: x"},
			drops: "building",
		},
		{
			name:  "cargo",
			got:   compressByCargoBuild([]string{"Compiling crate", "error[E0308]: mismatched types"}),
			keeps: []string{"error[E0308]: mismatched types"},
			drops: "Compiling crate",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.keeps {
				if !strings.Contains(tt.got, want) {
					t.Errorf("compressed output %q does not contain %q", tt.got, want)
				}
			}
			if strings.Contains(tt.got, tt.drops) {
				t.Errorf("compressed output %q retained noise %q", tt.got, tt.drops)
			}
		})
	}

	if got := compressByNpmBuild([]string{"building", "done"}); got != "" {
		t.Fatalf("noise-only npm output = %q, want empty", got)
	}
	if got := compressByCargoBuild([]string{"Compiling crate", "Finished"}); got != "" {
		t.Fatalf("noise-only cargo output = %q, want empty", got)
	}
}
