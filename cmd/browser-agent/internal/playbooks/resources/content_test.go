// content_test.go — Tests for playbook content serving and validation.

package resources

import (
	"strings"
	"testing"
)

func TestQuickstartContent_IncludesInteractFailureRecoveryExamples(t *testing.T) {
	t.Parallel()

	uri, text, ok := ResolveResourceContent("kaboom://quickstart")
	if !ok {
		t.Fatal("ResolveResourceContent(kaboom://quickstart) should succeed")
	}
	if uri != "kaboom://quickstart" {
		t.Fatalf("canonical uri = %q, want kaboom://quickstart", uri)
	}

	requiredTokens := []string{
		"element_not_found",
		"ambiguous_target",
		"stale_element_id",
		"scope_not_found",
		"blocked_by_overlay",
		"Stop and report evidence",
	}
	for _, token := range requiredTokens {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(token)) {
			t.Fatalf("quickstart missing token %q", token)
		}
	}
}
