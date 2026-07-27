// Purpose: Tests for test-generation self-healing repair.
// Docs: docs/features/feature/test-generation/index.md

// heal_test.go — Tests for heal.go functions.
// Covers: mapAnalyzeError, handleHealRepair, handleHealBatch, mapBatchError, heal.FormatHealSummary.
package testgenhandler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen/heal"
)

// ============================================
// Tests for mapAnalyzeError
// ============================================

func TestMapAnalyzeError_FileNotFound(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{TestFile: "tests/missing.spec.ts"}
	err := fmt.Errorf("%s: %s", heal.ErrTestFileNotFound, "tests/missing.spec.ts")

	resp := mapAnalyzeError(req, params, err)

	if resp.JSONRPC != "2.0" {
		t.Fatalf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Fatalf("ID = %v, want 1", resp.ID)
	}
	assertResultContains(t, resp.Result, heal.ErrTestFileNotFound)
	assertResultContains(t, resp.Result, "tests/missing.spec.ts")
}

func TestMapAnalyzeError_PathNotAllowed(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: "req-42"}
	params := heal.TestHealRequest{TestFile: "../etc/passwd"}
	err := fmt.Errorf("%s: path contains '..'", mcp.ErrPathNotAllowed)

	resp := mapAnalyzeError(req, params, err)

	if resp.ID != "req-42" {
		t.Fatalf("ID = %v, want req-42", resp.ID)
	}
	assertResultContains(t, resp.Result, mcp.ErrPathNotAllowed)
	assertResultContains(t, resp.Result, "within the project directory")
}

func TestMapAnalyzeError_GenericError(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 99}
	params := heal.TestHealRequest{TestFile: "test.spec.ts"}
	err := errors.New("permission denied")

	resp := mapAnalyzeError(req, params, err)

	assertResultContains(t, resp.Result, mcp.ErrInternal)
	assertResultContains(t, resp.Result, "permission denied")
	assertResultContains(t, resp.Result, "Failed to analyze test file")
}

func TestMapAnalyzeError_PreservesRequestID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		id   any
	}{
		{"integer ID", 42},
		{"string ID", "abc-123"},
		{"nil ID", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: tc.id}
			resp := mapAnalyzeError(req, heal.TestHealRequest{}, errors.New("some error"))
			if resp.ID != tc.id {
				t.Fatalf("ID = %v, want %v", resp.ID, tc.id)
			}
		})
	}
}

// ============================================
// Tests for handleHealRepair
// ============================================

func TestHandleHealRepair_NoBrokenSelectors(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "repair", BrokenSelectors: nil}

	_, resp, isErr := h.handleHealRepair(req, params, t.TempDir())
	if !isErr {
		t.Fatal("handleHealRepair should return error when broken_selectors is empty")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
	assertResultContains(t, resp.Result, "broken_selectors")
}

func TestHandleHealRepair_EmptyBrokenSelectors(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "repair", BrokenSelectors: []string{}}

	_, resp, isErr := h.handleHealRepair(req, params, t.TempDir())
	if !isErr {
		t.Fatal("handleHealRepair should return error when broken_selectors is empty")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
}

func TestHandleHealRepair_ValidSelectors(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{
		Action:          "repair",
		BrokenSelectors: []string{"#login", ".button"},
	}

	result, _, isErr := h.handleHealRepair(req, params, t.TempDir())
	if isErr {
		t.Fatal("handleHealRepair should mcp.Succeed with valid selectors")
	}

	healResult, ok := result.(*heal.HealResult)
	if !ok {
		t.Fatalf("result type = %T, want *heal.HealResult", result)
	}
	if len(healResult.Healed) != 2 {
		t.Fatalf("Healed len = %d, want 2", len(healResult.Healed))
	}
	if healResult.Summary.TotalBroken != 2 {
		t.Fatalf("TotalBroken = %d, want 2", healResult.Summary.TotalBroken)
	}
}

func TestHandleHealRepair_MixedSelectors(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{
		Action:          "repair",
		BrokenSelectors: []string{"#valid", "", "javascript:alert(1)", ".class"},
		AutoApply:       true,
	}

	result, _, isErr := h.handleHealRepair(req, params, t.TempDir())
	if isErr {
		t.Fatal("handleHealRepair should mcp.Succeed even with some invalid selectors")
	}

	healResult := result.(*heal.HealResult)
	if len(healResult.Healed) != 2 {
		t.Fatalf("Healed len = %d, want 2 (#valid and .class)", len(healResult.Healed))
	}
	if len(healResult.Unhealed) != 2 {
		t.Fatalf("Unhealed len = %d, want 2 (empty and dangerous)", len(healResult.Unhealed))
	}
	if healResult.Summary.TotalBroken != 4 {
		t.Fatalf("TotalBroken = %d, want 4", healResult.Summary.TotalBroken)
	}
}

func TestHandleHealRepair_PreservesRequestID(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: "heal-req-42"}
	params := heal.TestHealRequest{Action: "repair", BrokenSelectors: nil}

	_, resp, isErr := h.handleHealRepair(req, params, t.TempDir())
	if !isErr {
		t.Fatal("expected error for nil selectors")
	}
	if resp.ID != "heal-req-42" {
		t.Fatalf("error response ID = %v, want heal-req-42", resp.ID)
	}
}

// ============================================
// Tests for handleHealBatch
// ============================================

func TestHandleHealBatch_NoTestDir(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "batch", TestDir: ""}

	_, resp, isErr := h.handleHealBatch(req, params, t.TempDir())
	if !isErr {
		t.Fatal("handleHealBatch should return error when test_dir is empty")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
	assertResultContains(t, resp.Result, "test_dir")
}

func TestHandleHealBatch_ValidDir(t *testing.T) {
	projectDir := t.TempDir()
	h := newPureHandler()

	testDir := filepath.Join(projectDir, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	testFile := filepath.Join(testDir, "login.spec.ts")
	content := `await page.locator("#login").click();`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "batch", TestDir: testDir}

	result, _, isErr := h.handleHealBatch(req, params, projectDir)
	if isErr {
		t.Fatal("handleHealBatch should mcp.Succeed with valid directory")
	}

	batchResult, ok := result.(*heal.BatchHealResult)
	if !ok {
		t.Fatalf("result type = %T, want *heal.BatchHealResult", result)
	}
	if batchResult.FilesProcessed != 1 {
		t.Fatalf("FilesProcessed = %d, want 1", batchResult.FilesProcessed)
	}
	if batchResult.TotalSelectors != 1 {
		t.Fatalf("TotalSelectors = %d, want 1", batchResult.TotalSelectors)
	}
}

func TestHandleHealBatch_NonexistentDir(t *testing.T) {
	projectDir := t.TempDir()
	h := newPureHandler()

	missingDir := filepath.Join(projectDir, "nonexistent-dir")
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "batch", TestDir: missingDir}

	_, resp, isErr := h.handleHealBatch(req, params, projectDir)
	if !isErr {
		t.Fatal("handleHealBatch should mcp.Fail for nonexistent directory")
	}
	assertResultContains(t, resp.Result, heal.ErrTestFileNotFound)
}

func TestHandleHealBatch_PathTraversal(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "batch", TestDir: "../../../etc"}

	_, resp, isErr := h.handleHealBatch(req, params, t.TempDir())
	if !isErr {
		t.Fatal("handleHealBatch should mcp.Fail for path traversal")
	}
	assertResultContains(t, resp.Result, mcp.ErrPathNotAllowed)
}

func TestHandleHealBatch_PreservesRequestID(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: "batch-99"}
	params := heal.TestHealRequest{Action: "batch", TestDir: ""}

	_, resp, isErr := h.handleHealBatch(req, params, t.TempDir())
	if !isErr {
		t.Fatal("expected error for empty test_dir")
	}
	if resp.ID != "batch-99" {
		t.Fatalf("error response ID = %v, want batch-99", resp.ID)
	}
}

func TestHandleHealBatch_EmptyDir(t *testing.T) {
	projectDir := t.TempDir()
	h := newPureHandler()

	testDir := filepath.Join(projectDir, "empty-tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	params := heal.TestHealRequest{Action: "batch", TestDir: testDir}

	result, _, isErr := h.handleHealBatch(req, params, projectDir)
	if isErr {
		t.Fatal("handleHealBatch should mcp.Succeed on empty directory (zero files)")
	}

	batchResult := result.(*heal.BatchHealResult)
	if batchResult.FilesProcessed != 0 {
		t.Fatalf("FilesProcessed = %d, want 0", batchResult.FilesProcessed)
	}
	if batchResult.TotalSelectors != 0 {
		t.Fatalf("TotalSelectors = %d, want 0", batchResult.TotalSelectors)
	}
}

// ============================================
// Tests for mapBatchError
// ============================================

func TestMapBatchError_PathNotAllowed(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	err := fmt.Errorf("%s: path contains '..'", mcp.ErrPathNotAllowed)

	resp := mapBatchError(req, err)

	assertResultContains(t, resp.Result, mcp.ErrPathNotAllowed)
	assertResultContains(t, resp.Result, "within the project directory")
}

func TestMapBatchError_BatchTooLarge(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	err := fmt.Errorf("%s: too many files", testgen.ErrBatchTooLarge)

	resp := mapBatchError(req, err)

	assertResultContains(t, resp.Result, testgen.ErrBatchTooLarge)
	assertResultContains(t, resp.Result, "Reduce the number or size")
}

func TestMapBatchError_GenericError(t *testing.T) {
	t.Parallel()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	err := errors.New("unexpected filesystem error")

	resp := mapBatchError(req, err)

	assertResultContains(t, resp.Result, mcp.ErrInternal)
	assertResultContains(t, resp.Result, "unexpected filesystem error")
	assertResultContains(t, resp.Result, "Failed to heal test batch")
}

func TestMapBatchError_PreservesRequestID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		id   any
	}{
		{"integer ID", 77},
		{"string ID", "batch-xyz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: tc.id}
			resp := mapBatchError(req, errors.New("some error"))
			if resp.ID != tc.id {
				t.Fatalf("ID = %v, want %v", resp.ID, tc.id)
			}
			if resp.JSONRPC != "2.0" {
				t.Fatalf("JSONRPC = %q, want 2.0", resp.JSONRPC)
			}
		})
	}
}

// ============================================
// Tests for heal.FormatHealSummary
// ============================================

func TestFormatHealSummary_Analyze(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "analyze", TestFile: "tests/login.spec.ts"}
	result := map[string]any{
		"broken_selectors": []string{"#btn", ".submit"},
		"count":            2,
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Found 2 selectors") {
		t.Fatalf("summary = %q, want to contain 'Found 2 selectors'", summary)
	}
	if !strings.Contains(summary, "tests/login.spec.ts") {
		t.Fatalf("summary = %q, want to contain file path", summary)
	}
}

func TestFormatHealSummary_AnalyzeZeroCount(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "analyze", TestFile: "tests/clean.spec.ts"}
	result := map[string]any{
		"count": 0,
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Found 0 selectors") {
		t.Fatalf("summary = %q, want to contain 'Found 0 selectors'", summary)
	}
}

func TestFormatHealSummary_AnalyzeNonMapResult(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "analyze", TestFile: "test.spec.ts"}
	// When result is not a map, count defaults to 0
	summary := heal.FormatHealSummary(params, "not a map")

	if !strings.Contains(summary, "Found 0 selectors") {
		t.Fatalf("summary = %q, want 'Found 0 selectors' for non-map result", summary)
	}
}

func TestFormatHealSummary_Repair(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "repair"}
	result := &heal.HealResult{
		Healed:   []heal.HealedSelector{{OldSelector: "#a", NewSelector: "#b"}},
		Unhealed: []string{".broken"},
		Summary: heal.HealSummary{
			TotalBroken:  3,
			HealedAuto:   1,
			HealedManual: 0,
			Unhealed:     2,
		},
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Healed 1/3") {
		t.Fatalf("summary = %q, want to contain 'Healed 1/3'", summary)
	}
	if !strings.Contains(summary, "2 unhealed") {
		t.Fatalf("summary = %q, want to contain '2 unhealed'", summary)
	}
	if !strings.Contains(summary, "1 auto-applied") {
		t.Fatalf("summary = %q, want to contain '1 auto-applied'", summary)
	}
}

func TestFormatHealSummary_RepairAllHealed(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "repair"}
	result := &heal.HealResult{
		Healed:   []heal.HealedSelector{{}, {}, {}},
		Unhealed: []string{},
		Summary: heal.HealSummary{
			TotalBroken:  3,
			HealedAuto:   3,
			HealedManual: 0,
			Unhealed:     0,
		},
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Healed 3/3") {
		t.Fatalf("summary = %q, want 'Healed 3/3'", summary)
	}
	if !strings.Contains(summary, "0 unhealed") {
		t.Fatalf("summary = %q, want '0 unhealed'", summary)
	}
}

func TestFormatHealSummary_Batch(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "batch"}
	result := &heal.BatchHealResult{
		FilesProcessed: 5,
		FilesSkipped:   2,
		TotalSelectors: 20,
		TotalHealed:    15,
		TotalUnhealed:  5,
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Healed 15/20 selectors") {
		t.Fatalf("summary = %q, want to contain 'Healed 15/20 selectors'", summary)
	}
	if !strings.Contains(summary, "across 5 files") {
		t.Fatalf("summary = %q, want to contain 'across 5 files'", summary)
	}
	if !strings.Contains(summary, "2 files skipped") {
		t.Fatalf("summary = %q, want to contain '2 files skipped'", summary)
	}
	if !strings.Contains(summary, "5 selectors unhealed") {
		t.Fatalf("summary = %q, want to contain '5 selectors unhealed'", summary)
	}
}

func TestFormatHealSummary_BatchEmpty(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "batch"}
	result := &heal.BatchHealResult{
		FilesProcessed: 0,
		FilesSkipped:   0,
		TotalSelectors: 0,
		TotalHealed:    0,
		TotalUnhealed:  0,
	}

	summary := heal.FormatHealSummary(params, result)

	if !strings.Contains(summary, "Healed 0/0 selectors") {
		t.Fatalf("summary = %q, want 'Healed 0/0 selectors'", summary)
	}
	if !strings.Contains(summary, "across 0 files") {
		t.Fatalf("summary = %q, want 'across 0 files'", summary)
	}
}

func TestFormatHealSummary_UnknownAction(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: "unknown"}
	summary := heal.FormatHealSummary(params, nil)

	if summary != "" {
		t.Fatalf("heal.FormatHealSummary(unknown) = %q, want empty string", summary)
	}
}

func TestFormatHealSummary_EmptyAction(t *testing.T) {
	t.Parallel()

	params := heal.TestHealRequest{Action: ""}
	summary := heal.FormatHealSummary(params, nil)

	if summary != "" {
		t.Fatalf("heal.FormatHealSummary(empty) = %q, want empty string", summary)
	}
}
