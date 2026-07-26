// Purpose: Tests for test-generation classify dispatch routing.
// Docs: docs/features/feature/test-generation/index.md

// testgen_classify_dispatch_test.go — Tests for classify dispatch functions at 0% coverage.
// Covers: dispatchClassifyAction, classifySingleFailure, classifyBatchFailures.
package testgenhandler

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Tests for dispatchClassifyAction
// ============================================

func TestDispatchClassifyAction_Failure(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	params := TestClassifyRequest{
		Action: "failure",
		Failure: &TestFailure{
			TestName: "login test",
			Error:    `Timeout waiting for selector "#login-btn"`,
		},
	}

	result, summary, errResp := h.dispatchClassifyAction(1, params)
	if errResp != nil {
		t.Fatalf("dispatchClassifyAction(failure) returned error response: %v", errResp)
	}
	if result == nil {
		t.Fatal("dispatchClassifyAction(failure) returned nil result")
	}
	if summary == "" {
		t.Fatal("dispatchClassifyAction(failure) returned empty summary")
	}
	if !strings.Contains(summary, "selector_broken") {
		t.Fatalf("summary = %q, want to contain selector_broken", summary)
	}

	// Verify result is a map with classification
	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if _, ok := data["classification"]; !ok {
		t.Fatal("result missing 'classification' key")
	}
}

func TestDispatchClassifyAction_Batch(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	params := TestClassifyRequest{
		Action: "batch",
		Failures: []TestFailure{
			{TestName: "t1", Error: "net::ERR_CONNECTION_REFUSED"},
			{TestName: "t2", Error: `Expected "a" to be "b"`},
		},
	}

	result, summary, errResp := h.dispatchClassifyAction(1, params)
	if errResp != nil {
		t.Fatalf("dispatchClassifyAction(batch) returned error response: %v", errResp)
	}
	if result == nil {
		t.Fatal("dispatchClassifyAction(batch) returned nil result")
	}
	if !strings.Contains(summary, "Classified 2 failures") {
		t.Fatalf("summary = %q, want to contain 'Classified 2 failures'", summary)
	}

	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if _, ok := data["batch_result"]; !ok {
		t.Fatal("result missing 'batch_result' key")
	}
}

func TestDispatchClassifyAction_UnknownAction(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	params := TestClassifyRequest{
		Action: "nonexistent",
	}

	result, summary, errResp := h.dispatchClassifyAction(1, params)
	// Unknown action falls through without error
	if errResp != nil {
		t.Fatalf("unknown action should not return error; got %v", errResp)
	}
	if result != nil {
		t.Fatalf("unknown action should return nil result; got %v", result)
	}
	if summary != "" {
		t.Fatalf("unknown action should return empty summary; got %q", summary)
	}
}

func TestDispatchClassifyAction_FailureMissingParam(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	params := TestClassifyRequest{
		Action:  "failure",
		Failure: nil, // missing
	}

	_, _, errResp := h.dispatchClassifyAction(42, params)
	if errResp == nil {
		t.Fatal("dispatchClassifyAction(failure, nil) should return error response")
	}
	if errResp.ID != 42 {
		t.Fatalf("error response ID = %v, want 42", errResp.ID)
	}
	assertResultContains(t, errResp.Result, mcp.ErrMissingParam)
}

func TestDispatchClassifyAction_BatchMissingParam(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	params := TestClassifyRequest{
		Action:   "batch",
		Failures: nil, // empty
	}

	_, _, errResp := h.dispatchClassifyAction(99, params)
	if errResp == nil {
		t.Fatal("dispatchClassifyAction(batch, nil failures) should return error response")
	}
	assertResultContains(t, errResp.Result, mcp.ErrMissingParam)
}

func TestDispatchClassifyAction_BatchTooLarge(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	failures := make([]TestFailure, maxFailuresPerBatch+1)
	for i := range failures {
		failures[i] = TestFailure{
			TestName: fmt.Sprintf("test-%d", i),
			Error:    "some error",
		}
	}

	params := TestClassifyRequest{
		Action:   "batch",
		Failures: failures,
	}

	_, _, errResp := h.dispatchClassifyAction(1, params)
	if errResp == nil {
		t.Fatal("dispatchClassifyAction should reject batch > maxFailuresPerBatch")
	}
	assertResultContains(t, errResp.Result, ErrBatchTooLarge)
}

// ============================================
// Tests for classifySingleFailure
// ============================================

func TestClassifySingleFailure_NilFailure(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	_, _, resp, ok := h.classifySingleFailure(1, TestClassifyRequest{
		Action:  "failure",
		Failure: nil,
	})
	if ok {
		t.Fatal("classifySingleFailure(nil) should return ok=false")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
	assertResultContains(t, resp.Result, "failure")
}

func TestClassifySingleFailure_HighConfidence(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	result, summary, _, ok := h.classifySingleFailure(1, TestClassifyRequest{
		Action: "failure",
		Failure: &TestFailure{
			TestName:   "broken selector test",
			Error:      `Timeout waiting for selector "#submit-btn"`,
			DurationMs: 5000,
		},
	})
	if !ok {
		t.Fatal("classifySingleFailure should return ok=true for high confidence")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
	if !strings.Contains(summary, "selector_broken") {
		t.Fatalf("summary = %q, want to contain selector_broken", summary)
	}
	if !strings.Contains(summary, "90%") {
		t.Fatalf("summary = %q, want to contain 90%%", summary)
	}
	if !strings.Contains(summary, "heal") {
		t.Fatalf("summary = %q, want to contain recommended action 'heal'", summary)
	}

	// Verify result structure
	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if _, exists := data["classification"]; !exists {
		t.Fatal("result missing 'classification'")
	}
	if _, exists := data["suggested_fix"]; !exists {
		t.Fatal("result missing 'suggested_fix' for selector_broken category")
	}
}

func TestClassifySingleFailure_LowConfidence(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	_, _, resp, ok := h.classifySingleFailure(7, TestClassifyRequest{
		Action: "failure",
		Failure: &TestFailure{
			TestName: "unknown failure",
			Error:    "random gibberish error nobody knows about",
		},
	})
	if ok {
		t.Fatal("classifySingleFailure should return ok=false for low confidence (< 0.5)")
	}
	assertResultContains(t, resp.Result, ErrClassificationUncertain)
	if resp.ID != 7 {
		t.Fatalf("error response ID = %v, want 7", resp.ID)
	}
}

func TestClassifySingleFailure_NoSuggestedFix(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	// Real bug category has no suggested fix
	result, _, _, ok := h.classifySingleFailure(1, TestClassifyRequest{
		Action: "failure",
		Failure: &TestFailure{
			TestName: "assertion test",
			Error:    `Expected "hello" to be "world"`,
		},
	})
	if !ok {
		t.Fatal("classifySingleFailure should return ok=true for real bug (confidence >= 0.5)")
	}

	data, _ := result.(map[string]any)
	if _, exists := data["suggested_fix"]; exists {
		t.Fatal("real_bug classification should not have suggested_fix in result")
	}
}

// ============================================
// Tests for classifyBatchFailures
// ============================================

func TestClassifyBatchFailures_EmptyFailures(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	_, _, resp, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action:   "batch",
		Failures: nil,
	})
	if ok {
		t.Fatal("classifyBatchFailures(nil) should return ok=false")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
}

func TestClassifyBatchFailures_EmptySlice(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	_, _, resp, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action:   "batch",
		Failures: []TestFailure{},
	})
	if ok {
		t.Fatal("classifyBatchFailures([]) should return ok=false")
	}
	assertResultContains(t, resp.Result, mcp.ErrMissingParam)
}

func TestClassifyBatchFailures_ExceedsMaxBatch(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	failures := make([]TestFailure, maxFailuresPerBatch+1)
	for i := range failures {
		failures[i] = TestFailure{TestName: fmt.Sprintf("t%d", i), Error: "error"}
	}

	_, _, resp, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action:   "batch",
		Failures: failures,
	})
	if ok {
		t.Fatal("classifyBatchFailures should reject batch exceeding limit")
	}
	assertResultContains(t, resp.Result, ErrBatchTooLarge)
	assertResultContains(t, resp.Result, fmt.Sprintf("%d", maxFailuresPerBatch+1))
}

func TestClassifyBatchFailures_ExactMax(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	failures := make([]TestFailure, maxFailuresPerBatch)
	for i := range failures {
		failures[i] = TestFailure{TestName: fmt.Sprintf("t%d", i), Error: "net::ERR_CONNECTION_REFUSED"}
	}

	result, summary, _, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action:   "batch",
		Failures: failures,
	})
	if !ok {
		t.Fatal("classifyBatchFailures should accept exactly maxFailuresPerBatch failures")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(summary, fmt.Sprintf("Classified %d failures", maxFailuresPerBatch)) {
		t.Fatalf("summary = %q, want to contain 'Classified %d failures'", summary, maxFailuresPerBatch)
	}
}

func TestClassifyBatchFailures_MixedResults(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	failures := []TestFailure{
		{TestName: "t1", Error: `Timeout waiting for selector "#btn"`},
		{TestName: "t2", Error: "net::ERR_CONNECTION_REFUSED"},
		{TestName: "t3", Error: `Expected "a" to be "b"`},
		{TestName: "t4", Error: "Element is outside viewport"},
	}

	result, summary, _, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action:   "batch",
		Failures: failures,
	})
	if !ok {
		t.Fatal("classifyBatchFailures should mcp.Succeed")
	}

	data, _ := result.(map[string]any)
	batchResult, _ := data["batch_result"].(*BatchClassifyResult)
	if batchResult == nil {
		t.Fatal("batch_result should not be nil")
	}
	if batchResult.TotalClassified != 4 {
		t.Fatalf("TotalClassified = %d, want 4", batchResult.TotalClassified)
	}
	if batchResult.RealBugs != 1 {
		t.Fatalf("RealBugs = %d, want 1", batchResult.RealBugs)
	}
	if batchResult.FlakyTests != 1 {
		t.Fatalf("FlakyTests = %d, want 1", batchResult.FlakyTests)
	}
	if batchResult.TestBugs != 1 {
		t.Fatalf("TestBugs = %d, want 1", batchResult.TestBugs)
	}
	if len(batchResult.Classifications) != 4 {
		t.Fatalf("Classifications len = %d, want 4", len(batchResult.Classifications))
	}

	// Check summary contains all counts
	if !strings.Contains(summary, "1 real bugs") {
		t.Fatalf("summary missing real bugs count; got %q", summary)
	}
	if !strings.Contains(summary, "1 flaky") {
		t.Fatalf("summary missing flaky count; got %q", summary)
	}
}

func TestClassifyBatchFailures_SingleFailure(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	result, summary, _, ok := h.classifyBatchFailures(1, TestClassifyRequest{
		Action: "batch",
		Failures: []TestFailure{
			{TestName: "only", Error: "net::ERR_TIMEOUT"},
		},
	})
	if !ok {
		t.Fatal("should mcp.Succeed with single failure")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(summary, "Classified 1 failures") {
		t.Fatalf("summary = %q, want 'Classified 1 failures'", summary)
	}
}

func TestClassifyBatchFailures_PreservesRequestID(t *testing.T) {
	t.Parallel()
	h := newPureHandler()

	// Test with nil failures to trigger error response with specific ID
	_, _, resp, ok := h.classifyBatchFailures("req-abc-123", TestClassifyRequest{
		Action:   "batch",
		Failures: nil,
	})
	if ok {
		t.Fatal("should mcp.Fail with nil failures")
	}
	if resp.ID != "req-abc-123" {
		t.Fatalf("error response ID = %v, want req-abc-123", resp.ID)
	}
}

// ============================================
// Test helpers
// ============================================

// assertResultContains checks that a JSON-encoded result contains the expected substring.
func assertResultContains(t *testing.T, result json.RawMessage, want string) {
	t.Helper()
	if !strings.Contains(string(result), want) {
		t.Fatalf("result does not contain %q;\nresult: %s", want, string(result))
	}
}
