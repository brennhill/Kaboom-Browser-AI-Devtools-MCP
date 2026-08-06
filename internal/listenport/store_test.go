// store_test.go — Verifies canonical daemon listen-port state.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package listenport

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
)

func TestStoreDefaultsAndRejectsInvalidReplacement(t *testing.T) {
	t.Parallel()
	store := New()
	if got := store.Get(); got != serverdefaults.Port {
		t.Fatalf("default port = %d", got)
	}
	store.Set(8123)
	store.Set(0)
	if got := store.Get(); got != 8123 {
		t.Fatalf("port after invalid replacement = %d", got)
	}
}
