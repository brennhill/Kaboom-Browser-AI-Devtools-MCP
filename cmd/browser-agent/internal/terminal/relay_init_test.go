// relay_init_test.go — WaitForPromptViaRelay must use a UNIQUE fanout subscriber id
// per call. With a constant "init-cmd" id, two concurrent inits on one relay both
// Subscribe("init-cmd"); Fanout overwrites the map entry without closing the first
// channel, and the first init's deferred Unsubscribe("init-cmd") then closes the
// SECOND's channel, making it bail early without writing its init command (I).

package terminal

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestWaitForPromptViaRelay_ConcurrentInitsUseDistinctSubscribers(t *testing.T) {
	// A `cat` session produces no output, so neither init sees a prompt: both block
	// until InitTimeout, staying subscribed long enough to observe the fanout.
	sess, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	relay := NewRelay(sess, "")
	t.Cleanup(func() { _ = sess.Close() })

	for _, cmd := range []string{"init-one", "init-two"} {
		c := cmd
		go func() { WaitForPromptViaRelay(relay, c) }()
	}

	// Two concurrent inits must register two distinct subscribers. With the fixed
	// "init-cmd" id the second Subscribe overwrites the first in the fanout map, so
	// the count never reaches 2.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if relay.Fanout().Count() >= 2 {
			return // distinct subscriber ids — the fix works
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("two concurrent inits must yield 2 distinct fanout subscribers, got %d (fixed sub id collides)", relay.Fanout().Count())
}
