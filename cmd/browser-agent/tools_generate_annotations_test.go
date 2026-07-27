// Purpose: Tests for generate annotation report output.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_annotations_test.go — Tests for annotation-driven generate formats.
// Covers: visual_test, annotation_report, annotation_issues.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate/annotations"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test Helpers
// ============================================

func seedAnnotationSession(t *testing.T, h *ToolHandler) {
	t.Helper()
	// Fresh store to avoid global state pollution from other tests
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	t.Cleanup(func() { h.annotationStore.Close() })
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:             "ann_1",
				Text:           "make this button darker",
				ElementSummary: "button.btn-primary 'Submit'",
				CorrelationID:  "detail_1",
				Rect:           annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
				PageURL:        "https://example.com/checkout",
			},
			{
				ID:             "ann_2",
				Text:           "font too small here",
				ElementSummary: "p.description 'Product details...'",
				CorrelationID:  "detail_2",
				Rect:           annotation.Rect{X: 50, Y: 400, Width: 300, Height: 100},
				PageURL:        "https://example.com/checkout",
			},
		},
		ScreenshotPath: "/tmp/kaboom/draw_test_annotated.png",
		PageURL:        "https://example.com/checkout",
		TabID:          1,
		Timestamp:      1707580800000,
	}
	h.annotationStore.StoreSession(1, session)

	// Store details with a11y flags
	h.annotationStore.StoreDetail("detail_1", annotation.Detail{
		CorrelationID:  "detail_1",
		Selector:       "button.btn-primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"btn-primary", "rounded-lg"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)", "color": "rgb(255, 255, 255)", "font-size": "14px"},
		ParentSelector: "form.checkout-form > div.actions",
		BoundingRect:   annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
		A11yFlags:      []string{},
	})
	h.annotationStore.StoreDetail("detail_2", annotation.Detail{
		CorrelationID:  "detail_2",
		Selector:       "p.description",
		Tag:            "p",
		TextContent:    "Product details...",
		Classes:        []string{"description", "text-sm"},
		ComputedStyles: map[string]string{"font-size": "12px", "color": "rgb(100, 100, 100)"},
		ParentSelector: "div.product-info",
		BoundingRect:   annotation.Rect{X: 50, Y: 400, Width: 300, Height: 100},
		A11yFlags:      []string{"low-contrast"},
	})
}

// ============================================
// visual_test format tests
// ============================================

func TestGenerate_VisualTest_NoAnnotations(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(strings.ToLower(text), "no annotation") {
		t.Errorf("expected 'no annotation' message, got %q", text)
	}
}

func TestGenerate_VisualTest_GeneratesPlaywright(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should contain Playwright test structure
	if !strings.Contains(text, "test(") {
		t.Errorf("expected Playwright test() call, got %q", text)
	}
	if !strings.Contains(text, "page.goto") {
		t.Errorf("expected page.goto() call, got %q", text)
	}
	if !strings.Contains(text, "resolveAnnotationLocator") {
		t.Errorf("expected resilient locator helper in generated test, got %q", text)
	}
	if !strings.Contains(text, "example.com/checkout") {
		t.Errorf("expected page URL in test, got %q", text)
	}
}

func TestGenerate_VisualTest_UsesSelectorCandidates(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	h.annotationStore.StoreDetail("detail_1", annotation.Detail{
		CorrelationID:      "detail_1",
		Selector:           "button.btn-primary",
		Tag:                "button",
		TextContent:        "Submit",
		Classes:            []string{"btn-primary"},
		ID:                 "submit-btn",
		ComputedStyles:     map[string]string{"background-color": "rgb(59, 130, 246)"},
		SelectorCandidates: []string{"testid=checkout-submit", "role=button|Submit", "css=button.btn-primary"},
		JSFramework:        "react",
		ParentSelector:     "form.checkout-form > div.actions",
		BoundingRect:       annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := annotations.HandleVisualTest(h, req, nil)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "testid=checkout-submit") {
		t.Errorf("expected testid candidate in generated test, got %q", text)
	}
	if !strings.Contains(text, "role=button|Submit") {
		t.Errorf("expected role candidate in generated test, got %q", text)
	}
	if !strings.Contains(text, "getByTestId") {
		t.Errorf("expected testid strategy support in helper, got %q", text)
	}
	if !strings.Contains(text, "getByRole") {
		t.Errorf("expected role strategy support in helper, got %q", text)
	}
}

func TestGenerate_VisualTest_IncludesAnnotationComments(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// annotation.Annotation text should appear as comments in the test
	if !strings.Contains(text, "make this button darker") {
		t.Errorf("expected annotation text in generated test, got %q", text)
	}
	if !strings.Contains(text, "button.btn-primary") {
		t.Errorf("expected selector in generated test, got %q", text)
	}
}

func TestGenerate_VisualTest_CustomTestName(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test","test_name":"checkout visual review"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "checkout visual review") {
		t.Errorf("expected custom test name, got %q", text)
	}
}

func TestGenerate_VisualTest_IncludesA11yAssertions(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should include a11y flag as a comment/todo
	if !strings.Contains(text, "low-contrast") {
		t.Errorf("expected a11y flag in generated test, got %q", text)
	}
}

func TestGenerate_VisualTest_NamedSession(t *testing.T) {
	h := createTestToolHandler(t)

	// Create a named session with 2 pages
	page1 := &annotation.Session{
		Annotations: []annotation.Annotation{{ID: "ann_p1", Text: "fix header", ElementSummary: "header.main 'Logo'", CorrelationID: "d_p1", PageURL: "https://example.com/"}},
		PageURL:     "https://example.com/",
		TabID:       1,
		Timestamp:   1000,
	}
	page2 := &annotation.Session{
		Annotations: []annotation.Annotation{{ID: "ann_p2", Text: "fix footer", ElementSummary: "footer.main 'Copyright'", CorrelationID: "d_p2", PageURL: "https://example.com/about"}},
		PageURL:     "https://example.com/about",
		TabID:       2,
		Timestamp:   2000,
	}
	h.annotationStore.AppendToNamedSession("qa-review", page1)
	h.annotationStore.AppendToNamedSession("qa-review", page2)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"visual_test","annot_session":"qa-review"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should contain both page URLs
	if !strings.Contains(text, "example.com/") {
		t.Errorf("expected first page URL, got %q", text)
	}
	if !strings.Contains(text, "example.com/about") {
		t.Errorf("expected second page URL, got %q", text)
	}
}

// ============================================
// annotation_report format tests
// ============================================

func TestGenerate_AnnotationReport_NoAnnotations(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_report"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(strings.ToLower(text), "no annotation") {
		t.Errorf("expected 'no annotation' message, got %q", text)
	}
}

func TestGenerate_AnnotationReport_GeneratesMarkdown(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_report"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should be markdown with header
	if !strings.Contains(text, "# ") {
		t.Errorf("expected markdown header, got %q", text)
	}
	// Should contain annotation text
	if !strings.Contains(text, "make this button darker") {
		t.Errorf("expected annotation text in report, got %q", text)
	}
	if !strings.Contains(text, "font too small here") {
		t.Errorf("expected second annotation text in report, got %q", text)
	}
}

func TestGenerate_AnnotationReport_IncludesScreenshotRef(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_report"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "draw_test_annotated.png") {
		t.Errorf("expected screenshot reference in report, got %q", text)
	}
}

func TestGenerate_AnnotationReport_IncludesA11yFlags(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_report"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "low-contrast") {
		t.Errorf("expected a11y flag in report, got %q", text)
	}
}

// ============================================
// annotation_issues format tests
// ============================================

func TestGenerate_AnnotationIssues_NoAnnotations(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_issues"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(strings.ToLower(text), "no annotation") {
		t.Errorf("expected 'no annotation' message, got %q", text)
	}
}

func TestGenerate_AnnotationIssues_ReturnsStructuredJSON(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_issues"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should contain JSON with issues array
	if !strings.Contains(text, "issues") {
		t.Errorf("expected 'issues' key in response, got %q", text)
	}
	if !strings.Contains(text, "make this button darker") {
		t.Errorf("expected annotation text in issues, got %q", text)
	}
}

func TestGenerate_AnnotationIssues_IncludesElementInfo(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_issues"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should include element summary and selector info
	if !strings.Contains(text, "button.btn-primary") {
		t.Errorf("expected element selector in issues, got %q", text)
	}
}

func TestGenerate_AnnotationIssues_CountsCorrect(t *testing.T) {
	h := createTestToolHandler(t)
	seedAnnotationSession(t, h)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotation_issues"}`)

	resp := h.generateDispatcher.Handle(req, args)
	text := unmarshalMCPText(t, resp.Result)

	// Should mention count of 2
	if !strings.Contains(text, "2") {
		t.Errorf("expected issue count '2' in response, got %q", text)
	}
}

// ============================================
// Panic safety for new formats
// ============================================

func TestGenerate_VisualTest_ExpiredDetailFallsBackToBody(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	t.Cleanup(func() { h.annotationStore.Close() })

	// Store a session but DON'T store any detail — simulates expired detail
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:             "ann_expired",
				Text:           "fix this",
				ElementSummary: "div.foo 'bar'",
				CorrelationID:  "detail_gone",
				Rect:           annotation.Rect{X: 10, Y: 20, Width: 30, Height: 40},
				PageURL:        "https://example.com",
			},
		},
		PageURL:   "https://example.com",
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := annotations.HandleVisualTest(h, req, nil)
	text := unmarshalMCPText(t, resp.Result)

	// Should include a fallback locator path and expired comment.
	// Current behavior prefers selectors derived from element summary text.
	if !strings.Contains(text, "resolveAnnotationLocator(page") {
		t.Errorf("expected fallback locator helper when detail is expired, got %q", text)
	}
	if !strings.Contains(text, "detail expired") {
		t.Errorf("expected 'detail expired' comment, got %q", text)
	}
}

func TestGenerate_VisualTest_EscapesSingleQuotes(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	t.Cleanup(func() { h.annotationStore.Close() })

	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:             "ann_esc",
				Text:           "make this button's color darker",
				ElementSummary: "div.o'malley 'Submit'",
				CorrelationID:  "detail_esc",
				Rect:           annotation.Rect{X: 10, Y: 20, Width: 30, Height: 40},
				PageURL:        "https://example.com/it's-a-test",
			},
		},
		ScreenshotPath: "",
		PageURL:        "https://example.com/it's-a-test",
		TabID:          1,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)
	h.annotationStore.StoreDetail("detail_esc", annotation.Detail{
		Selector: "div.o'malley",
		Tag:      "div",
		ID:       "btn-it's",
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := annotations.HandleVisualTest(h, req, nil)
	text := unmarshalMCPText(t, resp.Result)

	// The generated code must NOT have unescaped single quotes inside JS string literals
	if strings.Contains(text, "goto('https://example.com/it's-a-test')") {
		t.Error("URL contains unescaped single quote — JS injection risk")
	}
	// Should contain the escaped version
	if !strings.Contains(text, `it\'s-a-test`) {
		t.Error("Expected escaped single quote in URL")
	}
	if !strings.Contains(text, `#btn-it\'s`) {
		t.Error("Expected escaped single quote in selector ID")
	}
}

func TestGenerate_AnnotationFormats_NoPanic(t *testing.T) {
	formats := []struct {
		name string
		args string
	}{
		{"visual_test", `{"what":"visual_test"}`},
		{"annotation_report", `{"what":"annotation_report"}`},
		{"annotation_issues", `{"what":"annotation_issues"}`},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			h := createTestToolHandler(t)
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("generate(%s) PANICKED: %v", tc.name, r)
				}
			}()

			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
			resp := h.generateDispatcher.Handle(req, json.RawMessage(tc.args))
			if resp.Result == nil {
				t.Errorf("generate(%s) returned nil Result", tc.name)
			}
		})
	}
}
