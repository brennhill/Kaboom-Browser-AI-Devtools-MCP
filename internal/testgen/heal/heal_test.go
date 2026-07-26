// Purpose: Tests for self-healing test selector repair.
// Docs: docs/features/feature/test-generation/index.md

// heal_test.go — Tests for test healing functions.
package heal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTestFilePath(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	if err := ValidateTestFilePath("", projectDir); err == nil {
		t.Fatal("should reject empty path")
	}
	if err := ValidateTestFilePath("../outside.spec.ts", projectDir); err == nil {
		t.Fatal("should reject traversal path")
	}

	inside := filepath.Join(projectDir, "tests", "flow.spec.ts")
	if err := ValidateTestFilePath(inside, projectDir); err != nil {
		t.Fatalf("abs inside error = %v", err)
	}
	if err := ValidateTestFilePath("tests/flow.spec.ts", projectDir); err != nil {
		t.Fatalf("rel inside error = %v", err)
	}
}

func TestValidateSelector(t *testing.T) {
	t.Parallel()

	if err := ValidateSelector(""); err == nil {
		t.Fatal("should reject empty selector")
	}
	if err := ValidateSelector(strings.Repeat("a", 1001)); err == nil {
		t.Fatal("should reject >1000 chars")
	}

	for _, dangerous := range []string{
		"javascript:alert(1)",
		"<script>alert(1)</script>",
		"img[onerror=alert(1)]",
		"img[onload=run()]",
	} {
		if err := ValidateSelector(dangerous); err == nil {
			t.Fatalf("should reject dangerous pattern %q", dangerous)
		}
	}

	if err := ValidateSelector("1bad"); err == nil {
		t.Fatal("should reject selectors with invalid starting char")
	}

	for _, valid := range []string{"#id", ".class", "[role='button']", "*", "button"} {
		if err := ValidateSelector(valid); err != nil {
			t.Fatalf("ValidateSelector(%q) unexpected error: %v", valid, err)
		}
	}
}

func TestExtractSelectorsFromTestFile(t *testing.T) {
	t.Parallel()

	content := `
		await page.getByTestId('submit-btn').click();
		await page.locator("#email").fill("user@example.com");
		await page.getByRole("button").click();
		await page.getByText("Save").click();
		await document.querySelector("#root");
		await document.querySelectorAll(".item");
		await page.locator("#email").click(); // duplicate
	`
	selectors := ExtractSelectorsFromTestFile(content)
	if len(selectors) != 6 {
		t.Fatalf("len = %d, want 6; got %+v", len(selectors), selectors)
	}
}

func TestAnalyzeTestFile(t *testing.T) {
	projectDir := t.TempDir()

	missingReq := TestHealRequest{Action: "analyze", TestFile: "tests/missing.spec.ts"}
	if _, err := AnalyzeTestFile(missingReq, projectDir); err == nil || !strings.Contains(err.Error(), ErrTestFileNotFound) {
		t.Fatalf("missing error = %v, want %q", err, ErrTestFileNotFound)
	}

	testPath := filepath.Join(projectDir, "tests", "login.spec.ts")
	if err := os.MkdirAll(filepath.Dir(testPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	content := `await page.locator("#login").click(); await page.getByRole("button").click();`
	if err := os.WriteFile(testPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	selectors, err := AnalyzeTestFile(TestHealRequest{Action: "analyze", TestFile: testPath}, projectDir)
	if err != nil {
		t.Fatalf("valid file error = %v", err)
	}
	if len(selectors) != 2 {
		t.Fatalf("selectors len = %d, want 2; got %+v", len(selectors), selectors)
	}
}

func TestHealSelectorAndRepairSelectors(t *testing.T) {
	t.Parallel()

	idHealed, err := HealSelector("#login")
	if err != nil {
		t.Fatalf("HealSelector(#login) error = %v", err)
	}
	if idHealed.NewSelector != "[data-testid='login']" || idHealed.Strategy != "testid_match" {
		t.Fatalf("unexpected id healing result: %+v", idHealed)
	}

	classHealed, err := HealSelector(".button")
	if err != nil {
		t.Fatalf("HealSelector(.button) error = %v", err)
	}
	if classHealed.Strategy != "structural_match" || classHealed.Confidence != 0.3 {
		t.Fatalf("unexpected class healing result: %+v", classHealed)
	}

	if _, err := HealSelector("xpath=//div"); err == nil {
		t.Fatal("HealSelector should fail for unsupported selector style")
	}

	result, err := RepairSelectors(TestHealRequest{
		BrokenSelectors: []string{"#login", ".button", "", "javascript:alert(1)"},
		AutoApply:       true,
	})
	if err != nil {
		t.Fatalf("RepairSelectors error = %v", err)
	}
	if len(result.Healed) != 2 || len(result.Unhealed) != 2 {
		t.Fatalf("unexpected healed/unhealed: %+v", result)
	}
	if result.Summary.TotalBroken != 4 {
		t.Fatalf("TotalBroken = %d, want 4", result.Summary.TotalBroken)
	}
	if result.Summary.HealedAuto != 0 {
		t.Fatalf("HealedAuto = %d, want 0", result.Summary.HealedAuto)
	}
	if result.Summary.HealedManual != 1 {
		t.Fatalf("HealedManual = %d, want 1", result.Summary.HealedManual)
	}
}

func TestFindTestFiles(t *testing.T) {
	root := t.TempDir()

	paths := []string{
		filepath.Join(root, "a.spec.ts"),
		filepath.Join(root, "b.test.js"),
		filepath.Join(root, "c.ts"),
		filepath.Join(root, "node_modules", "ignored.spec.ts"),
		filepath.Join(root, ".git", "ignored.test.ts"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", p, err)
		}
		if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", p, err)
		}
	}

	files, err := FindTestFiles(root)
	if err != nil {
		t.Fatalf("FindTestFiles error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len = %d, want 2; files=%+v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f, "node_modules") || strings.Contains(f, ".git") {
			t.Fatalf("should skip ignored dirs; got %q", f)
		}
	}
}

func TestHealTestBatch(t *testing.T) {
	projectDir := t.TempDir()

	notDirPath := filepath.Join(projectDir, "single-file.spec.ts")
	if err := os.WriteFile(notDirPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if _, err := HealTestBatch(TestHealRequest{Action: "batch", TestDir: notDirPath}, projectDir); err == nil {
		t.Fatal("should fail when test_dir is not a directory")
	}

	testDir := filepath.Join(projectDir, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	good := filepath.Join(testDir, "good.spec.ts")
	goodContent := `await page.locator("#login").click(); await page.locator(".btn").click();`
	if err := os.WriteFile(good, []byte(goodContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	huge := filepath.Join(testDir, "huge.spec.ts")
	if err := os.WriteFile(huge, []byte(strings.Repeat("x", MaxFileSizeBytes+1)), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	batchResult, err := HealTestBatch(TestHealRequest{Action: "batch", TestDir: testDir}, projectDir)
	if err != nil {
		t.Fatalf("valid error = %v", err)
	}
	if batchResult.FilesProcessed != 1 {
		t.Fatalf("FilesProcessed = %d, want 1", batchResult.FilesProcessed)
	}
	if batchResult.FilesSkipped != 1 {
		t.Fatalf("FilesSkipped = %d, want 1", batchResult.FilesSkipped)
	}
	if batchResult.TotalSelectors != 2 {
		t.Fatalf("TotalSelectors = %d, want 2", batchResult.TotalSelectors)
	}
}

func TestFormatHealSummary(t *testing.T) {
	t.Parallel()

	t.Run("analyze", func(t *testing.T) {
		summary := FormatHealSummary(TestHealRequest{Action: "analyze", TestFile: "test.spec.ts"}, map[string]any{"count": 2})
		if !strings.Contains(summary, "Found 2 selectors") {
			t.Fatalf("summary = %q", summary)
		}
	})

	t.Run("repair", func(t *testing.T) {
		summary := FormatHealSummary(TestHealRequest{Action: "repair"}, &HealResult{
			Healed: []HealedSelector{{OldSelector: "#a", NewSelector: "#b"}},
			Summary: HealSummary{TotalBroken: 3, HealedAuto: 1, Unhealed: 2},
		})
		if !strings.Contains(summary, "Healed 1/3") {
			t.Fatalf("summary = %q", summary)
		}
	})

	t.Run("batch", func(t *testing.T) {
		summary := FormatHealSummary(TestHealRequest{Action: "batch"}, &BatchHealResult{
			FilesProcessed: 5, FilesSkipped: 2, TotalSelectors: 20, TotalHealed: 15, TotalUnhealed: 5,
		})
		if !strings.Contains(summary, "Healed 15/20 selectors") {
			t.Fatalf("summary = %q", summary)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if summary := FormatHealSummary(TestHealRequest{Action: "unknown"}, nil); summary != "" {
			t.Fatalf("summary = %q, want empty", summary)
		}
	})
}
