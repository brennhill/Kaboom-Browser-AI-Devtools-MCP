// owner_test.go — Verifies annotation-store resource lifecycle ownership.
// Docs: docs/features/feature/annotated-screenshots/index.md

package runtime

import (
	"testing"
	"time"
)

func TestOwnerReusesStoreUntilCloseAndThenRecreates(t *testing.T) {
	t.Parallel()
	owner := New(time.Minute)
	first := owner.Store()
	if second := owner.Store(); second != first {
		t.Fatal("Store returned different live instances")
	}
	owner.Close()
	if replacement := owner.Store(); replacement == first {
		t.Fatal("Store reused a closed instance")
	}
	owner.Close()
}
