// relay_boundary_test.go — the reconnect boundary between replayed scrollback and
// live fanout output must be SEAMLESS: every PTY chunk produced around the moment a
// viewer (re)subscribes must land in exactly one of {history snapshot, live
// channel} — never both (duplicate output) and never neither (lost output).
//
// SubscribeWithHistory snapshots scrollback and subscribes atomically (subMu)
// against the readLoop's appendAndBroadcast. This test runs a continuous producer
// streaming monotonic fixed-width markers while many subscribers join mid-stream,
// and asserts each subscriber's first live marker is exactly one past the last
// marker in its snapshot. With the two operations non-atomic, a marker in the gap
// is lost (channel starts past the boundary) or duplicated (channel repeats it), so
// some subscribe fails. (finding C)

package terminal

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestRelay_SubscribeWithHistory_SeamlessBoundary(t *testing.T) {
	// A dormant `cat` session: it never produces output on its own, so the relay's
	// readLoop stays blocked in Read and this test fully owns production via
	// appendAndBroadcast — no uncontrolled chunks perturb the marker stream.
	sess, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	relay := NewRelay(sess, "")
	t.Cleanup(func() { _ = sess.Close() })

	// Background subscribers that just drain: each broadcast then iterates ~25
	// channels under the fanout mutex, so a non-atomic subscribe blocks on that
	// mutex for longer — widening the window in which the producer appends markers
	// the subscribe then can't see (finding-C gap). The atomic code is unaffected
	// (subMu serializes append+broadcast against snapshot+subscribe regardless).
	var stopBg atomic.Bool
	bgDone := make(chan struct{}, 24)
	for i := 0; i < 24; i++ {
		ch, subErr := relay.Fanout().Subscribe(fmt.Sprintf("bg-%d", i))
		if subErr != nil {
			t.Fatalf("bg subscribe %d: %v", i, subErr)
		}
		go func() {
			defer func() { bgDone <- struct{}{} }()
			for range ch { // drain until closed so it is never dropped
				if stopBg.Load() {
					return
				}
			}
		}()
	}
	t.Cleanup(func() {
		stopBg.Store(true)
		for i := 0; i < 24; i++ {
			relay.Fanout().Unsubscribe(fmt.Sprintf("bg-%d", i))
		}
	})

	// One continuous producer keeps a constant stream (and constant fanout-lock
	// contention) so every mid-stream subscribe faces the same append/broadcast race
	// — far more reliable at exposing a non-atomic snapshot+subscribe than a
	// restart-per-round burst, whose contention is uneven.
	var stop atomic.Bool
	prodDone := make(chan struct{})
	go func() {
		defer close(prodDone)
		var v uint64
		for !stop.Load() {
			m := make([]byte, 8)
			binary.BigEndian.PutUint64(m, v)
			v++
			relay.appendAndBroadcast(m)
			runtime.Gosched()
		}
	}()
	t.Cleanup(func() { stop.Store(true); <-prodDone })

	// Let the stream warm up so history is non-empty for the first subscribe.
	warm := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(warm) {
		runtime.Gosched()
	}

	const rounds = 3000
	checked := 0
	for r := 0; r < rounds; r++ {
		subID := fmt.Sprintf("boundary-%d", r)
		history, sub, subErr := relay.SubscribeWithHistory(subID)
		if subErr != nil {
			t.Fatalf("round %d: subscribe: %v", r, subErr)
		}

		// Last marker captured in the snapshot. Scrollback eviction only trims whole
		// 8-byte markers off the FRONT, so the final 8 bytes are always the most
		// recent complete marker.
		var lastHist int64 = -1
		if n := len(history); n >= 8 {
			lastHist = int64(binary.BigEndian.Uint64(history[n-8 : n]))
		}

		select {
		case first, ok := <-sub:
			if ok && len(first) == 8 && lastHist >= 0 {
				c := int64(binary.BigEndian.Uint64(first))
				if c != lastHist+1 {
					t.Fatalf("round %d: non-seamless boundary — last snapshot marker=%d, first live marker=%d (gap or duplicate)", r, lastHist, c)
				}
				checked++
			}
			// !ok (dropped for backpressure) or a short read: not a boundary
			// observation — skip. Backpressure drop is a separate, legitimate path.
		case <-time.After(200 * time.Millisecond):
			// No live marker arrived promptly — skip (does not happen under a live stream).
		}
		relay.Fanout().Unsubscribe(subID)
	}

	if checked == 0 {
		t.Fatal("no round actually observed the snapshot/subscribe boundary — the test proves nothing; retune the producer")
	}
	t.Logf("observed %d boundary checks across %d rounds", checked, rounds)
}
