// retention_policy_test.go — Pins the disk budgets. These numbers are the whole
// defence against the 1.0GB state directory this feature was written for.

package main

import "testing"

func TestCaptureBudgetsAreAllBounded(t *testing.T) {
	budgets := captureBudgets()
	if len(budgets) == 0 {
		t.Fatal("captureBudgets() is empty; nothing would ever be reclaimed")
	}
	for _, budget := range budgets {
		if budget.Name == "" {
			t.Error("a budget has no name; its sweeps would be unattributable in logs")
		}
		if budget.Budget.IsZero() {
			t.Errorf("%s has a zero budget, which reclaims nothing", budget.Name)
		}
		if budget.Budget.MaxFiles <= 0 {
			t.Errorf("%s has no file-count ceiling; 5,975 screenshots accumulated without one", budget.Name)
		}
		if budget.Budget.MaxBytes <= 0 {
			t.Errorf("%s has no byte ceiling", budget.Name)
		}
		if budget.Budget.MaxAge <= 0 {
			t.Errorf("%s has no age ceiling", budget.Name)
		}
	}
}

// Every budgeted directory must resolve, or a typo would silently disable a sweep.
func TestCaptureBudgetsResolveTheirDirectories(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	for _, budget := range captureBudgets() {
		dir, err := budget.Dir()
		if err != nil {
			t.Errorf("%s directory did not resolve: %v", budget.Name, err)
		}
		if dir == "" {
			t.Errorf("%s resolved to an empty path", budget.Name)
		}
	}
}
