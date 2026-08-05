// catalog_test.go — Verifies the MCP resource discovery catalog and package boundary.

package resources

import (
	"os"
	"testing"
)

func TestResourcesPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("playbook resources package has %d files; want at most 10 change-coupled owners", files)
	}
}

func TestResourcesMatchResolvableCanonicalContent(t *testing.T) {
	resources := Resources()
	if len(resources) != 3 {
		t.Fatalf("expected three canonical resources, got %d", len(resources))
	}
	for _, resource := range resources {
		canonicalURI, content, ok := ResolveResourceContent(resource.URI)
		if !ok {
			t.Fatalf("catalog resource %q is not resolvable", resource.URI)
		}
		if canonicalURI != resource.URI {
			t.Fatalf("resource %q resolved as %q", resource.URI, canonicalURI)
		}
		if content == "" {
			t.Fatalf("resource %q has empty content", resource.URI)
		}
		if resource.MimeType != "text/markdown" {
			t.Fatalf("resource %q mime type = %q", resource.URI, resource.MimeType)
		}
	}
}

func TestResourceTemplatesDeclarePlaybookAndDemoFamilies(t *testing.T) {
	templates := ResourceTemplates()
	if len(templates) != 2 {
		t.Fatalf("expected two resource templates, got %d", len(templates))
	}
	want := map[string]bool{
		"kaboom://playbook/{capability}/{level}": false,
		"kaboom://demo/{name}":                   false,
	}
	for _, template := range templates {
		fields, ok := template.(map[string]any)
		if !ok {
			t.Fatalf("template has unexpected type %T", template)
		}
		uri, _ := fields["uriTemplate"].(string)
		if _, ok := want[uri]; !ok {
			t.Fatalf("unexpected resource template %q", uri)
		}
		want[uri] = true
	}
	for uri, found := range want {
		if !found {
			t.Fatalf("missing resource template %q", uri)
		}
	}
}
