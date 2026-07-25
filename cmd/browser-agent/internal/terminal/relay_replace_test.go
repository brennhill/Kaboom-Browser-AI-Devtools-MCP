// relay_replace_test.go — the self-heal path must rebind a session's relay to the
// FRESH session atomically. A separate Remove + GetOrCreate lets a concurrent WS
// GetOrCreate (a reconnect that fetched the session before its token was
// invalidated) slip into the gap and bind the relay to the just-evicted DEAD
// session, which GetOrCreate then returns unchanged. ReplaceRelay overwrites under
// one lock so the fresh binding always wins (finding H).

package terminal

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestMap_ReplaceRelay_BindsFreshSessionUnderConcurrency(t *testing.T) {
	spawnCat := func() *pty.Session {
		s, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
		if err != nil {
			t.Fatalf("spawn: %v", err)
		}
		return s
	}

	for round := 0; round < 60; round++ {
		dead := spawnCat()
		fresh := spawnCat()
		m := NewMap()
		// A WS attached to the (soon-to-be-dead) session before the heal.
		m.GetOrCreate("id", dead, "")

		// Hammers model concurrent WS reconnects on the dead session, calling
		// GetOrCreate continuously so one reliably lands in any Remove→create gap.
		var stop atomic.Bool
		var wg sync.WaitGroup
		for h := 0; h < 4; h++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for !stop.Load() {
					m.GetOrCreate("id", dead, "")
				}
			}()
		}

		// Self-heal replaces the relay with one bound to the fresh session.
		m.ReplaceRelay("id", fresh, "dir")
		stop.Store(true)
		wg.Wait()

		got := m.Get("id")
		if got == nil {
			t.Fatalf("round %d: no relay bound after replace", round)
		}
		if got.sess != fresh {
			t.Fatalf("round %d: relay must bind the FRESH session after self-heal (bound-to-dead=%v)", round, got.sess == dead)
		}

		m.CloseAll()
		_ = dead.Close()
		_ = fresh.Close()
	}
}
