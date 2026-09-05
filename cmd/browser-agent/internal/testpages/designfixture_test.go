// designfixture_test.go — Binds design-drift.html to the computed-style capture
// the design_audit analyzers are tested against.
//
// PURPOSE: the analyzers read a JSON capture of this page, not the page. Nothing
// connected the two, so editing the markup or the stylesheet left the capture
// describing a page that no longer exists — and the whole Go suite stayed green
// while UAT category 36, which runs the same expectations against the live page,
// was the only thing that could notice.
//
// Two halves. The digests fail on ANY change to <style> or <body>, which is the
// signal to re-capture. The structural assertions say WHAT diverged, so the
// failure names the group whose element count, generated selector or :root token
// no longer matches.

package testpages

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const designFixtureCapture = "../toolanalyze/designdrift/testdata/fixture-probe.json"

// capturedPayload is the part of one selector's capture this test can derive
// from the page itself. Geometry is deliberately absent: the capture records
// absolute page coordinates, which every section above a group shifts, while the
// analyzers only ever read differences.
type capturedPayload struct {
	Elements []struct {
		Selector string `json:"selector"`
		InFlow   bool   `json:"in_flow"`
	} `json:"elements"`
	Count      int               `json:"count"`
	MatchCount int               `json:"match_count"`
	RootTokens map[string]string `json:"root_tokens"`
}

type captureSource struct {
	Page        string `json:"page"`
	StyleSHA256 string `json:"style_sha256"`
	BodySHA256  string `json:"body_sha256"`
}

// htmlElement is one opening tag in document order, in the terms the extension's
// buildSelector uses.
type htmlElement struct {
	tag     string
	id      string
	classes []string
}

var (
	styleBlockPattern = regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	bodyBlockPattern  = regexp.MustCompile(`(?s)<body>(.*?)</body>`)
	rootBlockPattern  = regexp.MustCompile(`(?s):root\s*\{(.*?)\}`)
	customPropPattern = regexp.MustCompile(`(--[A-Za-z0-9_-]+)\s*:\s*([^;]+);`)
	openTagPattern    = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)((?:\s+[^<>]*)?)>`)
	attrPattern       = regexp.MustCompile(`([a-zA-Z-]+)="([^"]*)"`)
)

// TestDesignDriftFixtureMatchesItsPage is the link that was missing.
func TestDesignDriftFixtureMatchesItsPage(t *testing.T) {
	t.Parallel()

	page, err := os.ReadFile(filepath.Join("pages", "design-drift.html"))
	if err != nil {
		t.Fatalf("read design-drift.html: %v", err)
	}
	raw, err := os.ReadFile(designFixtureCapture)
	if err != nil {
		t.Fatalf("read %s: %v", designFixtureCapture, err)
	}
	var capture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatalf("parse %s: %v", designFixtureCapture, err)
	}

	style := blockOf(t, styleBlockPattern, string(page), "<style>")
	body := blockOf(t, bodyBlockPattern, string(page), "<body>")
	assertSourceDigests(t, capture, style, body)

	elements := parseElements(body)
	tokens := parseRootTokens(style)
	outOfFlow := 0

	for key, rawPayload := range capture {
		if strings.HasPrefix(key, "_") {
			continue
		}
		if !strings.HasPrefix(key, ".") {
			t.Errorf("capture key %q is not a class selector; this test derives matches from the class attribute", key)
			continue
		}
		var payload capturedPayload
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			t.Errorf("parse capture for %s: %v", key, err)
			continue
		}
		outOfFlow += assertGroupMatchesPage(t, key, payload, elements, tokens)
	}

	// The out-of-flow protection is the one analyzer guard that only exists at
	// the wire boundary: every element view a unit test builds sets InFlow
	// itself, so a capture where everything is in flow lets viewsFrom read the
	// field as true and nothing notices.
	if outOfFlow == 0 {
		t.Error("no captured element is out of flow; the in_flow translation is unexercised by the fixture")
	}
}

// assertGroupMatchesPage checks one selector's capture against the page and
// returns how many of its elements are out of flow.
func assertGroupMatchesPage(t *testing.T, key string, payload capturedPayload, elements []htmlElement, tokens map[string]string) int {
	t.Helper()
	class := strings.TrimPrefix(key, ".")

	var want []string
	for _, el := range elements {
		if namesClass(el, class) {
			want = append(want, generatedSelector(el))
		}
	}
	if len(want) == 0 {
		t.Errorf("%s: the capture holds a group the page no longer renders", key)
		return 0
	}
	if payload.Count != len(want) || payload.MatchCount != len(want) || len(payload.Elements) != len(want) {
		t.Errorf("%s: the page renders %d element(s); the capture holds count=%d match_count=%d elements=%d — re-capture the fixture",
			key, len(want), payload.Count, payload.MatchCount, len(payload.Elements))
		return 0
	}

	outOfFlow := 0
	for i, el := range payload.Elements {
		if el.Selector != want[i] {
			t.Errorf("%s: element %d is %q in the capture and %q on the page — a class was renamed or the markup was reordered",
				key, i, el.Selector, want[i])
		}
		if !el.InFlow {
			outOfFlow++
		}
	}
	if !equalTokens(payload.RootTokens, tokens) {
		t.Errorf("%s: captured root_tokens %v do not match the page's :root block %v", key, payload.RootTokens, tokens)
	}
	return outOfFlow
}

// assertSourceDigests is the half that fires on a change this test cannot model:
// a margin, a height, a colour. It cannot say what broke, only that the capture
// is no longer of this page.
func assertSourceDigests(t *testing.T, capture map[string]json.RawMessage, style, body string) {
	t.Helper()
	rawSource, present := capture["_source"]
	if !present {
		t.Fatal("the capture declares no _source; nothing records which page it was taken from")
	}
	var source captureSource
	if err := json.Unmarshal(rawSource, &source); err != nil {
		t.Fatalf("parse _source: %v", err)
	}
	if source.Page != "cmd/browser-agent/internal/testpages/pages/design-drift.html" {
		t.Errorf("_source.page = %q, want the design-drift fixture page", source.Page)
	}
	for _, tc := range []struct{ name, got, want string }{
		{"style_sha256", source.StyleSHA256, digest(style)},
		{"body_sha256", source.BodySHA256, digest(body)},
	} {
		if tc.got != tc.want {
			t.Errorf("design-drift.html %s changed (capture records %s, page hashes to %s): the capture describes the previous page. Re-run the probe against the edited page, update fixture-probe.json and expected-findings.json, then record the new digest.",
				tc.name, tc.got, tc.want)
		}
	}
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func blockOf(t *testing.T, pattern *regexp.Regexp, page, name string) string {
	t.Helper()
	m := pattern.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("design-drift.html has no %s block", name)
	}
	return m[1]
}

// parseElements reads the opening tags of the body in document order. The
// fixture is hand-written, single-quoted-attribute-free markup, so a tag scan is
// enough — and it stays enough because this test fails the moment the body
// changes at all.
func parseElements(body string) []htmlElement {
	var out []htmlElement
	for _, m := range openTagPattern.FindAllStringSubmatch(body, -1) {
		el := htmlElement{tag: strings.ToLower(m[1])}
		for _, attr := range attrPattern.FindAllStringSubmatch(m[2], -1) {
			switch strings.ToLower(attr[1]) {
			case "class":
				el.classes = strings.Fields(attr[2])
			case "id":
				el.id = attr[2]
			}
		}
		out = append(out, el)
	}
	return out
}

func parseRootTokens(style string) map[string]string {
	block := rootBlockPattern.FindStringSubmatch(style)
	if block == nil {
		return nil
	}
	tokens := make(map[string]string)
	for _, m := range customPropPattern.FindAllStringSubmatch(block[1], -1) {
		tokens[m[1]] = strings.TrimSpace(m[2])
	}
	return tokens
}

func namesClass(el htmlElement, class string) bool {
	for _, candidate := range el.classes {
		if candidate == class {
			return true
		}
	}
	return false
}

// generatedSelector mirrors buildSelector in src/inject/computed-styles.ts: the
// id if there is one, otherwise the tag and its first three classes. The capture
// records whatever that function produced, so this test has to produce the same.
func generatedSelector(el htmlElement) string {
	if el.id != "" {
		return "#" + el.id
	}
	classes := el.classes
	if len(classes) > 3 {
		classes = classes[:3]
	}
	return el.tag + "." + strings.Join(classes, ".")
}

func equalTokens(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for name, value := range want {
		if got[name] != value {
			return false
		}
	}
	return true
}
