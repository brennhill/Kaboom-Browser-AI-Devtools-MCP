// templates.go — Hardcoded issue templates for common report categories.

package issuereport

import "sort"

// templates is the set of available issue templates (unexported to prevent mutation).
var templates = map[string]IssueTemplate{
	"bug": {
		Name:        "bug",
		Title:       "Bug Report",
		Description: "Report unexpected behavior or errors",
		Labels:      []string{"bug", "user-reported"},
		Sections:    []string{"description", "steps_to_reproduce", "expected_behavior", "diagnostics"},
	},
	"crash": {
		Name:        "crash",
		Title:       "Crash Report",
		Description: "Report a daemon crash or hang",
		Labels:      []string{"bug", "crash", "user-reported"},
		Sections:    []string{"description", "crash_context", "diagnostics"},
	},
	"extension_issue": {
		Name:        "extension_issue",
		Title:       "Extension Issue",
		Description: "Report extension connectivity or behavior problems",
		Labels:      []string{"extension", "user-reported"},
		Sections:    []string{"description", "extension_state", "diagnostics"},
	},
	"performance": {
		Name:        "performance",
		Title:       "Performance Issue",
		Description: "Report slow responses or high resource usage",
		Labels:      []string{"performance", "user-reported"},
		Sections:    []string{"description", "performance_context", "diagnostics"},
	},
	"feature_request": {
		Name:        "feature_request",
		Title:       "Feature Request",
		Description: "Suggest a new feature or improvement",
		Labels:      []string{"enhancement", "user-reported"},
		Sections:    []string{"description", "use_case"},
	},
}

// TemplateNames returns the sorted list of available template names.
// Derived from the map keys to prevent drift.
func TemplateNames() []string {
	names := make([]string, 0, len(templates))
	for k := range templates {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// GetTemplate returns a template by name, or nil if not found.
func GetTemplate(name string) *IssueTemplate {
	t, ok := templates[name]
	if !ok {
		return nil
	}
	return &t
}
