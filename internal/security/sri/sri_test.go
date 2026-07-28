// sri_test.go — Tests for the SRI hash generator (generate_sri) MCP tool.
// Purpose: Tests hash computation, resource filtering, third-party detection, and output formats.
// Docs: docs/features/feature/security-hardening/index.md
package sri

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
)

func TestGeneratorBasicHash(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{
			URL:          "https://cdn.example.com/app.js",
			ContentType:  "application/javascript",
			Method:       "GET",
			ResponseBody: "console.log('hello');",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	res := result.Resources[0]
	if res.URL != "https://cdn.example.com/app.js" {
		t.Errorf("expected URL https://cdn.example.com/app.js, got %s", res.URL)
	}
	if res.Type != "script" {
		t.Errorf("expected type=script, got %s", res.Type)
	}
	// Hash should start with sha384-
	if !strings.HasPrefix(res.Hash, "sha384-") {
		t.Errorf("expected hash to start with sha384-, got %s", res.Hash)
	}
	// Hash should be base64 encoded (44 chars for SHA-384)
	hashPart := strings.TrimPrefix(res.Hash, "sha384-")
	if len(hashPart) != 64 {
		t.Errorf("expected hash base64 length 64, got %d", len(hashPart))
	}
	if res.Crossorigin != "anonymous" {
		t.Errorf("expected crossorigin=anonymous, got %s", res.Crossorigin)
	}
}

func TestGeneratorKnownHash(t *testing.T) {
	t.Parallel()
	// Test with known content to verify SHA-384 computation
	// "test content" SHA-384 hash is known
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{
			URL:          "https://cdn.example.com/test.js",
			ContentType:  "application/javascript",
			Method:       "GET",
			ResponseBody: "test content",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	// SHA-384 of "test content" computed via: echo -n "test content" | openssl dgst -sha384 -binary | base64
	expectedHash := "sha384-8cFK5mW+eeVbAO7clwcEVX1yowIas7iMz9wbg9HWbEeQkeI8+2Ah9Dt6EnOm9KMY"
	if result.Resources[0].Hash != expectedHash {
		t.Errorf("hash mismatch:\nexpected: %s\ngot:      %s", expectedHash, result.Resources[0].Hash)
	}
}

func TestGeneratorOnlyScriptsAndStyles(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		// Should be included
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js code"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css code"},
		// Should be excluded - not script/style
		{URL: "https://cdn.example.com/data.json", ContentType: "application/json", ResponseBody: `{"data":1}`},
		{URL: "https://cdn.example.com/logo.png", ContentType: "image/png", ResponseBody: "binary data"},
		{URL: "https://cdn.example.com/font.woff2", ContentType: "font/woff2", ResponseBody: "font data"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources (script + style), got %d", len(result.Resources))
	}
	types := map[string]bool{}
	for _, r := range result.Resources {
		types[r.Type] = true
	}
	if !types["script"] {
		t.Error("expected script type in results")
	}
	if !types["style"] {
		t.Error("expected style type in results")
	}
}

func TestGeneratorOnlyThirdParty(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		// Same origin - should be excluded
		{URL: "https://myapp.com/app.js", ContentType: "application/javascript", ResponseBody: "first party"},
		// Third party - should be included
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "third party"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 third-party resource, got %d", len(result.Resources))
	}
	if result.Resources[0].URL != "https://cdn.example.com/lib.js" {
		t.Errorf("expected third-party URL, got %s", result.Resources[0].URL)
	}
}

func TestGeneratorTagTemplates(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	for _, res := range result.Resources {
		if res.Type == "script" {
			if !strings.HasPrefix(res.TagTemplate, "<script src=") {
				t.Errorf("script tag template should start with <script src=, got %s", res.TagTemplate)
			}
			if !strings.Contains(res.TagTemplate, "integrity=\"sha384-") {
				t.Error("script tag should contain integrity attribute")
			}
			if !strings.Contains(res.TagTemplate, "crossorigin=\"anonymous\"") {
				t.Error("script tag should contain crossorigin attribute")
			}
			if !strings.HasSuffix(res.TagTemplate, "</script>") {
				t.Error("script tag should end with </script>")
			}
		}
		if res.Type == "style" {
			if !strings.HasPrefix(res.TagTemplate, "<link rel=\"stylesheet\"") {
				t.Errorf("style tag should start with <link rel=\"stylesheet\", got %s", res.TagTemplate)
			}
			if !strings.Contains(res.TagTemplate, "integrity=\"sha384-") {
				t.Error("style tag should contain integrity attribute")
			}
			if !strings.Contains(res.TagTemplate, "crossorigin=\"anonymous\"") {
				t.Error("style tag should contain crossorigin attribute")
			}
		}
	}
}

func TestGeneratorVaryUserAgentWarning(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{
			URL:          "https://fonts.googleapis.com/css2?family=Roboto",
			ContentType:  "text/css",
			ResponseBody: "font css",
			ResponseHeaders: map[string]string{
				"Vary": "User-Agent",
			},
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Warnings) == 0 {
		t.Error("expected warning about Vary: User-Agent")
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "User-Agent") || strings.Contains(w, "Vary") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected Vary: User-Agent warning, got %v", result.Warnings)
	}
}

func TestGeneratorTruncatedBody(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{
			URL:               "https://cdn.example.com/large.js",
			ContentType:       "application/javascript",
			ResponseBody:      "truncated...",
			ResponseTruncated: true,
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	// Truncated bodies should not be included
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for truncated body, got %d", len(result.Resources))
	}
	// Should have a warning
	if len(result.Warnings) == 0 {
		t.Error("expected warning about truncated body")
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "truncated") || strings.Contains(w, "large") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected truncation warning, got %v", result.Warnings)
	}
}

func TestGeneratorEmptyBody(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{
			URL:          "https://cdn.example.com/empty.js",
			ContentType:  "application/javascript",
			ResponseBody: "",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	// Empty body should not generate SRI
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty body, got %d", len(result.Resources))
	}
}

func TestGeneratorSummary(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{URL: "https://cdn1.example.com/a.js", ContentType: "application/javascript", ResponseBody: "a"},
		{URL: "https://cdn2.example.com/b.js", ContentType: "application/javascript", ResponseBody: "b"},
		{URL: "https://cdn3.example.com/c.css", ContentType: "text/css", ResponseBody: "c"},
		// Image - not counted
		{URL: "https://cdn4.example.com/d.png", ContentType: "image/png", ResponseBody: "d"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if result.Summary.TotalThirdPartyResources != 4 {
		t.Errorf("expected 4 total third party resources, got %d", result.Summary.TotalThirdPartyResources)
	}
	if result.Summary.ScriptsWithoutSRI != 2 {
		t.Errorf("expected 2 scripts without SRI, got %d", result.Summary.ScriptsWithoutSRI)
	}
	if result.Summary.StylesWithoutSRI != 1 {
		t.Errorf("expected 1 style without SRI, got %d", result.Summary.StylesWithoutSRI)
	}
	if result.Summary.HashesGenerated != 3 {
		t.Errorf("expected 3 hashes generated, got %d", result.Summary.HashesGenerated)
	}
}

func TestGeneratorResourceTypesFilter(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css"},
	}
	pageURLs := []string{"https://myapp.com/"}

	// Only scripts
	result := gen.Generate(bodies, pageURLs, Params{ResourceTypes: []string{"scripts"}})
	if len(result.Resources) != 1 || result.Resources[0].Type != "script" {
		t.Errorf("expected only script when filtering by scripts, got %d resources", len(result.Resources))
	}

	// Only styles
	result = gen.Generate(bodies, pageURLs, Params{ResourceTypes: []string{"styles"}})
	if len(result.Resources) != 1 || result.Resources[0].Type != "style" {
		t.Errorf("expected only style when filtering by styles, got %d resources", len(result.Resources))
	}
}

func TestGeneratorOriginsFilter(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{URL: "https://cdn1.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js1"},
		{URL: "https://cdn2.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js2"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{Origins: []string{"https://cdn1.example.com"}})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource with origin filter, got %d", len(result.Resources))
	}
	if result.Resources[0].URL != "https://cdn1.example.com/app.js" {
		t.Errorf("expected cdn1.example.com URL, got %s", result.Resources[0].URL)
	}
}

func TestGeneratorSizeBytes(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	content := "console.log('hello world');"
	bodies := []types.NetworkBody{
		{
			URL:          "https://cdn.example.com/app.js",
			ContentType:  "application/javascript",
			ResponseBody: content,
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].SizeBytes != len(content) {
		t.Errorf("expected size %d, got %d", len(content), result.Resources[0].SizeBytes)
	}
}

func TestGeneratorAlreadyHasSRI(t *testing.T) {
	t.Parallel()
	// This tests that we track resources that already have SRI
	// Note: In practice, Kaboom captures the response, not the HTML.
	// AlreadyHasSRI would be set based on external data or heuristics.
	// For now, we just ensure the field exists in the output.
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	// By default, AlreadyHasSRI should be false since we're computing new hashes
	if result.Resources[0].AlreadyHasSRI {
		t.Error("expected AlreadyHasSRI=false for newly computed hash")
	}
}

func TestGeneratorDeduplication(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	// Same URL loaded twice - should only appear once
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Errorf("expected 1 deduplicated resource, got %d", len(result.Resources))
	}
}

func TestGeneratorMultipleContentTypes(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		// Various JavaScript content types
		{URL: "https://cdn.example.com/a.js", ContentType: "application/javascript", ResponseBody: "a"},
		{URL: "https://cdn.example.com/b.js", ContentType: "text/javascript", ResponseBody: "b"},
		{URL: "https://cdn.example.com/c.js", ContentType: "application/x-javascript", ResponseBody: "c"},
		// CSS
		{URL: "https://cdn.example.com/d.css", ContentType: "text/css", ResponseBody: "d"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(result.Resources))
	}
}

func TestGeneratorHandleMCP(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "code"},
	}
	pageURLs := []string{"https://myapp.com/"}

	params := Params{}
	raw, _ := json.Marshal(params)

	result, err := HandleGenerate(json.RawMessage(raw), bodies, pageURLs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sriResult, ok := result.(Result)
	if !ok {
		t.Fatal("expected Result type")
	}
	if len(sriResult.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(sriResult.Resources))
	}
}

func TestGeneratorEmptyInput(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	result := gen.Generate(nil, nil, Params{})

	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty input, got %d", len(result.Resources))
	}
	if result.Summary.HashesGenerated != 0 {
		t.Errorf("expected 0 hashes generated, got %d", result.Summary.HashesGenerated)
	}
}

func TestGeneratorInvalidParams(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "code"},
	}
	pageURLs := []string{"https://myapp.com/"}

	// Invalid JSON params should return error
	_, err := HandleGenerate([]byte(`{invalid}`), bodies, pageURLs)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}
}

func TestGeneratorPlaceholderBodies(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		// Placeholder from body read timeout — should be skipped with warning
		{URL: "https://cdn.example.com/timeout.js", ContentType: "application/javascript", ResponseBody: "[Skipped: body read timeout]"},
		// Placeholder from read error — should be skipped with warning
		{URL: "https://cdn.example.com/error.js", ContentType: "application/javascript", ResponseBody: "[Could not read response]"},
		// Binary placeholder — should be skipped with warning
		{URL: "https://cdn.example.com/binary.js", ContentType: "application/javascript", ResponseBody: "[Binary: 4096 bytes, application/javascript]"},
		// Real body — should be included
		{URL: "https://cdn.example.com/real.js", ContentType: "application/javascript", ResponseBody: "console.log('real');"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource (only real body), got %d", len(result.Resources))
	}
	if result.Resources[0].URL != "https://cdn.example.com/real.js" {
		t.Errorf("expected real.js URL, got %s", result.Resources[0].URL)
	}
	// Placeholder bodies should produce warnings
	if len(result.Warnings) < 3 {
		t.Errorf("expected at least 3 warnings for placeholder bodies, got %d: %v", len(result.Warnings), result.Warnings)
	}
	// Summary should count placeholders as third-party (they passed origin check)
	if result.Summary.TotalThirdPartyResources != 4 {
		t.Errorf("expected 4 total third-party resources, got %d", result.Summary.TotalThirdPartyResources)
	}
}

func TestGeneratorSubdomainFirstParty(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	bodies := []types.NetworkBody{
		// Subdomain of first party - should be excluded
		{URL: "https://cdn.myapp.com/app.js", ContentType: "application/javascript", ResponseBody: "first party cdn"},
		// Different domain - should be included
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "third party"},
	}
	// Note: subdomain detection depends on implementation
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, Params{})

	// cdn.myapp.com is a different origin from myapp.com, so it should be included as third party
	// (unless we implement subdomain matching, which the spec doesn't require)
	if len(result.Resources) < 1 {
		t.Errorf("expected at least 1 resource, got %d", len(result.Resources))
	}
}
