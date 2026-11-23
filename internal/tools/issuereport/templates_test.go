// templates_test.go — Tests for issue template lookup and consistency.

package issuereport

import (
	"testing"
)

func TestTemplateNames_ReturnsFiveTemplates(t *testing.T) {
	t.Parallel()
	names := TemplateNames()
	if len(names) != 5 {
		t.Fatalf("TemplateNames() returned %d, want 5", len(names))
	}
}

func TestTemplateNames_AreSorted(t *testing.T) {
	t.Parallel()
	names := TemplateNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("TemplateNames() not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestGetTemplate_Found(t *testing.T) {
	t.Parallel()
	for _, name := range TemplateNames() {
		tmpl := GetTemplate(name)
		if tmpl == nil {
			t.Errorf("GetTemplate(%q) = nil, want template", name)
			continue
		}
		if tmpl.Name != name {
			t.Errorf("GetTemplate(%q).Name = %q", name, tmpl.Name)
		}
		if tmpl.Title == "" {
			t.Errorf("GetTemplate(%q).Title is empty", name)
		}
		if len(tmpl.Labels) == 0 {
			t.Errorf("GetTemplate(%q).Labels is empty", name)
		}
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	t.Parallel()
	if tmpl := GetTemplate("nonexistent"); tmpl != nil {
		t.Fatalf("GetTemplate(nonexistent) = %v, want nil", tmpl)
	}
}

func TestGetTemplate_AllHaveUserReportedLabel(t *testing.T) {
	t.Parallel()
	for _, name := range TemplateNames() {
		tmpl := GetTemplate(name)
		found := false
		for _, label := range tmpl.Labels {
			if label == "user-reported" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetTemplate(%q) missing user-reported label", name)
		}
	}
}
