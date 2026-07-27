// handler_test.go — Tests combined-audit category validation.

package combinedaudit

import "testing"

func TestValidateCategories(t *testing.T) {
	t.Parallel()
	if categories, invalid := validateCategories(nil); invalid != "" || len(categories) != 4 {
		t.Fatalf("defaults = %v, invalid = %q", categories, invalid)
	}
	if _, invalid := validateCategories([]string{"performance", "unknown"}); invalid != "unknown" {
		t.Fatalf("invalid category = %q", invalid)
	}
}
