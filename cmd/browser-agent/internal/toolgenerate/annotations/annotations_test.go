// annotations_test.go — Unit tests for the annotation artifact generators and handlers.

package annotations

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ---------------------------------------------------------------------------
// Test fakes
// ---------------------------------------------------------------------------

type fakeAnnotationDeps struct {
	annStore *annotation.Store
}

func (f *fakeAnnotationDeps) GetAnnotationStore() *annotation.Store { return f.annStore }

func newGenDeps() *fakeAnnotationDeps {
	return &fakeAnnotationDeps{annStore: annotation.NewStore(1 * time.Hour)}
}

func genReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

// parseResult decodes an MCP tool result into (isError, text).
func parseResult(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}

// ---------------------------------------------------------------------------
// Annotation handlers (visual_test / annotation_report / annotation_issues)
// ---------------------------------------------------------------------------

func addAnonymousSession(d *fakeAnnotationDeps) {
	d.annStore.StoreSession(1, &annotation.Session{
		PageURL:        "https://example.com/page",
		ScreenshotPath: "/tmp/shot.png",
		Timestamp:      time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{
			{ID: "a1", Text: "Fix header", ElementSummary: "h1 'Welcome'", CorrelationID: "c1"},
		},
	})
}

func TestHandleVisualTest(t *testing.T) {
	t.Run("no annotations returns no_data", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleVisualTest(d, genReq(), json.RawMessage(`{}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("no_data path should not be an error response: %s", text)
		}
	})

	t.Run("with annotations generates script", func(t *testing.T) {
		d := newGenDeps()
		addAnonymousSession(d)
		resp := HandleVisualTest(d, genReq(), json.RawMessage(`{"test_name":"My Test"}`))
		isErr, text := parseResult(t, resp)
		if isErr {
			t.Fatalf("visual test should succeed: %s", text)
		}
		if len(text) == 0 {
			t.Error("expected non-empty script text")
		}
	})

	t.Run("default test name when empty", func(t *testing.T) {
		d := newGenDeps()
		addAnonymousSession(d)
		resp := HandleVisualTest(d, genReq(), nil)
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("should succeed with default test name")
		}
	})

	t.Run("named session not found", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleVisualTest(d, genReq(), json.RawMessage(`{"annot_session":"ghost"}`))
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("missing named session yields no_data success, not error")
		}
	})

	t.Run("named session found", func(t *testing.T) {
		d := newGenDeps()
		d.annStore.AppendToNamedSession("flow", &annotation.Session{
			PageURL:     "https://example.com/x",
			Annotations: []annotation.Annotation{{ID: "n1", Text: "note"}},
		})
		resp := HandleVisualTest(d, genReq(), json.RawMessage(`{"annot_session":"flow"}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("named session should generate script: %s", text)
		}
	})
}

func TestHandleAnnotationReport(t *testing.T) {
	t.Run("no data", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleAnnotationReport(d, genReq(), json.RawMessage(`{}`))
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("no data path should be success no_data")
		}
	})
	t.Run("with data", func(t *testing.T) {
		d := newGenDeps()
		addAnonymousSession(d)
		resp := HandleAnnotationReport(d, genReq(), nil)
		isErr, text := parseResult(t, resp)
		if isErr {
			t.Fatalf("report should succeed: %s", text)
		}
		if len(text) == 0 {
			t.Error("expected markdown report text")
		}
	})
}

func TestHandleAnnotationIssues(t *testing.T) {
	t.Run("no data", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleAnnotationIssues(d, genReq(), json.RawMessage(`{}`))
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("no data path should be success no_data")
		}
	})
	t.Run("with data", func(t *testing.T) {
		d := newGenDeps()
		addAnonymousSession(d)
		resp := HandleAnnotationIssues(d, genReq(), nil)
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("issues should succeed: %s", text)
		}
	})
}

// ---------------------------------------------------------------------------
// JsEscapeSingle
// ---------------------------------------------------------------------------

func TestJsEscapeSingle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"it's", `it\'s`},
		{`back\slash`, `back\\slash`},
		{"line\nbreak", `line\nbreak`},
		{"return\rchar", `return\rchar`},
		{"combo'\n\\", `combo\'\n\\`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := JsEscapeSingle(tt.input)
			if got != tt.want {
				t.Errorf("JsEscapeSingle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JsStringArray
// ---------------------------------------------------------------------------

func TestJsStringArray(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"empty", nil, "[]"},
		{"empty slice", []string{}, "[]"},
		{"single", []string{"foo"}, "['foo']"},
		{"multiple", []string{"a", "b"}, "['a', 'b']"},
		{"with quotes", []string{"it's"}, `['it\'s']`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JsStringArray(tt.values)
			if got != tt.want {
				t.Errorf("JsStringArray = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractSummaryText
// ---------------------------------------------------------------------------

func TestExtractSummaryText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"button 'Submit'", "Submit"},
		{"<div class='foo'>", "foo"},
		{"no quotes here", ""},
		{"'single'", "single"},
		{"'start and 'end'", "start and 'end"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractSummaryText(tt.input)
			if got != tt.want {
				t.Errorf("ExtractSummaryText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildLocatorCandidates
// ---------------------------------------------------------------------------

func TestBuildLocatorCandidates_WithDetail(t *testing.T) {
	ann := annotation.Annotation{
		ElementSummary: "button 'Submit'",
	}
	detail := &annotation.Detail{
		ID:                 "submit-btn",
		Selector:           "#submit-btn",
		SelectorCandidates: []string{"testid=submit", "role=button|Submit"},
	}
	candidates := BuildLocatorCandidates(ann, detail)

	if len(candidates) < 3 {
		t.Errorf("expected at least 3 candidates, got %d: %v", len(candidates), candidates)
	}
	// SelectorCandidates should come first.
	if candidates[0] != "testid=submit" {
		t.Errorf("first candidate should be testid=submit, got %s", candidates[0])
	}
}

func TestBuildLocatorCandidates_NilDetail(t *testing.T) {
	ann := annotation.Annotation{
		ElementSummary: "button 'Submit'",
	}
	candidates := BuildLocatorCandidates(ann, nil)

	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate from summary text, got %d: %v", len(candidates), candidates)
	}
	if candidates[0] != "text=Submit" {
		t.Errorf("expected text=Submit, got %s", candidates[0])
	}
}

func TestBuildLocatorCandidates_Dedup(t *testing.T) {
	ann := annotation.Annotation{ElementSummary: ""}
	detail := &annotation.Detail{
		Selector:           "#foo",
		SelectorCandidates: []string{"css=#foo"},
	}
	candidates := BuildLocatorCandidates(ann, detail)

	// "css=#foo" appears in both SelectorCandidates and as constructed from Selector.
	count := 0
	for _, c := range candidates {
		if c == "css=#foo" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected deduplication, got %d instances of css=#foo", count)
	}
}

// ---------------------------------------------------------------------------
// GenerateMarkdownReport
// ---------------------------------------------------------------------------

func TestGenerateMarkdownReport_EmptySessions(t *testing.T) {
	store := annotation.NewStore(1 * time.Hour)
	defer store.Close()

	report := GenerateMarkdownReport(nil, store)
	if !strings.Contains(report, "# Annotation Report") {
		t.Error("report should contain header")
	}
	if !strings.Contains(report, "**Total annotations:** 0 across 0 page(s)") {
		t.Error("report should show zero counts")
	}
}

func TestGenerateMarkdownReport_WithAnnotations(t *testing.T) {
	store := annotation.NewStore(1 * time.Hour)
	defer store.Close()

	store.StoreDetail("corr-1", annotation.Detail{
		Selector:  "#submit-btn",
		A11yFlags: []string{"missing-label"},
	})

	pages := []*annotation.Session{
		{
			PageURL:        "https://example.com/page1",
			ScreenshotPath: "/tmp/screenshot.png",
			Annotations: []annotation.Annotation{
				{
					ID:             "ann-1",
					Text:           "Fix this button",
					ElementSummary: "button 'Submit'",
					CorrelationID:  "corr-1",
					Rect:           annotation.Rect{X: 10, Y: 20, Width: 100, Height: 50},
				},
			},
		},
	}

	report := GenerateMarkdownReport(pages, store)
	if !strings.Contains(report, "## Page 1: https://example.com/page1") {
		t.Error("report should contain page section header")
	}
	if !strings.Contains(report, "Fix this button") {
		t.Error("report should contain annotation text")
	}
	if !strings.Contains(report, "#submit-btn") {
		t.Error("report should contain selector from detail")
	}
	if !strings.Contains(report, "missing-label") {
		t.Error("report should contain a11y flag")
	}
}

// ---------------------------------------------------------------------------
// BuildIssueList
// ---------------------------------------------------------------------------

func TestBuildIssueList_Empty(t *testing.T) {
	store := annotation.NewStore(1 * time.Hour)
	defer store.Close()

	issues := BuildIssueList(nil, store)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestBuildIssueList_WithAnnotations(t *testing.T) {
	store := annotation.NewStore(1 * time.Hour)
	defer store.Close()

	store.StoreDetail("corr-1", annotation.Detail{
		Selector:  "div.error",
		Tag:       "div",
		A11yFlags: []string{"low-contrast"},
	})

	pages := []*annotation.Session{
		{
			PageURL: "https://example.com",
			Annotations: []annotation.Annotation{
				{ID: "a1", Text: "Issue 1", CorrelationID: "corr-1", ElementSummary: "div"},
				{ID: "a2", Text: "Issue 2", CorrelationID: "corr-missing", ElementSummary: "span"},
			},
		},
	}

	issues := BuildIssueList(pages, store)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	// First issue should have detail enrichment.
	if issues[0]["selector"] != "div.error" {
		t.Errorf("first issue selector: want div.error, got %v", issues[0]["selector"])
	}
	flags, ok := issues[0]["a11y_flags"].([]string)
	if !ok || len(flags) != 1 {
		t.Errorf("first issue a11y_flags: want [low-contrast], got %v", issues[0]["a11y_flags"])
	}

	// Second issue should NOT have selector (detail missing).
	if _, exists := issues[1]["selector"]; exists {
		t.Error("second issue should not have selector (detail missing)")
	}
}

// ---------------------------------------------------------------------------
// GeneratePlaywrightFromAnnotations
// ---------------------------------------------------------------------------

func TestGeneratePlaywrightFromAnnotations_Structure(t *testing.T) {
	store := annotation.NewStore(1 * time.Hour)
	defer store.Close()

	pages := []*annotation.Session{
		{
			PageURL: "https://example.com",
			Annotations: []annotation.Annotation{
				{Text: "Check heading", ElementSummary: "h1 'Welcome'", CorrelationID: "c1"},
			},
		},
	}

	script := GeneratePlaywrightFromAnnotations("My Test", pages, store)
	if !strings.Contains(script, "import { test, expect }") {
		t.Error("script should have Playwright imports")
	}
	if !strings.Contains(script, "test('My Test'") {
		t.Error("script should contain test name")
	}
	if !strings.Contains(script, "page.goto('https://example.com')") {
		t.Error("script should navigate to page URL")
	}
	if !strings.Contains(script, "resolveAnnotationLocator") {
		t.Error("script should contain the locator resolution helper")
	}
}
